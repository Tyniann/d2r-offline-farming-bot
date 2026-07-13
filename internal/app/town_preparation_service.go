package app

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/pathing"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/town"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

const (
	townExecutorInputBudget  = 256
	townExecutorVerifyBudget = 6000
	townRestockVerifyTicks   = 200
)

func (a *townPreparationAdapter) start(state world.State) string {
	healing, mana := countPotionSupplies(state)
	levels := []town.RestockLevel{
		{Resource: town.RestockHealing, Current: healing, Threshold: a.thresholds.Healing, Target: len(a.profile.Healing.BeltSlots) * 4},
		{Resource: town.RestockMana, Current: mana, Threshold: a.thresholds.Mana, Target: len(a.profile.Mana.BeltSlots) * 4},
	}
	needsPotions := healing < a.thresholds.Healing || mana < a.thresholds.Mana
	if !a.services || !needsPotions {
		traversals, err := a.graph.RouteForLayout(a.layout, town.AnchorStash, nil, town.AnchorWaypoint)
		if err != nil {
			return err.Error()
		}
		a.traversals = traversals
		a.started = true
		a.log.Info("central town preparation started", "origin", "stash", "services", []string{}, "handoff", "countess", "edge_count", len(traversals), "scroll_demand", "unavailable_skip", "town_layout", a.layout)
		return ""
	}

	maximumCost, reason := town.MaximumRestockCost(levels)
	if reason != "" {
		return string(reason)
	}
	if !state.Player.GoldKnown || uint64(state.Player.Gold) < uint64(maximumCost) {
		a.log.Warn("town restock gold gate failed", "gold_known", state.Player.GoldKnown, "carried_gold", state.Player.Gold, "required_maximum", maximumCost)
		return string(town.ReasonGoldUnavailable)
	}
	beltComplete, slotsReason := completeBeltProfile(a.profile)
	if slotsReason != "" {
		return slotsReason
	}
	orders, reason := town.PlanRestock(town.RestockInput{Levels: levels, BeltLayoutComplete: beltComplete, GoldKnown: true, GoldSufficient: true})
	if reason != "" {
		return string(reason)
	}
	planner, err := town.NewPlanner(a.townCfg)
	if err != nil {
		return err.Error()
	}
	snapshot := town.InspectDemand(town.SupplySnapshot{
		Healing: healing, Mana: mana, BeltLayoutComplete: beltComplete,
		TownPortalScrolls: a.thresholds.TownPortalScrolls, IdentifyScrolls: a.thresholds.IdentifyScrolls,
	}, a.thresholds)
	plan, reason := planner.Plan(town.Origin{Act: town.OriginAct1, Anchor: town.AnchorStash}, snapshot, town.NextRunTarget{ID: "countess", Act: town.OriginAct1})
	if reason != "" {
		return string(reason)
	}
	start, required, end, reason := planner.GraphAnchors(plan)
	if reason != "" {
		return string(reason)
	}
	traversals, err := a.graph.RouteForLayout(a.layout, start, required, end)
	if err != nil {
		return err.Error()
	}
	handler := newTownPreparationStepHandler(a, traversals, orders)
	executor, err := town.NewExecutor(plan, town.Budgets{InputAttempts: townExecutorInputBudget, VerifyAttempts: townExecutorVerifyBudget, RetryAttempts: 0, TotalSteps: len(plan.Steps)}, handler, a.telemetry)
	if err != nil {
		return err.Error()
	}
	a.traversals = traversals
	a.handler = handler
	a.executor = executor
	a.started = true
	a.log.Info("central town preparation started", "origin", "stash", "services", []string{"potions"}, "handoff", "countess", "edge_count", len(traversals), "healing", healing, "mana", mana, "gold", state.Player.Gold, "required_maximum_gold", maximumCost, "town_layout", a.layout)
	return ""
}

func countPotionSupplies(state world.State) (healing, mana int) {
	for _, item := range state.ItemsByLocation(world.ItemLocationBelt) {
		switch item.Type {
		case "hpot":
			healing++
		case "mpot":
			mana++
		}
	}
	return healing, mana
}

