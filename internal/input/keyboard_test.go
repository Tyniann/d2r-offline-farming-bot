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

func TestPressBeltUnconfiguredSlot(t *testing.T) {
	kb := DefaultKeyboardConfig()
	kb.Belt[0] = ""
	c := testKeyboardController(&mockKeySender{}, kb)

	err := c.PressBelt(1)
	if !errors.Is(err, ErrUnconfiguredSlot) {
		t.Fatalf("PressBelt err = %v, want ErrUnconfiguredSlot", err)
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

func TestPressSkillResolvesConfig(t *testing.T) {
	mock := &mockKeySender{}
	kb := DefaultKeyboardConfig()
	kb.Skills[2] = "f3"
	c := testKeyboardController(mock, kb)

	if err := c.PressSkill(3); err != nil {
		t.Fatal(err)
	}
	if len(mock.downCalls) != 1 || mock.downCalls[0] != "f3" {
		t.Fatalf("down = %v, want [f3]", mock.downCalls)
	}
}

func TestPressBeltResolvesConfig(t *testing.T) {
	mock := &mockKeySender{}
	kb := DefaultKeyboardConfig()
	kb.Belt[1] = "2"
	c := testKeyboardController(mock, kb)

	if err := c.PressBelt(2); err != nil {
		t.Fatal(err)
	}
	if len(mock.downCalls) != 1 || mock.downCalls[0] != "2" {
		t.Fatalf("down = %v, want [2]", mock.downCalls)
	}
}

func TestPressTownPortalResolvesConfig(t *testing.T) {
	mock := &mockKeySender{}
	kb := DefaultKeyboardConfig()
	kb.TownPortal = "f8"
	c := testKeyboardController(mock, kb)

	if err := c.PressTownPortal(); err != nil {
		t.Fatal(err)
	}
	if len(mock.downCalls) != 1 || mock.downCalls[0] != "f8" {
		t.Fatalf("down = %v, want [f8]", mock.downCalls)
	}
}

func TestPressSkillInvalidSlot(t *testing.T) {
	c := testKeyboardController(&mockKeySender{}, DefaultKeyboardConfig())
	for _, slot := range []int{0, 9} {
		err := c.PressSkill(slot)
		if !errors.Is(err, ErrInvalidSlot) {
			t.Fatalf("PressSkill(%d) err = %v, want ErrInvalidSlot", slot, err)
		}
	}
}

func TestPressBeltInvalidSlot(t *testing.T) {
	c := testKeyboardController(&mockKeySender{}, DefaultKeyboardConfig())
	for _, slot := range []int{0, 5} {
		err := c.PressBelt(slot)
		if !errors.Is(err, ErrInvalidSlot) {
			t.Fatalf("PressBelt(%d) err = %v, want ErrInvalidSlot", slot, err)
		}
	}
}

func TestPressSkillUnconfiguredSlot(t *testing.T) {
	kb := DefaultKeyboardConfig()
	kb.Skills[0] = ""
	c := testKeyboardController(&mockKeySender{}, kb)

	err := c.PressSkill(1)
	if !errors.Is(err, ErrUnconfiguredSlot) {
		t.Fatalf("PressSkill err = %v, want ErrUnconfiguredSlot", err)
	}
}

func TestPressTownPortalUnconfigured(t *testing.T) {
	kb := DefaultKeyboardConfig()
	kb.TownPortal = ""
	c := testKeyboardController(&mockKeySender{}, kb)

	err := c.PressTownPortal()
	if !errors.Is(err, ErrUnconfiguredSlot) {
		t.Fatalf("PressTownPortal err = %v, want ErrUnconfiguredSlot", err)
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

func TestInputRecordSize(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows-only struct size test")
	}
	const wantSize = 40
	if got := unsafe.Sizeof(inputRecord{}); got != wantSize {
		t.Fatalf("sizeof(inputRecord) = %d, want %d", got, wantSize)
	}
}
