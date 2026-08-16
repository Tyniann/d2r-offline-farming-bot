package input

import (
	"errors"
	"fmt"
	"runtime"
	"sync"
	"testing"
	"time"
	"unsafe"
)

type mockMouseSender struct {
	moveCalls []struct{ x, y int }
	downCalls []MouseButton
	upCalls   []MouseButton
	moveErr   error
	downErr   map[MouseButton]error
	upErr     map[MouseButton]error
	upFn      func(call int, button MouseButton) error
	upCallsN  int
}

func (m *mockMouseSender) MoveTo(screenX, screenY int) error {
	m.moveCalls = append(m.moveCalls, struct{ x, y int }{screenX, screenY})
	return m.moveErr
}

func (m *mockMouseSender) ButtonDown(button MouseButton) error {
	m.downCalls = append(m.downCalls, button)
	if m.downErr != nil {
		if err, ok := m.downErr[button]; ok {
			return err
		}
	}
	return nil
}

func (m *mockMouseSender) ButtonUp(button MouseButton) error {
	m.upCallsN++
	m.upCalls = append(m.upCalls, button)
	if m.upFn != nil {
		return m.upFn(m.upCallsN, button)
	}
	if m.upErr != nil {
		if err, ok := m.upErr[button]; ok {
			return err
		}
	}
	return nil
}

var testWindowFixture = WindowInfo{
	ClientLeft:   100,
	ClientTop:    200,
	ClientWidth:  800,
	ClientHeight: 600,
}

func testMouseController(api windowAPI, mouse MouseSender) *Controller {
	return mustNewTestController(api, &mockKeySender{}, mouse, DefaultKeyboardConfig(), testSafetyEnabled(), testKeyTimings())
}

func boundMouseController(mouse MouseSender) *Controller {
	api := &mockWindowAPI{
		findHWND: 0x1,
		area:     testWindowFixture,
	}
	c := testMouseController(api, mouse)
	if err := c.Bind(1); err != nil {
		panic(err)
	}
	return c
}

func TestMoveToWithoutBoundWindow(t *testing.T) {
	mock := &mockMouseSender{}
	c := testMouseController(&mockWindowAPI{}, mock)

	err := c.MoveTo(10, 20)
	if !errors.Is(err, ErrWindowNotBound) {
		t.Fatalf("MoveTo err = %v, want ErrWindowNotBound", err)
	}
	if len(mock.moveCalls) != 0 {
		t.Fatal("sender should not be called without bound window")
	}
}

func TestClickWithModifierOrdersAndReleasesInput(t *testing.T) {
	keys := &mockKeySender{}
	mouse := &mockMouseSender{}
	api := &mockWindowAPI{findHWND: 0x1, area: testWindowFixture}
	c := mustNewTestController(api, keys, mouse, DefaultKeyboardConfig(), testSafetyEnabled(), testKeyTimings())
	if err := c.Bind(1); err != nil {
		t.Fatal(err)
	}
	if err := c.ClickWithModifier("ctrl", MouseLeft); err != nil {
		t.Fatalf("ClickWithModifier() error = %v", err)
	}
	if len(keys.downCalls) != 1 || keys.downCalls[0] != "ctrl" || len(keys.upCalls) != 1 || keys.upCalls[0] != "ctrl" {
		t.Fatalf("key down/up = %v/%v, want ctrl once", keys.downCalls, keys.upCalls)
	}
	if len(mouse.downCalls) != 1 || mouse.downCalls[0] != MouseLeft || len(mouse.upCalls) != 1 || mouse.upCalls[0] != MouseLeft {
		t.Fatalf("mouse down/up = %v/%v, want left once", mouse.downCalls, mouse.upCalls)
	}
}

func TestClickWithModifierReleasesCtrlAfterMouseFailure(t *testing.T) {
	keys := &mockKeySender{}
	mouse := &mockMouseSender{downErr: map[MouseButton]error{MouseLeft: errors.New("mouse failed")}}
	api := &mockWindowAPI{findHWND: 0x1, area: testWindowFixture}
	c := mustNewTestController(api, keys, mouse, DefaultKeyboardConfig(), testSafetyEnabled(), testKeyTimings())
	if err := c.Bind(1); err != nil {
		t.Fatal(err)
	}
	if err := c.ClickWithModifier("ctrl", MouseLeft); err == nil {
		t.Fatal("ClickWithModifier() error = nil, want mouse failure")
	}
	if len(keys.upCalls) != 1 || keys.upCalls[0] != "ctrl" {
		t.Fatalf("key up calls = %v, want ctrl cleanup", keys.upCalls)
	}
}

