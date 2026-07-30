package pathing

import (
	"context"
	"testing"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

func testNavigator(in *mockInput) *Navigator {
	cfg := DefaultConfig()
	cfg.MoveInterval = 0
	return NewNavigator(testLogger(), Deps{
		Input:    in,
		Bindings: mockBindings{},
		Config:   cfg,
	})
}

func navState(at time.Time, area world.AreaID, x, y uint32) world.State {
	return world.State{
		At:     at,
		Valid:  true,
		Phase:  world.GamePhaseInGame,
		Area:   world.LookupArea(area),
		Player: world.Player{Position: world.Position{X: x, Y: y}},
	}
}

func TestNavigatorArrivesOnAreaChange(t *testing.T) {
	in := newMockInput()
	n := testNavigator(in)
	if err := n.Start(Goal{Kind: GoalKindMoveToArea, TargetArea: world.BlackMarsh}); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if !n.Active() {
		t.Fatal("navigator must be active after Start")
	}

	base := time.Now()
	// Ticks in the wrong area: navigator explores by teleport.
	res := n.Tick(context.Background(), navState(base, world.BloodMoor, 5000, 5000))
	if res.Done || res.Status != NavExploring {
		t.Fatalf("Tick() = %+v, want exploring", res)
	}
	if len(in.casts) != 1 {
		t.Fatalf("casts=%d, want 1 teleport", len(in.casts))
	}

	// Area matches: arrived.
	res = n.Tick(context.Background(), navState(base.Add(time.Second), world.BlackMarsh, 5100, 5100))
	if !res.Done || res.Status != NavArrived || res.Reason != "" {
		t.Fatalf("Tick() = %+v, want arrived", res)
	}
	if n.Active() {
		t.Fatal("navigator must be inactive after arrival")
	}
	if got := n.LastResult(); got.Status != NavArrived {
		t.Fatalf("LastResult() = %+v, want arrived", got)
	}
}

func TestNavigatorStuckWithoutProgress(t *testing.T) {
	in := newMockInput()
	n := testNavigator(in)
	if err := n.Start(Goal{Kind: GoalKindMoveToArea, TargetArea: world.BlackMarsh}); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	base := time.Now()
	var last NavTickResult
	// Same position, ticks spread within tickGapReset but beyond stuck timeout.
	for i := 0; i < 20; i++ {
		at := base.Add(time.Duration(i) * time.Second)
		last = n.Tick(context.Background(), navState(at, world.BloodMoor, 5000, 5000))
		if last.Done {
			break
		}
	}
	if !last.Done || last.Status != NavStuck || last.Reason != ReasonStuck {
		t.Fatalf("final Tick() = %+v, want stuck", last)
	}
}

func TestNavigatorSkipsTicksWhileLoading(t *testing.T) {
	in := newMockInput()
	n := testNavigator(in)
	if err := n.Start(Goal{Kind: GoalKindMoveToArea, TargetArea: world.BlackMarsh}); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	loading := world.State{Valid: false, Phase: world.GamePhaseLoading}
	res := n.Tick(context.Background(), loading)
	if res.Done {
		t.Fatalf("Tick() during loading = %+v, want in-flight", res)
	}
	if len(in.casts) != 0 {
		t.Fatalf("casts=%d, want 0 during loading", len(in.casts))
	}
}

func TestNavigatorCancelledContext(t *testing.T) {
	in := newMockInput()
	n := testNavigator(in)
	if err := n.Start(Goal{Kind: GoalKindMoveToArea, TargetArea: world.BlackMarsh}); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	res := n.Tick(ctx, navState(time.Now(), world.BloodMoor, 5000, 5000))
	if !res.Done || res.Status != NavFailed || res.Reason != ReasonCancelled {
		t.Fatalf("Tick() = %+v, want failed/cancelled", res)
	}
}

func TestNavigatorRejectsInvalidGoals(t *testing.T) {
	n := testNavigator(newMockInput())
	if err := n.Start(Goal{Kind: GoalKindMoveToArea}); err == nil {
		t.Fatal("Start() without target area must fail")
	}
	if err := n.Start(Goal{Kind: GoalKindMoveToPosition}); err == nil {
		t.Fatal("Start() without target position must fail")
	}
	if err := n.Start(Goal{}); err == nil {
		t.Fatal("Start() with no goal kind must fail")
	}
}

func TestNavigatorMoveToPositionArrives(t *testing.T) {
	in := newMockInput()
	n := testNavigator(in)
	target := world.Position{X: 5100, Y: 5000}
	if err := n.Start(Goal{Kind: GoalKindMoveToPosition, TargetPos: target}); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	base := time.Now()
	res := n.Tick(context.Background(), navState(base, world.BloodMoor, 5000, 5000))
	if res.Done || res.Status != NavMoving {
		t.Fatalf("Tick() = %+v, want moving", res)
	}
	if !res.MovementInputSent || res.NextMovementInputAt != base || res.MovementProgressTiles != 3 {
		t.Fatalf("movement input result = %+v", res)
	}
	if len(in.casts) != 1 {
		t.Fatalf("casts=%d, want 1 teleport", len(in.casts))
	}

	res = n.Tick(context.Background(), navState(base.Add(time.Second), world.BloodMoor, 5095, 5000))
	if !res.Done || res.Status != NavArrived {
		t.Fatalf("Tick() = %+v, want arrived within arrival_distance", res)
	}
}

func TestNavigatorKeepsBearingWhileProgressing(t *testing.T) {
	in := newMockInput()
	n := testNavigator(in)
	if err := n.Start(Goal{Kind: GoalKindMoveToArea, TargetArea: world.BlackMarsh}); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	base := time.Now()
	n.Tick(context.Background(), navState(base, world.BloodMoor, 5000, 5000))
	if len(in.casts) != 1 {
		t.Fatalf("casts=%d, want 1", len(in.casts))
	}

	// Teleport landed: position advanced well before the settle timeout.
	n.Tick(context.Background(), navState(base.Add(300*time.Millisecond), world.BloodMoor, 5020, 5000))
	if len(in.casts) != 2 {
		t.Fatalf("casts=%d, want 2 (fast chaining on progress)", len(in.casts))
	}
	if n.explorer.BearingIndex() != 0 {
		t.Fatalf("BearingIndex = %d, want 0 (no rotation while progressing)", n.explorer.BearingIndex())
	}
}

func TestNavigatorWaitsForTeleportSettle(t *testing.T) {
	in := newMockInput()
	n := testNavigator(in)
	if err := n.Start(Goal{Kind: GoalKindMoveToArea, TargetArea: world.BlackMarsh}); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	base := time.Now()
	n.Tick(context.Background(), navState(base, world.BloodMoor, 5000, 5000))
	// Cast still in flight: no movement yet, settle timeout not elapsed.
	n.Tick(context.Background(), navState(base.Add(200*time.Millisecond), world.BloodMoor, 5000, 5000))
	if len(in.casts) != 1 {
		t.Fatalf("casts=%d, want 1 (no re-cast before settle)", len(in.casts))
	}
	if n.explorer.BearingIndex() != 0 {
		t.Fatalf("BearingIndex = %d, want 0 (no rotation before settle)", n.explorer.BearingIndex())
	}
}

func TestNavigatorRotatesBearingAfterBlockedCast(t *testing.T) {
	in := newMockInput()
	n := testNavigator(in)
	if err := n.Start(Goal{Kind: GoalKindMoveToArea, TargetArea: world.BlackMarsh}); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	base := time.Now()
	n.Tick(context.Background(), navState(base, world.BloodMoor, 5000, 5000))
	// Settle timeout elapsed without movement: cast was blocked by terrain.
	n.Tick(context.Background(), navState(base.Add(teleportSettleTimeout+100*time.Millisecond), world.BloodMoor, 5000, 5000))
	if n.explorer.BearingIndex() != 1 {
		t.Fatalf("BearingIndex = %d, want 1 (rotation after blocked cast)", n.explorer.BearingIndex())
	}
	if len(in.casts) != 2 {
		t.Fatalf("casts=%d, want 2 (re-cast with new bearing)", len(in.casts))
	}
}

func TestNavigatorForceClicksAfterBlockedEntityApproach(t *testing.T) {
	in := newMockInput()
	n := testNavigator(in)
	goal := Goal{
		Kind:        GoalKindMoveToArea,
		TargetArea:  world.TowerCellarLevel2,
		ViaEntrance: world.EntranceKindTowerCellarDown,
	}
	if err := n.Start(goal); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	base := time.Now()
	down := world.Entrance{
		Kind:     world.EntranceKindTowerCellarDown,
		UnitID:   13,
		Position: world.Position{X: 5016, Y: 5000},
		Name:     "Act 1 Tower Cellar Down",
	}
	state := navState(base, world.TowerCellarLevel1, 5000, 5000)
	state.Entrances = []world.Entrance{down}

	res := n.Tick(context.Background(), state)
	if res.Done || res.Status != NavExploring {
		t.Fatalf("first Tick() = %+v, want exploring", res)
	}
	if len(in.casts) != 1 {
		t.Fatalf("casts=%d, want 1 entity approach teleport", len(in.casts))
	}

	state.At = base.Add(teleportSettleTimeout + 100*time.Millisecond)
	res = n.Tick(context.Background(), state)
	if res.Done || res.Status != NavClicking {
		t.Fatalf("blocked approach Tick() = %+v, want clicking", res)
	}
	if len(in.casts) != 1 {
		t.Fatalf("casts=%d, want no repeated blocked approach cast", len(in.casts))
	}
	if len(in.moves) != 1 {
		t.Fatalf("moves=%d, want 1 hover probe after blocked approach", len(in.moves))
	}
}

func TestNavigatorNotWiredRejectsStart(t *testing.T) {
	n := NewNavigator(testLogger(), Deps{Config: DefaultConfig()})
	if n.Ready() {
		t.Fatal("Ready() = true without input/bindings")
	}
	if err := n.Start(Goal{Kind: GoalKindMoveToArea, TargetArea: world.BlackMarsh}); err == nil {
		t.Fatal("Start() must fail when not wired")
	}
}
