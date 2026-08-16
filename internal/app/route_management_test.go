package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/pathing"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/tasks"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/town"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
	"gopkg.in/yaml.v3"
)

func managementTestConfig(t *testing.T) *config.Config {
	t.Helper()
	root := t.TempDir()
	return &config.Config{LoadedFrom: filepath.Join(root, "config.yaml"), Routes: config.RoutesConfig{FarmingRoot: "routes/farming", CandidateRoot: "routes/candidates", LifecycleFile: "lifecycle.yaml", AssignmentsFile: "assignments.yaml", RecoveryFile: "recovery.yaml"}, Session: config.SessionConfig{Character: "MrBones", Difficulty: "nightmare"}}
}

func freezeManagementCandidate(t *testing.T, cfg *config.Config) RouteCandidate {
	t.Helper()
	lifecycle, _ := NewRouteLifecycleStore(cfg)
	manifest, catalog, err := lifecycle.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	assignments, _ := NewRouteAssignmentStore(cfg)
	assignment, err := assignments.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	store, _ := NewCandidateStore(cfg)
	seed := uint32(42)
	route := pathing.Route{Version: 1, ID: "candidate-route", Name: "Kandidat", Kind: pathing.RouteKindNavigation, Binding: pathing.RouteBinding{CharacterName: "MrBones", CharacterClass: "necromancer", Difficulty: pathing.RouteDifficultyNightmare, MapSeed: &seed, GameVersion: "3.2.92777", LayoutFingerprint: pathing.RouteLayoutFingerprint{Version: 1, AreaID: world.BlackMarsh, AnchorCount: 1, Hash: strings.Repeat("a", 64)}}, Recording: pathing.RouteRecording{RecordedAt: time.Now().UTC(), SampleDistanceTiles: 4}, Playback: pathing.RoutePlayback{WaypointToleranceTiles: 3, MaxDriftTiles: 8, MaxLocalCorrections: 2, SegmentTimeoutMs: 30000, TransitionTimeoutMs: 10000}, Segments: []pathing.RouteSegment{{ID: "black-marsh", FromAreaID: world.BlackMarsh, ToAreaID: world.TowerCellarLevel5, Movement: pathing.RouteMovementTeleport, Points: []pathing.RoutePoint{{X: 100, Y: 100}, {X: 110, Y: 110}}, Transition: pathing.RouteTransition{Type: "entrance", EntranceKind: "tower_cellar_down"}}, {ID: "tower-cellar-level-5", FromAreaID: world.TowerCellarLevel5, ToAreaID: world.TowerCellarLevel5, Movement: pathing.RouteMovementTeleport, Points: []pathing.RoutePoint{{X: 120, Y: 120}, {X: 125, Y: 125}}, Transition: pathing.RouteTransition{Type: "terminal"}}}}
	candidate, err := store.Freeze(route, RouteCandidate{RunID: tasks.RunIDCountess, Character: "MrBones", Difficulty: "nightmare", GameVersion: "3.2.92777", State: RouteCandidateRecorded, MeasuredBossDistance: 20, SourceCatalogRevision: catalog.Revision, SourceAssignmentRevision: assignment.Revision, SourceAssignedRouteID: assignedRoute(assignment, "mrbones", tasks.RunIDCountess, ""), CreatedAt: time.Now().UTC()})
	if err != nil {
		t.Fatal(err)
	}
	candidate, err = store.UpdateState(candidate.CandidateID, RouteCandidateValidated, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Revision != catalog.Revision {
		t.Fatalf("revision mismatch")
	}
	return candidate
}

func freezeTestPassedCandidate(t *testing.T, cfg *config.Config) RouteCandidate {
	t.Helper()
	candidate := freezeManagementCandidate(t, cfg)
	store, _ := NewCandidateStore(cfg)
	now := time.Now().UTC()
	passed, err := store.UpdateState(candidate.CandidateID, RouteCandidateTestPassed, "", &now)
	if err != nil {
		t.Fatal(err)
	}
	return passed
}

func freezeCowManagementCandidate(t *testing.T, cfg *config.Config, role pathing.RouteRole) RouteCandidate {
	t.Helper()
	lifecycle, _ := NewRouteLifecycleStore(cfg)
	_, catalog, err := lifecycle.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	assignments, _ := NewRouteAssignmentStore(cfg)
	assignment, err := assignments.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	seed := uint32(42)
	startArea, hash := world.StonyField, strings.Repeat("c", 64)
	segments := []pathing.RouteSegment{
		{ID: "stony-field", FromAreaID: world.StonyField, ToAreaID: world.Tristram, Movement: pathing.RouteMovementTeleport, Points: []pathing.RoutePoint{{X: 100, Y: 100}, {X: 110, Y: 100}}, Transition: pathing.RouteTransition{Type: "object_portal", ObjectKind: world.ObjectKindPermanentPortal, ExpectedToArea: world.Tristram}},
		{ID: "tristram", FromAreaID: world.Tristram, ToAreaID: world.Tristram, Movement: pathing.RouteMovementTeleport, Points: []pathing.RoutePoint{{X: 200, Y: 200}, {X: 210, Y: 200}}, Transition: pathing.RouteTransition{Type: "terminal"}},
	}
	if role == pathing.RouteRoleCowSweep {
		startArea, hash = world.MooMooFarm, strings.Repeat("d", 64)
		segments = []pathing.RouteSegment{{ID: "moo-moo-farm", FromAreaID: world.MooMooFarm, ToAreaID: world.MooMooFarm, Movement: pathing.RouteMovementTeleport, Points: []pathing.RoutePoint{{X: 300, Y: 300}, {X: 310, Y: 300}}, Transition: pathing.RouteTransition{Type: "terminal"}}}
	}
	routeID := "cow-candidate-" + strings.ReplaceAll(string(role), "_", "-")
	route := pathing.Route{Version: pathing.RouteVersion, ID: routeID, Name: "Cow Kandidat", Kind: pathing.RouteKindNavigation, Binding: pathing.RouteBinding{RouteRole: role, CharacterName: "MrBones", CharacterClass: "necromancer", ProfileID: "necro_bone_spear", Difficulty: pathing.RouteDifficultyNightmare, MapSeed: &seed, GameVersion: "3.2.92777", LayoutFingerprint: pathing.RouteLayoutFingerprint{Version: 1, AreaID: startArea, AnchorCount: 1, Hash: hash}}, Recording: pathing.RouteRecording{RecordedAt: time.Now().UTC(), SampleDistanceTiles: 4}, Playback: pathing.RoutePlayback{WaypointToleranceTiles: 3, MaxDriftTiles: 8, MaxLocalCorrections: 2, SegmentTimeoutMs: 30000, TransitionTimeoutMs: 10000}, Segments: segments}
	store, _ := NewCandidateStore(cfg)
	candidate, err := store.Freeze(route, RouteCandidate{RunID: tasks.RunIDCows, RouteRole: role, Character: "MrBones", Difficulty: "nightmare", GameVersion: "3.2.92777", State: RouteCandidateRecorded, SourceCatalogRevision: catalog.Revision, SourceAssignmentRevision: assignment.Revision, SourceAssignedRouteID: assignment.RouteSets["mrbones"][tasks.RunIDCows][role], CreatedAt: time.Now().UTC()})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	candidate, err = store.UpdateState(candidate.CandidateID, RouteCandidateTestPassed, "", &now)
	if err != nil {
		t.Fatal(err)
	}
	return candidate
}

func TestCowCandidatesPublishBothRolesWithoutInvalidatingSiblingCandidate(t *testing.T) {
	cfg := managementTestConfig(t)
	leg := freezeCowManagementCandidate(t, cfg, pathing.RouteRoleLegAcquisition)
	sweep := freezeCowManagementCandidate(t, cfg, pathing.RouteRoleCowSweep)
	service, serviceErr := NewRouteManagementService(cfg, RouteManagementHooks{})
	if serviceErr != nil {
		t.Fatal(serviceErr)
	}
	for _, candidate := range []RouteCandidate{leg, sweep} {
		preview, previewErr := service.PreviewCandidate(candidate.CandidateID)
		if previewErr != nil {
			t.Fatalf("preview %s: %v", candidate.RouteRole, previewErr)
		}
		if preview.RouteRole != candidate.RouteRole || preview.Operation != RouteMutationPublish {
			t.Fatalf("preview = %+v", preview)
		}
		if err := service.Confirm(RouteMutationConfirm{Token: preview.Token}); err != nil {
			t.Fatalf("publish %s: %v", candidate.RouteRole, err)
		}
	}
	manifest, err := service.assignments.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	roles := manifest.RouteSets["mrbones"][tasks.RunIDCows]
	if roles[pathing.RouteRoleLegAcquisition] == "" || roles[pathing.RouteRoleCowSweep] == "" || roles[pathing.RouteRoleLegAcquisition] == roles[pathing.RouteRoleCowSweep] {
		t.Fatalf("cow role assignments = %+v", roles)
	}
}

func TestPreviewCandidateSurvivesUnrelatedCharacterConfirmation(t *testing.T) {
	cfg := managementTestConfig(t)
	candidate := freezeTestPassedCandidate(t, cfg)
	lifecycle, err := NewRouteLifecycleStore(cfg)
	if err != nil {
		t.Fatal(err)
	}
	preview, err := lifecycle.Preview("MrHammer", "hell")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = lifecycle.Confirm(preview, time.Now()); err != nil {
		t.Fatal(err)
	}
	service, err := NewRouteManagementService(cfg, RouteManagementHooks{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.PreviewCandidate(candidate.CandidateID); err != nil {
		t.Fatalf("publish preview after unrelated character confirm: %v", err)
	}
}

func freezeMephistoManagementCandidate(t *testing.T, cfg *config.Config) RouteCandidate {
	t.Helper()
	lifecycle, err := NewRouteLifecycleStore(cfg)
	if err != nil {
		t.Fatal(err)
	}
	_, catalog, err := lifecycle.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	assignments, _ := NewRouteAssignmentStore(cfg)
	assignment, err := assignments.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	store, _ := NewCandidateStore(cfg)
	seed := uint32(43)
	route := pathing.Route{
		Version: 1, ID: "mephisto-candidate-route", Name: "Mephisto-Kandidat", Kind: pathing.RouteKindNavigation,
		Binding: pathing.RouteBinding{
			CharacterName: "MrBones", CharacterClass: "necromancer", Difficulty: pathing.RouteDifficultyNightmare,
			MapSeed: &seed, GameVersion: "3.2.92777",
			LayoutFingerprint: pathing.RouteLayoutFingerprint{Version: 1, AreaID: world.DuranceOfHateLevel2, AnchorCount: 1, Hash: strings.Repeat("b", 64)},
		},
		Recording: pathing.RouteRecording{RecordedAt: time.Now().UTC(), SampleDistanceTiles: 4},
		Playback:  pathing.RoutePlayback{WaypointToleranceTiles: 3, MaxDriftTiles: 8, MaxLocalCorrections: 2, SegmentTimeoutMs: 30000, TransitionTimeoutMs: 10000},
		Segments: []pathing.RouteSegment{
			{
				ID: "durance-level-2", FromAreaID: world.DuranceOfHateLevel2, ToAreaID: world.DuranceOfHateLevel3,
				Movement: pathing.RouteMovementTeleport, Points: []pathing.RoutePoint{{X: 100, Y: 100}, {X: 110, Y: 110}},
				Transition: pathing.RouteTransition{Type: "entrance", EntranceKind: "durance_down"},
			},
			{
				ID: "durance-level-3", FromAreaID: world.DuranceOfHateLevel3, ToAreaID: world.DuranceOfHateLevel3,
				Movement: pathing.RouteMovementTeleport, Points: []pathing.RoutePoint{{X: 120, Y: 120}, {X: 125, Y: 125}},
				Transition: pathing.RouteTransition{Type: "terminal"},
			},
		},
	}
	candidate, err := store.Freeze(route, RouteCandidate{
		RunID: tasks.RunIDMephisto, Character: "MrBones", Difficulty: "nightmare", GameVersion: "3.2.92777",
		State: RouteCandidateRecorded, MeasuredBossDistance: 20,
		SourceCatalogRevision: catalog.Revision, SourceAssignmentRevision: assignment.Revision,
		SourceAssignedRouteID: assignedRoute(assignment, "mrbones", tasks.RunIDMephisto, ""), CreatedAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatal(err)
	}
	candidate, err = store.UpdateState(candidate.CandidateID, RouteCandidateValidated, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	return candidate
}

type candidatePlaybackMock struct {
	calls        []string
	evidence     RecordingTerminalEvidence
	failAt       string
	startAct     town.OriginAct
	target       pathing.WaypointTargetID
	startRole    pathing.RouteRole
	terminalRole pathing.RouteRole
}

func (m *candidatePlaybackMock) call(name string) error {
	m.calls = append(m.calls, name)
	if m.failAt == name {
		return errors.New(name)
	}
	return nil
}
func (m *candidatePlaybackMock) EnsureTown(context.Context, town.OriginAct) error {
	return m.call("ensure_town")
}
func (m *candidatePlaybackMock) TravelToStart(_ context.Context, contract tasks.RecordingContract) error {
	m.startAct = contract.EgressOriginAct
	m.target = contract.StartWaypoint
	m.startRole = contract.RouteRole
	return m.call("waypoint")
}
func (m *candidatePlaybackMock) PlayCandidate(context.Context, pathing.Route) error {
	return m.call("play_candidate")
}
func (m *candidatePlaybackMock) TerminalEvidence(_ context.Context, contract tasks.RecordingContract) (RecordingTerminalEvidence, error) {
	m.terminalRole = contract.RouteRole
	if err := m.call("terminal"); err != nil {
		return RecordingTerminalEvidence{}, err
	}
	return m.evidence, nil
}

func TestCowCandidatePlaybackUsesRoleContractAndNavigationOnly(t *testing.T) {
	for _, role := range []pathing.RouteRole{pathing.RouteRoleLegAcquisition, pathing.RouteRoleCowSweep} {
		t.Run(string(role), func(t *testing.T) {
			cfg := managementTestConfig(t)
			candidate := freezeCowManagementCandidate(t, cfg, role)
			store, _ := NewCandidateStore(cfg)
			candidate, err := store.UpdateState(candidate.CandidateID, RouteCandidateValidated, "", nil)
			if err != nil {
				t.Fatal(err)
			}
			state := recordingState(world.MooMooFarm, 310, 300)
			evidence := RecordingTerminalEvidence{World: state}
			if role == pathing.RouteRoleLegAcquisition {
				state = recordingState(world.Tristram, 210, 200)
				wirt := world.Object{Kind: world.ObjectKindWirtsBody, UnitID: 268, Position: world.Position{X: 212, Y: 200}}
				evidence = RecordingTerminalEvidence{World: state, Object: &wirt}
			}
			driver := &candidatePlaybackMock{evidence: evidence}
			orchestrator, _ := NewCandidateTestOrchestrator(store, tasks.DefaultRunRegistry())
			passed, err := orchestrator.Test(context.Background(), candidate.CandidateID, driver)
			if err != nil || passed.State != RouteCandidateTestPassed {
				t.Fatalf("test result = %+v, %v", passed, err)
			}
			if driver.startRole != role || driver.terminalRole != role || !reflect.DeepEqual(driver.calls, []string{"ensure_town", "waypoint", "play_candidate", "terminal", "return"}) {
				t.Fatalf("driver = roles %s/%s calls=%v", driver.startRole, driver.terminalRole, driver.calls)
			}
		})
	}
}
func (m *candidatePlaybackMock) ReturnAfterTest(context.Context, town.OriginAct) error {
	return m.call("return")
}

func TestCandidatePlaybackUsesNavigationOnlyAndMarksPassed(t *testing.T) {
	cfg := managementTestConfig(t)
	candidate := freezeManagementCandidate(t, cfg)
	store, _ := NewCandidateStore(cfg)
	orchestrator, _ := NewCandidateTestOrchestrator(store, tasks.DefaultRunRegistry())
	state := recordingState(world.TowerCellarLevel5, 125, 125)
	boss := world.Monster{NPCID: world.DarkStalker, MonsterTypeFlag: world.SuperUniqueMonsterFlag, Position: world.Position{X: 130, Y: 125}}
	driver := &candidatePlaybackMock{evidence: RecordingTerminalEvidence{World: state, Boss: &boss}}
	var progress []RouteWorkflowState
	passed, err := orchestrator.TestWithProgress(context.Background(), candidate.CandidateID, driver, func(event RouteWorkflowProgress) {
		progress = append(progress, event.State)
	})
	if err != nil {
		t.Fatal(err)
	}
	if passed.State != RouteCandidateTestPassed || passed.TestedAt == nil {
		t.Fatalf("candidate=%+v", passed)
	}
	want := []string{"ensure_town", "waypoint", "play_candidate", "terminal", "return"}
	if !reflect.DeepEqual(driver.calls, want) {
		t.Fatalf("calls=%v want=%v", driver.calls, want)
	}
	if driver.startAct != town.OriginAct1 || driver.target != pathing.WaypointTargetBlackMarsh {
		t.Fatalf("candidate start=%s/%s", driver.startAct, driver.target)
	}
	wantProgress := []RouteWorkflowState{RouteWorkflowPreparingPlayback, RouteWorkflowPreparingPlayback, RouteWorkflowPreparingPlayback, RouteWorkflowPlayingCandidate, RouteWorkflowValidatingTerminal, RouteWorkflowReturningAfterTest, RouteWorkflowAwaitingPublishConfirmation}
	if !reflect.DeepEqual(progress, wantProgress) {
		t.Fatalf("progress=%v want=%v", progress, wantProgress)
	}
}

func TestCandidatePlaybackFailureReturnsToTownAndNeverPasses(t *testing.T) {
	cfg := managementTestConfig(t)
	candidate := freezeManagementCandidate(t, cfg)
	store, _ := NewCandidateStore(cfg)
	orchestrator, _ := NewCandidateTestOrchestrator(store, tasks.DefaultRunRegistry())
	driver := &candidatePlaybackMock{failAt: "play_candidate"}
	if _, err := orchestrator.Test(context.Background(), candidate.CandidateID, driver); err == nil {
		t.Fatal("playback failure accepted")
	}
	loaded, _, _ := store.Load(candidate.CandidateID)
	if loaded.State != RouteCandidateFailed || loaded.FailureReason != RouteReasonTestPlaybackFailed || driver.calls[len(driver.calls)-1] != "return" {
		t.Fatalf("candidate=%+v calls=%v", loaded, driver.calls)
	}
}

func TestCandidatePlaybackStartFailureKeepsValidatedCandidateRetryable(t *testing.T) {
	cfg := managementTestConfig(t)
	candidate := freezeManagementCandidate(t, cfg)
	store, _ := NewCandidateStore(cfg)
	orchestrator, _ := NewCandidateTestOrchestrator(store, tasks.DefaultRunRegistry())
	driver := &candidatePlaybackMock{failAt: "ensure_town"}
	if _, err := orchestrator.Test(context.Background(), candidate.CandidateID, driver); err == nil || !strings.Contains(err.Error(), string(RouteReasonTestStartFailed)) {
		t.Fatalf("start failure=%v", err)
	}
	loaded, _, err := store.Load(candidate.CandidateID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.State != RouteCandidateValidated || loaded.FailureReason != "" {
		t.Fatalf("start failure invalidated candidate: %+v", loaded)
	}
}

func TestCandidatePlaybackUsesOriginActWaypointWithoutAct1Normalization(t *testing.T) {
	cfg := managementTestConfig(t)
	candidate := freezeMephistoManagementCandidate(t, cfg)
	store, _ := NewCandidateStore(cfg)
	orchestrator, _ := NewCandidateTestOrchestrator(store, tasks.DefaultRunRegistry())
	driver := &candidatePlaybackMock{failAt: "waypoint"}

	if _, err := orchestrator.Test(context.Background(), candidate.CandidateID, driver); err == nil {
		t.Fatal("waypoint start failure accepted")
	}
	if driver.startAct != town.OriginAct3 || driver.target != pathing.WaypointTargetDuranceOfHateLevel2 {
		t.Fatalf("Mephisto candidate start=%s/%s", driver.startAct, driver.target)
	}
	if want := []string{"ensure_town", "waypoint"}; !reflect.DeepEqual(driver.calls, want) {
		t.Fatalf("calls=%v want=%v", driver.calls, want)
	}
}

func TestCandidatePlaybackStartRequiresPortalArrivalProximity(t *testing.T) {
	state := world.State{Valid: true, Phase: world.GamePhaseInGame, Area: world.LookupArea(world.RogueEncampment), Player: world.Player{Position: world.Position{X: 100, Y: 100}}}
	if candidatePortalArrivalReady(state, 15) {
		t.Fatal("new-game town spawn without portal was accepted")
	}
	state.Objects = []world.Object{{Kind: world.ObjectKindTownPortal, UnitID: 7, Position: world.Position{X: 110, Y: 100}}}
	if !candidatePortalArrivalReady(state, 15) {
		t.Fatal("Memory-confirmed nearby portal arrival was rejected")
	}
	if candidatePortalArrivalReady(state, 5) {
		t.Fatal("portal outside configured arrival distance was accepted")
	}
}

func TestRouteManagementPublishReplaceArchiveRestoreDelete(t *testing.T) {
	cfg := managementTestConfig(t)
	first := freezeTestPassedCandidate(t, cfg)
	service, err := NewRouteManagementService(cfg, RouteManagementHooks{})
	if err != nil {
		t.Fatal(err)
	}
	publish, err := service.PreviewCandidate(first.CandidateID)
	if err != nil {
		t.Fatal(err)
	}
	if publish.Operation != RouteMutationPublish {
		t.Fatalf("operation=%s", publish.Operation)
	}
	if confirmErr := service.Confirm(RouteMutationConfirm{Token: publish.Token}); confirmErr != nil {
		t.Fatal(confirmErr)
	}
	assignments, _ := service.assignments.Snapshot()
	if assignments.Assignments["mrbones"][tasks.RunIDCountess] != publish.RouteID {
		t.Fatalf("assignments=%+v", assignments)
	}

	second := freezeTestPassedCandidate(t, cfg)
	replace, err := service.PreviewCandidate(second.CandidateID)
	if err != nil {
		t.Fatal(err)
	}
	if replace.Operation != RouteMutationReplace || replace.PreviousRouteID != publish.RouteID {
		t.Fatalf("replace=%+v", replace)
	}
	if confirmErr := service.Confirm(RouteMutationConfirm{Token: replace.Token}); confirmErr != nil {
		t.Fatal(confirmErr)
	}
	_, catalog, _ := service.lifecycle.Snapshot()
	old, _ := catalogEntryByID(catalog, publish.RouteID)
	current, _ := catalogEntryByID(catalog, replace.RouteID)
	if old.ManagementStatus != RouteManagementArchived || current.ManagementStatus != RouteManagementActive {
		t.Fatalf("old=%+v current=%+v", old, current)
	}

	archive, _ := service.PreviewRoute(RouteMutationArchive, replace.RouteID)
	if confirmErr := service.Confirm(RouteMutationConfirm{Token: archive.Token}); confirmErr != nil {
		t.Fatal(confirmErr)
	}
	assignments, _ = service.assignments.Snapshot()
	if assignments.Assignments["mrbones"][tasks.RunIDCountess] != "" {
		t.Fatalf("archive assignment=%+v", assignments)
	}
	restore, _ := service.PreviewRoute(RouteMutationRestore, publish.RouteID)
	if confirmErr := service.Confirm(RouteMutationConfirm{Token: restore.Token}); confirmErr != nil {
		t.Fatal(confirmErr)
	}
	assignments, _ = service.assignments.Snapshot()
	if assignments.Assignments["mrbones"][tasks.RunIDCountess] != publish.RouteID {
		t.Fatalf("restore assignment=%+v", assignments)
	}

	deletePreview, err := service.PreviewRoute(RouteMutationDelete, replace.RouteID)
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Confirm(RouteMutationConfirm{Token: deletePreview.Token, ConfirmRouteID: "wrong"}); err == nil {
		t.Fatal("wrong delete confirmation accepted")
	}
	deletePreview, _ = service.PreviewRoute(RouteMutationDelete, replace.RouteID)
	if err := service.Confirm(RouteMutationConfirm{Token: deletePreview.Token, ConfirmRouteID: replace.RouteID}); err != nil {
		t.Fatal(err)
	}
	if _, statErr := os.Stat(current.Path); !os.IsNotExist(statErr) {
		t.Fatalf("deleted route stat=%v", statErr)
	}
}

func TestRouteManagementReplaceRollbackKeepsExactlyOldAssignment(t *testing.T) {
	cfg := managementTestConfig(t)
	first := freezeTestPassedCandidate(t, cfg)
	service, _ := NewRouteManagementService(cfg, RouteManagementHooks{})
	publish, _ := service.PreviewCandidate(first.CandidateID)
	if err := service.Confirm(RouteMutationConfirm{Token: publish.Token}); err != nil {
		t.Fatal(err)
	}
	second := freezeTestPassedCandidate(t, cfg)
	failing, err := NewRouteManagementService(cfg, RouteManagementHooks{AfterCheckpoint: func(checkpoint string) error {
		if checkpoint == "after_old_archive_prepare" {
			return errors.New("injected")
		}
		return nil
	}})
	if err != nil {
		t.Fatal(err)
	}
	replace, _ := failing.PreviewCandidate(second.CandidateID)
	if err := failing.Confirm(RouteMutationConfirm{Token: replace.Token}); err == nil {
		t.Fatal("injected replace succeeded")
	}
	assignments, _ := failing.assignments.Snapshot()
	if assignments.Assignments["mrbones"][tasks.RunIDCountess] != publish.RouteID {
		t.Fatalf("assignment=%+v", assignments)
	}
	_, catalog, _ := failing.lifecycle.Snapshot()
	old, _ := catalogEntryByID(catalog, publish.RouteID)
	if old.ManagementStatus != RouteManagementActive {
		t.Fatalf("old status=%s", old.ManagementStatus)
	}
	if _, ok := catalogEntryByID(catalog, replace.RouteID); ok {
		t.Fatalf("rolled-back route remained")
	}
}

func TestRouteManagementReplaceRollbackAtPersistentCheckpoints(t *testing.T) {
	for _, checkpoint := range []string{"after_route_publish", "after_old_archive_prepare", "after_assignment_commit"} {
		t.Run(checkpoint, func(t *testing.T) {
			cfg := managementTestConfig(t)
			first := freezeTestPassedCandidate(t, cfg)
			service, _ := NewRouteManagementService(cfg, RouteManagementHooks{})
			publish, _ := service.PreviewCandidate(first.CandidateID)
			if err := service.Confirm(RouteMutationConfirm{Token: publish.Token}); err != nil {
				t.Fatal(err)
			}
			second := freezeTestPassedCandidate(t, cfg)
			failing, err := NewRouteManagementService(cfg, RouteManagementHooks{AfterCheckpoint: func(current string) error {
				if current == checkpoint {
					return errors.New("injected")
				}
				return nil
			}})
			if err != nil {
				t.Fatal(err)
			}
			replace, _ := failing.PreviewCandidate(second.CandidateID)
			if err := failing.Confirm(RouteMutationConfirm{Token: replace.Token}); err == nil {
				t.Fatal("injected replace succeeded")
			}
			assignment, _ := failing.assignments.Snapshot()
			if assignment.Assignments["mrbones"][tasks.RunIDCountess] != publish.RouteID {
				t.Fatalf("assignment=%+v", assignment)
			}
			_, catalog, _ := failing.lifecycle.Snapshot()
			old, _ := catalogEntryByID(catalog, publish.RouteID)
			if old.ManagementStatus != RouteManagementActive {
				t.Fatalf("old=%+v", old)
			}
			if _, exists := catalogEntryByID(catalog, replace.RouteID); exists {
				t.Fatalf("new route remained")
			}
		})
	}
}

func TestRouteManagementRejectsStalePreviewAndAssignedDelete(t *testing.T) {
	cfg := managementTestConfig(t)
	candidate := freezeTestPassedCandidate(t, cfg)
	service, _ := NewRouteManagementService(cfg, RouteManagementHooks{})
	preview, err := service.PreviewCandidate(candidate.CandidateID)
	if err != nil {
		t.Fatal(err)
	}
	assignments, _ := service.assignments.Snapshot()
	if _, err := service.assignments.Commit(assignments.Revision, cloneRouteAssignmentManifest(assignments).Assignments); err != nil {
		t.Fatal(err)
	}
	if err := service.Confirm(RouteMutationConfirm{Token: preview.Token}); err == nil {
		t.Fatal("stale preview accepted")
	}

	candidate = freezeTestPassedCandidate(t, cfg)
	publish, _ := service.PreviewCandidate(candidate.CandidateID)
	if err := service.Confirm(RouteMutationConfirm{Token: publish.Token}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.PreviewRoute(RouteMutationDelete, publish.RouteID); err == nil {
		t.Fatal("assigned route delete preview accepted")
	}
}

func TestRouteManagementGeneratedIDsAreCollisionFreeAndImmutable(t *testing.T) {
	seen := map[string]bool{}
	for range 200 {
		id, err := generatedRouteID(tasks.RunIDCountess, "mrbones")
		if err != nil {
			t.Fatal(err)
		}
		if seen[id] {
			t.Fatalf("duplicate generated route ID %q", id)
		}
		seen[id] = true
		if err := pathing.ValidateRouteID(id); err != nil {
			t.Fatal(err)
		}
	}
}

func TestRouteManagementCandidateDeleteIsStateAndHashBound(t *testing.T) {
	cfg := managementTestConfig(t)
	candidate := freezeManagementCandidate(t, cfg)
	service, err := NewRouteManagementService(cfg, RouteManagementHooks{})
	if err != nil {
		t.Fatal(err)
	}
	stale, err := service.PreviewCandidateDelete(candidate.CandidateID)
	if err != nil {
		t.Fatal(err)
	}
	if stale.Operation != RouteMutationDeleteCandidate || stale.CandidateSHA256 != candidate.ImmutableRouteSHA256 {
		t.Fatalf("preview=%+v", stale)
	}
	if _, updateErr := service.candidates.UpdateState(candidate.CandidateID, RouteCandidateTestRunning, "", nil); updateErr != nil {
		t.Fatal(updateErr)
	}
	if confirmErr := service.Confirm(RouteMutationConfirm{Token: stale.Token}); confirmErr == nil || !strings.Contains(confirmErr.Error(), string(RouteReasonCandidateChanged)) {
		t.Fatalf("changed candidate delete err=%v", confirmErr)
	}
	current, _, err := service.candidates.Load(candidate.CandidateID)
	if err != nil {
		t.Fatal(err)
	}
	preview, err := service.PreviewCandidateDelete(current.CandidateID)
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Confirm(RouteMutationConfirm{Token: preview.Token}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := service.candidates.Load(current.CandidateID); !os.IsNotExist(err) {
		t.Fatalf("deleted candidate load err=%v", err)
	}
}

func TestRouteManagementUnknownRecoveryCheckpointBlocksStartup(t *testing.T) {
	cfg := managementTestConfig(t)
	journal := RouteRecoveryJournal{SchemaVersion: 1, Operation: RouteMutationPublish, RouteID: "test-route", Checkpoint: "unknown", StartedAt: time.Now().UTC()}
	data, err := yaml.Marshal(journal)
	if err != nil {
		t.Fatal(err)
	}
	if err := writeAtomicYAML(cfg.ResolvePath(cfg.Routes.RecoveryFile), data, "test-recovery"); err != nil {
		t.Fatal(err)
	}
	if _, err := NewRouteManagementService(cfg, RouteManagementHooks{}); err == nil || !strings.Contains(err.Error(), string(RouteReasonTransactionRecoveryRequired)) {
		t.Fatalf("recovery error=%v", err)
	}
}

func TestRouteManagementStartupRecoveryRemovesUnassignedPublishAndRestoresOld(t *testing.T) {
	cfg := managementTestConfig(t)
	candidate := freezeTestPassedCandidate(t, cfg)
	service, _ := NewRouteManagementService(cfg, RouteManagementHooks{})
	publish, _ := service.PreviewCandidate(candidate.CandidateID)
	if err := service.Confirm(RouteMutationConfirm{Token: publish.Token}); err != nil {
		t.Fatal(err)
	}
	_, catalog, _ := service.lifecycle.Snapshot()
	old, _ := catalogEntryByID(catalog, publish.RouteID)
	route := old.Route
	route.ID = "orphan-recovery-route"
	orphanPath := filepath.Join(filepath.Dir(old.Path), route.ID+".yaml")
	if err := pathing.SaveRoute(orphanPath, route); err != nil {
		t.Fatal(err)
	}
	if _, err := service.lifecycle.RecordRoute(orphanPath); err != nil {
		t.Fatal(err)
	}
	journal := RouteRecoveryJournal{SchemaVersion: 1, Operation: RouteMutationReplace, RouteID: route.ID, PreviousRouteID: old.ID, Character: "mrbones", RunID: tasks.RunIDCountess, Checkpoint: "after_old_archive_prepare", StartedAt: time.Now().UTC()}
	data, _ := yaml.Marshal(journal)
	if err := writeAtomicYAML(cfg.ResolvePath(cfg.Routes.RecoveryFile), data, "test-recovery"); err != nil {
		t.Fatal(err)
	}
	if _, err := NewRouteManagementService(cfg, RouteManagementHooks{}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(orphanPath); !os.IsNotExist(err) {
		t.Fatalf("orphan stat=%v", err)
	}
	manifest, _ := service.assignments.Snapshot()
	if manifest.Assignments["mrbones"][tasks.RunIDCountess] != old.ID {
		t.Fatalf("assignment=%+v", manifest)
	}
}

func TestRouteManagementDeleteRollbackAtRenameAndManifestCheckpoints(t *testing.T) {
	for _, checkpoint := range []string{"after_quarantine_rename", "after_manifest_commit"} {
		t.Run(checkpoint, func(t *testing.T) {
			cfg := managementTestConfig(t)
			first := freezeTestPassedCandidate(t, cfg)
			service, _ := NewRouteManagementService(cfg, RouteManagementHooks{})
			publish, _ := service.PreviewCandidate(first.CandidateID)
			if err := service.Confirm(RouteMutationConfirm{Token: publish.Token}); err != nil {
				t.Fatal(err)
			}
			archive, _ := service.PreviewRoute(RouteMutationArchive, publish.RouteID)
			if err := service.Confirm(RouteMutationConfirm{Token: archive.Token}); err != nil {
				t.Fatal(err)
			}
			failing, err := NewRouteManagementService(cfg, RouteManagementHooks{AfterCheckpoint: func(current string) error {
				if current == checkpoint {
					return errors.New("injected")
				}
				return nil
			}})
			if err != nil {
				t.Fatal(err)
			}
			preview, err := failing.PreviewRoute(RouteMutationDelete, publish.RouteID)
			if err != nil {
				t.Fatal(err)
			}
			if confirmErr := failing.Confirm(RouteMutationConfirm{Token: preview.Token, ConfirmRouteID: publish.RouteID}); confirmErr == nil {
				t.Fatal("injected delete succeeded")
			}
			_, catalog, err := failing.lifecycle.Snapshot()
			if err != nil {
				t.Fatal(err)
			}
			entry, exists := catalogEntryByID(catalog, publish.RouteID)
			if !exists || entry.ManagementStatus != RouteManagementArchived {
				t.Fatalf("entry=%+v exists=%v", entry, exists)
			}
			if _, err := os.Stat(entry.Path); err != nil {
				t.Fatalf("route not restored: %v", err)
			}
		})
	}
}
