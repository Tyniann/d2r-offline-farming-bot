package town

import (
	"fmt"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

// ItemServiceKind identifies an explicitly authorized inventory operation.
type ItemServiceKind string

const (
	ItemServiceIdentify ItemServiceKind = "identify"
	ItemServiceSell     ItemServiceKind = "sell"
)

// ItemServiceCandidate is the decision handoff from loot classification.
type ItemServiceCandidate struct {
	UnitID           uint32
	Code             string
	IdentifyRequired bool
	VendorCandidate  bool
	Keep             bool
	Stash            bool
	InventoryLocked  bool
}

// ItemServiceOrder pins one item and its only authorized operation.
type ItemServiceOrder struct {
	Kind   ItemServiceKind
	UnitID uint32
	Code   string
}

// PlanItemServices rejects conflicting classifications and protects keep/stash/lock items.
func PlanItemServices(candidates []ItemServiceCandidate) ([]ItemServiceOrder, Reason) {
	orders := make([]ItemServiceOrder, 0, len(candidates))
	seen := map[uint32]bool{}
	for _, candidate := range candidates {
		if candidate.UnitID == 0 || seen[candidate.UnitID] || (candidate.IdentifyRequired && candidate.VendorCandidate) {
			return nil, ReasonItemClassificationInvalid
		}
		seen[candidate.UnitID] = true
		if candidate.Keep || candidate.Stash || candidate.InventoryLocked {
			continue
		}
		if candidate.IdentifyRequired {
			orders = append(orders, ItemServiceOrder{Kind: ItemServiceIdentify, UnitID: candidate.UnitID, Code: candidate.Code})
		}
		if candidate.VendorCandidate {
			orders = append(orders, ItemServiceOrder{Kind: ItemServiceSell, UnitID: candidate.UnitID, Code: candidate.Code})
		}
	}
	return orders, ""
}

// ItemServiceInput performs one already-gated item operation.
type ItemServiceInput interface {
	Identify(uint32) error
	Sell(uint32) error
}

// ItemServiceExecutor sends one input and verifies the pinned item transition.
type ItemServiceExecutor struct {
	input       ItemServiceInput
	order       ItemServiceOrder
	actionSent  bool
	verifyTicks int
	maxVerify   int
}

// NewItemServiceExecutor creates a finite executor for one classified item.
func NewItemServiceExecutor(input ItemServiceInput, order ItemServiceOrder, maxVerifyTicks int) (*ItemServiceExecutor, error) {
	if input == nil || order.UnitID == 0 || maxVerifyTicks <= 0 || (order.Kind != ItemServiceIdentify && order.Kind != ItemServiceSell) {
		return nil, fmt.Errorf("invalid item service contract")
	}
	return &ItemServiceExecutor{input: input, order: order, maxVerify: maxVerifyTicks}, nil
}

// Tick pins the item before input and confirms identification or location change afterward.
func (e *ItemServiceExecutor) Tick(state world.State) InteractionResult {
	if e == nil || !state.Valid {
		return InteractionResult{Status: InteractionFailed, Reason: string(ReasonItemStateInvalid), Done: true}
	}
	item, found := state.FindItemByUnitID(e.order.UnitID)
	if e.actionSent {
		complete := e.order.Kind == ItemServiceIdentify && found && item.Identified
		complete = complete || (e.order.Kind == ItemServiceSell && (!found || item.Location != world.ItemLocationInventory))
		if complete {
			return InteractionResult{Status: InteractionComplete, UnitID: e.order.UnitID, Done: true}
		}
		e.verifyTicks++
		if e.verifyTicks >= e.maxVerify {
			return InteractionResult{Status: InteractionFailed, Reason: string(ReasonItemVerifyTimeout), UnitID: e.order.UnitID, Done: true}
		}
		return InteractionResult{Status: InteractionPending, UnitID: e.order.UnitID}
	}
	if !found || item.Location != world.ItemLocationInventory || !item.PlayerOwned || item.Page != 0 || (e.order.Code != "" && item.Code != e.order.Code) {
		return InteractionResult{Status: InteractionFailed, Reason: string(ReasonItemPinInvalid), UnitID: e.order.UnitID, Done: true}
	}
	var err error
	action := ""
	if e.order.Kind == ItemServiceIdentify {
		if item.Identified {
			return InteractionResult{Status: InteractionFailed, Reason: string(ReasonItemStateInvalid), UnitID: e.order.UnitID, Done: true}
		}
		err, action = e.input.Identify(e.order.UnitID), "item_identify"
	} else {
		err, action = e.input.Sell(e.order.UnitID), "item_sell"
	}
	if err != nil {
		return InteractionResult{Status: InteractionFailed, Reason: fmt.Sprintf("item_service_input_failed: %v", err), UnitID: e.order.UnitID, Done: true}
	}
	e.actionSent = true
	return InteractionResult{Status: InteractionAction, Action: action, UnitID: e.order.UnitID}
}
