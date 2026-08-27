package api

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/app"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/pathing"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/tasks"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/telemetry"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

type liveBackendQueueRunner struct {
	started chan app.SupervisorRunRequest
	release chan app.SupervisorRunResult
}

func (r *liveBackendQueueRunner) Run(ctx context.Context, request app.SupervisorRunRequest) app.SupervisorRunResult {
	r.started <- request
	select {
	case <-ctx.Done():
		return app.SupervisorRunResult{Disposition: app.QueueRunStop, Reason: string(app.SupervisorReasonEmergencyStopRequested)}
	case result := <-r.release:
		return result
	}
}

func TestQueueStatusDTOEntriesNeverJSONNull(t *testing.T) {
	dto := queueStatusDTO(app.SupervisorSnapshot{QueueKnown: true, Queue: nil})
	encoded, err := json.Marshal(dto)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encoded), `"entries":[]`) {
		t.Fatalf("encoded=%s, want entries empty array", encoded)
	}
	if strings.Contains(string(encoded), `"entries":null`) {
		t.Fatalf("encoded=%s still contains null entries", encoded)
	}
	backend := &LiveBackend{status: StatusDTO{State: "idle", LifecyclePhase: "idle"}}
	backend.UpdateSupervisor(app.SupervisorSnapshot{
		Generation: 1, State: app.SupervisorStateIdle, QueueKnown: true, Queue: nil,
		LastResult: app.SupervisorRunResult{Disposition: app.QueueRunStop, Reason: string(app.SupervisorReasonEmergencyStopRequested)},
	})
	statusEncoded, err := json.Marshal(backend.Status())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(statusEncoded), `"entries":[]`) || strings.Contains(string(statusEncoded), `"entries":null`) {
		t.Fatalf("status after cleared queue = %s", statusEncoded)
	}
}

func TestLiveBackendProjectsControlledRetryFailureContext(t *testing.T) {
	backend := &LiveBackend{status: StatusDTO{State: "idle", LifecyclePhase: "idle"}}
	backend.UpdateSupervisor(app.SupervisorSnapshot{
		Generation: 1,
		State:      app.SupervisorStateStoppedError,
		LastResult: app.SupervisorRunResult{
			Disposition: app.QueueRunStop, Reason: "retry_return_failed",
			OriginalReason: "mercenary_died_during_run", RecoveryReason: "town_portal_not_found",
		},
	})

	status := backend.Status()
	if status.LastResult == nil || status.LastResult.OriginalReason != "mercenary_died_during_run" ||
		status.LastResult.RecoveryReason != "town_portal_not_found" {
		t.Fatalf("last result = %+v", status.LastResult)
	}
	if status.LastError == nil || status.LastError.Params["original_reason"] != "mercenary_died_during_run" ||
		status.LastError.Params["recovery_reason"] != "town_portal_not_found" {
		t.Fatalf("last error = %+v", status.LastError)
	}
}

func TestLiveBackendProjectsCurrentRecoveryStep(t *testing.T) {
	backend := &LiveBackend{status: StatusDTO{State: "running_run", LifecyclePhase: "running_run"}}
	backend.UpdateRuntime(app.UIStatusSnapshot{RecoveryStep: "local_recovery_clear"})
	if got := backend.Status().RecoveryStep; got != "local_recovery_clear" {
		t.Fatalf("runtime recovery step = %q", got)
	}

	backend.UpdateSupervisor(app.SupervisorSnapshot{
		Generation: 1,
		State:      app.SupervisorStateExitingGame,
		LastResult: app.SupervisorRunResult{ExitAuthorization: app.ExitAuthorizationMemoryGatedCurrentArea},
	})
	if got := backend.Status().RecoveryStep; got != "direct_exit" {
		t.Fatalf("direct-exit recovery step = %q", got)
	}
}

func TestLiveBackendProjectsStatusAndMeaningfulEvents(t *testing.T) {
	cfg, err := config.Load("../../configs/config.example.yaml")
	if err != nil {
		t.Fatal(err)
	}
	cfg.Routes.LifecycleFile = filepath.Join(t.TempDir(), "route-lifecycle.local.yaml")
	publisher := telemetry.NewLivePublisher(16, 4)
	backend, err := NewLiveBackend(cfg, publisher)
	if err != nil {
		t.Fatal(err)
	}
	backend.Update(app.UIStatusSnapshot{
		ProcessState: "attached", PID: 42, WindowBound: true, ClientWidth: 1280, ClientHeight: 720,
		InputEnabled: true, WorldValid: true, WorldPhase: "gameplay", AreaID: 1, AreaName: "Rogue Encampment",
		Compatibility: compatibleRuntimeSnapshot(),
	}, app.SupervisorSnapshot{
		State: app.SupervisorStateRunningRun, Generation: 7, QueueKnown: true, Queue: []string{"countess", "mephisto"}, QueueIndex: 1, Cycle: 2, Retry: 1,
		StartedRuns: 5, TotalRestarts: 1, Budgets: app.FarmQueueBudgets{MaxRuns: 8, MaxDuration: time.Hour, MaxConsecutiveFailures: 2, MaxTotalRestarts: 3},
	})

	status := backend.Status()
	if status.Generation != 7 || status.D2R.PID != 42 || !status.D2R.WindowBound || status.World.AreaName != "Rogue Encampment" || status.Queue.Index != 1 || status.Queue.Budgets.MaxDurationMs != 3600000 {
		t.Fatalf("live status = %+v", status)
	}
	replay, subscription := publisher.Subscribe(0)
	subscription.Close()
	if !containsLiveEvent(replay, "d2r_state_changed") || !containsLiveEvent(replay, "input_state_changed") || !containsLiveEvent(replay, "area_changed") || !containsLiveEvent(replay, "world_state_changed") {
		t.Fatalf("live events = %+v", replay)
	}
	backend.Update(app.UIStatusSnapshot{
		ProcessState: "attached", PID: 42, WindowBound: true, ClientWidth: 1280, ClientHeight: 720,
		InputEnabled: true, WorldValid: true, WorldPhase: "gameplay", AreaID: 1, AreaName: "Rogue Encampment",
		Compatibility: compatibleRuntimeSnapshot(),
	}, app.SupervisorSnapshot{
		State: app.SupervisorStateRunningRun, Generation: 7, QueueKnown: true, Queue: []string{"countess", "mephisto"}, QueueIndex: 1, Cycle: 2, Retry: 1,
		StartedRuns: 5, TotalRestarts: 1, Budgets: app.FarmQueueBudgets{MaxRuns: 8, MaxDuration: time.Hour, MaxConsecutiveFailures: 2, MaxTotalRestarts: 3},
	})
	if publisher.Sequence() != uint64(len(replay)) {
		t.Fatalf("unchanged status published duplicate events; sequence = %d", publisher.Sequence())
	}
	backend.Update(app.UIStatusSnapshot{ProcessState: "detached"}, app.SupervisorSnapshot{
		State: app.SupervisorStateIdle, Generation: 8, QueueKnown: true,
	})
	if got := backend.Status().Queue.Entries; len(got) != 0 {
		t.Fatalf("authoritative empty queue was not projected: %v", got)
	}
	backend.UpdateRuntime(app.UIStatusSnapshot{ProcessState: "attached"})
	if got := backend.Status().Queue.Entries; len(got) != 0 {
		t.Fatalf("passive runtime update restored stale queue: %v", got)
	}
}

func TestLiveBackendPublishesOnlyUserFacingRunProgressChanges(t *testing.T) {
	publisher := telemetry.NewLivePublisher(16, 4)
	backend := &LiveBackend{publisher: publisher, status: StatusDTO{Queue: QueueStatusDTO{Entries: []string{}, DefaultEntries: []string{}}}}
	first := &tasks.RunProgress{StageCode: "travel_tower", Current: 3, Total: 13}
	backend.UpdateRuntime(app.UIStatusSnapshot{RunID: "countess", Step: "play_bound_route", RunProgress: first})
	status := backend.Status()
	if status.RunProgress == nil || !reflect.DeepEqual(*status.RunProgress, RunProgressDTO{StageCode: "travel_tower", Current: 3, Total: 13}) {
		t.Fatalf("run progress = %+v", status.RunProgress)
	}
	sequence := publisher.Sequence()
	if sequence != 1 {
		t.Fatalf("sequence after first progress = %d, want 1", sequence)
	}

	// Internal steps may change inside one visible stage without causing an SSE refresh.
	backend.UpdateRuntime(app.UIStatusSnapshot{RunID: "countess", Step: "wait_entry_area", RunProgress: first})
	if publisher.Sequence() != sequence {
		t.Fatalf("internal step change published event; sequence = %d, want %d", publisher.Sequence(), sequence)
	}

	next := &tasks.RunProgress{StageCode: "cellar_floor", Params: map[string]any{"floor": 1, "floors": 5}, Current: 4, Total: 13}
	backend.UpdateRuntime(app.UIStatusSnapshot{RunID: "countess", Step: "play_bound_route", RunProgress: next})
	replay, subscription := publisher.Subscribe(0)
	subscription.Close()
	if !containsLiveEvent(replay, "run_progress_changed") || publisher.Sequence() != sequence+1 {
		t.Fatalf("progress events = %+v", replay)
	}

	invalid := &tasks.RunProgress{StageCode: "invalid", Current: 14, Total: 13}
	backend.UpdateRuntime(app.UIStatusSnapshot{RunID: "countess", RunProgress: invalid})
	if backend.Status().RunProgress != nil {
		t.Fatalf("invalid run progress was published: %+v", backend.Status().RunProgress)
	}
}

