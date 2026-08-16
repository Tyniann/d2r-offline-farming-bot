package input

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"
)

type captureLogHandler struct {
	mu      sync.Mutex
	records []slog.Record
}

func (h *captureLogHandler) Enabled(context.Context, slog.Level) bool { return true }

func (h *captureLogHandler) Handle(ctx context.Context, r slog.Record) error {
	h.mu.Lock()
	h.records = append(h.records, r.Clone())
	h.mu.Unlock()
	return nil
}

func (h *captureLogHandler) WithAttrs(attrs []slog.Attr) slog.Handler { return h }

func (h *captureLogHandler) WithGroup(string) slog.Handler { return h }

func (h *captureLogHandler) findActionLog(allowed bool, blockedBy string) (slog.Record, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, r := range h.records {
		if r.Message != "input action" {
			continue
		}
		gotAllowed := false
		gotBlockedBy := ""
		r.Attrs(func(a slog.Attr) bool {
			switch a.Key {
			case "allowed":
				gotAllowed = a.Value.Bool()
			case "blocked_by":
				gotBlockedBy = a.Value.String()
			}
			return true
		})
		if gotAllowed == allowed && (blockedBy == "" || gotBlockedBy == blockedBy) {
			return r, true
		}
	}
	return slog.Record{}, false
}

func testKeyboardControllerWithLog(log *slog.Logger, keys KeySender, kb KeyboardConfig) *Controller {
	c, err := newWithBackends(log, &mockWindowAPI{}, keys, &mockMouseSender{}, kb, testSafetyEnabled(), testKeyTimings(), nil)
	if err != nil {
		panic(err)
	}
	return c
}

func TestActionLoggingAllowed(t *testing.T) {
	handler := &captureLogHandler{}
	log := slog.New(handler)
	mock := &mockKeySender{}
	c := testKeyboardControllerWithLog(log, mock, DefaultKeyboardConfig())

	if err := c.PressKey("f1"); err != nil {
		t.Fatal(err)
	}
	if _, ok := handler.findActionLog(true, ""); !ok {
		t.Fatal("expected input action log with allowed=true")
	}
}

func TestActionLoggingBlocked(t *testing.T) {
	handler := &captureLogHandler{}
	log := slog.New(handler)
	mock := &mockKeySender{}
	c := testKeyboardControllerWithLog(log, mock, DefaultKeyboardConfig())
	c.SetEnabled(false)

	_ = c.PressKey("f1")
	if _, ok := handler.findActionLog(false, "disabled"); !ok {
		t.Fatal("expected input action log with allowed=false blocked_by=disabled")
	}
}

func TestResumeClearsPause(t *testing.T) {
	c := testController(&mockWindowAPI{})
	c.Pause("test")
	c.Resume("test")
	if c.Status().Paused {
		t.Fatal("expected not paused after Resume")
	}
}

func TestPausedBlocksMouse(t *testing.T) {
	mock := &mockMouseSender{}
	c := boundMouseController(mock)
	c.Pause("test")

	err := c.Click(MouseLeft)
	if !errors.Is(err, ErrInputPaused) {
		t.Fatalf("Click err = %v, want ErrInputPaused", err)
	}
	if len(mock.downCalls) != 0 {
		t.Fatal("sender should not be called when paused")
	}
}

func TestStoppedBlocksMouse(t *testing.T) {
	mock := &mockMouseSender{}
	c := boundMouseController(mock)
	c.Stop("test")

	err := c.Click(MouseLeft)
	if !errors.Is(err, ErrInputStopped) {
		t.Fatalf("Click err = %v, want ErrInputStopped", err)
	}
	if len(mock.downCalls) != 0 {
		t.Fatal("sender should not be called when stopped")
	}
}

func TestInvalidMouseButtonBeforeGuard(t *testing.T) {
	mock := &mockMouseSender{}
	c := boundMouseController(mock)
	c.SetEnabled(false)

	err := c.Click(MouseButton("middle"))
	if !errors.Is(err, ErrInvalidMouseButton) {
		t.Fatalf("Click err = %v, want ErrInvalidMouseButton", err)
	}
	if len(mock.downCalls) != 0 {
		t.Fatal("sender should not be called for invalid button")
	}
}

func TestDefaultSafetyState(t *testing.T) {
	c := testController(&mockWindowAPI{})
	st := c.Status()
	if st.Enabled || st.Paused || st.Stopped {
		t.Fatalf("default status = %+v, want disabled/not paused/not stopped", st)
	}
}

func TestSetEnabledUpdatesStatus(t *testing.T) {
	c := testController(&mockWindowAPI{})
	c.SetEnabled(true)
	if !c.Status().Enabled {
		t.Fatal("expected enabled after SetEnabled(true)")
	}
}

func TestEnabledActionReachesSender(t *testing.T) {
	mock := &mockKeySender{}
	c := testKeyboardController(mock, DefaultKeyboardConfig())

	if err := c.PressKey("f1"); err != nil {
		t.Fatal(err)
	}
	if len(mock.downCalls) != 1 || len(mock.upCalls) != 1 {
		t.Fatalf("down/up calls = %d/%d, want 1/1", len(mock.downCalls), len(mock.upCalls))
	}
}

func TestDisabledBlocksKeyboard(t *testing.T) {
	mock := &mockKeySender{}
	c := testKeyboardController(mock, DefaultKeyboardConfig())
	c.SetEnabled(false)

	err := c.PressKey("f1")
	if !errors.Is(err, ErrInputDisabled) {
		t.Fatalf("PressKey err = %v, want ErrInputDisabled", err)
	}
	if len(mock.downCalls) != 0 {
		t.Fatal("sender should not be called when disabled")
	}
}

