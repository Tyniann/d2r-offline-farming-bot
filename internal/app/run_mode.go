package app

import (
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/input"
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
	if opts.UIStateProbe != "" || opts.ScreenAnchorCapture != "" || opts.OfflineExitTest || opts.OfflineDifficulty != "" {
		return ""
	}
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
		StepTimeout:     time.Duration(runs.StepTimeoutMs) * time.Millisecond,
		CountessRouteID: runs.Countess.RouteID,
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
	if opts.UIStateProbe != "" {
		if opts.InputTest != "" || opts.PathingTest != "" || opts.OfflineDifficulty != "" || opts.OfflineCharacter != "" || opts.OfflineExitTest || opts.ScreenAnchorCapture != "" || opts.Route != "" || opts.Run != "" || opts.RunPhase != "" {
			return fmt.Errorf("--ui-state-probe is mutually exclusive with run and other test modes")
		}
		if err := validateUIStateProbeLabel(opts.UIStateProbe); err != nil {
			return err
		}
		if opts.UIStateProbeTimeoutMs < 0 {
			return fmt.Errorf("--ui-state-probe-timeout-ms must not be negative")
		}
	}
	if opts.ScreenAnchorCapture != "" {
		if opts.InputTest != "" || opts.PathingTest != "" || opts.OfflineDifficulty != "" || opts.OfflineCharacter != "" || opts.OfflineExitTest || opts.UIStateProbe != "" || opts.Route != "" || opts.Run != "" || opts.RunPhase != "" {
			return fmt.Errorf("--screen-anchor-capture is mutually exclusive with run and other test modes")
		}
		if err := validateUIStateProbeLabel(opts.ScreenAnchorCapture); err != nil {
			return fmt.Errorf("screen anchor label: %w", err)
		}
	}
	if opts.OfflineExitTest {
		if !cfg.Input.Enabled {
			return fmt.Errorf("offline exit test requires input.enabled=true")
		}
		if opts.InputTest != "" || opts.PathingTest != "" || opts.OfflineDifficulty != "" || opts.OfflineCharacter != "" || opts.UIStateProbe != "" || opts.ScreenAnchorCapture != "" || opts.Route != "" || opts.Run != "" || opts.RunPhase != "" {
			return fmt.Errorf("--offline-exit-test is mutually exclusive with run and other test modes")
		}
	}
	if opts.Route != "" {
		if opts.InputTest != "" || opts.PathingTest != "" || opts.OfflineDifficulty != "" || opts.OfflineExitTest || opts.ScreenAnchorCapture != "" || sel.Run != "" || sel.Phase != "" {
			return fmt.Errorf("--route is mutually exclusive with run and other test modes")
		}
		command, err := parseRouteCommand(opts.Route)
		if err != nil {
			return err
		}
		if command.action == "record" {
			if _, err := parseOfflineDifficulty(opts.RouteDifficulty); err != nil {
				return fmt.Errorf("--route-difficulty is required for record: %w", err)
			}
		} else if opts.RouteName != "" || opts.RouteDifficulty != "" {
			return fmt.Errorf("--route-name and --route-difficulty are only valid with route record")
		}
		if (command.action == "play-segment" || command.action == "play") && !cfg.Input.Enabled {
			return fmt.Errorf("route playback requires input.enabled=true")
		}
	}
	if opts.Route == "" && (opts.RouteName != "" || opts.RouteDifficulty != "") {
		return fmt.Errorf("--route-name and --route-difficulty require --route record:<id>")
	}
	if opts.OfflineDifficulty != "" {
		if !cfg.Input.Enabled {
			return fmt.Errorf("offline difficulty test requires input.enabled=true")
		}
		if opts.InputTest != "" || opts.PathingTest != "" || opts.OfflineExitTest || opts.ScreenAnchorCapture != "" || sel.Run != "" || sel.Phase != "" {
			return fmt.Errorf("--offline-difficulty-test is mutually exclusive with run and other test modes")
		}
		if _, err := parseOfflineDifficulty(opts.OfflineDifficulty); err != nil {
			return err
		}
		if _, err := validateOfflineCharacter(opts.OfflineCharacter); err != nil {
			return err
		}
	} else if opts.OfflineCharacter != "" {
		return fmt.Errorf("--offline-character requires --offline-difficulty-test")
	}
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
	if sel.Run == "countess" && sel.Phase == "" {
		if cfg.Runs.Countess.RouteID == "" {
			return fmt.Errorf("runs.countess.route_id is required for the full Countess run")
		}
		if err := validateFullCountessBindings(cfg); err != nil {
			return err
		}
	}
	if sel.Run == "countess" && sel.Phase == tasks.CountessPhaseTravelCellar5 && cfg.Runs.Countess.RouteID == "" {
		return fmt.Errorf("runs.countess.route_id is required for travel-cellar5")
	}
	if sel.Run == "countess" && sel.Phase == tasks.CountessPhaseKillCountess {
		if err := validateKillCountessBindings(cfg); err != nil {
			return err
		}
	}
	if sel.Run == "countess" && sel.Phase == tasks.CountessPhaseLootCountess {
		if err := validateLootCountessBindings(cfg); err != nil {
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
	case tasks.CountessPhaseTravelMarsh, tasks.CountessPhaseTravelCellar5, tasks.CountessPhaseKillCountess, tasks.CountessPhaseLootCountess, tasks.CountessPhaseStashPersonal:
		return true
	default:
		return false
	}
}

func validateFullCountessBindings(cfg *config.Config) error {
	bindings, err := newConfigBindingSource(cfg.Input.Bindings)
	if err != nil {
		return err
	}
	if _, err := bindings.Resolve(memory.SkillTeleport); err != nil {
		return fmt.Errorf("countess requires input.bindings.skills.teleport: %w", err)
	}
	if _, err := bindings.Resolve(memory.SkillBoneSpear); err != nil {
		return fmt.Errorf("countess requires input.bindings.skills.bone_spear: %w", err)
	}
	if _, err := bindings.Resolve(memory.SkillTownPortal); err != nil {
		return fmt.Errorf("countess requires input.bindings.skills.town_portal: %w", err)
	}
	if err := validateBeltSlotConfigured(bindings, 1, "countess"); err != nil {
		return err
	}
	if err := validateBeltSlotConfigured(bindings, 4, "countess"); err != nil {
		return err
	}
	return nil
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

func validateLootCountessBindings(cfg *config.Config) error {
	bindings, err := newConfigBindingSource(cfg.Input.Bindings)
	if err != nil {
		return err
	}
	if _, err := bindings.Resolve(memory.SkillTeleport); err != nil {
		return fmt.Errorf("loot-countess requires input.bindings.skills.teleport: %w", err)
	}
	if _, err := bindings.Resolve(memory.SkillTownPortal); err != nil {
		return fmt.Errorf("loot-countess requires input.bindings.skills.town_portal: %w", err)
	}
	if err := validateBeltSlotConfigured(bindings, 1, "loot-countess"); err != nil {
		return err
	}
	if err := validateBeltSlotConfigured(bindings, 4, "loot-countess"); err != nil {
		return err
	}
	return nil
}

func validateBeltSlotConfigured(bindings configBindingSource, slot int, scope string) error {
	key, err := bindings.BeltKeyName(slot)
	if err != nil {
		return fmt.Errorf("%s requires input.bindings.belt.slot_%d: %w", scope, slot, err)
	}
	if key == "" {
		return fmt.Errorf("%s requires input.bindings.belt.slot_%d: %w", scope, slot, input.ErrUnconfiguredSlot)
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
