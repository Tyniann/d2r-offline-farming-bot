package app

import (
	"errors"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/tasks"
)

func TestFarmQueueCyclesCountessMephistoUntilRunBudget(t *testing.T) {
	runner := &supervisorFakeRunner{started: make(chan SupervisorRunRequest, 4), release: make(chan SupervisorRunResult, 4)}
	supervisor, _ := NewSessionSupervisor(runner)
	plan := queueSchedulerTestPlan([]string{"countess", "mephisto"}, 3)
	if _, err := supervisor.StartQueue(SupervisorCommandMeta{CommandID: "queue", ExpectedGeneration: 0}, plan); err != nil {
		t.Fatal(err)
	}
	want := []SupervisorRunRequest{
		{DefinitionID: "countess", QueueIndex: 0, Cycle: 0, Retry: 0},
		{DefinitionID: "mephisto", QueueIndex: 1, Cycle: 0, Retry: 0},
		{DefinitionID: "countess", QueueIndex: 0, Cycle: 1, Retry: 0},
	}
	got := make([]SupervisorRunRequest, 0, len(want))
	for range want {
		got = append(got, <-runner.started)
		runner.release <- SupervisorRunResult{Disposition: QueueRunAdvance, ExitAuthorization: ExitAuthorizationVerifiedRogueTown}
	}
	waitSupervisorState(t, supervisor, SupervisorStateIdle)
	seen := make(map[string]bool, len(got))
	for index := range got {
		executionID := got[index].ExecutionID
		got[index].ExecutionID = ""
		if executionID == "" || seen[executionID] {
			t.Fatalf("execution IDs are not globally unique: %+v", got)
		}
		seen[executionID] = true
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("queue sequence = %+v, want %+v", got, want)
	}
	snapshot := supervisor.Snapshot()
	if snapshot.StartedRuns != 3 || snapshot.LastResult.Reason != string(QueueReasonRunBudgetExhausted) {
		t.Fatalf("budget snapshot = %+v", snapshot)
	}
}

func TestFarmQueueRejectsDuplicateBeforeWorker(t *testing.T) {
	runner := &supervisorFakeRunner{started: make(chan SupervisorRunRequest, 1), release: make(chan SupervisorRunResult, 1)}
	supervisor, _ := NewSessionSupervisor(runner)
	_, err := supervisor.StartQueue(SupervisorCommandMeta{CommandID: "duplicates", ExpectedGeneration: 0}, queueSchedulerTestPlan([]string{"countess", "countess"}, 3))
	var queueErr *QueueValidationError
	if !errors.As(err, &queueErr) || queueErr.Code != QueueReasonDuplicateRun || queueErr.FirstIndex != 0 || queueErr.EntryIndex != 1 || queueErr.RunID != "countess" {
		t.Fatalf("duplicate error = %#v, %v", queueErr, err)
	}
	if runner.calls.Load() != 0 {
		t.Fatalf("worker calls = %d, want 0", runner.calls.Load())
	}
}

func TestFarmQueueRetryStaysOnSameIndex(t *testing.T) {
	runner := &supervisorFakeRunner{started: make(chan SupervisorRunRequest, 3), release: make(chan SupervisorRunResult, 3)}
	supervisor, _ := NewSessionSupervisor(runner)
	plan := queueSchedulerTestPlan([]string{"countess", "mephisto"}, 4)
	plan.Budgets.MaxConsecutiveFailures = 2
	plan.Budgets.MaxTotalRestarts = 2
	if _, err := supervisor.StartQueue(SupervisorCommandMeta{CommandID: "retry", ExpectedGeneration: 0}, plan); err != nil {
		t.Fatal(err)
	}
	first := <-runner.started
	runner.release <- SupervisorRunResult{Disposition: QueueRunRetryCurrent, Reason: "hard_stuck", ExitAuthorization: ExitAuthorizationVerifiedRogueTown}
	second := <-runner.started
	runner.release <- SupervisorRunResult{Disposition: QueueRunAdvance, ExitAuthorization: ExitAuthorizationVerifiedRogueTown}
	third := <-runner.started
	if first.DefinitionID != "countess" || second.DefinitionID != "countess" || second.QueueIndex != 0 || second.Retry != 1 || third.DefinitionID != "mephisto" || third.QueueIndex != 1 {
		t.Fatalf("retry sequence first=%+v second=%+v third=%+v", first, second, third)
	}
	runner.release <- SupervisorRunResult{Disposition: QueueRunStop, Reason: "terminal", ExitAuthorization: ExitAuthorizationNone}
	waitSupervisorState(t, supervisor, SupervisorStateStoppedError)
}

