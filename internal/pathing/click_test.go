package pathing

import (
	"io"
	"log/slog"
	"testing"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func testClickState(hover world.HoverInfo) world.State {
	return world.State{
		Valid:  true,
		Phase:  world.GamePhaseInGame,
		Player: world.Player{Position: world.Position{X: 5000, Y: 5000}},
		Hover:  hover,
	}
}

func testClickTarget() ClickTarget {
	return ClickTarget{
		UnitID:   42,
		UnitType: world.HoverUnitTypeObject,
		Position: world.Position{X: 5005, Y: 5005},
		Name:     "Waypoint",
	}
}

func TestEntityClickerHitAfterHoverMatch(t *testing.T) {
	in := newMockInput()
	clicker := NewEntityClicker(testLogger(), in, DefaultRelativeProjector(), DefaultConfig().Click)
	target := testClickTarget()

	// First tick: no hover yet — mouse moves, no click.
	res, err := clicker.Tick(testClickState(world.HoverInfo{}), target, 0)
	if err != nil {
		t.Fatalf("Tick() error = %v", err)
	}
	if res.Status != ClickPending || res.Done {
		t.Fatalf("Tick() = %+v, want pending", res)
	}
	if len(in.moves) != 1 || len(in.clicks) != 0 {
		t.Fatalf("moves=%d clicks=%d, want 1 move and no click", len(in.moves), len(in.clicks))
	}

	// Second tick: hover confirms the target — click happens.
	hover := world.HoverInfo{IsHovered: true, UnitType: world.HoverUnitTypeObject, UnitID: 42}
	res, err = clicker.Tick(testClickState(hover), target, 0)
	if err != nil {
		t.Fatalf("Tick() error = %v", err)
	}
	if res.Status != ClickHit || !res.Done {
		t.Fatalf("Tick() = %+v, want hit", res)
	}
	if len(in.clicks) != 1 {
		t.Fatalf("clicks=%d, want exactly 1", len(in.clicks))
	}
}

func TestEntityClickerHoverNotFound(t *testing.T) {
	in := newMockInput()
	cfg := DefaultConfig().Click
	cfg.MaxHoverAttempts = 3
	clicker := NewEntityClicker(testLogger(), in, DefaultRelativeProjector(), cfg)
	target := testClickTarget()

	var last ClickTickResult
	for i := 0; i < 10; i++ {
		res, err := clicker.Tick(testClickState(world.HoverInfo{}), target, 0)
		if err != nil {
			t.Fatalf("Tick() error = %v", err)
		}
		last = res
		if res.Done {
			break
		}
	}
	if last.Status != ClickHoverNotFound || !last.Done {
		t.Fatalf("final result = %+v, want hover_not_found", last)
	}
	if len(in.clicks) != 0 {
		t.Fatalf("clicks=%d, want 0 (no blind click)", len(in.clicks))
	}
	if len(in.moves) != 3 {
		t.Fatalf("moves=%d, want 3 (max attempts)", len(in.moves))
	}
}

func TestEntityClickerNoClickOnWrongUnit(t *testing.T) {
	in := newMockInput()
	clicker := NewEntityClicker(testLogger(), in, DefaultRelativeProjector(), DefaultConfig().Click)
	target := testClickTarget()

	if _, err := clicker.Tick(testClickState(world.HoverInfo{}), target, 0); err != nil {
		t.Fatalf("Tick() error = %v", err)
	}
	// Hovering a monster with the same UnitID must not trigger a click.
	wrongType := world.HoverInfo{IsHovered: true, UnitType: world.HoverUnitTypeMonster, UnitID: 42}
	res, err := clicker.Tick(testClickState(wrongType), target, 0)
	if err != nil {
		t.Fatalf("Tick() error = %v", err)
	}
	if res.Status != ClickPending || len(in.clicks) != 0 {
		t.Fatalf("res=%+v clicks=%d, want pending without click", res, len(in.clicks))
	}
	// Hovering a different object must not trigger a click either.
	wrongID := world.HoverInfo{IsHovered: true, UnitType: world.HoverUnitTypeObject, UnitID: 43}
	res, err = clicker.Tick(testClickState(wrongID), target, 0)
	if err != nil {
		t.Fatalf("Tick() error = %v", err)
	}
	if res.Status != ClickPending || len(in.clicks) != 0 {
		t.Fatalf("res=%+v clicks=%d, want pending without click", res, len(in.clicks))
	}
}

func TestEntityClickerTooFar(t *testing.T) {
	in := newMockInput()
	clicker := NewEntityClicker(testLogger(), in, DefaultRelativeProjector(), DefaultConfig().Click)
	target := testClickTarget()
	target.Position = world.Position{X: 5100, Y: 5100}

	res, err := clicker.Tick(testClickState(world.HoverInfo{}), target, 15)
	if err != nil {
		t.Fatalf("Tick() error = %v", err)
	}
	if res.Status != ClickTooFar || !res.Done {
		t.Fatalf("Tick() = %+v, want too_far", res)
	}
	if len(in.moves) != 0 {
		t.Fatalf("moves=%d, want 0", len(in.moves))
	}
}

func TestEntityClickerProjectionFailedWithoutWindow(t *testing.T) {
	in := newMockInput()
	in.unbound = true
	clicker := NewEntityClicker(testLogger(), in, DefaultRelativeProjector(), DefaultConfig().Click)

	res, err := clicker.Tick(testClickState(world.HoverInfo{}), testClickTarget(), 0)
	if err != nil {
		t.Fatalf("Tick() error = %v", err)
	}
	if res.Status != ClickProjectionFailed || !res.Done {
		t.Fatalf("Tick() = %+v, want projection_failed", res)
	}
}

func TestEntityClickerProjectionFailedInsideBottomUI(t *testing.T) {
	in := newMockInput()
	clicker := NewEntityClicker(testLogger(), in, fixedProjector{x: 426, y: 800, ok: true}, DefaultConfig().Click)

	res, err := clicker.Tick(testClickState(world.HoverInfo{}), testClickTarget(), 0)
	if err != nil {
		t.Fatalf("Tick() error = %v", err)
	}
	if res.Status != ClickProjectionFailed || !res.Done {
		t.Fatalf("Tick() = %+v, want projection_failed", res)
	}
	if len(in.moves) != 0 || len(in.clicks) != 0 {
		t.Fatalf("moves=%d clicks=%d, want no UI input", len(in.moves), len(in.clicks))
	}
}

func TestSpiralOffsetDeterministic(t *testing.T) {
	x0, y0 := spiralOffset(0, 40)
	if x0 != 4 || y0 != 0 {
		t.Fatalf("spiralOffset(0) = (%d,%d), want (4,0)", x0, y0)
	}
	for attempt := 0; attempt < 5; attempt++ {
		x1, y1 := spiralOffset(attempt, 40)
		x2, y2 := spiralOffset(attempt, 40)
		if x1 != x2 || y1 != y2 {
			t.Fatalf("spiralOffset(%d) not deterministic: (%d,%d) vs (%d,%d)", attempt, x1, y1, x2, y2)
		}
	}
	// Offsets must vary across attempts so new screen positions are probed.
	x1, y1 := spiralOffset(1, 40)
	x3, y3 := spiralOffset(3, 40)
	if x1 == x3 && y1 == y3 {
		t.Fatalf("spiral offsets for attempts 1 and 3 identical: (%d,%d)", x1, y1)
	}
}

func TestAnchoredPosition(t *testing.T) {
	got := anchoredPosition(world.Position{X: 100, Y: 50}, 2)
	if got != (world.Position{X: 98, Y: 48}) {
		t.Fatalf("anchoredPosition() = %+v, want {98 48}", got)
	}
	// Near-zero coordinates must not underflow.
	got = anchoredPosition(world.Position{X: 1, Y: 0}, 2)
	if got != (world.Position{X: 1, Y: 0}) {
		t.Fatalf("anchoredPosition() low coords = %+v, want unchanged", got)
	}
}
