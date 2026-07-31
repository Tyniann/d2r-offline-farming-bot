package app

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/input"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/loot"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/pathing"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/telemetry"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/town"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

const (
	// Budgets are global executor ceilings, not timing knobs. Lower-level gates
	// retain their own finite limits and the executor remains the final backstop.
	townExecutorInputBudget    = 256
	townExecutorVerifyBudget   = 6000
	townRestockVerifyTicks     = 200
	townMercenaryVerifyTimeout = 8 * time.Second
)

func (a *townPreparationAdapter) mercenaryPolicy() town.MercenaryPolicy {
	enabled, rule := a.profile.Mercenary.Resolve()
	return town.MercenaryPolicy{Enabled: enabled, ThresholdPercent: rule.UseBelowPercent}
}

func (a *townPreparationAdapter) start(state world.State) string {
	// Planning consumes one coherent snapshot. It must not mix belt or carried-
	// gold values from later ticks before the immutable plan is constructed.
	healing, mana := countPotionSupplies(state)
	levels := []town.RestockLevel{
		{Resource: town.RestockHealing, Current: healing, Threshold: a.thresholds.Healing, Target: len(a.profile.Healing.BeltSlots) * 4},
		{Resource: town.RestockMana, Current: mana, Threshold: a.thresholds.Mana, Target: len(a.profile.Mana.BeltSlots) * 4},
	}
	needsPotions := healing < a.thresholds.Healing || mana < a.thresholds.Mana
	itemOrders, itemReason := a.planItemServiceOrders(state)
	if itemReason != "" {
		return string(itemReason)
	}
	needsIdentify, needsSell := itemServiceDemand(itemOrders)
	mercHeal, mercRevive := false, false
	if a.services {
		var mercFail town.Reason
		mercHeal, mercRevive, mercFail = town.EvaluateMercenaryTownDemand(a.mercenaryPolicy(), state.Mercenary)
		if mercFail != "" {
			return string(mercFail)
		}
	}
	if !a.services || (!needsPotions && len(itemOrders) == 0 && !mercHeal && !mercRevive) {
		// No demand means no NPC detour. Initial run setup also enters here even
		// with a low belt because its only responsibility is reaching Waypoint.
		startAnchor := a.startAnchor
		if startAnchor == "" {
			startAnchor = town.AnchorStash
		}
		traversals, err := a.graph.RouteForLayout(a.layout, startAnchor, nil, town.AnchorWaypoint)
		if err != nil {
			return err.Error()
		}
		a.traversals = traversals
		a.started = true
		a.log.Info("central town preparation started", "origin", startAnchor, "services", []string{}, "handoff", a.nextRunID, "edge_count", len(traversals), "scroll_demand", "unavailable_skip", "town_layout", a.layout)
		return ""
	}

	maximumCost := 0
	beltComplete := true
	restockOrders := []town.RestockOrder(nil)
	if needsPotions {
		var reason town.Reason
		maximumCost, reason = town.MaximumRestockCost(levels)
		if reason != "" {
			return string(reason)
		}
		// Vendors consume carried gold. Private/shared Stash gold is intentionally
		// excluded because Phase 9 has no verified withdrawal transaction.
		if !state.Player.GoldKnown || uint64(state.Player.Gold) < uint64(maximumCost) {
			a.log.Warn("town restock gold gate failed", "gold_known", state.Player.GoldKnown, "carried_gold", state.Player.Gold, "required_maximum", maximumCost)
			return string(town.ReasonGoldUnavailable)
		}
		var slotsReason string
		beltComplete, slotsReason = completeBeltProfile(a.profile)
		if slotsReason != "" {
			return slotsReason
		}
		restockOrders, reason = town.PlanRestock(town.RestockInput{Levels: levels, BeltLayoutComplete: beltComplete, GoldKnown: true, GoldSufficient: true})
		if reason != "" {
			return string(reason)
		}
	}
	planner, err := town.NewPlanner(a.townCfg)
	if err != nil {
		return err.Error()
	}
	snapshot := town.InspectDemand(town.SupplySnapshot{
		Healing: healing, Mana: mana, BeltLayoutComplete: beltComplete,
		TownPortalScrolls: a.thresholds.TownPortalScrolls, IdentifyScrolls: a.thresholds.IdentifyScrolls,
		IdentifyRequired: needsIdentify, VendorCandidates: needsSell,
		MercenaryHeal: mercHeal, MercenaryRevive: mercRevive,
	}, a.thresholds)
	plan, reason := planner.Plan(town.Origin{Act: town.OriginAct1, Anchor: town.AnchorStash}, snapshot, town.NextRunTarget{ID: a.nextRunID, Act: town.OriginAct1})
	if reason != "" {
		return string(reason)
	}
	start, required, end, reason := planner.GraphAnchorSequence(plan)
	if reason != "" {
		return string(reason)
	}
	traversals, err := a.graph.RouteOrderedForLayout(a.layout, start, required, end)
	if err != nil {
		return err.Error()
	}
	handler := newTownPreparationStepHandler(a, traversals, restockOrders, itemOrders)
	executor, err := town.NewExecutor(plan, town.Budgets{InputAttempts: townExecutorInputBudget, VerifyAttempts: townExecutorVerifyBudget, RetryAttempts: 0, TotalSteps: len(plan.Steps)}, handler, a.telemetry)
	if err != nil {
		return err.Error()
	}
	a.traversals = traversals
	a.handler = handler
	a.executor = executor
	a.started = true
	a.log.Info("central town preparation started", "origin", "stash", "potions", needsPotions, "identify", needsIdentify, "sell", needsSell, "mercenary_heal", mercHeal, "mercenary_revive", mercRevive, "item_orders", len(itemOrders), "handoff", a.nextRunID, "edge_count", len(traversals), "healing", healing, "mana", mana, "gold", state.Player.Gold, "required_maximum_gold", maximumCost, "town_layout", a.layout)
	return ""
}