func TestFarmQueueTerminalAndRestartBudgetStopWithoutAdvancing(t *testing.T) {
	tests := []struct {
		name   string
		result SupervisorRunResult
	}{
		{name: "terminal", result: SupervisorRunResult{Disposition: QueueRunStop, Reason: "terminal_context", ExitAuthorization: ExitAuthorizationNone}},
		{name: "restart budget", result: SupervisorRunResult{Disposition: QueueRunRetryCurrent, Reason: "hard_stuck", ExitAuthorization: ExitAuthorizationVerifiedRogueTown}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			runner := &supervisorFakeRunner{started: make(chan SupervisorRunRequest, 2), release: make(chan SupervisorRunResult, 2)}
			supervisor, _ := NewSessionSupervisor(runner)
			plan := queueSchedulerTestPlan([]string{"countess", "mephisto"}, 4)
			plan.Budgets.MaxTotalRestarts = 0
			_, _ = supervisor.StartQueue(SupervisorCommandMeta{CommandID: test.name, ExpectedGeneration: 0}, plan)
			<-runner.started
			runner.release <- test.result
			waitSupervisorState(t, supervisor, SupervisorStateStoppedError)
			if runner.calls.Load() != 1 || supervisor.Snapshot().QueueIndex != 0 {
				t.Fatalf("terminal queue advanced: calls=%d snapshot=%+v", runner.calls.Load(), supervisor.Snapshot())
			}
		})
	}
}

func TestFarmQueueDurationWinsBeforeNextRun(t *testing.T) {
	runner := &supervisorFakeRunner{started: make(chan SupervisorRunRequest, 2), release: make(chan SupervisorRunResult, 2)}
	supervisor, _ := NewSessionSupervisor(runner)
	now := time.Unix(100, 0)
	supervisor.now = func() time.Time { return now }
	plan := queueSchedulerTestPlan([]string{"countess", "mephisto"}, 10)
	plan.Budgets.MaxDuration = time.Minute
	_, _ = supervisor.StartQueue(SupervisorCommandMeta{CommandID: "duration", ExpectedGeneration: 0}, plan)
	<-runner.started
	supervisor.mu.Lock()
	now = now.Add(time.Minute)
	supervisor.mu.Unlock()
	runner.release <- SupervisorRunResult{Disposition: QueueRunAdvance, ExitAuthorization: ExitAuthorizationVerifiedRogueTown}
	waitSupervisorState(t, supervisor, SupervisorStateIdle)
	if runner.calls.Load() != 1 || supervisor.Snapshot().LastResult.Reason != string(QueueReasonDurationBudgetExhausted) {
		t.Fatalf("duration snapshot = %+v calls=%d", supervisor.Snapshot(), runner.calls.Load())
	}
}

