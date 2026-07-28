package api

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
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
	runner.release <- app.SupervisorRunResult{Disposition: app.QueueRunAdvance}
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
	runner.release <- app.SupervisorRunResult{Disposition: app.QueueRunAdvance}
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
	if err := backend.SetSessionSupervisor(supervisor, nil, func(initialInGame bool) {
		beginCalled = true
		adopted = initialInGame
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
	configureDesktopCharacterContract(t, backend, cfg, "countess")
	run := cfg.Runs.Definitions["countess"]
	run.Combat.Profile = "different_profile"
	cfg.Runs.Definitions["countess"] = run

	_, _, err = backend.validateDesktopCharacterContract("MrBones", []string{"countess"})
	var commandErr *commandError
	if !errors.As(err, &commandErr) || commandErr.code != string(tasks.RunReasonCharacterProfileRunIncompatible) {
		t.Fatalf("profile mismatch error = %v", err)
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
	if _, err = settings.AssignCharacterProfile("MrBones", "necromancer", "necro_bone_spear", snapshot.Revision); err != nil {
		t.Fatal(err)
	}
	if err = backend.SetOperatorSettingsStore(settings); err != nil {
		t.Fatal(err)
	}
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
