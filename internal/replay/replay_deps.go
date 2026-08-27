package replay

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/input"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/pathing"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/profile"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/tasks"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/telemetry"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/town"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

type replayDependencies struct {
	frame   Frame
	index   int
	fault   error
	current time.Time
}

func (d *replayDependencies) beginFrame(frame Frame) {
	d.frame = frame
	d.index = 0
	d.fault = nil
	d.current = time.Unix(0, 0).Add(time.Duration(frame.ElapsedNS))
}

func (d *replayDependencies) consume(name string) DependencyCall {
	if d.fault != nil {
		return DependencyCall{}
	}
	if d.index >= len(d.frame.Dependencies) {
		d.fault = fmt.Errorf("unexpected additional call %q", name)
		return DependencyCall{}
	}
	call := d.frame.Dependencies[d.index]
	if call.Name != name {
		d.fault = fmt.Errorf("call %d expected %q, got %q", d.index+1, call.Name, name)
		return DependencyCall{}
	}
	d.index++
	return call
}

func (d *replayDependencies) endFrame() error {
	if d.fault != nil {
		return d.fault
	}
	if d.index != len(d.frame.Dependencies) {
		return fmt.Errorf("missing call %d %q", d.index+1, d.frame.Dependencies[d.index].Name)
	}
	return nil
}

func (d *replayDependencies) expectedCall() string {
	if d.index < len(d.frame.Dependencies) {
		return d.frame.Dependencies[d.index].Name
	}
	return "no further call"
}

func (d *replayDependencies) Status() input.Status {
	call := d.consume("input.status")
	return input.Status{Enabled: boolValue(call.Result, "enabled"), Paused: boolValue(call.Result, "paused"), Stopped: boolValue(call.Result, "stopped")}
}
func (d *replayDependencies) Bound() bool {
	return boolValue(d.consume("input.bound").Result, "bound")
}
func (d *replayDependencies) Window() (input.WindowInfo, bool) {
	call := d.consume("input.window")
	return input.WindowInfo{ClientLeft: int(int64Value(call.Result, "client_left")), ClientTop: int(int64Value(call.Result, "client_top")), ClientWidth: int(int64Value(call.Result, "client_width")), ClientHeight: int(int64Value(call.Result, "client_height"))}, boolValue(call.Result, "available")
}

func (d *replayDependencies) Ready() bool {
	return boolValue(d.consume("pathing.ready").Result, "ready")
}
func (d *replayDependencies) Active() bool {
	return boolValue(d.consume("pathing.active").Result, "active")
}
func (d *replayDependencies) Start(pathing.Goal) error {
	return replayCallError(d.consume("pathing.start"))
}
func (d *replayDependencies) TickNavigator(ctx context.Context, state world.State) pathing.NavTickResult {
	call := d.consume("pathing.tick")
	return pathing.NavTickResult{Status: pathing.NavStatus(stringValue(call.Result, "status")), Reason: stringValue(call.Result, "reason"), Done: boolValue(call.Result, "done"), MovementInputSent: boolValue(call.Result, "movement_input_sent"), NextMovementInputAt: offsetTime(state.At, int64Value(call.Result, "next_movement_input_offset_ns")), MovementOutcomeAt: offsetTime(state.At, int64Value(call.Result, "movement_outcome_offset_ns")), MovementProgressTiles: float64Value(call.Result, "movement_progress_tiles")}
}

// Go cannot implement both Navigator.Tick and RoutePlayback.Tick on one type
// because their return signatures differ, so narrow adapters share one transcript.
type replayNavigator struct{ deps *replayDependencies }
type replayRoute struct{ deps *replayDependencies }
type replayTownEgress struct{ deps *replayDependencies }

func (n replayNavigator) Ready() bool                   { return n.deps.Ready() }
func (n replayNavigator) Start(goal pathing.Goal) error { return n.deps.Start(goal) }
func (n replayNavigator) Tick(ctx context.Context, state world.State) pathing.NavTickResult {
	return n.deps.TickNavigator(ctx, state)
}
func (n replayNavigator) Active() bool { return n.deps.Active() }
func (n replayNavigator) Reset()       {}

