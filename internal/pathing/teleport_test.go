package pathing

import (
	"testing"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/input"
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

func TestTeleportMoverClampsCastOutsideBottomUI(t *testing.T) {
	in := newMockInput()
	m := NewTeleportMover(
		testLogger(),
		in,
		mockBindings{},
		fixedProjector{x: 640, y: 680, ok: true},
		0,
	)

	x, y, err := m.TeleportTo(time.Now(), world.Position{X: 1, Y: 1}, world.Position{X: 2, Y: 2})
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
	if len(in.casts) != 1 || in.casts[0] != [2]int{640, maxY} {
		t.Fatalf("casts = %v, want one clamped cast", in.casts)
	}
}

func TestTeleportMoverClampsCastInsideClientBounds(t *testing.T) {
	in := newMockInput()
	m := NewTeleportMover(
		testLogger(),
		in,
		mockBindings{},
		fixedProjector{x: -20, y: -10, ok: true},
		0,
	)

	x, y, err := m.TeleportTo(time.Now(), world.Position{X: 1, Y: 1}, world.Position{X: 2, Y: 2})
	if err != nil {
		t.Fatalf("TeleportTo() error = %v", err)
	}

	if x != 0 || y != 0 {
		t.Fatalf("client = (%d,%d), want (0,0)", x, y)
	}
	if len(in.casts) != 1 || in.casts[0] != [2]int{0, 0} {
		t.Fatalf("casts = %v, want one clamped cast", in.casts)
	}
}
