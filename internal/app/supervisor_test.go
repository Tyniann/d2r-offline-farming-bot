package app

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type supervisorFakeRunner struct {
	started chan SupervisorRunRequest
	release chan SupervisorRunResult
	calls   atomic.Int32
	panic   bool
}

func (r *supervisorFakeRunner) Run(ctx context.Context, request SupervisorRunRequest) SupervisorRunResult {
	r.calls.Add(1)
	if r.panic {
		panic("injected worker panic")
	}
	select {
	case r.started <- request:
	case <-ctx.Done():
		return SupervisorRunResult{Disposition: QueueRunStop, Reason: "cancelled"}
	}
	select {
	case result := <-r.release:
		return result
	case <-ctx.Done():
		return SupervisorRunResult{Disposition: QueueRunStop, Reason: "cancelled"}
	}
}

func TestSessionSupervisorStartPauseResumeAndStopAfterRun(t *testing.T) {
	runner := &supervisorFakeRunner{started: make(chan SupervisorRunRequest, 2), release: make(chan SupervisorRunResult, 2)}
	supervisor, err := NewSessionSupervisor(runner)
	if err != nil {
		t.Fatal(err)
	}
	start, err := supervisor.StartQueue(SupervisorCommandMeta{CommandID: "start-1", ExpectedGeneration: 0}, queueSchedulerTestPlan([]string{"countess"}, 3))
	if err != nil {
		t.Fatal(err)
	}
	if start.State != SupervisorStateStartingRun || start.Generation != 1 {
		t.Fatalf("start snapshot = %+v", start)
	}
	<-runner.started
	running := supervisor.Snapshot()
	pausedIntent, err := supervisor.PauseAfterRun(SupervisorCommandMeta{CommandID: "pause-1", ExpectedGeneration: running.Generation})
	if err != nil {
		t.Fatal(err)
	}
	if pausedIntent.PendingIntent != SupervisorIntentPauseAfterRun {
		t.Fatalf("pause intent snapshot = %+v", pausedIntent)
	}
	runner.release <- SupervisorRunResult{Disposition: QueueRunAdvance}
	waitSupervisorState(t, supervisor, SupervisorStatePausedBetweenRuns)

	paused := supervisor.Snapshot()
	if _, err := supervisor.Resume(SupervisorCommandMeta{CommandID: "resume-1", ExpectedGeneration: paused.Generation}); err != nil {
		t.Fatal(err)
	}
	<-runner.started
	running = supervisor.Snapshot()
	if _, err := supervisor.StopAfterRun(SupervisorCommandMeta{CommandID: "stop-after-1", ExpectedGeneration: running.Generation}); err != nil {
		t.Fatal(err)
	}
	runner.release <- SupervisorRunResult{Disposition: QueueRunAdvance}
	waitSupervisorState(t, supervisor, SupervisorStateIdle)
	if runner.calls.Load() != 2 {
		t.Fatalf("runner calls = %d, want 2 fresh generations", runner.calls.Load())
	}
}

func TestSessionSupervisorEmergencyStopUsesF11Reason(t *testing.T) {
	runner := &supervisorFakeRunner{started: make(chan SupervisorRunRequest, 1), release: make(chan SupervisorRunResult)}
	supervisor, _ := NewSessionSupervisor(runner)
	_, err := supervisor.StartQueue(SupervisorCommandMeta{CommandID: "start", ExpectedGeneration: 0}, queueSchedulerTestPlan([]string{"mephisto"}, 1))
	if err != nil {
		t.Fatal(err)
	}
	<-runner.started
	running := supervisor.Snapshot()
	cancelling, err := supervisor.EmergencyStop(SupervisorCommandMeta{CommandID: "f11", ExpectedGeneration: running.Generation})
	if err != nil {
		t.Fatal(err)
	}
	if cancelling.State != SupervisorStateCancelling {
		t.Fatalf("emergency snapshot = %+v", cancelling)
	}
	waitSupervisorState(t, supervisor, SupervisorStateIdle)
	if got := supervisor.Snapshot().LastResult.Reason; got != string(SupervisorReasonEmergencyStopRequested) {
		t.Fatalf("emergency reason = %q", got)
	}
}

