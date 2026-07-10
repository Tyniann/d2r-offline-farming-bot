package tasks

import (
	"context"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/input"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/pathing"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

// Deps holds shared runtime dependencies injected into task runs.
type Deps struct {
	Input    Input
	Pathing  Navigator
	Waypoint WaypointActions
	TownWalk TownWalker
	Combat   CombatActions
	Actions  RunActions
	Loot     LootActions
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

// TownWalker is the narrow town-walk surface used by task runs.
type TownWalker interface {
	Reset()
	TickAct1Waypoint(context.Context, world.State) pathing.TownWalkResult
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
	// Reset clears active pickup and in-step skipped targets.
	Reset()
}

// LootScanResult summarizes a task-visible loot scan without exposing pickit internals.
type LootScanResult struct {
	GroundItemCount int
	CandidateCount  int
	NextTarget      LootTarget
	HasTarget       bool
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
)

// LootPickupResult reports one task-visible pickup tick.
type LootPickupResult struct {
	Status       LootPickupStatus
	Done         bool
	Target       LootTarget
	Retry        int
	HoverAttempt int
}

var (
	_ Navigator = (*pathing.Navigator)(nil)
	_ Input     = (*input.Controller)(nil)
)
