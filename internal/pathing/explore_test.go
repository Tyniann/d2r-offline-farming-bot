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
	if plan.Mode != ExploreEntityApproach {
		t.Fatalf("far entrance: Mode = %q, want entity_approach", plan.Mode)
	}
	if plan.Target == (world.Position{}) || plan.Target == far.Position {
		t.Fatalf("far entrance approach Target = %+v, want intermediate non-zero target", plan.Target)
	}
	if got := world.Distance(world.Position{X: 5000, Y: 5000}, plan.Target); got > cfg.MaxEntranceClickDistance/2+0.75 {
		t.Fatalf("far entrance approach distance = %.2f, want short approach step", got)
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

func TestExplorePlannerSelectsRequestedEntranceKindNotNearest(t *testing.T) {
	cfg := ExploreConfig{BearingCount: 8, StepDistanceTiles: 25, MaxEntranceClickDistance: 15}
	p := NewExplorePlanner(cfg)
	nearBackEntrance := world.Entrance{
		Kind:     world.EntranceKindWildernessToTower,
		UnitID:   2,
		Position: world.Position{X: 5002, Y: 5002},
	}
	targetEntrance := world.Entrance{
		Kind:     world.EntranceKindTowerToWilderness,
		UnitID:   4,
		Position: world.Position{X: 5008, Y: 5008},
	}
	goal := Goal{
		Kind:        GoalKindMoveToArea,
		TargetArea:  world.TowerCellarLevel1,
		ViaEntrance: world.EntranceKindTowerToWilderness,
	}

	plan := p.Plan(exploreState(world.ForgottenTower, nearBackEntrance, targetEntrance), goal)
	if plan.Mode != ExploreEntity {
		t.Fatalf("Mode = %q, want entity", plan.Mode)
	}
	if plan.Entrance.UnitID != targetEntrance.UnitID {
		t.Fatalf("Entrance.UnitID = %d, want %d", plan.Entrance.UnitID, targetEntrance.UnitID)
	}
}

func TestExplorePlannerForgottenTowerCellar1UsesUnknownEntrance(t *testing.T) {
	cfg := ExploreConfig{BearingCount: 8, StepDistanceTiles: 25, MaxEntranceClickDistance: 15}
	p := NewExplorePlanner(cfg)
	backToBlackMarsh := world.Entrance{
		Kind:     world.EntranceKindWildernessToTower,
		UnitID:   2,
		Position: world.Position{X: 5001, Y: 5001},
	}
	backToSurface := world.Entrance{
		Kind:     world.EntranceKindTowerToWilderness,
		UnitID:   4,
		Position: world.Position{X: 5002, Y: 5002},
	}
	cellarBreakthrough := world.Entrance{
		Kind:     world.EntranceKindUnknown,
		UnitID:   3,
		Position: world.Position{X: 5008, Y: 5008},
	}
	goal := Goal{
		Kind:       GoalKindMoveToArea,
		TargetArea: world.TowerCellarLevel1,
	}

	plan := p.Plan(exploreState(world.ForgottenTower, backToBlackMarsh, backToSurface, cellarBreakthrough), goal)
	if plan.Mode != ExploreEntity {
		t.Fatalf("Mode = %q, want entity", plan.Mode)
	}
	if plan.Entrance.UnitID != cellarBreakthrough.UnitID {
		t.Fatalf("Entrance.UnitID = %d, want unknown cellar breakthrough unit %d", plan.Entrance.UnitID, cellarBreakthrough.UnitID)
	}
}

func TestExplorePlannerTowerCellarDownIgnoresUpEntrance(t *testing.T) {
	cfg := ExploreConfig{BearingCount: 8, StepDistanceTiles: 25, MaxEntranceClickDistance: 15}
	p := NewExplorePlanner(cfg)
	up := world.Entrance{
		Kind:     world.EntranceKindTowerCellarUp,
		UnitID:   5,
		Position: world.Position{X: 5002, Y: 5002},
	}
	down := world.Entrance{
		Kind:     world.EntranceKindTowerCellarDown,
		UnitID:   6,
		Position: world.Position{X: 5008, Y: 5008},
	}
	goal := Goal{
		Kind:        GoalKindMoveToArea,
		TargetArea:  world.TowerCellarLevel2,
		ViaEntrance: world.EntranceKindTowerCellarDown,
	}

	plan := p.Plan(exploreState(world.TowerCellarLevel1, up, down), goal)
	if plan.Mode != ExploreEntity {
		t.Fatalf("Mode = %q, want entity", plan.Mode)
	}
	if plan.Entrance.UnitID != down.UnitID {
		t.Fatalf("Entrance.UnitID = %d, want down entrance unit %d", plan.Entrance.UnitID, down.UnitID)
	}
}

func TestExplorePlannerForceClicksBlockedApproachEntrance(t *testing.T) {
	cfg := ExploreConfig{BearingCount: 8, StepDistanceTiles: 25, MaxEntranceClickDistance: 15}
	p := NewExplorePlanner(cfg)
	down := world.Entrance{
		Kind:     world.EntranceKindTowerCellarDown,
		UnitID:   13,
		Position: world.Position{X: 5040, Y: 5000},
	}
	goal := Goal{
		Kind:        GoalKindMoveToArea,
		TargetArea:  world.TowerCellarLevel2,
		ViaEntrance: world.EntranceKindTowerCellarDown,
	}

	plan := p.Plan(exploreState(world.TowerCellarLevel1, down), goal)
	if plan.Mode != ExploreEntityApproach {
		t.Fatalf("before force click Mode = %q, want entity_approach", plan.Mode)
	}
	if plan.Entrance.UnitID != down.UnitID {
		t.Fatalf("approach Entrance.UnitID = %d, want %d", plan.Entrance.UnitID, down.UnitID)
	}

	p.ForceClickEntrance(down.UnitID)
	plan = p.Plan(exploreState(world.TowerCellarLevel1, down), goal)
	if plan.Mode != ExploreEntity {
		t.Fatalf("after force click Mode = %q, want entity", plan.Mode)
	}
	if !plan.ForceClick {
		t.Fatal("ForceClick = false, want true")
	}
	if plan.Entrance.UnitID != down.UnitID {
		t.Fatalf("click Entrance.UnitID = %d, want %d", plan.Entrance.UnitID, down.UnitID)
	}
}

func TestExplorePlannerIgnoresStaleFarEntranceAfterAreaChange(t *testing.T) {
	cfg := ExploreConfig{BearingCount: 8, StepDistanceTiles: 25, MaxEntranceClickDistance: 15}
	p := NewExplorePlanner(cfg)
	staleLevel1Down := world.Entrance{
		Kind:     world.EntranceKindTowerCellarDown,
		UnitID:   27,
		Position: world.Position{X: 12546, Y: 5181},
	}
	state := exploreState(world.TowerCellarLevel2, staleLevel1Down)
	state.Player.Position = world.Position{X: 12680, Y: 6507}
	goal := Goal{
		Kind:        GoalKindMoveToArea,
		TargetArea:  world.TowerCellarLevel3,
		ViaEntrance: world.EntranceKindTowerCellarDown,
	}

	plan := p.Plan(state, goal)
	if plan.Mode != ExploreBearing {
		t.Fatalf("Mode = %q, want bearing for stale far entrance", plan.Mode)
	}
	if plan.Entrance.UnitID != 0 {
		t.Fatalf("Entrance.UnitID = %d, want no selected entrance", plan.Entrance.UnitID)
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
