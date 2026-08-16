package input

import (
	"fmt"
	"time"
)

const modifierClickHold = 40 * time.Millisecond

const defaultMouseEdgeMargin = 10

// MouseButton identifies a mouse button for click primitives.
type MouseButton string

const (
	// MouseLeft is the primary mouse button.
	MouseLeft MouseButton = "left"
	// MouseRight is the secondary mouse button.
	MouseRight MouseButton = "right"
)

// MouseSender sends low-level mouse move and button events to the OS.
type MouseSender interface {
	MoveTo(screenX, screenY int) error
	ButtonDown(button MouseButton) error
	ButtonUp(button MouseButton) error
}

type mousePoint struct {
	clientX int
	clientY int
	screenX int
	screenY int
	clamped bool
}

func isValidMouseButton(button MouseButton) bool {
	return button == MouseLeft || button == MouseRight
}

func clampMouseCoord(value, clientSize int) (clamped int, wasClamped bool) {
	margin := defaultMouseEdgeMargin
	minBound := margin
	maxBound := clientSize - 1 - margin

	if clientSize < 2*margin+1 {
		center := (clientSize - 1) / 2
		if value != center {
			return center, true
		}
		return center, false
	}

	if value < minBound {
		return minBound, true
	}
	if value > maxBound {
		return maxBound, true
	}
	return value, false
}

func clientToScreenPoint(win WindowInfo, clientX, clientY int) mousePoint {
	clampedX, clampX := clampMouseCoord(clientX, win.ClientWidth)
	clampedY, clampY := clampMouseCoord(clientY, win.ClientHeight)
	return mousePoint{
		clientX: clampedX,
		clientY: clampedY,
		screenX: win.ClientLeft + clampedX,
		screenY: win.ClientTop + clampedY,
		clamped: clampX || clampY,
	}
}

// MoveTo moves the mouse to client-relative coordinates within the bound window.
func (c *Controller) MoveTo(clientX, clientY int) error {
	return c.withGameplayAction(func() error {
		c.mu.Lock()
		if !c.bound {
			c.mu.Unlock()
			return fmt.Errorf("move to: %w", ErrWindowNotBound)
		}
		win := c.window
		c.mu.Unlock()
		return c.moveTo(win, clientX, clientY, "mouse_move")
	})
}

// Click sends a button down and up at the current cursor position without moving the mouse.
func (c *Controller) Click(button MouseButton) error {
	if !isValidMouseButton(button) {
		return fmt.Errorf("click: %w", ErrInvalidMouseButton)
	}

	return c.withGameplayAction(func() error {
		c.mu.Lock()
		if !c.bound {
			c.mu.Unlock()
			return fmt.Errorf("click: %w", ErrWindowNotBound)
		}
		c.mu.Unlock()
		return c.click(button, "mouse_click")
	})
}

// ClickWithModifier holds one keyboard modifier for a mouse click and always releases it afterward.
func (c *Controller) ClickWithModifier(modifier string, button MouseButton) error {
	key, err := NormalizeKey(modifier)
	if err != nil {
		return err
	}
	return c.withGameplayAction(func() error { return c.clickWithModifier(key, button) })
}

// ClickAtWithModifier moves to a client-relative point and performs exactly
// one modified click under the same gameplay lease. It requires the bound D2R
// window to still be foreground after the lease is acquired.
func (c *Controller) ClickAtWithModifier(clientX, clientY int, modifier string, button MouseButton) error {
	key, err := NormalizeKey(modifier)
	if err != nil {
		return err
	}
	if !isValidMouseButton(button) {
		return fmt.Errorf("modified click at: %w", ErrInvalidMouseButton)
	}
	return c.withGameplayAction(func() error {
		return c.clickAtWithModifier(clientX, clientY, key, button)
	})
}

