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
		{UnitID: 1, Code: "rin", IdentifyRequired: true, VendorCandidate: true},
		{UnitID: 2, Code: "amu", VendorCandidate: true},
	}
	orders, reason := PlanItemServices(candidates)
	if reason != "" || len(orders) != 3 || orders[0].Kind != ItemServiceIdentify || orders[1].Kind != ItemServiceSell || orders[0].UnitID != orders[1].UnitID || orders[2].Kind != ItemServiceSell {
		t.Fatalf("orders=%+v reason=%s", orders, reason)
	}
	for _, conflict := range []ItemServiceCandidate{
		{UnitID: 3, VendorCandidate: true, Keep: true},
		{UnitID: 4, VendorCandidate: true, Stash: true},
		{UnitID: 5, VendorCandidate: true, InventoryLocked: true},
	} {
		if _, reason := PlanItemServices([]ItemServiceCandidate{conflict}); reason != ReasonItemClassificationInvalid {
			t.Fatalf("conflict %+v reason=%s", conflict, reason)
		}
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

func TestPlanItemServicesMissingAndIdentifiedCandidate(t *testing.T) {
	orders, reason := PlanItemServices(nil)
	if reason != "" || len(orders) != 0 {
		t.Fatalf("missing candidate orders=%+v reason=%s", orders, reason)
	}
	orders, reason = PlanItemServices([]ItemServiceCandidate{{UnitID: 12, Code: "xap", VendorCandidate: true}})
	if reason != "" || len(orders) != 1 || orders[0].Kind != ItemServiceSell {
		t.Fatalf("identified candidate orders=%+v reason=%s", orders, reason)
	}
}

func TestItemServiceExecutorAcceptsCainAlreadyIdentifiedWithoutInput(t *testing.T) {
	in := &itemServiceInputMock{}
	exec, _ := NewItemServiceExecutor(in, ItemServiceOrder{Kind: ItemServiceIdentify, UnitID: 11, Code: "rin"}, 2)
	item := world.Item{UnitID: 11, Code: "rin", Location: world.ItemLocationInventory, PlayerOwned: true, Identified: true}
	if got := exec.Tick(itemServiceState(item)); got.Status != InteractionComplete || len(in.identified) != 0 {
		t.Fatalf("already identified result=%+v input=%v", got, in.identified)
	}
}