func (r replayRoute) Start(routeID string, state world.State) error {
	return replayCallError(r.deps.consume("route.start"))
}
func (r replayRoute) Progress(state world.State) (tasks.RouteProgress, bool) {
	call := r.deps.consume("route.progress")
	result := tasks.RouteProgress{RouteID: stringValue(call.Result, "route_id"), RouteRole: pathing.RouteRole(stringValue(call.Result, "route_role")), SegmentID: stringValue(call.Result, "segment_id"), SegmentIndex: int(int64Value(call.Result, "segment_index")), PointIndex: int(int64Value(call.Result, "point_index")), PreviousConfirmed: mapPosition(call.Result, "previous_confirmed"), MovementTarget: mapPosition(call.Result, "movement_target"), TargetAvailable: boolValue(call.Result, "target_available"), Mode: tasks.RouteProgressMode(stringValue(call.Result, "mode")), DriftTiles: float64Value(call.Result, "drift_tiles"), LocalRecoveryAttempts: int(int64Value(call.Result, "local_recovery_attempts")), RecoveryInputSent: boolValue(call.Result, "recovery_input_sent"), RecoveryInputAt: offsetTime(state.At, int64Value(call.Result, "recovery_input_offset_ns")), RecoveryInputOrigin: mapPosition(call.Result, "recovery_input_origin"), RecoveryNextInputAt: offsetTime(state.At, int64Value(call.Result, "recovery_next_input_offset_ns")), RecoveryOutcomeAt: offsetTime(state.At, int64Value(call.Result, "recovery_outcome_offset_ns")), RecoveryProgressTiles: float64Value(call.Result, "recovery_progress_tiles")}
	return result, boolValue(call.Result, "available")
}
func (r replayRoute) Hold(world.State) error { return replayCallError(r.deps.consume("route.hold")) }
func (r replayRoute) Tick(context.Context, world.State) (bool, error) {
	call := r.deps.consume("route.tick")
	return boolValue(call.Result, "done"), replayCallError(call)
}
func (r replayRoute) Reset() {}

func (e replayTownEgress) Start(town.OriginAct, world.State) error {
	return replayCallError(e.deps.consume("town_egress.start"))
}
func (e replayTownEgress) Tick(context.Context, world.State) (bool, error) {
	call := e.deps.consume("town_egress.tick")
	return boolValue(call.Result, "done"), replayCallError(call)
}
func (e replayTownEgress) Reset() {}

func (d *replayDependencies) taskDeps(names []string) (tasks.Deps, error) {
	var deps tasks.Deps
	if len(names) == 0 {
		return deps, fmt.Errorf("runtime replay contract has no dependency presence snapshot")
	}
	for _, name := range names {
		switch name {
		case "input":
			deps.Input = d
		case "pathing":
			deps.Pathing = replayNavigator{d}
		case "waypoint":
			deps.Waypoint = d
		case "portal":
			deps.Portal = replayPortal{d}
		case "town_walk":
			deps.TownWalk = d
		case "stash":
			deps.Stash = replayStash{d}
		case "combat":
			deps.Combat = d
		case "actions":
			deps.Actions = d
		case "loot":
			deps.Loot = d
		case "route":
			deps.Route = replayRoute{d}
		case "route_clear":
			deps.RouteClear = d
		case "town_egress":
			deps.TownEgress = replayTownEgress{d}
		case "profile":
			deps.Profile = d
		case "town":
			deps.Town = replayTown{d}
		case "cow":
			deps.Cow = d
		case "cow_recipe":
			deps.CowRecipe = replayCowRecipe{d}
		case "chest":
			deps.Chest = replayChest{d}
		case "telemetry":
			deps.Telemetry = d
		default:
			return tasks.Deps{}, fmt.Errorf("runtime replay contract contains unknown dependency %q", name)
		}
	}
	return deps, nil
}