func TestClickWithoutBoundWindow(t *testing.T) {
	mock := &mockMouseSender{}
	c := testMouseController(&mockWindowAPI{}, mock)

	err := c.Click(MouseLeft)
	if !errors.Is(err, ErrWindowNotBound) {
		t.Fatalf("Click err = %v, want ErrWindowNotBound", err)
	}
	if len(mock.downCalls) != 0 || len(mock.upCalls) != 0 {
		t.Fatal("sender should not be called without bound window")
	}
}

func TestMoveToClientToScreenConversion(t *testing.T) {
	cases := []struct {
		name    string
		clientX int
		clientY int
		wantX   int
		wantY   int
	}{
		{"origin clamped", 0, 0, 110, 210},
		{"max clamped", 799, 599, 889, 789},
		{"negative clamped", -50, -50, 110, 210},
		{"inside safe area", 400, 300, 500, 500},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mock := &mockMouseSender{}
			c := boundMouseController(mock)

			if err := c.MoveTo(tc.clientX, tc.clientY); err != nil {
				t.Fatal(err)
			}
			if len(mock.moveCalls) != 1 {
				t.Fatalf("move calls = %d, want 1", len(mock.moveCalls))
			}
			got := mock.moveCalls[0]
			if got.x != tc.wantX || got.y != tc.wantY {
				t.Fatalf("screen = (%d,%d), want (%d,%d)", got.x, got.y, tc.wantX, tc.wantY)
			}
		})
	}
}

func TestClampSmallClientArea(t *testing.T) {
	api := &mockWindowAPI{
		findHWND: 0x1,
		area: WindowInfo{
			ClientLeft:   0,
			ClientTop:    0,
			ClientWidth:  15,
			ClientHeight: 15,
		},
	}
	mock := &mockMouseSender{}
	c := testMouseController(api, mock)
	if err := c.Bind(1); err != nil {
		t.Fatal(err)
	}

	if err := c.MoveTo(0, 0); err != nil {
		t.Fatal(err)
	}
	got := mock.moveCalls[0]
	if got.x != 7 || got.y != 7 {
		t.Fatalf("small client clamp screen = (%d,%d), want (7,7)", got.x, got.y)
	}
}

func TestClickLeftOrder(t *testing.T) {
	mock := &mockMouseSender{}
	c := boundMouseController(mock)

	if err := c.Click(MouseLeft); err != nil {
		t.Fatal(err)
	}
	if len(mock.downCalls) != 1 || mock.downCalls[0] != MouseLeft {
		t.Fatalf("down = %v, want [left]", mock.downCalls)
	}
	if len(mock.upCalls) != 1 || mock.upCalls[0] != MouseLeft {
		t.Fatalf("up = %v, want [left]", mock.upCalls)
	}
}

func TestClickRightOrder(t *testing.T) {
	mock := &mockMouseSender{}
	c := boundMouseController(mock)

	if err := c.Click(MouseRight); err != nil {
		t.Fatal(err)
	}
	if len(mock.downCalls) != 1 || mock.downCalls[0] != MouseRight {
		t.Fatalf("down = %v, want [right]", mock.downCalls)
	}
	if len(mock.upCalls) != 1 || mock.upCalls[0] != MouseRight {
		t.Fatalf("up = %v, want [right]", mock.upCalls)
	}
}

func TestClickInvalidButton(t *testing.T) {
	mock := &mockMouseSender{}
	c := boundMouseController(mock)

	err := c.Click(MouseButton("middle"))
	if !errors.Is(err, ErrInvalidMouseButton) {
		t.Fatalf("Click err = %v, want ErrInvalidMouseButton", err)
	}
	if len(mock.downCalls) != 0 || len(mock.upCalls) != 0 {
		t.Fatal("sender should not be called for invalid button")
	}
}

