package town

import "fmt"

// RestockResource identifies a vendor-buyable Town supply.
type RestockResource string

const (
	RestockHealing          RestockResource = "healing"
	RestockMana             RestockResource = "mana"
	RestockTownPortalScroll RestockResource = "town_portal_scroll"
	RestockIdentifyScroll   RestockResource = "identify_scroll"
)

// RestockLevel contains the trigger threshold, observed count, and fill target.
type RestockLevel struct {
	Resource  RestockResource
	Current   int
	Threshold int
	Target    int
}

// RestockInput is one coherent input to purchase planning.
type RestockInput struct {
	Levels             []RestockLevel
	BeltLayoutComplete bool
	GoldKnown          bool
	GoldSufficient     bool
}

// RestockOrder is one bounded vendor operation and its verification contract.
type RestockOrder struct {
	Resource RestockResource
	Mode     BuyMode
	Before   int
	Target   int
	Clicks   int
}

// MaximumRestockCost calculates a fail-closed upper bound before Town input.
func MaximumRestockCost(levels []RestockLevel) (int, Reason) {
	total := 0
	seen := map[RestockResource]bool{}
	for _, level := range levels {
		if !validRestockResource(level.Resource) || seen[level.Resource] || level.Current < 0 || level.Threshold < 0 || level.Target < level.Threshold {
			return 0, ReasonRestockStateInvalid
		}
		seen[level.Resource] = true
		if level.Current >= level.Threshold {
			continue
		}
		missing := level.Target - level.Current
		unitCost, ok := MaximumAkaraUnitCost(level.Resource)
		if missing <= 0 || !ok || missing > int(^uint(0)>>1)/unitCost || total > int(^uint(0)>>1)-missing*unitCost {
			return 0, ReasonRestockStateInvalid
		}
		total += missing * unitCost
	}
	return total, ""
}

// PlanRestock builds orders only for quantities strictly below their thresholds.
func PlanRestock(input RestockInput) ([]RestockOrder, Reason) {
	if !input.GoldKnown || !input.GoldSufficient {
		return nil, ReasonGoldUnavailable
	}
	orders := make([]RestockOrder, 0, len(input.Levels))
	seen := map[RestockResource]bool{}
	for _, level := range input.Levels {
		if !validRestockResource(level.Resource) || seen[level.Resource] || level.Current < 0 || level.Threshold < 0 || level.Target < level.Threshold {
			return nil, ReasonRestockStateInvalid
		}
		seen[level.Resource] = true
		if level.Current >= level.Threshold {
			continue
		}
		missing := level.Target - level.Current
		if missing <= 0 {
			return nil, ReasonRestockStateInvalid
		}
		mode, clicks := BuyModeBulk, 1
		if !input.BeltLayoutComplete && (level.Resource == RestockHealing || level.Resource == RestockMana) {
			mode, clicks = BuyModeSingle, missing
		}
		orders = append(orders, RestockOrder{Resource: level.Resource, Mode: mode, Before: level.Current, Target: level.Target, Clicks: clicks})
	}
	return orders, ""
}

// RestockVerifier bounds purchase input and requires a confirmed final count.
type RestockVerifier struct {
	order       RestockOrder
	clicks      int
	verifyTicks int
	maxVerify   int
	complete    bool
}

// NewRestockVerifier creates a finite verifier for one planned order.
func NewRestockVerifier(order RestockOrder, maxVerifyTicks int) (*RestockVerifier, error) {
	if order.Clicks <= 0 || order.Target <= order.Before || maxVerifyTicks <= 0 {
		return nil, fmt.Errorf("invalid restock verification contract")
	}
	return &RestockVerifier{order: order, maxVerify: maxVerifyTicks}, nil
}

// Tick observes a count and reports whether one purchase input is allowed.
func (v *RestockVerifier) Tick(current int) InteractionResult {
	if v == nil || current < 0 {
		return InteractionResult{Status: InteractionFailed, Reason: string(ReasonRestockStateInvalid), Done: true}
	}
	if v.complete {
		return InteractionResult{Status: InteractionComplete, Done: true}
	}
	if current >= v.order.Target {
		v.complete = true
		return InteractionResult{Status: InteractionComplete, Done: true}
	}
	if v.clicks < v.order.Clicks {
		v.clicks++
		action := "vendor_buy_single"
		if v.order.Mode == BuyModeBulk {
			action = "vendor_buy_bulk"
		}
		return InteractionResult{Status: InteractionAction, Action: action}
	}
	v.verifyTicks++
	if v.verifyTicks >= v.maxVerify {
		return InteractionResult{Status: InteractionFailed, Reason: string(ReasonRestockVerifyTimeout), Done: true}
	}
	return InteractionResult{Status: InteractionPending}
}

func validRestockResource(resource RestockResource) bool {
	switch resource {
	case RestockHealing, RestockMana, RestockTownPortalScroll, RestockIdentifyScroll:
		return true
	default:
		return false
	}
}
