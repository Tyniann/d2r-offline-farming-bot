package input

import "context"

// HotkeyAction identifies which safety action a global hotkey triggers.
type HotkeyAction string

const (
	// HotkeyActionPause toggles the pause state of the input controller.
	HotkeyActionPause HotkeyAction = "pause"
	// HotkeyActionStopAfterRun requests an orderly stop at the next safe run boundary.
	HotkeyActionStopAfterRun HotkeyAction = "stop_after_run"
	// HotkeyActionRecordingFinish requests controlled F9 recording freeze.
	HotkeyActionRecordingFinish HotkeyAction = "recording_finish"
	// HotkeyActionStop terminates input and signals application shutdown.
	HotkeyActionStop HotkeyAction = "stop"
)

// HotkeyEvent is delivered when a registered global hotkey is pressed.
type HotkeyEvent struct {
	Action HotkeyAction
	Key    Key
}

// HotkeyBindings holds normalized keys for global queue-control and emergency hotkeys.
type HotkeyBindings struct {
	Pause           Key
	StopAfterRun    Key
	RecordingFinish Key
	Stop            Key
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
	c.hotkeyWG.Add(1)
	go func() {
		defer c.hotkeyWG.Done()
		listener.Listen(ctx, c.hotkeyBindings, events, ready)
	}()
}

// WaitHotkeys waits until all listeners have unregistered their global hotkeys.
func (c *Controller) WaitHotkeys() { c.hotkeyWG.Wait() }
