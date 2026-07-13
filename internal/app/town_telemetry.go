package app

import (
	"fmt"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/telemetry"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/town"
)

type townTelemetryEmitter interface {
	Emit(telemetry.Event) error
}

type townTelemetryAdapter struct{ emitter townTelemetryEmitter }

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
	name := telemetry.EventName(event.Event)
	if name != telemetry.TownAction && name != telemetry.TownStepCompleted {
		return fmt.Errorf("unsupported town telemetry event %q", event.Event)
	}
	step, current, threshold, cost, verified := event.Step, event.Current, event.Threshold, event.Cost, event.VerifiedFinal
	return a.emitter.Emit(telemetry.Event{
		Event: name, Reason: event.Reason, UnitID: event.VendorUnitID, Decision: event.Action,
		TownStep: &step, TownKind: string(event.Kind), TownService: string(event.Service),
		CurrentCount: &current, TriggerThreshold: &threshold, BeltSlots: append([]int(nil), event.BeltSlots...),
		PurchaseMode: string(event.Mode), Vendor: string(event.Vendor), Cost: &cost, VerifiedFinalCount: &verified,
	})
}
