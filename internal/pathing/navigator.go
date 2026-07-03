package pathing

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

// tickGapReset resets the stuck baseline when ticks were suspended (e.g. pause
// hotkey) so wall-clock pause time never counts as "no progress".
const tickGapReset = 2 * time.Second

// teleportSettleTimeout is how long a teleport cast may take before it counts
// as blocked. The cast animation (FCR-dependent) delays the memory position
// update by several hundred milliseconds; judging the cast on the next poll
// tick would treat every cast as blocked and spin the bearing in circles.
const teleportSettleTimeout = 700 * time.Millisecond

// ErrNavigatorNotWired reports that movement dependencies were not injected.
var ErrNavigatorNotWired = errors.New("navigator not wired")

// Navigator is the Phase 4.3 movement state machine. It moves exclusively by
// teleport, detects arrival primarily via area change (`state.Area.ID`), and
// aborts on stuck or failed entity clicks.
//
// States: idle → moving/exploring → clicking → arrived | stuck | failed.
// Abort conditions: context cancel (cancelled), stuck detector (stuck),
// exhausted hover attempts (hover_not_found), unbound window or invalid
// projection (projection_failed).
type Navigator struct {
	log   *slog.Logger
	deps  Deps
	wired bool

	mover    *TeleportMover
	clicker  *EntityClicker
	explorer *ExplorePlanner
	stuck    *StuckDetector

	goal       Goal
	active     bool
	status     NavStatus
	lastResult NavResult
	lastTickAt time.Time
	lastCast   struct {
		pos            world.Position
		at             time.Time
		pending        bool
		approachUnitID uint32
	}
}

// NewNavigator builds a navigator from injected dependencies. A navigator
// constructed without input/bindings (e.g. read-only probe mode) reports
// Ready() == false and rejects Start.
func NewNavigator(log *slog.Logger, deps Deps) *Navigator {
	n := &Navigator{
		log:    log.With("component", "pathing"),
		deps:   deps,
		status: NavIdle,
	}
	if deps.Input != nil && deps.Bindings != nil {
		projector := deps.Config.Projector()
		n.mover = NewTeleportMover(n.log, deps.Input, deps.Bindings, projector, deps.Config.MoveInterval)
		n.clicker = NewEntityClicker(n.log, deps.Input, projector, deps.Config.Click)
		n.explorer = NewExplorePlanner(deps.Config.Explore)
		n.stuck = NewStuckDetector(deps.Config.StuckTimeout, deps.Config.StuckProgressTiles)
		n.wired = true
	}
	return n
}

// Ready reports whether input and bindings are wired for movement.
func (n *Navigator) Ready() bool {
	return n.wired
}

// Active reports whether a goal is currently being pursued.
func (n *Navigator) Active() bool {
	return n.active
}

// LastResult returns the outcome of the most recently finished goal.
func (n *Navigator) LastResult() NavResult {
	return n.lastResult
}

// Reset aborts any active goal and clears per-goal movement state.
func (n *Navigator) Reset() {
	n.goal = Goal{}
	n.active = false
	n.status = NavIdle
	n.lastResult = NavResult{}
	n.lastTickAt = time.Time{}
	n.lastCast.pending = false
	n.lastCast.approachUnitID = 0
	if n.mover != nil {
		n.mover.Reset()
	}
	if n.clicker != nil {
		n.clicker.Reset()
	}
	if n.explorer != nil {
		n.explorer.Reset()
	}
	if n.stuck != nil {
		n.stuck.Reset()
	}
}

// Start begins pursuing goal. An already active goal is replaced.
func (n *Navigator) Start(goal Goal) error {
	if !n.wired {
		return fmt.Errorf("%w: input and bindings required", ErrNavigatorNotWired)
	}
	switch goal.Kind {
	case GoalKindMoveToArea:
		if goal.TargetArea == 0 {
			return fmt.Errorf("%s: target area required", ReasonInvalidGoal)
		}
	case GoalKindMoveToPosition:
		if goal.TargetPos == (world.Position{}) {
			return fmt.Errorf("%s: target position required", ReasonInvalidGoal)
		}
	default:
		return fmt.Errorf("%s: unsupported goal kind %q", ReasonInvalidGoal, goal.Kind.String())
	}

	n.goal = goal
	n.active = true
	n.status = NavMoving
	n.lastTickAt = time.Time{}
	n.lastCast.pending = false
	n.lastCast.approachUnitID = 0
	n.mover.Reset()
	n.clicker.Reset()
	n.explorer.Reset()
	n.stuck.Reset()

	n.log.Info("pathing nav goal started",
		"goal", goal.Kind.String(),
		"target_area", uint32(goal.TargetArea),
		"target_x", goal.TargetPos.X,
		"target_y", goal.TargetPos.Y,
		"via_entrance", goal.ViaEntrance.String(),
	)
	return nil
}

