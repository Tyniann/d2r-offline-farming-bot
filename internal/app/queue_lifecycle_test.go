package app

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"
)

type lifecycleRunnerEvent struct {
	Name    string
	Request SupervisorRunRequest
	Reason  string
}

type fakeLifecycleRunner struct {
	mu        sync.Mutex
	events    []lifecycleRunnerEvent
	started   chan SupervisorRunRequest
	release   chan SupervisorRunResult
	startErr  error
	verifyErr error
	exitErr   error
}

func newFakeLifecycleRunner(buffer int) *fakeLifecycleRunner {
	return &fakeLifecycleRunner{started: make(chan SupervisorRunRequest, buffer), release: make(chan SupervisorRunResult, buffer)}
}

func (r *fakeLifecycleRunner) record(name string, request SupervisorRunRequest, reason string) {
	r.mu.Lock()
	r.events = append(r.events, lifecycleRunnerEvent{Name: name, Request: request, Reason: reason})
	r.mu.Unlock()
}

func (r *fakeLifecycleRunner) StartGame(_ context.Context, request SupervisorRunRequest) error {
	r.record("start_game", request, "")
	return r.startErr
}

func (r *fakeLifecycleRunner) RevalidateGame(_ context.Context, request SupervisorRunRequest) error {
	r.record("revalidate_game", request, "")
	return r.verifyErr
}

func (r *fakeLifecycleRunner) Run(ctx context.Context, request SupervisorRunRequest) SupervisorRunResult {
	r.record("run", request, "")
	r.started <- request
	select {
	case <-ctx.Done():
		return emergencyStopResult()
	case result := <-r.release:
		return result
	}
}

func (r *fakeLifecycleRunner) ExitGame(_ context.Context, request SupervisorRunRequest, reason string) error {
	r.record("exit_game", request, reason)
	return r.exitErr
}

func (r *fakeLifecycleRunner) CloseQueue() {
	r.record("close_queue", SupervisorRunRequest{}, "")
}

func (r *fakeLifecycleRunner) Events() []lifecycleRunnerEvent {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]lifecycleRunnerEvent(nil), r.events...)
}

func TestSameGameQueueStartsOnceRunsAllEntriesAndExitsAtWrap(t *testing.T) {
	runner := newFakeLifecycleRunner(4)
	supervisor, _ := NewSessionSupervisor(runner)
	plan := queueSchedulerTestPlan([]string{"countess", "mephisto"}, 3)
	if _, err := supervisor.StartQueue(SupervisorCommandMeta{CommandID: "same-game", ExpectedGeneration: 0}, plan); err != nil {
		t.Fatal(err)
	}
	first := <-runner.started
	runner.release <- SupervisorRunResult{Disposition: QueueRunAdvance, SafeToExit: true}
	second := <-runner.started
	runner.release <- SupervisorRunResult{Disposition: QueueRunAdvance, SafeToExit: true}
	third := <-runner.started
	runner.release <- SupervisorRunResult{Disposition: QueueRunAdvance, SafeToExit: true}
	waitSupervisorState(t, supervisor, SupervisorStateIdle)

	if first.GameID == "" || first.GameID != second.GameID || third.GameID == "" || third.GameID == first.GameID {
		t.Fatalf("game IDs first=%q second=%q third=%q", first.GameID, second.GameID, third.GameID)
	}
	if first.Cycle != 0 || second.Cycle != 0 || third.Cycle != 1 || first.QueueIndex != 0 || second.QueueIndex != 1 || third.QueueIndex != 0 {
		t.Fatalf("requests first=%+v second=%+v third=%+v", first, second, third)
	}
	events := runner.Events()
	names := make([]string, len(events))
	for i, event := range events {
		names[i] = event.Name
	}
	want := []string{"start_game", "run", "run", "exit_game", "start_game", "run", "exit_game", "close_queue"}
	if !reflect.DeepEqual(names, want) {
		t.Fatalf("events = %v, want %v (%+v)", names, want, events)
	}
	if events[3].Reason != "queue_wrap" || events[6].Reason != string(QueueReasonRunBudgetExhausted) {
		t.Fatalf("exit reasons = %q, %q", events[3].Reason, events[6].Reason)
	}
}

