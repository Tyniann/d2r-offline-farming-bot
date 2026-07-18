//go:build windows

package input

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"
)

func TestVirtualKeyPauseAndF12(t *testing.T) {
	cases := map[string]uint16{
		"pause": 0x13,
		"f12":   0x7B,
	}
	for name, want := range cases {
		vk, ok := virtualKey(Key(name))
		if !ok || vk != want {
			t.Fatalf("virtualKey(%q) = 0x%02X ok=%v, want 0x%02X", name, vk, ok, want)
		}
	}
}

func TestWinHotkeyListenerPartialRegistrationCleanup(t *testing.T) {
	var registered []int
	var unregistered []int

	listener := &winHotkeyListener{
		register: func(id int, vk uint16) error {
			if id == hotkeyIDStop {
				return fmt.Errorf("%w: taken", ErrHotkeyUnavailable)
			}
			registered = append(registered, id)
			return nil
		},
		unregister: func(id int) error {
			unregistered = append(unregistered, id)
			return nil
		},
		peek:  func() (int, bool) { return 0, false },
		sleep: func(_ time.Duration) {},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	events := make(chan HotkeyEvent, 1)
	ready := make(chan error, 1)
	go listener.Listen(ctx, HotkeyBindings{Pause: "pause", RecordingFinish: "f9", StopAfterRun: "f10", Stop: "f12"}, events, ready)

	err := <-ready
	if !errors.Is(err, ErrHotkeyUnavailable) {
		t.Fatalf("ready err = %v, want ErrHotkeyUnavailable", err)
	}
	if !reflect.DeepEqual(registered, []int{hotkeyIDPause, hotkeyIDStopAfterRun, hotkeyIDRecordingFinish}) {
		t.Fatalf("registered = %v, want [pause stop-after-run recording-finish]", registered)
	}
	if !reflect.DeepEqual(unregistered, []int{hotkeyIDRecordingFinish, hotkeyIDStopAfterRun, hotkeyIDPause}) {
		t.Fatalf("unregistered = %v, want reverse registration order", unregistered)
	}
}

func TestWinHotkeyListenerDispatchesStopAfterRun(t *testing.T) {
	peekCalls := 0
	listener := &winHotkeyListener{
		register:   func(int, uint16) error { return nil },
		unregister: func(int) error { return nil },
		peek: func() (int, bool) {
			peekCalls++
			if peekCalls == 1 {
				return hotkeyIDStopAfterRun, true
			}
			return 0, false
		},
		sleep: func(_ time.Duration) {},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	events := make(chan HotkeyEvent, 4)
	ready := make(chan error, 1)
	go listener.Listen(ctx, HotkeyBindings{Pause: "pause", RecordingFinish: "f9", StopAfterRun: "f10", Stop: "f12"}, events, ready)

	if err := <-ready; err != nil {
		t.Fatal(err)
	}

	select {
	case ev := <-events:
		if ev.Action != HotkeyActionStopAfterRun || ev.Key != "f10" {
			t.Fatalf("event = %+v, want stop-after-run/f10 action", ev)
		}
	default:
		t.Fatal("expected pause hotkey event")
	}
}
