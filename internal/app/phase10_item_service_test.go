package app

import (
	"testing"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/loot"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/town"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

func TestTownPreparationClassifiesOnlyUnlockedMephistoSellCandidates(t *testing.T) {
	pickup, err := loot.CompilePickitRules("test", []loot.PickitRuleSpec{
		{ProfileID: "gems", RuleID: "gem", Action: loot.ActionKeep, Expression: `[name] == gpv`},
		{ProfileID: "mephisto-standard", RuleID: "candidate", Action: loot.ActionSell, Expression: `([quality] == set || [quality] == unique) && ([tier] == exceptional || [tier] == elite)`},
	})
	if err != nil {
		t.Fatal(err)
	}
	grid := [][]int{{0, 0, 0, 0, 0, 0, 0, 0, 0, 0}, {0, 0, 0, 0, 0, 0, 0, 0, 0, 0}, {0, 0, 0, 0, 0, 0, 0, 0, 0, 0}, {0, 0, 0, 0, 0, 0, 0, 0, 0, 0}}
	lock, _ := loot.NewInventoryLock(grid)
	adapter := &townPreparationAdapter{lootFilter: loot.NewFilter(config.NewLogger("error"), lock, pickup)}
	item := world.Item{UnitID: 31, Code: "xap", Quality: world.ItemQualityUnique, BaseTier: world.BaseTierExceptional, Location: world.ItemLocationInventory, PlayerOwned: true, Width: 2, Height: 2}
	orders, reason := adapter.planItemServiceOrders(world.State{Items: []world.Item{item}})
	if reason != "" || len(orders) != 2 || orders[0].Kind != town.ItemServiceIdentify || orders[1].Kind != town.ItemServiceSell || orders[0].UnitID != orders[1].UnitID {
		t.Fatalf("orders=%+v reason=%s", orders, reason)
	}
	item.Identified = true
	orders, reason = adapter.planItemServiceOrders(world.State{Items: []world.Item{item}})
	if reason != "" || len(orders) != 1 || orders[0].Kind != town.ItemServiceSell {
		t.Fatalf("identified orders=%+v reason=%s", orders, reason)
	}
	orders, reason = adapter.planItemServiceOrders(world.State{})
	if reason != "" || len(orders) != 0 {
		t.Fatalf("missing orders=%+v reason=%s", orders, reason)
	}
	grid[0][0] = 1
	lock, _ = loot.NewInventoryLock(grid)
	adapter.lootFilter = loot.NewFilter(config.NewLogger("error"), lock, pickup)
	item.GridX, item.GridY = 0, 0
	if _, reason = adapter.planItemServiceOrders(world.State{Items: []world.Item{item}}); reason != town.ReasonItemClassificationInvalid {
		t.Fatalf("locked candidate reason=%s", reason)
	}
}

func TestTownItemServiceInputGatesCainAkaraAndClosesUI(t *testing.T) {
	in := &preparationInputMock{}
	cfg := config.LootStashConfig{InventoryLeft: 847, InventoryTop: 369, InventoryCellW: 33, InventoryCellH: 33}
	item := world.Item{UnitID: 41, Code: "xap", Identified: false, Location: world.ItemLocationInventory, PlayerOwned: true, GridX: 2, GridY: 1}
	service := &townItemServiceInput{controller: in, cfg: cfg}
	service.bind(world.State{Valid: true, UI: world.UIState{NPCInteractOpen: true}, Items: []world.Item{item}})
	if err := service.Identify(item.UnitID); err != nil || in.keys != 3 || len(in.pressed) != 3 || in.pressed[0] != "home" || in.pressed[1] != "down" || in.pressed[2] != "enter" {
		t.Fatalf("identify err=%v keys=%v", err, in.pressed)
	}
	item.Identified = true
	service.bind(world.State{Valid: true, UI: world.UIState{NPCShopOpen: true}, Items: []world.Item{item}})
	if err := service.Sell(item.UnitID); err != nil || in.moves != 1 || in.modified != 1 {
		t.Fatalf("sell err=%v moves=%d modified=%d", err, in.moves, in.modified)
	}
	handler := &townPreparationStepHandler{adapter: &townPreparationAdapter{controller: in}}
	if result := handler.tickCloseUI(world.State{UI: world.UIState{NPCShopOpen: true}}, town.AnchorAkara); result.Action != "shop_close" || in.keys != 4 {
		t.Fatalf("close action=%+v keys=%v", result, in.pressed)
	}
	if result := handler.tickCloseUI(world.State{}, town.AnchorAkara); result.Status != town.InteractionComplete {
		t.Fatalf("close verification=%+v", result)
	}
}

func TestTownItemServiceHandlerPreservesSellOrderAfterIdentify(t *testing.T) {
	in := &preparationInputMock{}
	policy, err := loot.CompilePickitRules("test", []loot.PickitRuleSpec{{ProfileID: "mephisto", RuleID: "sell", Action: loot.ActionSell, Expression: `[name] == "xap"`}})
	if err != nil {
		t.Fatal(err)
	}
	lock, err := loot.NewInventoryLock([][]int{make([]int, 10), make([]int, 10), make([]int, 10), make([]int, 10)})
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.LootStashConfig{InventoryLeft: 847, InventoryTop: 369, InventoryCellW: 33, InventoryCellH: 33}
	handler := &townPreparationStepHandler{
		adapter: &townPreparationAdapter{controller: in, lootFilter: loot.NewFilter(config.NewLogger("error"), lock, policy)},
		itemOrders: orderedItemServiceOrders([]town.ItemServiceOrder{
			{Kind: town.ItemServiceIdentify, UnitID: 51, Code: "xap"},
			{Kind: town.ItemServiceSell, UnitID: 51, Code: "xap"},
		}),
		itemInput: &townItemServiceInput{controller: in, cfg: cfg},
		stage:     "items",
	}
	item := world.Item{UnitID: 51, Code: "xap", Location: world.ItemLocationInventory, PlayerOwned: true, Page: 0, GridX: 4, GridY: 1}
	cain := world.State{Valid: true, UI: world.UIState{NPCInteractOpen: true}, Items: []world.Item{item}}
	if got := handler.tickItemOrders(cain, town.ItemServiceIdentify, town.AnchorCain); got.Action != "item_identify" {
		t.Fatalf("identify action=%+v", got)
	}
	item.Identified = true
	cain.Items = []world.Item{item}
	if got := handler.tickItemOrders(cain, town.ItemServiceIdentify, town.AnchorCain); got.Status != town.InteractionPending || handler.itemOrder != 1 {
		t.Fatalf("identify verify=%+v cursor=%d", got, handler.itemOrder)
	}
	if got := handler.tickItemOrders(cain, town.ItemServiceIdentify, town.AnchorCain); got.Status != town.InteractionPending || handler.stage != "close" || handler.itemOrder != 1 {
		t.Fatalf("identify boundary=%+v stage=%s cursor=%d", got, handler.stage, handler.itemOrder)
	}
	handler.ResetStep()
	handler.stage = "items"
	akara := world.State{Valid: true, UI: world.UIState{NPCShopOpen: true}, Items: []world.Item{item}}
	if got := handler.tickItemOrders(akara, town.ItemServiceSell, town.AnchorAkara); got.Action != "item_sell" || in.modified != 1 {
		t.Fatalf("sell action=%+v modified=%d", got, in.modified)
	}
	akara.Items = nil
	if got := handler.tickItemOrders(akara, town.ItemServiceSell, town.AnchorAkara); got.Status != town.InteractionPending || handler.itemOrder != 2 {
		t.Fatalf("sell verify=%+v cursor=%d", got, handler.itemOrder)
	}
}

func TestTownSellRechecksIdentityAndRevokesDriftedMatch(t *testing.T) {
	in := &preparationInputMock{}
	policy, err := loot.CompilePickitRules("drift", []loot.PickitRuleSpec{{ProfileID: "unique", RuleID: "shako", Action: loot.ActionSell, Expression: `[uniqueitem] == "Harlequin Crest"`, ProfileRevision: 3, AssignmentRevision: 5}})
	if err != nil {
		t.Fatal(err)
	}
	lock, _ := loot.NewInventoryLock([][]int{make([]int, 10), make([]int, 10), make([]int, 10), make([]int, 10)})
	handler := &townPreparationStepHandler{adapter: &townPreparationAdapter{controller: in, lootFilter: loot.NewFilter(config.NewLogger("error"), lock, policy)}, itemOrders: orderedItemServiceOrders([]town.ItemServiceOrder{{Kind: town.ItemServiceIdentify, UnitID: 71, Code: "uap"}, {Kind: town.ItemServiceSell, UnitID: 71, Code: "uap"}}), itemInput: &townItemServiceInput{controller: in, cfg: config.LootStashConfig{InventoryLeft: 847, InventoryTop: 369, InventoryCellW: 33, InventoryCellH: 33}}, stage: "items"}
	item := world.Item{UnitID: 71, Code: "uap", Location: world.ItemLocationInventory, PlayerOwned: true, Page: 0, IdentityKind: world.ItemIdentityUnique, IdentityKey: "Harlequin Crest", IdentityAvailable: true, IdentityValid: true}
	state := world.State{Valid: true, UI: world.UIState{NPCInteractOpen: true}, Items: []world.Item{item}}
	if got := handler.tickItemOrders(state, town.ItemServiceIdentify, town.AnchorCain); got.Action != "item_identify" || got.ProfileRevision != 3 {
		t.Fatalf("identify = %+v", got)
	}
	item.Identified, item.IdentityValid = true, false
	state.Items = []world.Item{item}
	if got := handler.tickItemOrders(state, town.ItemServiceIdentify, town.AnchorCain); got.Status != town.InteractionPending {
		t.Fatalf("identify verify = %+v", got)
	}
	handler.ResetStep()
	handler.stage = "items"
	if got := handler.tickItemOrders(state, town.ItemServiceSell, town.AnchorAkara); got.Reason != "pickit_recheck_no_match" || in.modified != 0 {
		t.Fatalf("drifted sell = %+v modified=%d", got, in.modified)
	}
}

func TestItemServicesAcceptancePreflightRejectsMissingOrPartialCandidate(t *testing.T) {
	valid := []town.ItemServiceOrder{{Kind: town.ItemServiceIdentify, UnitID: 61, Code: "xap"}, {Kind: town.ItemServiceSell, UnitID: 61, Code: "xap"}}
	if err := validateItemServicesAcceptanceOrders(valid); err != nil {
		t.Fatal(err)
	}
	if err := validateItemServicesAcceptanceOrders([]town.ItemServiceOrder{{Kind: town.ItemServiceSell, UnitID: 61, Code: "xap"}}); err != nil {
		t.Fatalf("identified resume rejected: %v", err)
	}
	for _, orders := range [][]town.ItemServiceOrder{
		nil,
		{{Kind: town.ItemServiceIdentify, UnitID: 61, Code: "xap"}},
		{{Kind: town.ItemServiceIdentify, UnitID: 61, Code: "xap"}, {Kind: town.ItemServiceSell, UnitID: 62, Code: "xap"}},
	} {
		if err := validateItemServicesAcceptanceOrders(orders); err == nil {
			t.Fatalf("invalid acceptance orders allowed: %+v", orders)
		}
	}
}
