package town

import (
	"fmt"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

// RepairAssessment contains only validated repair evidence.
type RepairAssessment struct {
	DurabilityKnown bool
	CostKnown       bool
	UIKnown         bool
	Required        bool
}

// PlanRepair authorizes repair only when every required evidence source is reliable.
func PlanRepair(assessment RepairAssessment) (bool, Reason) {
	if !assessment.Required {
		return false, ""
	}
	if !assessment.DurabilityKnown || !assessment.CostKnown || !assessment.UIKnown {
		return false, ReasonRepairStateUnavailable
	}
	return true, ""
}

// WaypointTransferInput selects one validated destination from an open waypoint UI.
type WaypointTransferInput interface {
	SelectDestination(world.AreaID) error
}

// WaypointTransfer describes one source/destination transition.
type WaypointTransfer struct {
	FromAct OriginAct
	ToArea  world.AreaID
}

// WaypointTransferExecutor sends one selection and verifies the destination area.
type WaypointTransferExecutor struct {
	input       WaypointTransferInput
	transfer    WaypointTransfer
	actionSent  bool
	verifyTicks int
	maxVerify   int
}

// NewWaypointTransferExecutor validates a finite waypoint transition.
func NewWaypointTransferExecutor(input WaypointTransferInput, transfer WaypointTransfer, maxVerifyTicks int) (*WaypointTransferExecutor, Reason) {
	if input == nil || transfer.FromAct == OriginActUnknown || transfer.ToArea == 0 || maxVerifyTicks <= 0 {
		return nil, ReasonTransferStateInvalid
	}
	toAct := originActFromWorld(transfer.ToArea.Act())
	validHub := transfer.FromAct == OriginAct3 && transfer.ToArea == world.RogueEncampment
	validHandoff := transfer.FromAct == OriginAct1 && toAct == OriginAct1 && transfer.ToArea == world.BlackMarsh
	if !validHub && !validHandoff {
		if transfer.FromAct != OriginAct1 && transfer.ToArea == world.RogueEncampment {
			return nil, ReasonHubTransferUnsupported
		}
		return nil, ReasonNextTargetUnsupported
	}
	return &WaypointTransferExecutor{input: input, transfer: transfer, maxVerify: maxVerifyTicks}, ""
}

// Tick selects at most once and waits for a Memory-confirmed destination area.
func (e *WaypointTransferExecutor) Tick(state world.State) InteractionResult {
	if e == nil || !state.Valid {
		return InteractionResult{Status: InteractionFailed, Reason: string(ReasonTransferStateInvalid), Done: true}
	}
	if state.Area.ID == e.transfer.ToArea {
		return InteractionResult{Status: InteractionComplete, Done: true}
	}
	if !e.actionSent {
		if originActFromWorld(state.Area.Act) != e.transfer.FromAct || !state.Area.IsTown() {
			return InteractionResult{Status: InteractionFailed, Reason: string(ReasonTransferStateInvalid), Done: true}
		}
		if err := e.input.SelectDestination(e.transfer.ToArea); err != nil {
			return InteractionResult{Status: InteractionFailed, Reason: fmt.Sprintf("town_transfer_input_failed: %v", err), Done: true}
		}
		e.actionSent = true
		return InteractionResult{Status: InteractionAction, Action: "waypoint_transfer"}
	}
	e.verifyTicks++
	if e.verifyTicks >= e.maxVerify {
		return InteractionResult{Status: InteractionFailed, Reason: string(ReasonTransferVerifyTimeout), Done: true}
	}
	return InteractionResult{Status: InteractionPending}
}

func originActFromWorld(act world.Act) OriginAct {
	switch act {
	case world.Act1:
		return OriginAct1
	case world.Act2:
		return OriginAct2
	case world.Act3:
		return OriginAct3
	case world.Act4:
		return OriginAct4
	case world.Act5:
		return OriginAct5
	default:
		return OriginActUnknown
	}
}