func (c *Controller) clickAtWithModifier(clientX, clientY int, key Key, button MouseButton) error {
	c.mu.Lock()
	if !c.bound {
		c.mu.Unlock()
		return fmt.Errorf("modified click at: %w", ErrWindowNotBound)
	}
	win := c.window
	c.mu.Unlock()
	fresh, err := c.api.ClientArea(nativeWindow(win.Handle))
	if err != nil {
		return fmt.Errorf("refresh modified click window: %w", err)
	}
	if fresh.Handle != win.Handle || fresh.ClientWidth <= 0 || fresh.ClientHeight <= 0 {
		return fmt.Errorf("refresh modified click window: %w", ErrInvalidClientArea)
	}
	fresh.PID = win.PID
	win = fresh
	pt := clientToScreenPoint(win, clientX, clientY)
	if err := c.actionGuard("mouse", "modified_click_at", "mouse_modified_click_at",
		"modifier", key, "button", button,
		"client_x", pt.clientX, "client_y", pt.clientY,
		"screen_x", pt.screenX, "screen_y", pt.screenY, "clamped", pt.clamped,
	); err != nil {
		return err
	}
	if !c.api.IsForeground(nativeWindow(win.Handle)) {
		err := fmt.Errorf("modified click at: %w", ErrWindowNotForeground)
		c.logInputAction("mouse", "modified_click_at", "mouse_modified_click_at", false, "foreground", err,
			"modifier", key, "button", button, "client_x", pt.clientX, "client_y", pt.clientY)
		return err
	}

	c.keyMu.Lock()
	defer c.keyMu.Unlock()
	c.mouseMu.Lock()
	defer c.mouseMu.Unlock()

	if err := c.mouse.MoveTo(pt.screenX, pt.screenY); err != nil {
		return err
	}
	if err := c.keys.KeyDown(key); err != nil {
		return err
	}
	time.Sleep(modifierClickHold)
	keyHeld := true
	releaseKey := func() error {
		if !keyHeld {
			return nil
		}
		if err := c.keys.KeyUp(key); err != nil {
			return err
		}
		keyHeld = false
		return nil
	}
	cleanupKey := func() {
		if err := releaseKey(); err != nil {
			c.log.Warn("input modifier cleanup failed", "key", key, "error", err)
			c.Stop("modifier_cleanup_failed")
		}
	}

	if err := c.mouse.ButtonDown(button); err != nil {
		c.releaseMouseButton(button)
		cleanupKey()
		return err
	}
	if err := c.mouse.ButtonUp(button); err != nil {
		c.releaseMouseButton(button)
		cleanupKey()
		return err
	}
	time.Sleep(modifierClickHold)
	if err := releaseKey(); err != nil {
		cleanupKey()
		return err
	}
	c.logAllowedAction("mouse", "modified_click_at", "mouse_modified_click_at",
		"modifier", key, "button", button,
		"client_x", pt.clientX, "client_y", pt.clientY,
		"screen_x", pt.screenX, "screen_y", pt.screenY, "clamped", pt.clamped,
	)
	return nil
}

func (c *Controller) clickWithModifier(key Key, button MouseButton) error {
	if !isValidMouseButton(button) {
		return fmt.Errorf("modified click: %w", ErrInvalidMouseButton)
	}
	c.mu.Lock()
	bound := c.bound
	c.mu.Unlock()
	if !bound {
		return fmt.Errorf("modified click: %w", ErrWindowNotBound)
	}
	if err := c.actionGuard("mouse", "modified_click", "mouse_modified_click", "modifier", key, "button", button); err != nil {
		return err
	}

	c.keyMu.Lock()
	defer c.keyMu.Unlock()
	c.mouseMu.Lock()
	defer c.mouseMu.Unlock()

	if err := c.keys.KeyDown(key); err != nil {
		return err
	}
	keyHeld := true
	defer func() {
		if keyHeld {
			if releaseErr := c.keys.KeyUp(key); releaseErr != nil {
				c.log.Warn("input modifier cleanup failed", "key", key, "error", releaseErr)
			}
		}
	}()
	if err := c.mouse.ButtonDown(button); err != nil {
		return err
	}
	if err := c.mouse.ButtonUp(button); err != nil {
		c.releaseMouseButton(button)
		return err
	}
	if err := c.keys.KeyUp(key); err != nil {
		return err
	}
	keyHeld = false
	c.logAllowedAction("mouse", "modified_click", "mouse_modified_click", "modifier", key, "button", button)
	return nil
}