func TestClickCleanupOnButtonUpFailure(t *testing.T) {
	sendErr := errors.New("send failed")
	mock := &mockMouseSender{
		upFn: func(call int, _ MouseButton) error {
			if call == 1 {
				return sendErr
			}
			return nil
		},
	}
	c := boundMouseController(mock)

	err := c.Click(MouseLeft)
	if !errors.Is(err, sendErr) {
		t.Fatalf("Click err = %v, want original send error", err)
	}
	if len(mock.downCalls) != 1 || len(mock.upCalls) != 2 {
		t.Fatalf("down=%d up=%d, want down=1 up=2 (cleanup)", len(mock.downCalls), len(mock.upCalls))
	}
}

func TestClickCleanupUpErrorStillReturnsOriginal(t *testing.T) {
	sendErr := errors.New("send failed")
	cleanupErr := errors.New("cleanup failed")
	mock := &mockMouseSender{
		upFn: func(call int, _ MouseButton) error {
			if call == 1 {
				return sendErr
			}
			return cleanupErr
		},
	}
	c := boundMouseController(mock)

	err := c.Click(MouseLeft)
	if !errors.Is(err, sendErr) {
		t.Fatalf("Click err = %v, want original send error", err)
	}
}

func TestMoveToSenderError(t *testing.T) {
	mock := &mockMouseSender{moveErr: fmt.Errorf("move: %w", ErrMouseSendFailed)}
	c := boundMouseController(mock)

	err := c.MoveTo(100, 100)
	if !errors.Is(err, ErrMouseSendFailed) {
		t.Fatalf("MoveTo err = %v, want ErrMouseSendFailed", err)
	}
}

func TestClickSenderErrorOnButtonDown(t *testing.T) {
	mock := &mockMouseSender{
		downErr: map[MouseButton]error{
			MouseLeft: fmt.Errorf("down: %w", ErrMouseSendFailed),
		},
	}
	c := boundMouseController(mock)

	err := c.Click(MouseLeft)
	if !errors.Is(err, ErrMouseSendFailed) {
		t.Fatalf("Click err = %v, want ErrMouseSendFailed", err)
	}
	if len(mock.downCalls) != 1 || len(mock.upCalls) != 0 {
		t.Fatalf("down=%d up=%d, want down=1 up=0", len(mock.downCalls), len(mock.upCalls))
	}
}

func TestClampMouseCoord(t *testing.T) {
	cases := []struct {
		value, size int
		want        int
		clamped     bool
	}{
		{0, 800, 10, true},
		{789, 800, 789, false},
		{799, 800, 789, true},
		{7, 15, 7, false},
		{0, 15, 7, true},
	}
	for _, tc := range cases {
		got, clamped := clampMouseCoord(tc.value, tc.size)
		if got != tc.want || clamped != tc.clamped {
			t.Fatalf("clampMouseCoord(%d,%d) = (%d,%v), want (%d,%v)",
				tc.value, tc.size, got, clamped, tc.want, tc.clamped)
		}
	}
}

func TestWinMouseSenderButtonFlags(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows-only mouse flag test")
	}
	cases := []struct {
		button  MouseButton
		release bool
		want    uint32
	}{
		{MouseLeft, false, mouseEventFLeftDown},
		{MouseLeft, true, mouseEventFLeftUp},
		{MouseRight, false, mouseEventFRightDown},
		{MouseRight, true, mouseEventFRightUp},
	}
	for _, tc := range cases {
		got, err := mouseButtonFlags(tc.button, tc.release)
		if err != nil {
			t.Fatalf("mouseButtonFlags(%q,%v): %v", tc.button, tc.release, err)
		}
		if got != tc.want {
			t.Fatalf("mouseButtonFlags(%q,%v) = 0x%X, want 0x%X", tc.button, tc.release, got, tc.want)
		}
	}
}

func TestWinMouseSenderSetCursorPos(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows-only SetCursorPos test")
	}
	var gotX, gotY int
	sender := &winMouseSender{
		moveCursor: func(x, y int) error {
			gotX, gotY = x, y
			return nil
		},
		send: func(_ []mouseInputRecord) (uint32, error) { return 0, nil },
	}
	if err := sender.MoveTo(42, 99); err != nil {
		t.Fatal(err)
	}
	if gotX != 42 || gotY != 99 {
		t.Fatalf("SetCursorPos = (%d,%d), want (42,99)", gotX, gotY)
	}
}

