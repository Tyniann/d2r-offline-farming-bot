package input

import (
	"errors"
	"fmt"
	"log/slog"
	"testing"
	"time"
)

type mockWindowAPI struct {
	findHWND nativeWindow
	findErr  error
	area     WindowInfo
	areaErr  error

	findCalls     int
	areaCalls     int
	lastPID       uint32
	lastTitle     string
	activateErr   error
	foreground    bool
	foregroundAt  int
	activateCalls int
}

func (m *mockWindowAPI) FindMainWindow(pid uint32, title string) (nativeWindow, error) {
	m.findCalls++
	m.lastPID = pid
	m.lastTitle = title
	if m.findErr != nil {
		return 0, m.findErr
	}
	return m.findHWND, nil
}

func (m *mockWindowAPI) ClientArea(_ nativeWindow) (WindowInfo, error) {
	m.areaCalls++
	if m.areaErr != nil {
		return WindowInfo{}, m.areaErr
	}
	return m.area, nil
}

func (m *mockWindowAPI) Activate(_ nativeWindow) error {
	m.activateCalls++
	return m.activateErr
}

func (m *mockWindowAPI) IsForeground(_ nativeWindow) bool {
	return m.foreground || (m.foregroundAt > 0 && m.activateCalls >= m.foregroundAt)
}

func testSafetyEnabled() SafetyConfig {
	return SafetyConfig{Enabled: true, PauseHotkey: "pause", StopHotkey: "f12"}
}

func testSafetyDisabled() SafetyConfig {
	return SafetyConfig{Enabled: false, PauseHotkey: "pause", StopHotkey: "f12"}
}

func mustNewTestController(api windowAPI, keys KeySender, mouse MouseSender, kb KeyboardConfig, safety SafetyConfig, timings keyTimings) *Controller {
	c, err := newWithBackends(slog.Default(), api, keys, mouse, kb, safety, timings, nil)
	if err != nil {
		panic(err)
	}
	return c
}

func testController(api windowAPI) *Controller {
	return mustNewTestController(api, &mockKeySender{}, &mockMouseSender{}, DefaultKeyboardConfig(), testSafetyDisabled(), testKeyTimings())
}

func testKeyTimings() keyTimings {
	return keyTimings{
		sleep: func(time.Duration) {},
		delay: func(minMs, maxMs int) time.Duration {
			return time.Duration(minMs) * time.Millisecond
		},
	}
}

func TestBindStoresWindowInfo(t *testing.T) {
	api := &mockWindowAPI{
		findHWND: 0x1234,
		area: WindowInfo{
			Title:        defaultWindowTitle,
			Handle:       0x1234,
			ClientLeft:   10,
			ClientTop:    20,
			ClientWidth:  800,
			ClientHeight: 600,
		},
	}
	c := testController(api)

	if err := c.Bind(42); err != nil {
		t.Fatal(err)
	}
	if api.findCalls != 1 || api.areaCalls != 1 {
		t.Fatalf("api calls find=%d area=%d, want 1/1", api.findCalls, api.areaCalls)
	}
	if api.lastPID != 42 || api.lastTitle != defaultWindowTitle {
		t.Fatalf("FindMainWindow args pid=%d title=%q", api.lastPID, api.lastTitle)
	}

	info, ok := c.Window()
	if !ok || !c.Bound() {
		t.Fatal("expected bound window")
	}
	if info.PID != 42 || info.ClientWidth != 800 {
		t.Fatalf("unexpected window info: %+v", info)
	}
}

func TestFocusActivatesAndVerifiesBoundWindow(t *testing.T) {
	api := &mockWindowAPI{findHWND: 0x1234, area: WindowInfo{Handle: 0x1234, ClientWidth: 1280, ClientHeight: 720}, foreground: true}
	c := mustNewTestController(api, &mockKeySender{}, &mockMouseSender{}, DefaultKeyboardConfig(), testSafetyEnabled(), testKeyTimings())
	if err := c.Bind(42); err != nil {
		t.Fatal(err)
	}
	if err := c.Focus(); err != nil {
		t.Fatalf("Focus() error = %v", err)
	}
	if api.activateCalls != 1 {
		t.Fatalf("activate calls = %d, want 1", api.activateCalls)
	}
}

func TestFocusRejectsUnconfirmedForeground(t *testing.T) {
	api := &mockWindowAPI{findHWND: 0x1234, area: WindowInfo{Handle: 0x1234, ClientWidth: 1280, ClientHeight: 720}}
	c := mustNewTestController(api, &mockKeySender{}, &mockMouseSender{}, DefaultKeyboardConfig(), testSafetyEnabled(), testKeyTimings())
	if err := c.Bind(42); err != nil {
		t.Fatal(err)
	}
	if err := c.Focus(); !errors.Is(err, ErrWindowNotForeground) {
		t.Fatalf("Focus() error = %v, want ErrWindowNotForeground", err)
	}
}

