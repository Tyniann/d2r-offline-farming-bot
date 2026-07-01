package tasks

import (
	"context"
	"log/slog"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

// RunConfig holds task-runner settings mapped from application config.
type RunConfig struct {
	// StepTimeout is the default per-step wait timeout for non-tick-based steps.
	StepTimeout time.Duration
}

// Runner executes high-level run state machines.
type Runner struct {
	log           *slog.Logger
	deps          Deps
	runConfig     RunConfig
	configuredRun string
	run           runMachine
	tracker       stepTracker

	started  bool
	terminal bool
	reset    bool
	outcome  RunOutcome
}

// NewRunner builds a task runner for runName (empty = passive mode).
func NewRunner(log *slog.Logger, runName string, cfg RunConfig, deps Deps) *Runner {
	r := &Runner{
		log:           log.With("component", "tasks"),
		deps:          deps,
		runConfig:     cfg,
		configuredRun: runName,
		outcome:       RunOutcomeIdle,
	}
	if runName != "" {
		run, err := newRunMachine(runName)
		if err == nil {
			r.run = run
		}
	}
	return r
}

// Ready reports whether the runner is initialized for component verification.
func (r *Runner) Ready() bool {
	return r != nil && r.log != nil
}

// ConfiguredRun returns the resolved run name from CLI or config.
func (r *Runner) ConfiguredRun() string {
	return r.configuredRun
}

// Terminal reports whether the run finished with success or failure.
func (r *Runner) Terminal() bool {
	return r.terminal
}

// WasReset reports whether the run was reset (e.g. after process lost).
func (r *Runner) WasReset() bool {
	return r.reset
}

// Reset aborts an active run. No-op without log when configuredRun is empty.
func (r *Runner) Reset(reason string) {
	if r.configuredRun == "" {
		return
	}
	if r.reset {
		return
	}
	r.reset = true
	r.outcome = RunOutcomeIdle
	r.log.Info("task run reset", "run", r.configuredRun, "reason", reason)
}

// Tick advances the configured run by one poll when guards allow.
func (r *Runner) Tick(_ context.Context, w world.State, now time.Time) TickResult {
	if r.configuredRun == "" || r.reset || r.terminal {
		return r.inactiveResult()
	}
	if r.run == nil {
		return TickResult{
			Active:  false,
			Outcome: RunOutcomeFailed,
			Reason:  "unknown_run",
		}
	}

	if !r.started {
		r.started = true
		r.outcome = RunOutcomeRunning
		r.log.Info("task run started", "run", r.configuredRun)
		r.beginStep(r.run.firstStep(), now)
	}

	r.tracker.incrementTick()
	result := r.run.onTick(r.tracker.name, w, r.tracker.ticksInStep)

	if result.failed {
		return r.finishStepFailed(now, result.reason)
	}
	if result.complete {
		return r.finishStepComplete(now)
	}
	if r.tracker.timedOut(now) {
		return r.finishStepFailed(now, "timeout")
	}

	return TickResult{
		Active:  true,
		Outcome: RunOutcomeRunning,
		Step:    r.tracker.name,
	}
}

func (r *Runner) inactiveResult() TickResult {
	outcome := r.outcome
	if outcome == RunOutcomeIdle {
		return TickResult{Active: false, Outcome: RunOutcomeIdle}
	}
	return TickResult{
		Active:  false,
		Outcome: outcome,
		Step:    r.tracker.name,
	}
}

func (r *Runner) beginStep(name string, now time.Time) {
	timeout := time.Duration(0)
	if r.run != nil && !r.run.usesTickTimeout(name) {
		timeout = r.runConfig.StepTimeout
	}
	r.tracker.begin(name, now, timeout)
	r.log.Info("task step started", "run", r.configuredRun, "step", name)
}

func (r *Runner) finishStepComplete(now time.Time) TickResult {
	step := r.tracker.name
	logArgs := []any{"run", r.configuredRun, "step", step}
	if r.run != nil && r.run.usesTickTimeout(step) {
		logArgs = append(logArgs, "ticks", r.tracker.ticksInStep)
	} else {
		logArgs = append(logArgs, "elapsed_ms", r.tracker.elapsed(now).Milliseconds())
	}
	r.log.Info("task step complete", logArgs...)

	next := r.run.nextStep(step)
	if next == "" {
		r.terminal = true
		r.outcome = RunOutcomeSuccess
		r.log.Info("task run finished", "run", r.configuredRun, "outcome", RunOutcomeSuccess)
		return TickResult{
			Active:  true,
			Outcome: RunOutcomeSuccess,
			Step:    step,
		}
	}

	r.beginStep(next, now)
	return TickResult{
		Active:  true,
		Outcome: RunOutcomeRunning,
		Step:    next,
	}
}

func (r *Runner) finishStepFailed(now time.Time, reason string) TickResult {
	step := r.tracker.name
	r.log.Info("task step failed",
		"run", r.configuredRun,
		"step", step,
		"reason", reason,
		"elapsed_ms", r.tracker.elapsed(now).Milliseconds(),
	)
	r.terminal = true
	r.outcome = RunOutcomeFailed
	r.log.Info("task run finished",
		"run", r.configuredRun,
		"outcome", RunOutcomeFailed,
		"reason", reason,
	)
	return TickResult{
		Active:  true,
		Outcome: RunOutcomeFailed,
		Step:    step,
		Reason:  reason,
	}
}
