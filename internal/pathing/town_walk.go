package pathing

import (
	"context"
	"errors"
	"log/slog"
	"os"
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

// TownWalker force-moves inside Rogue Encampment toward the Act-1 waypoint.
type TownWalker struct {
	log                   *slog.Logger
	input                 InputDriver
	projector             Projector
	cfg                   TownWalkConfig
	waypointClickDistance float64

	points []world.Position
	loaded bool
	index  int

	lastMoveAt     time.Time
	waiting        bool
	waitStartedAt  time.Time
	lastProgressAt time.Time
	lastPos        world.Position
}

// NewTownWalker wires town walking to input and pathing config.
func NewTownWalker(log *slog.Logger, in InputDriver, cfg Config) *TownWalker {
	return &TownWalker{
		log:                   log.With("component", "pathing.town_walk"),
		input:                 in,
		projector:             cfg.Projector(),
		cfg:                   cfg.TownWalk,
		waypointClickDistance: cfg.Waypoint.MaxClickDistance,
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

// TickAct1Waypoint advances the force-move route until the town waypoint is visible.
func (w *TownWalker) TickAct1Waypoint(ctx context.Context, state world.State) TownWalkResult {
	if ctx.Err() != nil {
		return TownWalkResult{Status: TownWalkInputError, Reason: ctx.Err().Error(), Done: true}
	}
	if w == nil || w.input == nil {
		return TownWalkResult{Status: TownWalkInputError, Reason: "town walker not wired", Done: true}
	}
	if townWaypointClickable(state, w.waypointClickDistance) {
		w.Reset()
		return TownWalkResult{Status: TownWalkWaypointVisible, Done: true}
	}
	if !state.Valid || state.Phase != world.GamePhaseInGame {
		return TownWalkResult{Status: TownWalkPending}
	}
	if state.Area.ID != world.RogueEncampment {
		return TownWalkResult{Status: TownWalkWrongArea, Reason: string(TownWalkWrongArea), Done: true}
	}
	if err := w.ensureRoute(); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return TownWalkResult{Status: TownWalkRouteMissing, Reason: err.Error(), Done: true}
		}
		return TownWalkResult{Status: TownWalkInputError, Reason: err.Error(), Done: true}
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
		w.index++
		w.waiting = false
		w.lastMoveAt = time.Time{}
		if w.index >= len(w.points) {
			return TownWalkResult{Status: TownWalkArrived, Done: true}
		}
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

func townWaypointClickable(state world.State, maxDistance float64) bool {
	wp, ok := state.NearestObject(world.ObjectKindWaypoint)
	if !ok {
		return false
	}
	if maxDistance <= 0 {
		return true
	}
	return world.Distance(state.Player.Position, wp.Position) <= maxDistance
}

func (w *TownWalker) ensureRoute() error {
	if w.loaded {
		return nil
	}
	w.loaded = true
	if len(w.cfg.Act1WaypointPoints) > 0 {
		w.points = append([]world.Position(nil), w.cfg.Act1WaypointPoints...)
		return nil
	}
	if w.cfg.RouteFile != "" {
		points, err := LoadTownRoute(w.cfg.RouteFile)
		if err == nil {
			w.points = points
			return nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			w.log.Warn("town route override ignored; using built-in preset",
				"route_file", w.cfg.RouteFile,
				"error", err,
			)
		}
	}
	w.points = defaultAct1WaypointRoute()
	return nil
}
