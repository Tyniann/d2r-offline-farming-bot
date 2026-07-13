package town

import (
	"testing"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

type itemServiceInputMock struct{ identified, sold []uint32 }

func (m *itemServiceInputMock) Identify(id uint32) error {
	m.identified = append(m.identified, id)
	return nil
}
func (m *itemServiceInputMock) Sell(id uint32) error { m.sold = append(m.sold, id); return nil }

func itemServiceState(items ...world.Item) world.State { return world.State{Valid: true, Items: items} }

func TestPlanItemServicesMatrixProtectsKeepStashAndLock(t *testing.T) {
	candidates := []ItemServiceCandidate{
		{UnitID: 1, Code: "rin", IdentifyRequired: true},
		{UnitID: 2, Code: "amu", VendorCandidate: true},
		{UnitID: 3, VendorCandidate: true, Keep: true},
		{UnitID: 4, VendorCandidate: true, Stash: true},
		{UnitID: 5, VendorCandidate: true, InventoryLocked: true},
	}
	orders, reason := PlanItemServices(candidates)
	if reason != "" || len(orders) != 2 || orders[0].Kind != ItemServiceIdentify || orders[1].Kind != ItemServiceSell {
		t.Fatalf("orders=%+v reason=%s", orders, reason)
	}
	candidates[0].VendorCandidate = true
	if _, reason := PlanItemServices(candidates); reason != ReasonItemClassificationInvalid {
		t.Fatalf("conflict reason=%s", reason)
	}
}

func TestItemServiceExecutorPinsAndVerifiesIdentifyAndSell(t *testing.T) {
	in := &itemServiceInputMock{}
	item := world.Item{UnitID: 11, Code: "rin", Location: world.ItemLocationInventory, PlayerOwned: true, Page: 0}
	identify, _ := NewItemServiceExecutor(in, ItemServiceOrder{Kind: ItemServiceIdentify, UnitID: 11, Code: "rin"}, 2)
	if got := identify.Tick(itemServiceState(item)); got.Action != "item_identify" || len(in.identified) != 1 {
		t.Fatalf("identify action=%+v", got)
	}
	item.Identified = true
	if got := identify.Tick(itemServiceState(item)); got.Status != InteractionComplete {
		t.Fatalf("identify complete=%+v", got)
	}
	sell, _ := NewItemServiceExecutor(in, ItemServiceOrder{Kind: ItemServiceSell, UnitID: 11, Code: "rin"}, 2)
	item.Identified = false
	if got := sell.Tick(itemServiceState(item)); got.Action != "item_sell" || len(in.sold) != 1 {
		t.Fatalf("sell action=%+v", got)
	}
	if got := sell.Tick(itemServiceState()); got.Status != InteractionComplete {
		t.Fatalf("sell complete=%+v", got)
	}
}

func TestItemServiceExecutorRejectsWrongPinAndTimesOutWithoutRepeat(t *testing.T) {
	in := &itemServiceInputMock{}
	exec, _ := NewItemServiceExecutor(in, ItemServiceOrder{Kind: ItemServiceSell, UnitID: 11, Code: "rin"}, 1)
	wrong := world.Item{UnitID: 11, Code: "amu", Location: world.ItemLocationInventory, PlayerOwned: true}
	if got := exec.Tick(itemServiceState(wrong)); got.Reason != string(ReasonItemPinInvalid) {
		t.Fatalf("wrong pin=%+v", got)
	}
	exec, _ = NewItemServiceExecutor(in, ItemServiceOrder{Kind: ItemServiceSell, UnitID: 11, Code: "rin"}, 1)
	item := world.Item{UnitID: 11, Code: "rin", Location: world.ItemLocationInventory, PlayerOwned: true}
	_ = exec.Tick(itemServiceState(item))
	if got := exec.Tick(itemServiceState(item)); got.Reason != string(ReasonItemVerifyTimeout) || len(in.sold) != 1 {
		t.Fatalf("timeout=%+v sold=%v", got, in.sold)
	}
}