func TestWinMouseSenderMoveToErrMouseSendFailed(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows-only SetCursorPos error test")
	}
	posErr := errors.New("set cursor pos failed")
	sender := &winMouseSender{
		moveCursor: func(int, int) error { return posErr },
		send:       func(_ []mouseInputRecord) (uint32, error) { return 0, nil },
	}

	err := sender.MoveTo(1, 2)
	if !errors.Is(err, ErrMouseSendFailed) {
		t.Fatalf("MoveTo err = %v, want ErrMouseSendFailed", err)
	}
	if !errors.Is(err, posErr) {
		t.Fatalf("MoveTo err = %v, want underlying pos error", err)
	}
}

func TestWinMouseSenderSendInputPayload(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows-only SendInput adapter test")
	}

	var captured []mouseInputRecord
	sender := &winMouseSender{
		moveCursor: func(int, int) error { return nil },
		send: func(inputs []mouseInputRecord) (uint32, error) {
			captured = append(captured, inputs...)
			return uint32(len(inputs)), nil
		},
	}

	if err := sender.ButtonDown(MouseLeft); err != nil {
		t.Fatal(err)
	}
	if err := sender.ButtonUp(MouseLeft); err != nil {
		t.Fatal(err)
	}
	if len(captured) != 2 {
		t.Fatalf("captured inputs = %d, want 2", len(captured))
	}
	if captured[0].Type != inputMouse || captured[0].Mi.Flags != mouseEventFLeftDown {
		t.Fatalf("left down payload = %+v", captured[0])
	}
	if captured[1].Type != inputMouse || captured[1].Mi.Flags != mouseEventFLeftUp {
		t.Fatalf("left up payload = %+v", captured[1])
	}
}

func TestWinMouseSenderErrMouseSendFailed(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows-only SendInput adapter test")
	}

	sendErr := errors.New("send input failed")
	sender := &winMouseSender{
		moveCursor: func(int, int) error { return nil },
		send: func(_ []mouseInputRecord) (uint32, error) {
			return 0, sendErr
		},
	}

	err := sender.ButtonDown(MouseRight)
	if !errors.Is(err, ErrMouseSendFailed) {
		t.Fatalf("ButtonDown err = %v, want ErrMouseSendFailed", err)
	}
	if !errors.Is(err, sendErr) {
		t.Fatalf("ButtonDown err = %v, want underlying send error", err)
	}
}

func TestMouseInputRecordSize(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows-only struct size test")
	}
	const wantSize = 40
	if got := unsafe.Sizeof(mouseInputRecord{}); got != wantSize {
		t.Fatalf("sizeof(mouseInputRecord) = %d, want %d", got, wantSize)
	}
}

type orderedInputRecorder struct {
	mu     sync.Mutex
	events []string
}

func (r *orderedInputRecorder) add(event string) {
	r.mu.Lock()
	r.events = append(r.events, event)
	r.mu.Unlock()
}

func (r *orderedInputRecorder) snapshot() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.events...)
}

type orderedKeySender struct {
	recorder  *orderedInputRecorder
	downHook  func(Key)
	upCalls   int
	failUpFor int
}

func (s *orderedKeySender) KeyDown(key Key) error {
	s.recorder.add("key_down:" + string(key))
	if s.downHook != nil {
		s.downHook(key)
	}
	return nil
}

func (s *orderedKeySender) KeyUp(key Key) error {
	s.upCalls++
	s.recorder.add("key_up:" + string(key))
	if s.upCalls <= s.failUpFor {
		return errors.New("key up failed")
	}
	return nil
}

type orderedMouseSender struct {
	recorder *orderedInputRecorder
}

func (s *orderedMouseSender) MoveTo(x, y int) error {
	s.recorder.add(fmt.Sprintf("move:%d,%d", x, y))
	return nil
}

func (s *orderedMouseSender) ButtonDown(button MouseButton) error {
	s.recorder.add("mouse_down:" + string(button))
	return nil
}

func (s *orderedMouseSender) ButtonUp(button MouseButton) error {
	s.recorder.add("mouse_up:" + string(button))
	return nil
}

