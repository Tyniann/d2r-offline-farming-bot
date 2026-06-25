package input

import (
	"fmt"
	"log/slog"
	"sync"
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

	mu      sync.Mutex
	keyMu   sync.Mutex
	mouseMu sync.Mutex
	window  WindowInfo
	bound   bool
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
	if pid == 0 {
		return fmt.Errorf("bind window: %w", ErrInvalidPID)
	}

	hwnd, err := c.api.FindMainWindow(pid, defaultWindowTitle)
	if err != nil {
		return err
	}

	info, err := c.api.ClientArea(hwnd)
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
