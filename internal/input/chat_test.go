package input

import (
	"errors"
	"testing"
	"time"
)

type mockRuneSender struct {
	mockKeySender
	runes []rune
	err   error
}

func (m *mockRuneSender) TypeRune(r rune) error {
	m.runes = append(m.runes, r)
	return m.err
}

func TestSendChatCommandTypesPlayersAndSubmits(t *testing.T) {
	mock := &mockRuneSender{}
	var sleeps []time.Duration
	timings := keyTimings{
		sleep: func(d time.Duration) { sleeps = append(sleeps, d) },
		delay: func(_, _ int) time.Duration { return 5 * time.Millisecond },
	}
	c := mustNewTestController(&mockWindowAPI{}, mock, &mockMouseSender{}, DefaultKeyboardConfig(), testSafetyEnabled(), timings)

	if err := c.SendChatCommand("/players 8"); err != nil {
		t.Fatal(err)
	}
	if len(mock.downCalls) != 2 || mock.downCalls[0] != "enter" || mock.downCalls[1] != "enter" {
		t.Fatalf("keys = %v", mock.downCalls)
	}
	if string(mock.runes) != "/players 8" {
		t.Fatalf("typed = %q", string(mock.runes))
	}
	if len(sleeps) == 0 || sleeps[0] != chatOpenSettle {
		t.Fatalf("first sleep = %v, want chat settle", sleeps)
	}
}

func TestSendChatCommandRejectsUnknownText(t *testing.T) {
	mock := &mockRuneSender{}
	c := testKeyboardController(mock, DefaultKeyboardConfig())
	if err := c.SendChatCommand("/help"); !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("err = %v", err)
	}
	if len(mock.downCalls) != 0 || len(mock.runes) != 0 {
		t.Fatal("rejected command still sent input")
	}
}

func TestSendChatCommandRequiresRuneSender(t *testing.T) {
	c := testKeyboardController(&mockKeySender{}, DefaultKeyboardConfig())
	if err := c.SendChatCommand("/players 1"); !errors.Is(err, ErrUnsupportedPlatform) {
		t.Fatalf("err = %v", err)
	}
}
