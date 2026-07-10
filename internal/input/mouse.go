package input

import (
	"fmt"
)

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
	c.mu.Lock()
	if !c.bound {
		c.mu.Unlock()
		return fmt.Errorf("move to: %w", ErrWindowNotBound)
	}
	win := c.window
	c.mu.Unlock()

	return c.moveTo(win, clientX, clientY, "mouse_move")
}

// Click sends a button down and up at the current cursor position without moving the mouse.
func (c *Controller) Click(button MouseButton) error {
	if !isValidMouseButton(button) {
		return fmt.Errorf("click: %w", ErrInvalidMouseButton)
	}

	c.mu.Lock()
	if !c.bound {
		c.mu.Unlock()
		return fmt.Errorf("click: %w", ErrWindowNotBound)
	}
	c.mu.Unlock()

	return c.click(button, "mouse_click")
}

// ClickWithModifier holds one keyboard modifier for a mouse click and always releases it afterward.
func (c *Controller) ClickWithModifier(modifier string, button MouseButton) error {
	key, err := NormalizeKey(modifier)
	if err != nil {
		return err
	}
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