func completeBeltProfile(profile config.ProfileResourcesConfig) (bool, string) {
	assigned := map[int]string{}
	for resource, slots := range map[string][]int{"healing": profile.Healing.BeltSlots, "mana": profile.Mana.BeltSlots, "rejuvenation": profile.Rejuvenation.BeltSlots} {
		for _, slot := range slots {
			if previous := assigned[slot]; previous != "" {
				return false, fmt.Sprintf("town belt slot %d assigned to %s and %s", slot, previous, resource)
			}
			assigned[slot] = resource
		}
	}
	return len(assigned) == 4, ""
}

type townPreparationStepHandler struct {
	adapter       *townPreparationAdapter
	traversals    []town.Traversal
	traversal     int
	anchor        town.Anchor
	walker        *pathing.TownWalker
	orders        []town.RestockOrder
	order         int
	stage         string
	npc           *town.NPCInteractor
	shop          *town.ShopOpener
	verifier      *town.RestockVerifier
	buyer         *town.VendorBuyer
	buyerActed    bool
	buyerCode     string
	buyerCost     int
	settleUntil   time.Time
	shopCloseSent bool
}

func newTownPreparationStepHandler(adapter *townPreparationAdapter, traversals []town.Traversal, orders []town.RestockOrder) *townPreparationStepHandler {
	clickCfg := adapter.pathCfg.Click
	clickCfg.AnchorOffsetTiles = 0
	clicker := pathing.NewEntityClicker(adapter.log, adapter.controller, adapter.pathCfg.Projector(), clickCfg)
	return &townPreparationStepHandler{
		adapter: adapter, traversals: append([]town.Traversal(nil), traversals...), orders: append([]town.RestockOrder(nil), orders...),
		anchor: town.AnchorStash, stage: "walk", npc: town.NewNPCInteractor(townNPCClickerAdapter{clicker: clicker}, world.Akara, 15, 8*time.Second),
		shop: town.NewShopOpener(adapter.controller, 8*time.Second),
	}
}

func (h *townPreparationStepHandler) Tick(ctx context.Context, step town.PlanStep, state world.State) town.InteractionResult {
	if h == nil || h.adapter == nil {
		return town.InteractionResult{Status: town.InteractionFailed, Reason: "town_handler_unavailable", Done: true}
	}
	switch step.Kind {
	case town.StepService:
		if step.Service != town.ServicePotions {
			return town.InteractionResult{Status: town.InteractionFailed, Reason: "town_service_unsupported", Done: true}
		}
		return h.tickPotions(ctx, state)
	case town.StepAct1Waypoint:
		return h.tickWalk(ctx, state, town.AnchorWaypoint)
	case town.StepHandoff:
		waypoint, ok := state.NearestObject(world.ObjectKindWaypoint)
		if !ok || world.Distance(state.Player.Position, waypoint.Position) > h.adapter.pathCfg.Waypoint.MaxClickDistance {
			return town.InteractionResult{Status: town.InteractionFailed, Reason: "waypoint_handoff_unconfirmed", Done: true}
		}
		return town.InteractionResult{Status: town.InteractionComplete, Done: true}
	default:
		return town.InteractionResult{Status: town.InteractionFailed, Reason: "town_step_unsupported", Done: true}
	}
}

func (h *townPreparationStepHandler) tickPotions(ctx context.Context, state world.State) town.InteractionResult {
	switch h.stage {
	case "walk":
		result := h.tickWalk(ctx, state, town.AnchorAkara)
		if result.Status != town.InteractionComplete {
			return result
		}
		h.stage = "npc"
		return town.InteractionResult{Status: town.InteractionPending}
	case "npc":
		result := h.npc.Tick(state)
		if result.Status == town.InteractionComplete {
			h.stage = "shop"
			return town.InteractionResult{Status: town.InteractionPending}
		}
		return result
	case "shop":
		result := h.shop.Tick(state)
		if result.Status == town.InteractionComplete {
			h.stage = "orders"
			return town.InteractionResult{Status: town.InteractionPending}
		}
		return result
	case "orders":
		return h.tickOrders(state)
	case "close":
		if !h.shopCloseSent {
			if err := h.adapter.controller.PressKey("esc"); err != nil {
				return town.InteractionResult{Status: town.InteractionFailed, Reason: fmt.Sprintf("town_shop_close_failed: %v", err), Done: true}
			}
			h.shopCloseSent = true
			return town.InteractionResult{Status: town.InteractionAction, Action: "shop_close", Vendor: town.AnchorAkara}
		}
		if state.UI.NPCShopOpen || state.UI.NPCInteractOpen {
			return town.InteractionResult{Status: town.InteractionPending}
		}
		h.stage = "done"
		healing, mana := countPotionSupplies(state)
		return town.InteractionResult{Status: town.InteractionComplete, Current: healing + mana, VerifiedFinal: healing + mana, Done: true}
	case "done":
		return town.InteractionResult{Status: town.InteractionComplete, Done: true}
	default:
		return town.InteractionResult{Status: town.InteractionFailed, Reason: "town_service_state_invalid", Done: true}
	}
}

