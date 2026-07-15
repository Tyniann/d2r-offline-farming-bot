package pathing

import (
	"context"
	"log/slog"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/input"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

// WaypointActionStatus is a stable per-tick waypoint action outcome.
type WaypointActionStatus string

// Waypoint action statuses.
const (
	WaypointActionPending               WaypointActionStatus = "pending"
	WaypointActionClicked               WaypointActionStatus = "clicked"
	WaypointActionNotFound              WaypointActionStatus = "not_found"
	WaypointActionTooFar                WaypointActionStatus = "too_far"
	WaypointActionHoverNotFound         WaypointActionStatus = "hover_not_found"
	WaypointActionProjectionFailed      WaypointActionStatus = "projection_failed"
	WaypointActionInputError            WaypointActionStatus = "input_error"
	WaypointActionTargetUnsupported     WaypointActionStatus = "waypoint_target_unsupported"
	WaypointActionUIUnconfirmed         WaypointActionStatus = "waypoint_ui_unconfirmed"
	WaypointActionUnsupportedResolution WaypointActionStatus = "unsupported_resolution"
)

// WaypointActionResult reports the result of one waypoint action tick.
type WaypointActionResult struct {
	Status WaypointActionStatus
	Reason string
	Done   bool
}

// WaypointActions performs waypoint-object and waypoint-menu input for runs.
type WaypointActions struct {
	log             *slog.Logger
	input           InputDriver
	clicker         *EntityClicker
	cfg             Config
	registry        *WaypointTargetRegistry
	selectionTarget WaypointTargetID
	tabClickedAt    time.Time
	rowClicked      bool
}

// NewWaypointActions wires waypoint actions to input and pathing config.
func NewWaypointActions(log *slog.Logger, in InputDriver, cfg Config) *WaypointActions {
	projector := cfg.Projector()
	return &WaypointActions{
		log:      log.With("component", "pathing.waypoint"),
		input:    in,
		clicker:  NewEntityClicker(log, in, projector, cfg.Click),
		cfg:      cfg,
		registry: DefaultWaypointTargetRegistry(),
	}
}

// Reset clears in-flight hover-click state.
func (w *WaypointActions) Reset() {
	if w == nil {
		return
	}
	if w.clicker != nil {
		w.clicker.Reset()
	}
	w.selectionTarget = ""
	w.tabClickedAt = time.Time{}
	w.rowClicked = false
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

// SelectWaypointTarget advances a registered waypoint selection. It sends at
// most one click per tick, requires the Memory-confirmed waypoint menu before
// every click, and never repeats the destination click.
func (w *WaypointActions) SelectWaypointTarget(ctx context.Context, state world.State, target WaypointTargetID, now time.Time) WaypointActionResult {
	if ctx.Err() != nil {
		return WaypointActionResult{Status: WaypointActionInputError, Reason: ctx.Err().Error(), Done: true}
	}
	if w == nil || w.input == nil || w.registry == nil {
		return WaypointActionResult{Status: WaypointActionInputError, Reason: "waypoint actions not wired", Done: true}
	}
	action, ok := w.registry.Action(target)
	if !ok {
		return WaypointActionResult{Status: WaypointActionTargetUnsupported, Reason: string(WaypointActionTargetUnsupported), Done: true}
	}
	if !state.Valid || state.Phase != world.GamePhaseInGame || !state.UI.WaypointOpen {
		return WaypointActionResult{Status: WaypointActionUIUnconfirmed, Reason: string(WaypointActionUIUnconfirmed), Done: true}
	}
	window, bound := w.input.Window()
	if !bound || window.ClientWidth != action.ClientWidth || window.ClientHeight != action.ClientHeight {
		return WaypointActionResult{Status: WaypointActionUnsupportedResolution, Reason: string(WaypointActionUnsupportedResolution), Done: true}
	}
	if w.selectionTarget != "" && w.selectionTarget != target {
		return WaypointActionResult{Status: WaypointActionTargetUnsupported, Reason: "waypoint_target_changed", Done: true}
	}
	w.selectionTarget = target
	if w.rowClicked {
		return WaypointActionResult{Status: WaypointActionClicked, Done: true}
	}
	if w.tabClickedAt.IsZero() {
		if err := w.clickAt(action.TabX, action.TabY); err != nil {
			return WaypointActionResult{Status: WaypointActionInputError, Reason: err.Error(), Done: true}
		}
		w.tabClickedAt = now
		w.log.Info("waypoint ui act tab selected", "target", action.Name, "act", action.Act, "client_x", action.TabX, "client_y", action.TabY)
		return WaypointActionResult{Status: WaypointActionPending}
	}
	if now.Sub(w.tabClickedAt) < time.Duration(action.SettleMs)*time.Millisecond {
		return WaypointActionResult{Status: WaypointActionPending}
	}
	if err := w.clickAt(action.RowX, action.RowY); err != nil {
		return WaypointActionResult{Status: WaypointActionInputError, Reason: err.Error(), Done: true}
	}
	w.rowClicked = true
	w.log.Info("waypoint ui selected",
		"target", action.Name,
		"target_id", action.ID,
		"expected_area_id", action.ExpectedAreaID,
		"client_x", action.RowX,
		"client_y", action.RowY,
	)
	return WaypointActionResult{Status: WaypointActionClicked, Done: true}
}

func (w *WaypointActions) clickAt(x, y int) error {
	if err := w.input.MoveTo(x, y); err != nil {
		return err
	}
	return w.input.Click(input.MouseLeft)
}
