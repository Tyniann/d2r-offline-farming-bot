package loot

import "github.com/Tyniann/d2r-offline-farming-bot/internal/world"

// DecisionStage names one step in the read-only loot decision pipeline.
type DecisionStage string

// Decision stages are ordered as observe, classify, pick candidate, pickup
// attempt, verify, and the terminal keep, stash, or fail outcomes.
const (
	DecisionStageObserve       DecisionStage = "observe"
	DecisionStageClassify      DecisionStage = "classify"
	DecisionStagePickCandidate DecisionStage = "pick_candidate"
	DecisionStagePickupAttempt DecisionStage = "pickup_attempt"
	DecisionStageVerify        DecisionStage = "verify"
	DecisionStageKeep          DecisionStage = "keep"
	DecisionStageStash         DecisionStage = "stash"
	DecisionStageFail          DecisionStage = "fail"
)

// DecisionKind describes what a pipeline stage decided for an item.
type DecisionKind string

// Decision kinds keep Pickit matching, pickup intent, and later keep/stash
// handling separate so future automation cannot conflate them.
const (
	DecisionKindIgnore           DecisionKind = "ignore"
	DecisionKindClassifyMatch    DecisionKind = "classify_match"
	DecisionKindPickCandidate    DecisionKind = "pick_candidate"
	DecisionKindPickupPending    DecisionKind = "pickup_pending"
	DecisionKindVerifyPending    DecisionKind = "verify_pending"
	DecisionKindKeep             DecisionKind = "keep"
	DecisionKindStash            DecisionKind = "stash"
	DecisionKindIdentifyRequired DecisionKind = "identify_required"
	DecisionKindFail             DecisionKind = "fail"
)

// DecisionReason gives stable machine-readable context for a loot decision.
type DecisionReason string

// Decision reasons intentionally distinguish Pickit matching from real pickup,
// verification, and stash actions, which are implemented in later phases.
const (
	DecisionReasonPickitNoMatch      DecisionReason = "pickit_no_match"
	DecisionReasonPickitMatch        DecisionReason = "pickit_match"
	DecisionReasonInventoryFull      DecisionReason = "inventory_full"
	DecisionReasonCapacityUnsafe     DecisionReason = "capacity_unsafe"
	DecisionReasonPickupNotAttempted DecisionReason = "pickup_not_attempted"
	DecisionReasonVerifyNotAttempted DecisionReason = "verify_not_attempted"
	DecisionReasonStashCandidate     DecisionReason = "stash_candidate"
	DecisionReasonIdentifyRequired   DecisionReason = "identify_required"
	DecisionReasonUnknownSize        DecisionReason = DecisionReason(CapacityReasonUnknownSize)
	DecisionReasonOutOfBounds        DecisionReason = DecisionReason(CapacityReasonOutOfBounds)
	DecisionReasonOverlap            DecisionReason = DecisionReason(CapacityReasonOverlap)
)

// ItemDecision is one ordered event in the loot decision pipeline.
type ItemDecision struct {
	UnitID         uint32
	TxtFileNo      uint32
	Code           string
	Name           string
	Type           string
	Location       world.ItemLocation
	Stage          DecisionStage
	Kind           DecisionKind
	Reason         DecisionReason
	CapacityReason DecisionReason
	Pickit         PickitResult
	Width          int
	Height         int
	CanFit         bool
}

// DecisionReport contains one read-only evaluation of observed loot state.
type DecisionReport struct {
	GroundItemCount    int
	InventoryItemCount int
	InventoryCapacity  InventoryCapacity
	Decisions          []ItemDecision
}

