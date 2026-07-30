package tasks

import (
	"context"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/input"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/pathing"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/profile"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/telemetry"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/town"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

// Deps holds shared runtime dependencies injected into task runs.
type Deps struct {
	Input      Input
	Pathing    Navigator
	Waypoint   WaypointActions
	Portal     TownPortalActions
	TownWalk   TownWalker
	Stash      PersonalStashActions
	Combat     CombatActions
	Actions    RunActions
	Loot       LootActions
	Route      RoutePlayback
	RouteClear RouteClearExecutor
	TownEgress TownEgressPlayback
	Profile    ProfileActions
	Town       TownPreparationActions
	Telemetry  RunTelemetry
}

// RunTelemetry persists shared pipeline transitions before subsequent input.
type RunTelemetry interface {
	Emit(telemetry.Event) error
}

// TownPreparationActions executes the central post-run preparation handoff.
type TownPreparationActions interface {
	Tick(context.Context, world.State) TownPreparationResult
	Reset()
}

// TownPreparationResult reports the verified central Town endpoint.
type TownPreparationResult struct {
	Status string
	Reason string
	Done   bool
}

// ProfileActions evaluates class-gated hooks and prioritized in-run resources.
type ProfileActions interface {
	TickHook(context.Context, profile.Hook, world.State, profile.EncounterTarget, time.Time) profile.Result
	TickResources(world.State, profile.ResourceContext, time.Time) profile.Result
	Reset()
}

// RouteClearExecutor is the movement-free profile strategy used during route hold.
type RouteClearExecutor interface {
	TickRouteClear(context.Context, profile.RouteClearRequest, time.Time) profile.Result
	ResetRouteClear()
}

// RoutePlayback is the generic full-route surface used by run adapters.
type RoutePlayback interface {
	Start(routeID string, state world.State) error
	Progress(state world.State) (RouteProgress, bool)
	Hold(state world.State) error
	Tick(context.Context, world.State) (bool, error)
	Reset()
}

// TownEgressPlayback replays the configured local route from a foreign portal arrival to its waypoint.
type TownEgressPlayback interface {
	Start(town.OriginAct, world.State) error
	Tick(context.Context, world.State) (bool, error)
	Reset()
}

// Input is the subset of input.Controller used by task runs.
type Input interface {
	Status() input.Status
	Bound() bool
}

// Navigator is the subset of pathing.Navigator used by task runs.
// Start begins a goal, Tick advances it per poll cycle, Active reports whether
// a goal is still in flight, and Reset aborts stale movement between steps.
type Navigator interface {
	Ready() bool
	Start(goal pathing.Goal) error
	Tick(ctx context.Context, state world.State) pathing.NavTickResult
	Active() bool
	Reset()
}

// WaypointActions is the narrow waypoint-action surface used by task runs.
type WaypointActions interface {
	Reset()
	TickTownWaypoint(context.Context, world.State) pathing.WaypointActionResult
	SelectWaypointTarget(context.Context, world.State, pathing.WaypointTargetID, time.Time) pathing.WaypointActionResult
}

// TownPortalActions is the narrow hover-confirmed portal-entry surface used by task runs.
type TownPortalActions interface {
	Reset()
	Tick(context.Context, world.State, time.Time) pathing.TownPortalActionResult
}

// TownWalker is the narrow town-walk surface used by task runs.
type TownWalker interface {
	Reset()
	TickAct1Waypoint(context.Context, world.State) pathing.TownWalkResult
}

// PersonalStashActions is the transfer-free town navigation and stash-open surface.
type PersonalStashActions interface {
	Reset()
	Tick(context.Context, world.State) pathing.PersonalStashResult
}

// CombatActions is the narrow combat-action surface used by task runs.
type CombatActions interface {
	// CastAttackAtWorld verifies the configured mouse-side selection before attacking targetPos.
	// The boolean reports whether this call actually sent the attack click;
	// selection and throttled calls return false.
	CastAttackAtWorld(now time.Time, skillID uint16, player world.Player, targetPos world.Position) (bool, error)
	// CastAttackAtMonster aims at the supplied living monster and sends the
	// attack only after Memory confirms that exact UnitID under the cursor.
	CastAttackAtMonster(now time.Time, skillID uint16, player world.Player, target world.Monster) (bool, error)
	// MonsterAimProjectable reports whether the first visible-body hover probe
	// is inside the currently bound playable client area.
	MonsterAimProjectable(playerPos, targetPos world.Position) bool
	// FarthestProjectableMonsterDistance returns the greatest boss distance
	// reachable along playerPos→targetPos from which the first hover probe is
	// playable. It performs no input and does not claim that the tile is safe.
	FarthestProjectableMonsterDistance(playerPos, targetPos world.Position) (float64, bool)
	// StopAttack releases any stateful attack input before combat stops or repositions.
	StopAttack() error
	// TeleportToward moves toward targetPos while trying to preserve desiredDistanceTiles.
	// The boolean reports whether this call actually sent input; throttled calls return false.
	TeleportToward(now time.Time, playerPos, targetPos world.Position, desiredDistanceTiles float64) (bool, error)
	// ForceMoveToward uses the configured Force-Move key at targetPos and lets
	// D2R pathfind one bounded approach step without consuming Route.Tick.
	ForceMoveToward(now time.Time, playerPos, targetPos world.Position) (bool, error)
	// Reset clears per-step combat throttles.
	Reset()
}

// RunActions exposes high-level input actions needed by run orchestration.
type RunActions interface {
	// CastBelt uses the configured belt hotkey for slot.
	CastBelt(slot int) error
	// CastTownPortal casts Town Portal at the player-centered window position.
	CastTownPortal() error
}

// LootActions exposes the stateful loot pickup loop used by run pipelines.
type LootActions interface {
	// Scan evaluates current loot and returns the next non-skipped target.
	Scan(state world.State) LootScanResult
	// ScanRouteKeep evaluates only `keep` matches within maxDistanceTiles for
	// opportunistic pickup while a combat route is held.
	ScanRouteKeep(state world.State, maxDistanceTiles float64) LootScanResult
	// StartPickup starts pickup for target and fails if a pickup is already active.
	StartPickup(target LootTarget) error
	// TickPickup advances the active pickup executor.
	TickPickup(state world.State, now time.Time) LootPickupResult
	// ClearSkippedPickup removes unitID from the in-step skip set so one recovery retry may StartPickup again.
	ClearSkippedPickup(unitID uint32)
	// TickStash transfers Pickit-matching unlocked inventory items one at a time.
	TickStash(state world.State, now time.Time) LootStashResult
	// TickCloseStash closes the personal stash and confirms UI state.
	TickCloseStash(state world.State, now time.Time) LootStashResult
	// Reset clears active pickup and in-step skipped targets.
	Reset()
}

// LootStashStatus is the task-visible personal-stash executor status.
type LootStashStatus string

// Personal-stash statuses shared with task state machines.
const (
	LootStashPending               LootStashStatus = "pending"
	LootStashSuccess               LootStashStatus = "success"
	LootStashFailed                LootStashStatus = "stash_failed"
	LootStashFull                  LootStashStatus = "stash_full"
	LootStashCloseFailed           LootStashStatus = "stash_close_failed"
	LootStashClosed                LootStashStatus = "closed"
	LootStashUnsupportedResolution LootStashStatus = "unsupported_resolution"
	LootStashTelemetryFailed       LootStashStatus = "telemetry_failed"
)

// LootStashResult reports verified transfer progress or a terminal stash outcome.
type LootStashResult struct {
	Status      LootStashStatus
	Done        bool
	Attempted   bool
	Transferred bool
	UnitID      uint32
	Code        string
	Name        string
	Attempt     int
}

// LootScanResult summarizes a task-visible loot scan without exposing pickit internals.
type LootScanResult struct {
	GroundItemCount             int
	CandidateCount              int
	InventoryFullCandidateCount int
	InventoryFull               bool
	NextTarget                  LootTarget
	HasTarget                   bool
	TelemetryFailed             bool
}

// LootTarget is the frozen ground item selected for pickup.
type LootTarget struct {
	UnitID                   uint32
	TxtFileNo                uint32
	Code                     string
	Name                     string
	Quality                  world.ItemQuality
	IdentityKind             world.ItemIdentityKind
	IdentityKey              string
	IdentityValid            bool
	PickitProfileID          string
	PickitRuleID             string
	PickitAction             string
	PickitProfileRevision    uint64
	PickitAssignmentRevision uint64
	Position                 world.Position
	AreaID                   world.AreaID
}

// LootPickupStatus is the task-level result of one pickup executor tick.
type LootPickupStatus string

// Loot pickup statuses consumed by shared run logic.
const (
	LootPickupPending          LootPickupStatus = "pending"
	LootPickupPickedUp         LootPickupStatus = "picked_up"
	LootPickupTargetUnstable   LootPickupStatus = "target_unstable"
	LootPickupTargetLost       LootPickupStatus = "target_lost"
	LootPickupTooFar           LootPickupStatus = "too_far"
	LootPickupHoverNotFound    LootPickupStatus = "hover_not_found"
	LootPickupProjectionFailed LootPickupStatus = "projection_failed"
	LootPickupFailed           LootPickupStatus = "pickup_failed"
	LootPickupMonsterNearby    LootPickupStatus = "monster_nearby"
	LootPickupInvalidWorld     LootPickupStatus = "invalid_world"
	LootPickupInputBlocked     LootPickupStatus = "input_blocked"
	LootPickupTelemetryFailed  LootPickupStatus = "telemetry_failed"
)

// LootPickupResult reports one task-visible pickup tick.
type LootPickupResult struct {
	Status       LootPickupStatus
	Done         bool
	Attempted    bool
	Target       LootTarget
	Retry        int
	HoverAttempt int
}

var (
	_ Navigator = (*pathing.Navigator)(nil)
	_ Input     = (*input.Controller)(nil)
)
