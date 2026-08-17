package input

import (
	"errors"
	"fmt"
	"image"
	"log/slog"
	"sync"
	"time"
)

// Controller manages D2R window binding and keyboard/mouse input primitives.
type Controller struct {
	log      *slog.Logger
	api      windowAPI
	keys     KeySender
	mouse    MouseSender
	keyboard KeyboardConfig
	timings  keyTimings

	stateMu        sync.Mutex
	enabled        bool
	paused         bool
	stopped        bool
	hotkeyBindings HotkeyBindings
	hotkeyListen   HotkeyListener
	hotkeyWG       sync.WaitGroup

	mu         sync.Mutex
	gameplayMu sync.Mutex
	keyMu      sync.Mutex
	mouseMu    sync.Mutex
	heldMu     sync.Mutex
	window     WindowInfo
	bound      bool
	heldActive bool
	heldKey    Key
	heldButton MouseButton
}

// CaptureClient returns a read-only RGBA screenshot of the complete bound D2R
// client area. The capture does not activate the window or produce input.
func (c *Controller) CaptureClient() (*image.RGBA, error) {
	c.mu.Lock()
	if !c.bound {
		c.mu.Unlock()
		return nil, fmt.Errorf("capture client: %w", ErrWindowNotBound)
	}
	win := c.window
	c.mu.Unlock()
	img, err := captureClientWindow(win)
	if err != nil {
		return nil, err
	}
	c.log.Debug("input window captured",
		"client_width", win.ClientWidth,
		"client_height", win.ClientHeight,
	)
	return img, nil
}

// NewController creates an input controller with platform window, keyboard, and mouse backends.
// It returns an error when safety hotkey strings cannot be normalized.
func NewController(log *slog.Logger, keyboard KeyboardConfig, safety SafetyConfig) (*Controller, error) {
	return newWithBackends(log, defaultWindowAPI(log), defaultKeySender(log), defaultMouseSender(log), keyboard, safety, defaultKeyTimings(), nil)
}

func newWithBackends(
	log *slog.Logger,
	api windowAPI,
	keys KeySender,
	mouse MouseSender,
	keyboard KeyboardConfig,
	safety SafetyConfig,
	timings keyTimings,
	hotkeyListen HotkeyListener,
) (*Controller, error) {
	bindings, err := normalizeHotkeyBindings(safety)
	if err != nil {
		return nil, err
	}

	componentLog := log.With("component", "input")
	logSafetyConfigured(componentLog, safety)

	return &Controller{
		log:            componentLog,
		api:            api,
		keys:           keys,
		mouse:          mouse,
		keyboard:       keyboard,
		timings:        timings,
		enabled:        safety.Enabled,
		hotkeyBindings: bindings,
		hotkeyListen:   hotkeyListen,
	}, nil
}

// Ready reports whether the controller is initialized (not whether a window is bound).
func (c *Controller) Ready() bool {
	c.log.Debug("input controller ready")
	return true
}

// Bind finds the D2R main window for pid and stores its client-area geometry.
func (c *Controller) Bind(pid uint32) error {
	return c.bindWindow(pid, false)
}

// BindVisible restores a minimized D2R window once, then binds its client area.
// Idle polling uses [Bind] so a focused dashboard does not yank D2R every second;
// character selection and offline start call this when the client rect is still 0×0.
func (c *Controller) BindVisible(pid uint32) error {
	return c.bindWindow(pid, true)
}

func (c *Controller) bindWindow(pid uint32, restore bool) error {
	if pid == 0 {
		return fmt.Errorf("bind window: %w", ErrInvalidPID)
	}
	c.gameplayMu.Lock()
	defer c.gameplayMu.Unlock()

	hwnd, err := c.api.FindMainWindow(pid, defaultWindowTitle)
	if err != nil {
		return err
	}

	info, err := c.api.ClientArea(hwnd)
	if restore && errors.Is(err, ErrInvalidClientArea) {
		if activateErr := c.api.Activate(hwnd); activateErr != nil {
			return fmt.Errorf("restore window hwnd=%#x: %w", hwnd, activateErr)
		}
		info, err = c.api.ClientArea(hwnd)
	}
	if err != nil {
		return err
	}
	info.PID = pid

	c.mu.Lock()
	defer c.mu.Unlock()

	c.window = info
	c.bound = true
	c.log.Info("input window bound",
		"pid", info.PID,
		"title", info.Title,
		"hwnd", fmt.Sprintf("0x%X", info.Handle),
		"client_left", info.ClientLeft,
		"client_top", info.ClientTop,
		"client_width", info.ClientWidth,
		"client_height", info.ClientHeight,
	)
	return nil
}

// Unbind clears stored window metadata. It is idempotent.
func (c *Controller) Unbind() {
	c.gameplayMu.Lock()
	defer c.gameplayMu.Unlock()
	c.releaseHeldModifierClick()
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.bound {
		return
	}

	c.log.Info("input window unbound",
		"pid", c.window.PID,
		"hwnd", fmt.Sprintf("0x%X", c.window.Handle),
	)
	c.window = WindowInfo{}
	c.bound = false
}

// Bound reports whether a D2R window is currently bound.
func (c *Controller) Bound() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.bound
}

// Window returns a copy of the bound window metadata and whether binding is active.
func (c *Controller) Window() (WindowInfo, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.bound {
		return WindowInfo{}, false
	}
	return c.window, true
}

// Focus activates the bound D2R window and verifies that it became the
// foreground window before a keyboard-sensitive workflow continues.
func (c *Controller) Focus() error {
	return c.withGameplayAction(func() error {
		if err := c.actionGuard("window", "focus", "window_focus"); err != nil {
			return err
		}
		c.mu.Lock()
		if !c.bound {
			c.mu.Unlock()
			return fmt.Errorf("focus window: %w", ErrWindowNotBound)
		}
		hwnd := nativeWindow(c.window.Handle)
		c.mu.Unlock()
		for attempt := 0; attempt < 10; attempt++ {
			// Windows may reject the first foreground request while the dashboard owns
			// the foreground lock. Retrying remains input-free; the authoritative gate
			// is still GetForegroundWindow below.
			if err := c.api.Activate(hwnd); err != nil {
				return err
			}
			if c.api.IsForeground(hwnd) {
				c.logAllowedAction("window", "focus", "window_focus", "hwnd", fmt.Sprintf("0x%X", hwnd), "verify_attempt", attempt+1)
				return nil
			}
			time.Sleep(20 * time.Millisecond)
		}
		return fmt.Errorf("focus window hwnd=%#x: %w", hwnd, ErrWindowNotForeground)
	})
}
