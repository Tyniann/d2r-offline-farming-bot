package app

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/replay"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/tasks"
)

// runtimeControl records decisions made by the runtime rather than a task tick.
// Pending mercenary death stops attack without advancing or aborting the task.
func (rt *Runtime) runtimeControl(reason string, stopAttack bool) error {
	if rt.RuntimeTrace != nil && rt.RuntimeTrace.Enabled() {
		state, status := rt.World.Current(), rt.Input.Status()
		rt.RuntimeTrace.BeginTick(time.Now(), replay.NormalizeWorld(state), state.Generation, replay.RuntimeGates{InputEnabled: status.Enabled, Paused: status.Paused, Stopped: status.Stopped, WindowBound: rt.Input.Bound()}, traceTickState(rt.Tasks.Result()))
		rt.RuntimeTrace.RecordDependency("runtime.control", map[string]any{"reason": reason, "stop_attack": stopAttack}, nil, nil)
		defer func() { rt.RuntimeTrace.EndTick(traceTickState(rt.Tasks.Result())) }()
	}
	if stopAttack {
		if err := rt.taskDeps.Combat.StopAttack(); err != nil {
			return fmt.Errorf("runtime stop attack: %w", err)
		}
	}
	if reason != "" {
		return rt.Tasks.AbortOpenStep(reason)
	}
	return nil
}

// finishDiagnosticRecovery shares queue failure classification and exit gates.
// The productive trace is sealed after recovery: compression must never hold
// the character in combat, and recovery ticks belong to a different runner.
func (rt *Runtime) finishDiagnosticRecovery(ctx context.Context, result tasks.TickResult) error {
	if rt.Options.RuntimeTraceCapture == "" || result.Outcome != tasks.RunOutcomeFailed || rt.Input.Status().Stopped || ctx.Err() != nil {
		return nil
	}
	productiveTasks, recorder := rt.Tasks, rt.RuntimeTrace
	rt.RuntimeTrace = nil
	rt.productiveRunActive = false
	defer func() { rt.Tasks, rt.RuntimeTrace = productiveTasks, recorder }()
	classification, recoveryErr := classifyFailedQueueRun(ctx, productiveTasks.ConfiguredRun(), result.Reason, rt.Config.Session.RetryClasses, rt.World.Current(), rt.runRetryReturnToTown)
	if rt.Input.Status().Stopped || ctx.Err() != nil || errors.Is(recoveryErr, context.Canceled) {
		return recoveryErr
	}
	if classification.ExitAuthorization == ExitAuthorizationNone {
		return recoveryErr
	}
	unit := runtimeQueueUnit{runtime: rt}
	return errors.Join(recoveryErr, unit.ExitGame(ctx, classification, rt.Telemetry.Emit))
}

func traceTickState(result tasks.TickResult) replay.TickState {
	return replay.TickState{Step: result.Step, Outcome: string(result.Outcome), Reason: result.Reason, Active: result.Active}
}

func (rt *Runtime) runDiagnostic(ctx context.Context) error {
	rt.Log.Info("diagnostic run started", "run", rt.Tasks.ConfiguredRun())
	rt.productiveRunActive = true
	_, runErr := rt.runTaskToTerminal(ctx)
	rt.productiveRunActive = false
	result := rt.Tasks.Result()
	var recoveryErr error
	if runErr == nil {
		recoveryErr = rt.finishDiagnosticRecovery(ctx, result)
		if result.Outcome == tasks.RunOutcomeFailed {
			runErr = fmt.Errorf("diagnostic run failed at %s: %s", result.Step, result.Reason)
		}
	}
	return errors.Join(runErr, recoveryErr, rt.finalizeRuntimeTrace(replay.Terminal{Step: result.Step, Outcome: string(result.Outcome), Reason: result.Reason}))
}

func (rt *Runtime) finalizeRuntimeTrace(terminal replay.Terminal) error {
	if rt == nil || rt.RuntimeTrace == nil || !rt.RuntimeTrace.Enabled() {
		return nil
	}
	if rt.RuntimeTrace.FrameCount() == 0 {
		if err := rt.runtimeControl(terminal.Reason, false); err != nil {
			return err
		}
	}
	result, err := rt.RuntimeTrace.Finalize(terminal)
	if err != nil {
		return fmt.Errorf("finalize runtime trace: %w", err)
	}
	if result.Saved {
		rt.Log.Info("runtime trace saved", "file", result.Filename, "bytes", result.Bytes)
	}
	return nil
}

func runtimeTraceDirectory(cfg *config.Config) string {
	if cfg != nil && cfg.DataRoot != "" {
		return filepath.Join(cfg.DataRoot, "diagnostics", "runtime-traces")
	}
	return filepath.Join("diagnostics", "runtime-traces")
}