func (c *Controller) moveTo(win WindowInfo, clientX, clientY int, reason string) error {
	pt := clientToScreenPoint(win, clientX, clientY)

	if err := c.actionGuard("mouse", "move", reason,
		"client_x", pt.clientX,
		"client_y", pt.clientY,
		"screen_x", pt.screenX,
		"screen_y", pt.screenY,
		"clamped", pt.clamped,
	); err != nil {
		return err
	}

	c.mouseMu.Lock()
	defer c.mouseMu.Unlock()

	if err := c.mouse.MoveTo(pt.screenX, pt.screenY); err != nil {
		return err
	}

	c.logAllowedAction("mouse", "move", reason,
		"client_x", pt.clientX,
		"client_y", pt.clientY,
		"screen_x", pt.screenX,
		"screen_y", pt.screenY,
		"clamped", pt.clamped,
	)
	return nil
}

func (c *Controller) click(button MouseButton, reason string) error {
	if err := c.actionGuard("mouse", "click", reason, "button", button); err != nil {
		return err
	}

	c.mouseMu.Lock()
	defer c.mouseMu.Unlock()

	if err := c.mouse.ButtonDown(button); err != nil {
		return err
	}
	if err := c.mouse.ButtonUp(button); err != nil {
		c.releaseMouseButton(button)
		return err
	}

	c.logAllowedAction("mouse", "click", reason, "button", button)
	return nil
}

func (c *Controller) releaseMouseButton(button MouseButton) {
	if err := c.mouse.ButtonUp(button); err != nil {
		c.log.Warn("input mouse cleanup failed", "button", button, "error", err)
	}
}

// HoldAt moves to a client-relative point, presses the button, and leaves it
// down until [Controller.ReleaseModifierHold], Pause, Stop, or Unbind.
func (c *Controller) HoldAt(clientX, clientY int, button MouseButton) error {
	if !isValidMouseButton(button) {
		return fmt.Errorf("hold at: %w", ErrInvalidMouseButton)
	}
	return c.withGameplayAction(func() error {
		return c.holdAt(clientX, clientY, "", button)
	})
}

// HoldAtWithModifier moves to a client-relative point, presses modifier then
// button, and leaves both down until [Controller.ReleaseModifierHold], Pause,
// Stop, or Unbind.
func (c *Controller) HoldAtWithModifier(clientX, clientY int, modifier string, button MouseButton) error {
	key, err := NormalizeKey(modifier)
	if err != nil {
		return err
	}
	if !isValidMouseButton(button) {
		return fmt.Errorf("modified hold at: %w", ErrInvalidMouseButton)
	}
	return c.withGameplayAction(func() error {
		return c.holdAt(clientX, clientY, key, button)
	})
}

// ReleaseModifierHold raises a previously held modifier click. It is a no-op
// when nothing is held.
func (c *Controller) ReleaseModifierHold() error {
	return c.withGameplayAction(func() error {
		if err := c.actionGuard("mouse", "modified_hold_release", "mouse_modified_hold_release"); err != nil {
			c.releaseHeldModifierClick()
			if c.Status().Stopped || c.Status().Paused {
				return nil
			}
			return err
		}
		c.releaseHeldModifierClick()
		c.logAllowedAction("mouse", "modified_hold_release", "mouse_modified_hold_release")
		return nil
	})
}

// ModifierHoldActive reports whether a modifier click is currently held down.
func (c *Controller) ModifierHoldActive() bool {
	c.heldMu.Lock()
	defer c.heldMu.Unlock()
	return c.heldActive
}

