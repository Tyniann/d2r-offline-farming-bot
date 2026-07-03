package tasks

import (
	"context"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/pathing"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

const (
	// CountessPhaseTravelMarsh selects the Town -> Black Marsh travel phase.
	CountessPhaseTravelMarsh = "travel-marsh"

	countessStepPrecheck       = "precheck"
	countessStepArmed          = "armed"
	countessStepAcquireTownWP  = "acquire_town_waypoint"
	countessStepOpenWaypoint   = "open_waypoint"
	countessStepSelectMarsh    = "select_black_marsh"
	countessStepWaitBlackMarsh = "wait_black_marsh"
	countessStepComplete       = "complete"

	selectMarshSettleDelay = 500 * time.Millisecond
)

// countessRun executes the Countess stub or a selected Countess phase.
type countessRun struct {
	phase        string
	selectedOnce bool
}

func (c *countessRun) firstStep() string {
	return countessStepPrecheck
}

func (c *countessRun) nextStep(current string) string {
	if c.phase == CountessPhaseTravelMarsh {
		switch current {
		case countessStepPrecheck:
			return countessStepAcquireTownWP
		case countessStepAcquireTownWP:
			return countessStepOpenWaypoint
		case countessStepOpenWaypoint:
			return countessStepSelectMarsh
		case countessStepSelectMarsh:
			return countessStepWaitBlackMarsh
		case countessStepWaitBlackMarsh:
			return ""
		default:
			return ""
		}
	}
	switch current {
	case countessStepPrecheck:
		return countessStepArmed
	case countessStepArmed:
		return countessStepComplete
	default:
		return ""
	}
}

func (c *countessRun) usesTickTimeout(step string) bool {
	return c.phase == "" && step == countessStepArmed
}

func (c *countessRun) allowsNonInputTick(step string) bool {
	return c.phase == CountessPhaseTravelMarsh && step == countessStepWaitBlackMarsh
}

func (c *countessRun) onStepEnter(step string) {
	c.selectedOnce = false
}

func (c *countessRun) onTick(ctx context.Context, deps Deps, step string, w world.State, now time.Time, stepStartedAt time.Time, ticksInStep int) stepResult {
	if c.phase == CountessPhaseTravelMarsh {
		return c.onTravelMarshTick(ctx, deps, step, w, now, stepStartedAt)
	}
	switch step {
	case countessStepPrecheck:
		if !w.Valid {
			return stepResult{failed: true, reason: "invalid_world"}
		}
		if w.Area.IsTown() {
			return stepResult{complete: true}
		}
		return stepResult{failed: true, reason: "not_in_town"}
	case countessStepArmed:
		if ticksInStep >= 2 {
			return stepResult{complete: true}
		}
		return stepResult{}
	case countessStepComplete:
		return stepResult{complete: true}
	default:
		return stepResult{failed: true, reason: "unknown_step"}
	}
}

func (c *countessRun) onTravelMarshTick(ctx context.Context, deps Deps, step string, w world.State, now time.Time, stepStartedAt time.Time) stepResult {
	switch step {
	case countessStepPrecheck:
		if !w.Valid {
			return stepResult{failed: true, reason: "invalid_world"}
		}
		if w.Phase != world.GamePhaseInGame {
			return stepResult{failed: true, reason: "not_in_game"}
		}
		if w.Area.ID != world.RogueEncampment {
			return stepResult{failed: true, reason: "not_act1_town"}
		}
		return stepResult{complete: true}
	case countessStepAcquireTownWP:
		if deps.TownWalk == nil {
			return stepResult{failed: true, reason: "town_walk_not_wired"}
		}
		res := deps.TownWalk.TickAct1Waypoint(ctx, w)
		switch res.Status {
		case pathing.TownWalkPending:
			return stepResult{}
		case pathing.TownWalkWaypointVisible, pathing.TownWalkArrived:
			return stepResult{complete: true}
		default:
			return stepResult{failed: true, reason: townWalkFailureReason(res)}
		}
	case countessStepOpenWaypoint:
		if deps.Waypoint == nil {
			return stepResult{failed: true, reason: "waypoint_actions_not_wired"}
		}
		res := deps.Waypoint.TickTownWaypoint(ctx, w)
		switch res.Status {
		case pathing.WaypointActionPending:
			return stepResult{}
		case pathing.WaypointActionClicked:
			return stepResult{complete: true}
		default:
			return stepResult{failed: true, reason: waypointFailureReason(res)}
		}
	case countessStepSelectMarsh:
		if deps.Waypoint == nil {
			return stepResult{failed: true, reason: "waypoint_actions_not_wired"}
		}
		if now.Sub(stepStartedAt) < selectMarshSettleDelay {
			return stepResult{}
		}
		return c.selectBlackMarsh(ctx, deps, now)
	case countessStepWaitBlackMarsh:
		if w.Valid && w.Area.ID == world.BlackMarsh {
			return stepResult{complete: true}
		}
		return stepResult{}
	default:
		return stepResult{failed: true, reason: "unknown_step"}
	}
}

func (c *countessRun) selectBlackMarsh(ctx context.Context, deps Deps, _ time.Time) stepResult {
	if !c.selectedOnce {
		res := deps.Waypoint.SelectBlackMarsh(ctx)
		c.selectedOnce = true
		if res.Status != pathing.WaypointActionClicked {
			return stepResult{failed: true, reason: waypointFailureReason(res)}
		}
		return stepResult{complete: true}
	}
	return stepResult{complete: true}
}

func waypointFailureReason(res pathing.WaypointActionResult) string {
	if res.Reason != "" {
		return res.Reason
	}
	return string(res.Status)
}

func townWalkFailureReason(res pathing.TownWalkResult) string {
	if res.Reason != "" {
		return res.Reason
	}
	return string(res.Status)
}

type stepResult struct {
	complete bool
	failed   bool
	reason   string
}
