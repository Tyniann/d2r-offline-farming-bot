package app

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
)

type fakeSessionRun struct {
	result sessionRunResult
	calls  *[]string
}

func (r *fakeSessionRun) Execute(context.Context) sessionRunResult {
	*r.calls = append(*r.calls, "run.execute")
	return r.result
}

func (r *fakeSessionRun) Reset(reason string) {
	*r.calls = append(*r.calls, "run.reset:"+reason)
}

type fakeSessionDriver struct {
	calls       []string
	runResults  []sessionRunResult
	startErr    error
	verifyErr   error
	exitErr     error
	emitErrAt   string
	readyCancel bool
	readyGate   <-chan struct{}
	startCalled chan<- struct{}
	runsCreated int
}

func (d *fakeSessionDriver) AwaitReady(ctx context.Context) error {
	d.calls = append(d.calls, "ready")
	if d.readyCancel {
		return context.Canceled
	}
	if d.readyGate != nil {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-d.readyGate:
		}
	}
	return ctx.Err()
}
func (d *fakeSessionDriver) ResetForNextGame(reason string) error {
	d.calls = append(d.calls, "cycle_reset:"+reason)
	return nil
}
func (d *fakeSessionDriver) StartGame(context.Context) error {
	d.calls = append(d.calls, "start")
	if d.startCalled != nil {
		d.startCalled <- struct{}{}
	}
	return d.startErr
}
func (d *fakeSessionDriver) VerifyGame(context.Context) error {
	d.calls = append(d.calls, "verify")
	return d.verifyErr
}
func (d *fakeSessionDriver) NewRun() (sessionRunExecutor, error) {
	d.calls = append(d.calls, "new_run")
	index := d.runsCreated
	d.runsCreated++
	result := sessionRunResult{Outcome: sessionRunSuccess}
	if index < len(d.runResults) {
		result = d.runResults[index]
	}
	return &fakeSessionRun{result: result, calls: &d.calls}, nil
}
func (d *fakeSessionDriver) ExitGame(_ context.Context, result sessionRunResult) error {
	d.calls = append(d.calls, "exit:"+string(result.Outcome))
	return d.exitErr
}
func (d *fakeSessionDriver) EmitLifecycle(event, _ string) error {
	d.calls = append(d.calls, "emit:"+event)
	if event == d.emitErrAt {
		return errors.New("telemetry unavailable")
	}
	return nil
}

func TestSessionCycleThreeFreshSuccessfulRuns(t *testing.T) {
	driver := &fakeSessionDriver{}
	orchestrator := newSessionCycleOrchestrator(driver)
	for cycle := 0; cycle < 3; cycle++ {
		result, err := orchestrator.execute(context.Background(), cycle == 0)
		if err != nil {
			t.Fatal(err)
		}
		if result.Outcome != sessionCycleSuccess {
			t.Fatalf("cycle %d outcome = %s", cycle, result.Outcome)
		}
	}
	if driver.runsCreated != 3 {
		t.Fatalf("runs created = %d, want 3", driver.runsCreated)
	}
}

func TestSessionCycleFailureResetsBeforeExit(t *testing.T) {
	driver := &fakeSessionDriver{runResults: []sessionRunResult{{Outcome: sessionRunFailed, Reason: "hard_stuck", Step: "travel"}}}
	result, err := newSessionCycleOrchestrator(driver).execute(context.Background(), true)
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != sessionCycleFailed || result.Reason != "hard_stuck" {
		t.Fatalf("result = %+v", result)
	}
	resetIndex, exitIndex := indexOf(driver.calls, "run.reset:cycle_evaluate"), indexOf(driver.calls, "exit:failed")
	if resetIndex < 0 || exitIndex < 0 || resetIndex >= exitIndex {
		t.Fatalf("reset must precede exit: %v", driver.calls)
	}
}

func TestSessionCycleStopAtPauseGateProducesNoInput(t *testing.T) {
	driver := &fakeSessionDriver{readyCancel: true}
	result, err := newSessionCycleOrchestrator(driver).execute(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != sessionCycleStopped {
		t.Fatalf("result = %+v", result)
	}
	for _, forbidden := range []string{"start", "verify", "new_run", "exit:success"} {
		if indexOf(driver.calls, forbidden) >= 0 {
			t.Fatalf("unexpected call %q: %v", forbidden, driver.calls)
		}
	}
}

func TestSessionCyclePauseGateBlocksNextActionUntilResume(t *testing.T) {
	resume := make(chan struct{})
	started := make(chan struct{}, 1)
	driver := &fakeSessionDriver{readyGate: resume, startCalled: started}
	done := make(chan error, 1)
	go func() {
		_, err := newSessionCycleOrchestrator(driver).execute(context.Background(), false)
		done <- err
	}()
	select {
	case <-started:
		t.Fatal("start occurred while paused")
	case <-time.After(30 * time.Millisecond):
	}
	close(resume)
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("start missing after resume")
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("cycle did not resume")
	}
}

func TestSessionCycleTelemetryFailureBlocksFollowingAction(t *testing.T) {
	driver := &fakeSessionDriver{emitErrAt: "game_start_requested"}
	result, err := newSessionCycleOrchestrator(driver).execute(context.Background(), false)
	if err == nil || result.Reason != "start_game_blocked" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if indexOf(driver.calls, "start") >= 0 {
		t.Fatalf("start occurred after telemetry failure: %v", driver.calls)
	}
}

func TestSessionCycleActiveGameSkipsStartAndPreservesOrder(t *testing.T) {
	driver := &fakeSessionDriver{}
	_, err := newSessionCycleOrchestrator(driver).execute(context.Background(), true)
	if err != nil {
		t.Fatal(err)
	}
	wantSubsequence := []string{"emit:cycle_reset_requested", "cycle_reset:cycle_start", "emit:game_verification_requested", "verify", "new_run", "emit:run_started", "run.execute", "run.reset:cycle_evaluate", "emit:game_exit_requested", "exit:success"}
	if !isSubsequence(driver.calls, wantSubsequence) {
		t.Fatalf("calls = %v, missing ordered %v", driver.calls, wantSubsequence)
	}
}

func TestSessionCycleLoadingTimeoutFailsWithoutRunOrExit(t *testing.T) {
	driver := &fakeSessionDriver{verifyErr: context.DeadlineExceeded}
	result, err := newSessionCycleOrchestrator(driver).execute(context.Background(), true)
	if err == nil || result.Outcome != sessionCycleFailed || result.Reason != "verify_game_failed" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	for _, forbidden := range []string{"new_run", "exit:success", "exit:failed"} {
		if indexOf(driver.calls, forbidden) >= 0 {
			t.Fatalf("unexpected call %q after loading timeout: %v", forbidden, driver.calls)
		}
	}
}

func indexOf(values []string, target string) int {
	for i, value := range values {
		if value == target {
			return i
		}
	}
	return -1
}

func isSubsequence(values, want []string) bool {
	position := 0
	for _, value := range values {
		if position < len(want) && value == want[position] {
			position++
		}
	}
	return position == len(want) || reflect.DeepEqual(want, []string{})
}