func (a *townPreparationAdapter) planItemServiceOrders(state world.State) ([]town.ItemServiceOrder, town.Reason) {
	if a == nil || a.lootFilter == nil {
		return nil, ""
	}
	candidates := make([]town.ItemServiceCandidate, 0)
	for _, item := range state.InventoryItems() {
		result := a.lootFilter.Evaluate(item)
		if !result.Matched || result.Action != loot.ActionSell {
			continue
		}
		locked := a.lootFilter == nil || a.lootFilter.InventoryLocked(item)
		candidates = append(candidates, town.ItemServiceCandidate{
			UnitID: item.UnitID, Code: item.Code, Name: item.Name, Quality: item.Quality,
			IdentityKind: item.IdentityKind, IdentityKey: item.IdentityKey, IdentityValid: item.IdentityValid,
			IdentifyRequired: !item.Identified,
			VendorCandidate:  true, InventoryLocked: locked,
		})
	}
	return town.PlanItemServices(candidates)
}

func itemServiceDemand(orders []town.ItemServiceOrder) (identify, sell bool) {
	for _, order := range orders {
		identify = identify || order.Kind == town.ItemServiceIdentify
		sell = sell || order.Kind == town.ItemServiceSell
	}
	return identify, sell
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

// townPreparationStepHandler owns the cross-step graph cursor and the current
// service sub-state. ResetStep clears only the latter; Reset clears both.
// Potion stages are `walk → npc → shop → orders → close → done`.
type townPreparationStepHandler struct {
	adapter               *townPreparationAdapter
	traversals            []town.Traversal
	traversal             int
	anchor                town.Anchor
	walker                *pathing.TownWalker
	orders                []town.RestockOrder
	order                 int
	itemOrders            []town.ItemServiceOrder
	itemOrder             int
	itemExecutor          *town.ItemServiceExecutor
	itemPolicy            loot.PickitResult
	itemInput             *townItemServiceInput
	stage                 string
	npc                   *town.NPCInteractor
	shop                  *town.ShopOpener
	menu                  *town.MenuSelector
	verifier              *town.RestockVerifier
	buyer                 *town.VendorBuyer
	buyerActed            bool
	buyerCode             string
	buyerCost             int
	settleUntil           time.Time
	shopCloseSent         bool
	authorizedAkaraDialog bool
	authorizedAkaraUnitID uint32
	mercHPBefore          uint32
	mercUnitBefore        uint32
	mercClickAt           time.Time
	mercVerifyStarted     time.Time
	mercHealRequested     bool
	mercReviveRequested   bool
	mercReviveEntered     bool
	mercVerifyTicks       int
}

func newTownPreparationStepHandler(adapter *townPreparationAdapter, traversals []town.Traversal, orders []town.RestockOrder, itemOrders []town.ItemServiceOrder) *townPreparationStepHandler {
	return &townPreparationStepHandler{
		adapter: adapter, traversals: append([]town.Traversal(nil), traversals...), orders: append([]town.RestockOrder(nil), orders...), itemOrders: orderedItemServiceOrders(itemOrders),
		anchor: town.AnchorStash, stage: "walk", itemInput: &townItemServiceInput{controller: adapter.controller, cfg: adapter.stashConfig},
	}
}

func (h *townPreparationStepHandler) Tick(ctx context.Context, step town.PlanStep, state world.State) town.InteractionResult {
	if h == nil || h.adapter == nil {
		return town.InteractionResult{Status: town.InteractionFailed, Reason: "town_handler_unavailable", Done: true}
	}
	switch step.Kind {
	case town.StepService:
		switch step.Service {
		case town.ServicePotions:
			return h.tickPotions(ctx, state)
		case town.ServiceIdentify:
			return h.tickItems(ctx, state, town.ServiceIdentify)
		case town.ServiceSell:
			return h.tickItems(ctx, state, town.ServiceSell)
		case town.ServiceMercenaryHeal:
			return h.tickMercenaryHeal(ctx, state)
		case town.ServiceMercenaryRevive:
			return h.tickMercenaryRevive(ctx, state)
		default:
			return town.InteractionResult{Status: town.InteractionFailed, Reason: "town_service_unsupported", Done: true}
		}
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
		if h.authorizedAkaraDialog && (state.UI.NPCInteractOpen || state.UI.NPCShopOpen) {
			if state.UI.NPCShopOpen {
				h.stage = "orders"
			} else {
				h.stage = "shop"
			}
			return town.InteractionResult{Status: town.InteractionPending}
		}
		if state.UI.NPCInteractOpen || state.UI.NPCShopOpen {
			return town.InteractionResult{Status: town.InteractionFailed, Reason: "npc_ui_preopened", Done: true}
		}
		h.ensureNPC(world.Akara)
		result := h.npc.Tick(state)
		// Mark the dialog as ours on the click itself. The next Memory tick can
		// already show NPCInteractOpen before InteractionComplete, and the
		// preopened gate must not reject that as a foreign UI.
		if result.Status == town.InteractionAction && result.Action == "npc_click" {
			h.authorizedAkaraDialog = true
			h.authorizedAkaraUnitID = result.UnitID
			return result
		}
		if result.Status == town.InteractionComplete {
			h.authorizedAkaraDialog = true
			h.authorizedAkaraUnitID = result.UnitID
			h.stage = "shop"
			return town.InteractionResult{Status: town.InteractionPending}
		}
		return result
	case "shop":
		if state.UI.NPCShopOpen {
			h.stage = "orders"
			return town.InteractionResult{Status: town.InteractionPending}
		}
		h.ensureShop()
		result := h.shop.Tick(state)
		if result.Status == town.InteractionComplete {
			h.stage = "orders"
			return town.InteractionResult{Status: town.InteractionPending}
		}
		return result
	case "orders":
		return h.tickOrders(state)
	case "close":
		if hasItemOrders(h.itemOrders, town.ItemServiceSell) {
			// The ordered sell step remains at Akara and may reuse this shop.
			h.stage = "done"
			return town.InteractionResult{Status: town.InteractionComplete, Done: true}
		}
		if result := h.tickCloseUI(state, town.AnchorAkara); result.Status != town.InteractionComplete {
			return result
		}
		h.authorizedAkaraDialog, h.authorizedAkaraUnitID = false, 0
		h.stage = "done"
		healing, mana := countPotionSupplies(state)
		return town.InteractionResult{Status: town.InteractionComplete, Current: healing + mana, VerifiedFinal: healing + mana, Done: true}
	case "done":
		return town.InteractionResult{Status: town.InteractionComplete, Done: true}
	default:
		return town.InteractionResult{Status: town.InteractionFailed, Reason: "town_service_state_invalid", Done: true}
	}
}

func (h *townPreparationStepHandler) tickItems(ctx context.Context, state world.State, service town.Service) town.InteractionResult {
	anchor, npcID, kind := town.AnchorAkara, world.Akara, town.ItemServiceSell
	if service == town.ServiceIdentify {
		anchor, npcID, kind = town.AnchorCain, world.DeckardCain, town.ItemServiceIdentify
	}
	switch h.stage {
	case "walk":
		result := h.tickWalk(ctx, state, anchor)
		if result.Status != town.InteractionComplete {
			return result
		}
		if service == town.ServiceSell && state.UI.NPCShopOpen {
			h.stage = "items"
		} else {
			h.stage = "npc"
		}
		return town.InteractionResult{Status: town.InteractionPending}
	case "npc":
		h.ensureNPC(npcID)
		result := h.npc.Tick(state)
		if result.Status == town.InteractionComplete {
			if service == town.ServiceSell {
				h.stage = "shop"
			} else {
				h.stage = "items"
			}
			return town.InteractionResult{Status: town.InteractionPending}
		}
		return result
	case "shop":
		if state.UI.NPCShopOpen {
			h.stage = "items"
			return town.InteractionResult{Status: town.InteractionPending}
		}
		h.ensureShop()
		result := h.shop.Tick(state)
		if result.Status == town.InteractionComplete {
			h.stage = "items"
			return town.InteractionResult{Status: town.InteractionPending}
		}
		return result
	case "items":
		return h.tickItemOrders(state, kind, anchor)
	case "close":
		result := h.tickCloseUI(state, anchor)
		if result.Status == town.InteractionComplete {
			h.stage = "done"
			result.Done = true
		}
		return result
	case "done":
		return town.InteractionResult{Status: town.InteractionComplete, Done: true}
	default:
		return town.InteractionResult{Status: town.InteractionFailed, Reason: "town_item_service_state_invalid", Done: true}
	}
}

func (h *townPreparationStepHandler) tickItemOrders(state world.State, kind town.ItemServiceKind, anchor town.Anchor) town.InteractionResult {
	if h.itemOrder >= len(h.itemOrders) || h.itemOrders[h.itemOrder].Kind != kind {
		// Orders are grouped Identify then Sell. Reaching the next service kind
		// completes only the current step; the shared cursor must remain pinned
		// so the following Akara step can execute the same item's sell order.
		h.stage = "close"
		return town.InteractionResult{Status: town.InteractionPending}
	}
	order := h.itemOrders[h.itemOrder]
	if h.itemExecutor == nil {
		item, found := state.FindItemByUnitID(order.UnitID)
		policy := loot.PickitResult{}
		if found && h.adapter.lootFilter != nil {
			policy = h.adapter.lootFilter.Evaluate(item)
		}
		if !found || !policy.Matched || policy.Action != loot.ActionSell {
			// Identify may change identity-dependent predicates. A missing or drifted
			// match revokes the queued operation without authorizing Stash or Sell.
			h.itemOrder++
			return town.InteractionResult{Status: town.InteractionPending, UnitID: order.UnitID, Reason: "pickit_recheck_no_match", Vendor: anchor}
		}
		// Identification may refine set/unique identity. Freeze the latest
		// memory-backed item context together with the revalidated sell policy.
		order.Code, order.Name, order.Quality = item.Code, item.Name, item.Quality
		order.IdentityKind, order.IdentityKey, order.IdentityValid = item.IdentityKind, item.IdentityKey, item.IdentityValid
		h.itemOrders[h.itemOrder] = order
		executor, err := town.NewItemServiceExecutor(h.itemInput, order, townRestockVerifyTicks)
		if err != nil {
			return town.InteractionResult{Status: town.InteractionFailed, Reason: err.Error(), Done: true}
		}
		h.itemExecutor = executor
		h.itemPolicy = policy
	}
	h.itemInput.bind(state)
	result := h.itemExecutor.Tick(state)
	result.Code, result.Name, result.Quality = order.Code, order.Name, order.Quality
	result.IdentityKind, result.IdentityKey, result.IdentityValid = order.IdentityKind, order.IdentityKey, order.IdentityValid
	result.ProfileID, result.RuleID, result.PickitAction = h.itemPolicy.ProfileID, h.itemPolicy.RuleID, string(h.itemPolicy.Action)
	result.ProfileRevision, result.AssignmentRevision = h.itemPolicy.ProfileRevision, h.itemPolicy.AssignmentRevision
	if result.Status == town.InteractionAction && h.adapter.log != nil {
		h.adapter.log.Info("pickit town action", "unit_id", order.UnitID, "input_action", result.Action, "profile_id", result.ProfileID, "rule_id", result.RuleID, "action", result.PickitAction, "profile_revision", result.ProfileRevision, "assignment_revision", result.AssignmentRevision)
	}
	result.Vendor = anchor
	if result.Status == town.InteractionComplete {
		if order.Kind == town.ItemServiceSell {
			// The item-service executor returns complete only after a coherent World
			// snapshot proves that the pinned UnitID left personal inventory. The
			// terminal sell event therefore cannot be derived from the vendor click.
			if h.adapter == nil || h.adapter.telemetry == nil {
				return town.InteractionResult{Status: town.InteractionFailed, Reason: string(town.ReasonTelemetryFailed), UnitID: order.UnitID, Done: true}
			}
			if err := h.adapter.telemetry.EmitTown(town.ExecutorEvent{
				Event: string(telemetry.SellSuccess), VendorUnitID: order.UnitID, Vendor: anchor,
				Code: order.Code, Name: order.Name, Quality: order.Quality,
				IdentityKind: order.IdentityKind, IdentityKey: order.IdentityKey, IdentityValid: order.IdentityValid,
				ProfileID: h.itemPolicy.ProfileID, RuleID: h.itemPolicy.RuleID, PickitAction: string(h.itemPolicy.Action),
				ProfileRevision: h.itemPolicy.ProfileRevision, AssignmentRevision: h.itemPolicy.AssignmentRevision,
			}); err != nil {
				return town.InteractionResult{Status: town.InteractionFailed, Reason: string(town.ReasonTelemetryFailed), UnitID: order.UnitID, Done: true}
			}
		}
		h.itemOrder++
		h.itemExecutor = nil
		h.itemPolicy = loot.PickitResult{}
		return town.InteractionResult{Status: town.InteractionPending, UnitID: order.UnitID, Vendor: anchor}
	}
	return result
}

func (h *townPreparationStepHandler) tickCloseUI(state world.State, anchor town.Anchor) town.InteractionResult {
	if !state.UI.NPCShopOpen && !state.UI.NPCInteractOpen {
		return town.InteractionResult{Status: town.InteractionComplete, Vendor: anchor, Done: true}
	}
	if !h.shopCloseSent {
		if err := h.adapter.controller.PressKey("esc"); err != nil {
			return town.InteractionResult{Status: town.InteractionFailed, Reason: fmt.Sprintf("town_ui_close_failed: %v", err), Done: true}
		}
		h.shopCloseSent = true
		return town.InteractionResult{Status: town.InteractionAction, Action: "shop_close", Vendor: anchor}
	}
	return town.InteractionResult{Status: town.InteractionPending, Vendor: anchor}
}

func (h *townPreparationStepHandler) tickMercenaryHeal(ctx context.Context, state world.State) town.InteractionResult {
	policy := h.adapter.mercenaryPolicy()
	switch h.stage {
	case "walk":
		result := h.tickWalk(ctx, state, town.AnchorAkara)
		if result.Status != town.InteractionComplete {
			return result
		}
		h.stage = "interact"
		return town.InteractionResult{Status: town.InteractionPending}
	case "interact":
		merc := state.Mercenary
		if !merc.Alive || !merc.VitalsKnown {
			return town.InteractionResult{Status: town.InteractionFailed, Reason: string(town.ReasonMercenaryHealStateInvalid), Done: true}
		}
		if int(merc.HPPercent()) >= policy.ThresholdPercent {
			h.stage = "done"
			return town.InteractionResult{Status: town.InteractionComplete, Done: true}
		}
		if h.authorizedAkaraDialog && (state.UI.NPCInteractOpen || state.UI.NPCShopOpen) {
			h.mercHPBefore, h.mercUnitBefore = merc.HP, merc.UnitID
			h.mercClickAt = state.At
			if h.mercClickAt.IsZero() {
				h.mercClickAt = time.Now()
			}
			h.stage = "verify"
			h.mercVerifyTicks = 0
			return town.InteractionResult{Status: town.InteractionPending}
		}
		if state.UI.NPCInteractOpen || state.UI.NPCShopOpen {
			return town.InteractionResult{Status: town.InteractionFailed, Reason: "npc_ui_preopened", Done: true}
		}
		h.ensureNPC(world.Akara)
		result := h.npc.Tick(state)
		if result.Status == town.InteractionAction && result.Action == "npc_click" {
			h.mercHPBefore, h.mercUnitBefore = merc.HP, merc.UnitID
			h.mercClickAt = state.At
			if h.mercClickAt.IsZero() {
				h.mercClickAt = time.Now()
			}
			if !h.mercHealRequested {
				if err := h.emitMercTownEvent(string(telemetry.TownMercenaryHealRequested), town.AnchorAkara, h.mercUnitBefore, int(h.mercHPBefore), 0); err != nil {
					return town.InteractionResult{Status: town.InteractionFailed, Reason: string(town.ReasonTelemetryFailed), Done: true}
				}
				h.mercHealRequested = true
			}
			// Akara heals on the confirmed NPC click. Her dialog is irrelevant
			// for healing and may close naturally when the next walk starts.
			h.authorizedAkaraDialog = true
			h.authorizedAkaraUnitID = result.UnitID
			h.stage = "verify"
			h.mercVerifyTicks = 0
		}
		return result
	case "verify":
		now := state.At
		if now.IsZero() {
			now = time.Now()
		}
		if h.mercVerifyStarted.IsZero() {
			h.mercVerifyStarted = now
		}
		h.mercVerifyTicks++
		if h.mercVerifyTicks < 2 {
			return town.InteractionResult{Status: town.InteractionPending}
		}
		if now.Sub(h.mercVerifyStarted) >= townMercenaryVerifyTimeout {
			return town.InteractionResult{Status: town.InteractionFailed, Reason: string(town.ReasonMercenaryHealVerifyTimeout), Done: true}
		}
		merc := state.Mercenary
		if merc.Dead || !merc.HiredKnown || !merc.Hired {
			return town.InteractionResult{Status: town.InteractionFailed, Reason: string(town.ReasonMercenaryHealStateInvalid), Done: true}
		}
		if !merc.Alive || !merc.VitalsKnown {
			return town.InteractionResult{Status: town.InteractionFailed, Reason: string(town.ReasonMercenaryHealStateInvalid), Done: true}
		}
		if merc.HPPercent() != 100 {
			return town.InteractionResult{Status: town.InteractionPending}
		}
		if err := h.emitMercTownEvent(string(telemetry.TownMercenaryHealConfirmed), town.AnchorAkara, merc.UnitID, int(h.mercHPBefore), int(merc.HP)); err != nil {
			return town.InteractionResult{Status: town.InteractionFailed, Reason: string(town.ReasonTelemetryFailed), Done: true}
		}
		h.stage = "done"
		return town.InteractionResult{Status: town.InteractionComplete, Done: true}
	case "done":
		return town.InteractionResult{Status: town.InteractionComplete, Done: true}
	default:
		return town.InteractionResult{Status: town.InteractionFailed, Reason: "town_mercenary_heal_state_invalid", Done: true}
	}
}

func (h *townPreparationStepHandler) tickMercenaryRevive(ctx context.Context, state world.State) town.InteractionResult {
	switch h.stage {
	case "walk":
		result := h.tickWalk(ctx, state, town.AnchorKashya)
		if result.Status != town.InteractionComplete {
			return result
		}
		h.stage = "interact"
		return town.InteractionResult{Status: town.InteractionPending}
	case "interact":
		merc := state.Mercenary
		if !merc.HiredKnown || !merc.Hired || !merc.Dead || merc.Alive {
			return town.InteractionResult{Status: town.InteractionFailed, Reason: string(town.ReasonMercenaryReviveStateInvalid), Done: true}
		}
		// Preopened only rejects unknown dialogs before our NPC gate starts.
		// After ensureNPC, an open dialog is the expected click outcome.
		if h.npc == nil && (state.UI.NPCInteractOpen || state.UI.NPCShopOpen) {
			return town.InteractionResult{Status: town.InteractionFailed, Reason: "npc_ui_preopened", Done: true}
		}
		h.ensureNPC(world.Kashya)
		result := h.npc.Tick(state)
		if result.Status == town.InteractionComplete {
			h.mercUnitBefore = merc.UnitID
			h.stage = "select"
			return town.InteractionResult{Status: town.InteractionPending, UnitID: result.UnitID}
		}
		return result
	case "select":
		if !state.UI.NPCInteractOpen {
			return town.InteractionResult{Status: town.InteractionFailed, Reason: "npc_dialog_not_open", Done: true}
		}
		h.ensureMenu()
		result := h.menu.Tick(state)
		if result.Status == town.InteractionFailed {
			return result
		}
		if result.Action == "dialog_enter" {
			h.mercReviveEntered = true
			h.mercClickAt = state.At
			if h.mercClickAt.IsZero() {
				h.mercClickAt = time.Now()
			}
			if !h.mercReviveRequested {
				if err := h.emitMercTownEvent(string(telemetry.TownMercenaryReviveRequested), town.AnchorKashya, h.mercUnitBefore, 0, 0); err != nil {
					return town.InteractionResult{Status: town.InteractionFailed, Reason: string(town.ReasonTelemetryFailed), Done: true}
				}
				h.mercReviveRequested = true
			}
		}
		if result.Status == town.InteractionComplete {
			h.stage = "verify"
			h.mercVerifyStarted = time.Time{}
			h.mercVerifyTicks = 0
			return town.InteractionResult{Status: town.InteractionPending}
		}
		return result
	case "verify":
		if !h.mercReviveEntered {
			return town.InteractionResult{Status: town.InteractionFailed, Reason: string(town.ReasonMercenaryReviveStateInvalid), Done: true}
		}
		now := state.At
		if now.IsZero() {
			now = time.Now()
		}
		if h.mercVerifyStarted.IsZero() {
			h.mercVerifyStarted = now
		}
		h.mercVerifyTicks++
		timedOut := now.Sub(h.mercVerifyStarted) >= townMercenaryVerifyTimeout
		if h.mercVerifyTicks < 2 && !timedOut {
			return town.InteractionResult{Status: town.InteractionPending}
		}
		merc := state.Mercenary
		if merc.Alive && merc.VitalsKnown && merc.HP > 0 && merc.MaxHP > 0 && !merc.Dead {
			if err := h.emitMercTownEvent(string(telemetry.TownMercenaryReviveConfirmed), town.AnchorKashya, merc.UnitID, 0, int(merc.HP)); err != nil {
				return town.InteractionResult{Status: town.InteractionFailed, Reason: string(town.ReasonTelemetryFailed), Done: true}
			}
			// A revived Merc that is not full HP is an invalid revive contract, not
			// a dynamically appended Akara heal step.
			if merc.HPPercent() != 100 {
				return town.InteractionResult{Status: town.InteractionFailed, Reason: string(town.ReasonMercenaryReviveStateInvalid), Done: true}
			}
			h.stage = "close"
			return town.InteractionResult{Status: town.InteractionPending, UnitID: merc.UnitID}
		}
		if timedOut {
			if merc.HiredKnown && merc.Hired && merc.Dead && !merc.Alive {
				return town.InteractionResult{Status: town.InteractionFailed, Reason: string(town.ReasonMercenaryReviveInsufficientGold), Done: true}
			}
			return town.InteractionResult{Status: town.InteractionFailed, Reason: string(town.ReasonMercenaryReviveVerifyTimeout), Done: true}
		}
		return town.InteractionResult{Status: town.InteractionPending}
	case "close":
		if result := h.tickCloseUI(state, town.AnchorKashya); result.Status != town.InteractionComplete {
			return result
		}
		h.stage = "done"
		return town.InteractionResult{Status: town.InteractionComplete, Done: true}
	case "done":
		return town.InteractionResult{Status: town.InteractionComplete, Done: true}
	default:
		return town.InteractionResult{Status: town.InteractionFailed, Reason: "town_mercenary_revive_state_invalid", Done: true}
	}
}

func (h *townPreparationStepHandler) emitMercTownEvent(name string, vendor town.Anchor, mercUnitID uint32, hpBefore, hpAfter int) error {
	if h.adapter == nil || h.adapter.telemetry == nil {
		return fmt.Errorf("town telemetry unavailable")
	}
	return h.adapter.telemetry.EmitTown(town.ExecutorEvent{
		Event: name, Kind: town.StepService, Vendor: vendor, MercUnitID: mercUnitID, HPBefore: hpBefore, HPAfter: hpAfter,
		Service: func() town.Service {
			if vendor == town.AnchorKashya {
				return town.ServiceMercenaryRevive
			}
			return town.ServiceMercenaryHeal
		}(),
	})
}

func (h *townPreparationStepHandler) ensureNPC(id uint32) {
	if h.npc != nil {
		return
	}
	clickCfg := h.adapter.pathCfg.Click
	clickCfg.AnchorOffsetTiles = 0
	clicker := pathing.NewEntityClicker(h.adapter.log, h.adapter.controller, h.adapter.pathCfg.Projector(), clickCfg)
	h.npc = town.NewNPCInteractor(townNPCClickerAdapter{clicker: clicker}, id, 15, 8*time.Second)
}

func (h *townPreparationStepHandler) ensureShop() {
	if h.shop == nil {
		h.shop = town.NewShopOpener(h.adapter.controller, 8*time.Second)
	}
}

func (h *townPreparationStepHandler) ensureMenu() {
	if h.menu == nil {
		h.menu = town.NewMenuSelector(h.adapter.controller, 8*time.Second)
	}
}

func hasItemOrders(orders []town.ItemServiceOrder, kind town.ItemServiceKind) bool {
	for _, order := range orders {
		if order.Kind == kind {
			return true
		}
	}
	return false
}

func orderedItemServiceOrders(orders []town.ItemServiceOrder) []town.ItemServiceOrder {
	out := make([]town.ItemServiceOrder, 0, len(orders))
	for _, kind := range []town.ItemServiceKind{town.ItemServiceIdentify, town.ItemServiceSell} {
		for _, order := range orders {
			if order.Kind == kind {
				out = append(out, order)
			}
		}
	}
	return out
}

// townItemServiceInput converts an already classified and UnitID-pinned order
// into the smallest supported Cain or Akara UI action.
type townItemServiceInput struct {
	controller townPreparationController
	cfg        config.LootStashConfig
	state      world.State
}

func (i *townItemServiceInput) bind(state world.State) { i.state = state }

func (i *townItemServiceInput) Identify(unitID uint32) error {
	item, ok := i.state.FindItemByUnitID(unitID)
	if !ok || item.Location != world.ItemLocationInventory || item.Identified || !i.state.UI.NPCInteractOpen || i.state.UI.NPCShopOpen {
		return fmt.Errorf("cain identification gate failed for UnitID %d", unitID)
	}
	if err := i.controller.PressKey("home"); err != nil {
		return fmt.Errorf("select Cain identify option: %w", err)
	}
	// Cain's first dialog entry is Talk; Identify Items is the next entry.
	// Home alone therefore must never be confirmed.
	if err := i.controller.PressKey("down"); err != nil {
		return fmt.Errorf("select Cain identify option: %w", err)
	}
	if err := i.controller.PressKey("enter"); err != nil {
		return fmt.Errorf("confirm Cain identify option: %w", err)
	}
	return nil
}

func (i *townItemServiceInput) Sell(unitID uint32) error {
	item, ok := i.state.FindItemByUnitID(unitID)
	if !ok || item.Location != world.ItemLocationInventory || !item.Identified || !i.state.UI.NPCShopOpen {
		return fmt.Errorf("akara sell gate failed for UnitID %d", unitID)
	}
	window, ok := i.controller.Window()
	if !ok || window.ClientWidth != 1280 || window.ClientHeight != 720 {
		return fmt.Errorf("akara sell requires 1280x720")
	}
	x := i.cfg.InventoryLeft + item.GridX*i.cfg.InventoryCellW + i.cfg.InventoryCellW/2
	y := i.cfg.InventoryTop + item.GridY*i.cfg.InventoryCellH + i.cfg.InventoryCellH/2
	if err := i.controller.MoveTo(x, y); err != nil {
		return fmt.Errorf("move to sell candidate: %w", err)
	}
	if err := i.controller.ClickWithModifier("ctrl", input.MouseLeft); err != nil {
		return fmt.Errorf("sell candidate: %w", err)
	}
	return nil
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
			// The purchase may update the belt before VendorBuyer receives its
			// completion tick. Never recompute a now-zero missing quantity or issue
			// another click; finish the already recorded atomic action first.
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
		// Reprice the concrete live vendor code immediately before the only
		// purchase action; the earlier maximum is solely a pre-navigation gate.
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
		// The verifier authorizes an input count; VendorBuyer separately pins the
		// exact shop item and turns that authorization into one atomic click.
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
		// Strictly verify only the external start. Composed edges share semantic
		// NPC boundaries whose separately recorded first points may differ slightly.
		// Stash/portal/waypoint origins use Memory object proximity; Points[0] alone
		// rejects valid post-stash stands inside click range but off the sample.
		if h.traversal == 0 && !h.adapter.externalStartConfirmed(state, points[0]) {
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
	// Keep traversal, anchor, completed order index, and an Akara dialog that a
	// prior Merc-heal step explicitly authorized for the following shop step.
	h.npc, h.shop, h.menu, h.itemExecutor = nil, nil, nil, nil
	h.itemPolicy = loot.PickitResult{}
	h.verifier, h.buyer = nil, nil
	h.buyerActed, h.buyerCode, h.buyerCost = false, "", 0
	h.settleUntil, h.shopCloseSent = time.Time{}, false
	h.mercHPBefore, h.mercUnitBefore = 0, 0
	h.mercClickAt, h.mercVerifyStarted = time.Time{}, time.Time{}
	h.mercHealRequested, h.mercReviveRequested, h.mercReviveEntered = false, false, false
	h.mercVerifyTicks = 0
	h.stage = "walk"
}

func (h *townPreparationStepHandler) Reset() {
	if h == nil {
		return
	}
	if h.walker != nil {
		h.walker.Reset()
	}
	h.authorizedAkaraDialog, h.authorizedAkaraUnitID = false, 0
	h.ResetStep()
	// A session/run reset invalidates graph continuity and all completed orders.
	h.traversal, h.order, h.itemOrder, h.anchor = 0, 0, 0, town.AnchorStash
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
		// Higher TxtFileNo is the strongest available potion tier Akara exposes;
		// pricing and the later buyer remain bound to the selected concrete code.
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