func (d *replayDependencies) Reset() {}
func (d *replayDependencies) TickTownWaypoint(context.Context, world.State) pathing.WaypointActionResult {
	return decodeWaypoint(d.consume("waypoint.tick_town"))
}
func (d *replayDependencies) SelectWaypointTarget(context.Context, world.State, pathing.WaypointTargetID, time.Time) pathing.WaypointActionResult {
	return decodeWaypoint(d.consume("waypoint.select"))
}
func decodeWaypoint(call DependencyCall) pathing.WaypointActionResult {
	return pathing.WaypointActionResult{Status: pathing.WaypointActionStatus(stringValue(call.Result, "status")), Reason: stringValue(call.Result, "reason"), Done: boolValue(call.Result, "done")}
}
func (d *replayDependencies) TickPortal(context.Context, world.State, time.Time) pathing.TownPortalActionResult {
	call := d.consume("portal.tick")
	return pathing.TownPortalActionResult{
		Status: pathing.TownPortalActionStatus(stringValue(call.Result, "status")),
		Reason: stringValue(call.Result, "reason"), Done: boolValue(call.Result, "done"),
		PortalUnitID: uint32(int64Value(call.Result, "portal_unit_id")), BlockerUnitID: uint32(int64Value(call.Result, "blocker_unit_id")),
	}
}

type replayPortal struct{ deps *replayDependencies }

func (p replayPortal) Reset() {}
func (p replayPortal) Tick(ctx context.Context, state world.State, now time.Time) pathing.TownPortalActionResult {
	return p.deps.TickPortal(ctx, state, now)
}

func (d *replayDependencies) TickAct1Waypoint(context.Context, world.State) pathing.TownWalkResult {
	call := d.consume("town_walk.tick")
	return pathing.TownWalkResult{Status: pathing.TownWalkStatus(stringValue(call.Result, "status")), Reason: stringValue(call.Result, "reason"), Done: boolValue(call.Result, "done")}
}
func (d *replayDependencies) TickStashOpen(context.Context, world.State) pathing.PersonalStashResult {
	call := d.consume("stash.tick")
	return pathing.PersonalStashResult{Status: pathing.PersonalStashStatus(stringValue(call.Result, "status")), Reason: stringValue(call.Result, "reason"), Done: boolValue(call.Result, "done")}
}

type replayStash struct{ deps *replayDependencies }

func (s replayStash) Reset() {}
func (s replayStash) Tick(ctx context.Context, state world.State) pathing.PersonalStashResult {
	return s.deps.TickStashOpen(ctx, state)
}

func (d *replayDependencies) CastAttackAtWorld(time.Time, uint16, world.Player, world.Position) (bool, error) {
	call := d.consume("combat.cast_world")
	return boolValue(call.Result, "sent"), replayCallError(call)
}
func (d *replayDependencies) HoldStandardAttack(time.Time, uint16, world.Player, world.Monster) (profile.MonsterCastResult, error) {
	call := d.consume("combat.hold_standard_attack")
	return profile.MonsterCastResult{Sent: boolValue(call.Result, "sent"), TargetingMode: profile.MonsterTargetingMode(stringValue(call.Result, "targeting_mode")), AimRequested: boolValue(call.Result, "aim_requested")}, replayCallError(call)
}
func (d *replayDependencies) CastAttackAtMonster(time.Time, uint16, world.Player, world.Monster) (profile.MonsterCastResult, error) {
	call := d.consume("combat.cast_monster")
	return profile.MonsterCastResult{Sent: boolValue(call.Result, "sent"), TargetingMode: profile.MonsterTargetingMode(stringValue(call.Result, "targeting_mode")), AimRequested: boolValue(call.Result, "aim_requested")}, replayCallError(call)
}
func (d *replayDependencies) MonsterAimProjectable(world.Position, world.Position) bool {
	return boolValue(d.consume("combat.monster_aim_projectable").Result, "projectable")
}
func (d *replayDependencies) FarthestProjectableMonsterApproach(world.Position, world.Position) (world.Position, float64, bool) {
	call := d.consume("combat.farthest_projectable_approach")
	return mapPosition(call.Result, "approach"), float64Value(call.Result, "distance"), boolValue(call.Result, "available")
}
func (d *replayDependencies) StopAttack() error {
	return replayCallError(d.consume("combat.stop"))
}
func (d *replayDependencies) TeleportToward(time.Time, world.Player, world.Position, float64) (bool, error) {
	call := d.consume("combat.teleport")
	return boolValue(call.Result, "sent"), replayCallError(call)
}
func (d *replayDependencies) ForceMoveToward(time.Time, world.Position, world.Position) (bool, error) {
	call := d.consume("combat.force_move")
	return boolValue(call.Result, "sent"), replayCallError(call)
}

