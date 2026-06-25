package input

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"
	"time"
)

type mockHotkeyListener struct {
	readyErr error
	events   []HotkeyEvent
	mu       sync.Mutex
	started  chan struct{}
}

func (m *mockHotkeyListener) Listen(ctx context.Context, _ HotkeyBindings, events chan<- HotkeyEvent, ready chan<- error) {
	if m.started != nil {
		close(m.started)
	}
	ready <- m.readyErr
	if m.readyErr != nil {
		return
	}
	m.mu.Lock()
	pending := append([]HotkeyEvent(nil), m.events...)
	m.mu.Unlock()
	for _, ev := range pending {
		select {
		case events <- ev:
		case <-ctx.Done():
			return
		}
	}
	<-ctx.Done()
}

func TestListenHotkeysReadyError(t *testing.T) {
	listener := &mockHotkeyListener{readyErr: ErrHotkeyUnavailable}
	c, err := newWithBackends(
		slogDefault(),
		&mockWindowAPI{},
		&mockKeySender{},
		&mockMouseSender{},
		DefaultKeyboardConfig(),
		testSafetyDisabled(),
		testKeyTimings(),
		listener,
	)
	if err != nil {
		t.Fatal(err)
	}

	ready := make(chan error, 1)
	c.ListenHotkeys(context.Background(), make(chan HotkeyEvent, 1), ready)

	if err := <-ready; !errors.Is(err, ErrHotkeyUnavailable) {
		t.Fatalf("ready err = %v, want ErrHotkeyUnavailable", err)
	}
}

func TestListenHotkeysDeliversEvents(t *testing.T) {
	listener := &mockHotkeyListener{
		events: []HotkeyEvent{{Action: HotkeyActionStop, Key: "f12"}},
	}
	c, err := newWithBackends(
		slogDefault(),
		&mockWindowAPI{},
		&mockKeySender{},
		&mockMouseSender{},
		DefaultKeyboardConfig(),
		testSafetyDisabled(),
		testKeyTimings(),
		listener,
	)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	events := make(chan HotkeyEvent, 4)
	ready := make(chan error, 1)
	c.ListenHotkeys(ctx, events, ready)

	if err := <-ready; err != nil {
		t.Fatalf("ready err = %v", err)
	}

	select {
	case ev := <-events:
		if ev.Action != HotkeyActionStop || ev.Key != "f12" {
			t.Fatalf("event = %+v, want stop/f12", ev)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for hotkey event")
	}
}

func slogDefault() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}