func TestFarmQueueRechecksAvailabilityBetweenRuns(t *testing.T) {
	runner := &supervisorFakeRunner{started: make(chan SupervisorRunRequest, 2), release: make(chan SupervisorRunResult, 2)}
	supervisor, _ := NewSessionSupervisor(runner)
	checks := 0
	if err := supervisor.SetQueueGuard(func(_ FarmQueuePlan, index int) error {
		checks++
		if index == 1 {
			return &QueueValidationError{Code: QueueReasonEntryUnavailable, EntryIndex: 1, RunID: "mephisto", Reasons: []tasks.RunReason{tasks.RunReasonRouteStale}}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	_, _ = supervisor.StartQueue(SupervisorCommandMeta{CommandID: "guard", ExpectedGeneration: 0}, queueSchedulerTestPlan([]string{"countess", "mephisto"}, 3))
	<-runner.started
	runner.release <- SupervisorRunResult{Disposition: QueueRunAdvance, ExitAuthorization: ExitAuthorizationVerifiedRogueTown}
	waitSupervisorState(t, supervisor, SupervisorStateStoppedError)
	if runner.calls.Load() != 1 || checks != 2 || supervisor.Snapshot().QueueIndex != 1 {
		t.Fatalf("between-run guard calls=%d checks=%d snapshot=%+v", runner.calls.Load(), checks, supervisor.Snapshot())
	}
}

func TestFarmQueueWorkerF11UsesEmergencyStopSemantics(t *testing.T) {
	runner := &supervisorFakeRunner{started: make(chan SupervisorRunRequest, 1), release: make(chan SupervisorRunResult, 1)}
	supervisor, err := NewSessionSupervisor(runner)
	if err != nil {
		t.Fatal(err)
	}
	plan := queueSchedulerTestPlan([]string{"countess"}, 3)
	if _, err := supervisor.StartQueue(SupervisorCommandMeta{CommandID: "f11", ExpectedGeneration: 0}, plan); err != nil {
		t.Fatal(err)
	}
	<-runner.started
	runner.release <- SupervisorRunResult{Disposition: QueueRunStop, Reason: string(SupervisorReasonEmergencyStopRequested), ExitAuthorization: ExitAuthorizationNone}
	waitSupervisorState(t, supervisor, SupervisorStateIdle)
	snapshot := supervisor.Snapshot()
	if snapshot.State != SupervisorStateIdle || snapshot.LastResult.Reason != string(SupervisorReasonEmergencyStopRequested) || len(snapshot.Queue) != 0 {
		t.Fatalf("F11 snapshot = %+v", snapshot)
	}
}

func TestValidateFarmQueueRejectsInvalidContextRevisionAndEntry(t *testing.T) {
	backend := newQueueValidationConfig(t)
	context := FarmQueueValidationContext{Character: "MrBones", Difficulty: "nightmare", CatalogRevision: 7}
	tests := []struct {
		name    string
		request FarmQueueValidationRequest
		code    string
	}{
		{name: "empty", request: FarmQueueValidationRequest{Character: "MrBones", Difficulty: "nightmare", CatalogRevision: 7}, code: string(QueueReasonEmpty)},
		{name: "duplicate", request: FarmQueueValidationRequest{RunIDs: []string{"countess", "countess"}, Character: "MrBones", Difficulty: "nightmare", CatalogRevision: 7}, code: string(QueueReasonDuplicateRun)},
		{name: "revision", request: FarmQueueValidationRequest{RunIDs: []string{"countess"}, Character: "MrBones", Difficulty: "nightmare", CatalogRevision: 6}, code: string(SupervisorReasonStateChanged)},
		{name: "context", request: FarmQueueValidationRequest{RunIDs: []string{"countess"}, Character: "MrBones", Difficulty: "hell", CatalogRevision: 7}, code: string(QueueReasonContextMismatch)},
		{name: "unknown", request: FarmQueueValidationRequest{RunIDs: []string{"unknown"}, Character: "MrBones", Difficulty: "nightmare", CatalogRevision: 7}, code: string(QueueReasonEntryUnavailable)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := ValidateFarmQueue(backend, test.request, context)
			if queueErrorCode(err) != test.code {
				t.Fatalf("error=%v code=%q want=%q", err, queueErrorCode(err), test.code)
			}
		})
	}
}

func TestValidateFarmQueueUsesClassDefaultInsteadOfNecroFallback(t *testing.T) {
	cfg := newQueueValidationConfig(t)
	context := FarmQueueValidationContext{
		Character: "MrBones", CharacterClass: "necromancer", Difficulty: "nightmare", CatalogRevision: 3,
	}
	_, err := ValidateFarmQueue(cfg, FarmQueueValidationRequest{
		RunIDs: []string{"countess"}, Character: "MrBones", Difficulty: "nightmare", CatalogRevision: 3,
	}, context)
	if err != nil {
		t.Fatalf("class default queue: %v", err)
	}
}

func TestValidateFarmQueueRejectsAvailableDuplicatesWithIndices(t *testing.T) {
	cfg := newQueueValidationConfig(t)
	context := FarmQueueValidationContext{Character: "MrBones", Difficulty: "nightmare", CatalogRevision: 3}
	request := FarmQueueValidationRequest{RunIDs: []string{"countess", "mephisto", "countess"}, Character: "MrBones", Difficulty: "nightmare", CatalogRevision: 3}
	_, err := ValidateFarmQueue(cfg, request, context)
	var queueErr *QueueValidationError
	if !errors.As(err, &queueErr) || queueErr.Code != QueueReasonDuplicateRun || queueErr.FirstIndex != 0 || queueErr.EntryIndex != 2 || queueErr.RunID != "countess" {
		t.Fatalf("duplicate error = %#v, %v", queueErr, err)
	}
}

func queueSchedulerTestPlan(runIDs []string, maxRuns int) FarmQueuePlan {
	return FarmQueuePlan{
		RunIDs: append([]string(nil), runIDs...), Character: "MrBones", Difficulty: "nightmare", CatalogRevision: 1,
		Budgets: FarmQueueBudgets{MaxRuns: maxRuns, MaxDuration: time.Hour, MaxConsecutiveFailures: 2, MaxTotalRestarts: 3},
	}
}

func newQueueValidationConfig(t *testing.T) *config.Config {
	t.Helper()
	cfg, err := config.Load("../../configs/config.example.yaml")
	if err != nil {
		t.Fatal(err)
	}
	cfg.Session.Character = "MrBones"
	cfg.Session.Difficulty = "nightmare"
	writeTestRouteAssignments(t, cfg, map[tasks.RunID]string{tasks.RunIDCountess: "black-marsh-cellar5-nightmare-mrbones", tasks.RunIDMephisto: "durance-2-mephisto-nightmare-mrbones"})
	cfg.Routes.LifecycleFile = filepath.Join(t.TempDir(), "route-lifecycle.local.yaml")
	// Bootstrap the isolated manifest without changing production lifecycle metadata.
	store, err := NewRouteLifecycleStore(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.Snapshot(); err != nil {
		t.Fatal(err)
	}
	return cfg
}

func queueErrorCode(err error) string {
	var queueErr *QueueValidationError
	if errors.As(err, &queueErr) {
		return string(queueErr.Code)
	}
	var supervisorErr *SupervisorCommandError
	if errors.As(err, &supervisorErr) {
		return string(supervisorErr.Code)
	}
	return ""
}
