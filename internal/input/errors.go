package input

import "errors"

var (
	// ErrInvalidPID is returned when Bind is called with PID 0.
	ErrInvalidPID = errors.New("invalid pid")
	// ErrWindowNotFound is returned when no visible top-level window matches the target PID and title.
	ErrWindowNotFound = errors.New("window not found")
	// ErrInvalidClientArea is returned when the client rectangle is empty or cannot be measured.
	ErrInvalidClientArea = errors.New("invalid client area")
	// ErrUnsupportedPlatform is returned on non-Windows builds where User32 window discovery is unavailable.
	ErrUnsupportedPlatform = errors.New("unsupported platform")
	// ErrInvalidKey is returned when a key string is empty or not in the supported key table.
	ErrInvalidKey = errors.New("invalid key")
	// ErrUnconfiguredSlot is returned when a configured action has no key mapping.
	ErrUnconfiguredSlot = errors.New("unconfigured slot")
	// ErrInvalidSlot is returned when a skill or belt slot index is out of range.
	ErrInvalidSlot = errors.New("invalid slot")
	// ErrKeySendFailed is returned when the OS keyboard backend rejects a send operation.
	ErrKeySendFailed = errors.New("key send failed")
	// ErrWindowNotBound is returned when a mouse action requires a bound D2R window.
	ErrWindowNotBound = errors.New("window not bound")
	// ErrInvalidMouseButton is returned when a mouse button identifier is not left or right.
	ErrInvalidMouseButton = errors.New("invalid mouse button")
	// ErrMouseSendFailed is returned when the OS mouse backend rejects a send operation.
	ErrMouseSendFailed = errors.New("mouse send failed")
	// ErrInputDisabled is returned when input actions are blocked because enabled=false.
	ErrInputDisabled = errors.New("input disabled")
	// ErrInputPaused is returned when input actions are blocked because the controller is paused.
	ErrInputPaused = errors.New("input paused")
	// ErrInputStopped is returned when input actions are blocked because Stop was called.
	ErrInputStopped = errors.New("input stopped")
	// ErrHotkeyUnavailable is returned when global hotkey registration fails.
	ErrHotkeyUnavailable = errors.New("hotkey unavailable")
)

// IsBindRetryable reports whether a Bind failure should be retried without aborting the run loop.
// Window-not-found and invalid-client-area errors are transient during D2R startup or minimization.
func IsBindRetryable(err error) bool {
	return errors.Is(err, ErrWindowNotFound) || errors.Is(err, ErrInvalidClientArea)
}