func (c *Controller) holdAt(clientX, clientY int, key Key, button MouseButton) error {
	label := "hold at"
	action := "hold_at"
	reason := "mouse_hold_at"
	if key != "" {
		label = "modified hold at"
		action = "modified_hold_at"
		reason = "mouse_modified_hold_at"
	}
	c.heldMu.Lock()
	alreadyHeld := c.heldActive
	c.heldMu.Unlock()
	if alreadyHeld {
		return nil
	}
	c.mu.Lock()
	if !c.bound {
		c.mu.Unlock()
		return fmt.Errorf("%s: %w", label, ErrWindowNotBound)
	}
	win := c.window
	c.mu.Unlock()
	fresh, err := c.api.ClientArea(nativeWindow(win.Handle))
	if err != nil {
		return fmt.Errorf("refresh modified hold window: %w", err)
	}
	if fresh.Handle != win.Handle || fresh.ClientWidth <= 0 || fresh.ClientHeight <= 0 {
		return fmt.Errorf("refresh modified hold window: %w", ErrInvalidClientArea)
	}
	fresh.PID = win.PID
	win = fresh
	pt := clientToScreenPoint(win, clientX, clientY)
	guardAttrs := []any{"button", button, "client_x", pt.clientX, "client_y", pt.clientY, "screen_x", pt.screenX, "screen_y", pt.screenY, "clamped", pt.clamped}
	if key != "" {
		guardAttrs = append([]any{"modifier", key}, guardAttrs...)
	}
	if err := c.actionGuard("mouse", action, reason, guardAttrs...); err != nil {
		return err
	}
	if !c.api.IsForeground(nativeWindow(win.Handle)) {
		err := fmt.Errorf("%s: %w", label, ErrWindowNotForeground)
		c.logInputAction("mouse", action, reason, false, "foreground", err, guardAttrs...)
		return err
	}

	c.keyMu.Lock()
	c.mouseMu.Lock()
	if err := c.mouse.MoveTo(pt.screenX, pt.screenY); err != nil {
		c.mouseMu.Unlock()
		c.keyMu.Unlock()
		return err
	}
	if key != "" {
		if err := c.keys.KeyDown(key); err != nil {
			c.mouseMu.Unlock()
			c.keyMu.Unlock()
			return err
		}
		time.Sleep(modifierClickHold)
	}
	if err := c.mouse.ButtonDown(button); err != nil {
		if key != "" {
			if upErr := c.keys.KeyUp(key); upErr != nil {
				c.log.Warn("input modifier cleanup failed", "key", key, "error", upErr)
				c.Stop("modifier_cleanup_failed")
			}
		}
		c.mouseMu.Unlock()
		c.keyMu.Unlock()
		return err
	}
	c.mouseMu.Unlock()
	c.keyMu.Unlock()

	c.heldMu.Lock()
	c.heldActive = true
	c.heldKey = key
	c.heldButton = button
	c.heldMu.Unlock()

	if status := c.Status(); status.Paused || status.Stopped {
		c.releaseHeldModifierClick()
		if status.Stopped {
			return fmt.Errorf("%s: %w", label, ErrInputStopped)
		}
		return fmt.Errorf("%s: %w", label, ErrInputPaused)
	}
	c.logAllowedAction("mouse", action, reason, guardAttrs...)
	return nil
}

func (c *Controller) releaseHeldModifierClick() {
	c.heldMu.Lock()
	defer c.heldMu.Unlock()
	if !c.heldActive {
		return
	}
	c.keyMu.Lock()
	c.mouseMu.Lock()
	if err := c.mouse.ButtonUp(c.heldButton); err != nil {
		c.log.Warn("input mouse cleanup failed", "button", c.heldButton, "error", err)
	}
	if c.heldKey != "" {
		if err := c.keys.KeyUp(c.heldKey); err != nil {
			c.log.Warn("input modifier cleanup failed", "key", c.heldKey, "error", err)
		}
	}
	c.mouseMu.Unlock()
	c.keyMu.Unlock()
	c.heldActive = false
	c.heldKey = ""
	c.heldButton = ""
}
