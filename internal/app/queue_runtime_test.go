package app

import (
	"context"
	"reflect"
	"testing"
)

func TestRuntimeQueueRunnerConsumesOnlyFirstConfirmedGame(t *testing.T) {
	var initial []bool
	runner := &RuntimeQueueRunner{initialInGame: true}
	runner.execute = func(_ context.Context, _ SupervisorRunRequest, active bool) (SupervisorRunResult, bool) {
		initial = append(initial, active)
		return SupervisorRunResult{Disposition: QueueRunAdvance}, true
	}
	for _, runID := range []string{"countess", "mephisto", "countess"} {
		if result := runner.Run(context.Background(), SupervisorRunRequest{RunID: runID}); result.Disposition != QueueRunAdvance {
			t.Fatalf("result = %+v", result)
		}
	}
	if want := []bool{true, false, false}; !reflect.DeepEqual(initial, want) {
		t.Fatalf("initial-in-game sequence = %v, want %v", initial, want)
	}
}

func TestRuntimeQueueRunnerPreservesConfirmedGameBeforeExit(t *testing.T) {
	var initial []bool
	runner := &RuntimeQueueRunner{initialInGame: true}
	runner.execute = func(_ context.Context, _ SupervisorRunRequest, active bool) (SupervisorRunResult, bool) {
		initial = append(initial, active)
		if len(initial) == 1 {
			return SupervisorRunResult{Disposition: QueueRunStop, Reason: "preflight_failed"}, false
		}
		return SupervisorRunResult{Disposition: QueueRunAdvance}, true
	}
	runner.Run(context.Background(), SupervisorRunRequest{RunID: "countess"})
	runner.Run(context.Background(), SupervisorRunRequest{RunID: "countess"})
	if want := []bool{true, true}; !reflect.DeepEqual(initial, want) {
		t.Fatalf("initial-in-game sequence = %v, want %v", initial, want)
	}
}

func TestRuntimeQueueRunnerResetsInitialGameForEveryQueue(t *testing.T) {
	var initial []bool
	runner := &RuntimeQueueRunner{}
	runner.execute = func(_ context.Context, _ SupervisorRunRequest, active bool) (SupervisorRunResult, bool) {
		initial = append(initial, active)
		return SupervisorRunResult{Disposition: QueueRunAdvance}, true
	}
	runner.BeginQueue(false)
	runner.Run(context.Background(), SupervisorRunRequest{RunID: "countess"})
	runner.BeginQueue(true)
	runner.Run(context.Background(), SupervisorRunRequest{RunID: "mephisto"})
	if want := []bool{false, true}; !reflect.DeepEqual(initial, want) {
		t.Fatalf("queue reset sequence = %v, want %v", initial, want)
	}
}

func TestRuntimeQueueRunnerRoutesPauseAfterRun(t *testing.T) {
	runner := &RuntimeQueueRunner{}
	calls := 0
	runner.SetPauseAfterRunHandler(func() error {
		calls++
		return nil
	})
	if err := runner.requestPauseAfterRun(); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("pause-after-run calls = %d, want 1", calls)
	}
}
