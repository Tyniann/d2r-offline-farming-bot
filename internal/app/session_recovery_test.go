package app

import (
	"errors"
	"testing"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/telemetry"
)

type fakeLifecycleEmitter struct {
	events []telemetry.Event
	failAt int
}

func (e *fakeLifecycleEmitter) Emit(event telemetry.Event) error {
	if e.failAt > 0 && len(e.events)+1 == e.failAt {
		return errors.New("disk full")
	}
	e.events = append(e.events, event)
	return nil
}

func TestSessionRecoveryHardStuckOrderAndBudgets(t *testing.T) {
	emitter := &fakeLifecycleEmitter{}
	policy := newSessionRecoveryPolicy([]string{"hard_stuck"}, 2, 2)
	coordinator := sessionRecoveryCoordinator{policy: policy, emitter: emitter}
	decision, err := coordinator.handle(
		sessionRunResult{Outcome: sessionRunAborted, Reason: "hard_stuck", Step: "play_route"},
		sessionRunContext{GameID: "game-1", RunID: "run-1", Run: "countess", Ordinal: 1, Stuck: sessionStuckContext{RouteID: "route", SegmentID: "segment", PointIndex: 4, LastConfirmedPoint: 3, LocalRecoveryAttempts: 2}},
	)
	if err != nil {
		t.Fatal(err)
	}
	if decision != sessionRecoveryRestart {
		t.Fatalf("decision = %s", decision)
	}
	want := []telemetry.EventName{telemetry.StuckDetected, telemetry.RunAborted, telemetry.GameRestartRequested}
	if len(emitter.events) != len(want) {
		t.Fatalf("events = %+v", emitter.events)
	}
	for i, event := range emitter.events {
		if event.Event != want[i] || event.GameID != "game-1" || event.RunID != "run-1" {
			t.Fatalf("event %d = %+v", i, event)
		}
	}
	if emitter.events[2].RemainingRestarts != 1 || policy.totalRestarts != 1 || policy.consecutiveFailures != 1 {
		t.Fatalf("recovery budgets event=%+v policy=%+v", emitter.events[2], policy)
	}
}

func TestSessionRecoveryEnforcesExactReasonsAndHardLimits(t *testing.T) {
	policy := newSessionRecoveryPolicy([]string{"hard_stuck"}, 2, 1)
	if got := policy.evaluate(sessionRunResult{Outcome: sessionRunFailed, Reason: "hard_stuck_extra"}); got != sessionRecoveryTerminal {
		t.Fatalf("unknown prefixed reason decision = %s", got)
	}
	policy = newSessionRecoveryPolicy([]string{"hard_stuck"}, 2, 1)
	if got := policy.evaluate(sessionRunResult{Outcome: sessionRunAborted, Reason: "hard_stuck"}); got != sessionRecoveryRestart {
		t.Fatalf("first decision = %s", got)
	}
	if got := policy.evaluate(sessionRunResult{Outcome: sessionRunAborted, Reason: "hard_stuck"}); got != sessionRecoveryTerminal {
		t.Fatalf("restart budget decision = %s", got)
	}
	if got := policy.evaluate(sessionRunResult{Outcome: sessionRunSuccess}); got != sessionRecoveryContinue || policy.consecutiveFailures != 0 {
		t.Fatalf("success decision=%s failures=%d", got, policy.consecutiveFailures)
	}
}

func TestSessionRecoveryTelemetryFailureBlocksDecision(t *testing.T) {
	emitter := &fakeLifecycleEmitter{failAt: 2}
	policy := newSessionRecoveryPolicy([]string{"hard_stuck"}, 2, 2)
	decision, err := (&sessionRecoveryCoordinator{policy: policy, emitter: emitter}).handle(
		sessionRunResult{Outcome: sessionRunAborted, Reason: "hard_stuck"}, sessionRunContext{},
	)
	if err == nil || decision != sessionRecoveryTerminal {
		t.Fatalf("decision=%s err=%v", decision, err)
	}
	if policy.totalRestarts != 0 || len(emitter.events) != 1 || emitter.events[0].Event != telemetry.StuckDetected {
		t.Fatalf("recovery progressed after telemetry failure: events=%+v policy=%+v", emitter.events, policy)
	}
}

func TestSessionRecoveryTerminalSummaryContainsCounters(t *testing.T) {
	emitter := &fakeLifecycleEmitter{}
	policy := newSessionRecoveryPolicy([]string{"hard_stuck"}, 2, 2)
	coordinator := &sessionRecoveryCoordinator{policy: policy, emitter: emitter}
	policy.evaluate(sessionRunResult{Outcome: sessionRunSuccess})
	policy.evaluate(sessionRunResult{Outcome: sessionRunAborted, Reason: "hard_stuck"})
	if err := coordinator.emitTerminal(telemetry.SessionFailed, "budget_exhausted", 1234); err != nil {
		t.Fatal(err)
	}
	event := emitter.events[0]
	if event.RunsStarted != 2 || event.RunsSuccessful != 1 || event.RunsAborted != 1 || event.RunsFailed != 0 || event.TotalRestarts != 1 {
		t.Fatalf("summary = %+v", event)
	}
}
