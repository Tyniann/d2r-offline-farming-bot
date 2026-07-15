package app

import (
	"context"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/pathing"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

// runWaypointAdapter exposes the registered waypoint executor to the shared task pipeline.
type runWaypointAdapter struct {
	actions *pathing.WaypointActions
}

func (a *runWaypointAdapter) Reset() {
	if a != nil && a.actions != nil {
		a.actions.Reset()
	}
}

func (a *runWaypointAdapter) TickTownWaypoint(ctx context.Context, state world.State) pathing.WaypointActionResult {
	if a == nil || a.actions == nil {
		return pathing.WaypointActionResult{Status: pathing.WaypointActionInputError, Reason: "waypoint actions not wired", Done: true}
	}
	return a.actions.TickTownWaypoint(ctx, state)
}

func (a *runWaypointAdapter) SelectWaypointTarget(ctx context.Context, state world.State, target pathing.WaypointTargetID, now time.Time) pathing.WaypointActionResult {
	if a == nil || a.actions == nil {
		return pathing.WaypointActionResult{Status: pathing.WaypointActionInputError, Reason: "waypoint actions not wired", Done: true}
	}
	return a.actions.SelectWaypointTarget(ctx, state, target, now)
}
