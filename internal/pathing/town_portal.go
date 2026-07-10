package pathing

import (
	"context"
	"log/slog"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

// TownPortalActionStatus is a stable per-tick outcome of player-cast portal entry.
type TownPortalActionStatus string

// Town portal action statuses.
const (
	TownPortalActionPending          TownPortalActionStatus = "pending"
	TownPortalActionClicked          TownPortalActionStatus = "clicked"
	TownPortalActionNotFound         TownPortalActionStatus = "not_found"
	TownPortalActionTooFar           TownPortalActionStatus = "too_far"
	TownPortalActionHoverNotFound    TownPortalActionStatus = "hover_not_found"
	TownPortalActionProjectionFailed TownPortalActionStatus = "projection_failed"
	TownPortalActionInputError       TownPortalActionStatus = "input_error"
)

// TownPortalActionResult reports one portal-entry tick.
type TownPortalActionResult struct {
	Status TownPortalActionStatus
	Reason string
	Done   bool
}

// TownPortalActions discovers and hover-clicks the temporary player-cast portal.
type TownPortalActions struct {
	log          *slog.Logger
	clicker      *EntityClicker
	cfg          TownPortalConfig
	missingSince time.Time
}

// NewTownPortalActions wires safe portal entry to the shared entity clicker.
func NewTownPortalActions(log *slog.Logger, in InputDriver, cfg Config) *TownPortalActions {
	return &TownPortalActions{
		log:     log.With("component", "pathing.town_portal"),
		clicker: NewEntityClicker(log, in, cfg.Projector(), cfg.Click),
		cfg:     cfg.TownPortal,
	}
}

// Reset clears portal discovery and hover-click state.
func (a *TownPortalActions) Reset() {
	if a == nil {
		return
	}
	a.missingSince = time.Time{}
	if a.clicker != nil {
		a.clicker.Reset()
	}
}

// Tick advances discovery and hover-confirmed entry of the nearest town portal.
func (a *TownPortalActions) Tick(ctx context.Context, state world.State, now time.Time) TownPortalActionResult {
	if ctx.Err() != nil {
		return TownPortalActionResult{Status: TownPortalActionInputError, Reason: ctx.Err().Error(), Done: true}
	}
	if a == nil || a.clicker == nil {
		return TownPortalActionResult{Status: TownPortalActionInputError, Reason: "town portal actions not wired", Done: true}
	}
	portal, ok := state.NearestObject(world.ObjectKindTownPortal)
	if !ok {
		if a.missingSince.IsZero() {
			a.missingSince = now
			return TownPortalActionResult{Status: TownPortalActionPending}
		}
		if now.Sub(a.missingSince) < a.cfg.AppearTimeout {
			return TownPortalActionResult{Status: TownPortalActionPending}
		}
		a.Reset()
		return TownPortalActionResult{Status: TownPortalActionNotFound, Reason: string(TownPortalActionNotFound), Done: true}
	}
	a.missingSince = time.Time{}
	res, err := a.clicker.Tick(state, ClickTarget{
		UnitID:   portal.UnitID,
		UnitType: world.HoverUnitTypeObject,
		Position: portal.Position,
		Name:     portal.Name,
	}, a.cfg.MaxClickDistance)
	if err != nil {
		return TownPortalActionResult{Status: TownPortalActionInputError, Reason: err.Error(), Done: true}
	}
	switch res.Status {
	case ClickPending:
		return TownPortalActionResult{Status: TownPortalActionPending}
	case ClickHit:
		a.log.Info("town portal entry clicked", "unit_id", portal.UnitID, "hover_attempts", res.Attempt)
		return TownPortalActionResult{Status: TownPortalActionClicked, Done: true}
	case ClickTooFar:
		return TownPortalActionResult{Status: TownPortalActionTooFar, Reason: string(TownPortalActionTooFar), Done: true}
	case ClickHoverNotFound:
		return TownPortalActionResult{Status: TownPortalActionHoverNotFound, Reason: string(TownPortalActionHoverNotFound), Done: true}
	case ClickProjectionFailed:
		return TownPortalActionResult{Status: TownPortalActionProjectionFailed, Reason: string(TownPortalActionProjectionFailed), Done: true}
	default:
		return TownPortalActionResult{Status: TownPortalActionInputError, Reason: string(res.Status), Done: true}
	}
}
