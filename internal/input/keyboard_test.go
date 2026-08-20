package input

import (
	"errors"
	"runtime"
	"testing"
	"time"
	"unsafe"
)

type mockKeySender struct {
	downCalls []Key
	upCalls   []Key
	downErr   map[Key]error
	upErr     map[Key]error
}

func (m *mockKeySender) KeyDown(key Key) error {
	m.downCalls = append(m.downCalls, key)
	if m.downErr != nil {
		if err, ok := m.downErr[key]; ok {
			return err
		}
	}
	return nil
}

func (m *mockKeySender) KeyUp(key Key) error {
	m.upCalls = append(m.upCalls, key)
	if m.upErr != nil {
		if err, ok := m.upErr[key]; ok {
			return err
		}
	}
	return nil
}

func testKeyboardController(keys KeySender, kb KeyboardConfig) *Controller {
	return mustNewTestController(&mockWindowAPI{}, keys, &mockMouseSender{}, kb, testSafetyEnabled(), testKeyTimings())
}

func TestNormalizeKey(t *testing.T) {
	cases := []struct {
		raw  string
		want Key
		err  bool
	}{
		{"F1", "f1", false},
		{" 1 ", "1", false},
		{"ctrl", "ctrl", false},
		{",", ",", false},
		{".", ".", false},
		{"-", "-", false},
		{"]", "]", false},
		{"", "", true},
		{"control", "", true},
		{"lctrl", "", true},
		{"unknown", "", true},
	}
	for _, tc := range cases {
		got, err := NormalizeKey(tc.raw)
		if tc.err {
			if !errors.Is(err, ErrInvalidKey) {
				t.Errorf("NormalizeKey(%q) err = %v, want ErrInvalidKey", tc.raw, err)
			}
			continue
		}
		if err != nil {
			t.Errorf("NormalizeKey(%q) unexpected err: %v", tc.raw, err)
			continue
		}
		if got != tc.want {
			t.Errorf("NormalizeKey(%q) = %q, want %q", tc.raw, got, tc.want)
		}
	}
}

func TestValidateKeyStrings(t *testing.T) {
	if err := ValidateKeyStrings("f1", "", "2"); err != nil {
		t.Fatalf("ValidateKeyStrings valid keys: %v", err)
	}
	if err := ValidateKeyStrings("bad"); !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("ValidateKeyStrings invalid: %v", err)
	}
}

func TestPressKeyDelayBetweenDownAndUp(t *testing.T) {
	mock := &mockKeySender{}
	var slept time.Duration
	timings := keyTimings{
		sleep: func(d time.Duration) { slept = d },
		delay: func(minMs, maxMs int) time.Duration {
			return time.Duration(minMs) * time.Millisecond
		},
	}
	kb := DefaultKeyboardConfig()
	kb.KeyDelayMsMin = 17
	kb.KeyDelayMsMax = 17
	c := mustNewTestController(&mockWindowAPI{}, mock, &mockMouseSender{}, kb, testSafetyEnabled(), timings)

	if err := c.PressKey("f1"); err != nil {
		t.Fatal(err)
	}
	if len(mock.downCalls) != 1 || len(mock.upCalls) != 1 {
		t.Fatalf("down/up calls = %d/%d, want 1/1", len(mock.downCalls), len(mock.upCalls))
	}
	if slept != 17*time.Millisecond {
		t.Fatalf("sleep between down/up = %v, want 17ms", slept)
	}
}

func TestPressComboUsesConfiguredHold(t *testing.T) {
	mock := &mockKeySender{}
	var slept time.Duration
	timings := keyTimings{
		sleep: func(d time.Duration) { slept = d },
		delay: func(_, _ int) time.Duration { return 0 },
	}
	kb := DefaultKeyboardConfig()
	kb.ComboHoldMs = 123
	c := mustNewTestController(&mockWindowAPI{}, mock, &mockMouseSender{}, kb, testSafetyEnabled(), timings)

	if err := c.PressCombo("ctrl", "f1"); err != nil {
		t.Fatal(err)
	}
	if slept != 123*time.Millisecond {
		t.Fatalf("combo hold sleep = %v, want 123ms", slept)
	}
}

