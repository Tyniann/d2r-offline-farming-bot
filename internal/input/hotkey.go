package input

import "context"

// HotkeyAction identifies which safety action a global hotkey triggers.
type HotkeyAction string

const (
	// HotkeyActionPause toggles the pause state of the input controller.
	HotkeyActionPause HotkeyAction = "pause"
	// HotkeyActionStop terminates input and signals application shutdown.
	HotkeyActionStop HotkeyAction = "stop"
)

// HotkeyEvent is delivered when a registered global hotkey is pressed.
type HotkeyEvent struct {
	Action HotkeyAction
	Key    Key
}

// HotkeyBindings holds normalized keys for global pause and stop hotkeys.
type HotkeyBindings struct {
	Pause Key
	Stop  Key
}

// HotkeyListener registers global hotkeys and delivers events until ctx is cancelled.
type HotkeyListener interface {
	Listen(ctx context.Context, bindings HotkeyBindings, events chan<- HotkeyEvent, ready chan<- error)
}

// ListenHotkeys starts the global hotkey listener using bindings from controller construction.
func (c *Controller) ListenHotkeys(ctx context.Context, events chan<- HotkeyEvent, ready chan<- error) {
	listener := c.hotkeyListen
	if listener == nil {
		listener = defaultHotkeyListener()
	}
	go listener.Listen(ctx, c.hotkeyBindings, events, ready)
}
