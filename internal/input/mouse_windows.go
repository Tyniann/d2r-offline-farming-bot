//go:build windows

package input

import (
	"fmt"
	"log/slog"
	"unsafe"
)

const (
	inputMouse                  = 0
	mouseEventFLeftDown  uint32 = 0x0002
	mouseEventFLeftUp    uint32 = 0x0004
	mouseEventFRightDown uint32 = 0x0008
	mouseEventFRightUp   uint32 = 0x0010
)

var (
	procSetCursorPos        = modUser32Send.NewProc("SetCursorPos")
	defaultSetCursorPosFn   = setCursorPosCall
	defaultSendMouseInputFn = sendMouseInputCall
)

type mouseInput struct {
	Dx        int32
	Dy        int32
	MouseData uint32
	Flags     uint32
	Time      uint32
	_         uint32 // align dwExtraInfo to 8 bytes (Windows MOUSEINPUT layout)
	ExtraInfo uintptr
}

// mouseInputRecord mirrors the Windows INPUT structure for mouse events (40 bytes on amd64).
type mouseInputRecord struct {
	Type uint32
	_    uint32
	Mi   mouseInput
}

type setCursorPosFunc func(x, y int) error
type sendMouseInputFunc func(inputs []mouseInputRecord) (uint32, error)

func setCursorPosCall(x, y int) error {
	ret, _, err := procSetCursorPos.Call(uintptr(x), uintptr(y))
	if ret == 0 {
		return fmt.Errorf("set cursor pos: %w", err)
	}
	return nil
}

func sendMouseInputCall(inputs []mouseInputRecord) (uint32, error) {
	if len(inputs) == 0 {
		return 0, nil
	}
	ret, _, err := procSendInput.Call(
		uintptr(len(inputs)),
		uintptr(unsafe.Pointer(&inputs[0])),
		unsafe.Sizeof(mouseInputRecord{}),
	)
	if ret == 0 {
		return 0, fmt.Errorf("send input: %w", err)
	}
	return uint32(ret), nil
}

type winMouseSender struct {
	moveCursor setCursorPosFunc
	send       sendMouseInputFunc
}

func defaultMouseSender(_ *slog.Logger) MouseSender {
	return &winMouseSender{
		moveCursor: defaultSetCursorPosFn,
		send:       defaultSendMouseInputFn,
	}
}

func (s *winMouseSender) MoveTo(screenX, screenY int) error {
	if err := s.moveCursor(screenX, screenY); err != nil {
		return fmt.Errorf("mouse move screen=(%d,%d): %w", screenX, screenY, fmt.Errorf("%w: %w", ErrMouseSendFailed, err))
	}
	return nil
}

func mouseButtonFlags(button MouseButton, release bool) (uint32, error) {
	switch button {
	case MouseLeft:
		if release {
			return mouseEventFLeftUp, nil
		}
		return mouseEventFLeftDown, nil
	case MouseRight:
		if release {
			return mouseEventFRightUp, nil
		}
		return mouseEventFRightDown, nil
	default:
		return 0, fmt.Errorf("button %q: %w", button, ErrInvalidMouseButton)
	}
}

func (s *winMouseSender) ButtonDown(button MouseButton) error {
	return s.sendButtonEvent(button, false)
}

func (s *winMouseSender) ButtonUp(button MouseButton) error {
	return s.sendButtonEvent(button, true)
}

func (s *winMouseSender) sendButtonEvent(button MouseButton, release bool) error {
	flags, err := mouseButtonFlags(button, release)
	if err != nil {
		return err
	}

	inputs := []mouseInputRecord{{
		Type: inputMouse,
		Mi: mouseInput{
			Flags: flags,
		},
	}}

	sent, err := s.send(inputs)
	if err != nil {
		return fmt.Errorf("mouse button %q release=%v: %w", button, release, fmt.Errorf("%w: %w", ErrMouseSendFailed, err))
	}
	if sent != 1 {
		return fmt.Errorf("mouse button %q release=%v sent=%d: %w", button, release, sent, ErrMouseSendFailed)
	}
	return nil
}
