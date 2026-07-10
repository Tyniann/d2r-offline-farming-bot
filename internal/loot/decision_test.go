package loot

import (
	"testing"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

func TestDecideUnmatchedGroundItemIgnored(t *testing.T) {
	filter := testDecisionFilter(t, allFreeLock(), "[type] == rune")
	report := filter.Decide(world.State{Items: []world.Item{
		groundItem(1001, "hp1", "misc", 1, 1),
	}})

	if report.GroundItemCount != 1 || report.InventoryItemCount != 0 {
		t.Fatalf("counts = ground %d inventory %d, want 1/0", report.GroundItemCount, report.InventoryItemCount)
	}
	assertDecisionSequence(t, report.Decisions, []decisionWant{
		{unitID: 1001, stage: DecisionStageClassify, kind: DecisionKindIgnore, reason: DecisionReasonPickitNoMatch},
	})
}

func TestDecideUnmatchedInventoryItemEmitsNoDecision(t *testing.T) {
	filter := testDecisionFilter(t, allFreeLock(), "[type] == rune")
	report := filter.Decide(world.State{Items: []world.Item{
		inventoryItem(2001, "hp1", "misc", 0, 0, 1, 1),
	}})

	if report.InventoryItemCount != 1 {
		t.Fatalf("InventoryItemCount = %d, want 1", report.InventoryItemCount)
	}
	if len(report.Decisions) != 0 {
		t.Fatalf("Decisions = %+v, want none", report.Decisions)
	}
}

func TestDecideMatchedGroundItemEmitsPipelineStages(t *testing.T) {
	filter := testDecisionFilter(t, allFreeLock(), `
[type] == rune
[name] == pk1
`)
	report := filter.Decide(world.State{Items: []world.Item{
		groundItem(1001, "r01", "rune", 1, 1),
	}})

	assertDecisionSequence(t, report.Decisions, []decisionWant{
		{unitID: 1001, stage: DecisionStageClassify, kind: DecisionKindClassifyMatch, reason: DecisionReasonPickitMatch},
		{unitID: 1001, stage: DecisionStagePickCandidate, kind: DecisionKindPickCandidate, reason: DecisionReasonPickitMatch, canFit: true},
		{unitID: 1001, stage: DecisionStagePickupAttempt, kind: DecisionKindPickupPending, reason: DecisionReasonPickupNotAttempted},
		{unitID: 1001, stage: DecisionStageVerify, kind: DecisionKindVerifyPending, reason: DecisionReasonVerifyNotAttempted},
	})
	if got := report.Decisions[0].Pickit; !got.Matched || got.RuleIndex != 0 || got.Line != 2 || got.Rule != "[type] == rune" {
		t.Fatalf("Pickit metadata = %+v, want first rule on line 2", got)
	}
}

func TestDecideMatchedGroundItemFailsWhenCapacityUnsafe(t *testing.T) {
	tests := []struct {
		name           string
		inventoryItems []world.Item
		wantCapacity   DecisionReason
	}{
		{
			name:           "unknown size",
			inventoryItems: []world.Item{inventoryItem(2001, "rin", "ring", 0, 0, 0, 1)},
			wantCapacity:   DecisionReasonUnknownSize,
		},
		{
			name:           "out of bounds",
			inventoryItems: []world.Item{inventoryItem(2001, "rin", "ring", 9, 3, 2, 1)},
			wantCapacity:   DecisionReasonOutOfBounds,
		},
		{
			name: "overlap",
			inventoryItems: []world.Item{
				inventoryItem(2001, "rin", "ring", 0, 0, 2, 2),
				inventoryItem(2002, "rin", "ring", 1, 1, 1, 1),
			},
			wantCapacity: DecisionReasonOverlap,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			filter := testDecisionFilter(t, allFreeLock(), "[type] == rune")
			items := append([]world.Item{groundItem(1001, "r01", "rune", 1, 1)}, tc.inventoryItems...)
			report := filter.Decide(world.State{Items: items})

			assertDecisionSequence(t, report.Decisions[:2], []decisionWant{
				{unitID: 1001, stage: DecisionStageClassify, kind: DecisionKindClassifyMatch, reason: DecisionReasonPickitMatch},
				{unitID: 1001, stage: DecisionStageFail, kind: DecisionKindFail, reason: DecisionReasonCapacityUnsafe, capacityReason: tc.wantCapacity},
			})
		})
	}
}

