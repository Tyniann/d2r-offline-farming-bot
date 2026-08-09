package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/pathing"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/tasks"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

func newRecordingTestStore(t *testing.T) (*CandidateStore, *config.Config) {
	t.Helper()
	root := t.TempDir()
	cfg := &config.Config{LoadedFrom: filepath.Join(root, "config.yaml"), Routes: config.RoutesConfig{FarmingRoot: "routes/farming", CandidateRoot: "routes/candidates"}}
	store, err := NewCandidateStore(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return store, cfg
}

func recordingState(area world.AreaID, x, y uint32) world.State {
	return world.State{At: time.Now(), Valid: true, Phase: world.GamePhaseInGame, Area: world.LookupArea(area), Player: world.Player{Position: world.Position{X: x, Y: y}}, Identity: world.GameIdentity{Valid: true, CharacterName: "MrBones", Class: world.CharacterClassNecromancer, MapSeed: 42}, Objects: []world.Object{{ID: 119, Kind: world.ObjectKindWaypoint, Position: world.Position{X: 90, Y: 90}}}}
}

func validRecordingPreflight() RecordingPreflight {
	return RecordingPreflight{RunID: tasks.RunIDCountess, Character: "MrBones", ExpectedClass: "necromancer", Difficulty: pathing.RouteDifficultyNightmare, GameVersion: "3.2.92777", SourceCatalogRevision: 4, SourceAssignmentRevision: 7, WaypointContextConfirmed: true, BlockingUIClosed: true, D2RFocused: true, InputOwnerAvailable: true}
}

func TestGuidedRecordingStartRequiresConfiguredWaypointProximity(t *testing.T) {
	definition, ok := tasks.DefaultRunRegistry().Definition(tasks.RunIDCountess)
	if !ok {
		t.Fatal("countess definition missing")
	}
	state := recordingState(world.BlackMarsh, 100, 100)
	contract := definition.Recording
	state.UI.WaypointOpen = true
	if !guidedRecordingStartReady(state, contract, 15) {
		t.Fatal("waypoint inside configured proximity was rejected because of stale post-transfer UI flag")
	}
	if guidedRecordingStartReady(state, contract, 5) {
		t.Fatal("waypoint outside configured proximity was accepted")
	}
	state.UI.InventoryOpen = true
	if guidedRecordingStartReady(state, contract, 15) {
		t.Fatal("actually blocking inventory UI was accepted")
	}
}

func TestRecordingStartToleranceUsesPortalConfigForCowSweep(t *testing.T) {
	definition, ok := tasks.DefaultRunRegistry().Definition(tasks.RunIDCows)
	if !ok {
		t.Fatal("cow definition missing")
	}
	contract, ok := definition.RecordingForRole(pathing.RouteRoleCowSweep)
	if !ok {
		t.Fatal("cow sweep recording contract missing")
	}
	cfg := &config.Config{Pathing: config.PathingConfig{
		Waypoint:   config.PathingWaypointConfig{MaxClickDistance: 5},
		TownPortal: config.PathingTownPortalConfig{MaxClickDistance: 17},
	}}
	if got := recordingStartTolerance(cfg, contract); got != 17 {
		t.Fatalf("cow sweep start tolerance = %v, want portal tolerance 17", got)
	}
}

func TestCowRecordingContractsFreezeRoleBoundCandidates(t *testing.T) {
	for _, role := range []pathing.RouteRole{pathing.RouteRoleLegAcquisition, pathing.RouteRoleCowSweep} {
		t.Run(string(role), func(t *testing.T) {
			store, _ := newRecordingTestStore(t)
			coordinator, coordinatorErr := NewRecordingCoordinator(store, tasks.DefaultRunRegistry())
			if coordinatorErr != nil {
				t.Fatal(coordinatorErr)
			}
			request := RecordingPreflight{RunID: tasks.RunIDCows, RouteRole: role, Character: "MrBones", ExpectedClass: "necromancer", ProfileID: "necro_bone_spear", Difficulty: pathing.RouteDifficultyHell, GameVersion: "3.2.92777", SourceCatalogRevision: 1, SourceAssignmentRevision: 1, WaypointContextConfirmed: true, BlockingUIClosed: true, D2RFocused: true, InputOwnerAvailable: true}
			var start world.State
			if role == pathing.RouteRoleLegAcquisition {
				start = recordingState(world.StonyField, 100, 100)
			} else {
				start = recordingState(world.MooMooFarm, 100, 100)
				start.Objects = []world.Object{{Kind: world.ObjectKindPermanentPortal, UnitID: 60, Position: world.Position{X: 101, Y: 100}}}
			}
			if err := coordinator.Start(request, start); err != nil {
				t.Fatal(err)
			}
			var evidence RecordingTerminalEvidence
			if role == pathing.RouteRoleLegAcquisition {
				atPortal := recordingState(world.StonyField, 110, 100)
				atPortal.Objects = []world.Object{{Kind: world.ObjectKindPermanentPortal, UnitID: 61, Position: world.Position{X: 111, Y: 100}}}
				if err := coordinator.Tick(context.Background(), atPortal); err != nil {
					t.Fatal(err)
				}
				if err := coordinator.Tick(context.Background(), recordingState(world.Tristram, 200, 200)); err != nil {
					t.Fatal(err)
				}
				terminal := recordingState(world.Tristram, 210, 200)
				wirt := world.Object{Kind: world.ObjectKindWirtsBody, UnitID: 268, Position: world.Position{X: 212, Y: 200}}
				terminal.Objects = []world.Object{wirt}
				if err := coordinator.Tick(context.Background(), terminal); err != nil {
					t.Fatal(err)
				}
				evidence = RecordingTerminalEvidence{World: terminal, Object: &wirt}
			} else {
				terminal := recordingState(world.MooMooFarm, 112, 100)
				if err := coordinator.Tick(context.Background(), terminal); err != nil {
					t.Fatal(err)
				}
				evidence = RecordingTerminalEvidence{World: terminal}
			}
			candidate, candidateErr := coordinator.Finish(evidence)
			if candidateErr != nil || candidate.State != RouteCandidateValidated || candidate.RouteRole != role || len(candidate.ImmutableRouteSHA256) != 64 {
				t.Fatalf("candidate = %+v, err=%v", candidate, candidateErr)
			}
			if err := coordinator.CompleteSafetyReturn(nil); err != nil {
				t.Fatal(err)
			}
			_, route, err := store.Load(candidate.CandidateID)
			if err != nil || route.Binding.RouteRole != role || route.Binding.ProfileID != "" {
				t.Fatalf("route = %+v, err=%v", route.Binding, err)
			}
		})
	}
}

func recordCountessPath(t *testing.T, coordinator *RecordingCoordinator) world.State {
	t.Helper()
	x := uint32(100)
	for _, area := range []world.AreaID{world.BlackMarsh, world.ForgottenTower, world.TowerCellarLevel1, world.TowerCellarLevel2, world.TowerCellarLevel3, world.TowerCellarLevel4, world.TowerCellarLevel5} {
		if err := coordinator.Tick(context.Background(), recordingState(area, x+5, x+5)); err != nil {
			t.Fatal(err)
		}
		x += 10
		if area != world.TowerCellarLevel5 {
			if err := coordinator.Tick(context.Background(), recordingState(nextCountessArea(area), x, x)); err != nil {
				t.Fatal(err)
			}
		}
	}
	return recordingState(world.TowerCellarLevel5, x+5, x+5)
}

func nextCountessArea(area world.AreaID) world.AreaID {
	areas := []world.AreaID{world.BlackMarsh, world.ForgottenTower, world.TowerCellarLevel1, world.TowerCellarLevel2, world.TowerCellarLevel3, world.TowerCellarLevel4, world.TowerCellarLevel5}
	for index := range len(areas) - 1 {
		if areas[index] == area {
			return areas[index+1]
		}
	}
	return area
}

func TestRecordingCoordinatorFreezeValidateReturnAndIdempotentFinish(t *testing.T) {
	store, cfg := newRecordingTestStore(t)
	coordinator, _ := NewRecordingCoordinator(store, tasks.DefaultRunRegistry())
	start := recordingState(world.BlackMarsh, 100, 100)
	if err := coordinator.Start(validRecordingPreflight(), start); err != nil {
		t.Fatal(err)
	}
	terminal := recordCountessPath(t, coordinator)
	boss := world.Monster{NPCID: world.DarkStalker, UnitID: 9, MonsterTypeFlag: world.SuperUniqueMonsterFlag, Position: world.Position{X: terminal.Player.Position.X + 20, Y: terminal.Player.Position.Y}}
	first, err := coordinator.Finish(RecordingTerminalEvidence{World: terminal, Boss: &boss})
	if err != nil {
		t.Fatal(err)
	}
	second, err := coordinator.Finish(RecordingTerminalEvidence{})
	if err != nil || second.CandidateID != first.CandidateID {
		t.Fatalf("idempotent finish=%+v err=%v", second, err)
	}
	if snapshot := coordinator.Snapshot(); snapshot.State != RouteWorkflowReturningViaPortal || snapshot.CandidateID == "" {
		t.Fatalf("freeze-before-TP snapshot=%+v", snapshot)
	}
	if _, _, loadErr := store.Load(first.CandidateID); loadErr != nil {
		t.Fatalf("frozen candidate missing before TP: %v", loadErr)
	}
	if returnErr := coordinator.CompleteSafetyReturn(nil); returnErr != nil {
		t.Fatal(returnErr)
	}
	if coordinator.Snapshot().State != RouteWorkflowCandidateReady {
		t.Fatalf("state=%s", coordinator.Snapshot().State)
	}
	farmingEntries, err := os.ReadDir(cfg.ResolvePath(cfg.Routes.FarmingRoot))
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if len(farmingEntries) != 0 {
		t.Fatalf("recording wrote into Farming root: %v", farmingEntries)
	}
}

func TestRecordingCoordinatorTerminalDiagnosticsAndDistanceBoundary(t *testing.T) {
	tests := []struct {
		name     string
		mutate   func(world.State, *world.Monster) RecordingTerminalEvidence
		want     RouteReason
		accepted bool
	}{
		{"missing boss", func(state world.State, _ *world.Monster) RecordingTerminalEvidence {
			return RecordingTerminalEvidence{World: state}
		}, RouteReasonRecordingBossMissing, false},
		{"wrong boss", func(state world.State, boss *world.Monster) RecordingTerminalEvidence {
			boss.NPCID = 999
			return RecordingTerminalEvidence{World: state, Boss: boss}
		}, RouteReasonRecordingBossMissing, false},
		{"dead boss", func(state world.State, boss *world.Monster) RecordingTerminalEvidence {
			return RecordingTerminalEvidence{World: state, Boss: boss, BossDead: true}
		}, RouteReasonRecordingBossDead, false},
		{"wrong area", func(state world.State, boss *world.Monster) RecordingTerminalEvidence {
			state.Area = world.LookupArea(world.TowerCellarLevel4)
			return RecordingTerminalEvidence{World: state, Boss: boss}
		}, RouteReasonRecordingTerminalAreaMismatch, false},
		{"maximum distance", func(state world.State, boss *world.Monster) RecordingTerminalEvidence {
			boss.Position = world.Position{X: state.Player.Position.X + 80, Y: state.Player.Position.Y}
			return RecordingTerminalEvidence{World: state, Boss: boss}
		}, "", true},
		{"too far", func(state world.State, boss *world.Monster) RecordingTerminalEvidence {
			boss.Position = world.Position{X: state.Player.Position.X + 81, Y: state.Player.Position.Y}
			return RecordingTerminalEvidence{World: state, Boss: boss}
		}, RouteReasonRecordingEndpointTooFar, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store, _ := newRecordingTestStore(t)
			coordinator, _ := NewRecordingCoordinator(store, tasks.DefaultRunRegistry())
			if err := coordinator.Start(validRecordingPreflight(), recordingState(world.BlackMarsh, 100, 100)); err != nil {
				t.Fatal(err)
			}
			terminal := recordCountessPath(t, coordinator)
			boss := &world.Monster{NPCID: world.DarkStalker, MonsterTypeFlag: world.SuperUniqueMonsterFlag, Position: world.Position{X: terminal.Player.Position.X + 20, Y: terminal.Player.Position.Y}}
			candidate, err := coordinator.Finish(test.mutate(terminal, boss))
			if err != nil {
				t.Fatal(err)
			}
			if test.accepted && candidate.State != RouteCandidateValidated || !test.accepted && (candidate.State != RouteCandidateFailed || candidate.FailureReason != test.want) {
				t.Fatalf("candidate=%+v", candidate)
			}
			_ = coordinator.CompleteSafetyReturn(nil)
		})
	}
}