// Decide evaluates ground and inventory items without performing input actions.
//
// The returned Decisions slice is a stable ordered stage/event list. Multiple
// decisions for the same item are normal and represent progression through the
// pipeline, not a single final status per item.
func (f *Filter) Decide(state world.State) DecisionReport {
	if f == nil {
		return DecisionReport{}
	}

	groundItems := state.GroundItems()
	inventoryItems := state.InventoryItems()
	grid := NewInventoryGrid(f.inventoryLock, inventoryItems)
	capacity := grid.Capacity()
	report := DecisionReport{
		GroundItemCount:    len(groundItems),
		InventoryItemCount: len(inventoryItems),
		InventoryCapacity:  capacity,
		Decisions:          make([]ItemDecision, 0),
	}

	for _, item := range groundItems {
		result := f.evaluate(item)
		if !result.Matched {
			report.Decisions = append(report.Decisions, newItemDecision(item, DecisionStageClassify, DecisionKindIgnore, DecisionReasonPickitNoMatch, result))
			continue
		}

		report.Decisions = append(report.Decisions, newItemDecision(item, DecisionStageClassify, DecisionKindClassifyMatch, DecisionReasonPickitMatch, result))
		if capacity.Unsafe {
			decision := newItemDecision(item, DecisionStageFail, DecisionKindFail, DecisionReasonCapacityUnsafe, result)
			decision.CapacityReason = mapCapacityReason(capacity.Reason)
			report.Decisions = append(report.Decisions, decision)
			continue
		}
		if item.Width <= 0 || item.Height <= 0 {
			report.Decisions = append(report.Decisions, newItemDecision(item, DecisionStageFail, DecisionKindFail, DecisionReasonUnknownSize, result))
			continue
		}
		if !grid.CanFit(item.Width, item.Height) {
			report.Decisions = append(report.Decisions, newItemDecision(item, DecisionStageFail, DecisionKindFail, DecisionReasonInventoryFull, result))
			continue
		}

		candidate := newItemDecision(item, DecisionStagePickCandidate, DecisionKindPickCandidate, DecisionReasonPickitMatch, result)
		candidate.CanFit = true
		report.Decisions = append(report.Decisions,
			candidate,
			newItemDecision(item, DecisionStagePickupAttempt, DecisionKindPickupPending, DecisionReasonPickupNotAttempted, result),
			newItemDecision(item, DecisionStageVerify, DecisionKindVerifyPending, DecisionReasonVerifyNotAttempted, result),
		)
	}

	for _, item := range inventoryItems {
		result := f.evaluate(item)
		if !result.Matched {
			continue
		}
		if RequiresIdentificationForKeep(item) {
			report.Decisions = append(report.Decisions, newItemDecision(item, DecisionStageKeep, DecisionKindIdentifyRequired, DecisionReasonIdentifyRequired, result))
			continue
		}
		report.Decisions = append(report.Decisions, newItemDecision(item, DecisionStageKeep, DecisionKindKeep, DecisionReasonPickitMatch, result))
		if capacity.Unsafe || !stashEligible(f.inventoryLock, item) {
			continue
		}
		report.Decisions = append(report.Decisions, newItemDecision(item, DecisionStageStash, DecisionKindStash, DecisionReasonStashCandidate, result))
	}

	return report
}

func (f *Filter) evaluate(item world.Item) PickitResult {
	if f == nil || f.pickit == nil {
		return PickitResult{}
	}
	return f.pickit.Evaluate(item)
}

func newItemDecision(item world.Item, stage DecisionStage, kind DecisionKind, reason DecisionReason, result PickitResult) ItemDecision {
	return ItemDecision{
		UnitID:    item.UnitID,
		TxtFileNo: item.TxtFileNo,
		Code:      item.Code,
		Name:      item.Name,
		Type:      item.Type,
		Location:  item.Location,
		Stage:     stage,
		Kind:      kind,
		Reason:    reason,
		Pickit:    result,
		Width:     item.Width,
		Height:    item.Height,
	}
}

func mapCapacityReason(reason string) DecisionReason {
	switch reason {
	case CapacityReasonUnknownSize:
		return DecisionReasonUnknownSize
	case CapacityReasonOutOfBounds:
		return DecisionReasonOutOfBounds
	case CapacityReasonOverlap:
		return DecisionReasonOverlap
	default:
		return ""
	}
}

func stashEligible(lock InventoryLock, item world.Item) bool {
	if item.Location != world.ItemLocationInventory || !item.PlayerOwned || item.Page != 0 {
		return false
	}
	if item.Width <= 0 || item.Height <= 0 ||
		item.GridX < 0 || item.GridY < 0 ||
		item.GridX+item.Width > inventoryCols ||
		item.GridY+item.Height > inventoryRows {
		return false
	}
	return !itemTouchesLockedSlot(lock, item)
}

func itemTouchesLockedSlot(lock InventoryLock, item world.Item) bool {
	for row := item.GridY; row < item.GridY+item.Height; row++ {
		for col := item.GridX; col < item.GridX+item.Width; col++ {
			if lock.Locked(row, col) {
				return true
			}
		}
	}
	return false
}