func TestLiveBackendDoesNotRestoreSelectionWithoutCharacterProfile(t *testing.T) {
	cfg, err := config.Load("../../configs/config.example.yaml")
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	cfg.DataRoot = root
	cfg.Session.Character = ""
	cfg.Session.Difficulty = "normal"
	cfg.Routes.FarmingRoot = filepath.Join(root, "farming")
	cfg.Routes.CandidateRoot = filepath.Join(root, "candidates")
	cfg.Routes.LifecycleFile = filepath.Join(root, "lifecycle.yaml")
	cfg.Routes.AssignmentsFile = filepath.Join(root, "assignments.yaml")
	cfg.Routes.RecoveryFile = filepath.Join(root, "recovery.yaml")

	settings, err := app.NewOperatorSettingsStore(root, cfg, []string{"MrBones", "MrHammer"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = settings.Snapshot(); err != nil {
		t.Fatal(err)
	}
	lifecycle, err := app.NewRouteLifecycleStore(cfg)
	if err != nil {
		t.Fatal(err)
	}
	preview, err := lifecycle.Preview("MrBones", "hell")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = lifecycle.Confirm(preview, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	backend, err := NewLiveBackend(cfg, telemetry.NewLivePublisher(16, 4))
	if err != nil {
		t.Fatal(err)
	}
	backend.mu.Lock()
	backend.status.Selection = SelectionStatusDTO{Character: "MrBones", Difficulty: "hell"}
	backend.mu.Unlock()
	if err = backend.SetOperatorSettingsStore(settings); err != nil {
		t.Fatal(err)
	}
	status := backend.Status()
	if status.Selection.Character != "" || status.Selection.Difficulty != "" {
		t.Fatalf("selection=%+v", status.Selection)
	}
	persisted, err := settings.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if persisted.LastCharacter != "" {
		t.Fatalf("persisted=%+v", persisted)
	}
}

func TestRouteWorkflowRejectsStaleGenerationAndActiveSession(t *testing.T) {
	cfg, err := config.Load("../../configs/config.example.yaml")
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	cfg.Routes.FarmingRoot = filepath.Join(root, "farming")
	cfg.Routes.CandidateRoot = filepath.Join(root, "candidates")
	cfg.Routes.LifecycleFile = filepath.Join(root, "lifecycle.yaml")
	cfg.Routes.AssignmentsFile = filepath.Join(root, "assignments.yaml")
	cfg.Routes.RecoveryFile = filepath.Join(root, "recovery.yaml")
	cfg.Input.Enabled = true
	backend, err := NewLiveBackend(cfg, telemetry.NewLivePublisher(16, 4))
	if err != nil {
		t.Fatal(err)
	}
	backend.SetRouteWorkflowHandler(func(RouteWorkflowRequest, <-chan struct{}, app.RouteWorkflowReporter) error { return nil })
	markBackendCompatible(backend)
	if _, routeErr := backend.StartRouteWorkflow(RouteWorkflowRequest{ExpectedGeneration: 99, Operation: "record", RunID: "countess"}); routeErr == nil {
		t.Fatal("stale generation accepted")
	}
	backend.mu.Lock()
	backend.status.State = string(app.SupervisorStateRunningRun)
	backend.mu.Unlock()
	if _, routeErr := backend.StartRouteWorkflow(RouteWorkflowRequest{ExpectedGeneration: 1, Operation: "record", RunID: "countess"}); routeErr == nil {
		t.Fatal("active session conflict accepted")
	}
	backend.mu.Lock()
	backend.status.State = string(app.SupervisorStateIdle)
	backend.status.Selection = SelectionStatusDTO{Character: "MrBones", Difficulty: "nightmare"}
	backend.mu.Unlock()
	snapshot, err := backend.StartRouteWorkflow(RouteWorkflowRequest{ExpectedGeneration: 1, Operation: "record", RunID: "countess"})
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Generation != 2 || snapshot.State != string(app.RouteWorkflowPreflight) {
		t.Fatalf("workflow = %+v", snapshot)
	}
}

func TestCowRecordingOptionsExposeTwoFixedRoleWorkflows(t *testing.T) {
	cfg, err := config.Load("../../configs/config.example.yaml")
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	cfg.Routes.FarmingRoot = filepath.Join(root, "farming")
	cfg.Routes.CandidateRoot = filepath.Join(root, "candidates")
	cfg.Routes.LifecycleFile = filepath.Join(root, "lifecycle.yaml")
	cfg.Routes.AssignmentsFile = filepath.Join(root, "assignments.yaml")
	cfg.Routes.RecoveryFile = filepath.Join(root, "recovery.yaml")
	backend, err := NewLiveBackend(cfg, telemetry.NewLivePublisher(16, 4))
	if err != nil {
		t.Fatal(err)
	}
	roles := map[string]RecordingOptionDTO{}
	for _, option := range backend.RecordingOptions("") {
		if option.RunID == "cows" {
			roles[option.RouteRole] = option
		}
	}
	leg, legOK := roles[string(pathing.RouteRoleLegAcquisition)]
	sweep, sweepOK := roles[string(pathing.RouteRoleCowSweep)]
	if !legOK || !sweepOK || leg.StartKind != string(tasks.RecordingStartWaypoint) || sweep.StartKind != string(tasks.RecordingStartObjectPortalArrival) {
		t.Fatalf("cow recording options = %+v", roles)
	}
	for _, prerequisite := range sweep.Prerequisites {
		if prerequisite.ID == "waypoint" {
			t.Fatalf("cow sweep must not claim a waypoint prerequisite: %+v", sweep.Prerequisites)
		}
	}
	if !reflect.DeepEqual(leg.OperatorHintCodes, []string{"cow_leg_portal_open", "cow_leg_do_not_click_wirt"}) ||
		!reflect.DeepEqual(sweep.OperatorHintCodes, []string{"cow_portal_open", "cow_level_clear"}) {
		t.Fatalf("cow operator hint codes = %+v / %+v", leg.OperatorHintCodes, sweep.OperatorHintCodes)
	}
}

func TestRouteWorkflowFinishIsOneShotIdempotentAndPublishesWorkflowFields(t *testing.T) {
	cfg, err := config.Load("../../configs/config.example.yaml")
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	cfg.Routes.FarmingRoot = filepath.Join(root, "farming")
	cfg.Routes.CandidateRoot = filepath.Join(root, "candidates")
	cfg.Routes.LifecycleFile = filepath.Join(root, "lifecycle.yaml")
	cfg.Routes.AssignmentsFile = filepath.Join(root, "assignments.yaml")
	cfg.Routes.RecoveryFile = filepath.Join(root, "recovery.yaml")
	cfg.Input.Enabled = true
	publisher := telemetry.NewLivePublisher(16, 4)
	backend, err := NewLiveBackend(cfg, publisher)
	if err != nil {
		t.Fatal(err)
	}
	markBackendCompatible(backend)
	finishReceived := make(chan struct{})
	recordingReady := make(chan struct{})
	release := make(chan struct{})
	backend.SetRouteWorkflowHandler(func(_ RouteWorkflowRequest, finish <-chan struct{}, reporter app.RouteWorkflowReporter) error {
		reporter(app.RouteWorkflowProgress{State: app.RouteWorkflowRecording, AreaID: uint32(world.BlackMarsh), Segment: 1, Progress: 0.25})
		close(recordingReady)
		<-finish
		close(finishReceived)
		<-release
		return nil
	})
	backend.mu.Lock()
	backend.status.Selection = SelectionStatusDTO{Character: "MrBones", Difficulty: "nightmare"}
	backend.mu.Unlock()
	started, err := backend.StartRouteWorkflow(RouteWorkflowRequest{ExpectedGeneration: 1, Operation: "record", RunID: "countess"})
	if err != nil {
		t.Fatal(err)
	}
	<-recordingReady
	recording := backend.RouteWorkflow()
	first, err := backend.FinishRouteWorkflow(started.WorkflowID, RouteWorkflowFinishRequest{ExpectedGeneration: recording.Generation})
	if err != nil {
		t.Fatal(err)
	}
	<-finishReceived
	second, err := backend.FinishRouteWorkflow(started.WorkflowID, RouteWorkflowFinishRequest{ExpectedGeneration: recording.Generation})
	if err != nil || second != first || first.State != string(app.RouteWorkflowFreezing) {
		t.Fatalf("idempotent finish first=%+v second=%+v err=%v", first, second, err)
	}
	replay, subscription := publisher.Subscribe(0)
	subscription.Close()
	found := false
	for _, event := range replay {
		if event.Event == "route_workflow_changed" && event.WorkflowID == started.WorkflowID && event.State == string(app.RouteWorkflowFreezing) && event.Run == "countess" {
			found = true
		}
	}
	if !found {
		t.Fatalf("workflow SSE fields missing: %+v", replay)
	}
	close(release)
}

func TestRouteWorkflowBlocksSelectionAndRouteMutation(t *testing.T) {
	cfg, err := config.Load("../../configs/config.example.yaml")
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	cfg.Routes.FarmingRoot = filepath.Join(root, "farming")
	cfg.Routes.CandidateRoot = filepath.Join(root, "candidates")
	cfg.Routes.LifecycleFile = filepath.Join(root, "lifecycle.yaml")
	cfg.Routes.AssignmentsFile = filepath.Join(root, "assignments.yaml")
	cfg.Routes.RecoveryFile = filepath.Join(root, "recovery.yaml")
	backend, err := NewLiveBackend(cfg, telemetry.NewLivePublisher(16, 4))
	if err != nil {
		t.Fatal(err)
	}
	backend.mu.Lock()
	backend.routeWorkflow.State = string(app.RouteWorkflowRecording)
	revision := backend.catalog.Revision
	backend.mu.Unlock()
	if _, err = backend.PreviewSelection(SelectionPreviewRequest{Character: "MrBones", Difficulty: "nightmare", CatalogRevision: revision}); err == nil {
		t.Fatal("selection preview accepted during route workflow")
	}
	if _, err = backend.PreviewRouteMutation(RouteMutationPreviewRequest{Operation: "archive", RouteID: "route"}); err == nil {
		t.Fatal("route mutation preview accepted during route workflow")
	}
	if _, err = backend.ValidateQueue(QueueValidationRequest{}); err == nil {
		t.Fatal("queue validation accepted during route workflow")
	}
}

func TestConfirmedRoutePublicationRefreshesRunCatalog(t *testing.T) {
	cfg, err := config.Load("../../configs/config.example.yaml")
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	cfg.Routes.FarmingRoot = filepath.Join(root, "farming")
	cfg.Routes.CandidateRoot = filepath.Join(root, "candidates")
	cfg.Routes.LifecycleFile = filepath.Join(root, "lifecycle.yaml")
	cfg.Routes.AssignmentsFile = filepath.Join(root, "assignments.yaml")
	cfg.Routes.RecoveryFile = filepath.Join(root, "recovery.yaml")

	lifecycle, err := app.NewRouteLifecycleStore(cfg)
	if err != nil {
		t.Fatal(err)
	}
	_, routeCatalog, err := lifecycle.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	assignments, err := app.NewRouteAssignmentStore(cfg)
	if err != nil {
		t.Fatal(err)
	}
	assignment, err := assignments.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	store, err := app.NewCandidateStore(cfg)
	if err != nil {
		t.Fatal(err)
	}
	seed := uint32(42)
	route := pathing.Route{
		Version: 1, ID: "candidate-route", Name: "Countess candidate", Kind: pathing.RouteKindNavigation,
		Binding: pathing.RouteBinding{
			CharacterName: "MrBones", CharacterClass: "necromancer", Difficulty: pathing.RouteDifficultyNightmare,
			MapSeed: &seed, GameVersion: cfg.Memory.GameVersion,
			LayoutFingerprint: pathing.RouteLayoutFingerprint{Version: 1, AreaID: world.BlackMarsh, AnchorCount: 1, Hash: strings.Repeat("a", 64)},
		},
		Recording: pathing.RouteRecording{RecordedAt: time.Now().UTC(), SampleDistanceTiles: 4},
		Playback:  pathing.RoutePlayback{WaypointToleranceTiles: 3, MaxDriftTiles: 8, MaxLocalCorrections: 2, SegmentTimeoutMs: 30000, TransitionTimeoutMs: 10000},
		Segments: []pathing.RouteSegment{{
			ID: "black-marsh", FromAreaID: world.BlackMarsh, ToAreaID: world.TowerCellarLevel5, Movement: pathing.RouteMovementTeleport,
			Points:     []pathing.RoutePoint{{X: 100, Y: 100}, {X: 110, Y: 110}},
			Transition: pathing.RouteTransition{Type: "entrance", EntranceKind: "tower_cellar_down"},
		}, {
			ID: "tower-cellar-level-5", FromAreaID: world.TowerCellarLevel5, ToAreaID: world.TowerCellarLevel5, Movement: pathing.RouteMovementTeleport,
			Points:     []pathing.RoutePoint{{X: 120, Y: 120}, {X: 125, Y: 125}},
			Transition: pathing.RouteTransition{Type: "terminal"},
		}},
	}
	candidate, err := store.Freeze(route, app.RouteCandidate{
		RunID: tasks.RunIDCountess, Character: "MrBones", Difficulty: "nightmare", GameVersion: cfg.Memory.GameVersion,
		State: app.RouteCandidateRecorded, MeasuredBossDistance: 20, SourceCatalogRevision: routeCatalog.Revision,
		SourceAssignmentRevision: assignment.Revision, CreatedAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatal(err)
	}
	candidate, err = store.UpdateState(candidate.CandidateID, app.RouteCandidateValidated, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	candidate, err = store.UpdateState(candidate.CandidateID, app.RouteCandidateTestPassed, "", &now)
	if err != nil {
		t.Fatal(err)
	}

	backend, err := NewLiveBackend(cfg, telemetry.NewLivePublisher(16, 4))
	if err != nil {
		t.Fatal(err)
	}
	markBackendCompatible(backend)
	backend.mu.Lock()
	backend.status.Selection = SelectionStatusDTO{Character: "MrBones", Difficulty: "nightmare"}
	backend.mu.Unlock()
	preview, err := backend.PreviewRouteMutation(RouteMutationPreviewRequest{CandidateID: candidate.CandidateID})
	if err != nil {
		t.Fatal(err)
	}
	if err := backend.ConfirmRouteMutation(RouteMutationConfirmRequest{ConfirmationToken: preview.ConfirmationToken, ConfirmRouteID: preview.RouteID}); err != nil {
		t.Fatal(err)
	}
	for _, run := range backend.Catalog().Runs {
		if run.RunID == "countess" {
			if run.Status != "runtime_validation_required" || containsString(run.Reasons, "route_assignment_missing") {
				t.Fatalf("published Countess catalog is stale: %+v", run)
			}
			return
		}
	}
	t.Fatal("Countess run missing from refreshed catalog")
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func TestLiveBackendSessionCommandsUseSupervisorAndRemainIdempotent(t *testing.T) {
	cfg, err := config.Load("../../configs/config.example.yaml")
	if err != nil {
		t.Fatal(err)
	}
	cfg.Routes.LifecycleFile = filepath.Join(t.TempDir(), "route-lifecycle.local.yaml")
	cfg.Session.MaxRuns = 4
	writeAPITestAssignments(t, cfg)
	publisher := telemetry.NewLivePublisher(32, 8)
	backend, err := NewLiveBackend(cfg, publisher)
	if err != nil {
		t.Fatal(err)
	}
	configureDesktopCharacterContract(t, backend, cfg, "countess", "mephisto")
	markBackendCompatible(backend)
	backend.mu.Lock()
	backend.status.State = string(app.SupervisorStateIdleInGame)
	backend.status.Selection = SelectionStatusDTO{Character: "MrBones", Difficulty: "nightmare"}
	revision := backend.catalog.Revision
	backend.mu.Unlock()
	runner := &liveBackendQueueRunner{started: make(chan app.SupervisorRunRequest, 3), release: make(chan app.SupervisorRunResult, 3)}
	supervisor, err := app.NewSessionSupervisor(runner)
	if err != nil {
		t.Fatal(err)
	}
	if setupErr := backend.SetSessionSupervisor(supervisor, nil, nil); setupErr != nil {
		t.Fatal(setupErr)
	}
	payload, _ := json.Marshal(SessionStartPayload{Entries: []string{"countess", "mephisto"}, Character: "MrBones", Difficulty: "nightmare", CatalogRevision: revision})
	startRequest := CommandRequest{CommandID: "start-queue", ExpectedGeneration: 0, Payload: payload}
	start, err := backend.Command("start_queue", startRequest)
	if err != nil {
		t.Fatal(err)
	}
	if replay, err := backend.Command("start_queue", startRequest); err != nil || replay != start {
		t.Fatalf("start replay = %+v, %v; want %+v", replay, err, start)
	}
	changedPayload, _ := json.Marshal(SessionStartPayload{Entries: []string{"countess"}, Character: "MrBones", Difficulty: "nightmare", CatalogRevision: revision})
	if _, err := backend.Command("start_queue", CommandRequest{CommandID: "start-queue", ExpectedGeneration: 0, Payload: changedPayload}); err == nil {
		t.Fatal("command ID reuse with changed queue was accepted")
	}
	if request := <-runner.started; request.DefinitionID != "countess" || request.QueueIndex != 0 {
		t.Fatalf("first request = %+v", request)
	}
	backend.UpdateSupervisor(supervisor.Snapshot())
	pauseGeneration := backend.Status().Generation
	if _, err := backend.Command("pause_after_run", CommandRequest{CommandID: "pause", ExpectedGeneration: pauseGeneration}); err != nil {
		t.Fatal(err)
	}
	runner.release <- app.SupervisorRunResult{Disposition: app.QueueRunAdvance, ExitAuthorization: app.ExitAuthorizationVerifiedRogueTown}
	waitAPIBackendSupervisor(t, supervisor, app.SupervisorStatePausedBetweenRuns)
	backend.UpdateSupervisor(supervisor.Snapshot())
	if backend.Status().Queue.Index != 1 || backend.Status().PendingIntent != string(app.SupervisorIntentNone) {
		t.Fatalf("paused status = %+v", backend.Status())
	}
	if _, err := backend.Command("resume", CommandRequest{CommandID: "resume", ExpectedGeneration: backend.Status().Generation}); err != nil {
		t.Fatal(err)
	}
	if request := <-runner.started; request.DefinitionID != "mephisto" || request.QueueIndex != 1 {
		t.Fatalf("resumed request = %+v", request)
	}
	backend.UpdateSupervisor(supervisor.Snapshot())
	if _, err := backend.Command("stop_after_run", CommandRequest{CommandID: "stop", ExpectedGeneration: backend.Status().Generation}); err != nil {
		t.Fatal(err)
	}
	runner.release <- app.SupervisorRunResult{Disposition: app.QueueRunAdvance, ExitAuthorization: app.ExitAuthorizationVerifiedRogueTown}
	waitAPIBackendSupervisor(t, supervisor, app.SupervisorStateIdle)
	backend.UpdateSupervisor(supervisor.Snapshot())
	if len(backend.Status().Queue.Entries) != 0 || len(backend.Status().Queue.DefaultEntries) != 2 {
		t.Fatalf("stopped queue status = %+v", backend.Status().Queue)
	}
	restartPayload, _ := json.Marshal(SessionStartPayload{Entries: []string{"countess"}, Character: "MrBones", Difficulty: "nightmare", CatalogRevision: revision})
	if _, err := backend.Command("start_queue", CommandRequest{CommandID: "restart", ExpectedGeneration: backend.Status().Generation, Payload: restartPayload}); err != nil {
		t.Fatal(err)
	}
	<-runner.started
	backend.UpdateSupervisor(supervisor.Snapshot())
	if _, err := backend.Command("emergency_stop", CommandRequest{CommandID: "emergency", ExpectedGeneration: backend.Status().Generation}); err != nil {
		t.Fatal(err)
	}
	waitAPIBackendSupervisor(t, supervisor, app.SupervisorStateIdle)
	backend.UpdateSupervisor(supervisor.Snapshot())
	if backend.Status().State != string(app.SupervisorStateIdle) || len(backend.Status().Queue.Entries) != 0 || backend.Status().LastResult == nil || backend.Status().LastResult.Reason != string(app.SupervisorReasonEmergencyStopRequested) {
		t.Fatalf("emergency status = %+v", backend.Status())
	}
}

func TestLiveBackendQueueAdoptsPassiveConfirmedOpenGameFromIdle(t *testing.T) {
	cfg, err := config.Load("../../configs/config.example.yaml")
	if err != nil {
		t.Fatal(err)
	}
	cfg.Routes.LifecycleFile = filepath.Join(t.TempDir(), "route-lifecycle.local.yaml")
	writeAPITestAssignments(t, cfg)
	backend, err := NewLiveBackend(cfg, telemetry.NewLivePublisher(16, 4))
	if err != nil {
		t.Fatal(err)
	}
	configureDesktopCharacterContract(t, backend, cfg, "countess")
	backend.mu.Lock()
	backend.status.State = string(app.SupervisorStateIdle)
	backend.status.Selection = SelectionStatusDTO{Character: "MrBones", Difficulty: "nightmare"}
	revision := backend.catalog.Revision
	backend.mu.Unlock()
	backend.UpdateRuntime(app.UIStatusSnapshot{
		ProcessState: "attached", WindowBound: true, WorldValid: true,
		WorldPhase: "in_game", AreaID: uint32(world.RogueEncampment), AreaName: "Rogue Encampment",
		Compatibility: compatibleRuntimeSnapshot(),
	})
	runner := &liveBackendQueueRunner{started: make(chan app.SupervisorRunRequest, 1), release: make(chan app.SupervisorRunResult, 1)}
	supervisor, err := app.NewSessionSupervisor(runner)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := supervisor.Shutdown(ctx); err != nil {
			t.Errorf("shutdown supervisor: %v", err)
		}
	})
	beginCalled := false
	adopted := false
	if err := backend.SetSessionSupervisor(supervisor, nil, func(initialInGame bool) error {
		beginCalled = true
		adopted = initialInGame
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	payload, _ := json.Marshal(SessionStartPayload{
		Entries: []string{"countess"}, Character: "MrBones", Difficulty: "nightmare", CatalogRevision: revision,
	})
	if _, err := backend.Command("start_queue", CommandRequest{CommandID: "adopt-open-game", ExpectedGeneration: backend.Status().Generation, Payload: payload}); err != nil {
		t.Fatal(err)
	}
	if !beginCalled || !adopted {
		t.Fatalf("queue begin called=%t adopted=%t; want confirmed open game adoption", beginCalled, adopted)
	}
}

func writeAPITestAssignments(t *testing.T, cfg *config.Config) {
	t.Helper()
	cfg.Routes.AssignmentsFile = filepath.Join(t.TempDir(), "route-assignments.local.yaml")
	body := []byte("schema_version: 1\nrevision: 1\nassignments:\n  mrbones:\n    countess: black-marsh-cellar5-nightmare-mrbones\n    mephisto: durance-2-mephisto-nightmare-mrbones\n")
	if err := os.WriteFile(cfg.ResolvePath(cfg.Routes.AssignmentsFile), body, 0o600); err != nil {
		t.Fatal(err)
	}
}

func waitAPIBackendSupervisor(t *testing.T, supervisor *app.SessionSupervisor, state app.SupervisorState) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if supervisor.Snapshot().State == state {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("supervisor state = %s, want %s", supervisor.Snapshot().State, state)
}

func TestLiveBackendAppliesOnlySelectableSameDifficultyCharacter(t *testing.T) {
	cfg, err := config.Load("../../configs/config.example.yaml")
	if err != nil {
		t.Fatal(err)
	}
	cfg.Session.Difficulty = "nightmare"
	cfg.Session.Character = "MrBones"
	cfg.Routes.LifecycleFile = filepath.Join(t.TempDir(), "route-lifecycle.local.yaml")
	publisher := telemetry.NewLivePublisher(16, 4)
	backend, err := NewLiveBackend(cfg, publisher)
	if err != nil {
		t.Fatal(err)
	}
	markBackendCompatible(backend)
	configureDesktopCharacterContract(t, backend, cfg)
	called := 0
	backend.SetSelectionHandler(func(request app.CharacterSelectionRequest) error {
		called++
		if request.Character != "MrBones" || request.Difficulty != "nightmare" || request.CharacterCount != 1 {
			t.Fatalf("selection request = %+v", request)
		}
		return nil
	})
	backend.mu.Lock()
	backend.status.Input = InputDTO{Enabled: true}
	backend.mu.Unlock()
	preview, err := backend.PreviewSelection(SelectionPreviewRequest{Character: "MrBones", Difficulty: "nightmare", CatalogRevision: 1})
	if err != nil {
		t.Fatal(err)
	}
	payload, _ := json.Marshal(map[string]any{"character": "MrBones", "difficulty": "nightmare", "catalog_revision": 1, "confirmation_token": preview.ConfirmationToken})
	response, err := backend.Command("apply_selection", CommandRequest{CommandID: "selection-1", ExpectedGeneration: 0, Payload: payload})
	if err != nil {
		t.Fatal(err)
	}
	if called != 1 || response.State != string(app.SupervisorStateIdleInGame) || response.Generation != 2 {
		t.Fatalf("response=%+v calls=%d", response, called)
	}
	if _, err := backend.Command("apply_selection", CommandRequest{CommandID: "selection-1", ExpectedGeneration: 0, Payload: payload}); err != nil || called != 1 {
		t.Fatalf("idempotent replay err=%v calls=%d", err, called)
	}
}

func TestLiveBackendRejectsMissingAnchorBeforeSelectionInput(t *testing.T) {
	cfg, err := config.Load("../../configs/config.example.yaml")
	if err != nil {
		t.Fatal(err)
	}
	cfg.Routes.LifecycleFile = filepath.Join(t.TempDir(), "route-lifecycle.local.yaml")
	publisher := telemetry.NewLivePublisher(8, 2)
	backend, err := NewLiveBackend(cfg, publisher)
	if err != nil {
		t.Fatal(err)
	}
	setBackendCharacterCatalogProjection(backend, app.CharacterCatalogEntry{Name: "Missing", Slug: "missing", ExpectedClass: "necromancer", Selectable: false, Reasons: []string{app.CharacterReasonAnchorMissing}})
	backend.SetSelectionHandler(func(app.CharacterSelectionRequest) error { t.Fatal("disabled character reached selector"); return nil })
	if _, err := backend.PreviewSelection(SelectionPreviewRequest{Character: "Missing", Difficulty: cfg.Session.Difficulty, CatalogRevision: 1}); err == nil {
		t.Fatal("missing anchor selection was accepted")
	}
}

func TestLiveBackendCorrelatesRunsWithoutGameExitBetweenEntries(t *testing.T) {
	publisher := telemetry.NewLivePublisher(32, 8)
	backend := &LiveBackend{publisher: publisher, status: StatusDTO{State: "idle", LifecyclePhase: "idle", Queue: QueueStatusDTO{Entries: []string{"countess", "mephisto"}}}}
	backend.UpdateSupervisor(app.SupervisorSnapshot{Generation: 1, State: app.SupervisorStateRunningRun, QueueKnown: true, Queue: []string{"countess", "mephisto"}, QueueIndex: 0, GameID: "game-001", ActiveRunID: "countess", RunInstanceID: "run-001"})
	backend.UpdateSupervisor(app.SupervisorSnapshot{Generation: 2, State: app.SupervisorStateRunningRun, QueueKnown: true, Queue: []string{"countess", "mephisto"}, QueueIndex: 1, GameID: "game-001", ActiveRunID: "mephisto", RunInstanceID: "run-002"})
	backend.UpdateSupervisor(app.SupervisorSnapshot{Generation: 3, State: app.SupervisorStateExitingGame, QueueKnown: true, Queue: []string{"countess", "mephisto"}, QueueIndex: 0, GameID: "game-001", ActiveRunID: "mephisto", RunInstanceID: "run-002"})
	backend.UpdateSupervisor(app.SupervisorSnapshot{Generation: 4, State: app.SupervisorStateRunningRun, QueueKnown: true, Queue: []string{"countess", "mephisto"}, QueueIndex: 0, Cycle: 1, GameID: "game-002", ActiveRunID: "countess", RunInstanceID: "run-003"})
	replay, subscription := publisher.Subscribe(0)
	subscription.Close()
	var gameEvents []telemetry.LiveEvent
	for _, event := range replay {
		if event.Event == "game_started" || event.Event == "game_exited" {
			gameEvents = append(gameEvents, event)
		}
	}
	if len(gameEvents) != 3 || gameEvents[0].Event != "game_started" || gameEvents[0].GameID != "game-001" || gameEvents[1].Event != "game_exited" || gameEvents[1].GameID != "game-001" || gameEvents[2].Event != "game_started" || gameEvents[2].GameID != "game-002" {
		t.Fatalf("game events = %+v", gameEvents)
	}
}

func TestSelectionPreviewIsSideEffectFreeAndListsDifficultyImpact(t *testing.T) {
	backend := newSelectionTestBackend(t)
	before, _, err := backend.lifecycle.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	exact, err := backend.lifecycle.Preview("MrBones", "nightmare")
	if err != nil {
		t.Fatal(err)
	}
	if _, confirmErr := backend.lifecycle.Confirm(exact, time.Now().UTC()); confirmErr != nil {
		t.Fatal(confirmErr)
	}
	confirmed, _, err := backend.lifecycle.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	preview, err := backend.PreviewSelection(SelectionPreviewRequest{Character: "MrBones", Difficulty: "hell", CatalogRevision: 1})
	if err != nil {
		t.Fatal(err)
	}
	after, _, err := backend.lifecycle.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if before.Revision >= confirmed.Revision || after.Revision != confirmed.Revision {
		t.Fatalf("preview changed lifecycle revision: before=%d confirmed=%d after=%d", before.Revision, confirmed.Revision, after.Revision)
	}
	if !preview.RequiresConfirmation || preview.InvalidationReason != "difficulty_changed" || len(preview.AffectedRoutes) == 0 {
		t.Fatalf("difficulty preview = %+v", preview)
	}
}

func TestSelectionRejectsStalePreviewBeforeInput(t *testing.T) {
	backend := newSelectionTestBackend(t)
	preview, err := backend.PreviewSelection(SelectionPreviewRequest{Character: "MrBones", Difficulty: "nightmare", CatalogRevision: 1})
	if err != nil {
		t.Fatal(err)
	}
	if _, invalidateErr := backend.lifecycle.InvalidateLayout("MrBones", preview.LifecycleRevision, time.Now().UTC()); invalidateErr != nil {
		t.Fatal(invalidateErr)
	}
	calls := 0
	backend.SetSelectionHandler(func(app.CharacterSelectionRequest) error { calls++; return nil })
	_, err = backend.Command("apply_selection", selectionCommand(t, preview, 0))
	var commandErr *commandError
	if !errors.As(err, &commandErr) || commandErr.code != "selection_confirmation_invalid" || calls != 0 {
		t.Fatalf("stale preview err=%v calls=%d", err, calls)
	}
}

func TestSelectionReloadsSaveProjectionAndStopsChangedClassBeforeInput(t *testing.T) {
	backend := newSelectionTestBackend(t)
	preview, err := backend.PreviewSelection(SelectionPreviewRequest{Character: "MrBones", Difficulty: "nightmare", CatalogRevision: 1})
	if err != nil {
		t.Fatal(err)
	}
	changed := app.CharacterCatalog{Revision: 2, Characters: []app.CharacterCatalogEntry{{
		Name: "MrBones", Slug: "mrbones", ExpectedClass: "paladin",
		Reasons: []string{app.CharacterReasonClassUnsupported},
	}}}
	backend.mu.Lock()
	backend.characterCatalogReload = func() (app.CharacterCatalog, error) { return changed, nil }
	backend.mu.Unlock()
	calls := 0
	backend.SetSelectionHandler(func(app.CharacterSelectionRequest) error { calls++; return nil })

	_, err = backend.Command("apply_selection", selectionCommand(t, preview, 0))
	var commandErr *commandError
	if !errors.As(err, &commandErr) || commandErr.code != "selection_confirmation_invalid" || calls != 0 {
		t.Fatalf("changed save err=%v calls=%d", err, calls)
	}
}

func TestSelectionFailureLeavesLifecycleUnchanged(t *testing.T) {
	backend := newSelectionTestBackend(t)
	preview, err := backend.PreviewSelection(SelectionPreviewRequest{Character: "MrBones", Difficulty: "nightmare", CatalogRevision: 1})
	if err != nil {
		t.Fatal(err)
	}
	before, _, err := backend.lifecycle.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	backend.SetSelectionHandler(func(app.CharacterSelectionRequest) error { return errors.New("memory confirmation failed") })
	if _, commandErr := backend.Command("apply_selection", selectionCommand(t, preview, 0)); commandErr == nil {
		t.Fatal("unconfirmed game entry was accepted")
	}
	after, _, err := backend.lifecycle.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if after.Revision != before.Revision || after.Characters["mrbones"].LastConfirmedDifficulty != before.Characters["mrbones"].LastConfirmedDifficulty {
		t.Fatalf("failed selection changed lifecycle: before=%+v after=%+v", before, after)
	}
}

func TestSelectionCommitsDifficultyInvalidationAfterVerifiedInput(t *testing.T) {
	backend := newSelectionTestBackend(t)
	exact, err := backend.lifecycle.Preview("MrBones", "nightmare")
	if err != nil {
		t.Fatal(err)
	}
	if _, confirmErr := backend.lifecycle.Confirm(exact, time.Now().UTC()); confirmErr != nil {
		t.Fatal(confirmErr)
	}
	preview, err := backend.PreviewSelection(SelectionPreviewRequest{Character: "MrBones", Difficulty: "hell", CatalogRevision: 1})
	if err != nil {
		t.Fatal(err)
	}
	called := false
	backend.SetSelectionHandler(func(app.CharacterSelectionRequest) error { called = true; return nil })
	if _, commandErr := backend.Command("apply_selection", selectionCommand(t, preview, 0)); commandErr != nil {
		t.Fatal(commandErr)
	}
	manifest, catalog, err := backend.lifecycle.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if !called || manifest.Characters["mrbones"].LastConfirmedDifficulty != "hell" || backend.Status().Selection.Difficulty != "hell" {
		t.Fatalf("selection not committed: called=%t manifest=%+v status=%+v", called, manifest.Characters["mrbones"], backend.Status())
	}
	for _, entry := range catalog.Entries {
		if entry.Character == "MrBones" && entry.Status != app.RouteLifecycleStale {
			t.Fatalf("affected route remained available: %+v", entry)
		}
	}
}

func newSelectionTestBackend(t *testing.T) *LiveBackend {
	t.Helper()
	cfg, err := config.Load("../../configs/config.example.yaml")
	if err != nil {
		t.Fatal(err)
	}
	cfg.Session.Character = "MrBones"
	cfg.Session.Difficulty = "nightmare"
	cfg.Routes.LifecycleFile = filepath.Join(t.TempDir(), "route-lifecycle.local.yaml")
	backend, err := NewLiveBackend(cfg, telemetry.NewLivePublisher(32, 8))
	if err != nil {
		t.Fatal(err)
	}
	markBackendCompatible(backend)
	configureDesktopCharacterContract(t, backend, cfg)
	backend.mu.Lock()
	backend.status.Input = InputDTO{Enabled: true}
	backend.mu.Unlock()
	return backend
}

func TestSelectionRequiresEffectiveRuntimeInputBeforeHandler(t *testing.T) {
	backend := newSelectionTestBackend(t)
	preview, err := backend.PreviewSelection(SelectionPreviewRequest{Character: "MrBones", Difficulty: "nightmare", CatalogRevision: 1})
	if err != nil {
		t.Fatal(err)
	}
	backend.mu.Lock()
	backend.status.Input = InputDTO{Enabled: false}
	backend.mu.Unlock()
	calls := 0
	backend.SetSelectionHandler(func(app.CharacterSelectionRequest) error {
		calls++
		return nil
	})

	_, err = backend.Command("apply_selection", selectionCommand(t, preview, 0))
	var commandErr *commandError
	if !errors.As(err, &commandErr) || commandErr.code != "input_not_ready" || calls != 0 {
		t.Fatalf("err=%v calls=%d, want input_not_ready before handler", err, calls)
	}
}

func TestDesktopCharacterContractRejectsRunProfileMismatchBeforeQueueValidation(t *testing.T) {
	cfg, err := config.Load("../../configs/config.example.yaml")
	if err != nil {
		t.Fatal(err)
	}
	cfg.Routes.LifecycleFile = filepath.Join(t.TempDir(), "route-lifecycle.local.yaml")
	backend, err := NewLiveBackend(cfg, telemetry.NewLivePublisher(8, 2))
	if err != nil {
		t.Fatal(err)
	}
	different := cfg.Profiles["necro_bone_spear"]
	different.Setup.Default = false
	cfg.Profiles["different_profile"] = different
	configureDesktopCharacterContract(t, backend, cfg, "countess")
	settings, err := backend.operatorSettings.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if _, assignErr := backend.operatorSettings.AssignCharacterProfile("MrBones", "necromancer", "different_profile", settings.Revision); assignErr != nil {
		t.Fatal(assignErr)
	}
	setBackendCharacterCatalogProjection(backend, app.CharacterCatalogEntry{
		Name: "MrBones", Slug: "mrbones", ExpectedClass: "necromancer", CombatProfile: "different_profile",
		Selectable: true, AnchorPath: "anchor.png",
	})

	_, _, err = backend.validateDesktopCharacterContract("MrBones", []string{"countess"})
	var commandErr *commandError
	if !errors.As(err, &commandErr) || commandErr.code != string(tasks.RunReasonProfileRunStrategyUnavailable) {
		t.Fatalf("strategy mismatch error = %v", err)
	}
}

func TestDesktopCharacterContractChecksPickitOnlyForRequestedRun(t *testing.T) {
	cfg, err := config.Load("../../configs/config.example.yaml")
	if err != nil {
		t.Fatal(err)
	}
	cfg.Routes.LifecycleFile = filepath.Join(t.TempDir(), "route-lifecycle.local.yaml")
	backend, err := NewLiveBackend(cfg, telemetry.NewLivePublisher(8, 2))
	if err != nil {
		t.Fatal(err)
	}
	configureDesktopCharacterContract(t, backend, cfg, "countess")
	if _, _, err = backend.validateDesktopCharacterContract("MrBones", []string{"countess"}); err != nil {
		t.Fatalf("configured run rejected: %v", err)
	}
	_, _, err = backend.validateDesktopCharacterContract("MrBones", []string{"mephisto"})
	var commandErr *commandError
	if !errors.As(err, &commandErr) || commandErr.code != "pickit_assignment_invalid" {
		t.Fatalf("missing Mephisto pickit error = %v", err)
	}
}

func selectionCommand(t *testing.T, preview SelectionPreviewDTO, generation uint64) CommandRequest {
	t.Helper()
	payload, err := json.Marshal(map[string]any{
		"character": preview.Character, "difficulty": preview.NewDifficulty,
		"catalog_revision": preview.CatalogRevision, "confirmation_token": preview.ConfirmationToken,
	})
	if err != nil {
		t.Fatal(err)
	}
	return CommandRequest{CommandID: "selection-" + preview.ConfirmationToken, ExpectedGeneration: generation, Payload: payload}
}

func containsLiveEvent(events []telemetry.LiveEvent, name string) bool {
	for _, event := range events {
		if event.Event == name {
			return true
		}
	}
	return false
}

func markBackendCompatible(backend *LiveBackend) {
	backend.UpdateRuntime(app.UIStatusSnapshot{ProcessState: "attached", Compatibility: compatibleRuntimeSnapshot()})
}

func compatibleRuntimeSnapshot() app.D2RCompatibilitySnapshot {
	return app.D2RCompatibilitySnapshot{State: app.D2RCompatibilityCompatible, SupportedVersion: "3.2.92777", ExpectedVersion: "3.2.92777", OffsetVersion: "3.2.92777", ActualVersion: "3.2.92777"}
}

func configureDesktopCharacterContract(t *testing.T, backend *LiveBackend, cfg *config.Config, runIDs ...string) {
	t.Helper()
	settings, err := app.NewOperatorSettingsStore(t.TempDir(), cfg, []string{"MrBones"})
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := settings.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	assigned, err := settings.AssignCharacterProfile("MrBones", "necromancer", "necro_bone_spear", snapshot.Revision)
	if err != nil {
		t.Fatal(err)
	}
	replacement := assigned.Settings
	value := replacement.Characters["mrbones"]
	value.ProfileBindings = map[string]app.OperatorProfileBindings{
		"necro_bone_spear": {
			Skills: map[string]string{
				"teleport": "f7", "town_portal": "f6", "amplify_damage": "f1", "corpse_explosion": "f2",
				"bone_prison": "f3", "bone_armor": "f5", "bone_spear": "f8",
			},
			Belt: app.OperatorBeltBindings{Slot1: "1", Slot2: "2", Slot3: "3", Slot4: "4"},
		},
	}
	value.InventoryLock = &app.OperatorInventoryLock{Grid: sampleAPIInventoryGrid()}
	replacement.Characters["mrbones"] = value
	if _, err = settings.Update(assigned.Settings.Revision, replacement); err != nil {
		t.Fatal(err)
	}
	if err = backend.SetOperatorSettingsStore(settings); err != nil {
		t.Fatal(err)
	}
	backend.SetLoadoutResolver(app.NewCharacterLoadoutResolver(settings, cfg.Profiles, replacement.Input))
	assignments, err := app.NewPickitAssignmentStore(filepath.Join(t.TempDir(), "pickit-assignments.local.yaml"), backend.pickitProfiles)
	if err != nil {
		t.Fatal(err)
	}
	values := make(map[string]map[tasks.RunID][]string)
	if len(runIDs) > 0 {
		values["mrbones"] = make(map[tasks.RunID][]string, len(runIDs))
		for _, runID := range runIDs {
			values["mrbones"][tasks.RunID(runID)] = append([]string(nil), cfg.CharacterSetup.PickitDefaults[runID]...)
		}
	}
	if _, err = assignments.Initialize(values); err != nil {
		t.Fatal(err)
	}
	backend.mu.Lock()
	backend.pickitAssignments = assignments
	backend.mu.Unlock()
	setBackendCharacterCatalogProjection(backend, app.CharacterCatalogEntry{
		Name: "MrBones", Slug: "mrbones", ExpectedClass: "necromancer", CombatProfile: "necro_bone_spear",
		Selectable: true, AnchorPath: "anchor.png",
	})
}

func configureHammerdinRecordingContext(t *testing.T, backend *LiveBackend, cfg *config.Config) {
	t.Helper()
	settings, err := app.NewOperatorSettingsStore(t.TempDir(), cfg, []string{"MrHammer"})
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := settings.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	assigned, err := settings.AssignCharacterProfile("MrHammer", "paladin", "paladin_hammerdin", snapshot.Revision)
	if err != nil {
		t.Fatal(err)
	}
	replacement := assigned.Settings
	replacement.LastCharacter = "MrHammer"
	value := replacement.Characters["mrhammer"]
	value.LastDifficulty = "hell"
	value.ProfileBindings = map[string]app.OperatorProfileBindings{
		"paladin_hammerdin": {
			Skills: map[string]string{
				"blessed_hammer": "f1", "concentration": "f2", "teleport": "f3",
				"holy_shield": "f4", "town_portal": "f5", "battle_command": "f6", "battle_orders": "f7",
			},
			Belt: app.OperatorBeltBindings{Slot1: "1", Slot2: "2", Slot3: "3", Slot4: "4"},
		},
	}
	value.InventoryLock = &app.OperatorInventoryLock{Grid: sampleAPIInventoryGrid()}
	replacement.Characters["mrhammer"] = value
	if _, err = settings.Update(assigned.Settings.Revision, replacement); err != nil {
		t.Fatal(err)
	}
	if err = backend.SetOperatorSettingsStore(settings); err != nil {
		t.Fatal(err)
	}
	backend.SetLoadoutResolver(app.NewCharacterLoadoutResolver(settings, cfg.Profiles, replacement.Input))
	assignments, err := app.NewPickitAssignmentStore(filepath.Join(t.TempDir(), "pickit-assignments.local.yaml"), backend.pickitProfiles)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = assignments.Initialize(map[string]map[tasks.RunID][]string{
		"MrHammer": {tasks.RunIDMephisto: append([]string(nil), cfg.CharacterSetup.PickitDefaults["mephisto"]...)},
	}); err != nil {
		t.Fatal(err)
	}
	backend.mu.Lock()
	backend.pickitAssignments = assignments
	backend.mu.Unlock()
}

func TestCatalogFarmReadyDependsOnProfileBindings(t *testing.T) {
	cfg, err := config.Load("../../configs/config.example.yaml")
	if err != nil {
		t.Fatal(err)
	}
	cfg.Routes.LifecycleFile = filepath.Join(t.TempDir(), "route-lifecycle.local.yaml")
	backend, err := NewLiveBackend(cfg, telemetry.NewLivePublisher(16, 4))
	if err != nil {
		t.Fatal(err)
	}
	settings, err := app.NewOperatorSettingsStore(t.TempDir(), cfg, []string{"MrBones"})
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := settings.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	assigned, err := settings.AssignCharacterProfile("MrBones", "necromancer", "necro_bone_spear", snapshot.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if err = backend.SetOperatorSettingsStore(settings); err != nil {
		t.Fatal(err)
	}
	setBackendCharacterCatalogProjection(backend, app.CharacterCatalogEntry{
		Name: "MrBones", Slug: "mrbones", ExpectedClass: "necromancer", CombatProfile: "necro_bone_spear",
		Selectable: true, AnchorPath: "anchor.png",
	})
	incomplete := backend.Catalog().Characters[0]
	if incomplete.FarmReady || len(incomplete.FarmReadyReasons) != 1 || incomplete.FarmReadyReasons[0] != string(app.QueueReasonProfileBindingsIncomplete) {
		t.Fatalf("incomplete farm ready=%+v", incomplete)
	}

	replacement := assigned.Settings
	value := replacement.Characters["mrbones"]
	value.ProfileBindings = map[string]app.OperatorProfileBindings{
		"necro_bone_spear": {
			Skills: map[string]string{
				"teleport": "f7", "town_portal": "f6", "amplify_damage": "f1", "corpse_explosion": "f2",
				"bone_prison": "f3", "bone_armor": "f5", "bone_spear": "f8",
			},
			Belt: app.OperatorBeltBindings{Slot1: "1", Slot2: "2", Slot3: "3", Slot4: "4"},
		},
	}
	replacement.Characters["mrbones"] = value
	updated, err := settings.Update(assigned.Settings.Revision, replacement)
	if err != nil {
		t.Fatal(err)
	}
	bindingsOnly := backend.Catalog().Characters[0]
	if bindingsOnly.FarmReady || len(bindingsOnly.FarmReadyReasons) != 1 || bindingsOnly.FarmReadyReasons[0] != string(app.QueueReasonCharacterInventoryUnconfigured) {
		t.Fatalf("inventory-unconfigured farm ready=%+v", bindingsOnly)
	}

	replacement = updated.Settings
	value = replacement.Characters["mrbones"]
	value.InventoryLock = &app.OperatorInventoryLock{Grid: sampleAPIInventoryGrid()}
	replacement.Characters["mrbones"] = value
	if _, err = settings.Update(updated.Settings.Revision, replacement); err != nil {
		t.Fatal(err)
	}
	ready := backend.Catalog().Characters[0]
	if !ready.FarmReady || len(ready.FarmReadyReasons) != 0 {
		t.Fatalf("ready farm ready=%+v", ready)
	}
}

func TestRecordingPrerequisitesUseSchema3ProfileBindings(t *testing.T) {
	cfg, err := config.Load("../../configs/config.example.yaml")
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	cfg.Routes.FarmingRoot = filepath.Join(root, "farming")
	cfg.Routes.CandidateRoot = filepath.Join(root, "candidates")
	cfg.Routes.LifecycleFile = filepath.Join(root, "lifecycle.yaml")
	cfg.Routes.AssignmentsFile = filepath.Join(root, "assignments.yaml")
	cfg.Routes.RecoveryFile = filepath.Join(root, "recovery.yaml")
	backend, err := NewLiveBackend(cfg, telemetry.NewLivePublisher(16, 4))
	if err != nil {
		t.Fatal(err)
	}
	backend.mu.Lock()
	backend.status.Selection = SelectionStatusDTO{Character: "MrBones", Difficulty: "nightmare"}
	backend.mu.Unlock()

	if recordingPrerequisiteReady(backend.RecordingOptions(""), "countess", "teleport") || recordingPrerequisiteReady(backend.RecordingOptions(""), "countess", "town_portal") {
		t.Fatal("recording skill prerequisites must stay unready without Schema 3 profile bindings")
	}

	configureDesktopCharacterContract(t, backend, cfg, "countess")
	if !recordingPrerequisiteReady(backend.RecordingOptions(""), "countess", "teleport") || !recordingPrerequisiteReady(backend.RecordingOptions(""), "countess", "town_portal") {
		t.Fatal("recording skill prerequisites must become ready from Schema 3 profile bindings")
	}

	settings := backend.operatorSettings
	snapshot, err := settings.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	replacement := snapshot
	value := replacement.Characters["mrbones"]
	bindings := value.ProfileBindings["necro_bone_spear"]
	delete(bindings.Skills, "teleport")
	value.ProfileBindings["necro_bone_spear"] = bindings
	replacement.Characters["mrbones"] = value
	if _, err = settings.Update(snapshot.Revision, replacement); err != nil {
		t.Fatal(err)
	}
	if recordingPrerequisiteReady(backend.RecordingOptions(""), "countess", "teleport") {
		t.Fatal("teleport prerequisite must reflect removed Schema 3 binding")
	}
	if !recordingPrerequisiteReady(backend.RecordingOptions(""), "countess", "town_portal") {
		t.Fatal("town portal prerequisite must remain ready when only teleport is missing")
	}
}

func TestRecordingPrerequisitesUseSelectedCharacterWithoutConfirmedSelection(t *testing.T) {
	cfg, err := config.Load("../../configs/config.example.yaml")
	if err != nil {
		t.Fatal(err)
	}
	cfg.Input.Enabled = true
	root := t.TempDir()
	cfg.Routes.FarmingRoot = filepath.Join(root, "farming")
	cfg.Routes.CandidateRoot = filepath.Join(root, "candidates")
	cfg.Routes.LifecycleFile = filepath.Join(root, "lifecycle.yaml")
	cfg.Routes.AssignmentsFile = filepath.Join(root, "assignments.yaml")
	cfg.Routes.RecoveryFile = filepath.Join(root, "recovery.yaml")
	backend, err := NewLiveBackend(cfg, telemetry.NewLivePublisher(16, 4))
	if err != nil {
		t.Fatal(err)
	}
	configureHammerdinRecordingContext(t, backend, cfg)

	options := backend.RecordingOptions("MrHammer")
	if !recordingPrerequisiteReady(options, "mephisto", "teleport") || !recordingPrerequisiteReady(options, "mephisto", "town_portal") || !recordingPrerequisiteReady(options, "mephisto", "pickit") {
		t.Fatal("Hammerdin Mephisto recording must use Schema 3 bindings and Mephisto pickit without a confirmed selection")
	}
	if recordingPrerequisiteReady(options, "countess", "pickit") {
		t.Fatal("Countess pickit must stay unready when only Mephisto is assigned")
	}
	if !recordingOptionAvailable(options, "mephisto") {
		t.Fatal("Hammerdin Mephisto recording must be available from operator last difficulty")
	}

	backend.SetRouteWorkflowHandler(func(RouteWorkflowRequest, <-chan struct{}, app.RouteWorkflowReporter) error { return nil })
	markBackendCompatible(backend)
	snapshot, err := backend.StartRouteWorkflow(RouteWorkflowRequest{ExpectedGeneration: 1, Operation: "record", RunID: "mephisto", Character: "MrHammer"})
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Character != "MrHammer" {
		t.Fatalf("workflow character = %q, want MrHammer", snapshot.Character)
	}
}

func freezeHammerdinMephistoCandidate(t *testing.T, backend *LiveBackend, cfg *config.Config) app.RouteCandidate {
	t.Helper()
	_, catalog, err := backend.lifecycle.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	assignment, err := backend.routeAssignments.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	seed := uint32(42)
	route := pathing.Route{
		Version: 1, ID: "hammerdin-mephisto-candidate", Name: "Mephisto-Entwurf", Kind: pathing.RouteKindNavigation,
		Binding: pathing.RouteBinding{
			CharacterName: "MrHammer", CharacterClass: "paladin", Difficulty: pathing.RouteDifficultyHell,
			MapSeed: &seed, GameVersion: cfg.Memory.GameVersion,
			LayoutFingerprint: pathing.RouteLayoutFingerprint{Version: 1, AreaID: world.DuranceOfHateLevel2, AnchorCount: 1, Hash: strings.Repeat("c", 64)},
		},
		Recording: pathing.RouteRecording{RecordedAt: time.Now().UTC(), SampleDistanceTiles: 4},
		Playback:  pathing.RoutePlayback{WaypointToleranceTiles: 3, MaxDriftTiles: 8, MaxLocalCorrections: 2, SegmentTimeoutMs: 30000, TransitionTimeoutMs: 10000},
		Segments: []pathing.RouteSegment{{
			ID: "durance-level-2", FromAreaID: world.DuranceOfHateLevel2, ToAreaID: world.DuranceOfHateLevel3, Movement: pathing.RouteMovementTeleport,
			Points:     []pathing.RoutePoint{{X: 100, Y: 100}, {X: 110, Y: 110}},
			Transition: pathing.RouteTransition{Type: "entrance", EntranceKind: "durance_down"},
		}, {
			ID: "durance-level-3", FromAreaID: world.DuranceOfHateLevel3, ToAreaID: world.DuranceOfHateLevel3, Movement: pathing.RouteMovementTeleport,
			Points:     []pathing.RoutePoint{{X: 120, Y: 120}, {X: 125, Y: 125}},
			Transition: pathing.RouteTransition{Type: "terminal"},
		}},
	}
	candidate, err := backend.routeCandidates.Freeze(route, app.RouteCandidate{
		RunID: tasks.RunIDMephisto, Character: "MrHammer", Difficulty: "hell", GameVersion: cfg.Memory.GameVersion,
		State: app.RouteCandidateRecorded, MeasuredBossDistance: 18, SourceCatalogRevision: catalog.Revision,
		SourceAssignmentRevision: assignment.Revision, CreatedAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatal(err)
	}
	return candidate
}

func TestCandidateTestUsesDraftContextWithoutConfirmedSelection(t *testing.T) {
	cfg, err := config.Load("../../configs/config.example.yaml")
	if err != nil {
		t.Fatal(err)
	}
	cfg.Input.Enabled = true
	root := t.TempDir()
	cfg.Routes.FarmingRoot = filepath.Join(root, "farming")
	cfg.Routes.CandidateRoot = filepath.Join(root, "candidates")
	cfg.Routes.LifecycleFile = filepath.Join(root, "lifecycle.yaml")
	cfg.Routes.AssignmentsFile = filepath.Join(root, "assignments.yaml")
	cfg.Routes.RecoveryFile = filepath.Join(root, "recovery.yaml")
	backend, err := NewLiveBackend(cfg, telemetry.NewLivePublisher(16, 4))
	if err != nil {
		t.Fatal(err)
	}
	configureHammerdinRecordingContext(t, backend, cfg)
	candidate := freezeHammerdinMephistoCandidate(t, backend, cfg)
	started := make(chan RouteWorkflowRequest, 1)
	backend.SetRouteWorkflowHandler(func(request RouteWorkflowRequest, _ <-chan struct{}, _ app.RouteWorkflowReporter) error {
		started <- request
		return nil
	})
	markBackendCompatible(backend)
	backend.mu.Lock()
	backend.status.State = string(app.SupervisorStateIdle)
	backend.status.Selection = SelectionStatusDTO{}
	backend.mu.Unlock()

	if _, err = backend.PreviewRouteMutation(RouteMutationPreviewRequest{CandidateID: candidate.CandidateID, Operation: string(app.RouteMutationDeleteCandidate)}); err != nil {
		t.Fatalf("preview without confirmed selection: %v", err)
	}
	snapshot, err := backend.StartRouteWorkflow(RouteWorkflowRequest{ExpectedGeneration: 1, Operation: "test", CandidateID: candidate.CandidateID})
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Character != "MrHammer" {
		t.Fatalf("workflow character = %q, want MrHammer", snapshot.Character)
	}
	select {
	case got := <-started:
		if got.Character != "MrHammer" || got.Difficulty != "hell" || got.CandidateID != candidate.CandidateID {
			t.Fatalf("handler request = %+v", got)
		}
	case <-time.After(time.Second):
		t.Fatal("candidate test handler was not called")
	}
}

func TestCandidateTestRejectsConflictingConfirmedSelection(t *testing.T) {
	cfg, err := config.Load("../../configs/config.example.yaml")
	if err != nil {
		t.Fatal(err)
	}
	cfg.Input.Enabled = true
	root := t.TempDir()
	cfg.Routes.FarmingRoot = filepath.Join(root, "farming")
	cfg.Routes.CandidateRoot = filepath.Join(root, "candidates")
	cfg.Routes.LifecycleFile = filepath.Join(root, "lifecycle.yaml")
	cfg.Routes.AssignmentsFile = filepath.Join(root, "assignments.yaml")
	cfg.Routes.RecoveryFile = filepath.Join(root, "recovery.yaml")
	backend, err := NewLiveBackend(cfg, telemetry.NewLivePublisher(16, 4))
	if err != nil {
		t.Fatal(err)
	}
	configureHammerdinRecordingContext(t, backend, cfg)
	candidate := freezeHammerdinMephistoCandidate(t, backend, cfg)
	backend.SetRouteWorkflowHandler(func(RouteWorkflowRequest, <-chan struct{}, app.RouteWorkflowReporter) error {
		return nil
	})
	markBackendCompatible(backend)
	backend.mu.Lock()
	backend.status.State = string(app.SupervisorStateIdle)
	backend.status.Selection = SelectionStatusDTO{Character: "MrBones", Difficulty: "nightmare"}
	backend.mu.Unlock()

	_, err = backend.StartRouteWorkflow(RouteWorkflowRequest{ExpectedGeneration: 1, Operation: "test", CandidateID: candidate.CandidateID})
	if err == nil || !strings.Contains(err.Error(), "live candidate context changed") {
		t.Fatalf("err = %v, want live candidate context changed", err)
	}
}

func recordingOptionAvailable(options []RecordingOptionDTO, runID string) bool {
	for _, option := range options {
		if option.RunID == runID {
			return option.Available
		}
	}
	return false
}

func TestValidateQueueRejectsIncompleteAndUnconfiguredLoadout(t *testing.T) {
	cfg, err := config.Load("../../configs/config.example.yaml")
	if err != nil {
		t.Fatal(err)
	}
	cfg.Routes.LifecycleFile = filepath.Join(t.TempDir(), "route-lifecycle.local.yaml")
	writeAPITestAssignments(t, cfg)
	backend, err := NewLiveBackend(cfg, telemetry.NewLivePublisher(16, 4))
	if err != nil {
		t.Fatal(err)
	}
	configureDesktopCharacterContract(t, backend, cfg, "countess")
	backend.mu.Lock()
	backend.status.Selection = SelectionStatusDTO{Character: "MrBones", Difficulty: "nightmare"}
	revision := backend.catalog.Revision
	backend.mu.Unlock()

	settings := backend.operatorSettings
	snapshot, err := settings.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	replacement := snapshot
	value := replacement.Characters["mrbones"]
	value.InventoryLock = nil
	replacement.Characters["mrbones"] = value
	updated, err := settings.Update(snapshot.Revision, replacement)
	if err != nil {
		t.Fatal(err)
	}
	_, err = backend.ValidateQueue(QueueValidationRequest{
		Entries: []string{"countess"}, Character: "MrBones", Difficulty: "nightmare", CatalogRevision: revision,
	})
	var commandErr *commandError
	if !errors.As(err, &commandErr) || commandErr.code != string(app.QueueReasonCharacterInventoryUnconfigured) {
		t.Fatalf("inventory gate error=%v", err)
	}

	replacement = updated.Settings
	value = replacement.Characters["mrbones"]
	value.ProfileBindings = map[string]app.OperatorProfileBindings{
		"necro_bone_spear": {Skills: map[string]string{"teleport": "f7"}, Belt: app.OperatorBeltBindings{Slot1: "1", Slot2: "2", Slot3: "3", Slot4: "4"}},
	}
	value.InventoryLock = &app.OperatorInventoryLock{Grid: sampleAPIInventoryGrid()}
	replacement.Characters["mrbones"] = value
	if _, err = settings.Update(updated.Settings.Revision, replacement); err != nil {
		t.Fatal(err)
	}
	_, err = backend.ValidateQueue(QueueValidationRequest{
		Entries: []string{"countess"}, Character: "MrBones", Difficulty: "nightmare", CatalogRevision: revision,
	})
	if !errors.As(err, &commandErr) || commandErr.code != string(app.QueueReasonProfileBindingsIncomplete) {
		t.Fatalf("bindings gate error=%v", err)
	}
}

func TestValidateQueueAllowsSavedAllLockedInventoryForCountess(t *testing.T) {
	cfg, err := config.Load("../../configs/config.example.yaml")
	if err != nil {
		t.Fatal(err)
	}
	cfg.Routes.LifecycleFile = filepath.Join(t.TempDir(), "route-lifecycle.local.yaml")
	writeAPITestAssignments(t, cfg)
	backend, err := NewLiveBackend(cfg, telemetry.NewLivePublisher(16, 4))
	if err != nil {
		t.Fatal(err)
	}
	configureDesktopCharacterContract(t, backend, cfg, "countess")
	backend.mu.Lock()
	backend.status.Selection = SelectionStatusDTO{Character: "MrBones", Difficulty: "nightmare"}
	revision := backend.catalog.Revision
	backend.mu.Unlock()

	settings := backend.operatorSettings
	snapshot, err := settings.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	replacement := snapshot
	value := replacement.Characters["mrbones"]
	value.InventoryLock = &app.OperatorInventoryLock{Grid: sampleAPIInventoryGridAllLocked()}
	replacement.Characters["mrbones"] = value
	if _, err = settings.Update(snapshot.Revision, replacement); err != nil {
		t.Fatal(err)
	}

	validation, err := backend.ValidateQueue(QueueValidationRequest{
		Entries: []string{"countess"}, Character: "MrBones", Difficulty: "nightmare", CatalogRevision: revision,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(validation.Warnings) != 0 {
		t.Fatalf("Countess-only all-locked warnings=%v", validation.Warnings)
	}
}

func TestStartQueueDoesNotPaperOverBeginQueueFreezeFailure(t *testing.T) {
	cfg, err := config.Load("../../configs/config.example.yaml")
	if err != nil {
		t.Fatal(err)
	}
	cfg.Routes.LifecycleFile = filepath.Join(t.TempDir(), "route-lifecycle.local.yaml")
	writeAPITestAssignments(t, cfg)
	backend, err := NewLiveBackend(cfg, telemetry.NewLivePublisher(16, 4))
	if err != nil {
		t.Fatal(err)
	}
	configureDesktopCharacterContract(t, backend, cfg, "countess")
	markBackendCompatible(backend)
	backend.mu.Lock()
	backend.status.Selection = SelectionStatusDTO{Character: "MrBones", Difficulty: "nightmare"}
	revision := backend.catalog.Revision
	backend.mu.Unlock()
	runner := &liveBackendQueueRunner{started: make(chan app.SupervisorRunRequest, 1), release: make(chan app.SupervisorRunResult, 1)}
	supervisor, err := app.NewSessionSupervisor(runner)
	if err != nil {
		t.Fatal(err)
	}
	if setupErr := backend.SetSessionSupervisor(supervisor, nil, func(bool) error {
		return errors.New("freeze exploded")
	}); setupErr != nil {
		t.Fatal(setupErr)
	}
	payload, _ := json.Marshal(SessionStartPayload{Entries: []string{"countess"}, Character: "MrBones", Difficulty: "nightmare", CatalogRevision: revision})
	_, err = backend.Command("start_queue", CommandRequest{CommandID: "freeze-fail", ExpectedGeneration: backend.Status().Generation, Payload: payload})
	var commandErr *commandError
	if err == nil || !errors.As(err, &commandErr) {
		t.Fatalf("expected freeze failure, got %v", err)
	}
	if commandErr.code == string(app.QueueReasonProfileBindingsIncomplete) {
		t.Fatalf("freeze failure papered over as bindings incomplete: %+v", commandErr)
	}
	if commandErr.code != "command_conflict" || commandErr.params["operation"] != "freeze_loadout" || commandErr.cause == nil {
		t.Fatalf("freeze failure error=%+v", commandErr)
	}
}

func recordingPrerequisiteReady(options []RecordingOptionDTO, runID, prerequisiteID string) bool {
	for _, option := range options {
		if option.RunID != runID {
			continue
		}
		for _, prerequisite := range option.Prerequisites {
			if prerequisite.ID == prerequisiteID {
				return prerequisite.Ready
			}
		}
	}
	return false
}

func TestQueueLoadoutWarningsForUnsuitableCowInventory(t *testing.T) {
	cfg, err := config.Load("../../configs/config.example.yaml")
	if err != nil {
		t.Fatal(err)
	}
	cfg.Routes.LifecycleFile = filepath.Join(t.TempDir(), "route-lifecycle.local.yaml")
	backend, err := NewLiveBackend(cfg, telemetry.NewLivePublisher(16, 4))
	if err != nil {
		t.Fatal(err)
	}
	configureDesktopCharacterContract(t, backend, cfg, "countess")
	settings := backend.operatorSettings
	snapshot, err := settings.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	replacement := snapshot
	value := replacement.Characters["mrbones"]
	value.InventoryLock = &app.OperatorInventoryLock{Grid: sampleAPIInventoryGridAllLocked()}
	replacement.Characters["mrbones"] = value
	if _, err = settings.Update(snapshot.Revision, replacement); err != nil {
		t.Fatal(err)
	}
	if warnings := backend.queueLoadoutWarnings("MrBones", []string{"cows"}); len(warnings) != 1 || warnings[0] != "inventory_layout_unsuitable_for_cows" {
		t.Fatalf("cow warnings=%v", warnings)
	}
	if warnings := backend.queueLoadoutWarnings("MrBones", []string{"countess"}); len(warnings) != 0 {
		t.Fatalf("countess warnings=%v", warnings)
	}
}

func sampleAPIInventoryGrid() [][]int {
	grid := make([][]int, 4)
	for row := range grid {
		grid[row] = make([]int, 10)
		for col := 0; col < 4; col++ {
			grid[row][col] = 1
		}
	}
	return grid
}

func sampleAPIInventoryGridAllLocked() [][]int {
	grid := make([][]int, 4)
	for row := range grid {
		grid[row] = []int{1, 1, 1, 1, 1, 1, 1, 1, 1, 1}
	}
	return grid
}

func setBackendCharacterCatalogProjection(backend *LiveBackend, entry app.CharacterCatalogEntry) {
	catalog := app.CharacterCatalog{Revision: 1, Characters: []app.CharacterCatalogEntry{entry}}
	backend.mu.Lock()
	backend.characterCatalogReload = func() (app.CharacterCatalog, error) { return catalog, nil }
	backend.mu.Unlock()
	backend.publishCharacterCatalog(catalog, false)
}

func TestRouteWorkflowEventProjectsSystemAct(t *testing.T) {
	event := routeWorkflowEvent("route_workflow_changed", RouteWorkflowDTO{
		WorkflowID: "egress-1",
		Generation: 2,
		State:      string(app.RouteWorkflowPreflight),
		Act:        "act2",
		Reason:     "town_egress_start_unconfirmed",
	})
	if event.Act != "act2" || event.State != string(app.RouteWorkflowPreflight) || event.Reason != "town_egress_start_unconfirmed" {
		t.Fatalf("system workflow diagnostics were not projected: %+v", event)
	}
}

func TestCompatibilityBlocksInputCommandsAndPublishesStableReason(t *testing.T) {
	backend := newSelectionTestBackend(t)
	preview, err := backend.PreviewSelection(SelectionPreviewRequest{Character: "MrBones", Difficulty: "nightmare", CatalogRevision: 1})
	if err != nil {
		t.Fatal(err)
	}
	called := 0
	backend.SetSelectionHandler(func(app.CharacterSelectionRequest) error { called++; return nil })
	backend.UpdateRuntime(app.UIStatusSnapshot{
		ProcessState: "attached",
		Compatibility: app.D2RCompatibilitySnapshot{
			State: app.D2RCompatibilityIncompatible, Reason: app.Phase15ReasonD2RVersionUnsupported,
			SupportedVersion: "3.2.92777", ExpectedVersion: "3.2.92777", OffsetVersion: "3.2.92777", ActualVersion: "3.3.1",
		},
	})
	_, commandErr := backend.Command("apply_selection", selectionCommand(t, preview, backend.Status().Generation))
	var typed *commandError
	if !errors.As(commandErr, &typed) || typed.code != string(app.Phase15ReasonD2RVersionUnsupported) || called != 0 {
		t.Fatalf("selection error=%v calls=%d", commandErr, called)
	}
	if _, workflowErr := backend.StartRouteWorkflow(RouteWorkflowRequest{ExpectedGeneration: 1, Operation: "record", RunID: "countess"}); !errors.As(workflowErr, &typed) || typed.code != string(app.Phase15ReasonD2RVersionUnsupported) {
		t.Fatalf("workflow error=%v", workflowErr)
	}
	replay, subscription := backend.publisher.Subscribe(0)
	subscription.Close()
	found := false
	for _, event := range replay {
		if event.Event == "compatibility_changed" && event.Reason == string(app.Phase15ReasonD2RVersionUnsupported) {
			found = true
		}
	}
	if !found {
		t.Fatalf("compatibility SSE event missing: %+v", replay)
	}
}
