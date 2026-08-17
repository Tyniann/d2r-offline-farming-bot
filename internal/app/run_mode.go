package app

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/input"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/memory"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/pathing"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/replay"
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
	if opts.Desktop || opts.UIStateProbe != "" || opts.ScreenAnchorCapture != "" || opts.OfflineExitTest || opts.OfflineDifficulty != "" || opts.TownTest != "" || opts.TownInspect || opts.MercenaryProbe != "" || opts.CowProbe != "" || opts.WeaponSetProbe != "" {
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

func mapRunConfig(cfg *config.Config, runID string, requireFarmingRoute bool) (tasks.RunConfig, error) {
	return mapRunConfigWithProfile(cfg, runID, "", requireFarmingRoute)
}

func mapRunConfigWithProfile(cfg *config.Config, runID, profileID string, requireFarmingRoute bool) (tasks.RunConfig, error) {
	run, ok := cfg.Runs.Run(runID)
	if !ok {
		return tasks.RunConfig{}, fmt.Errorf("%s: %q", tasks.RunReasonConfigMissing, runID)
	}
	if strings.TrimSpace(profileID) == "" {
		resolved, err := resolveActiveCombatProfileID(cfg, nil, cfg.Session.Character, "")
		if err != nil {
			return tasks.RunConfig{}, err
		}
		profileID = resolved
	}
	profileCfg, profileOK := cfg.Profiles[profileID]
	if !profileOK {
		return tasks.RunConfig{}, fmt.Errorf("combat_profiles.%s is required", profileID)
	}
	combat, err := mapCombatConfigFromProfile(profileID, profileCfg)
	if err != nil {
		return tasks.RunConfig{}, err
	}
	mapped := tasks.RunConfig{
		StepTimeout:             time.Duration(cfg.Runs.StepTimeoutMs) * time.Millisecond,
		LootPickupDistanceTiles: cfg.Loot.Pickup.MaxDistanceTiles,
		RouteCombat: tasks.RouteCombatConfig{
			Enabled:                    run.RouteCombat.EnabledValue(),
			ImmediateRadiusTiles:       run.RouteCombat.ImmediateRadiusTiles,
			CorridorWidthTiles:         run.RouteCombat.CorridorWidthTiles,
			LandingRadiusTiles:         run.RouteCombat.LandingRadiusTiles,
			AttackDistanceTiles:        run.RouteCombat.AttackDistanceTiles,
			NoProgressTimeout:          time.Duration(run.RouteCombat.NoProgressTimeoutMs) * time.Millisecond,
			TeleportManaReservePercent: run.RouteCombat.TeleportManaReservePercent,
			ResumeManaPercent:          run.RouteCombat.ResumeManaPercent,
			EmergencyManaPercent:       run.RouteCombat.EmergencyManaPercent,
			ManaRecoveryTimeout:        time.Duration(run.RouteCombat.ManaRecoveryTimeoutMs) * time.Millisecond,
		},
		Combat: combat,
	}
	definition, definitionOK := tasks.DefaultRunRegistry().Definition(tasks.RunID(runID))
	if mapped.RouteCombat.Enabled && (!definitionOK || !definition.HasCapability(tasks.RunCapabilityRouteClear)) {
		return tasks.RunConfig{}, fmt.Errorf("runs.definitions.%s.route_combat.enabled requires %s", runID, tasks.RunCapabilityRouteClear)
	}
	if requireFarmingRoute {
		assignmentStore, assignmentErr := NewRouteAssignmentStore(cfg)
		if assignmentErr != nil {
			return tasks.RunConfig{}, assignmentErr
		}
		definition, definitionOK := tasks.DefaultRunRegistry().Definition(tasks.RunID(runID))
		if definitionOK && definition.RouteSet != nil {
			var roles map[pathing.RouteRole]string
			roles, _, assignmentErr = assignmentStore.ResolveRouteSet(cfg.Session.Character, tasks.RunID(runID))
			mapped.SetupRouteID = roles[pathing.RouteRoleLegAcquisition]
			mapped.RouteID = roles[pathing.RouteRoleCowSweep]
			if assignmentErr == nil && (mapped.SetupRouteID == "" || mapped.RouteID == "") {
				assignmentErr = fmt.Errorf("%s", RouteReasonAssignmentMissing)
			}
		} else {
			mapped.RouteID, _, assignmentErr = assignmentStore.Resolve(cfg.Session.Character, tasks.RunID(runID))
		}
		if assignmentErr != nil {
			return tasks.RunConfig{}, assignmentErr
		}
	}
	resolved, err := tasks.DefaultRunRegistry().Resolve(tasks.RunID(runID), map[tasks.RunID]tasks.RunConfig{tasks.RunID(runID): mapped})
	if err != nil {
		return tasks.RunConfig{}, err
	}
	return resolved.Config, nil
}

func mapCowConfig(cfg *config.Config, bindings configBindingSource, inventoryGrid [][]int) tasks.CowConfig {
	mapped := tasks.CowConfig{
		Character: cfg.Session.Character, Difficulty: cfg.Session.Difficulty,
		ClientWidth: offlineDifficultyClientWidth, ClientHeight: offlineDifficultyClientHeight,
	}
	for row := 0; row < len(inventoryGrid) && row < len(mapped.InventoryLocked); row++ {
		for col := 0; col < len(inventoryGrid[row]) && col < len(mapped.InventoryLocked[row]); col++ {
			mapped.InventoryLocked[row][col] = inventoryGrid[row][col] == 1
		}
	}
	mapped.HasTownPortal = cowBindingAvailable(bindings, memory.SkillTownPortal)
	mapped.HasTeleport = cowBindingAvailable(bindings, memory.SkillTeleport)
	mapped.HasAmplifyDamage = cowBindingAvailable(bindings, memory.SkillAmplifyDamage)
	mapped.HasCorpseExplosion = cowBindingAvailable(bindings, memory.SkillCorpseExplosion)
	mapped.HasBoneSpear = cowBindingAvailable(bindings, memory.SkillBoneSpear)
	return mapped
}

func cowBindingAvailable(bindings configBindingSource, skillID uint16) bool {
	_, err := bindings.Resolve(skillID)
	return err == nil
}

// validateRunMode checks run prerequisites after resolving CLI vs config.
func validateRunMode(sel tasks.RunSelection, cfg *config.Config, opts Options, log *slog.Logger) error {
	if opts.RuntimeTraceCapture != "" {
		if err := replay.ValidateCaptureLabel(opts.RuntimeTraceCapture); err != nil {
			return fmt.Errorf("--runtime-trace-capture: %w", err)
		}
		if opts.Run == "" || opts.RunPhase != "" {
			return fmt.Errorf("--runtime-trace-capture requires an explicit full --run without --phase")
		}
		if opts.InputTest != "" || opts.PathingTest != "" || opts.OfflineDifficulty != "" || opts.OfflineCharacter != "" || opts.OfflineExitTest || opts.UIStateProbe != "" || opts.ScreenAnchorCapture != "" || opts.MercenaryProbe != "" || opts.CowProbe != "" || opts.WeaponSetProbe != "" || opts.Route != "" || opts.TownInspect || opts.TownTest != "" || opts.SessionInspect || opts.RunsInspect || opts.WaypointTargetsInspect {
			return fmt.Errorf("--runtime-trace-capture is only valid with one explicit full run")
		}
	}
	if opts.UIStateProbe != "" {
		if opts.InputTest != "" || opts.PathingTest != "" || opts.OfflineDifficulty != "" || opts.OfflineCharacter != "" || opts.OfflineExitTest || opts.ScreenAnchorCapture != "" || opts.MercenaryProbe != "" || opts.Route != "" || opts.Run != "" || opts.RunPhase != "" {
			return fmt.Errorf("--ui-state-probe is mutually exclusive with run and other test modes")
		}
		if err := validateUIStateProbeLabel(opts.UIStateProbe); err != nil {
			return err
		}
		if opts.UIStateProbeTimeoutMs < 0 {
			return fmt.Errorf("--ui-state-probe-timeout-ms must not be negative")
		}
	}
	if opts.MercenaryProbe != "" {
		if opts.InputTest != "" || opts.PathingTest != "" || opts.OfflineDifficulty != "" || opts.OfflineCharacter != "" || opts.OfflineExitTest || opts.UIStateProbe != "" || opts.ScreenAnchorCapture != "" || opts.Route != "" || opts.Run != "" || opts.RunPhase != "" || opts.TownInspect || opts.TownTest != "" {
			return fmt.Errorf("--mercenary-probe is mutually exclusive with run and other test modes")
		}
		if err := validateMercenaryProbeLabel(opts.MercenaryProbe); err != nil {
			return err
		}
		if opts.MercenaryProbeTimeoutMs < 0 {
			return fmt.Errorf("--mercenary-probe-timeout-ms must not be negative")
		}
	}
	if opts.CowProbe != "" {
		if opts.InputTest != "" || opts.PathingTest != "" || opts.OfflineDifficulty != "" || opts.OfflineCharacter != "" || opts.OfflineExitTest || opts.UIStateProbe != "" || opts.ScreenAnchorCapture != "" || opts.MercenaryProbe != "" || opts.WeaponSetProbe != "" || opts.Route != "" || opts.Run != "" || opts.RunPhase != "" || opts.TownInspect || opts.TownTest != "" {
			return fmt.Errorf("--cow-probe is mutually exclusive with run and other test modes")
		}
		if err := validateCowProbeLabel(opts.CowProbe); err != nil {
			return err
		}
		if opts.CowProbeTimeoutMs < 0 {
			return fmt.Errorf("--cow-probe-timeout-ms must not be negative")
		}
	}
	if opts.WeaponSetProbe != "" {
		if opts.InputTest != "" || opts.PathingTest != "" || opts.OfflineDifficulty != "" || opts.OfflineCharacter != "" || opts.OfflineExitTest || opts.UIStateProbe != "" || opts.ScreenAnchorCapture != "" || opts.MercenaryProbe != "" || opts.CowProbe != "" || opts.Route != "" || opts.Run != "" || opts.RunPhase != "" || opts.TownInspect || opts.TownTest != "" {
			return fmt.Errorf("--weapon-set-probe is mutually exclusive with run and other test modes")
		}
		if err := validateWeaponSetProbeLabel(opts.WeaponSetProbe); err != nil {
			return err
		}
		if opts.WeaponSetProbeTimeoutMs < 0 {
			return fmt.Errorf("--weapon-set-probe-timeout-ms must not be negative")
		}
	}
	if opts.ScreenAnchorCapture != "" {
		if opts.InputTest != "" || opts.PathingTest != "" || opts.OfflineDifficulty != "" || opts.OfflineCharacter != "" || opts.OfflineExitTest || opts.UIStateProbe != "" || opts.MercenaryProbe != "" || opts.Route != "" || opts.Run != "" || opts.RunPhase != "" {
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
			if !cfg.Input.Enabled {
				return fmt.Errorf("guided route recording requires input.enabled=true for the TP safety return")
			}
			if _, err := parseOfflineDifficulty(opts.RouteDifficulty); err != nil {
				return fmt.Errorf("--route-difficulty is required for %s: %w", command.action, err)
			}
		} else if command.action == "record-egress" {
			if opts.RouteDifficulty != "" {
				return fmt.Errorf("--route-difficulty is invalid for global system Egress recording")
			}
		} else if opts.RouteName != "" || opts.RouteDifficulty != "" {
			return fmt.Errorf("--route-name and --route-difficulty are only valid with route record or record-egress")
		}
		if (command.action == "play-segment" || command.action == "play" || command.action == "play-egress") && !cfg.Input.Enabled {
			return fmt.Errorf("route playback requires input.enabled=true")
		}
	}
	if opts.Route == "" && (opts.RouteName != "" || opts.RouteDifficulty != "") {
		return fmt.Errorf("--route-name and --route-difficulty require --route record:<id> or record-egress:<act2|act3|act4|act5>")
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
	_, configured := cfg.Runs.Run(sel.Run)
	if !configured {
		return fmt.Errorf("%s: %q", tasks.RunReasonConfigMissing, sel.Run)
	}
	if sel.Phase != "" && !isSupportedRunPhase(sel.Phase) {
		return fmt.Errorf("%w: run=%q phase=%q", errUnsupportedRunPhase, sel.Run, sel.Phase)
	}
	profileID, err := resolveRuntimeCombatProfileID(cfg, opts.Loadout)
	if err != nil {
		return err
	}
	profileCfg := cfg.Profiles[profileID]
	availability, err := ResolveRunAvailabilities(cfg, RunAvailabilityContext{
		Character: cfg.Session.Character, CharacterClass: profileCfg.CharacterClass, CombatProfile: profileID,
		Difficulty: cfg.Session.Difficulty, GameVersion: cfg.Memory.GameVersion,
	})
	if err != nil {
		return err
	}
	selected, ok := findRunAvailability(availability.Runs, tasks.RunID(sel.Run))
	if !ok {
		return fmt.Errorf("%s: %q", tasks.RunReasonUnknown, sel.Run)
	}
	blockingReasons := selected.Reasons
	if runPhaseAllowsUnavailableFarmingRoute(sel.Phase) {
		blockingReasons = withoutFarmingRouteReasons(blockingReasons)
	}
	if selected.Status == tasks.RunAvailabilityUnavailable && len(blockingReasons) > 0 {
		return fmt.Errorf("run %q unavailable: %s", sel.Run, joinRunReasons(blockingReasons))
	}
	if sel.Phase == "" {
		if err := validateFullRunBindingsWithProfile(cfg, sel.Run, opts.LoadoutBindings(), profileID); err != nil {
			return err
		}
	}
	if sel.Phase == tasks.RunPhaseTravelEntry {
		if err := validateProfileBindingsWithSource(cfg, opts.LoadoutBindings(), profileID, "profile"); err != nil {
			return err
		}
	}
	if sel.Phase == tasks.RunPhaseTownReady {
		if err := validateProfileBindingsWithSource(cfg, opts.LoadoutBindings(), profileID, "profile"); err != nil {
			return err
		}
	}
	if sel.Phase == tasks.RunPhaseBoss {
		if err := validateBossBindingsWithProfile(cfg, sel.Run, opts.LoadoutBindings(), profileID); err != nil {
			return err
		}
	}
	if sel.Phase == tasks.RunPhaseLootAndReturn {
		if err := validateLootBindings(opts.LoadoutBindings()); err != nil {
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

func runPhaseAllowsUnavailableFarmingRoute(phase string) bool {
	return phase == tasks.RunPhaseLootAndReturn
}

// farmingRouteRequired reports whether Runtime construction must resolve a published
// character/run farming assignment. Desktop without an explicit run and isolated
// Town/Merc diagnostics keep Countess only as a profile carrier.
func farmingRouteRequired(opts Options, sel tasks.RunSelection) bool {
	if runPhaseAllowsUnavailableFarmingRoute(sel.Phase) {
		return false
	}
	if opts.Desktop && sel.Run == "" {
		return false
	}
	if sel.Run == "" && (opts.TownTest != "" || opts.TownInspect || opts.MercenaryProbe != "" || opts.CowProbe != "" || opts.WeaponSetProbe != "" || opts.InputTest != "" || opts.Probe || opts.UIStateProbe != "" || opts.ScreenAnchorCapture != "" || opts.PathingTest != "" || opts.OfflineDifficulty != "" || opts.OfflineExitTest || opts.Route != "") {
		return false
	}
	return !opts.Desktop || sel.Run != ""
}

func withoutFarmingRouteReasons(reasons []tasks.RunReason) []tasks.RunReason {
	filtered := make([]tasks.RunReason, 0, len(reasons))
	for _, reason := range reasons {
		switch reason {
		case tasks.RunReasonRouteAssignmentMissing,
			tasks.RunReasonRouteMissing,
			tasks.RunReasonRouteBindingMismatch,
			tasks.RunReasonRouteLayoutMismatch,
			tasks.RunReasonRouteRuntimeValidation,
			tasks.RunReasonRouteStale,
			tasks.RunReasonRouteLifecycleUnavailable:
			continue
		default:
			filtered = append(filtered, reason)
		}
	}
	return filtered
}

func isSupportedRunPhase(phase string) bool {
	switch phase {
	case tasks.RunPhaseTravelEntry, tasks.RunPhasePlayRoute, tasks.RunPhaseBoss, tasks.RunPhaseLootAndReturn, tasks.RunPhaseStashPersonal, tasks.RunPhaseTownReady:
		return true
	default:
		return false
	}
}

func validateFullRunBindingsWithProfile(cfg *config.Config, runID string, bindings configBindingSource, profileID string) error {
	if _, resolveErr := bindings.Resolve(memory.SkillTeleport); resolveErr != nil {
		return fmt.Errorf("%s requires teleport binding: %w", runID, resolveErr)
	}
	if _, ok := cfg.Runs.Run(runID); !ok {
		return fmt.Errorf("%s: %q", tasks.RunReasonConfigMissing, runID)
	}
	profileCfg, ok := cfg.Profiles[profileID]
	if !ok {
		return fmt.Errorf("%s requires combat_profiles.%s", runID, profileID)
	}
	attackSkill, err := memory.ParseSkillTestName(profileCfg.Combat.StandardAttack)
	if err != nil {
		return fmt.Errorf("%s attack skill %q: %w", runID, profileCfg.Combat.StandardAttack, err)
	}
	attackCast, err := bindings.Resolve(attackSkill)
	if err != nil {
		return fmt.Errorf("%s requires %s binding: %w", runID, profileCfg.Combat.StandardAttack, err)
	}
	if err := requireStandardAttackButton(runID, profileCfg, attackCast); err != nil {
		return err
	}
	runCfg, _ := cfg.Runs.Run(runID)
	if runCfg.RouteCombat.EnabledValue() {
		openerCast, resolveErr := bindings.Resolve(memory.SkillAmplifyDamage)
		if resolveErr != nil {
			return fmt.Errorf("%s route combat requires amplify_damage binding: %w", runID, resolveErr)
		}
		if openerCast.CastButton != input.MouseRight {
			return fmt.Errorf("%s route combat opener amplify_damage must use right mouse, configured=%s", runID, openerCast.CastButton)
		}
	}
	if _, err := bindings.Resolve(memory.SkillTownPortal); err != nil {
		return fmt.Errorf("%s requires town_portal binding: %w", runID, err)
	}
	if err := validateBeltSlotConfigured(bindings, 1, runID); err != nil {
		return err
	}
	if err := validateBeltSlotConfigured(bindings, 4, runID); err != nil {
		return err
	}
	if err := validateProfileBindingsWithSource(cfg, bindings, profileID, runID); err != nil {
		return err
	}
	return nil
}

func validateProfileBindingsWithSource(cfg *config.Config, bindings configBindingSource, profileID, scope string) error {
	profileCfg, ok := cfg.Profiles[profileID]
	if !ok {
		return fmt.Errorf("%s requires combat_profiles.%s", scope, profileID)
	}
	for _, actions := range [][]config.ProfileActionConfig{profileCfg.Hooks.TownReady, profileCfg.Hooks.BossEngage} {
		for _, action := range actions {
			skillID, err := memory.ParseSkillTestName(action.Skill)
			if err != nil {
				return fmt.Errorf("%s profile skill %q: %w", scope, action.Skill, err)
			}
			if _, err := bindings.Resolve(skillID); err != nil {
				return fmt.Errorf("%s requires %s binding: %w", scope, action.Skill, err)
			}
		}
	}
	if maintenance := profileCfg.RouteMaintenance.BoneArmor; maintenance.Enabled != nil && *maintenance.Enabled {
		skillID, err := memory.ParseSkillTestName(maintenance.Skill)
		if err != nil {
			return fmt.Errorf("%s profile maintenance skill %q: %w", scope, maintenance.Skill, err)
		}
		if _, err := bindings.Resolve(skillID); err != nil {
			return fmt.Errorf("%s requires %s binding: %w", scope, maintenance.Skill, err)
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

func validateBossBindingsWithProfile(cfg *config.Config, runID string, bindings configBindingSource, profileID string) error {
	if _, resolveErr := bindings.Resolve(memory.SkillTeleport); resolveErr != nil {
		return fmt.Errorf("boss requires teleport binding: %w", resolveErr)
	}
	if _, ok := cfg.Runs.Run(runID); !ok {
		return fmt.Errorf("%s: %q", tasks.RunReasonConfigMissing, runID)
	}
	profileCfg, ok := cfg.Profiles[profileID]
	if !ok {
		return fmt.Errorf("boss requires combat_profiles.%s", profileID)
	}
	attackSkill, err := memory.ParseSkillTestName(profileCfg.Combat.StandardAttack)
	if err != nil {
		return fmt.Errorf("boss attack skill %q: %w", profileCfg.Combat.StandardAttack, err)
	}
	attackCast, err := bindings.Resolve(attackSkill)
	if err != nil {
		return fmt.Errorf("boss requires %s binding: %w", profileCfg.Combat.StandardAttack, err)
	}
	if err := requireStandardAttackButton("boss", profileCfg, attackCast); err != nil {
		return err
	}
	return nil
}

// requireStandardAttackButton compares the frozen binding against the
// profile-declared mouse slot of `combat.standard_attack`. Necromancer Bone
// Spear stays RMB; Hammerdin Blessed Hammer is LMB because required_skills
// already pins that slot. There is no RMB fallback for Blessed Hammer.
func requireStandardAttackButton(scope string, profileCfg config.ProfileConfig, attackCast input.SkillCast) error {
	want := standardAttackMouseButton(profileCfg)
	if attackCast.CastButton != want {
		return fmt.Errorf("%s attack skill %s must use %s mouse, configured=%s", scope, profileCfg.Combat.StandardAttack, want, attackCast.CastButton)
	}
	return nil
}

func standardAttackMouseButton(profileCfg config.ProfileConfig) input.MouseButton {
	if profileSkillSlot(profileCfg, profileCfg.Combat.StandardAttack) == "left" {
		return input.MouseLeft
	}
	return input.MouseRight
}

func validateLootBindings(bindings configBindingSource) error {
	if _, err := bindings.Resolve(memory.SkillTeleport); err != nil {
		return fmt.Errorf("loot-and-return requires teleport binding: %w", err)
	}
	if _, err := bindings.Resolve(memory.SkillTownPortal); err != nil {
		return fmt.Errorf("loot-and-return requires town_portal binding: %w", err)
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
		return fmt.Errorf("%s requires belt slot_%d: %w", scope, slot, err)
	}
	if key == "" {
		return fmt.Errorf("%s requires belt slot_%d: %w", scope, slot, input.ErrUnconfiguredSlot)
	}
	return nil
}

// LoadoutBindings returns the frozen loadout bindings or an empty source when unset.
func (opts Options) LoadoutBindings() configBindingSource {
	if opts.Loadout == nil {
		return configBindingSource{skills: make(map[uint16]input.SkillCast)}
	}
	return opts.Loadout.Bindings
}

func (rt *Runtime) shouldTickTasks(cur world.State) bool {
	if rt.CompatibilitySnapshot().State != D2RCompatibilityCompatible {
		return false
	}
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