func TestDecideMatchedGroundItemFailsForUnknownSizeAndFullInventory(t *testing.T) {
	tests := []struct {
		name   string
		lock   [][]int
		item   world.Item
		reason DecisionReason
	}{
		{
			name:   "unknown candidate size",
			lock:   allFreeLock(),
			item:   groundItem(1001, "r01", "rune", 0, 1),
			reason: DecisionReasonUnknownSize,
		},
		{
			name:   "no fitting free space",
			lock:   allLockedInventory(),
			item:   groundItem(1001, "r01", "rune", 1, 1),
			reason: DecisionReasonInventoryFull,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			filter := testDecisionFilter(t, tc.lock, "[type] == rune")
			report := filter.Decide(world.State{Items: []world.Item{tc.item}})

			assertDecisionSequence(t, report.Decisions, []decisionWant{
				{unitID: 1001, stage: DecisionStageClassify, kind: DecisionKindClassifyMatch, reason: DecisionReasonPickitMatch},
				{unitID: 1001, stage: DecisionStageFail, kind: DecisionKindFail, reason: tc.reason},
			})
		})
	}
}

func TestDecideGroundCandidatesDoNotReserveInventorySlots(t *testing.T) {
	lock := [][]int{
		{0, 0, 1, 1, 1, 1, 1, 1, 1, 1},
		{0, 0, 1, 1, 1, 1, 1, 1, 1, 1},
		{1, 1, 1, 1, 1, 1, 1, 1, 1, 1},
		{1, 1, 1, 1, 1, 1, 1, 1, 1, 1},
	}
	filter := testDecisionFilter(t, lock, "[type] == rune")
	report := filter.Decide(world.State{Items: []world.Item{
		groundItem(1001, "r01", "rune", 2, 2),
		groundItem(1002, "r02", "rune", 2, 2),
	}})

	var candidates int
	for _, decision := range report.Decisions {
		if decision.Kind == DecisionKindPickCandidate {
			candidates++
		}
	}
	if candidates != 2 {
		t.Fatalf("pick candidates = %d, want both candidates independently fit current inventory", candidates)
	}
}

func TestDecideMatchedInventoryItemEmitsKeepAndStashWhenEligible(t *testing.T) {
	filter := testDecisionFilter(t, allFreeLock(), "[type] == rune")
	report := filter.Decide(world.State{Items: []world.Item{
		inventoryItem(2001, "r01", "rune", 0, 0, 1, 1),
	}})

	assertDecisionSequence(t, report.Decisions, []decisionWant{
		{unitID: 2001, stage: DecisionStageKeep, kind: DecisionKindKeep, reason: DecisionReasonPickitMatch},
		{unitID: 2001, stage: DecisionStageStash, kind: DecisionKindStash, reason: DecisionReasonStashCandidate},
	})
}

