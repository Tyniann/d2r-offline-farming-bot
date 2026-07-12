package app

import (
	"context"
	"errors"
	"fmt"
)

type sessionCycleOutcome string

const (
	sessionCycleSuccess sessionCycleOutcome = "success"
	sessionCycleFailed  sessionCycleOutcome = "failed"
	sessionCycleStopped sessionCycleOutcome = "stopped"
)

type sessionRunOutcome string

const (
	sessionRunSuccess sessionRunOutcome = "success"
	sessionRunFailed  sessionRunOutcome = "failed"
	sessionRunAborted sessionRunOutcome = "aborted"
)

type sessionRunResult struct {
	Outcome sessionRunOutcome
	Reason  string
	Step    string
}

type sessionCycleResult struct {
	Outcome sessionCycleOutcome
	Run     sessionRunResult
	Reason  string
}

type sessionRunExecutor interface {
	Execute(context.Context) sessionRunResult
	Reset(reason string)
}

type sessionCycleDriver interface {
	AwaitReady(context.Context) error
	ResetForNextGame(reason string) error
	StartGame(context.Context) error
	VerifyGame(context.Context) error
	NewRun() (sessionRunExecutor, error)
	ExitGame(context.Context, sessionRunResult) error
	EmitLifecycle(event, reason string) error
}

type sessionCycleOrchestrator struct {
	driver sessionCycleDriver
}

func newSessionCycleOrchestrator(driver sessionCycleDriver) *sessionCycleOrchestrator {
	return &sessionCycleOrchestrator{driver: driver}
}

func (o *sessionCycleOrchestrator) execute(ctx context.Context, gameAlreadyActive bool) (sessionCycleResult, error) {
	if o == nil || o.driver == nil {
		return sessionCycleResult{Outcome: sessionCycleFailed, Reason: "driver_missing"}, fmt.Errorf("session cycle driver is required")
	}
	if err := o.emit("cycle_started", ""); err != nil {
		return sessionCycleResult{Outcome: sessionCycleFailed, Reason: "telemetry_failed"}, err
	}
	if err := o.beforeAction(ctx, "cycle_reset_requested"); err != nil {
		return o.stopOrFail(err, "cycle_reset_blocked")
	}
	if err := o.driver.ResetForNextGame("cycle_start"); err != nil {
		return o.fail("cycle_reset_failed", err)
	}
	if !gameAlreadyActive {
		if err := o.beforeAction(ctx, "game_start_requested"); err != nil {
			return o.stopOrFail(err, "start_game_blocked")
		}
		if err := o.driver.StartGame(ctx); err != nil {
			return o.fail("start_game_failed", err)
		}
	}
	if err := o.beforeAction(ctx, "game_verification_requested"); err != nil {
		return o.stopOrFail(err, "verify_game_blocked")
	}
	if err := o.driver.VerifyGame(ctx); err != nil {
		return o.fail("verify_game_failed", err)
	}
	if err := o.beforeAction(ctx, "run_creation_requested"); err != nil {
		return o.stopOrFail(err, "run_creation_blocked")
	}
	run, err := o.driver.NewRun()
	if err != nil {
		return o.fail("run_creation_failed", err)
	}
	if run == nil {
		return o.fail("run_creation_failed", fmt.Errorf("run factory returned nil"))
	}
	if err := o.emit("run_started", ""); err != nil {
		run.Reset("telemetry_failed")
		return sessionCycleResult{Outcome: sessionCycleFailed, Reason: "telemetry_failed"}, err
	}
	runResult := run.Execute(ctx)
	if err := ctx.Err(); err != nil {
		run.Reset("operator_stop")
		_ = o.emit("cycle_stopped", "operator_stop")
		return sessionCycleResult{Outcome: sessionCycleStopped, Run: sessionRunResult{Outcome: sessionRunAborted, Reason: "operator_stop"}, Reason: "operator_stop"}, nil
	}
	if runResult.Outcome != sessionRunSuccess && runResult.Outcome != sessionRunFailed && runResult.Outcome != sessionRunAborted {
		runResult = sessionRunResult{Outcome: sessionRunFailed, Reason: "invalid_run_outcome", Step: runResult.Step}
	}
	run.Reset("cycle_evaluate")
	if err := o.beforeAction(ctx, "game_exit_requested"); err != nil {
		return o.stopOrFailWithRun(err, "exit_game_blocked", runResult)
	}
	if err := o.driver.ExitGame(ctx, runResult); err != nil {
		return o.failWithRun("exit_game_failed", runResult, err)
	}
	if runResult.Outcome != sessionRunSuccess {
		return sessionCycleResult{Outcome: sessionCycleFailed, Run: runResult, Reason: runResult.Reason}, nil
	}
	return sessionCycleResult{Outcome: sessionCycleSuccess, Run: runResult}, nil
}

func (o *sessionCycleOrchestrator) beforeAction(ctx context.Context, event string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := o.driver.AwaitReady(ctx); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return o.emit(event, "")
}

func (o *sessionCycleOrchestrator) emit(event, reason string) error {
	if err := o.driver.EmitLifecycle(event, reason); err != nil {
		return fmt.Errorf("emit lifecycle event %s: %w", event, err)
	}
	return nil
}

func (o *sessionCycleOrchestrator) stopOrFail(err error, reason string) (sessionCycleResult, error) {
	return o.stopOrFailWithRun(err, reason, sessionRunResult{})
}

func (o *sessionCycleOrchestrator) stopOrFailWithRun(err error, reason string, run sessionRunResult) (sessionCycleResult, error) {
	if errors.Is(err, context.Canceled) {
		_ = o.emit("cycle_stopped", "operator_stop")
		return sessionCycleResult{Outcome: sessionCycleStopped, Run: run, Reason: "operator_stop"}, nil
	}
	return o.failWithRun(reason, run, err)
}

func (o *sessionCycleOrchestrator) fail(reason string, err error) (sessionCycleResult, error) {
	return o.failWithRun(reason, sessionRunResult{}, err)
}

func (o *sessionCycleOrchestrator) failWithRun(reason string, run sessionRunResult, err error) (sessionCycleResult, error) {
	_ = o.emit("cycle_failed", reason)
	return sessionCycleResult{Outcome: sessionCycleFailed, Run: run, Reason: reason}, fmt.Errorf("session cycle %s: %w", reason, err)
}
