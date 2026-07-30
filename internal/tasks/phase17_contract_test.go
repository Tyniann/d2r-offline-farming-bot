package tasks_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/memory"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/profile"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/tasks"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

func TestPhase17RouteCombatDefaultsMatchHardenedContract(t *testing.T) {
	if config.DefaultRouteCombatImmediateRadiusTiles != 18 ||
		config.DefaultRouteCombatCorridorWidthTiles != 7 ||
		config.DefaultRouteCombatLandingRadiusTiles != 10 ||
		config.DefaultRouteCombatAttackDistanceTiles != 30 ||
		config.DefaultRouteCombatNoProgressTimeoutMs != 12000 ||
		config.DefaultRouteCombatTeleportManaReservePercent != 20 ||
		config.DefaultRouteCombatResumeManaPercent != 35 ||
		config.DefaultRouteCombatEmergencyManaPercent != 10 ||
		config.DefaultRouteCombatManaRecoveryTimeoutMs != 5000 {
		t.Fatal("Phase-17-Route-Combat-Defaults sind vom gehärteten Vertrag abgewichen")
	}
	if memory.Phase17MaxRuntimeMonsters != 512 || tasks.Phase17StableClearSnapshots != 3 {
		t.Fatalf("compile-nahe Grenzen: monsters=%d stable=%d", memory.Phase17MaxRuntimeMonsters, tasks.Phase17StableClearSnapshots)
	}
}

func TestPhase17PureContractsPreservePresenceSnapshotAndRecoveryTarget(t *testing.T) {
	disabled := false
	cfg := config.RouteCombatConfig{Enabled: &disabled}
	if cfg.Enabled == nil || *cfg.Enabled {
		t.Fatalf("explizites false ging verloren: %+v", cfg)
	}

	at := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	target := world.Position{X: 25446, Y: 5426}
	progress := tasks.RouteProgress{
		RouteID:           "summoner-mrbones-e59fe08f23",
		SegmentID:         "arcane-sanctuary",
		SegmentIndex:      0,
		PointIndex:        1,
		PreviousConfirmed: world.Position{X: 25448, Y: 5448},
		MovementTarget:    target,
		TargetAvailable:   true,
		Mode:              tasks.RouteProgressRecovery,
	}
	assessment := tasks.ThreatAssessment{SnapshotAt: at, RequiredCoverageTiles: 40, CoverageComplete: true}
	resource := profile.ResourceContext{MobilityCritical: true, Threatened: true, EmergencyMana: true}
	rawCoverage := memory.MonsterCoverage{EligibleMonsterCount: 513, MonstersTruncated: true, MonsterCoverageRadiusTiles: 42}
	worldCoverage := world.MonsterCoverage(rawCoverage)

	if progress.Mode != tasks.RouteProgressRecovery || progress.MovementTarget != target || assessment.SnapshotAt != at {
		t.Fatalf("Route-/Snapshot-Bindung ging verloren: progress=%+v assessment=%+v", progress, assessment)
	}
	if !resource.MobilityCritical || !resource.Threatened || !resource.EmergencyMana {
		t.Fatalf("ResourceContext=%+v", resource)
	}
	if !reflect.DeepEqual(worldCoverage, world.MonsterCoverage{EligibleMonsterCount: 513, MonstersTruncated: true, MonsterCoverageRadiusTiles: 42}) {
		t.Fatalf("Coverage-Projektion=%+v", worldCoverage)
	}
}

func TestPhase17StatesReasonsOwnershipAndNonGoalsAreComplete(t *testing.T) {
	states := []tasks.RouteThreatState{
		tasks.RouteThreatMoving,
		tasks.RouteThreatClearing,
		tasks.RouteThreatDensityRelief,
		tasks.RouteThreatManaRecovery,
		tasks.RouteThreatRecoveryGuard,
	}
	if want := []tasks.RouteThreatState{"route_moving", "route_clearing", "density_relief", "route_mana_recovery", "route_recovery_guard"}; !reflect.DeepEqual(states, want) {
		t.Fatalf("states=%v", states)
	}

	reasons := tasks.Phase17RouteThreatReasons()
	if len(reasons) != 5 {
		t.Fatalf("reasons=%v", reasons)
	}
	seen := make(map[tasks.RouteThreatReason]struct{}, len(reasons))
	for _, reason := range reasons {
		if reason == "" {
			t.Fatal("leerer Reason-Code")
		}
		if _, duplicate := seen[reason]; duplicate {
			t.Fatalf("doppelter Reason-Code %q", reason)
		}
		seen[reason] = struct{}{}
	}
	if owners := tasks.Phase17ContractOwners(); len(owners) != 9 || owners[0].Owner != "internal/memory to internal/world" || owners[len(owners)-1].Owner != "internal/profile" {
		t.Fatalf("owners=%+v", owners)
	}
	if nonGoals := tasks.Phase17NonGoals(); len(nonGoals) != 9 {
		t.Fatalf("non-goals=%v", nonGoals)
	}
}
