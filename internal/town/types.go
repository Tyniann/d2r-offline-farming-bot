// Package town plans the fail-closed, act-aware preparation flow between runs.
package town

import "fmt"

// Anchor identifies a confirmed point in a town route graph.
type Anchor string

const (
	AnchorSpawn         Anchor = "spawn"
	AnchorPortalArrival Anchor = "portal_arrival"
	AnchorStash         Anchor = "stash"
	AnchorWaypoint      Anchor = "waypoint"
	AnchorAkara         Anchor = "akara"
	AnchorCharsi        Anchor = "charsi"
	AnchorCain          Anchor = "cain"
)

// OriginAct identifies the act in which the post-run preparation begins.
type OriginAct string

// OriginAct values are deliberately independent from World area IDs.
const (
	OriginActUnknown OriginAct = "unknown"
	OriginAct1       OriginAct = "act1"
	OriginAct2       OriginAct = "act2"
	OriginAct3       OriginAct = "act3"
	OriginAct4       OriginAct = "act4"
	OriginAct5       OriginAct = "act5"
)

// Service identifies one Town service. It deliberately contains no UI details.
type Service string

const (
	ServiceStash    Service = "stash"
	ServicePotions  Service = "potions"
	ServiceScrolls  Service = "scrolls"
	ServiceIdentify Service = "identify"
	ServiceSell     Service = "sell"
	ServiceRepair   Service = "repair"
)

// Reason is a stable terminal planning reason.
type Reason string

const (
	ReasonUnknownOrigin             Reason = "town_origin_unknown"
	ReasonEgressMissing             Reason = "town_egress_missing"
	ReasonHubTransferUnsupported    Reason = "hub_transfer_unsupported"
	ReasonNextTargetUnsupported     Reason = "next_target_unsupported"
	ReasonBudgetExhausted           Reason = "town_budget_exhausted"
	ReasonStopped                   Reason = "town_stopped"
	ReasonPaused                    Reason = "town_paused"
	ReasonGoldUnavailable           Reason = "town_gold_unavailable"
	ReasonRestockStateInvalid       Reason = "town_restock_state_invalid"
	ReasonRestockVerifyTimeout      Reason = "town_restock_verify_timeout"
	ReasonItemClassificationInvalid Reason = "town_item_classification_invalid"
	ReasonItemStateInvalid          Reason = "town_item_state_invalid"
	ReasonItemPinInvalid            Reason = "town_item_pin_invalid"
	ReasonItemVerifyTimeout         Reason = "town_item_verify_timeout"
	ReasonRepairStateUnavailable    Reason = "repair_state_unavailable"
	ReasonTransferStateInvalid      Reason = "town_transfer_state_invalid"
	ReasonTransferVerifyTimeout     Reason = "town_transfer_verify_timeout"
	ReasonTelemetryFailed           Reason = "town_telemetry_failed"
	ReasonTownLayoutUnavailable     Reason = "town_layout_unavailable"
	ReasonTownLayoutRouteMissing    Reason = "town_layout_route_missing"
	ReasonTownLayoutMismatch        Reason = "town_layout_mismatch"
)

// Origin identifies the confirmed act and anchor at which preparation starts.
type Origin struct {
	Act    OriginAct
	Anchor Anchor
}

// Egress defines the required local transition from a foreign town to its waypoint.
type Egress struct {
	Act  OriginAct
	From Anchor
	To   Anchor
}

// HubTransfer defines the validated waypoint transfer from an egress to the Act-1 hub.
type HubTransfer struct {
	From OriginAct
	To   OriginAct
}

// Demand is the immutable result of a read-only preparation inspection.
type Demand struct {
	Stash    bool
	Potions  bool
	Scrolls  bool
	Identify bool
	Sell     bool
	Repair   bool
}

// Empty reports whether no service is required.
func (d Demand) Empty() bool {
	return !d.Stash && !d.Potions && !d.Scrolls && !d.Identify && !d.Sell && !d.Repair
}

// NextRunTarget is the validated destination handed to the next run at the hub waypoint.
type NextRunTarget struct {
	ID  string
	Act OriginAct
}

// Budgets bounds every future Town executor action and the total preparation flow.
type Budgets struct {
	InputAttempts  int
	VerifyAttempts int
	RetryAttempts  int
	TotalSteps     int
}

// Validate rejects non-positive execution bounds before an executor can send input.
func (b Budgets) Validate() error {
	if b.InputAttempts <= 0 || b.VerifyAttempts <= 0 || b.RetryAttempts < 0 || b.TotalSteps <= 0 {
		return fmt.Errorf("town budgets require positive input, verify, and total limits; retry must be >= 0")
	}
	return nil
}

// PlanPhase separates mandatory normalization from later Act-1 services and handoff.
type PlanPhase string

const (
	PlanPhaseNormalize PlanPhase = "normalize"
	PlanPhaseServices  PlanPhase = "services"
	PlanPhaseHandoff   PlanPhase = "handoff"
)

// StepKind identifies one high-level, input-free planning action.
type StepKind string

const (
	StepEgress       StepKind = "egress"
	StepHubTransfer  StepKind = "hub_transfer"
	StepService      StepKind = "service"
	StepHandoff      StepKind = "next_run_handoff"
	StepStash        StepKind = "stash"
	StepAct1Waypoint StepKind = "act1_waypoint"
)

// PlanStep is an ordered operation the later executor must independently gate.
type PlanStep struct {
	Phase   PlanPhase
	Kind    StepKind
	Service Service
	Act     OriginAct
}

// Plan is a finite, immutable preparation plan.
type Plan struct {
	Origin Origin
	Steps  []PlanStep
}

// Validate enforces the fail-closed phase ordering independently of later planning logic.
func (p Plan) Validate() error {
	if p.Origin.Act == OriginActUnknown {
		return fmt.Errorf("%s", ReasonUnknownOrigin)
	}
	foreign := p.Origin.Act != OriginAct1
	normalized := !foreign
	egressSeen := false
	for i, step := range p.Steps {
		if step.Phase == PlanPhaseNormalize {
			if normalized {
				return fmt.Errorf("steps[%d]: duplicate or unnecessary normalization", i)
			}
			if step.Kind == StepEgress && step.Act == p.Origin.Act && !egressSeen {
				egressSeen = true
				continue
			}
			if step.Kind == StepHubTransfer && step.Act == OriginAct1 && egressSeen {
				normalized = true
				continue
			}
			return fmt.Errorf("steps[%d]: invalid normalization step", i)
		}
		if !normalized {
			return fmt.Errorf("steps[%d]: service before normalization", i)
		}
		if step.Phase == PlanPhaseServices && (step.Kind == StepStash || step.Kind == StepService) {
			continue
		}
		if step.Phase == PlanPhaseHandoff && (step.Kind == StepAct1Waypoint || step.Kind == StepHandoff) {
			continue
		}
		return fmt.Errorf("steps[%d]: invalid phase or step kind", i)
	}
	if foreign && !normalized {
		return fmt.Errorf("%s", ReasonEgressMissing)
	}
	return nil
}