func TestSameGameQueueRetryExitsAndRestartsSameIndex(t *testing.T) {
	runner := newFakeLifecycleRunner(3)
	supervisor, _ := NewSessionSupervisor(runner)
	plan := queueSchedulerTestPlan([]string{"countess", "mephisto"}, 3)
	if _, err := supervisor.StartQueue(SupervisorCommandMeta{CommandID: "retry", ExpectedGeneration: 0}, plan); err != nil {
		t.Fatal(err)
	}
	first := <-runner.started
	runner.release <- SupervisorRunResult{Disposition: QueueRunRetryCurrent, Reason: "hard_stuck", SafeToExit: true}
	second := <-runner.started
	if second.QueueIndex != first.QueueIndex || second.Retry != 1 || second.GameID == first.GameID {
		t.Fatalf("retry first=%+v second=%+v", first, second)
	}
	runner.release <- SupervisorRunResult{Disposition: QueueRunStop, Reason: "terminal"}
	waitSupervisorState(t, supervisor, SupervisorStateStoppedError)
	events := runner.Events()
	if len(events) < 5 || events[2].Name != "exit_game" || events[2].Reason != "retry_current" || events[3].Name != "start_game" {
		t.Fatalf("retry events = %+v", events)
	}
}

func TestSameGameQueueExecutesRequiredTerminalExitWithoutTownHandoff(t *testing.T) {
	runner := newFakeLifecycleRunner(1)
	supervisor, _ := NewSessionSupervisor(runner)
	if _, err := supervisor.StartQueue(SupervisorCommandMeta{CommandID: "required-exit", ExpectedGeneration: 0}, queueSchedulerTestPlan([]string{"cows"}, 1)); err != nil {
		t.Fatal(err)
	}
	<-runner.started
	runner.release <- SupervisorRunResult{Disposition: QueueRunStop, Reason: "cow_return_portal_failed", ExitRequired: true}
	waitSupervisorState(t, supervisor, SupervisorStateStoppedError)

	events := runner.Events()
	if len(events) < 4 || events[2].Name != "exit_game" || events[2].Reason != "cow_return_portal_failed" {
		t.Fatalf("required exit events=%+v", events)
	}
}

func TestSameGameQueueStartAndExitFailuresAreStable(t *testing.T) {
	t.Run("start", func(t *testing.T) {
		runner := newFakeLifecycleRunner(1)
		runner.startErr = fmt.Errorf("no character screen")
		supervisor, _ := NewSessionSupervisor(runner)
		_, _ = supervisor.StartQueue(SupervisorCommandMeta{CommandID: "start-fail", ExpectedGeneration: 0}, queueSchedulerTestPlan([]string{"countess"}, 1))
		waitSupervisorState(t, supervisor, SupervisorStateStoppedError)
		if got := supervisor.Snapshot().LastResult.Reason; got != string(SupervisorReasonGameStartFailed) {
			t.Fatalf("start reason = %q", got)
		}
	})
	t.Run("exit", func(t *testing.T) {
		runner := newFakeLifecycleRunner(1)
		runner.exitErr = fmt.Errorf("dialog missing")
		supervisor, _ := NewSessionSupervisor(runner)
		_, _ = supervisor.StartQueue(SupervisorCommandMeta{CommandID: "exit-fail", ExpectedGeneration: 0}, queueSchedulerTestPlan([]string{"countess"}, 1))
		<-runner.started
		runner.release <- SupervisorRunResult{Disposition: QueueRunAdvance, SafeToExit: true}
		waitSupervisorState(t, supervisor, SupervisorStateStoppedError)
		if got := supervisor.Snapshot().LastResult.Reason; got != string(SupervisorReasonGameExitFailed) {
			t.Fatalf("exit reason = %q", got)
		}
	})
}

func TestSameGamePauseResumeAndStopUseOneOpenGame(t *testing.T) {
	runner := newFakeLifecycleRunner(3)
	supervisor, _ := NewSessionSupervisor(runner)
	plan := queueSchedulerTestPlan([]string{"countess", "mephisto"}, 4)
	_, _ = supervisor.StartQueue(SupervisorCommandMeta{CommandID: "controls", ExpectedGeneration: 0}, plan)
	first := <-runner.started
	running := supervisor.Snapshot()
	if _, err := supervisor.PauseAfterRun(SupervisorCommandMeta{CommandID: "pause", ExpectedGeneration: running.Generation}); err != nil {
		t.Fatal(err)
	}
	runner.release <- SupervisorRunResult{Disposition: QueueRunAdvance, SafeToExit: true}
	waitSupervisorState(t, supervisor, SupervisorStatePausedBetweenRuns)
	paused := supervisor.Snapshot()
	if paused.QueueIndex != 1 || paused.GameID != first.GameID {
		t.Fatalf("paused snapshot = %+v first=%+v", paused, first)
	}
	for _, event := range runner.Events() {
		if event.Name == "exit_game" {
			t.Fatalf("pause exited the game: %+v", runner.Events())
		}
	}
	if _, err := supervisor.Resume(SupervisorCommandMeta{CommandID: "resume", ExpectedGeneration: paused.Generation}); err != nil {
		t.Fatal(err)
	}
	second := <-runner.started
	if second.DefinitionID != "mephisto" || second.GameID != first.GameID {
		t.Fatalf("same-game resume first=%+v second=%+v", first, second)
	}
	running = supervisor.Snapshot()
	if _, err := supervisor.StopAfterRun(SupervisorCommandMeta{CommandID: "stop", ExpectedGeneration: running.Generation}); err != nil {
		t.Fatal(err)
	}
	runner.release <- SupervisorRunResult{Disposition: QueueRunAdvance, SafeToExit: true}
	waitSupervisorState(t, supervisor, SupervisorStateIdle)
	exits := 0
	for _, event := range runner.Events() {
		if event.Name == "exit_game" {
			exits++
			if event.Reason != "stop_after_run" {
				t.Fatalf("stop exit reason = %q", event.Reason)
			}
		}
	}
	if exits != 1 {
		t.Fatalf("exit count = %d events=%+v", exits, runner.Events())
	}
}

