package town

// Planner creates finite preparation plans from immutable demand and registered town assets.
// It has no World or input dependency: observation, authorization, and action
// remain separate layers that can fail independently.
type Planner struct{ config Config }

// NewPlanner validates and binds the town registry without accessing memory or input.
func NewPlanner(config Config) (*Planner, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}
	return &Planner{config: config}, nil
}

// Plan creates egress → transfer → stash → services → waypoint → next-target and removes absent needs.
func (p *Planner) Plan(origin Origin, snapshot DemandSnapshot, target NextRunTarget) (Plan, Reason) {
	if p == nil || origin.Act == OriginActUnknown {
		return Plan{}, ReasonUnknownOrigin
	}
	steps := make([]PlanStep, 0, 10)
	if origin.Act != OriginAct1 {
		if _, reason := p.config.EgressFor(origin.Act); reason != "" {
			return Plan{}, reason
		}
		steps = append(steps, PlanStep{Phase: PlanPhaseNormalize, Kind: StepEgress, Act: origin.Act}, PlanStep{Phase: PlanPhaseNormalize, Kind: StepHubTransfer, Act: OriginAct1})
	}
	if snapshot.Demand.Stash {
		steps = append(steps, PlanStep{Phase: PlanPhaseServices, Kind: StepStash, Act: OriginAct1})
	}
	// Identification must precede every Akara action so an unid sell candidate
	// keeps the same UnitID across the ordered Cain -> Akara transaction.
	// Merc revive/heal sit between Cain and Akara shop work so a revived full
	// Merc never shares an Akara click with potions, and heal reuses dialog.
	for _, service := range []Service{ServiceIdentify, ServiceMercenaryRevive, ServiceMercenaryHeal, ServicePotions, ServiceScrolls, ServiceSell, ServiceRepair} {
		if demandNeeds(snapshot.Demand, service) {
			steps = append(steps, PlanStep{Phase: PlanPhaseServices, Kind: StepService, Service: service, Act: OriginAct1})
		}
	}
	if target.ID != "" {
		// Run identity is validated by the registry before Town planning. The
		// shared Act-1 handoff must not encode a particular farming definition.
		if target.Act != OriginAct1 {
			return Plan{}, ReasonNextTargetUnsupported
		}
		steps = append(steps, PlanStep{Phase: PlanPhaseHandoff, Kind: StepAct1Waypoint, Act: OriginAct1}, PlanStep{Phase: PlanPhaseHandoff, Kind: StepHandoff, Act: target.Act})
	}
	plan := Plan{Origin: origin, Steps: steps}
	if err := plan.Validate(); err != nil {
		return Plan{}, ReasonHubTransferUnsupported
	}
	return plan, ""
}

func demandNeeds(d Demand, service Service) bool {
	switch service {
	case ServicePotions:
		// City keys share Akara's existing potion shop path. No second vendor.
		return d.Potions || d.Keys
	case ServiceScrolls:
		return d.Scrolls
	case ServiceIdentify:
		return d.Identify
	case ServiceSell:
		return d.Sell
	case ServiceRepair:
		return d.Repair
	case ServiceMercenaryHeal:
		return d.MercenaryHeal
	case ServiceMercenaryRevive:
		return d.MercenaryRevive
	}
	return false
}

// GraphAnchors converts a validated Act-1 plan into an unordered set of required
// service anchors bracketed by its confirmed start and final waypoint. Required
// anchors are deduplicated and unordered so routing can minimize travel rather
// than inheriting arbitrary service declaration order.
func (p *Planner) GraphAnchors(plan Plan) (Anchor, []Anchor, Anchor, Reason) {
	if p == nil || plan.Origin.Act != OriginAct1 || !knownGraphAnchor(plan.Origin.Anchor) {
		return "", nil, "", ReasonUnknownOrigin
	}
	required := make([]Anchor, 0, 5)
	seen := map[Anchor]bool{}
	for _, step := range plan.Steps {
		anchor := Anchor("")
		switch step.Kind {
		case StepStash:
			anchor = AnchorStash
		case StepService:
			anchor = p.config.Hub.Services[step.Service]
		}
		if anchor != "" && !seen[anchor] {
			seen[anchor] = true
			required = append(required, anchor)
		}
	}
	return plan.Origin.Anchor, required, AnchorWaypoint, ""
}

// GraphAnchorSequence preserves service order, including a later return to the
// same provider, for workflows whose UI state depends on Cain-before-Akara.
func (p *Planner) GraphAnchorSequence(plan Plan) (Anchor, []Anchor, Anchor, Reason) {
	if p == nil || plan.Origin.Act != OriginAct1 || !knownGraphAnchor(plan.Origin.Anchor) {
		return "", nil, "", ReasonUnknownOrigin
	}
	sequence := make([]Anchor, 0, 6)
	for _, step := range plan.Steps {
		anchor := Anchor("")
		switch step.Kind {
		case StepStash:
			anchor = AnchorStash
		case StepService:
			anchor = p.config.Hub.Services[step.Service]
		}
		if anchor != "" && (len(sequence) == 0 || sequence[len(sequence)-1] != anchor) {
			sequence = append(sequence, anchor)
		}
	}
	return plan.Origin.Anchor, sequence, AnchorWaypoint, ""
}
