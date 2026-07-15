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

func mapRunConfig(runs config.RunsConfig, runID string) (tasks.RunConfig, error) {
	run, ok := runs.Run(runID)
	if !ok {
		return tasks.RunConfig{}, fmt.Errorf("%s: %q", tasks.RunReasonConfigMissing, runID)
	}
	attackSkillID, err := memory.ParseSkillTestName(run.Combat.AttackSkill)
	if err != nil {
		return tasks.RunConfig{}, fmt.Errorf("runs.definitions.%s.combat.attack_skill: %w", runID, err)
	}
	mapped := tasks.RunConfig{
		StepTimeout: time.Duration(runs.StepTimeoutMs) * time.Millisecond,
		RouteID:     run.RouteID,
		Combat: tasks.CombatConfig{
			Profile:                 run.Combat.Profile,
			AttackSkillID:           attackSkillID,
			AttackInterval:          time.Duration(run.Combat.AttackIntervalMs) * time.Millisecond,
			EngageDistanceTiles:     run.Combat.EngageDistanceTiles,
			RepositionDistanceTiles: run.Combat.RepositionDistanceTiles,
			KillConfirmTicks:        run.Combat.KillConfirmTicks,
		},
		Loot: tasks.RunLootConfig{PickupFile: run.Loot.PickupFile, SellFile: run.Loot.SellFile},
	}
	resolved, err := tasks.DefaultRunRegistry().Resolve(tasks.RunID(runID), map[tasks.RunID]tasks.RunConfig{tasks.RunID(runID): mapped})
	if err != nil {
		return tasks.RunConfig{}, err
	}
	return resolved.Config, nil
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
		if command.action == "record" || command.action == "record-egress" {
			if _, err := parseOfflineDifficulty(opts.RouteDifficulty); err != nil {
				return fmt.Errorf("--route-difficulty is required for %s: %w", command.action, err)
			}
		} else if opts.RouteName != "" || opts.RouteDifficulty != "" {
			return fmt.Errorf("--route-name and --route-difficulty are only valid with route record or record-egress")
		}
		if (command.action == "play-segment" || command.action == "play" || command.action == "play-egress") && !cfg.Input.Enabled {
			return fmt.Errorf("route playback requires input.enabled=true")
		}
	}
	if opts.Route == "" && (opts.RouteName != "" || opts.RouteDifficulty != "") {
		return fmt.Errorf("--route-name and --route-difficulty require --route record:<id> or record-egress:act3")
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
	runCfg, configured := cfg.Runs.Run(sel.Run)
	if !configured {
		return fmt.Errorf("%s: %q", tasks.RunReasonConfigMissing, sel.Run)
	}
	if sel.Phase != "" && !isSupportedRunPhase(sel.Phase) {
		return fmt.Errorf("%w: run=%q phase=%q", errUnsupportedRunPhase, sel.Run, sel.Phase)
	}
	availability, err := ResolveRunAvailabilities(cfg, RunAvailabilityContext{
		Character: cfg.Session.Character, Difficulty: cfg.Session.Difficulty, GameVersion: cfg.Memory.GameVersion,
	})
	if err != nil {
		return err
	}
	selected, ok := findRunAvailability(availability.Runs, tasks.RunID(sel.Run))
	if !ok {
		return fmt.Errorf("%s: %q", tasks.RunReasonUnknown, sel.Run)
	}
	if selected.Status == tasks.RunAvailabilityUnavailable {
		return fmt.Errorf("run %q unavailable: %s", sel.Run, joinRunReasons(selected.Reasons))
	}
	if sel.Phase == "" {
		if err := validateFullRunBindings(cfg, sel.Run); err != nil {
			return err
		}
	}
	if sel.Phase == tasks.RunPhasePlayRoute && runCfg.RouteID == "" {
		return fmt.Errorf("runs.definitions.%s.route_id is required for play-route", sel.Run)
	}
	if sel.Phase == tasks.RunPhaseTravelEntry {
		if err := validateProfileBindings(cfg, sel.Run); err != nil {
			return err
		}
	}
	if sel.Phase == tasks.RunPhaseTownReady {
		if err := validateProfileBindings(cfg, sel.Run); err != nil {
			return err
		}
	}
	if sel.Phase == tasks.RunPhaseBoss {
		if err := validateBossBindings(cfg, sel.Run); err != nil {
			return err
		}
	}
	if sel.Phase == tasks.RunPhaseLootAndReturn {
		if err := validateLootBindings(cfg); err != nil {
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

func isSupportedRunPhase(phase string) bool {
	switch phase {
	case tasks.RunPhaseTravelEntry, tasks.RunPhasePlayRoute, tasks.RunPhaseBoss, tasks.RunPhaseLootAndReturn, tasks.RunPhaseStashPersonal, tasks.RunPhaseTownReady:
		return true
	default:
		return false
	}
}

func validateFullRunBindings(cfg *config.Config, runID string) error {
	bindings, err := newConfigBindingSource(cfg.Input.Bindings)
	if err != nil {
		return err
	}
	if _, resolveErr := bindings.Resolve(memory.SkillTeleport); resolveErr != nil {
		return fmt.Errorf("%s requires input.bindings.skills.teleport: %w", runID, resolveErr)
	}
	runCfg, ok := cfg.Runs.Run(runID)
	if !ok {
		return fmt.Errorf("%s: %q", tasks.RunReasonConfigMissing, runID)
	}
	attackSkill, err := memory.ParseSkillTestName(runCfg.Combat.AttackSkill)
	if err != nil {
		return fmt.Errorf("%s attack skill %q: %w", runID, runCfg.Combat.AttackSkill, err)
	}
	attackCast, err := bindings.Resolve(attackSkill)
	if err != nil {
		return fmt.Errorf("%s requires input.bindings.skills.%s: %w", runID, runCfg.Combat.AttackSkill, err)
	}
	if attackCast.CastButton != input.MouseRight {
		return fmt.Errorf("%s attack skill %s must use right mouse, configured=%s", runID, runCfg.Combat.AttackSkill, attackCast.CastButton)
	}
	if _, err := bindings.Resolve(memory.SkillTownPortal); err != nil {
		return fmt.Errorf("%s requires input.bindings.skills.town_portal: %w", runID, err)
	}
	if err := validateBeltSlotConfigured(bindings, 1, runID); err != nil {
		return err
	}
	if err := validateBeltSlotConfigured(bindings, 4, runID); err != nil {
		return err
	}
	if err := validateProfileBindingsWithSource(cfg, bindings, runID, runID); err != nil {
		return err
	}
	return nil
}

func validateProfileBindings(cfg *config.Config, runID string) error {
	bindings, err := newConfigBindingSource(cfg.Input.Bindings)
	if err != nil {
		return err
	}
	return validateProfileBindingsWithSource(cfg, bindings, runID, "profile")
}

func validateProfileBindingsWithSource(cfg *config.Config, bindings configBindingSource, runID, scope string) error {
	runCfg, configured := cfg.Runs.Run(runID)
	if !configured {
		return fmt.Errorf("%s: %q", tasks.RunReasonConfigMissing, runID)
	}
	profileCfg, ok := cfg.Profiles[runCfg.Combat.Profile]
	if !ok {
		return fmt.Errorf("%s requires combat_profiles.%s", scope, runCfg.Combat.Profile)
	}
	for _, actions := range [][]config.ProfileActionConfig{profileCfg.Hooks.TownReady, profileCfg.Hooks.BossEngage} {
		for _, action := range actions {
			skillID, err := memory.ParseSkillTestName(action.Skill)
			if err != nil {
				return fmt.Errorf("%s profile skill %q: %w", scope, action.Skill, err)
			}
			if _, err := bindings.Resolve(skillID); err != nil {
				return fmt.Errorf("%s requires input.bindings.skills.%s: %w", scope, action.Skill, err)
			}
		}
	}
	for _, resource := range []config.ResourceRuleConfig{profileCfg.Resources.Healing, profileCfg.Resources.Mana, profileCfg.Resources.Rejuvenation} {
		for _, slot := range resource.BeltSlots {
			if err := validateBeltSlotConfigured(bindings, slot, scope); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateBossBindings(cfg *config.Config, runID string) error {
	bindings, err := newConfigBindingSource(cfg.Input.Bindings)
	if err != nil {
		return err
	}
	if _, resolveErr := bindings.Resolve(memory.SkillTeleport); resolveErr != nil {
		return fmt.Errorf("boss requires input.bindings.skills.teleport: %w", resolveErr)
	}
	runCfg, ok := cfg.Runs.Run(runID)
	if !ok {
		return fmt.Errorf("%s: %q", tasks.RunReasonConfigMissing, runID)
	}
	attackSkill, err := memory.ParseSkillTestName(runCfg.Combat.AttackSkill)
	if err != nil {
		return fmt.Errorf("boss attack skill %q: %w", runCfg.Combat.AttackSkill, err)
	}
	attackCast, err := bindings.Resolve(attackSkill)
	if err != nil {
		return fmt.Errorf("boss requires input.bindings.skills.%s: %w", runCfg.Combat.AttackSkill, err)
	}
	if attackCast.CastButton != input.MouseRight {
		return fmt.Errorf("boss attack skill %s must use right mouse, configured=%s", runCfg.Combat.AttackSkill, attackCast.CastButton)
	}
	return nil
}

func validateLootBindings(cfg *config.Config) error {
	bindings, err := newConfigBindingSource(cfg.Input.Bindings)
	if err != nil {
		return err
	}
	if _, err := bindings.Resolve(memory.SkillTeleport); err != nil {
		return fmt.Errorf("loot-and-return requires input.bindings.skills.teleport: %w", err)
	}
	if _, err := bindings.Resolve(memory.SkillTownPortal); err != nil {
		return fmt.Errorf("loot-and-return requires input.bindings.skills.town_portal: %w", err)
	}
	if err := validateBeltSlotConfigured(bindings, 1, "loot-and-return"); err != nil {
		return err
	}
	if err := validateBeltSlotConfigured(bindings, 4, "loot-and-return"); err != nil {
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
