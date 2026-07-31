package app

import (
	"fmt"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/telemetry"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/town"
)

// townTelemetryEmitter is the session-owned sink. Town code sees only its
// narrower synchronous adapter and cannot emit unrelated lifecycle events.
type townTelemetryEmitter interface {
	Emit(telemetry.Event) error
}

// townTelemetryAdapter maps the stable Town executor contract onto run JSONL.
type townTelemetryAdapter struct{ emitter townTelemetryEmitter }

// townTelemetryRelay keeps the long-lived Town adapter reusable while each run
// installs its own recorder. A nil sink fails closed instead of dropping events.
type townTelemetryRelay struct{ emitter townTelemetryEmitter }

func (r *townTelemetryRelay) setTelemetry(emitter townTelemetryEmitter) {
	if r != nil {
		r.emitter = emitter
	}
}

func (r *townTelemetryRelay) EmitTown(event town.ExecutorEvent) error {
	if r == nil {
		return fmt.Errorf("town telemetry relay unavailable")
	}
	return (townTelemetryAdapter{emitter: r.emitter}).EmitTown(event)
}

func (a townTelemetryAdapter) EmitTown(event town.ExecutorEvent) error {
	if a.emitter == nil {
		return fmt.Errorf("town telemetry unavailable")
	}
	// Reject arbitrary event names so executor progress cannot be disguised as
	// another telemetry category by a malformed handler result.
	name := telemetry.EventName(event.Event)
	switch name {
	case telemetry.TownAction, telemetry.TownStepCompleted, telemetry.SellSuccess,
		telemetry.TownMercenaryHealRequested, telemetry.TownMercenaryHealConfirmed,
		telemetry.TownMercenaryReviveRequested, telemetry.TownMercenaryReviveConfirmed:
	default:
		return fmt.Errorf("unsupported town telemetry event %q", event.Event)
	}
	step, current, threshold, cost, verified := event.Step, event.Current, event.Threshold, event.Cost, event.VerifiedFinal
	return a.emitter.Emit(telemetry.Event{
		Event: name, Stage: telemetry.HistoryStageReturnTown, Reason: event.Reason, UnitID: event.VendorUnitID, Decision: event.Action,
		TownStep: &step, TownKind: string(event.Kind), TownService: string(event.Service),
		CurrentCount: &current, TriggerThreshold: &threshold, BeltSlots: append([]int(nil), event.BeltSlots...),
		PurchaseMode: string(event.Mode), Vendor: string(event.Vendor), Cost: &cost, VerifiedFinalCount: &verified,
		PickitProfileID: event.ProfileID, PickitRuleID: event.RuleID, PickitAction: event.PickitAction,
		PickitProfileRevision: event.ProfileRevision, PickitAssignmentRevision: event.AssignmentRevision,
		Code: event.Code, Name: event.Name,
		ItemName: event.Name, BaseCode: event.Code, Quality: event.Quality.String(),
		ItemIdentityKind: string(event.IdentityKind),
		ItemIdentityKey: func() string {
			if event.IdentityValid {
				return event.IdentityKey
			}
			return ""
		}(),
		ItemKey:    itemTelemetryKey(event.Code, event.Quality, event.IdentityKind, event.IdentityKey, event.IdentityValid),
		MercUnitID: event.MercUnitID, HPBefore: event.HPBefore, HPAfter: event.HPAfter,
	})
}
