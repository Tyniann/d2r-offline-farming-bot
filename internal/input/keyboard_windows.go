//go:build windows

package input

import (
	"fmt"
	"log/slog"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	inputKeyboard           = 1
	keyEventFKeyUp   uint32 = 0x0002
	keyEventFUnicode uint32 = 0x0004
)

var (
	modUser32Send      = windows.NewLazySystemDLL("user32.dll")
	procSendInput      = modUser32Send.NewProc("SendInput")
	defaultSendInputFn = sendInputCall
)

type keybdInput struct {
	Vk        uint16
	Scan      uint16
	Flags     uint32
	Time      uint32
	_         uint32 // align dwExtraInfo to 8 bytes (Windows KEYBDINPUT layout)
	ExtraInfo uintptr
}

// inputRecord mirrors the Windows INPUT structure for keyboard events (40 bytes on amd64).
type inputRecord struct {
	Type uint32
	_    uint32
	Ki   keybdInput
	_    [8]byte // pad union to MOUSEINPUT size
}

type sendInputFunc func(inputs []inputRecord) (uint32, error)

func sendInputCall(inputs []inputRecord) (uint32, error) {
	if len(inputs) == 0 {
		return 0, nil
	}
	ret, _, err := procSendInput.Call(
		uintptr(len(inputs)),
		uintptr(unsafe.Pointer(&inputs[0])),
		unsafe.Sizeof(inputRecord{}),
	)
	if ret == 0 {
		return 0, fmt.Errorf("send input: %w", err)
	}
	return uint32(ret), nil
}

type winKeySender struct {
	send sendInputFunc
}

func defaultKeySender(_ *slog.Logger) KeySender {
	return &winKeySender{send: defaultSendInputFn}
}

func (s *winKeySender) KeyDown(key Key) error {
	return s.sendKeyEvent(key, false)
}

func (s *winKeySender) KeyUp(key Key) error {
	return s.sendKeyEvent(key, true)
}

func (s *winKeySender) TypeRune(r rune) error {
	if r < 0 || r > 0xFFFF {
		return fmt.Errorf("rune %U: %w", r, ErrInvalidKey)
	}
	scan := uint16(r)
	inputs := []inputRecord{
		{Type: inputKeyboard, Ki: keybdInput{Scan: scan, Flags: keyEventFUnicode}},
		{Type: inputKeyboard, Ki: keybdInput{Scan: scan, Flags: keyEventFUnicode | keyEventFKeyUp}},
	}
	sent, err := s.send(inputs)
	if err != nil {
		return fmt.Errorf("rune %U: %w", r, fmt.Errorf("%w: %w", ErrKeySendFailed, err))
	}
	if sent != 2 {
		return fmt.Errorf("rune %U sent=%d: %w", r, sent, ErrKeySendFailed)
	}
	return nil
}

func (s *winKeySender) sendKeyEvent(key Key, release bool) error {
	vk, ok := virtualKey(key)
	if !ok {
		return fmt.Errorf("key %q: %w", key, ErrInvalidKey)
	}

	flags := uint32(0)
	if release {
		flags = keyEventFKeyUp
	}

	inputs := []inputRecord{{
		Type: inputKeyboard,
		Ki: keybdInput{
			Vk:    vk,
			Flags: flags,
		},
	}}

	sent, err := s.send(inputs)
	if err != nil {
		return fmt.Errorf("key %q vk=0x%02X release=%v: %w", key, vk, release, fmt.Errorf("%w: %w", ErrKeySendFailed, err))
	}
	if sent != 1 {
		return fmt.Errorf("key %q vk=0x%02X release=%v sent=%d: %w", key, vk, release, sent, ErrKeySendFailed)
	}
	return nil
}
