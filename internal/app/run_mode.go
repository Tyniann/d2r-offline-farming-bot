package app

import (
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/memory"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/tasks"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

var (
	errInputRequiredForRun         = errors.New("input.enabled must be true when a run is configured")
	errUnknownRun                  = errors.New("unknown run name")
	errRunInputTestConflict        = errors.New("--run and --input-test are mutually exclusive")
	errPathingTestConflict         = errors.New("--pathing-test is mutually exclusive with --run and --input-test")
	errInputRequiredForPathingTest = errors.New("input.enabled must be true for this pathing test spec")
	errRunPhaseRequiresRun         = errors.New("--phase requires an active run")
	errUnsupportedRunPhase         = errors.New("unsupported run phase")
)

// resolveActiveRun returns the configured run name; CLI overrides YAML.
func resolveActiveRun(opts Options, cfg *config.Config) string {
	if opts.Run != "" {
		return opts.Run
	}
	return cfg.Runs.Active
}

func resolveRunSelection(opts Options, cfg *config.Config) tasks.RunSelection {
	return tasks.RunSelection{Run: resolveActiveRun(opts, cfg), Phase: opts.RunPhase}
}

func mapRunConfig(runs config.RunsConfig) tasks.RunConfig {
	attackSkillID, _ := memory.ParseSkillTestName(runs.Countess.Combat.AttackSkill)
	return tasks.RunConfig{
		StepTimeout: time.Duration(runs.StepTimeoutMs) * time.Millisecond,
		CountessCombat: tasks.CountessCombatConfig{
			AttackSkillID:           attackSkillID,
			AttackInterval:          time.Duration(runs.Countess.Combat.AttackIntervalMs) * time.Millisecond,
			EngageDistanceTiles:     runs.Countess.Combat.EngageDistanceTiles,
			RepositionDistanceTiles: runs.Countess.Combat.RepositionDistanceTiles,
			KillConfirmTicks:        runs.Countess.Combat.KillConfirmTicks,
		},
	}
}

// validateRunMode checks run prerequisites after resolving CLI vs config.
func validateRunMode(sel tasks.RunSelection, cfg *config.Config, opts Options, log *slog.Logger) error {
	if err := validatePathingTestMode(cfg, opts); err != nil {
		return err
	}
	if sel.Run == "" {
		if sel.Phase != "" {
			return errRunPhaseRequiresRun
		}
		return nil
	}
	if opts.InputTest != "" {
		return errRunInputTestConflict
	}
	if opts.PathingTest != "" {
		return errPathingTestConflict
	}
	if !cfg.Input.Enabled {
		return errInputRequiredForRun
	}
	if !tasks.IsKnownRun(sel.Run) {
		return fmt.Errorf("%w: %q", errUnknownRun, sel.Run)
	}
	if sel.Phase != "" && !(sel.Run == "countess" && isSupportedCountessPhase(sel.Phase)) {
		return fmt.Errorf("%w: run=%q phase=%q", errUnsupportedRunPhase, sel.Run, sel.Phase)
	}
	if sel.Run == "countess" && sel.Phase == tasks.CountessPhaseKillCountess {
		if err := validateKillCountessBindings(cfg); err != nil {
			return err
		}
	}

	source := "config"
	if opts.Run != "" {
		source = "cli"
	}
	log.Info("task run configured", "run", sel.Run, "phase", sel.Phase, "source", source)
	return nil
}

func isSupportedCountessPhase(phase string) bool {
	switch phase {
	case tasks.CountessPhaseTravelMarsh, tasks.CountessPhaseTravelCellar5, tasks.CountessPhaseKillCountess:
		return true
	default:
		return false
	}
}

func validateKillCountessBindings(cfg *config.Config) error {
	bindings, err := newConfigBindingSource(cfg.Input.Bindings)
	if err != nil {
		return err
	}
	if _, err := bindings.Resolve(memory.SkillTeleport); err != nil {
		return fmt.Errorf("kill-countess requires input.bindings.skills.teleport: %w", err)
	}
	if _, err := bindings.Resolve(memory.SkillBoneSpear); err != nil {
		return fmt.Errorf("kill-countess requires input.bindings.skills.bone_spear: %w", err)
	}
	return nil
}

func (rt *Runtime) shouldTickTasks(cur world.State) bool {
	if rt.Tasks.ConfiguredRun() == "" {
		return false
	}
	if rt.Tasks.Terminal() || rt.Tasks.WasReset() {
		return false
	}
	if !rt.Config.Input.Enabled {
		return false
	}
	if !rt.Input.Bound() {
		return false
	}
	st := rt.Input.Status()
	if !st.Enabled || st.Paused || st.Stopped {
		return false
	}
	if cur.Valid && cur.Phase == world.GamePhaseInGame {
		return true
	}
	return rt.Tasks.CurrentStepAllowsNonInputTick()
}
