package tasks

import (
	"context"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/input"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/pathing"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/profile"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

// Deps holds shared runtime dependencies injected into task runs.
type Deps struct {
	Input    Input
	Pathing  Navigator
	Waypoint WaypointActions
	Portal   TownPortalActions
	TownWalk TownWalker
	Stash    PersonalStashActions
	Combat   CombatActions
	Actions  RunActions
	Loot     LootActions
	Route    RoutePlayback
	Profile  ProfileActions
	Town     TownPreparationActions
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
	TickResources(world.State, time.Time) profile.Result
	Reset()
}

// RoutePlayback is the generic full-route surface used by run adapters.
type RoutePlayback interface {
	Start(routeID string, state world.State) error
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
	SelectBlackMarsh(context.Context) pathing.WaypointActionResult
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
	// CastSkillAtWorld casts skillID at targetPos projected from playerPos.
	CastSkillAtWorld(now time.Time, skillID uint16, playerPos, targetPos world.Position) error
	// TeleportToward moves toward targetPos while trying to preserve desiredDistanceTiles.
	TeleportToward(now time.Time, playerPos, targetPos world.Position, desiredDistanceTiles float64) error
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

// LootActions exposes the stateful loot pickup loop used by Countess tasks.
type LootActions interface {
	// Scan evaluates current loot and returns the next non-skipped target.
	Scan(state world.State) LootScanResult
	// StartPickup starts pickup for target and fails if a pickup is already active.
	StartPickup(target LootTarget) error
	// TickPickup advances the active pickup executor.
	TickPickup(state world.State, now time.Time) LootPickupResult
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
	UnitID    uint32
	TxtFileNo uint32
	Code      string
	Name      string
	Position  world.Position
	AreaID    world.AreaID
}

// LootPickupStatus is the task-level result of one pickup executor tick.
type LootPickupStatus string

// Loot pickup statuses consumed by Countess task logic.
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