func (d *replayDependencies) CastBelt(int) error {
	return replayCallError(d.consume("actions.cast_belt"))
}
func (d *replayDependencies) CastTownPortal(time.Time, world.State) error {
	return replayCallError(d.consume("actions.cast_town_portal"))
}

func (d *replayDependencies) Scan(world.State) tasks.LootScanResult {
	return decodeLootScan(d.consume("loot.scan"))
}
func (d *replayDependencies) ScanRouteKeep(world.State, float64) tasks.LootScanResult {
	return decodeLootScan(d.consume("loot.scan_route_keep"))
}
func decodeLootScan(call DependencyCall) tasks.LootScanResult {
	return tasks.LootScanResult{GroundItemCount: int(int64Value(call.Result, "ground_item_count")), CandidateCount: int(int64Value(call.Result, "candidate_count")), InventoryFullCandidateCount: int(int64Value(call.Result, "inventory_full_candidate_count")), InventoryFull: boolValue(call.Result, "inventory_full"), NextTarget: decodeLootTarget(mapValue(call.Result, "target")), HasTarget: boolValue(call.Result, "has_target"), TelemetryFailed: boolValue(call.Result, "telemetry_failed")}
}
func (d *replayDependencies) StartPickup(tasks.LootTarget) error {
	return replayCallError(d.consume("loot.start_pickup"))
}
func (d *replayDependencies) StartCowLegPickup(tasks.LootTarget) error {
	return replayCallError(d.consume("loot.start_cow_leg_pickup"))
}
func (d *replayDependencies) TickPickup(world.State, time.Time) tasks.LootPickupResult {
	call := d.consume("loot.tick_pickup")
	return tasks.LootPickupResult{Status: tasks.LootPickupStatus(stringValue(call.Result, "status")), Done: boolValue(call.Result, "done"), Attempted: boolValue(call.Result, "attempted"), Target: decodeLootTarget(mapValue(call.Result, "target")), Retry: int(int64Value(call.Result, "retry")), HoverAttempt: int(int64Value(call.Result, "hover_attempt"))}
}
func (d *replayDependencies) ClearSkippedPickup(uint32) {}
func (d *replayDependencies) TickStash(world.State, time.Time) tasks.LootStashResult {
	return decodeLootStash(d.consume("loot.tick_stash"))
}
func (d *replayDependencies) TickCloseStash(world.State, time.Time) tasks.LootStashResult {
	return decodeLootStash(d.consume("loot.tick_close_stash"))
}
func decodeLootStash(call DependencyCall) tasks.LootStashResult {
	return tasks.LootStashResult{Status: tasks.LootStashStatus(stringValue(call.Result, "status")), Done: boolValue(call.Result, "done"), Attempted: boolValue(call.Result, "attempted"), Transferred: boolValue(call.Result, "transferred"), UnitID: uint32(uint64Value(call.Result, "unit_id")), Code: stringValue(call.Result, "code"), Name: stringValue(call.Result, "name"), Attempt: int(int64Value(call.Result, "attempt"))}
}