func TestSameGameResumeFailsClosedWhenPausedGameIsLost(t *testing.T) {
	runner := newFakeLifecycleRunner(2)
	supervisor, _ := NewSessionSupervisor(runner)
	_, _ = supervisor.StartQueue(SupervisorCommandMeta{CommandID: "lost", ExpectedGeneration: 0}, queueSchedulerTestPlan([]string{"countess", "mephisto"}, 4))
	<-runner.started
	running := supervisor.Snapshot()
	_, _ = supervisor.PauseAfterRun(SupervisorCommandMeta{CommandID: "pause-lost", ExpectedGeneration: running.Generation})
	runner.release <- SupervisorRunResult{Disposition: QueueRunAdvance, SafeToExit: true}
	waitSupervisorState(t, supervisor, SupervisorStatePausedBetweenRuns)
	runner.verifyErr = fmt.Errorf("game identity changed")
	paused := supervisor.Snapshot()
	if _, err := supervisor.Resume(SupervisorCommandMeta{CommandID: "resume-lost", ExpectedGeneration: paused.Generation}); err != nil {
		t.Fatal(err)
	}
	waitSupervisorState(t, supervisor, SupervisorStateStoppedError)
	if got := supervisor.Snapshot().LastResult.Reason; got != string(SupervisorReasonPausedGameLost) {
		t.Fatalf("resume reason = %q", got)
	}
	select {
	case request := <-runner.started:
		t.Fatalf("lost game started input: %+v", request)
	default:
	}
}

func TestStopIntentCannotBeDowngradedByPause(t *testing.T) {
	runner := newFakeLifecycleRunner(1)
	supervisor, _ := NewSessionSupervisor(runner)
	_, _ = supervisor.StartQueue(SupervisorCommandMeta{CommandID: "priority", ExpectedGeneration: 0}, queueSchedulerTestPlan([]string{"countess"}, 2))
	<-runner.started
	snapshot := supervisor.Snapshot()
	stopped, err := supervisor.StopAfterRun(SupervisorCommandMeta{CommandID: "stop-priority", ExpectedGeneration: snapshot.Generation})
	if err != nil {
		t.Fatal(err)
	}
	paused, err := supervisor.PauseAfterRun(SupervisorCommandMeta{CommandID: "pause-priority", ExpectedGeneration: stopped.Generation})
	if err != nil {
		t.Fatal(err)
	}
	if paused.PendingIntent != SupervisorIntentStopAfterRun {
		t.Fatalf("pending intent = %q", paused.PendingIntent)
	}
	runner.release <- SupervisorRunResult{Disposition: QueueRunAdvance, SafeToExit: true}
	waitSupervisorState(t, supervisor, SupervisorStateIdle)
}

func TestSameGameDurationBudgetExitsBeforeNextRun(t *testing.T) {
	runner := newFakeLifecycleRunner(2)
	supervisor, _ := NewSessionSupervisor(runner)
	now := time.Unix(100, 0)
	supervisor.now = func() time.Time { return now }
	plan := queueSchedulerTestPlan([]string{"countess", "mephisto"}, 10)
	plan.Budgets.MaxDuration = time.Minute
	_, _ = supervisor.StartQueue(SupervisorCommandMeta{CommandID: "duration-game", ExpectedGeneration: 0}, plan)
	<-runner.started
	supervisor.mu.Lock()
	now = now.Add(time.Minute)
	supervisor.mu.Unlock()
	runner.release <- SupervisorRunResult{Disposition: QueueRunAdvance, SafeToExit: true}
	waitSupervisorState(t, supervisor, SupervisorStateIdle)
	if got := supervisor.Snapshot().LastResult.Reason; got != string(QueueReasonDurationBudgetExhausted) {
		t.Fatalf("duration reason = %q", got)
	}
	events := runner.Events()
	if len(events) < 4 || events[2].Name != "exit_game" || events[2].Reason != string(QueueReasonDurationBudgetExhausted) {
		t.Fatalf("duration events = %+v", events)
	}
}
