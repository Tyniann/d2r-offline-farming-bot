package api

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/app"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
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
	if request := <-runner.started; request.RunID != "countess" || request.QueueIndex != 0 {
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
	if request := <-runner.started; request.RunID != "mephisto" || request.QueueIndex != 1 {
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
	backend.mu.Lock()
	backend.status.State = string(app.SupervisorStateIdle)
	backend.status.Selection = SelectionStatusDTO{Character: "MrBones", Difficulty: "nightmare"}
	revision := backend.catalog.Revision
	backend.mu.Unlock()
	backend.UpdateRuntime(app.UIStatusSnapshot{
		ProcessState: "attached", WindowBound: true, WorldValid: true,
		WorldPhase: "in_game", AreaID: uint32(world.RogueEncampment), AreaName: "Rogue Encampment",
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
	entry := app.CharacterCatalogEntry{Name: "MrBones", Slug: "mrbones", ExpectedClass: "necromancer", Selectable: true, AnchorPath: "anchor.png"}
	backend.bootstrap.characters = map[string]app.CharacterCatalogEntry{"mrbones": entry}
	backend.bootstrap.catalog.Characters = []CharacterCatalogEntry{{Name: "MrBones", Slug: "mrbones", Selectable: true}}
	called := 0
	backend.SetSelectionHandler(func(request app.CharacterSelectionRequest) error {
		called++
		if request.Character != "MrBones" || request.Difficulty != "nightmare" || request.CharacterCount != 1 {
			t.Fatalf("selection request = %+v", request)
		}
		return nil
	})
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
	backend.bootstrap.characters = map[string]app.CharacterCatalogEntry{"missing": {Name: "Missing", Slug: "missing", Selectable: false, Reasons: []string{app.CharacterReasonAnchorMissing}}}
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
	entry := app.CharacterCatalogEntry{Name: "MrBones", Slug: "mrbones", ExpectedClass: "necromancer", Selectable: true, AnchorPath: "anchor.png"}
	backend.bootstrap.characters = map[string]app.CharacterCatalogEntry{"mrbones": entry}
	backend.bootstrap.catalog.Characters = []CharacterCatalogEntry{{Name: "MrBones", Slug: "mrbones", Selectable: true}}
	return backend
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