func (h *townPreparationStepHandler) tickOrders(state world.State) town.InteractionResult {
	if h.order >= len(h.orders) {
		h.stage = "close"
		return town.InteractionResult{Status: town.InteractionPending}
	}
	order := h.orders[h.order]
	current := countRestockResource(state, order.Resource)
	metadata := town.InteractionResult{Current: current, Threshold: thresholdFor(h.adapter.thresholds, order.Resource), BeltSlots: slotsFor(h.adapter.profile, order.Resource), Mode: order.Mode, Vendor: town.AnchorAkara}
	if time.Now().Before(h.settleUntil) {
		metadata.Status = town.InteractionPending
		return metadata
	}
	if h.buyer != nil {
		if h.buyerActed {
			result := h.buyer.Tick(state)
			result.Current, result.Threshold, result.BeltSlots, result.Mode, result.Vendor = metadata.Current, metadata.Threshold, metadata.BeltSlots, metadata.Mode, metadata.Vendor
			result.Code, result.Cost = h.buyerCode, h.buyerCost
			if result.Status == town.InteractionComplete {
				h.buyer = nil
				h.buyerActed, h.buyerCode, h.buyerCost = false, "", 0
				return metadataWithStatus(metadata, town.InteractionPending)
			}
			return result
		}
		code, cost, ok := purchaseCostForState(state, order)
		if !ok || !state.Player.GoldKnown || uint64(state.Player.Gold) < uint64(cost) {
			metadata.Status, metadata.Reason, metadata.Done = town.InteractionFailed, string(town.ReasonGoldUnavailable), true
			return metadata
		}
		result := h.buyer.Tick(state)
		result.Current, result.Threshold, result.BeltSlots, result.Mode, result.Vendor = metadata.Current, metadata.Threshold, metadata.BeltSlots, metadata.Mode, metadata.Vendor
		result.Code, result.Cost = code, cost
		if result.Action == "vendor_buy_bulk" || result.Action == "vendor_buy_single" {
			h.buyerActed, h.buyerCode, h.buyerCost = true, code, cost
			h.settleUntil = time.Now().Add(townPurchaseSettle)
		}
		if result.Status == town.InteractionComplete {
			h.buyer = nil
			return metadataWithStatus(metadata, town.InteractionPending)
		}
		return result
	}
	if h.verifier == nil {
		verifier, err := town.NewRestockVerifier(order, townRestockVerifyTicks)
		if err != nil {
			metadata.Status, metadata.Reason, metadata.Done = town.InteractionFailed, string(town.ReasonRestockStateInvalid), true
			return metadata
		}
		h.verifier = verifier
	}
	result := h.verifier.Tick(current)
	if result.Status == town.InteractionAction {
		h.buyer = town.NewVendorBuyer(h.adapter.controller, vendorRequest(order))
		return metadataWithStatus(metadata, town.InteractionPending)
	}
	if result.Status == town.InteractionComplete {
		metadata.Status, metadata.VerifiedFinal = town.InteractionPending, current
		h.order++
		h.verifier = nil
		return metadata
	}
	result.Current, result.Threshold, result.BeltSlots, result.Mode, result.Vendor = metadata.Current, metadata.Threshold, metadata.BeltSlots, metadata.Mode, metadata.Vendor
	return result
}

