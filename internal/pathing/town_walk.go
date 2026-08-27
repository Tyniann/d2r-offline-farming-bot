package pathing

import (
	"context"
	"log/slog"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

// TownWalkStatus is a stable per-tick town walking outcome.
type TownWalkStatus string

// Town walk statuses.
const (
	TownWalkPending          TownWalkStatus = "pending"
	TownWalkWaypointVisible  TownWalkStatus = "waypoint_visible"
	TownWalkArrived          TownWalkStatus = "arrived"
	TownWalkRouteMissing     TownWalkStatus = "route_missing"
	TownWalkRouteExhausted   TownWalkStatus = "route_exhausted"
	TownWalkWrongArea        TownWalkStatus = "wrong_area"
	TownWalkProjectionFailed TownWalkStatus = "projection_failed"
	TownWalkStuck            TownWalkStatus = "stuck"
	TownWalkInputError       TownWalkStatus = "input_error"
)

// TownWalkResult reports the result of one town-walk tick.
type TownWalkResult struct {
	Status TownWalkStatus
	Reason string
	Done   bool
}

// TownWalker force-moves along one already validated Rogue Encampment graph edge.
// It owns movement timing only; layout selection and semantic edge composition
// remain caller responsibilities so this component is reusable across anchors.
type TownWalker struct {
	log          *slog.Logger
	input        InputDriver
	projector    Projector
	cfg          TownWalkConfig
	points       []world.Position
	expectedArea world.AreaID
	index        int

	lastMoveAt     time.Time
	waiting        bool
	waitStartedAt  time.Time
	lastProgressAt time.Time
	lastPos        world.Position
}

// NewTownRouteWalker creates a walker for one already validated Town graph edge.
func NewTownRouteWalker(log *slog.Logger, in InputDriver, cfg Config, points []world.Position) *TownWalker {
	return NewAreaTownRouteWalker(log, in, cfg, world.RogueEncampment, points)
}

// NewAreaTownRouteWalker creates a force-move walker bound to one confirmed Town area.
// It is used for foreign-Town egress routes where teleport movement is unavailable.
func NewAreaTownRouteWalker(log *slog.Logger, in InputDriver, cfg Config, expectedArea world.AreaID, points []world.Position) *TownWalker {
	return &TownWalker{
		log:          log.With("component", "pathing.town_walk"),
		input:        in,
		projector:    cfg.Projector(),
		cfg:          cfg.TownWalk,
		points:       append([]world.Position(nil), points...),
		expectedArea: expectedArea,
	}
}

// Reset clears in-flight walking state.
func (w *TownWalker) Reset() {
	if w == nil {
		return
	}
	w.index = 0
	w.lastMoveAt = time.Time{}
	w.waiting = false
	w.waitStartedAt = time.Time{}
	w.lastProgressAt = time.Time{}
	w.lastPos = world.Position{}
}

// CurrentRoutePointIndex returns the point currently targeted by the walker.
// Callers use it only for route-contract gates that must run before input.
func (w *TownWalker) CurrentRoutePointIndex() int {
	if w == nil {
		return 0
	}
	return w.index
}

// TickRoute advances a generic Act-1 graph edge and succeeds at its final recorded point.
func (w *TownWalker) TickRoute(ctx context.Context, state world.State) TownWalkResult {
	return w.tick(ctx, state)
}

func (w *TownWalker) tick(ctx context.Context, state world.State) TownWalkResult {
	if ctx.Err() != nil {
		return TownWalkResult{Status: TownWalkInputError, Reason: ctx.Err().Error(), Done: true}
	}
	if w == nil || w.input == nil {
		return TownWalkResult{Status: TownWalkInputError, Reason: "town walker not wired", Done: true}
	}
	if !state.Valid || state.Phase != world.GamePhaseInGame {
		return TownWalkResult{Status: TownWalkPending}
	}
	if state.Area.ID != w.expectedArea {
		return TownWalkResult{Status: TownWalkWrongArea, Reason: string(TownWalkWrongArea), Done: true}
	}
	if len(w.points) < 2 {
		return TownWalkResult{Status: TownWalkRouteMissing, Reason: string(TownWalkRouteMissing), Done: true}
	}
	if w.index >= len(w.points) {
		return TownWalkResult{Status: TownWalkRouteExhausted, Reason: string(TownWalkRouteExhausted), Done: true}
	}

	now := state.At
	if now.IsZero() {
		now = time.Now()
	}
	target := w.points[w.index]
	if world.Distance(state.Player.Position, target) <= w.cfg.ArrivalDistance {
		if w.index >= len(w.points)-1 {
			// Entering the final tolerance radius is not arrival while Force Move
			// may still be carrying the character beyond the recorded endpoint.
			return w.tickFinalRoutePoint(now, state)
		}
		w.index++
		w.waiting = false
		w.lastMoveAt = time.Time{}
		return TownWalkResult{Status: TownWalkPending}
	}

	if w.waiting {
		if w.lastProgressAt.IsZero() {
			w.lastProgressAt = now
			w.lastPos = state.Player.Position
		}
		if world.Distance(w.lastPos, state.Player.Position) >= 1 {
			w.lastProgressAt = now
			w.lastPos = state.Player.Position
		}
		if now.Sub(w.waitStartedAt) >= w.cfg.SettleTimeout && now.Sub(w.lastProgressAt) >= w.cfg.StuckTimeout {
			return TownWalkResult{Status: TownWalkStuck, Reason: string(TownWalkStuck), Done: true}
		}
		if now.Sub(w.lastMoveAt) < w.cfg.MoveInterval {
			return TownWalkResult{Status: TownWalkPending}
		}
	}

	win, ok := w.input.Window()
	if !ok {
		return TownWalkResult{Status: TownWalkProjectionFailed, Reason: string(TownWalkProjectionFailed), Done: true}
	}
	clientX, clientY, ok := w.projector.Project(state.Player.Position, target, win)
	if !ok {
		return TownWalkResult{Status: TownWalkProjectionFailed, Reason: string(TownWalkProjectionFailed), Done: true}
	}
	if err := w.input.MoveTo(clientX, clientY); err != nil {
		return TownWalkResult{Status: TownWalkInputError, Reason: err.Error(), Done: true}
	}
	if err := w.input.PressKey(w.cfg.ForceMoveKey); err != nil {
		return TownWalkResult{Status: TownWalkInputError, Reason: err.Error(), Done: true}
	}
	w.lastMoveAt = now
	w.waiting = true
	w.waitStartedAt = now
	w.lastProgressAt = now
	w.lastPos = state.Player.Position
	w.log.Debug("town walk force move",
		"target_x", target.X,
		"target_y", target.Y,
		"client_x", clientX,
		"client_y", clientY,
		"route_index", w.index,
	)
	return TownWalkResult{Status: TownWalkPending}
}

func (w *TownWalker) tickFinalRoutePoint(now time.Time, state world.State) TownWalkResult {
	// Require a stable Memory position rather than sleeping after input. Any
	// whole-tile movement restarts the settle window and keeps the edge active.
	if !w.waiting {
		w.waiting = true
		w.waitStartedAt = now
		w.lastProgressAt = now
		w.lastPos = state.Player.Position
		return TownWalkResult{Status: TownWalkPending}
	}
	if w.lastProgressAt.IsZero() {
		w.lastProgressAt = now
		w.lastPos = state.Player.Position
	}
	if world.Distance(w.lastPos, state.Player.Position) >= 1 {
		w.lastProgressAt = now
		w.lastPos = state.Player.Position
		return TownWalkResult{Status: TownWalkPending}
	}
	if now.Sub(w.lastProgressAt) < w.cfg.SettleTimeout {
		return TownWalkResult{Status: TownWalkPending}
	}
	w.Reset()
	return TownWalkResult{Status: TownWalkArrived, Done: true}
}