func runtimeTraceContract(cfg *config.Config, opts Options, selection tasks.RunSelection, runConfig tasks.RunConfig) replay.ContractSnapshot {
	definition, _ := tasks.DefaultRunRegistry().Definition(tasks.RunID(selection.Run))
	capabilities := make([]string, 0, len(definition.RequiredCaps))
	for _, capability := range definition.RequiredCaps {
		capabilities = append(capabilities, string(capability))
	}
	routeHostiles := append([]uint32(nil), definition.RouteHostileNPCIDs...)
	sort.Slice(routeHostiles, func(i, j int) bool { return routeHostiles[i] < routeHostiles[j] })
	contract := replay.ContractSnapshot{
		Policy:       map[string]any{"run_config": runConfig},
		RunID:        selection.Run,
		Phase:        selection.Phase,
		ProfileID:    runConfig.Combat.Profile,
		Difficulty:   cfg.Session.Difficulty,
		GameVersion:  cfg.Memory.GameVersion,
		Dependencies: []string{"input", "pathing", "waypoint", "portal", "town_walk", "stash", "combat", "actions", "loot", "compaction", "route", "route_clear", "town_egress", "profile", "town", "telemetry"},
		Definition: map[string]any{
			"display_name":                definition.DisplayName,
			"entry_area_id":               uint32(definition.EntryArea),
			"route_terminal_area_id":      uint32(definition.RouteTerminalArea),
			"waypoint_target":             string(definition.WaypointTarget),
			"boss_npc_id":                 definition.Boss.NPCID,
			"boss_require_super_unique":   definition.Boss.RequireSuperUnique,
			"boss_allow_fallback":         definition.Boss.AllowAnySuperUniqueFallback,
			"clear_nearby_after_boss":     definition.ClearNearbyAfterBoss,
			"route_hostile_npc_ids":       routeHostiles,
			"return_origin":               string(definition.ReturnOrigin),
			"required_capabilities":       capabilities,
			"recording_terminal_kind":     string(definition.Recording.TerminalKind),
			"recording_terminal_area_id":  uint32(definition.Recording.TerminalArea),
			"recording_terminal_distance": definition.Recording.TerminalMaxDistanceTiles,
		},
		Route: map[string]any{
			"route_id":       runConfig.RouteID,
			"setup_route_id": runConfig.SetupRouteID,
		},
		Tuning: map[string]any{
			"step_timeout_ms":                runConfig.StepTimeout.Milliseconds(),
			"loot_pickup_distance_tiles":     runConfig.LootPickupDistanceTiles,
			"attack_skill_id":                runConfig.Combat.AttackSkillID,
			"attack_interval_ms":             runConfig.Combat.AttackInterval.Milliseconds(),
			"engage_distance_tiles":          runConfig.Combat.EngageDistanceTiles,
			"reposition_distance_tiles":      runConfig.Combat.RepositionDistanceTiles,
			"kill_confirm_ticks":             runConfig.Combat.KillConfirmTicks,
			"route_combat_enabled":           runConfig.RouteCombat.Enabled,
			"route_immediate_radius_tiles":   runConfig.RouteCombat.ImmediateRadiusTiles,
			"route_corridor_width_tiles":     runConfig.RouteCombat.CorridorWidthTiles,
			"route_landing_radius_tiles":     runConfig.RouteCombat.LandingRadiusTiles,
			"route_attack_distance_tiles":    runConfig.RouteCombat.AttackDistanceTiles,
			"route_no_progress_timeout_ms":   runConfig.RouteCombat.NoProgressTimeout.Milliseconds(),
			"teleport_mana_reserve_percent":  runConfig.RouteCombat.TeleportManaReservePercent,
			"route_resume_mana_percent":      runConfig.RouteCombat.ResumeManaPercent,
			"route_emergency_mana_percent":   runConfig.RouteCombat.EmergencyManaPercent,
			"route_mana_recovery_timeout_ms": runConfig.RouteCombat.ManaRecoveryTimeout.Milliseconds(),
		},
	}
	if selection.Run == string(tasks.RunIDCows) {
		contract.Dependencies = append(contract.Dependencies, "cow", "cow_recipe")
	}
	if selection.Run == string(tasks.RunIDLowerKurast) {
		contract.Dependencies = append(contract.Dependencies, "chest")
	}
	if opts.Loadout == nil {
		return contract
	}
	contract.Character = opts.Loadout.Character
	bindings := make(map[string]any, len(opts.Loadout.Bindings.skills))
	for skillID, binding := range opts.Loadout.Bindings.skills {
		bindings[strconv.FormatUint(uint64(skillID), 10)] = map[string]any{
			"select_key":  binding.SelectKey,
			"cast_button": string(binding.CastButton),
		}
	}
	contract.Loadout = map[string]any{
		"character":            opts.Loadout.Character,
		"profile_id":           opts.Loadout.ProfileID,
		"revision":             opts.Loadout.Revision,
		"bindings_complete":    opts.Loadout.BindingsComplete,
		"inventory_configured": opts.Loadout.InventoryConfigured,
		"inventory_grid":       opts.Loadout.InventoryGrid,
		"skill_bindings":       bindings,
		"belt_bindings":        opts.Loadout.Bindings.belt,
	}
	return contract
}