func (h *townPreparationStepHandler) tickWalk(ctx context.Context, state world.State, target town.Anchor) town.InteractionResult {
	if h.anchor == target {
		return town.InteractionResult{Status: town.InteractionComplete, Done: true}
	}
	if h.traversal >= len(h.traversals) {
		return town.InteractionResult{Status: town.InteractionFailed, Reason: string(town.ReasonTownLayoutRouteMissing), Done: true}
	}
	traversal := h.traversals[h.traversal]
	destination := traversal.Edge.To
	if traversal.Reverse {
		destination = traversal.Edge.From
	}
	if h.walker == nil {
		points, err := pathing.LoadLayoutBoundTownRoute(filepath.Join(h.adapter.directory, traversal.Edge.Route), traversal.Edge.ID, h.adapter.layout, h.adapter.layoutOrigin)
		if err != nil {
			return town.InteractionResult{Status: town.InteractionFailed, Reason: err.Error(), Done: true}
		}
		if traversal.Reverse {
			reversePositions(points)
		}
		if h.traversal == 0 && world.Distance(state.Player.Position, points[0]) > h.adapter.pathCfg.TownWalk.ArrivalDistance {
			return town.InteractionResult{Status: town.InteractionFailed, Reason: "town_edge_start_unconfirmed", Done: true}
		}
		h.walker = pathing.NewTownRouteWalker(h.adapter.log, h.adapter.driver, h.adapter.pathCfg, points)
		h.adapter.log.Info("central town edge started", "edge", traversal.Edge.ID, "index", h.traversal)
	}
	result := h.walker.TickRoute(ctx, state)
	if !result.Done {
		return town.InteractionResult{Status: town.InteractionPending}
	}
	if result.Status != pathing.TownWalkArrived {
		return town.InteractionResult{Status: town.InteractionFailed, Reason: result.Reason, Done: true}
	}
	h.adapter.log.Info("central town edge completed", "edge", traversal.Edge.ID, "index", h.traversal)
	h.walker = nil
	h.traversal++
	h.anchor = destination
	if h.anchor == target {
		return town.InteractionResult{Status: town.InteractionComplete, Done: true}
	}
	return town.InteractionResult{Status: town.InteractionPending}
}

func (h *townPreparationStepHandler) ResetStep() {
	if h == nil {
		return
	}
	if h.npc != nil {
		h.npc.Reset()
	}
	h.verifier, h.buyer = nil, nil
	h.buyerActed, h.buyerCode, h.buyerCost = false, "", 0
	h.settleUntil, h.shopCloseSent = time.Time{}, false
	h.stage = "walk"
}

func (h *townPreparationStepHandler) Reset() {
	if h == nil {
		return
	}
	if h.walker != nil {
		h.walker.Reset()
	}
	h.ResetStep()
	h.traversal, h.order, h.anchor = 0, 0, town.AnchorStash
	h.walker = nil
}

func countRestockResource(state world.State, resource town.RestockResource) int {
	healing, mana := countPotionSupplies(state)
	if resource == town.RestockHealing {
		return healing
	}
	if resource == town.RestockMana {
		return mana
	}
	return 0
}

func vendorRequest(order town.RestockOrder) town.VendorRequest {
	typeCode := "hpot"
	if order.Resource == town.RestockMana {
		typeCode = "mpot"
	}
	return town.VendorRequest{Type: typeCode, Mode: order.Mode}
}

func purchaseCostForState(state world.State, order town.RestockOrder) (string, int, bool) {
	request := vendorRequest(order)
	var selected world.Item
	found := false
	for _, item := range state.ItemsByLocation(world.ItemLocationVendor) {
		if item.Type != request.Type {
			continue
		}
		if !found || item.TxtFileNo > selected.TxtFileNo {
			selected, found = item, true
		}
	}
	unit, ok := town.AkaraVendorCost(selected.Code)
	if !found || !ok {
		return "", 0, false
	}
	units := 1
	if order.Mode == town.BuyModeBulk {
		units = order.Target - countRestockResource(state, order.Resource)
	}
	if units <= 0 {
		return "", 0, false
	}
	return selected.Code, unit * units, true
}

func thresholdFor(thresholds town.Thresholds, resource town.RestockResource) int {
	if resource == town.RestockHealing {
		return thresholds.Healing
	}
	if resource == town.RestockMana {
		return thresholds.Mana
	}
	return 0
}

func slotsFor(profile config.ProfileResourcesConfig, resource town.RestockResource) []int {
	if resource == town.RestockHealing {
		return append([]int(nil), profile.Healing.BeltSlots...)
	}
	if resource == town.RestockMana {
		return append([]int(nil), profile.Mana.BeltSlots...)
	}
	return nil
}

func metadataWithStatus(result town.InteractionResult, status town.InteractionStatus) town.InteractionResult {
	result.Status = status
	return result
}