func newForegroundTransactionController(keys KeySender, mouse MouseSender) *Controller {
	api := &mockWindowAPI{
		findHWND:   0x1234,
		foreground: true,
		area:       WindowInfo{Handle: 0x1234, ClientLeft: 100, ClientTop: 200, ClientWidth: 800, ClientHeight: 600},
	}
	c := mustNewTestController(api, keys, mouse, DefaultKeyboardConfig(), testSafetyEnabled(), testKeyTimings())
	if err := c.Bind(42); err != nil {
		panic(err)
	}
	return c
}

func TestHoldAtLeavesOnlyLeftDown(t *testing.T) {
	recorder := &orderedInputRecorder{}
	c := newForegroundTransactionController(&orderedKeySender{recorder: recorder}, &orderedMouseSender{recorder: recorder})
	if err := c.HoldAt(400, 300, MouseLeft); err != nil {
		t.Fatal(err)
	}
	if !c.ModifierHoldActive() {
		t.Fatal("hold should stay active")
	}
	want := []string{"move:500,500", "mouse_down:left"}
	if got := recorder.snapshot(); fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("events = %v, want %v", got, want)
	}
	if err := c.ReleaseModifierHold(); err != nil {
		t.Fatal(err)
	}
	want = append(want, "mouse_up:left")
	if got := recorder.snapshot(); fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("release events = %v, want %v", got, want)
	}
}

func TestHoldAtWithModifierLeavesShiftAndLeftDown(t *testing.T) {
	recorder := &orderedInputRecorder{}
	c := newForegroundTransactionController(&orderedKeySender{recorder: recorder}, &orderedMouseSender{recorder: recorder})
	if err := c.HoldAtWithModifier(400, 300, "shift", MouseLeft); err != nil {
		t.Fatal(err)
	}
	if !c.ModifierHoldActive() {
		t.Fatal("hold should stay active")
	}
	want := []string{"move:500,500", "key_down:shift", "mouse_down:left"}
	if got := recorder.snapshot(); fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("events = %v, want %v", got, want)
	}
	if err := c.ReleaseModifierHold(); err != nil {
		t.Fatal(err)
	}
	if c.ModifierHoldActive() {
		t.Fatal("hold should be released")
	}
	want = append(want, "mouse_up:left", "key_up:shift")
	if got := recorder.snapshot(); fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("release events = %v, want %v", got, want)
	}
}

func TestPauseReleasesHeldModifierClick(t *testing.T) {
	recorder := &orderedInputRecorder{}
	c := newForegroundTransactionController(&orderedKeySender{recorder: recorder}, &orderedMouseSender{recorder: recorder})
	if err := c.HoldAtWithModifier(400, 300, "shift", MouseLeft); err != nil {
		t.Fatal(err)
	}
	c.Pause("test")
	if c.ModifierHoldActive() {
		t.Fatal("pause should release the hold")
	}
	wantSuffix := []string{"mouse_up:left", "key_up:shift"}
	events := recorder.snapshot()
	if len(events) < 2 || fmt.Sprint(events[len(events)-2:]) != fmt.Sprint(wantSuffix) {
		t.Fatalf("events=%v, want release suffix %v", events, wantSuffix)
	}
}

func TestClickAtWithModifierOrdersOneAtomicTransaction(t *testing.T) {
	recorder := &orderedInputRecorder{}
	c := newForegroundTransactionController(&orderedKeySender{recorder: recorder}, &orderedMouseSender{recorder: recorder})
	if err := c.ClickAtWithModifier(400, 300, "shift", MouseLeft); err != nil {
		t.Fatal(err)
	}
	want := []string{"move:500,500", "key_down:shift", "mouse_down:left", "mouse_up:left", "key_up:shift"}
	got := recorder.snapshot()
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("events = %v, want %v", got, want)
	}
}