func TestRecordingCoordinatorEmergencyCancellationConflictAndTPFailure(t *testing.T) {
	store, _ := newRecordingTestStore(t)
	first, _ := NewRecordingCoordinator(store, tasks.DefaultRunRegistry())
	second, _ := NewRecordingCoordinator(store, tasks.DefaultRunRegistry())
	if err := first.Start(validRecordingPreflight(), recordingState(world.BlackMarsh, 100, 100)); err != nil {
		t.Fatal(err)
	}
	if err := second.Start(validRecordingPreflight(), recordingState(world.BlackMarsh, 100, 100)); err == nil || !strings.Contains(err.Error(), string(RouteReasonRecordingConflict)) {
		t.Fatalf("conflict=%v", err)
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if err := first.Tick(cancelled, recordingState(world.BlackMarsh, 105, 105)); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel=%v", err)
	}
	if first.Snapshot().State != RouteWorkflowEmergencyCancelled {
		t.Fatalf("snapshot=%+v", first.Snapshot())
	}
	if list, err := store.List(); err != nil || len(list) != 0 {
		t.Fatalf("cancel candidates=%v err=%v", list, err)
	}

	third, _ := NewRecordingCoordinator(store, tasks.DefaultRunRegistry())
	if err := third.Start(validRecordingPreflight(), recordingState(world.BlackMarsh, 100, 100)); err != nil {
		t.Fatal(err)
	}
	terminal := recordCountessPath(t, third)
	boss := world.Monster{NPCID: world.DarkStalker, MonsterTypeFlag: world.SuperUniqueMonsterFlag, Position: terminal.Player.Position}
	candidate, err := third.Finish(RecordingTerminalEvidence{World: terminal, Boss: &boss})
	if err != nil {
		t.Fatal(err)
	}
	if returnErr := third.CompleteSafetyReturn(errors.New("portal missing")); returnErr == nil || third.Snapshot().Reason != RouteReasonSafetyReturnFailed {
		t.Fatalf("TP failure=%v snapshot=%+v", returnErr, third.Snapshot())
	}
	retained, _, err := store.Load(candidate.CandidateID)
	if err != nil {
		t.Fatalf("candidate lost after TP failure: %v", err)
	}
	if retained.State != RouteCandidateFailed || retained.FailureReason != RouteReasonSafetyReturnFailed {
		t.Fatalf("unsafe candidate remained testable after TP failure: %+v", retained)
	}
}

