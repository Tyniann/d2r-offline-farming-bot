package pathing

import (
	"testing"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

func exploreState(area world.AreaID, entrances ...world.Entrance) world.State {
	return world.State{
		Valid:     true,
		Phase:     world.GamePhaseInGame,
		Area:      world.LookupArea(area),
		Player:    world.Player{Position: world.Position{X: 5000, Y: 5000}},
		Entrances: entrances,
	}
}

func TestExplorePlannerBearingRotation(t *testing.T) {
	cfg := ExploreConfig{BearingCount: 4, StepDistanceTiles: 10, MaxEntranceClickDistance: 15}
	p := NewExplorePlanner(cfg)
	state := exploreState(world.BloodMoor)

	first := p.Plan(state, Goal{Kind: GoalKindMoveToArea, TargetArea: world.BlackMarsh})
	if first.Mode != ExploreBearing {
		t.Fatalf("Plan().Mode = %q, want bearing", first.Mode)
	}

	seen := map[world.Position]bool{first.Target: true}
	for i := 0; i < 3; i++ {
		p.Rotate()
		plan := p.Plan(state, Goal{Kind: GoalKindMoveToArea, TargetArea: world.BlackMarsh})
		if seen[plan.Target] {
			t.Fatalf("bearing %d produced duplicate target %+v", p.BearingIndex(), plan.Target)
		}
		seen[plan.Target] = true
	}

	// Full rotation wraps back to the first target.
	p.Rotate()
	plan := p.Plan(state, Goal{Kind: GoalKindMoveToArea, TargetArea: world.BlackMarsh})
	if plan.Target != first.Target {
		t.Fatalf("after full rotation Target = %+v, want %+v", plan.Target, first.Target)
	}
}

func TestExplorePlannerEntityOnlyWithinDistance(t *testing.T) {
	cfg := ExploreConfig{BearingCount: 8, StepDistanceTiles: 25, MaxEntranceClickDistance: 15}
	p := NewExplorePlanner(cfg)
	goal := Goal{
		Kind:        GoalKindMoveToArea,
		TargetArea:  world.ForgottenTower,
		ViaEntrance: world.EntranceKindWildernessToTower,
	}

	far := world.Entrance{
		Kind:     world.EntranceKindWildernessToTower,
		UnitID:   7,
		Position: world.Position{X: 5100, Y: 5100},
	}
	plan := p.Plan(exploreState(world.BlackMarsh, far), goal)
	if plan.Mode != ExploreBearing {
		t.Fatalf("far entrance: Mode = %q, want bearing", plan.Mode)
	}

	near := far
	near.Position = world.Position{X: 5008, Y: 5008}
	plan = p.Plan(exploreState(world.BlackMarsh, near), goal)
	if plan.Mode != ExploreEntity {
		t.Fatalf("near entrance: Mode = %q, want entity", plan.Mode)
	}
	if plan.Entrance.UnitID != 7 {
		t.Fatalf("Entrance.UnitID = %d, want 7", plan.Entrance.UnitID)
	}

	// Without a ViaEntrance hint, even a near entrance keeps bearing mode.
	plan = p.Plan(exploreState(world.BlackMarsh, near), Goal{Kind: GoalKindMoveToArea, TargetArea: world.ForgottenTower})
	if plan.Mode != ExploreBearing {
		t.Fatalf("no via_entrance: Mode = %q, want bearing", plan.Mode)
	}
}

func TestExplorePlannerResetsBearingOnAreaChange(t *testing.T) {
	cfg := ExploreConfig{BearingCount: 8, StepDistanceTiles: 25, MaxEntranceClickDistance: 15}
	p := NewExplorePlanner(cfg)
	goal := Goal{Kind: GoalKindMoveToArea, TargetArea: world.BlackMarsh}

	p.Plan(exploreState(world.BloodMoor), goal)
	p.Rotate()
	p.Rotate()
	if p.BearingIndex() != 2 {
		t.Fatalf("BearingIndex = %d, want 2", p.BearingIndex())
	}

	p.Plan(exploreState(world.ColdPlains), goal)
	if p.BearingIndex() != 0 {
		t.Fatalf("BearingIndex after area change = %d, want 0", p.BearingIndex())
	}
}
