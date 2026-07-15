package app

import (
	"context"
	"fmt"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/telemetry"
)

type sessionCycleExecution struct {
	Result sessionCycleResult
	Stuck  sessionStuckContext
}

type sessionCycleExecutor interface {
	Execute(context.Context, bool) (sessionCycleExecution, error)
}

type sessionMultiConfig struct {
	Run           string
	MaxRuns       int
	MaxDuration   time.Duration
	Cooldown      time.Duration
	InitialInGame bool
	IDPrefix      string
}

type sessionMultiResult struct {
	Outcome string
	Reason  string
}

type sessionMultiRunner struct {
	config   sessionMultiConfig
	cycles   sessionCycleExecutor
	recovery *sessionRecoveryCoordinator
	now      func() time.Time
	wait     func(context.Context, time.Duration) error
}

func (r *sessionMultiRunner) run(ctx context.Context) (sessionMultiResult, error) {
	if r == nil || r.cycles == nil || r.recovery == nil || r.recovery.policy == nil || r.recovery.emitter == nil {
		return sessionMultiResult{Outcome: "failed", Reason: "session_dependencies_missing"}, fmt.Errorf("session multi-run dependencies are required")
	}
	if r.config.MaxRuns <= 0 || r.config.MaxDuration <= 0 {
		return sessionMultiResult{Outcome: "failed", Reason: "session_budget_invalid"}, fmt.Errorf("session multi-run budgets must be finite and positive")
	}
	if r.now == nil {
		r.now = time.Now
	}
	if r.wait == nil {
		r.wait = waitSessionCooldown
	}
	startedAt := r.now()
	if err := r.recovery.emitter.Emit(telemetry.Event{Event: telemetry.SessionStarted, Run: r.config.Run, MaxRuns: r.config.MaxRuns, MaxDurationMs: r.config.MaxDuration.Milliseconds()}); err != nil {
		return sessionMultiResult{Outcome: "failed", Reason: "telemetry_failed"}, fmt.Errorf("emit session_started: %w", err)
	}
	gameAlreadyActive := r.config.InitialInGame
	for ordinal := 1; ordinal <= r.config.MaxRuns; ordinal++ {
		if r.now().Sub(startedAt) >= r.config.MaxDuration {
			return r.complete("max_duration", startedAt)
		}
		execution, err := r.cycles.Execute(ctx, gameAlreadyActive)
		gameAlreadyActive = false
		if err != nil {
			_ = r.recovery.emitTerminal(telemetry.SessionFailed, "cycle_failed", r.now().Sub(startedAt).Milliseconds())
			return sessionMultiResult{Outcome: "failed", Reason: "cycle_failed"}, err
		}
		if execution.Result.Outcome == sessionCycleStopped {
			if emitErr := r.recovery.emitTerminal(telemetry.SessionStopped, "operator_stop", r.now().Sub(startedAt).Milliseconds()); emitErr != nil {
				return sessionMultiResult{Outcome: "failed", Reason: "telemetry_failed"}, emitErr
			}
			return sessionMultiResult{Outcome: "stopped", Reason: "operator_stop"}, nil
		}
		runContext := sessionRunContext{
			GameID: fmt.Sprintf("%s-game-%03d", r.config.IDPrefix, ordinal),
			RunID:  fmt.Sprintf("%s-run-%03d", r.config.IDPrefix, ordinal),
			Run:    r.config.Run, Ordinal: ordinal, Stuck: execution.Stuck,
		}
		decision, err := r.recovery.handle(execution.Result.Run, runContext)
		if err != nil {
			_ = r.recovery.emitTerminal(telemetry.SessionFailed, "telemetry_failed", r.now().Sub(startedAt).Milliseconds())
			return sessionMultiResult{Outcome: "failed", Reason: "telemetry_failed"}, err
		}
		switch decision {
		case sessionRecoveryStopped:
			if err := r.recovery.emitTerminal(telemetry.SessionStopped, "operator_stop", r.now().Sub(startedAt).Milliseconds()); err != nil {
				return sessionMultiResult{Outcome: "failed", Reason: "telemetry_failed"}, err
			}
			return sessionMultiResult{Outcome: "stopped", Reason: "operator_stop"}, nil
		case sessionRecoveryTerminal:
			if err := r.recovery.emitTerminal(telemetry.SessionFailed, execution.Result.Run.Reason, r.now().Sub(startedAt).Milliseconds()); err != nil {
				return sessionMultiResult{Outcome: "failed", Reason: "telemetry_failed"}, err
			}
			return sessionMultiResult{Outcome: "failed", Reason: execution.Result.Run.Reason}, nil
		}
		if ordinal == r.config.MaxRuns {
			return r.complete("max_runs", startedAt)
		}
		if r.now().Sub(startedAt) >= r.config.MaxDuration {
			return r.complete("max_duration", startedAt)
		}
		if err := r.wait(ctx, r.config.Cooldown); err != nil {
			if err := r.recovery.emitTerminal(telemetry.SessionStopped, "operator_stop", r.now().Sub(startedAt).Milliseconds()); err != nil {
				return sessionMultiResult{Outcome: "failed", Reason: "telemetry_failed"}, err
			}
			return sessionMultiResult{Outcome: "stopped", Reason: "operator_stop"}, nil
		}
	}
	return r.complete("max_runs", startedAt)
}

func (r *sessionMultiRunner) complete(reason string, startedAt time.Time) (sessionMultiResult, error) {
	if err := r.recovery.emitTerminal(telemetry.SessionCompleted, reason, r.now().Sub(startedAt).Milliseconds()); err != nil {
		return sessionMultiResult{Outcome: "failed", Reason: "telemetry_failed"}, err
	}
	return sessionMultiResult{Outcome: "completed", Reason: reason}, nil
}

func waitSessionCooldown(ctx context.Context, duration time.Duration) error {
	if duration <= 0 {
		return ctx.Err()
	}
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
