package pathing

import (
	"context"
	"log/slog"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/input"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

// WaypointActionStatus is a stable per-tick waypoint action outcome.
type WaypointActionStatus string

// Waypoint action statuses.
const (
	WaypointActionPending          WaypointActionStatus = "pending"
	WaypointActionClicked          WaypointActionStatus = "clicked"
	WaypointActionNotFound         WaypointActionStatus = "not_found"
	WaypointActionTooFar           WaypointActionStatus = "too_far"
	WaypointActionHoverNotFound    WaypointActionStatus = "hover_not_found"
	WaypointActionProjectionFailed WaypointActionStatus = "projection_failed"
	WaypointActionInputError       WaypointActionStatus = "input_error"
)

// WaypointActionResult reports the result of one waypoint action tick.
type WaypointActionResult struct {
	Status WaypointActionStatus
	Reason string
	Done   bool
}

// WaypointActions performs waypoint-object and waypoint-menu input for runs.
type WaypointActions struct {
	log     *slog.Logger
	input   InputDriver
	clicker *EntityClicker
	cfg     Config
}

// NewWaypointActions wires waypoint actions to input and pathing config.
func NewWaypointActions(log *slog.Logger, in InputDriver, cfg Config) *WaypointActions {
	projector := cfg.Projector()
	return &WaypointActions{
		log:     log.With("component", "pathing.waypoint"),
		input:   in,
		clicker: NewEntityClicker(log, in, projector, cfg.Click),
		cfg:     cfg,
	}
}

// Reset clears in-flight hover-click state.
func (w *WaypointActions) Reset() {
	if w == nil || w.clicker == nil {
		return
	}
	w.clicker.Reset()
}

// TickTownWaypoint advances the hover-confirmed click on the nearest town waypoint.
func (w *WaypointActions) TickTownWaypoint(ctx context.Context, state world.State) WaypointActionResult {
	if ctx.Err() != nil {
		return WaypointActionResult{Status: WaypointActionInputError, Reason: ctx.Err().Error(), Done: true}
	}
	if w == nil || w.clicker == nil {
		return WaypointActionResult{Status: WaypointActionInputError, Reason: "waypoint actions not wired", Done: true}
	}
	obj, ok := state.NearestObject(world.ObjectKindWaypoint)
	if !ok {
		w.Reset()
		return WaypointActionResult{Status: WaypointActionNotFound, Reason: string(WaypointActionNotFound), Done: true}
	}
	target := ClickTarget{
		UnitID:   obj.UnitID,
		UnitType: world.HoverUnitTypeObject,
		Position: obj.Position,
		Name:     obj.Name,
	}
	res, err := w.clicker.Tick(state, target, w.cfg.Waypoint.MaxClickDistance)
	if err != nil {
		return WaypointActionResult{Status: WaypointActionInputError, Reason: err.Error(), Done: true}
	}
	switch res.Status {
	case ClickPending:
		return WaypointActionResult{Status: WaypointActionPending}
	case ClickHit:
		return WaypointActionResult{Status: WaypointActionClicked, Done: true}
	case ClickTooFar:
		return WaypointActionResult{Status: WaypointActionTooFar, Reason: string(WaypointActionTooFar), Done: true}
	case ClickHoverNotFound:
		return WaypointActionResult{Status: WaypointActionHoverNotFound, Reason: string(WaypointActionHoverNotFound), Done: true}
	case ClickProjectionFailed:
		return WaypointActionResult{Status: WaypointActionProjectionFailed, Reason: string(WaypointActionProjectionFailed), Done: true}
	default:
		return WaypointActionResult{Status: WaypointActionInputError, Reason: string(res.Status), Done: true}
	}
}

// SelectBlackMarsh clicks the configured Black Marsh row in the waypoint menu.
func (w *WaypointActions) SelectBlackMarsh(ctx context.Context) WaypointActionResult {
	if ctx.Err() != nil {
		return WaypointActionResult{Status: WaypointActionInputError, Reason: ctx.Err().Error(), Done: true}
	}
	if w == nil || w.input == nil {
		return WaypointActionResult{Status: WaypointActionInputError, Reason: "waypoint actions not wired", Done: true}
	}
	if err := w.input.MoveTo(w.cfg.WaypointUI.BlackMarshX, w.cfg.WaypointUI.BlackMarshY); err != nil {
		return WaypointActionResult{Status: WaypointActionInputError, Reason: err.Error(), Done: true}
	}
	if err := w.input.Click(input.MouseLeft); err != nil {
		return WaypointActionResult{Status: WaypointActionInputError, Reason: err.Error(), Done: true}
	}
	w.log.Info("waypoint ui selected",
		"target", "Black Marsh",
		"client_x", w.cfg.WaypointUI.BlackMarshX,
		"client_y", w.cfg.WaypointUI.BlackMarshY,
	)
	return WaypointActionResult{Status: WaypointActionClicked, Done: true}
}
