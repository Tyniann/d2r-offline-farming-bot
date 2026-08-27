package app

import (
	"context"
	"testing"
	"time"
)

func TestRunConfiguredQueueUsesLifecycleAndSingleExitBoundary(t *testing.T) {
	runner := newFakeLifecycleRunner(2)
	plan := queueSchedulerTestPlan([]string{"countess"}, 1)
	done := make(chan error, 1)
	go func() {
		done <- runConfiguredQueue(context.Background(), plan, runner)
	}()
	request := <-runner.started
	runner.release <- SupervisorRunResult{Disposition: QueueRunAdvance, ExitAuthorization: ExitAuthorizationVerifiedRogueTown}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("configured queue did not finish")
	}
	events := runner.Events()
	if countLifecycleEvent(events, "start_game") != 1 || countLifecycleEvent(events, "run") != 1 || countLifecycleEvent(events, "exit_game") != 1 {
		t.Fatalf("lifecycle events = %+v", events)
	}
	if request.DefinitionID != "countess" || events[len(events)-2].Reason != string(QueueReasonRunBudgetExhausted) {
		t.Fatalf("request=%+v events=%+v", request, events)
	}
}

func countLifecycleEvent(events []lifecycleRunnerEvent, name string) int {
	count := 0
	for _, event := range events {
		if event.Name == name {
			count++
		}
	}
	return count
}