func TestPressKeyEmptyReturnsErrInvalidKey(t *testing.T) {
	mock := &mockKeySender{}
	c := testKeyboardController(mock, DefaultKeyboardConfig())

	err := c.PressKey("")
	if !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("PressKey(\"\") err = %v, want ErrInvalidKey", err)
	}
	if len(mock.downCalls) != 0 {
		t.Fatal("sender should not be called for empty key")
	}
}

func TestWinKeySenderErrKeySendFailed(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows-only SendInput adapter test")
	}

	sendErr := errors.New("send input failed")
	sender := &winKeySender{
		send: func(_ []inputRecord) (uint32, error) {
			return 0, sendErr
		},
	}

	err := sender.KeyDown("a")
	if !errors.Is(err, ErrKeySendFailed) {
		t.Fatalf("KeyDown err = %v, want ErrKeySendFailed", err)
	}
	if !errors.Is(err, sendErr) {
		t.Fatalf("KeyDown err = %v, want underlying send error", err)
	}
}

func TestPressKeyOrder(t *testing.T) {
	mock := &mockKeySender{}
	c := testKeyboardController(mock, DefaultKeyboardConfig())

	if err := c.PressKey("F1"); err != nil {
		t.Fatal(err)
	}
	if len(mock.downCalls) != 1 || mock.downCalls[0] != "f1" {
		t.Fatalf("down calls = %v, want [f1]", mock.downCalls)
	}
	if len(mock.upCalls) != 1 || mock.upCalls[0] != "f1" {
		t.Fatalf("up calls = %v, want [f1]", mock.upCalls)
	}
}

func TestPressKeyInvalidDoesNotCallSender(t *testing.T) {
	mock := &mockKeySender{}
	c := testKeyboardController(mock, DefaultKeyboardConfig())

	err := c.PressKey("invalid")
	if !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("PressKey err = %v, want ErrInvalidKey", err)
	}
	if len(mock.downCalls) != 0 || len(mock.upCalls) != 0 {
		t.Fatal("sender should not be called for invalid key")
	}
}

func TestPressComboOrder(t *testing.T) {
	mock := &mockKeySender{}
	kb := DefaultKeyboardConfig()
	kb.ComboHoldMs = 50
	c := testKeyboardController(mock, kb)

	if err := c.PressCombo("ctrl", "f1"); err != nil {
		t.Fatal(err)
	}
	wantDown := []Key{"ctrl", "f1"}
	wantUp := []Key{"f1", "ctrl"}
	if len(mock.downCalls) != len(wantDown) {
		t.Fatalf("down calls = %v, want %v", mock.downCalls, wantDown)
	}
	for i := range wantDown {
		if mock.downCalls[i] != wantDown[i] {
			t.Fatalf("down[%d] = %q, want %q", i, mock.downCalls[i], wantDown[i])
		}
	}
	for i := range wantUp {
		if mock.upCalls[i] != wantUp[i] {
			t.Fatalf("up[%d] = %q, want %q", i, mock.upCalls[i], wantUp[i])
		}
	}
}

func TestPressComboCleanupOnSecondKeyDownFailure(t *testing.T) {
	sendErr := errors.New("send failed")
	mock := &mockKeySender{
		downErr: map[Key]error{"f1": sendErr},
	}
	c := testKeyboardController(mock, DefaultKeyboardConfig())

	err := c.PressCombo("ctrl", "f1")
	if !errors.Is(err, sendErr) {
		t.Fatalf("PressCombo err = %v, want send failed", err)
	}
	if len(mock.downCalls) != 2 {
		t.Fatalf("down calls = %d, want 2", len(mock.downCalls))
	}
	if len(mock.upCalls) != 1 || mock.upCalls[0] != "ctrl" {
		t.Fatalf("cleanup up calls = %v, want [ctrl]", mock.upCalls)
	}
}

func TestPressComboCleanupUpErrorStillReturnsOriginal(t *testing.T) {
	sendErr := errors.New("send failed")
	upErr := errors.New("cleanup failed")
	mock := &mockKeySender{
		downErr: map[Key]error{"f1": sendErr},
		upErr:   map[Key]error{"ctrl": upErr},
	}
	c := testKeyboardController(mock, DefaultKeyboardConfig())

	err := c.PressCombo("ctrl", "f1")
	if !errors.Is(err, sendErr) {
		t.Fatalf("PressCombo err = %v, want original send error", err)
	}
}

