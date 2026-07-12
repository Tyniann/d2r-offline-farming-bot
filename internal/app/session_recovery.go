package app

import (
	"fmt"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/telemetry"
)

type sessionFailureClass string

const (
	sessionFailureRestartable sessionFailureClass = "run_restartable"
	sessionFailureTerminal    sessionFailureClass = "terminal"
	sessionFailureStop        sessionFailureClass = "operator_stop"
)

type sessionRecoveryDecision string

const (
	sessionRecoveryContinue sessionRecoveryDecision = "continue"
	sessionRecoveryRestart  sessionRecoveryDecision = "restart_game"
	sessionRecoveryTerminal sessionRecoveryDecision = "fail_session"
	sessionRecoveryStopped  sessionRecoveryDecision = "stop_session"
)

var sessionRestartableReasons = map[string]struct{}{
	"hard_stuck":              {},
	"route_drift_exceeded":    {},
	"route_segment_timeout":   {},
	"route_transition_failed": {},
}

type sessionRecoveryPolicy struct {
	allowed                map[string]struct{}
	maxConsecutiveFailures int
	maxTotalRestarts       int
	consecutiveFailures    int
	totalRestarts          int
	runsStarted            int
	runsSuccessful         int
	runsAborted            int
	runsFailed             int
}

func newSessionRecoveryPolicy(allowed []string, maxConsecutiveFailures, maxTotalRestarts int) *sessionRecoveryPolicy {
	allowedSet := make(map[string]struct{}, len(allowed))
	for _, reason := range allowed {
		allowedSet[reason] = struct{}{}
	}
	return &sessionRecoveryPolicy{allowed: allowedSet, maxConsecutiveFailures: maxConsecutiveFailures, maxTotalRestarts: maxTotalRestarts}
}

func classifySessionFailure(reason string) sessionFailureClass {
	if reason == "operator_stop" {
		return sessionFailureStop
	}
	if _, ok := sessionRestartableReasons[reason]; ok {
		return sessionFailureRestartable
	}
	return sessionFailureTerminal
}

func (p *sessionRecoveryPolicy) evaluate(result sessionRunResult) sessionRecoveryDecision {
	p.runsStarted++
	if result.Outcome == sessionRunSuccess {
		p.runsSuccessful++
		p.consecutiveFailures = 0
		return sessionRecoveryContinue
	}
	if result.Outcome == sessionRunAborted {
		p.runsAborted++
	} else {
		p.runsFailed++
	}
	if classifySessionFailure(result.Reason) == sessionFailureStop {
		return sessionRecoveryStopped
	}
	p.consecutiveFailures++
	if classifySessionFailure(result.Reason) != sessionFailureRestartable {
		return sessionRecoveryTerminal
	}
	if _, allowed := p.allowed[result.Reason]; !allowed {
		return sessionRecoveryTerminal
	}
	if p.consecutiveFailures > p.maxConsecutiveFailures || p.totalRestarts >= p.maxTotalRestarts {
		return sessionRecoveryTerminal
	}
	p.totalRestarts++
	return sessionRecoveryRestart
}

func (c *sessionRecoveryCoordinator) emitTerminal(event telemetry.EventName, reason string, elapsedMs int64) error {
	if event != telemetry.SessionCompleted && event != telemetry.SessionStopped && event != telemetry.SessionFailed {
		return fmt.Errorf("invalid terminal session event %q", event)
	}
	summary := telemetry.Event{
		Event: event, Reason: reason, ElapsedMs: elapsedMs,
		RunsStarted: c.policy.runsStarted, RunsSuccessful: c.policy.runsSuccessful,
		RunsAborted: c.policy.runsAborted, RunsFailed: c.policy.runsFailed,
		ConsecutiveFailures: c.policy.consecutiveFailures, TotalRestarts: c.policy.totalRestarts,
	}
	if err := c.emitter.Emit(summary); err != nil {
		return fmt.Errorf("emit %s: %w", event, err)
	}
	return nil
}

type sessionStuckContext struct {
	RouteID               string
	SegmentID             string
	PointIndex            int
	LastConfirmedPoint    int
	TargetX               uint32
	TargetY               uint32
	DriftTiles            float64
	LocalRecoveryAttempts int
}

type sessionRunContext struct {
	GameID    string
	RunID     string
	Run       string
	Ordinal   int
	ElapsedMs int64
	Stuck     sessionStuckContext
}

type sessionLifecycleEmitter interface {
	Emit(telemetry.Event) error
}

type sessionRecoveryCoordinator struct {
	policy  *sessionRecoveryPolicy
	emitter sessionLifecycleEmitter
}

func (c *sessionRecoveryCoordinator) handle(result sessionRunResult, context sessionRunContext) (sessionRecoveryDecision, error) {
	if c == nil || c.policy == nil || c.emitter == nil {
		return sessionRecoveryTerminal, fmt.Errorf("session recovery dependencies are required")
	}
	base := telemetry.Event{
		GameID: context.GameID, RunID: context.RunID, Run: context.Run, RunOrdinal: context.Ordinal,
		Reason: result.Reason, LastStep: result.Step, ElapsedMs: context.ElapsedMs,
	}
	if result.Reason == "hard_stuck" {
		point, confirmed := context.Stuck.PointIndex, context.Stuck.LastConfirmedPoint
		stuck := base
		stuck.Event = telemetry.StuckDetected
		stuck.RouteID, stuck.SegmentID, stuck.PointIndex = context.Stuck.RouteID, context.Stuck.SegmentID, &point
		stuck.LastConfirmedPoint = &confirmed
		stuck.TargetX, stuck.TargetY = context.Stuck.TargetX, context.Stuck.TargetY
		stuck.DriftTiles, stuck.LocalRecoveryAttempts = context.Stuck.DriftTiles, context.Stuck.LocalRecoveryAttempts
		if err := c.emitter.Emit(stuck); err != nil {
			return sessionRecoveryTerminal, fmt.Errorf("emit stuck_detected: %w", err)
		}
	}
	runEvent := base
	switch result.Outcome {
	case sessionRunSuccess:
		runEvent.Event, runEvent.Outcome = telemetry.RunCompleted, string(sessionRunSuccess)
	case sessionRunAborted:
		runEvent.Event, runEvent.Outcome = telemetry.RunAborted, string(sessionRunAborted)
	default:
		runEvent.Event, runEvent.Outcome = telemetry.RunFailed, string(sessionRunFailed)
	}
	if err := c.emitter.Emit(runEvent); err != nil {
		return sessionRecoveryTerminal, fmt.Errorf("emit %s: %w", runEvent.Event, err)
	}
	decision := c.policy.evaluate(result)
	if decision == sessionRecoveryRestart {
		recovery := base
		recovery.Event = telemetry.GameRestartRequested
		recovery.Decision = string(decision)
		recovery.ConsecutiveFailures = c.policy.consecutiveFailures
		recovery.TotalRestarts = c.policy.totalRestarts
		recovery.RemainingRestarts = c.policy.maxTotalRestarts - c.policy.totalRestarts
		if err := c.emitter.Emit(recovery); err != nil {
			return sessionRecoveryTerminal, fmt.Errorf("emit game_restart_requested: %w", err)
		}
	}
	return decision, nil
}
