package replay

import (
	"context"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/input"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/pathing"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/profile"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/tasks"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/telemetry"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/town"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

// InstrumentDeps decorates task dependencies with passive result observation.
// Calls and return values are forwarded unchanged; the wrappers never retry,
// suppress, reorder, or originate gameplay actions.
func InstrumentDeps(deps tasks.Deps, recorder *Recorder) tasks.Deps {
	if recorder == nil || !recorder.Enabled() {
		return deps
	}
	if deps.Input != nil {
		deps.Input = &traceInput{next: deps.Input, recorder: recorder}
	}
	if deps.Pathing != nil {
		deps.Pathing = &traceNavigator{next: deps.Pathing, recorder: recorder}
	}
	if deps.Waypoint != nil {
		deps.Waypoint = &traceWaypoint{next: deps.Waypoint, recorder: recorder}
	}
	if deps.Portal != nil {
		deps.Portal = &tracePortal{next: deps.Portal, recorder: recorder}
	}
	if deps.TownWalk != nil {
		deps.TownWalk = &traceTownWalk{next: deps.TownWalk, recorder: recorder}
	}
	if deps.Stash != nil {
		deps.Stash = &traceStash{next: deps.Stash, recorder: recorder}
	}
	if deps.Combat != nil {
		deps.Combat = &traceCombat{next: deps.Combat, recorder: recorder}
	}
	if deps.Actions != nil {
		deps.Actions = &traceRunActions{next: deps.Actions, recorder: recorder}
	}
	if deps.Loot != nil {
		deps.Loot = &traceLoot{next: deps.Loot, recorder: recorder}
	}
	if deps.Route != nil {
		deps.Route = &traceRoute{next: deps.Route, recorder: recorder}
	}
	if deps.RouteClear != nil {
		base := &traceRouteClear{next: deps.RouteClear, recorder: recorder}
		if cow, ok := deps.RouteClear.(tasks.CowRouteClearExecutor); ok {
			deps.RouteClear = &traceCowRouteClear{traceRouteClear: base, cow: cow}
		} else {
			deps.RouteClear = base
		}
	}
	if deps.TownEgress != nil {
		deps.TownEgress = &traceTownEgress{next: deps.TownEgress, recorder: recorder}
	}
	if deps.Profile != nil {
		deps.Profile = &traceProfile{next: deps.Profile, recorder: recorder}
	}
	if deps.Town != nil {
		deps.Town = &traceTown{next: deps.Town, recorder: recorder}
	}
	if deps.Cow != nil {
		deps.Cow = &traceCow{next: deps.Cow, recorder: recorder}
	}
	if deps.CowRecipe != nil {
		deps.CowRecipe = &traceCowRecipe{next: deps.CowRecipe, recorder: recorder}
	}
	if deps.Telemetry != nil {
		deps.Telemetry = &traceTelemetry{next: deps.Telemetry, recorder: recorder}
	}
	return deps
}

type traceInput struct {
	next     tasks.Input
	recorder *Recorder
}

func (t *traceInput) Status() input.Status {
	result := t.next.Status()
	recordResult(t.recorder, "input.status", nil, map[string]any{"enabled": result.Enabled, "paused": result.Paused, "stopped": result.Stopped}, nil)
	return result
}
func (t *traceInput) Bound() bool {
	result := t.next.Bound()
	recordResult(t.recorder, "input.bound", nil, map[string]any{"bound": result}, nil)
	return result
}
func (t *traceInput) Window() (input.WindowInfo, bool) {
	window, ok := t.next.Window()
	// PID, title and HWND are deliberately excluded; only task-visible client
	// geometry participates in replay decisions.
	recordResult(t.recorder, "input.window", nil, map[string]any{"available": ok, "client_left": window.ClientLeft, "client_top": window.ClientTop, "client_width": window.ClientWidth, "client_height": window.ClientHeight}, nil)
	return window, ok
}

func position(value world.Position) map[string]any { return map[string]any{"x": value.X, "y": value.Y} }

func recordResult(recorder *Recorder, name string, args, result map[string]any, err error) {
	recorder.RecordDependency(name, args, result, err)
}

func recordIntent(recorder *Recorder, name string, params map[string]any, sent bool, err error) {
	if !sent && err == nil {
		return
	}
	outcome := "sent"
	if err != nil {
		outcome = "failed"
	}
	recorder.RecordIntent(name, params, outcome)
}

type traceNavigator struct {
	next     tasks.Navigator
	recorder *Recorder
}

func (t *traceNavigator) Ready() bool {
	result := t.next.Ready()
	recordResult(t.recorder, "pathing.ready", nil, map[string]any{"ready": result}, nil)
	return result
}
func (t *traceNavigator) Active() bool {
	result := t.next.Active()
	recordResult(t.recorder, "pathing.active", nil, map[string]any{"active": result}, nil)
	return result
}
func (t *traceNavigator) Reset() { t.next.Reset() }
func (t *traceNavigator) Start(goal pathing.Goal) error {
	args := map[string]any{"kind": goal.Kind.String(), "target_area_id": uint32(goal.TargetArea), "target": position(goal.TargetPos), "arrival_distance": goal.ArrivalDistance, "via_entrance": goal.ViaEntrance.String(), "via_entrance_unit_id": goal.ViaEntranceUnitID, "strict_entrance": goal.StrictEntrance, "via_object": goal.ViaObject.String(), "via_object_unit_id": goal.ViaObjectUnitID, "strict_object": goal.StrictObject}
	err := t.next.Start(goal)
	recordResult(t.recorder, "pathing.start", args, nil, err)
	return err
}
func (t *traceNavigator) Tick(ctx context.Context, state world.State) pathing.NavTickResult {
	result := t.next.Tick(ctx, state)
	recordResult(t.recorder, "pathing.tick", nil, map[string]any{"status": string(result.Status), "reason": result.Reason, "done": result.Done, "movement_input_sent": result.MovementInputSent, "next_movement_input_offset_ns": timeOffsetNS(state.At, result.NextMovementInputAt), "movement_outcome_offset_ns": timeOffsetNS(state.At, result.MovementOutcomeAt), "movement_progress_tiles": result.MovementProgressTiles}, nil)
	recordIntent(t.recorder, "navigate", nil, result.MovementInputSent, nil)
	return result
}

type traceWaypoint struct {
	next     tasks.WaypointActions
	recorder *Recorder
}

func (t *traceWaypoint) Reset() { t.next.Reset() }
func (t *traceWaypoint) TickTownWaypoint(ctx context.Context, state world.State) pathing.WaypointActionResult {
	result := t.next.TickTownWaypoint(ctx, state)
	recordResult(t.recorder, "waypoint.tick_town", nil, waypointResult(result), nil)
	recordIntent(t.recorder, "waypoint_object_click", nil, result.Status == pathing.WaypointActionClicked, nil)
	return result
}
func (t *traceWaypoint) SelectWaypointTarget(ctx context.Context, state world.State, target pathing.WaypointTargetID, now time.Time) pathing.WaypointActionResult {
	result := t.next.SelectWaypointTarget(ctx, state, target, now)
	recordResult(t.recorder, "waypoint.select", map[string]any{"target": string(target)}, waypointResult(result), nil)
	recordIntent(t.recorder, "waypoint_select", map[string]any{"target": string(target)}, result.Status == pathing.WaypointActionClicked, nil)
	return result
}
func waypointResult(result pathing.WaypointActionResult) map[string]any {
	return map[string]any{"status": string(result.Status), "reason": result.Reason, "done": result.Done}
}

type tracePortal struct {
	next     tasks.TownPortalActions
	recorder *Recorder
}

func (t *tracePortal) Reset() { t.next.Reset() }
func (t *tracePortal) Tick(ctx context.Context, state world.State, now time.Time) pathing.TownPortalActionResult {
	result := t.next.Tick(ctx, state, now)
	recordResult(t.recorder, "portal.tick", nil, map[string]any{"status": string(result.Status), "reason": result.Reason, "done": result.Done}, nil)
	recordIntent(t.recorder, "town_portal_enter", nil, result.Status == pathing.TownPortalActionClicked, nil)
	return result
}

type traceTownWalk struct {
	next     tasks.TownWalker
	recorder *Recorder
}

func (t *traceTownWalk) Reset() { t.next.Reset() }
func (t *traceTownWalk) TickAct1Waypoint(ctx context.Context, state world.State) pathing.TownWalkResult {
	result := t.next.TickAct1Waypoint(ctx, state)
	recordResult(t.recorder, "town_walk.tick", nil, map[string]any{"status": string(result.Status), "reason": result.Reason, "done": result.Done}, nil)
	recordIntent(t.recorder, "town_walk", nil, result.Status == pathing.TownWalkPending, nil)
	return result
}

type traceStash struct {
	next     tasks.PersonalStashActions
	recorder *Recorder
}

func (t *traceStash) Reset() { t.next.Reset() }
func (t *traceStash) Tick(ctx context.Context, state world.State) pathing.PersonalStashResult {
	result := t.next.Tick(ctx, state)
	recordResult(t.recorder, "stash.tick", nil, map[string]any{"status": string(result.Status), "reason": result.Reason, "done": result.Done}, nil)
	recordIntent(t.recorder, "stash_interaction", nil, result.Status == pathing.PersonalStashPending || result.Status == pathing.PersonalStashOpened, nil)
	return result
}

type traceCombat struct {
	next     tasks.CombatActions
	recorder *Recorder
}

func (t *traceCombat) CastAttackAtWorld(now time.Time, skillID uint16, player world.Player, target world.Position) (bool, error) {
	sent, err := t.next.CastAttackAtWorld(now, skillID, player, target)
	args := map[string]any{"skill_id": skillID, "target": position(target)}
	recordResult(t.recorder, "combat.cast_world", args, map[string]any{"sent": sent}, err)
	recordIntent(t.recorder, "combat_cast_world", args, sent, err)
	return sent, err
}
func (t *traceCombat) HoldStandardAttack(now time.Time, skillID uint16, player world.Player, target world.Monster) (profile.MonsterCastResult, error) {
	result, err := t.next.HoldStandardAttack(now, skillID, player, target)
	args := map[string]any{"skill_id": skillID, "target_unit_id": target.UnitID, "target_npc_id": target.NPCID, "target": position(target.Position)}
	recordResult(t.recorder, "combat.hold_standard_attack", args, map[string]any{"sent": result.Sent, "targeting_mode": string(result.TargetingMode), "aim_requested": result.AimRequested}, err)
	recordIntent(t.recorder, "combat_hold_standard_attack", args, result.Sent || result.AimRequested, err)
	return result, err
}
func (t *traceCombat) CastAttackAtMonster(now time.Time, skillID uint16, player world.Player, target world.Monster) (profile.MonsterCastResult, error) {
	result, err := t.next.CastAttackAtMonster(now, skillID, player, target)
	args := map[string]any{"skill_id": skillID, "target_unit_id": target.UnitID, "target_npc_id": target.NPCID, "target": position(target.Position)}
	recordResult(t.recorder, "combat.cast_monster", args, map[string]any{"sent": result.Sent, "targeting_mode": string(result.TargetingMode), "aim_requested": result.AimRequested}, err)
	recordIntent(t.recorder, "combat_cast_monster", args, result.Sent || result.AimRequested, err)
	return result, err
}
func (t *traceCombat) MonsterAimProjectable(playerPos, targetPos world.Position) bool {
	result := t.next.MonsterAimProjectable(playerPos, targetPos)
	recordResult(t.recorder, "combat.monster_aim_projectable", map[string]any{"player": position(playerPos), "target": position(targetPos)}, map[string]any{"projectable": result}, nil)
	return result
}
func (t *traceCombat) FarthestProjectableMonsterApproach(playerPos, targetPos world.Position) (world.Position, float64, bool) {
	approach, distance, ok := t.next.FarthestProjectableMonsterApproach(playerPos, targetPos)
	recordResult(t.recorder, "combat.farthest_projectable_approach", map[string]any{"player": position(playerPos), "target": position(targetPos)}, map[string]any{"approach": position(approach), "distance": distance, "available": ok}, nil)
	return approach, distance, ok
}
func (t *traceCombat) StopAttack() error {
	err := t.next.StopAttack()
	recordResult(t.recorder, "combat.stop", nil, nil, err)
	recordIntent(t.recorder, "combat_stop", nil, err == nil, err)
	return err
}
func (t *traceCombat) TeleportToward(now time.Time, player world.Player, target world.Position, distance float64) (bool, error) {
	sent, err := t.next.TeleportToward(now, player, target, distance)
	args := map[string]any{"target": position(target), "desired_distance_tiles": distance}
	recordResult(t.recorder, "combat.teleport", args, map[string]any{"sent": sent}, err)
	recordIntent(t.recorder, "teleport", args, sent, err)
	return sent, err
}
func (t *traceCombat) ForceMoveToward(now time.Time, playerPos, target world.Position) (bool, error) {
	sent, err := t.next.ForceMoveToward(now, playerPos, target)
	args := map[string]any{"target": position(target)}
	recordResult(t.recorder, "combat.force_move", args, map[string]any{"sent": sent}, err)
	recordIntent(t.recorder, "force_move", args, sent, err)
	return sent, err
}
func (t *traceCombat) Reset() { t.next.Reset() }

type traceRunActions struct {
	next     tasks.RunActions
	recorder *Recorder
}

func (t *traceRunActions) CastBelt(slot int) error {
	err := t.next.CastBelt(slot)
	args := map[string]any{"slot": slot}
	recordResult(t.recorder, "actions.cast_belt", args, nil, err)
	recordIntent(t.recorder, "belt_cast", args, err == nil, err)
	return err
}
func (t *traceRunActions) CastTownPortal(now time.Time, player world.Player) error {
	err := t.next.CastTownPortal(now, player)
	recordResult(t.recorder, "actions.cast_town_portal", nil, nil, err)
	recordIntent(t.recorder, "town_portal_cast", nil, err == nil, err)
	return err
}

type traceLoot struct {
	next     tasks.LootActions
	recorder *Recorder
}

func (t *traceLoot) Scan(state world.State) tasks.LootScanResult {
	result := t.next.Scan(state)
	recordResult(t.recorder, "loot.scan", nil, lootScanResult(result), nil)
	return result
}
func (t *traceLoot) ScanRouteKeep(state world.State, distance float64) tasks.LootScanResult {
	result := t.next.ScanRouteKeep(state, distance)
	recordResult(t.recorder, "loot.scan_route_keep", map[string]any{"maximum_distance_tiles": distance}, lootScanResult(result), nil)
	return result
}
func lootScanResult(result tasks.LootScanResult) map[string]any {
	return map[string]any{"ground_item_count": result.GroundItemCount, "candidate_count": result.CandidateCount, "inventory_full_candidate_count": result.InventoryFullCandidateCount, "inventory_full": result.InventoryFull, "has_target": result.HasTarget, "target": lootTargetResult(result.NextTarget), "telemetry_failed": result.TelemetryFailed}
}
func lootTargetResult(target tasks.LootTarget) map[string]any {
	return map[string]any{"unit_id": target.UnitID, "txt_file_no": target.TxtFileNo, "code": target.Code, "name": target.Name, "quality": uint32(target.Quality), "identity_kind": string(target.IdentityKind), "identity_key": target.IdentityKey, "identity_valid": target.IdentityValid, "pickit_profile_id": target.PickitProfileID, "pickit_rule_id": target.PickitRuleID, "pickit_action": target.PickitAction, "pickit_profile_revision": target.PickitProfileRevision, "pickit_assignment_revision": target.PickitAssignmentRevision, "position": position(target.Position), "area_id": uint32(target.AreaID)}
}
func (t *traceLoot) StartPickup(target tasks.LootTarget) error {
	err := t.next.StartPickup(target)
	recordResult(t.recorder, "loot.start_pickup", map[string]any{"unit_id": target.UnitID, "code": target.Code, "rule_id": target.PickitRuleID}, nil, err)
	return err
}
func (t *traceLoot) StartCowLegPickup(target tasks.LootTarget) error {
	err := t.next.StartCowLegPickup(target)
	recordResult(t.recorder, "loot.start_cow_leg_pickup", map[string]any{"unit_id": target.UnitID}, nil, err)
	return err
}
func (t *traceLoot) TickPickup(state world.State, now time.Time) tasks.LootPickupResult {
	result := t.next.TickPickup(state, now)
	recordResult(t.recorder, "loot.tick_pickup", nil, map[string]any{"status": string(result.Status), "done": result.Done, "attempted": result.Attempted, "target": lootTargetResult(result.Target), "retry": result.Retry, "hover_attempt": result.HoverAttempt}, nil)
	recordIntent(t.recorder, "loot_pickup", map[string]any{"unit_id": result.Target.UnitID}, result.Attempted, nil)
	return result
}
func (t *traceLoot) ClearSkippedPickup(unitID uint32) { t.next.ClearSkippedPickup(unitID) }
func (t *traceLoot) TickStash(state world.State, now time.Time) tasks.LootStashResult {
	result := t.next.TickStash(state, now)
	recordResult(t.recorder, "loot.tick_stash", nil, lootStashResult(result), nil)
	recordIntent(t.recorder, "stash_transfer", map[string]any{"unit_id": result.UnitID}, result.Attempted, nil)
	return result
}
func (t *traceLoot) TickCloseStash(state world.State, now time.Time) tasks.LootStashResult {
	result := t.next.TickCloseStash(state, now)
	recordResult(t.recorder, "loot.tick_close_stash", nil, lootStashResult(result), nil)
	recordIntent(t.recorder, "stash_close", nil, result.Attempted, nil)
	return result
}
func lootStashResult(result tasks.LootStashResult) map[string]any {
	return map[string]any{"status": string(result.Status), "done": result.Done, "attempted": result.Attempted, "transferred": result.Transferred, "unit_id": result.UnitID, "code": result.Code, "name": result.Name, "attempt": result.Attempt}
}
func (t *traceLoot) Reset() { t.next.Reset() }

type traceRoute struct {
	next     tasks.RoutePlayback
	recorder *Recorder
}

func (t *traceRoute) Start(routeID string, state world.State) error {
	err := t.next.Start(routeID, state)
	recordResult(t.recorder, "route.start", map[string]any{"route_id": routeID}, nil, err)
	return err
}
func (t *traceRoute) Progress(state world.State) (tasks.RouteProgress, bool) {
	result, ok := t.next.Progress(state)
	recordResult(t.recorder, "route.progress", nil, map[string]any{"available": ok, "route_id": result.RouteID, "route_role": string(result.RouteRole), "segment_id": result.SegmentID, "segment_index": result.SegmentIndex, "point_index": result.PointIndex, "previous_confirmed": position(result.PreviousConfirmed), "target_available": result.TargetAvailable, "movement_target": position(result.MovementTarget), "mode": string(result.Mode), "drift_tiles": result.DriftTiles, "local_recovery_attempts": result.LocalRecoveryAttempts, "recovery_input_sent": result.RecoveryInputSent, "recovery_input_offset_ns": timeOffsetNS(state.At, result.RecoveryInputAt), "recovery_input_origin": position(result.RecoveryInputOrigin), "recovery_next_input_offset_ns": timeOffsetNS(state.At, result.RecoveryNextInputAt), "recovery_outcome_offset_ns": timeOffsetNS(state.At, result.RecoveryOutcomeAt), "recovery_progress_tiles": result.RecoveryProgressTiles}, nil)
	recordIntent(t.recorder, "route_recovery", map[string]any{"target": position(result.MovementTarget)}, result.RecoveryInputSent, nil)
	return result, ok
}
func (t *traceRoute) Hold(state world.State) error {
	err := t.next.Hold(state)
	recordResult(t.recorder, "route.hold", nil, nil, err)
	return err
}
func (t *traceRoute) Tick(ctx context.Context, state world.State) (bool, error) {
	done, err := t.next.Tick(ctx, state)
	recordResult(t.recorder, "route.tick", nil, map[string]any{"done": done}, err)
	return done, err
}
func (t *traceRoute) Reset() { t.next.Reset() }

type traceRouteClear struct {
	next     tasks.RouteClearExecutor
	recorder *Recorder
}

func (t *traceRouteClear) TickRouteClear(ctx context.Context, request profile.RouteClearRequest, now time.Time) profile.Result {
	result := t.next.TickRouteClear(ctx, request, now)
	recordResult(t.recorder, "route_clear.tick", map[string]any{"run_id": request.RunID, "definition_id": request.DefinitionID, "target_unit_id": request.Target.UnitID, "mode": string(request.Mode)}, profileResult(result), nil)
	recordIntent(t.recorder, "route_clear", map[string]any{"target_unit_id": result.TargetUnitID, "action_kind": string(result.ActionKind)}, result.Status == profile.StatusAction, nil)
	return result
}
func (t *traceRouteClear) ResetRouteClear() { t.next.ResetRouteClear() }

type traceCowRouteClear struct {
	*traceRouteClear
	cow tasks.CowRouteClearExecutor
}

func (t *traceCowRouteClear) TickAuthorizedCorpseExplosion(ctx context.Context, state world.State, unitID uint32, now time.Time) profile.Result {
	result := t.cow.TickAuthorizedCorpseExplosion(ctx, state, unitID, now)
	recordResult(t.recorder, "route_clear.corpse_explosion", map[string]any{"corpse_unit_id": unitID}, profileResult(result), nil)
	recordIntent(t.recorder, "corpse_explosion", map[string]any{"corpse_unit_id": unitID}, result.Status == profile.StatusAction, nil)
	return result
}

type traceTownEgress struct {
	next     tasks.TownEgressPlayback
	recorder *Recorder
}

func (t *traceTownEgress) Start(origin town.OriginAct, state world.State) error {
	err := t.next.Start(origin, state)
	recordResult(t.recorder, "town_egress.start", map[string]any{"origin": string(origin)}, nil, err)
	return err
}
func (t *traceTownEgress) Tick(ctx context.Context, state world.State) (bool, error) {
	done, err := t.next.Tick(ctx, state)
	recordResult(t.recorder, "town_egress.tick", nil, map[string]any{"done": done}, err)
	return done, err
}
func (t *traceTownEgress) Reset() { t.next.Reset() }

type traceProfile struct {
	next     tasks.ProfileActions
	recorder *Recorder
}

func (t *traceProfile) TickHook(ctx context.Context, hook profile.Hook, state world.State, target profile.EncounterTarget, now time.Time) profile.Result {
	result := t.next.TickHook(ctx, hook, state, target, now)
	recordResult(t.recorder, "profile.tick_hook", map[string]any{"hook": string(hook), "target_unit_id": target.UnitID, "action_index": target.ActionIndex}, profileResult(result), nil)
	recordIntent(t.recorder, "profile_hook", map[string]any{"hook": string(hook), "skill_id": result.SkillID}, result.Status == profile.StatusAction, nil)
	return result
}
func (t *traceProfile) TickResources(state world.State, resourceContext profile.ResourceContext, now time.Time) profile.Result {
	result := t.next.TickResources(state, resourceContext, now)
	recordResult(t.recorder, "profile.tick_resources", map[string]any{"mobility_critical": resourceContext.MobilityCritical, "threatened": resourceContext.Threatened, "emergency_mana": resourceContext.EmergencyMana, "allow_mercenary": resourceContext.AllowMercenary, "fail_on_unavailable": resourceContext.FailOnUnavailable}, profileResult(result), nil)
	recordIntent(t.recorder, "profile_resource", map[string]any{"resource": string(result.Resource), "belt_slot": result.BeltSlot}, result.Status == profile.StatusAction, nil)
	return result
}
func (t *traceProfile) TickRouteMaintenance(state world.State, now time.Time) profile.Result {
	result := t.next.TickRouteMaintenance(state, now)
	recordResult(t.recorder, "profile.tick_route_maintenance", nil, profileResult(result), nil)
	recordIntent(t.recorder, "profile_route_maintenance", map[string]any{"skill_id": result.SkillID}, result.Status == profile.StatusAction, nil)
	return result
}
func (t *traceProfile) Reset() { t.next.Reset() }

func profileResult(result profile.Result) map[string]any {
	return map[string]any{"status": string(result.Status), "reason": result.Reason, "hook": string(result.Hook), "resource": string(result.Resource), "skill_id": result.SkillID, "belt_slot": result.BeltSlot, "action_kind": string(result.ActionKind), "targeting_mode": string(result.TargetingMode), "target_unit_id": result.TargetUnitID, "target_npc_id": result.TargetNPCID, "cow_group_anchor_unit_id": result.CowGroupAnchorUnitID, "cow_group_living_count": result.CowGroupLivingCount, "cow_corpse_anchor_distance_tiles": result.CowCorpseAnchorDistanceTiles, "cow_corpse_coverage_count": result.CowCorpseCoverageCount}
}

type traceTown struct {
	next     tasks.TownPreparationActions
	recorder *Recorder
}

func (t *traceTown) Tick(ctx context.Context, state world.State) tasks.TownPreparationResult {
	result := t.next.Tick(ctx, state)
	recordResult(t.recorder, "town.tick", nil, map[string]any{"status": result.Status, "reason": result.Reason, "done": result.Done}, nil)
	return result
}
func (t *traceTown) Reset() { t.next.Reset() }

type traceCow struct {
	next     tasks.CowSetupActions
	recorder *Recorder
}

func (t *traceCow) TickWirt(ctx context.Context, state world.State) tasks.CowSetupActionResult {
	result := t.next.TickWirt(ctx, state)
	recordResult(t.recorder, "cow.tick_wirt", nil, cowResult(result), nil)
	recordIntent(t.recorder, "cow_wirt", map[string]any{"unit_id": result.UnitID}, result.ProgressKind != "", nil)
	return result
}
func (t *traceCow) TickTome(ctx context.Context, state world.State) tasks.CowSetupActionResult {
	result := t.next.TickTome(ctx, state)
	recordResult(t.recorder, "cow.tick_tome", nil, cowResult(result), nil)
	recordIntent(t.recorder, "cow_tome", map[string]any{"unit_id": result.UnitID}, result.ProgressKind != "", nil)
	return result
}
func (t *traceCow) Reset() { t.next.Reset() }

type traceCowRecipe struct {
	next     tasks.CowPortalRecipeActions
	recorder *Recorder
}

func (t *traceCowRecipe) Tick(state world.State, now time.Time, legUnitID, tomeUnitID, cubeUnitID uint32) tasks.CowSetupActionResult {
	result := t.next.Tick(state, now, legUnitID, tomeUnitID, cubeUnitID)
	args := map[string]any{"leg_unit_id": legUnitID, "tome_unit_id": tomeUnitID, "cube_unit_id": cubeUnitID}
	recordResult(t.recorder, "cow_recipe.tick", args, cowResult(result), nil)
	recordIntent(t.recorder, "cow_recipe", args, result.ProgressKind != "", nil)
	return result
}
func (t *traceCowRecipe) Reset() { t.next.Reset() }

func cowResult(result tasks.CowSetupActionResult) map[string]any {
	return map[string]any{"done": result.Done, "reason": result.Reason, "unit_id": result.UnitID, "progress_kind": result.ProgressKind}
}

func timeOffsetNS(base, value time.Time) int64 {
	if value.IsZero() {
		return 0
	}
	return value.Sub(base).Nanoseconds()
}

type traceTelemetry struct {
	next     tasks.RunTelemetry
	recorder *Recorder
}

func (t *traceTelemetry) Emit(event telemetry.Event) error {
	err := t.next.Emit(event)
	recordResult(t.recorder, "telemetry.emit", map[string]any{"event": string(event.Event), "run": event.Run, "definition_id": event.DefinitionID, "phase": event.Phase, "step": event.Step, "reason": event.Reason, "outcome": event.Outcome, "unit_id": event.UnitID, "route_id": event.RouteID}, nil, err)
	return err
}
