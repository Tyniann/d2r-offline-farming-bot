package tasks

import (
	"context"
	"log/slog"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

const (
	safetyPotionThrottle = 1500 * time.Millisecond
	safetyHealingHP      = 65
	safetyFullRejuvHP    = 35
)

// RunConfig holds task-runner settings mapped from application config.
type RunConfig struct {
	// StepTimeout is the default per-step wait timeout for non-tick-based steps.
	StepTimeout time.Duration
	// CountessCombat tunes the Countess kill phase.
	CountessCombat CountessCombatConfig
}

// CountessCombatConfig holds resolved Countess combat settings for task logic.
type CountessCombatConfig struct {
	// AttackSkillID is the resolved skill ID used for attack casts.
	AttackSkillID uint16
	// AttackInterval is the minimum delay between real combat inputs.
	AttackInterval time.Duration
	// EngageDistanceTiles is the desired distance after combat repositioning.
	EngageDistanceTiles float64
	// RepositionDistanceTiles triggers teleport repositioning when exceeded.
	RepositionDistanceTiles float64
	// KillConfirmTicks confirms death after consecutive valid absence ticks.
	KillConfirmTicks int
}

// Runner executes high-level run state machines.
type Runner struct {
	log       *slog.Logger
	deps      Deps
	runConfig RunConfig
	selection RunSelection
	run       runMachine
	tracker   stepTracker

	started  bool
	terminal bool
	reset    bool
	outcome  RunOutcome

	lastSafetyPotionAt time.Time
}

// NewRunner builds a task runner for sel (empty Run = passive mode).
func NewRunner(log *slog.Logger, sel RunSelection, cfg RunConfig, deps Deps) *Runner {
	r := &Runner{
		log:       log.With("component", "tasks"),
		deps:      deps,
		runConfig: cfg,
		selection: sel,
		outcome:   RunOutcomeIdle,
	}
	if sel.Run != "" {
		run, err := newRunMachine(sel, cfg)
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
	return r.selection.Run
}

// ConfiguredPhase returns the optional selected run phase.
func (r *Runner) ConfiguredPhase() string {
	return r.selection.Phase
}

// Terminal reports whether the run finished with success or failure.
func (r *Runner) Terminal() bool {
	return r.terminal
}

// WasReset reports whether the run was reset (e.g. after process lost).
func (r *Runner) WasReset() bool {
	return r.reset
}

// Reset aborts an active run. No-op when no run is configured.
func (r *Runner) Reset(reason string) {
	if r.selection.Run == "" {
		return
	}
	if r.reset {
		return
	}
	r.reset = true
	r.outcome = RunOutcomeIdle
	if r.deps.Waypoint != nil {
		r.deps.Waypoint.Reset()
	}
	if r.deps.Portal != nil {
		r.deps.Portal.Reset()
	}
	if r.deps.TownWalk != nil {
		r.deps.TownWalk.Reset()
	}
	if r.deps.Stash != nil {
		r.deps.Stash.Reset()
	}
	if r.deps.Pathing != nil {
		r.deps.Pathing.Reset()
	}
	if r.deps.Combat != nil {
		r.deps.Combat.Reset()
	}
	if r.deps.Loot != nil {
		r.deps.Loot.Reset()
	}
	r.log.Info("task run reset", "run", r.selection.Run, "phase", r.selection.Phase, "reason", reason)
}

// Tick advances the configured run by one poll when guards allow.
func (r *Runner) Tick(ctx context.Context, w world.State, now time.Time) TickResult {
	if r.selection.Run == "" || r.reset || r.terminal {
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
		r.log.Info("task run started", "run", r.selection.Run, "phase", r.selection.Phase)
		r.beginStep(r.run.firstStep(), now)
	}

	r.tracker.incrementTick()
	if res, ok := r.tickSafetyPotion(now, w); ok {
		return res
	}
	result := r.run.onTick(ctx, r.deps, r.tracker.name, w, now, r.tracker.startedAt, r.tracker.ticksInStep)

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

func (r *Runner) tickSafetyPotion(now time.Time, w world.State) (TickResult, bool) {
	if !w.Valid || w.Phase != world.GamePhaseInGame || w.Player.MaxHP == 0 {
		return TickResult{}, false
	}
	slot := 0
	hp := w.Player.HPPercent()
	switch {
	case hp <= safetyFullRejuvHP:
		slot = 4
	case hp <= safetyHealingHP:
		slot = 1
	default:
		return TickResult{}, false
	}
	if !r.lastSafetyPotionAt.IsZero() && now.Sub(r.lastSafetyPotionAt) < safetyPotionThrottle {
		return TickResult{}, false
	}
	if r.deps.Actions == nil {
		return TickResult{}, false
	}
	if err := r.deps.Actions.CastBelt(slot); err != nil {
		return r.finishStepFailed(now, "safety_potion_failed"), true
	}
	r.lastSafetyPotionAt = now
	r.log.Info("safety potion cast",
		"run", r.selection.Run,
		"phase", r.selection.Phase,
		"step", r.tracker.name,
		"hp_percent", hp,
		"belt_slot", slot,
	)
	return TickResult{
		Active:  true,
		Outcome: RunOutcomeRunning,
		Step:    r.tracker.name,
	}, true
}

// CurrentStepAllowsNonInputTick reports whether the active step may tick while
// the world is loading/invalid. These ticks must not perform input.
func (r *Runner) CurrentStepAllowsNonInputTick() bool {
	if r == nil || !r.started || r.run == nil || r.reset || r.terminal {
		return false
	}
	return r.run.allowsNonInputTick(r.tracker.name)
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
	if r.deps.Waypoint != nil {
		r.deps.Waypoint.Reset()
	}
	if r.deps.Portal != nil {
		r.deps.Portal.Reset()
	}
	if r.deps.TownWalk != nil {
		r.deps.TownWalk.Reset()
	}
	if r.deps.Stash != nil {
		r.deps.Stash.Reset()
	}
	if r.deps.Pathing != nil {
		r.deps.Pathing.Reset()
	}
	if r.deps.Combat != nil {
		r.deps.Combat.Reset()
	}
	if name == countessStepPickLoot && r.deps.Loot != nil {
		r.deps.Loot.Reset()
	}
	if r.run != nil {
		r.run.onStepEnter(name)
	}
	r.log.Info("task step started", "run", r.selection.Run, "phase", r.selection.Phase, "step", name)
}

func (r *Runner) finishStepComplete(now time.Time) TickResult {
	step := r.tracker.name
	logArgs := []any{"run", r.selection.Run, "phase", r.selection.Phase, "step", step}
	if r.run != nil && r.run.usesTickTimeout(step) {
		logArgs = append(logArgs, "ticks", r.tracker.ticksInStep)
	} else {
		logArgs = append(logArgs, "elapsed_ms", r.tracker.elapsed(now).Milliseconds())
	}
	r.log.Info("task step complete", logArgs...)
	if r.selection.Run == "countess" && r.selection.Phase == "" && step == countessStepCloseStash {
		r.log.Info("countess run complete", "run", r.selection.Run, "step", countessStepComplete, "completion", "personal_stash_complete")
	}
	if step == countessStepPickLoot && r.deps.Loot != nil {
		r.deps.Loot.Reset()
	}

	next := r.run.nextStep(step)
	if next == "" {
		r.terminal = true
		r.outcome = RunOutcomeSuccess
		if r.deps.Loot != nil {
			r.deps.Loot.Reset()
		}
		r.log.Info("task run finished", "run", r.selection.Run, "phase", r.selection.Phase, "outcome", RunOutcomeSuccess)
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
		"run", r.selection.Run,
		"phase", r.selection.Phase,
		"step", step,
		"reason", reason,
		"elapsed_ms", r.tracker.elapsed(now).Milliseconds(),
	)
	r.terminal = true
	r.outcome = RunOutcomeFailed
	if r.deps.Waypoint != nil {
		r.deps.Waypoint.Reset()
	}
	if r.deps.Portal != nil {
		r.deps.Portal.Reset()
	}
	if r.deps.TownWalk != nil {
		r.deps.TownWalk.Reset()
	}
	if r.deps.Stash != nil {
		r.deps.Stash.Reset()
	}
	if r.deps.Pathing != nil {
		r.deps.Pathing.Reset()
	}
	if r.deps.Combat != nil {
		r.deps.Combat.Reset()
	}
	if r.deps.Loot != nil {
		r.deps.Loot.Reset()
	}
	r.log.Info("task run finished",
		"run", r.selection.Run,
		"phase", r.selection.Phase,
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
