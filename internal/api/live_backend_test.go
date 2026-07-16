package api

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/app"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/telemetry"
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

func TestLiveBackendSessionCommandsUseSupervisorAndRemainIdempotent(t *testing.T) {
	cfg, err := config.Load("../../configs/config.example.yaml")
	if err != nil {
		t.Fatal(err)
	}
	cfg.Routes.LifecycleFile = filepath.Join(t.TempDir(), "route-lifecycle.local.yaml")
	cfg.Session.MaxRuns = 4
	countess, _ := cfg.Runs.Run("countess")
	countess.RouteID = "black-marsh-cellar5-nightmare-mrbones"
	cfg.Runs.Definitions["countess"] = countess
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
	if err := backend.SetSessionSupervisor(supervisor, nil, nil); err != nil {
		t.Fatal(err)
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
	if _, err := backend.lifecycle.Confirm(exact, time.Now().UTC()); err != nil {
		t.Fatal(err)
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
	if _, err := backend.lifecycle.InvalidateLayout("MrBones", preview.LifecycleRevision, time.Now().UTC()); err != nil {
		t.Fatal(err)
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
	if _, err := backend.Command("apply_selection", selectionCommand(t, preview, 0)); err == nil {
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
	if _, err := backend.lifecycle.Confirm(exact, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	preview, err := backend.PreviewSelection(SelectionPreviewRequest{Character: "MrBones", Difficulty: "hell", CatalogRevision: 1})
	if err != nil {
		t.Fatal(err)
	}
	called := false
	backend.SetSelectionHandler(func(app.CharacterSelectionRequest) error { called = true; return nil })
	if _, err := backend.Command("apply_selection", selectionCommand(t, preview, 0)); err != nil {
		t.Fatal(err)
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