func TestClickAtWithModifierRechecksForegroundBeforeInput(t *testing.T) {
	recorder := &orderedInputRecorder{}
	api := &mockWindowAPI{
		findHWND: 0x1234,
		area:     WindowInfo{Handle: 0x1234, ClientLeft: 100, ClientTop: 200, ClientWidth: 800, ClientHeight: 600},
	}
	c := mustNewTestController(api, &orderedKeySender{recorder: recorder}, &orderedMouseSender{recorder: recorder}, DefaultKeyboardConfig(), testSafetyEnabled(), testKeyTimings())
	if err := c.Bind(42); err != nil {
		t.Fatal(err)
	}
	if err := c.ClickAtWithModifier(400, 300, "shift", MouseLeft); !errors.Is(err, ErrWindowNotForeground) {
		t.Fatalf("ClickAtWithModifier() error = %v, want ErrWindowNotForeground", err)
	}
	if got := recorder.snapshot(); len(got) != 0 {
		t.Fatalf("foreground rejection emitted input: %v", got)
	}
}

func TestClickAtWithModifierStopsAfterUnrecoverableModifierCleanup(t *testing.T) {
	recorder := &orderedInputRecorder{}
	keys := &orderedKeySender{recorder: recorder, failUpFor: 2}
	c := newForegroundTransactionController(keys, &orderedMouseSender{recorder: recorder})
	if err := c.ClickAtWithModifier(400, 300, "shift", MouseLeft); err == nil {
		t.Fatal("ClickAtWithModifier() error = nil, want modifier release failure")
	}
	if !c.Status().Stopped || keys.upCalls != 2 {
		t.Fatalf("status=%+v key-up calls=%d, want stopped after two failed releases", c.Status(), keys.upCalls)
	}
	events := recorder.snapshot()
	mouseDowns := 0
	for _, event := range events {
		if event == "mouse_down:left" {
			mouseDowns++
		}
	}
	if mouseDowns != 1 {
		t.Fatalf("events=%v, want exactly one LMB down and no fallback", events)
	}
}

func TestStopDuringModifiedClickAllowsCleanupButBlocksFollowingInput(t *testing.T) {
	recorder := &orderedInputRecorder{}
	keys := &orderedKeySender{recorder: recorder}
	mouse := &orderedMouseSender{recorder: recorder}
	c := newForegroundTransactionController(keys, mouse)
	keys.downHook = func(key Key) {
		if key == "shift" {
			c.Stop("test_during_transaction")
		}
	}
	if err := c.ClickAtWithModifier(400, 300, "shift", MouseLeft); err != nil {
		t.Fatalf("in-flight transaction should finish cleanup: %v", err)
	}
	if !c.Status().Stopped {
		t.Fatal("controller should remain stopped")
	}
	before := len(recorder.snapshot())
	if err := c.Click(MouseLeft); !errors.Is(err, ErrInputStopped) {
		t.Fatalf("following click error = %v, want ErrInputStopped", err)
	}
	if after := len(recorder.snapshot()); after != before {
		t.Fatalf("following stopped click emitted input: before=%d after=%d", before, after)
	}
}

func TestGameplayLeasePreventsPlayerPotionInterleaveWithShiftClick(t *testing.T) {
	recorder := &orderedInputRecorder{}
	shiftHeld := make(chan struct{})
	releaseShift := make(chan struct{})
	keys := &orderedKeySender{recorder: recorder}
	keys.downHook = func(key Key) {
		if key == "shift" {
			close(shiftHeld)
			<-releaseShift
		}
	}
	c := newForegroundTransactionController(keys, &orderedMouseSender{recorder: recorder})
	clickDone := make(chan error, 1)
	go func() { clickDone <- c.ClickAtWithModifier(400, 300, "shift", MouseLeft) }()
	<-shiftHeld
	beltDone := make(chan error, 1)
	go func() {
		beltDone <- c.CastBelt(mockBindingSource{beltKeys: [4]string{"1", "2", "3", "4"}}, 1)
	}()
	select {
	case err := <-beltDone:
		t.Fatalf("belt action interleaved before Shift cleanup: %v", err)
	case <-time.After(25 * time.Millisecond):
	}
	close(releaseShift)
	if err := <-clickDone; err != nil {
		t.Fatal(err)
	}
	if err := <-beltDone; err != nil {
		t.Fatal(err)
	}
	wantSuffix := []string{"mouse_down:left", "mouse_up:left", "key_up:shift", "key_down:1", "key_up:1"}
	events := recorder.snapshot()
	if len(events) < len(wantSuffix) || fmt.Sprint(events[len(events)-len(wantSuffix):]) != fmt.Sprint(wantSuffix) {
		t.Fatalf("events=%v, want suffix %v", events, wantSuffix)
	}
}
