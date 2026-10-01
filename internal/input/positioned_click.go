package input

import (
	"errors"
	"fmt"
	"time"
)

// ClickAt performs one client-relative plain click under the gameplay lease.
// It refreshes geometry and rejects focus or size changes before sending input.
func (c *Controller) ClickAt(clientX, clientY int, button MouseButton) error {
	return c.positionedClick(clientX, clientY, nil, button)
}

// ClickAtWithCtrlShift moves and performs exactly one Ctrl+Shift click under
// one gameplay lease. Both modifiers and the mouse button are released on every
// partial failure. Any release failure stops subsequent gameplay input.
func (c *Controller) ClickAtWithCtrlShift(clientX, clientY int, button MouseButton) error {
	return c.positionedClick(clientX, clientY, []Key{"ctrl", "shift"}, button)
}

func (c *Controller) positionedClick(clientX, clientY int, modifiers []Key, button MouseButton) error {
	if !isValidMouseButton(button) {
		return fmt.Errorf("positioned click: %w", ErrInvalidMouseButton)
	}
	return c.withGameplayAction(func() error { return c.sendPositionedClick(clientX, clientY, modifiers, button) })
}

func (c *Controller) sendPositionedClick(clientX, clientY int, modifiers []Key, button MouseButton) (err error) {
	c.mu.Lock()
	win, bound := c.window, c.bound
	c.mu.Unlock()
	if !bound {
		return fmt.Errorf("positioned click: %w", ErrWindowNotBound)
	}
	fresh, err := c.api.ClientArea(nativeWindow(win.Handle))
	if err != nil {
		return fmt.Errorf("refresh positioned click window: %w", err)
	}
	// Fixed UI coordinates were gated against the bound size. A resize between
	// the caller's check and this lease must not turn them into clamped clicks.
	if fresh.Handle != win.Handle || fresh.ClientWidth <= 0 || fresh.ClientHeight <= 0 || fresh.ClientWidth != win.ClientWidth || fresh.ClientHeight != win.ClientHeight {
		return fmt.Errorf("positioned click geometry changed: %w", ErrInvalidClientArea)
	}
	pt := clientToScreenPoint(fresh, clientX, clientY)
	attrs := []any{"modifiers", modifiers, "button", button, "client_x", pt.clientX, "client_y", pt.clientY, "screen_x", pt.screenX, "screen_y", pt.screenY, "clamped", pt.clamped}
	if err = c.actionGuard("mouse", "positioned_click", "mouse_positioned_click", attrs...); err != nil {
		return err
	}
	defer func() {
		if err != nil {
			c.logInputAction("mouse", "positioned_click", "mouse_positioned_click", false, "transaction", err, attrs...)
		} else {
			c.logAllowedAction("mouse", "positioned_click", "mouse_positioned_click", attrs...)
		}
	}()
	if !c.api.IsForeground(nativeWindow(win.Handle)) {
		return fmt.Errorf("positioned click: %w", ErrWindowNotForeground)
	}
	if c.ModifierHoldActive() {
		return fmt.Errorf("positioned click: held gameplay input active")
	}
	c.keyMu.Lock()
	c.mouseMu.Lock()
	stopAfterRelease := false
	defer func() {
		c.mouseMu.Unlock()
		c.keyMu.Unlock()
		// Stop may release a held gameplay action and acquire these locks itself.
		if stopAfterRelease {
			c.Stop("positioned_click_release_failed")
		}
	}()
	keysAttempted := make([]Key, 0, len(modifiers))
	mouseAttempted := false
	defer func() {
		if mouseAttempted {
			if releaseErr := c.mouse.ButtonUp(button); releaseErr != nil {
				stopAfterRelease = true
				err = errors.Join(err, fmt.Errorf("positioned click mouse cleanup: %w", releaseErr))
			}
		}
		for i := len(keysAttempted) - 1; i >= 0; i-- {
			if releaseErr := c.keys.KeyUp(keysAttempted[i]); releaseErr != nil {
				stopAfterRelease = true
				err = errors.Join(err, fmt.Errorf("positioned click %s cleanup: %w", keysAttempted[i], releaseErr))
				if retryErr := c.keys.KeyUp(keysAttempted[i]); retryErr != nil {
					err = errors.Join(err, fmt.Errorf("positioned click %s cleanup retry: %w", keysAttempted[i], retryErr))
				}
			}
		}
	}()
	if err = c.mouse.MoveTo(pt.screenX, pt.screenY); err != nil {
		return fmt.Errorf("positioned click move: %w", err)
	}
	for _, key := range modifiers {
		// A failed sender may have delivered part of the OS transaction. Release
		// every attempted key, including the one whose KeyDown returned an error.
		keysAttempted = append(keysAttempted, key)
		if err = c.keys.KeyDown(key); err != nil {
			return fmt.Errorf("positioned click %s down: %w", key, err)
		}
	}
	if len(modifiers) > 0 {
		time.Sleep(modifierClickHold)
	}
	if err = c.actionGuard("mouse", "positioned_click", "mouse_positioned_click", attrs...); err != nil {
		return err
	}
	if !c.api.IsForeground(nativeWindow(win.Handle)) {
		return fmt.Errorf("positioned click before button: %w", ErrWindowNotForeground)
	}
	mouseAttempted = true
	if err = c.mouse.ButtonDown(button); err != nil {
		return fmt.Errorf("positioned click button down: %w", err)
	}
	if err = c.mouse.ButtonUp(button); err != nil {
		stopAfterRelease = true
		return fmt.Errorf("positioned click button up: %w", err)
	}
	mouseAttempted = false
	if len(modifiers) > 0 {
		time.Sleep(modifierClickHold)
	}
	return nil
}
