package input

import (
	"errors"
	"testing"
)

type mockBindingSource struct {
	cast      SkillCast
	resolveOK bool
	beltKeys  [4]string
}

func (m mockBindingSource) Resolve(uint16) (SkillCast, error) {
	if !m.resolveOK {
		return SkillCast{}, errors.New("resolve failed")
	}
	return m.cast, nil
}

func (m mockBindingSource) BeltKeyName(slot int) (string, error) {
	if slot < 1 || slot > 4 {
		return "", ErrInvalidSlot
	}
	return m.beltKeys[slot-1], nil
}

func TestSelectSkillPressesBindingKey(t *testing.T) {
	mock := &mockKeySender{}
	c := testKeyboardController(mock, DefaultKeyboardConfig())
	src := mockBindingSource{
		resolveOK: true,
		cast:      SkillCast{SkillID: 54, SelectKey: "f3", CastButton: MouseLeft},
	}

	if err := c.SelectSkill(src, 54); err != nil {
		t.Fatal(err)
	}
	if len(mock.downCalls) != 1 || mock.downCalls[0] != "f3" {
		t.Fatalf("down = %v, want [f3]", mock.downCalls)
	}
}

func TestCastSkillSelectOnlyWithNegativeCoords(t *testing.T) {
	mock := &mockKeySender{}
	c := testKeyboardController(mock, DefaultKeyboardConfig())
	src := mockBindingSource{
		resolveOK: true,
		cast:      SkillCast{SkillID: 54, SelectKey: "f3", CastButton: MouseLeft},
	}
	if err := c.CastSkill(src, 54, NoClientCoord, NoClientCoord); err != nil {
		t.Fatal(err)
	}
	if len(mock.downCalls) != 1 {
		t.Fatalf("down calls = %v, want single select", mock.downCalls)
	}
}

func TestCastSkillAtSelectMoveClickOrder(t *testing.T) {
	keyMock := &mockKeySender{}
	mouseMock := &mockMouseSender{}
	c := mustNewTestController(&mockWindowAPI{
		findHWND: 0x1,
		area:     testWindowFixture,
	}, keyMock, mouseMock, DefaultKeyboardConfig(), testSafetyEnabled(), testKeyTimings())
	if err := c.Bind(42); err != nil {
		t.Fatal(err)
	}

	src := mockBindingSource{
		resolveOK: true,
		cast:      SkillCast{SkillID: 54, SelectKey: "f2", CastButton: MouseRight},
	}
	if err := c.CastSkillAt(src, 54, 400, 300); err != nil {
		t.Fatal(err)
	}
	if len(keyMock.downCalls) != 1 || keyMock.downCalls[0] != "f2" {
		t.Fatalf("key down = %v", keyMock.downCalls)
	}
	if len(mouseMock.moveCalls) != 1 {
		t.Fatalf("move calls = %v", mouseMock.moveCalls)
	}
	if len(mouseMock.downCalls) != 1 || mouseMock.downCalls[0] != MouseRight {
		t.Fatalf("click = %v, want right", mouseMock.downCalls)
	}
}

func TestCastBeltUsesConfiguredKey(t *testing.T) {
	mock := &mockKeySender{}
	c := testKeyboardController(mock, DefaultKeyboardConfig())
	src := mockBindingSource{beltKeys: [4]string{"1", "2", "3", "4"}}

	if err := c.CastBelt(src, 2); err != nil {
		t.Fatal(err)
	}
	if len(mock.downCalls) != 1 || mock.downCalls[0] != "2" {
		t.Fatalf("down = %v, want [2]", mock.downCalls)
	}
}

func TestCastBeltInvalidSlot(t *testing.T) {
	c := testKeyboardController(&mockKeySender{}, DefaultKeyboardConfig())
	src := mockBindingSource{}
	for _, slot := range []int{0, 5} {
		if err := c.CastBelt(src, slot); !errors.Is(err, ErrInvalidSlot) {
			t.Fatalf("CastBelt(%d) err = %v, want ErrInvalidSlot", slot, err)
		}
	}
}