func TestRecordingCoordinatorRejectsForbiddenCommandsAndInvalidPreflight(t *testing.T) {
	store, _ := newRecordingTestStore(t)
	coordinator, _ := NewRecordingCoordinator(store, tasks.DefaultRunRegistry())
	if _, err := coordinator.Finish(RecordingTerminalEvidence{}); err == nil {
		t.Fatal("idle Finish accepted")
	}
	if err := coordinator.CompleteSafetyReturn(nil); err == nil {
		t.Fatal("idle safety return accepted")
	}
	if err := coordinator.Tick(context.Background(), recordingState(world.BlackMarsh, 1, 1)); err == nil {
		t.Fatal("idle Tick accepted")
	}
	request := validRecordingPreflight()
	request.D2RFocused = false
	if err := coordinator.Start(request, recordingState(world.BlackMarsh, 100, 100)); err == nil || coordinator.Snapshot().Reason != RouteReasonRecordingPreflightFailed {
		t.Fatalf("invalid preflight error=%v snapshot=%+v", err, coordinator.Snapshot())
	}
	other, _ := NewRecordingCoordinator(store, tasks.DefaultRunRegistry())
	if err := other.Start(validRecordingPreflight(), recordingState(world.BlackMarsh, 100, 100)); err != nil {
		t.Fatalf("failed preflight leaked workflow lock: %v", err)
	}
	other.EmergencyCancel()
}

