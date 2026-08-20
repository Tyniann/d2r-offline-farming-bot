package town

import "testing"

func TestPlanRestockThresholdAndModes(t *testing.T) {
	levels := []RestockLevel{{Resource: RestockHealing, Current: 1, Threshold: 2, Target: 4}, {Resource: RestockMana, Current: 4, Threshold: 4, Target: 8}, {Resource: RestockTownPortalScroll, Current: 4, Threshold: 5, Target: 20}}
	orders, reason := PlanRestock(RestockInput{Levels: levels, BeltLayoutComplete: true, GoldKnown: true, GoldSufficient: true})
	if reason != "" || len(orders) != 2 || orders[0].Mode != BuyModeBulk || orders[0].Clicks != 1 || orders[1].Resource != RestockTownPortalScroll {
		t.Fatalf("orders = %+v reason=%s", orders, reason)
	}
	orders, reason = PlanRestock(RestockInput{Levels: levels, BeltLayoutComplete: false, GoldKnown: true, GoldSufficient: true})
	if reason != "" || orders[0].Mode != BuyModeSingle || orders[0].Clicks != 3 || orders[1].Mode != BuyModeBulk {
		t.Fatalf("fallback orders = %+v reason=%s", orders, reason)
	}
}

func TestPlanRestockFailsClosedAndNeverPlansRejuvenation(t *testing.T) {
	base := RestockInput{Levels: []RestockLevel{{Resource: RestockHealing, Current: 0, Threshold: 2, Target: 4}}, BeltLayoutComplete: true}
	if _, reason := PlanRestock(base); reason != ReasonGoldUnavailable {
		t.Fatalf("unknown gold reason = %s", reason)
	}
	base.GoldKnown = true
	if _, reason := PlanRestock(base); reason != ReasonGoldUnavailable {
		t.Fatalf("insufficient gold reason = %s", reason)
	}
	base.GoldSufficient = true
	base.Levels = append(base.Levels, RestockLevel{Resource: "rejuvenation", Current: 0, Threshold: 1, Target: 4})
	if _, reason := PlanRestock(base); reason != ReasonRestockStateInvalid {
		t.Fatalf("rejuvenation reason = %s", reason)
	}
}

func TestRestockVerifierBulkOnceSingleBoundedAndTimeout(t *testing.T) {
	bulk, _ := NewRestockVerifier(RestockOrder{Resource: RestockHealing, Mode: BuyModeBulk, Before: 1, Target: 4, Clicks: 1}, 2)
	if got := bulk.Tick(1); got.Action != "vendor_buy_bulk" {
		t.Fatalf("bulk action = %+v", got)
	}
	if got := bulk.Tick(1); got.Status != InteractionPending {
		t.Fatalf("bulk pending = %+v", got)
	}
	if got := bulk.Tick(1); got.Reason != string(ReasonRestockVerifyTimeout) {
		t.Fatalf("bulk timeout = %+v", got)
	}
	single, _ := NewRestockVerifier(RestockOrder{Resource: RestockMana, Mode: BuyModeSingle, Before: 1, Target: 3, Clicks: 2}, 2)
	if single.Tick(1).Action != "vendor_buy_single" || single.Tick(2).Action != "vendor_buy_single" {
		t.Fatal("single fallback did not emit bounded clicks")
	}
	if got := single.Tick(3); got.Status != InteractionComplete {
		t.Fatalf("single complete = %+v", got)
	}
	if got := single.Tick(3); got.Status != InteractionComplete {
		t.Fatalf("completed verifier repeated action: %+v", got)
	}
}

func TestPlanRestockCityKeysUseSingleClicks(t *testing.T) {
	orders, reason := PlanRestock(RestockInput{
		Levels:             []RestockLevel{{Resource: RestockKey, Current: 5, Threshold: KeyRestockThreshold, Target: KeyRestockTarget}},
		BeltLayoutComplete: true, GoldKnown: true, GoldSufficient: true,
	})
	if reason != "" || len(orders) != 1 || orders[0].Resource != RestockKey || orders[0].Mode != BuyModeSingle || orders[0].Clicks != 7 {
		t.Fatalf("orders=%+v reason=%s", orders, reason)
	}
	orders, reason = PlanRestock(RestockInput{
		Levels:    []RestockLevel{{Resource: RestockKey, Current: KeyRestockThreshold, Threshold: KeyRestockThreshold, Target: KeyRestockTarget}},
		GoldKnown: true, GoldSufficient: true,
	})
	if reason != "" || len(orders) != 0 {
		t.Fatalf("at-threshold orders=%+v reason=%s", orders, reason)
	}
}

func TestMaximumRestockCostUsesConservativeUnitPrices(t *testing.T) {
	levels := []RestockLevel{{Resource: RestockHealing, Current: 1, Threshold: 2, Target: 4}, {Resource: RestockMana, Current: 5, Threshold: 6, Target: 8}}
	got, reason := MaximumRestockCost(levels)
	if reason != "" || got != 4500 {
		t.Fatalf("cost=%d reason=%s", got, reason)
	}
	got, reason = MaximumRestockCost([]RestockLevel{{Resource: RestockKey, Current: 5, Threshold: KeyRestockThreshold, Target: KeyRestockTarget}})
	if reason != "" || got != 315 {
		t.Fatalf("key cost=%d reason=%s", got, reason)
	}
	levels = append(levels, RestockLevel{Resource: "rejuvenation", Current: 0, Threshold: 1, Target: 4})
	if _, reason := MaximumRestockCost(levels); reason != ReasonRestockStateInvalid {
		t.Fatalf("reason=%s", reason)
	}
}