func TestRandomDelayDeterministicWhenEqual(t *testing.T) {
	got := randomDelay(25, 25)
	if got != 25*time.Millisecond {
		t.Fatalf("randomDelay(25,25) = %v, want 25ms", got)
	}
}

func TestVirtualKeyMapping(t *testing.T) {
	cases := map[string]uint16{
		"1": 0x31, "a": 0x41, "f1": 0x70, "f12": 0x7B,
		"shift": 0xA0, "ctrl": 0xA2, "alt": 0xA4,
		"esc": 0x1B, "enter": 0x0D, "space": 0x20, "tab": 0x09,
		",": 0xBC, ".": 0xBE, "-": 0xBD, "]": 0xDD,
	}
	for name, wantVK := range cases {
		vk, ok := virtualKey(Key(name))
		if !ok {
			t.Fatalf("virtualKey(%q) not found", name)
		}
		if vk != wantVK {
			t.Fatalf("virtualKey(%q) = 0x%02X, want 0x%02X", name, vk, wantVK)
		}
	}
}

func TestWinKeySenderSendInputPayload(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows-only SendInput adapter test")
	}

	var captured []inputRecord
	sender := &winKeySender{
		send: func(inputs []inputRecord) (uint32, error) {
			captured = append(captured, inputs...)
			return uint32(len(inputs)), nil
		},
	}

	if err := sender.KeyDown("a"); err != nil {
		t.Fatal(err)
	}
	if err := sender.KeyUp("a"); err != nil {
		t.Fatal(err)
	}
	if len(captured) != 2 {
		t.Fatalf("captured inputs = %d, want 2", len(captured))
	}
	if captured[0].Ki.Vk != 0x41 || captured[0].Ki.Flags != 0 {
		t.Fatalf("keydown payload = %+v", captured[0])
	}
	if captured[1].Ki.Vk != 0x41 || captured[1].Ki.Flags != keyEventFKeyUp {
		t.Fatalf("keyup payload = %+v", captured[1])
	}
}

func TestTypeRuneSendsVirtualKeysNotUnicode(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows-only SendInput adapter test")
	}

	var captured []inputRecord
	sender := &winKeySender{
		send: func(inputs []inputRecord) (uint32, error) {
			captured = append(captured, inputs...)
			return uint32(len(inputs)), nil
		},
	}

	for _, r := range []rune{'p', '/', '8', ' '} {
		captured = captured[:0]
		if err := sender.TypeRune(r); err != nil {
			t.Fatalf("TypeRune(%q): %v", r, err)
		}
		if len(captured) < 2 {
			t.Fatalf("TypeRune(%q) inputs = %d, want at least key down/up", r, len(captured))
		}
		sawDown, sawUp := false, false
		for _, rec := range captured {
			if rec.Ki.Flags&keyEventFUnicode != 0 {
				t.Fatalf("TypeRune(%q) used UNICODE SendInput (%+v); D2R chat ignores that", r, rec.Ki)
			}
			if rec.Ki.Vk == 0 {
				t.Fatalf("TypeRune(%q) missing virtual key (%+v)", r, rec.Ki)
			}
			if rec.Ki.Flags&keyEventFKeyUp == 0 {
				sawDown = true
			} else {
				sawUp = true
			}
		}
		if !sawDown || !sawUp {
			t.Fatalf("TypeRune(%q) down=%v up=%v payload=%+v", r, sawDown, sawUp, captured)
		}
	}
}

func TestParseVkKeyScanShiftAndFailure(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows-only SendInput adapter test")
	}
	if _, _, err := parseVkKeyScan(0xFFFF, '/'); err == nil {
		t.Fatal("expected unmapped rune to fail")
	}
	vk, shift, err := parseVkKeyScan(0x0137, '/')
	if err != nil {
		t.Fatal(err)
	}
	if vk != 0x37 || !shift {
		t.Fatalf("shift+7 = vk=0x%02X shift=%v", vk, shift)
	}
	if _, _, err := parseVkKeyScan(0x0250, 'p'); err == nil {
		t.Fatal("expected ctrl mapping to fail")
	}
}

func TestInputRecordSize(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows-only struct size test")
	}
	const wantSize = 40
	if got := unsafe.Sizeof(inputRecord{}); got != wantSize {
		t.Fatalf("sizeof(inputRecord) = %d, want %d", got, wantSize)
	}
}
