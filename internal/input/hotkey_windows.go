//go:build windows

package input

import (
	"context"
	"fmt"
	"time"
	"unsafe"
)

const (
	hotkeyIDPause = 1
	hotkeyIDStop  = 2

	wmHotkey   = 0x0312
	pmRemove   = 0x0001
	modNoMod   = 0x0000
	hotkeyPoll = 10 * time.Millisecond
)

var (
	procPeekMessageW     = modUser32.NewProc("PeekMessageW")
	procRegisterHotKey   = modUser32.NewProc("RegisterHotKey")
	procUnregisterHotKey = modUser32.NewProc("UnregisterHotKey")
)

// winMsg mirrors the Windows MSG layout used by PeekMessageW on amd64.
type winMsg struct {
	Hwnd    uintptr
	Message uint32
	_       uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      struct {
		X, Y int32
	}
}

type hotkeyRegisterFunc func(id int, vk uint16) error
type hotkeyUnregisterFunc func(id int) error
type hotkeyPeekFunc func() (id int, ok bool)

type winHotkeyListener struct {
	register   hotkeyRegisterFunc
	unregister hotkeyUnregisterFunc
	peek       hotkeyPeekFunc
	sleep      func(time.Duration)
}

func defaultHotkeyListener() HotkeyListener {
	return &winHotkeyListener{
		register:   registerGlobalHotkey,
		unregister: unregisterGlobalHotkey,
		peek:       peekHotkeyMessage,
		sleep:      time.Sleep,
	}
}

func (l *winHotkeyListener) Listen(ctx context.Context, bindings HotkeyBindings, events chan<- HotkeyEvent, ready chan<- error) {
	registered := make([]int, 0, 2)

	cleanup := func() {
		for i := len(registered) - 1; i >= 0; i-- {
			_ = l.unregister(registered[i])
		}
	}

	pauseVK, ok := virtualKey(bindings.Pause)
	if !ok {
		ready <- fmt.Errorf("register pause hotkey: %w", ErrHotkeyUnavailable)
		return
	}
	if err := l.register(hotkeyIDPause, pauseVK); err != nil {
		ready <- fmt.Errorf("register pause hotkey: %w", err)
		return
	}
	registered = append(registered, hotkeyIDPause)

	stopVK, ok := virtualKey(bindings.Stop)
	if !ok {
		cleanup()
		ready <- fmt.Errorf("register stop hotkey: %w", ErrHotkeyUnavailable)
		return
	}
	if err := l.register(hotkeyIDStop, stopVK); err != nil {
		cleanup()
		ready <- fmt.Errorf("register stop hotkey: %w", err)
		return
	}
	registered = append(registered, hotkeyIDStop)

	defer cleanup()
	ready <- nil

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		if id, ok := l.peek(); ok {
			switch id {
			case hotkeyIDPause:
				events <- HotkeyEvent{Action: HotkeyActionPause, Key: bindings.Pause}
			case hotkeyIDStop:
				events <- HotkeyEvent{Action: HotkeyActionStop, Key: bindings.Stop}
			}
		}
		l.sleep(hotkeyPoll)
	}
}

func registerGlobalHotkey(id int, vk uint16) error {
	ret, _, err := procRegisterHotKey.Call(
		0,
		uintptr(id),
		uintptr(modNoMod),
		uintptr(vk),
	)
	if ret == 0 {
		return fmt.Errorf("%w: %v", ErrHotkeyUnavailable, err)
	}
	return nil
}

func unregisterGlobalHotkey(id int) error {
	ret, _, err := procUnregisterHotKey.Call(0, uintptr(id))
	if ret == 0 {
		return err
	}
	return nil
}

func peekHotkeyMessage() (int, bool) {
	var msg winMsg
	ret, _, _ := procPeekMessageW.Call(
		uintptr(unsafe.Pointer(&msg)),
		0,
		wmHotkey,
		wmHotkey,
		pmRemove,
	)
	if ret == 0 {
		return 0, false
	}
	if msg.Message != wmHotkey {
		return 0, false
	}
	return int(msg.WParam), true
}
