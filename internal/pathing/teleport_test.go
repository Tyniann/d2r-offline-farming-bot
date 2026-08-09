package pathing

import (
	"errors"
	"testing"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/input"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/memory"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

type fixedProjector struct {
	x  int
	y  int
	ok bool
}

func (p fixedProjector) Project(_, _ world.Position, _ input.WindowInfo) (int, int, bool) {
	return p.x, p.y, p.ok
}

func (fixedProjector) Mode() ProjectionMode {
	return ProjectionRelative
}

type stubRightSkillClicker struct {
	calls   int
	sent    bool
	err     error
	lastID  uint16
	lastRMB uint16
}

func (s *stubRightSkillClicker) CastRightSkillAt(skillID uint16, rightSkillID uint16, _ time.Time, _, _ int) (bool, error) {
	s.calls++
	s.lastID = skillID
	s.lastRMB = rightSkillID
	return s.sent, s.err
}

func TestTeleportMoverRequiresRightSkillClicker(t *testing.T) {
	m := NewTeleportMover(testLogger(), newMockInput(), mockBindings{}, fixedProjector{x: 100, y: 100, ok: true}, 0)
	_, _, err := m.TeleportTo(time.Now(), world.Player{Position: world.Position{X: 1, Y: 1}, RightSkillID: memory.SkillTeleport}, world.Position{X: 2, Y: 2})
	if err == nil || !stringsContains(err.Error(), "right skill clicker is required") {
		t.Fatalf("err = %v, want clicker required", err)
	}
}

func TestTeleportMoverClampsCastOutsideBottomUI(t *testing.T) {
	in := newMockInput()
	clicker := &stubRightSkillClicker{sent: true}
	m := NewTeleportMover(
		testLogger(),
		in,
		mockBindings{},
		fixedProjector{x: 640, y: 680, ok: true},
		0,
	)
	m.SetRightSkillClicker(clicker)

	x, y, err := m.TeleportTo(time.Now(), world.Player{Position: world.Position{X: 1, Y: 1}, RightSkillID: memory.SkillTeleport}, world.Position{X: 2, Y: 2})
	if err != nil {
		t.Fatalf("TeleportTo() error = %v", err)
	}

	maxY := int(float64(in.window.ClientHeight) * maxTeleportClientYFraction)
	if y != maxY {
		t.Fatalf("clientY = %d, want clamped %d", y, maxY)
	}
	if x != 640 {
		t.Fatalf("clientX = %d, want 640", x)
	}
	if clicker.calls != 1 || clicker.lastID != memory.SkillTeleport || len(in.casts) != 0 {
		t.Fatalf("clicker calls=%d id=%d casts=%v", clicker.calls, clicker.lastID, in.casts)
	}
}

func TestTeleportMoverClampsCastInsideClientBounds(t *testing.T) {
	in := newMockInput()
	clicker := &stubRightSkillClicker{sent: true}
	m := NewTeleportMover(
		testLogger(),
		in,
		mockBindings{},
		fixedProjector{x: -20, y: -10, ok: true},
		0,
	)
	m.SetRightSkillClicker(clicker)

	x, y, err := m.TeleportTo(time.Now(), world.Player{Position: world.Position{X: 1, Y: 1}, RightSkillID: memory.SkillTeleport}, world.Position{X: 2, Y: 2})
	if err != nil {
		t.Fatalf("TeleportTo() error = %v", err)
	}

	if x != 0 || y != 0 {
		t.Fatalf("client = (%d,%d), want (0,0)", x, y)
	}
	if clicker.calls != 1 || len(in.casts) != 0 {
		t.Fatalf("clicker calls=%d casts=%v", clicker.calls, in.casts)
	}
}

func TestTeleportMoverWaitsWhenSelectorPendingThenCastsWithoutRecastSkill(t *testing.T) {
	in := newMockInput()
	clicker := &stubRightSkillClicker{sent: false}
	m := NewTeleportMover(testLogger(), in, mockBindings{}, fixedProjector{x: 200, y: 200, ok: true}, 0)
	m.SetRightSkillClicker(clicker)
	player := world.Player{Position: world.Position{X: 1, Y: 1}, RightSkillID: memory.SkillBoneSpear}
	now := time.Now()

	_, _, err := m.TeleportTo(now, player, world.Position{X: 2, Y: 2})
	if !errors.Is(err, ErrTeleportSelectionPending) || clicker.calls != 1 {
		t.Fatalf("pending err=%v calls=%d", err, clicker.calls)
	}
	if len(in.casts) != 0 || len(in.keys) != 0 {
		t.Fatalf("unexpected direct cast path casts=%v keys=%v", in.casts, in.keys)
	}

	clicker.sent = true
	player.RightSkillID = memory.SkillTeleport
	_, _, err = m.TeleportTo(now.Add(time.Millisecond), player, world.Position{X: 3, Y: 3})
	if err != nil || clicker.calls != 2 || clicker.lastRMB != memory.SkillTeleport {
		t.Fatalf("confirm err=%v calls=%d right=%d", err, clicker.calls, clicker.lastRMB)
	}
	if len(in.casts) != 0 {
		t.Fatalf("CastSkillAt must stay unused, casts=%v", in.casts)
	}
}

func stringsContains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (haystack == needle || len(needle) == 0 ||
		(func() bool {
			for i := 0; i+len(needle) <= len(haystack); i++ {
				if haystack[i:i+len(needle)] == needle {
					return true
				}
			}
			return false
		})())
}
