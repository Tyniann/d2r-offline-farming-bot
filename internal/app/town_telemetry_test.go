package app

import (
	"testing"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/telemetry"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/town"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

type townTelemetryMock struct {
	events []telemetry.Event
	err    error
}

func TestTownTelemetryAdapterMapsVerifiedSellIdentity(t *testing.T) {
	m := &townTelemetryMock{}
	err := (townTelemetryAdapter{emitter: m}).EmitTown(town.ExecutorEvent{
		Event: string(telemetry.SellSuccess), VendorUnitID: 77, Vendor: town.AnchorAkara,
		Code: "uap", Name: "Harlequin Crest", Quality: world.ItemQualityUnique,
		IdentityKind: world.ItemIdentityUnique, IdentityKey: "Harlequin Crest", IdentityValid: true,
		ProfileID: "mephisto", RuleID: "sell-shako", PickitAction: "sell", ProfileRevision: 3, AssignmentRevision: 9,
	})
	if err != nil {
		t.Fatal(err)
	}
	got := m.events[0]
	if got.Event != telemetry.SellSuccess || got.Stage != telemetry.HistoryStageReturnTown || got.ItemKey != "unique:Harlequin Crest" || got.ItemIdentityKey != "Harlequin Crest" || got.PickitRuleID != "sell-shako" {
		t.Fatalf("sell event=%+v", got)
	}
}

func TestTownTelemetryAdapterMapsTrashSellWithoutPickitAction(t *testing.T) {
	m := &townTelemetryMock{}
	err := (townTelemetryAdapter{emitter: m}).EmitTown(town.ExecutorEvent{
		Event: string(telemetry.TrashSellSuccess), VendorUnitID: 81, Vendor: town.AnchorAkara,
		Code: "8ws", Name: "War Sword", Quality: world.ItemQualityNormal,
	})
	if err != nil {
		t.Fatal(err)
	}
	got := m.events[0]
	if got.Event != telemetry.TrashSellSuccess || got.Stage != telemetry.HistoryStageReturnTown || got.PickitAction != "" || got.UnitID != 81 {
		t.Fatalf("trash sell event=%+v", got)
	}
}

func (m *townTelemetryMock) Emit(e telemetry.Event) error {
	m.events = append(m.events, e)
	return m.err
}

func TestTownTelemetryAdapterMapsDecisionContext(t *testing.T) {
	m := &townTelemetryMock{}
	a := townTelemetryAdapter{emitter: m}
	err := a.EmitTown(town.ExecutorEvent{Event: "town_action", Step: 2, Kind: town.StepService, Service: town.ServicePotions, Action: "vendor_buy_bulk", Current: 1, Threshold: 2, BeltSlots: []int{1}, Mode: town.BuyModeBulk, VendorUnitID: 99, Vendor: town.AnchorAkara, Cost: 120, VerifiedFinal: 4, ProfileID: "mephisto", RuleID: "sell", PickitAction: "sell", ProfileRevision: 2, AssignmentRevision: 6})
	if err != nil {
		t.Fatal(err)
	}
	got := m.events[0]
	if got.Event != telemetry.TownAction || got.TownStep == nil || *got.TownStep != 2 || got.CurrentCount == nil || *got.CurrentCount != 1 || got.PurchaseMode != "bulk" || got.UnitID != 99 || len(got.BeltSlots) != 1 || got.PickitProfileID != "mephisto" || got.PickitRuleID != "sell" || got.PickitProfileRevision != 2 || got.PickitAssignmentRevision != 6 {
		t.Fatalf("event=%+v", got)
	}
}

func TestTownTelemetryAdapterFailsClosedWithoutEmitter(t *testing.T) {
	if err := (townTelemetryAdapter{}).EmitTown(town.ExecutorEvent{Event: "town_action"}); err == nil {
		t.Fatal("missing emitter accepted")
	}
}