func TestRecordingCoordinatorRejectsWrongMemoryConfirmedClass(t *testing.T) {
	store, _ := newRecordingTestStore(t)
	coordinator, _ := NewRecordingCoordinator(store, tasks.DefaultRunRegistry())
	state := recordingState(world.BlackMarsh, 100, 100)
	state.Identity.Class = world.CharacterClassSorceress
	if err := coordinator.Start(validRecordingPreflight(), state); err == nil || coordinator.Snapshot().Reason != RouteReasonRecordingStartAreaMismatch {
		t.Fatalf("class mismatch error=%v snapshot=%+v", err, coordinator.Snapshot())
	}
}

func TestRecordingCoordinatorEmergencyAfterFreezeRetainsCandidate(t *testing.T) {
	store, _ := newRecordingTestStore(t)
	coordinator, _ := NewRecordingCoordinator(store, tasks.DefaultRunRegistry())
	if err := coordinator.Start(validRecordingPreflight(), recordingState(world.BlackMarsh, 100, 100)); err != nil {
		t.Fatal(err)
	}
	terminal := recordCountessPath(t, coordinator)
	boss := world.Monster{NPCID: world.DarkStalker, MonsterTypeFlag: world.SuperUniqueMonsterFlag, Position: terminal.Player.Position}
	candidate, err := coordinator.Finish(RecordingTerminalEvidence{World: terminal, Boss: &boss})
	if err != nil {
		t.Fatal(err)
	}
	coordinator.EmergencyCancel()
	if coordinator.Snapshot().State != RouteWorkflowEmergencyCancelled {
		t.Fatalf("snapshot=%+v", coordinator.Snapshot())
	}
	if _, _, err := store.Load(candidate.CandidateID); err != nil {
		t.Fatalf("post-freeze F11 removed candidate: %v", err)
	}
}

