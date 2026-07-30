package tasks

import (
	"testing"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

var benchmarkThreatAssessment ThreatAssessment

func phase17ThreatConfig() RouteCombatConfig {
	return RouteCombatConfig{
		Enabled: true, ImmediateRadiusTiles: 18, CorridorWidthTiles: 7,
		LandingRadiusTiles: 10, AttackDistanceTiles: 30,
	}
}

func phase17ThreatProgress() RouteProgress {
	return RouteProgress{
		RouteID: "route", SegmentID: "segment", PointIndex: 1,
		PreviousConfirmed: world.Position{X: 100, Y: 100},
		MovementTarget:    world.Position{X: 140, Y: 100},
		TargetAvailable:   true, Mode: RouteProgressMovement,
	}
}

func phase17ThreatState(monsters ...world.Monster) world.State {
	return world.State{
		At:    time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC),
		Valid: true, Phase: world.GamePhaseInGame,
		Player:   world.Player{Position: world.Position{X: 100, Y: 100}},
		Monsters: monsters,
	}
}

func TestAssessThreatsPrioritizesZoneThenDistanceThenUnitID(t *testing.T) {
	state := phase17ThreatState(
		world.Monster{NPCID: world.ArcaneGhoulLord, UnitID: 90, Position: world.Position{X: 140, Y: 100}},
		world.Monster{NPCID: world.ArcaneHellClan, UnitID: 80, Position: world.Position{X: 125, Y: 106}},
		world.Monster{NPCID: world.ArcaneSpecter, UnitID: 20, Position: world.Position{X: 110, Y: 100}},
		world.Monster{NPCID: world.ArcaneSpecter, UnitID: 10, Position: world.Position{X: 110, Y: 100}},
		world.Monster{NPCID: world.ArcaneSpecter, UnitID: 1, Position: world.Position{X: 125, Y: 108}},
		world.Monster{NPCID: 999, UnitID: 2, Position: world.Position{X: 101, Y: 100}},
	)
	assessment := assessThreats(state, phase17ThreatProgress(),
		[]uint32{world.ArcaneSpecter, world.ArcaneHellClan, world.ArcaneGhoulLord}, phase17ThreatConfig())
	if !assessment.RouteTargetFound || assessment.RouteZone != ThreatZoneImmediate || assessment.RouteTarget.UnitID != 10 {
		t.Fatalf("route target = %+v zone=%s", assessment.RouteTarget, assessment.RouteZone)
	}
	if assessment.RelevantThreatCount != 4 {
		t.Fatalf("relevant count = %d", assessment.RelevantThreatCount)
	}
	if !assessment.DensityTargetFound || assessment.DensityTarget.UnitID != 10 {
		t.Fatalf("density target = %+v", assessment.DensityTarget)
	}
	if assessment.RequiredCoverageTiles != 50 || !assessment.CoverageComplete {
		t.Fatalf("coverage = %+v", assessment)
	}
}

func TestAssessThreatsGeometryBoundaries(t *testing.T) {
	tests := []struct {
		name     string
		position world.Position
		found    bool
		zone     ThreatZone
	}{
		{"immediate inclusive", world.Position{X: 118, Y: 100}, true, ThreatZoneImmediate},
		{"landing inclusive", world.Position{X: 150, Y: 100}, true, ThreatZoneLanding},
		{"corridor inclusive", world.Position{X: 125, Y: 107}, true, ThreatZoneCorridor},
		{"corridor outside", world.Position{X: 125, Y: 108}, false, ThreatZoneNone},
		{"behind corridor", world.Position{X: 80, Y: 107}, false, ThreatZoneNone},
		{"past corridor and landing", world.Position{X: 151, Y: 100}, false, ThreatZoneNone},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			state := phase17ThreatState(world.Monster{NPCID: world.ArcaneSpecter, UnitID: 1, Position: tc.position})
			got := assessThreats(state, phase17ThreatProgress(), []uint32{world.ArcaneSpecter}, phase17ThreatConfig())
			if got.RouteTargetFound != tc.found || got.RouteZone != tc.zone {
				t.Fatalf("assessment = %+v", got)
			}
		})
	}
}

func TestAssessThreatsCoverageUsesStrictRadiusAndTransitionHull(t *testing.T) {
	state := phase17ThreatState()
	state.MonsterCoverage = world.MonsterCoverage{EligibleMonsterCount: 513, MonstersTruncated: true, MonsterCoverageRadiusTiles: 50}
	atBoundary := assessThreats(state, phase17ThreatProgress(), []uint32{world.ArcaneSpecter}, phase17ThreatConfig())
	if atBoundary.CoverageComplete {
		t.Fatal("equal retained radius was treated as complete")
	}
	state.MonsterCoverage.MonsterCoverageRadiusTiles = 50.01
	overBoundary := assessThreats(state, phase17ThreatProgress(), []uint32{world.ArcaneSpecter}, phase17ThreatConfig())
	if !overBoundary.CoverageComplete {
		t.Fatal("strictly larger retained radius was not complete")
	}

	progress := phase17ThreatProgress()
	progress.TargetAvailable = false
	progress.Mode = RouteProgressTransition
	state.MonsterCoverage.MonsterCoverageRadiusTiles = 18
	transition := assessThreats(state, progress, []uint32{world.ArcaneSpecter}, phase17ThreatConfig())
	if transition.RequiredCoverageTiles != 18 || transition.CoverageComplete {
		t.Fatalf("transition coverage = %+v", transition)
	}
	state.MonsterCoverage.MonstersTruncated = false
	if got := assessThreats(state, progress, nil, phase17ThreatConfig()); !got.CoverageComplete || got.RouteTargetFound {
		t.Fatalf("complete no-threat assessment = %+v", got)
	}
}

func BenchmarkThreatAssessment512(b *testing.B) {
	monsters := make([]world.Monster, 512)
	for i := range monsters {
		monsters[i] = world.Monster{
			NPCID:    []uint32{world.ArcaneSpecter, world.ArcaneHellClan, world.ArcaneGhoulLord}[i%3],
			UnitID:   uint32(i + 1),
			Position: world.Position{X: uint32(80 + i%80), Y: uint32(80 + (i*7)%80)},
		}
	}
	state := phase17ThreatState(monsters...)
	state.MonsterCoverage = world.MonsterCoverage{EligibleMonsterCount: 512}
	progress := phase17ThreatProgress()
	allowed := []uint32{world.ArcaneSpecter, world.ArcaneHellClan, world.ArcaneGhoulLord}
	cfg := phase17ThreatConfig()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchmarkThreatAssessment = assessThreats(state, progress, allowed, cfg)
	}
}