func (d *replayDependencies) TickRouteClear(context.Context, profile.RouteClearRequest, time.Time) profile.Result {
	return decodeProfileResult(d.consume("route_clear.tick"))
}
func (d *replayDependencies) TickAuthorizedCorpseExplosion(context.Context, world.State, uint32, time.Time) profile.Result {
	return decodeProfileResult(d.consume("route_clear.corpse_explosion"))
}
func (d *replayDependencies) ResetRouteClear() {}

func (d *replayDependencies) TickHook(context.Context, profile.Hook, world.State, profile.EncounterTarget, time.Time) profile.Result {
	return decodeProfileResult(d.consume("profile.tick_hook"))
}
func (d *replayDependencies) TickResources(world.State, profile.ResourceContext, time.Time) profile.Result {
	return decodeProfileResult(d.consume("profile.tick_resources"))
}
func (d *replayDependencies) TickRouteMaintenance(world.State, time.Time) profile.Result {
	return decodeProfileResult(d.consume("profile.tick_route_maintenance"))
}
func decodeProfileResult(call DependencyCall) profile.Result {
	return profile.Result{Status: profile.Status(stringValue(call.Result, "status")), Reason: stringValue(call.Result, "reason"), Hook: profile.Hook(stringValue(call.Result, "hook")), Resource: profile.ResourceKind(stringValue(call.Result, "resource")), SkillID: uint16(uint64Value(call.Result, "skill_id")), BeltSlot: int(int64Value(call.Result, "belt_slot")), ActionKind: profile.RouteClearActionKind(stringValue(call.Result, "action_kind")), TargetingMode: profile.MonsterTargetingMode(stringValue(call.Result, "targeting_mode")), TargetUnitID: uint32(uint64Value(call.Result, "target_unit_id")), TargetNPCID: uint32(uint64Value(call.Result, "target_npc_id")), CowGroupAnchorUnitID: uint32(uint64Value(call.Result, "cow_group_anchor_unit_id")), CowGroupLivingCount: int(int64Value(call.Result, "cow_group_living_count")), CowCorpseAnchorDistanceTiles: float64Value(call.Result, "cow_corpse_anchor_distance_tiles"), CowCorpseCoverageCount: int(int64Value(call.Result, "cow_corpse_coverage_count"))}
}

func (d *replayDependencies) TickTown(context.Context, world.State) tasks.TownPreparationResult {
	call := d.consume("town.tick")
	return tasks.TownPreparationResult{Status: stringValue(call.Result, "status"), Reason: stringValue(call.Result, "reason"), Done: boolValue(call.Result, "done")}
}

type replayTown struct{ deps *replayDependencies }

func (t replayTown) Tick(ctx context.Context, state world.State) tasks.TownPreparationResult {
	return t.deps.TickTown(ctx, state)
}
func (t replayTown) Reset() {}

func (d *replayDependencies) TickWirt(context.Context, world.State) tasks.CowSetupActionResult {
	return decodeCowResult(d.consume("cow.tick_wirt"))
}
func (d *replayDependencies) TickTome(context.Context, world.State) tasks.CowSetupActionResult {
	return decodeCowResult(d.consume("cow.tick_tome"))
}
func (d *replayDependencies) TickCowRecipe(world.State, time.Time, uint32, uint32, uint32) tasks.CowSetupActionResult {
	return decodeCowResult(d.consume("cow_recipe.tick"))
}

type replayCowRecipe struct{ deps *replayDependencies }

func (c replayCowRecipe) Tick(state world.State, now time.Time, leg, tome, cube uint32) tasks.CowSetupActionResult {
	return c.deps.TickCowRecipe(state, now, leg, tome, cube)
}
func (c replayCowRecipe) Reset() {}
func decodeCowResult(call DependencyCall) tasks.CowSetupActionResult {
	return tasks.CowSetupActionResult{Done: boolValue(call.Result, "done"), Reason: stringValue(call.Result, "reason"), UnitID: uint32(uint64Value(call.Result, "unit_id")), ProgressKind: stringValue(call.Result, "progress_kind")}
}