// Tick advances the state machine by one poll cycle. Ticks during loading or
// invalid world states are skipped without counting toward stuck detection.
func (n *Navigator) Tick(ctx context.Context, state world.State) NavTickResult {
	if !n.active {
		return NavTickResult{Status: n.status, Reason: n.lastResult.Reason, Done: n.status != NavIdle}
	}
	if ctx.Err() != nil {
		return n.finish(NavFailed, ReasonCancelled, state)
	}
	if !state.Valid || state.Phase != world.GamePhaseInGame {
		n.lastTickAt = time.Time{}
		return NavTickResult{Status: n.status}
	}

	now := state.At
	if now.IsZero() {
		now = time.Now()
	}
	if !n.lastTickAt.IsZero() && now.Sub(n.lastTickAt) > tickGapReset {
		n.stuck.Reset()
	}
	n.lastTickAt = now

	if n.goalReached(state) {
		return n.finish(NavArrived, "", state)
	}

	if n.stuck.Update(now, state) {
		n.log.Warn("pathing nav stuck",
			"goal", n.goal.Kind.String(),
			"area", state.Area.Name,
			"player_x", state.Player.Position.X,
			"player_y", state.Player.Position.Y,
			"player_mana", state.Player.Mana,
		)
		return n.finish(NavStuck, ReasonStuck, state)
	}

	switch n.goal.Kind {
	case GoalKindMoveToArea:
		return n.tickMoveToArea(now, state)
	case GoalKindMoveToPosition:
		return n.tickMoveToPosition(now, state)
	default:
		return n.finish(NavFailed, ReasonInvalidGoal, state)
	}
}

// goalReached checks the primary success signal (area) or arrival distance.
func (n *Navigator) goalReached(state world.State) bool {
	switch n.goal.Kind {
	case GoalKindMoveToArea:
		return state.Area.ID == n.goal.TargetArea
	case GoalKindMoveToPosition:
		return world.Distance(state.Player.Position, n.goal.TargetPos) <= n.deps.Config.ArrivalDistance
	default:
		return false
	}
}

func (n *Navigator) tickMoveToArea(now time.Time, state world.State) NavTickResult {
	// Judge the previous teleport before planning so a rotation affects the
	// next target. While the cast has neither landed nor timed out, wait.
	if !n.evaluatePendingCast(now, state) {
		return NavTickResult{Status: n.status}
	}

	plan := n.explorer.Plan(state, n.goal)

	if plan.Mode == ExploreEntity {
		n.status = NavClicking
		target := ClickTarget{
			UnitID:   plan.Entrance.UnitID,
			UnitType: world.HoverUnitTypeEntrance,
			Position: plan.Entrance.Position,
			Name:     plan.Entrance.Name,
		}
		maxDistance := n.deps.Config.Explore.MaxEntranceClickDistance
		if plan.ForceClick {
			maxDistance = 0
		}
		res, err := n.clicker.Tick(state, target, maxDistance)
		if err != nil {
			n.log.Debug("pathing nav click blocked", "error", err)
			return NavTickResult{Status: n.status}
		}
		n.logNavTick(state, "entity", res.Attempt)
		switch res.Status {
		case ClickHit:
			// Area change confirms the transition on subsequent ticks.
			return NavTickResult{Status: n.status}
		case ClickHoverNotFound:
			return n.finish(NavFailed, ReasonHoverNotFound, state)
		case ClickProjectionFailed:
			return n.finish(NavFailed, ReasonProjectionFailed, state)
		default:
			return NavTickResult{Status: n.status}
		}
	}

	n.status = NavExploring
	if !n.mover.Ready(now) {
		return NavTickResult{Status: n.status}
	}
	if _, _, err := n.mover.TeleportTo(now, state.Player.Position, plan.Target); err != nil {
		n.log.Debug("pathing nav teleport blocked", "error", err)
		return NavTickResult{Status: n.status}
	}
	n.lastCast.pos = state.Player.Position
	n.lastCast.at = now
	n.lastCast.pending = true
	n.lastCast.approachUnitID = 0
	if plan.Mode == ExploreEntityApproach {
		n.lastCast.approachUnitID = plan.Entrance.UnitID
	}
	n.logNavTick(state, string(plan.Mode), 0)
	return NavTickResult{Status: n.status}
}

