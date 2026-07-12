package app

import (
	"context"
	"testing"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/telemetry"
)

type fakeMultiCycles struct {
	results []sessionCycleExecution
	calls   int
	starts  []bool
}

func (f *fakeMultiCycles) Execute(_ context.Context, active bool) (sessionCycleExecution, error) {
	f.starts = append(f.starts, active)
	result := f.results[f.calls]
	f.calls++
	return result, nil
}

func newTestMultiRunner(cycles *fakeMultiCycles, emitter *fakeLifecycleEmitter) *sessionMultiRunner {
	policy := newSessionRecoveryPolicy([]string{"hard_stuck"}, 2, 2)
	return &sessionMultiRunner{
		config: sessionMultiConfig{Run: "countess", MaxRuns: len(cycles.results), MaxDuration: time.Hour, IDPrefix: "session-test", InitialInGame: true},
		cycles: cycles, recovery: &sessionRecoveryCoordinator{policy: policy, emitter: emitter},
		wait: func(context.Context, time.Duration) error { return nil },
	}
}

func TestSessionMultiRunnerCompletesThreeCyclesWithFreshIDs(t *testing.T) {
	cycles := &fakeMultiCycles{results: []sessionCycleExecution{
		{Result: sessionCycleResult{Outcome: sessionCycleSuccess, Run: sessionRunResult{Outcome: sessionRunSuccess}}},
		{Result: sessionCycleResult{Outcome: sessionCycleSuccess, Run: sessionRunResult{Outcome: sessionRunSuccess}}},
		{Result: sessionCycleResult{Outcome: sessionCycleSuccess, Run: sessionRunResult{Outcome: sessionRunSuccess}}},
	}}
	emitter := &fakeLifecycleEmitter{}
	result, err := newTestMultiRunner(cycles, emitter).run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != "completed" || result.Reason != "max_runs" || cycles.calls != 3 {
		t.Fatalf("result=%+v calls=%d", result, cycles.calls)
	}
	if len(cycles.starts) != 3 || !cycles.starts[0] || cycles.starts[1] || cycles.starts[2] {
		t.Fatalf("initial game flags = %v", cycles.starts)
	}
	runIDs := map[string]struct{}{}
	for _, event := range emitter.events {
		if event.Event == telemetry.RunCompleted {
			runIDs[event.RunID] = struct{}{}
		}
	}
	if len(runIDs) != 3 {
		t.Fatalf("run IDs = %v", runIDs)
	}
}

func TestSessionMultiRunnerRestartsOneHardStuckWithinBudget(t *testing.T) {
	cycles := &fakeMultiCycles{results: []sessionCycleExecution{
		{Result: sessionCycleResult{Outcome: sessionCycleFailed, Run: sessionRunResult{Outcome: sessionRunAborted, Reason: "hard_stuck"}}, Stuck: sessionStuckContext{RouteID: "route", SegmentID: "segment"}},
		{Result: sessionCycleResult{Outcome: sessionCycleSuccess, Run: sessionRunResult{Outcome: sessionRunSuccess}}},
	}}
	emitter := &fakeLifecycleEmitter{}
	result, err := newTestMultiRunner(cycles, emitter).run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != "completed" || cycles.calls != 2 {
		t.Fatalf("result=%+v calls=%d", result, cycles.calls)
	}
	if indexEvent(emitter.events, telemetry.GameRestartRequested) < 0 {
		t.Fatalf("restart event missing: %+v", emitter.events)
	}
}

func TestSessionMultiRunnerStopsTerminalUnknownFailure(t *testing.T) {
	cycles := &fakeMultiCycles{results: []sessionCycleExecution{{Result: sessionCycleResult{Outcome: sessionCycleFailed, Run: sessionRunResult{Outcome: sessionRunFailed, Reason: "mystery"}}}}}
	emitter := &fakeLifecycleEmitter{}
	result, err := newTestMultiRunner(cycles, emitter).run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != "failed" || result.Reason != "mystery" {
		t.Fatalf("result = %+v", result)
	}
	if indexEvent(emitter.events, telemetry.SessionFailed) < 0 {
		t.Fatalf("terminal event missing: %+v", emitter.events)
	}
}

func indexEvent(events []telemetry.Event, name telemetry.EventName) int {
	for index, event := range events {
		if event.Event == name {
			return index
		}
	}
	return -1
}