func TestRecordingCoordinatorTimesOutFailClosed(t *testing.T) {
	store, _ := newRecordingTestStore(t)
	coordinator, _ := NewRecordingCoordinator(store, tasks.DefaultRunRegistry())
	start := recordingState(world.BlackMarsh, 100, 100)
	if err := coordinator.Start(validRecordingPreflight(), start); err != nil {
		t.Fatal(err)
	}
	late := recordingState(world.BlackMarsh, 110, 110)
	late.At = start.At.Add(guidedRecordingTimeout + time.Second)
	if err := coordinator.Tick(context.Background(), late); err == nil || coordinator.Snapshot().Reason != RouteReasonRecordingTimeout {
		t.Fatalf("timeout error=%v snapshot=%+v", err, coordinator.Snapshot())
	}
}

func TestCandidateStoreDetectsImmutableRouteTampering(t *testing.T) {
	store, cfg := newRecordingTestStore(t)
	coordinator, _ := NewRecordingCoordinator(store, tasks.DefaultRunRegistry())
	if err := coordinator.Start(validRecordingPreflight(), recordingState(world.BlackMarsh, 100, 100)); err != nil {
		t.Fatal(err)
	}
	terminal := recordCountessPath(t, coordinator)
	boss := world.Monster{NPCID: world.DarkStalker, MonsterTypeFlag: world.SuperUniqueMonsterFlag, Position: terminal.Player.Position}
	candidate, err := coordinator.Finish(RecordingTerminalEvidence{World: terminal, Boss: &boss})
	if err != nil {
		t.Fatal(err)
	}
	routePath := filepath.Join(cfg.ResolvePath(cfg.Routes.CandidateRoot), candidate.CandidateID, "route.yaml")
	file, err := os.OpenFile(routePath, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = file.WriteString("\n# tampered\n")
	_ = file.Close()
	if _, _, err := store.Load(candidate.CandidateID); err == nil || !strings.Contains(err.Error(), string(RouteReasonCandidateChanged)) {
		t.Fatalf("tamper error=%v", err)
	}
	coordinator.EmergencyCancel()
}
