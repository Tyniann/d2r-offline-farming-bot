package app

import (
	"testing"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/telemetry"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/town"
)

type townTelemetryMock struct {
	events []telemetry.Event
	err    error
}

func (m *townTelemetryMock) Emit(e telemetry.Event) error {
	m.events = append(m.events, e)
	return m.err
}

func TestTownTelemetryAdapterMapsDecisionContext(t *testing.T) {
	m := &townTelemetryMock{}
	a := townTelemetryAdapter{emitter: m}
	err := a.EmitTown(town.ExecutorEvent{Event: "town_action", Step: 2, Kind: town.StepService, Service: town.ServicePotions, Action: "vendor_buy_bulk", Current: 1, Threshold: 2, BeltSlots: []int{1}, Mode: town.BuyModeBulk, VendorUnitID: 99, Vendor: town.AnchorAkara, Cost: 120, VerifiedFinal: 4})
	if err != nil {
		t.Fatal(err)
	}
	got := m.events[0]
	if got.Event != telemetry.TownAction || got.TownStep == nil || *got.TownStep != 2 || got.CurrentCount == nil || *got.CurrentCount != 1 || got.PurchaseMode != "bulk" || got.UnitID != 99 || len(got.BeltSlots) != 1 {
		t.Fatalf("event=%+v", got)
	}
}

func TestTownTelemetryAdapterFailsClosedWithoutEmitter(t *testing.T) {
	if err := (townTelemetryAdapter{}).EmitTown(town.ExecutorEvent{Event: "town_action"}); err == nil {
		t.Fatal("missing emitter accepted")
	}
}