func TestSessionSupervisorRejectsConcurrentStartAndStaleGeneration(t *testing.T) {
	runner := &supervisorFakeRunner{started: make(chan SupervisorRunRequest, 1), release: make(chan SupervisorRunResult)}
	supervisor, _ := NewSessionSupervisor(runner)
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for _, id := range []string{"a", "b"} {
		wg.Add(1)
		go func(commandID string) {
			defer wg.Done()
			_, err := supervisor.StartQueue(SupervisorCommandMeta{CommandID: commandID, ExpectedGeneration: 0}, queueSchedulerTestPlan([]string{"countess"}, 1))
			errs <- err
		}(id)
	}
	wg.Wait()
	close(errs)
	successes, conflicts := 0, 0
	for err := range errs {
		if err == nil {
			successes++
			continue
		}
		var commandErr *SupervisorCommandError
		if errors.As(err, &commandErr) && (commandErr.Code == SupervisorReasonCommandConflict || commandErr.Code == SupervisorReasonStateChanged) {
			conflicts++
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("concurrent starts successes=%d conflicts=%d", successes, conflicts)
	}
	<-runner.started
	current := supervisor.Snapshot()
	_, err := supervisor.PauseAfterRun(SupervisorCommandMeta{CommandID: "stale", ExpectedGeneration: current.Generation - 1})
	var commandErr *SupervisorCommandError
	if !errors.As(err, &commandErr) || commandErr.Code != SupervisorReasonStateChanged {
		t.Fatalf("stale generation err = %v", err)
	}
	_, _ = supervisor.EmergencyStop(SupervisorCommandMeta{CommandID: "cleanup", ExpectedGeneration: current.Generation})
}

func TestSessionSupervisorCommandReplayIsIdempotent(t *testing.T) {
	runner := &supervisorFakeRunner{started: make(chan SupervisorRunRequest, 1), release: make(chan SupervisorRunResult)}
	supervisor, _ := NewSessionSupervisor(runner)
	meta := SupervisorCommandMeta{CommandID: "same", ExpectedGeneration: 0}
	plan := queueSchedulerTestPlan([]string{"countess"}, 1)
	first, err := supervisor.StartQueue(meta, plan)
	if err != nil {
		t.Fatal(err)
	}
	second, err := supervisor.StartQueue(meta, plan)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) || runner.calls.Load() > 1 {
		t.Fatalf("idempotent replay first=%+v second=%+v calls=%d", first, second, runner.calls.Load())
	}
	if _, err := supervisor.StartQueue(meta, queueSchedulerTestPlan([]string{"mephisto"}, 1)); err == nil {
		t.Fatal("expected reused command ID with different request to fail")
	}
	<-runner.started
	current := supervisor.Snapshot()
	_, _ = supervisor.EmergencyStop(SupervisorCommandMeta{CommandID: "cleanup", ExpectedGeneration: current.Generation})
}

func TestSessionSupervisorRecoversWorkerPanic(t *testing.T) {
	runner := &supervisorFakeRunner{panic: true}
	supervisor, _ := NewSessionSupervisor(runner)
	_, err := supervisor.StartQueue(SupervisorCommandMeta{CommandID: "panic", ExpectedGeneration: 0}, queueSchedulerTestPlan([]string{"countess"}, 1))
	if err != nil {
		t.Fatal(err)
	}
	waitSupervisorState(t, supervisor, SupervisorStateStoppedError)
	if got := supervisor.Snapshot().LastResult.Reason; got != string(SupervisorReasonWorkerPanic) {
		t.Fatalf("panic reason = %q", got)
	}
}

func TestSessionSupervisorShutdownCancelsWorker(t *testing.T) {
	runner := &supervisorFakeRunner{started: make(chan SupervisorRunRequest, 1), release: make(chan SupervisorRunResult)}
	supervisor, _ := NewSessionSupervisor(runner)
	_, _ = supervisor.StartQueue(SupervisorCommandMeta{CommandID: "start", ExpectedGeneration: 0}, queueSchedulerTestPlan([]string{"countess"}, 1))
	<-runner.started
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := supervisor.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
	if supervisor.Snapshot().State != SupervisorStateIdle {
		t.Fatalf("shutdown snapshot = %+v", supervisor.Snapshot())
	}
}

func waitSupervisorState(t *testing.T, supervisor *SessionSupervisor, want SupervisorState) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if supervisor.Snapshot().State == want {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("supervisor state = %q, want %q", supervisor.Snapshot().State, want)
}