func (n *Navigator) tickMoveToPosition(now time.Time, state world.State) NavTickResult {
	n.status = NavMoving
	if !n.mover.Ready(now) {
		return NavTickResult{Status: n.status}
	}
	target := stepToward(state.Player.Position, n.goal.TargetPos, n.deps.Config.Explore.StepDistanceTiles)
	if _, _, err := n.mover.TeleportTo(now, state.Player.Position, target); err != nil {
		n.log.Debug("pathing nav teleport blocked", "error", err)
		return NavTickResult{Status: n.status}
	}
	n.logNavTick(state, "direct", 0)
	return NavTickResult{Status: n.status}
}

// evaluatePendingCast resolves the outcome of the previous teleport cast.
// It returns true when planning/casting may continue this tick:
//   - Position advanced by at least stuck_progress_tiles → progress, continue
//     immediately (fast chaining while teleports land).
//   - No movement and teleportSettleTimeout elapsed → cast counts as blocked,
//     rotate the bearing, continue with the new direction.
//   - No movement but the cast may still be in flight → return false and wait.
func (n *Navigator) evaluatePendingCast(now time.Time, state world.State) bool {
	if !n.lastCast.pending {
		return true
	}
	moved := world.Distance(n.lastCast.pos, state.Player.Position)
	if moved >= n.deps.Config.StuckProgressTiles {
		n.lastCast.pending = false
		n.lastCast.approachUnitID = 0
		return true
	}
	if now.Sub(n.lastCast.at) >= teleportSettleTimeout {
		if n.lastCast.approachUnitID != 0 {
			n.explorer.ForceClickEntrance(n.lastCast.approachUnitID)
			n.log.Debug("pathing nav entity approach blocked",
				"reason", "teleport_blocked",
				"unit_id", n.lastCast.approachUnitID,
				"player_x", state.Player.Position.X,
				"player_y", state.Player.Position.Y,
			)
		} else {
			n.explorer.Rotate()
			n.log.Debug("pathing nav bearing rotated",
				"reason", "teleport_blocked",
				"bearing_index", n.explorer.BearingIndex(),
				"player_x", state.Player.Position.X,
				"player_y", state.Player.Position.Y,
			)
		}
		n.lastCast.pending = false
		n.lastCast.approachUnitID = 0
		return true
	}
	return false
}

func (n *Navigator) finish(status NavStatus, reason string, state world.State) NavTickResult {
	n.status = status
	n.active = false
	n.lastResult = NavResult{Status: status, Reason: reason, Goal: n.goal}
	n.log.Info("pathing nav finished",
		"goal", n.goal.Kind.String(),
		"status", string(status),
		"reason", reason,
		"area", state.Area.Name,
		"area_id", uint32(state.Area.ID),
		"player_x", state.Player.Position.X,
		"player_y", state.Player.Position.Y,
	)
	return NavTickResult{Status: status, Reason: reason, Done: true}
}

func (n *Navigator) logNavTick(state world.State, exploreMode string, hoverAttempt int) {
	n.log.Debug("pathing nav",
		"goal", n.goal.Kind.String(),
		"status", string(n.status),
		"area", state.Area.Name,
		"explore_mode", exploreMode,
		"bearing_index", n.explorer.BearingIndex(),
		"player_x", state.Player.Position.X,
		"player_y", state.Player.Position.Y,
		"hover_attempt", hoverAttempt,
	)
}

// stepToward moves from pos toward target by at most stepTiles.
func stepToward(pos, target world.Position, stepTiles float64) world.Position {
	dist := world.Distance(pos, target)
	if dist <= stepTiles || dist == 0 {
		return target
	}
	scale := stepTiles / dist
	dx := (float64(target.X) - float64(pos.X)) * scale
	dy := (float64(target.Y) - float64(pos.Y)) * scale
	return offsetPosition(pos, dx, dy)
}