func TestPausedBlocksKeyboard(t *testing.T) {
	mock := &mockKeySender{}
	c := testKeyboardController(mock, DefaultKeyboardConfig())
	c.Pause("test")

	err := c.PressKey("f1")
	if !errors.Is(err, ErrInputPaused) {
		t.Fatalf("PressKey err = %v, want ErrInputPaused", err)
	}
	if len(mock.downCalls) != 0 {
		t.Fatal("sender should not be called when paused")
	}
}

func TestStoppedBlocksKeyboard(t *testing.T) {
	mock := &mockKeySender{}
	c := testKeyboardController(mock, DefaultKeyboardConfig())
	c.Stop("test")

	err := c.PressKey("f1")
	if !errors.Is(err, ErrInputStopped) {
		t.Fatalf("PressKey err = %v, want ErrInputStopped", err)
	}
	if len(mock.downCalls) != 0 {
		t.Fatal("sender should not be called when stopped")
	}
}

func TestStoppedPriorityOverDisabled(t *testing.T) {
	c := testKeyboardController(&mockKeySender{}, DefaultKeyboardConfig())
	c.SetEnabled(false)
	c.Stop("test")

	err := c.PressKey("f1")
	if !errors.Is(err, ErrInputStopped) {
		t.Fatalf("PressKey err = %v, want ErrInputStopped before ErrInputDisabled", err)
	}
}

func TestGameplayActionRechecksStopAfterWaitingForLease(t *testing.T) {
	keys := &mockKeySender{}
	c := testKeyboardController(keys, DefaultKeyboardConfig())
	c.gameplayMu.Lock()
	started := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		close(started)
		done <- c.PressKey("f1")
	}()
	<-started
	c.Stop("queued_action_test")
	c.gameplayMu.Unlock()
	if err := <-done; !errors.Is(err, ErrInputStopped) {
		t.Fatalf("queued PressKey() error = %v, want ErrInputStopped", err)
	}
	if len(keys.downCalls) != 0 || len(keys.upCalls) != 0 {
		t.Fatalf("queued stopped action emitted input: down=%v up=%v", keys.downCalls, keys.upCalls)
	}
}

func TestInvalidKeyBeforeGuard(t *testing.T) {
	mock := &mockKeySender{}
	c := testKeyboardController(mock, DefaultKeyboardConfig())
	c.SetEnabled(false)

	err := c.PressKey("invalid")
	if !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("PressKey err = %v, want ErrInvalidKey", err)
	}
	if len(mock.downCalls) != 0 {
		t.Fatal("sender should not be called for invalid key")
	}
}

func TestDisabledBlocksMouse(t *testing.T) {
	mock := &mockMouseSender{}
	c := boundMouseController(mock)
	c.SetEnabled(false)

	err := c.MoveTo(100, 100)
	if !errors.Is(err, ErrInputDisabled) {
		t.Fatalf("MoveTo err = %v, want ErrInputDisabled", err)
	}
	if len(mock.moveCalls) != 0 {
		t.Fatal("sender should not be called when disabled")
	}
}

func TestWindowNotBoundBeforeGuard(t *testing.T) {
	mock := &mockMouseSender{}
	c := testMouseController(&mockWindowAPI{}, mock)
	c.SetEnabled(false)

	err := c.MoveTo(10, 20)
	if !errors.Is(err, ErrWindowNotBound) {
		t.Fatalf("MoveTo err = %v, want ErrWindowNotBound before guard", err)
	}
}

func TestTogglePause(t *testing.T) {
	c := testController(&mockWindowAPI{})

	if paused := c.TogglePause("test"); !paused {
		t.Fatal("expected initial toggle to pause")
	}
	if !c.Status().Paused {
		t.Fatal("expected paused after first toggle")
	}
	if paused := c.TogglePause("test"); paused {
		t.Fatal("expected second toggle to resume")
	}
	if c.Status().Paused {
		t.Fatal("expected not paused after second toggle")
	}
}

func TestTogglePauseNoOpAfterStop(t *testing.T) {
	c := testController(&mockWindowAPI{})
	c.Stop("test")

	before := c.Status()
	c.TogglePause("test")
	after := c.Status()
	if after != before {
		t.Fatalf("status changed after stop: before=%+v after=%+v", before, after)
	}
}

func TestStopIsTerminal(t *testing.T) {
	c := testController(&mockWindowAPI{})
	c.Pause("test")
	c.Stop("test")

	st := c.Status()
	if !st.Stopped || st.Paused {
		t.Fatalf("status after stop = %+v, want stopped and not paused", st)
	}

	c.Resume("test")
	if c.Status().Stopped {
		// still stopped
	} else {
		t.Fatal("resume should not clear stopped")
	}
}

func TestComboCleanupBypassesGuard(t *testing.T) {
	sendErr := errors.New("send failed")
	mock := &mockKeySender{
		downErr: map[Key]error{"f1": sendErr},
	}
	c := testKeyboardController(mock, DefaultKeyboardConfig())

	err := c.PressCombo("ctrl", "f1")
	if !errors.Is(err, sendErr) {
		t.Fatalf("PressCombo err = %v, want send failed", err)
	}
	if len(mock.upCalls) != 1 || mock.upCalls[0] != "ctrl" {
		t.Fatalf("cleanup up calls = %v, want [ctrl]", mock.upCalls)
	}
}

func TestNormalizeKeyPause(t *testing.T) {
	got, err := NormalizeKey("pause")
	if err != nil {
		t.Fatal(err)
	}
	if got != "pause" {
		t.Fatalf("NormalizeKey(pause) = %q, want pause", got)
	}
	vk, ok := virtualKey(got)
	if !ok || vk != 0x13 {
		t.Fatalf("virtualKey(pause) = 0x%02X, ok=%v, want 0x13", vk, ok)
	}
}