type replayChest struct{ deps *replayDependencies }

func (c replayChest) Tick(state world.State, target world.Object, maxDistance float64) tasks.ChestOperateResult {
	return c.deps.TickChest(state, target, maxDistance)
}
func (c replayChest) Reset() {}

func (d *replayDependencies) TickChest(world.State, world.Object, float64) tasks.ChestOperateResult {
	call := d.consume("chest.tick")
	return tasks.ChestOperateResult{
		Status: tasks.ChestOperateStatus(stringValue(call.Result, "status")), Done: boolValue(call.Result, "done"),
		Reason: stringValue(call.Result, "reason"), Attempt: int(uint64Value(call.Result, "attempt")),
		BlockerUnitID: uint32(uint64Value(call.Result, "blocker_unit_id")),
	}
}

func (d *replayDependencies) Emit(telemetry.Event) error {
	return replayCallError(d.consume("telemetry.emit"))
}

func mapValue(values map[string]any, key string) map[string]any {
	value, _ := valueAt(values, key).(map[string]any)
	return value
}
func mapPosition(values map[string]any, key string) world.Position {
	value := mapValue(values, key)
	return world.Position{X: uint32(uint64Value(value, "x")), Y: uint32(uint64Value(value, "y"))}
}
func decodeLootTarget(values map[string]any) tasks.LootTarget {
	return tasks.LootTarget{UnitID: uint32(uint64Value(values, "unit_id")), TxtFileNo: uint32(uint64Value(values, "txt_file_no")), Code: stringValue(values, "code"), Name: stringValue(values, "name"), Quality: world.ItemQuality(uint64Value(values, "quality")), IdentityKind: world.ItemIdentityKind(stringValue(values, "identity_kind")), IdentityKey: stringValue(values, "identity_key"), IdentityValid: boolValue(values, "identity_valid"), PickitProfileID: stringValue(values, "pickit_profile_id"), PickitRuleID: stringValue(values, "pickit_rule_id"), PickitAction: stringValue(values, "pickit_action"), PickitProfileRevision: uint64Value(values, "pickit_profile_revision"), PickitAssignmentRevision: uint64Value(values, "pickit_assignment_revision"), Position: mapPosition(values, "position"), AreaID: world.AreaID(uint64Value(values, "area_id"))}
}
func offsetTime(base time.Time, offset int64) time.Time {
	if offset == 0 {
		return time.Time{}
	}
	return base.Add(time.Duration(offset))
}

type replayError struct {
	text  string
	cause error
}

func (e replayError) Error() string { return e.text }
func (e replayError) Unwrap() error { return e.cause }

func replayCallError(call DependencyCall) error {
	if call.Error == "" {
		return nil
	}
	known := []error{
		profile.ErrSkillSelectionPending, profile.ErrRouteClearTargetUnprojectable, profile.ErrCorpseExplosionTargetUnprojectable,
		pathing.ErrRouteCharacterMismatch, pathing.ErrRouteGameVersionMismatch, pathing.ErrRouteLayoutUnverified, pathing.ErrRouteLayoutMismatch,
		pathing.ErrRouteStartMismatch, pathing.ErrRouteNotFound, pathing.ErrRouteHardStuck, pathing.ErrRouteDriftExceeded,
		pathing.ErrRouteTransitionFailed, pathing.ErrRouteSegmentTimeout, pathing.ErrRouteUnexpectedArea, pathing.ErrNavigatorNotWired,
		pathing.ErrGameIdentityUnavailable,
	}
	for _, candidate := range known {
		if strings.Contains(call.Error, candidate.Error()) {
			return replayError{text: call.Error, cause: candidate}
		}
	}
	return errors.New(call.Error)
}
