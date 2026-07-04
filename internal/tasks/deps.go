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

var (
	_ Navigator = (*pathing.Navigator)(nil)
	_ Input     = (*input.Controller)(nil)
)