func TestDecideStashRequiresUnlockedValidPersonalInventoryItem(t *testing.T) {
	tests := []struct {
		name string
		lock [][]int
		item world.Item
	}{
		{
			name: "partially locked footprint",
			lock: [][]int{
				{0, 1, 0, 0, 0, 0, 0, 0, 0, 0},
				{0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
				{0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
				{0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
			},
			item: inventoryItem(2001, "r01", "rune", 0, 0, 2, 2),
		},
		{
			name: "invalid footprint",
			lock: allFreeLock(),
			item: inventoryItem(2001, "r01", "rune", 9, 3, 2, 1),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			filter := testDecisionFilter(t, tc.lock, "[type] == rune")
			report := filter.Decide(world.State{Items: []world.Item{tc.item}})

			assertDecisionSequence(t, report.Decisions, []decisionWant{
				{unitID: 2001, stage: DecisionStageKeep, kind: DecisionKindKeep, reason: DecisionReasonPickitMatch},
			})
		})
	}
}

func TestDecideUnsafeCapacityKeepsMatchedInventoryItemWithoutStash(t *testing.T) {
	filter := testDecisionFilter(t, allFreeLock(), "[type] == rune")
	report := filter.Decide(world.State{Items: []world.Item{
		inventoryItem(2001, "r01", "rune", 0, 0, 0, 1),
	}})

	if !report.InventoryCapacity.Unsafe || report.InventoryCapacity.Reason != CapacityReasonUnknownSize {
		t.Fatalf("InventoryCapacity = %+v, want unknown_size unsafe", report.InventoryCapacity)
	}
	assertDecisionSequence(t, report.Decisions, []decisionWant{
		{unitID: 2001, stage: DecisionStageKeep, kind: DecisionKindKeep, reason: DecisionReasonPickitMatch},
	})
}

func TestDecideNilFilterAndNilPickit(t *testing.T) {
	var nilFilter *Filter
	if report := nilFilter.Decide(world.State{Items: []world.Item{groundItem(1001, "r01", "rune", 1, 1)}}); len(report.Decisions) != 0 {
		t.Fatalf("nil filter report = %+v, want empty", report)
	}

	lock := testLock(t, allFreeLock())
	filter := NewFilter(testLogger(), lock, nil)
	report := filter.Decide(world.State{Items: []world.Item{groundItem(1001, "r01", "rune", 1, 1)}})
	assertDecisionSequence(t, report.Decisions, []decisionWant{
		{unitID: 1001, stage: DecisionStageClassify, kind: DecisionKindIgnore, reason: DecisionReasonPickitNoMatch},
	})
}

func TestDecideMarksUnidentifiedQualityMatchAsIdentifyRequired(t *testing.T) {
	filter := testDecisionFilter(t, allFreeLock(), "[quality] == unique")
	item := inventoryItem(2001, "amu", "amul", 4, 2, 1, 1)
	item.Quality = world.ItemQualityUnique
	item.Identified = false
	report := filter.Decide(world.State{Items: []world.Item{item}})
	assertDecisionSequence(t, report.Decisions, []decisionWant{
		{unitID: 2001, stage: DecisionStageKeep, kind: DecisionKindIdentifyRequired, reason: DecisionReasonIdentifyRequired},
	})
}

type decisionWant struct {
	unitID         uint32
	stage          DecisionStage
	kind           DecisionKind
	reason         DecisionReason
	capacityReason DecisionReason
	canFit         bool
}

func assertDecisionSequence(t *testing.T, got []ItemDecision, want []decisionWant) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("decision count = %d, want %d\n got: %+v", len(got), len(want), got)
	}
	for idx, w := range want {
		g := got[idx]
		if g.UnitID != w.unitID || g.Stage != w.stage || g.Kind != w.kind || g.Reason != w.reason ||
			g.CapacityReason != w.capacityReason || g.CanFit != w.canFit {
			t.Fatalf("decision[%d] = %+v, want unit=%d stage=%s kind=%s reason=%s capacity_reason=%s can_fit=%v",
				idx, g, w.unitID, w.stage, w.kind, w.reason, w.capacityReason, w.canFit)
		}
	}
}

func testDecisionFilter(t *testing.T, lockCells [][]int, pickitText string) *Filter {
	t.Helper()
	lock := testLock(t, lockCells)
	pickit, err := parsePickit("test.nip", pickitText)
	if err != nil {
		t.Fatalf("parsePickit() error = %v", err)
	}
	return NewFilter(testLogger(), lock, pickit)
}

func groundItem(unitID uint32, code, itemType string, width, height int) world.Item {
	return world.Item{
		UnitID:   unitID,
		Code:     code,
		Name:     code,
		Type:     itemType,
		Location: world.ItemLocationGround,
		Width:    width,
		Height:   height,
	}
}

func inventoryItem(unitID uint32, code, itemType string, gridX, gridY, width, height int) world.Item {
	return world.Item{
		UnitID:      unitID,
		Code:        code,
		Name:        code,
		Type:        itemType,
		Location:    world.ItemLocationInventory,
		PlayerOwned: true,
		Page:        0,
		GridX:       gridX,
		GridY:       gridY,
		Width:       width,
		Height:      height,
	}
}