func TestFocusRetriesActivationUntilForegroundConfirmed(t *testing.T) {
	api := &mockWindowAPI{
		findHWND:     0x1234,
		area:         WindowInfo{Handle: 0x1234, ClientWidth: 1280, ClientHeight: 720},
		foregroundAt: 3,
	}
	c := mustNewTestController(api, &mockKeySender{}, &mockMouseSender{}, DefaultKeyboardConfig(), testSafetyEnabled(), testKeyTimings())
	if err := c.Bind(42); err != nil {
		t.Fatal(err)
	}
	if err := c.Focus(); err != nil {
		t.Fatalf("Focus() error = %v", err)
	}
	if api.activateCalls != 3 {
		t.Fatalf("activate calls = %d, want 3", api.activateCalls)
	}
}

func TestCaptureClientRequiresBoundWindow(t *testing.T) {
	c := testController(&mockWindowAPI{})
	if _, err := c.CaptureClient(); !errors.Is(err, ErrWindowNotBound) {
		t.Fatalf("CaptureClient() error = %v, want ErrWindowNotBound", err)
	}
}

func TestBindInvalidPID(t *testing.T) {
	api := &mockWindowAPI{}
	c := testController(api)

	err := c.Bind(0)
	if !errors.Is(err, ErrInvalidPID) {
		t.Fatalf("Bind(0) err = %v, want ErrInvalidPID", err)
	}
	if api.findCalls != 0 {
		t.Fatalf("FindMainWindow calls = %d, want 0", api.findCalls)
	}
}

func TestBindReplacesPreviousBinding(t *testing.T) {
	api := &mockWindowAPI{
		findHWND: 0x100,
		area: WindowInfo{
			Title:        defaultWindowTitle,
			Handle:       0x100,
			ClientWidth:  640,
			ClientHeight: 480,
		},
	}
	c := testController(api)

	if err := c.Bind(1); err != nil {
		t.Fatal(err)
	}

	api.findHWND = 0x200
	api.area.Handle = 0x200
	api.area.ClientWidth = 1024

	if err := c.Bind(2); err != nil {
		t.Fatal(err)
	}
	info, _ := c.Window()
	if info.PID != 2 || info.Handle != 0x200 || info.ClientWidth != 1024 {
		t.Fatalf("unexpected replaced window: %+v", info)
	}
}

func TestUnbindIdempotent(t *testing.T) {
	api := &mockWindowAPI{
		findHWND: 0x1,
		area:     WindowInfo{Handle: 0x1, ClientWidth: 100, ClientHeight: 100},
	}
	c := testController(api)

	c.Unbind()
	c.Unbind()

	if c.Bound() {
		t.Fatal("expected unbound after idempotent Unbind")
	}
	if _, ok := c.Window(); ok {
		t.Fatal("Window() should report not bound")
	}

	if err := c.Bind(7); err != nil {
		t.Fatal(err)
	}
	c.Unbind()
	if c.Bound() {
		t.Fatal("expected unbound after Bind then Unbind")
	}
}

func TestReadyIndependentOfBound(t *testing.T) {
	c := testController(&mockWindowAPI{})
	if !c.Ready() {
		t.Fatal("Ready() should be true before Bind")
	}
	if c.Bound() {
		t.Fatal("Bound() should be false before Bind")
	}
}

func TestIsBindRetryable(t *testing.T) {
	cases := []struct {
		err  error
		want bool
	}{
		{ErrWindowNotFound, true},
		{ErrInvalidClientArea, true},
		{fmt.Errorf("wrap: %w", ErrWindowNotFound), true},
		{ErrInvalidPID, false},
		{ErrUnsupportedPlatform, false},
		{errors.New("other"), false},
	}
	for _, tc := range cases {
		if got := IsBindRetryable(tc.err); got != tc.want {
			t.Fatalf("IsBindRetryable(%v) = %v, want %v", tc.err, got, tc.want)
		}
	}
}

func TestWindowReturnsCopy(t *testing.T) {
	api := &mockWindowAPI{
		findHWND: 0x55,
		area: WindowInfo{
			Handle:       0x55,
			ClientWidth:  300,
			ClientHeight: 200,
		},
	}
	c := testController(api)
	if err := c.Bind(9); err != nil {
		t.Fatal(err)
	}

	info, ok := c.Window()
	if !ok {
		t.Fatal("expected bound window")
	}
	info.ClientWidth = 999

	info2, _ := c.Window()
	if info2.ClientWidth != 300 {
		t.Fatalf("Window() copy mutated: got width %d", info2.ClientWidth)
	}
}
