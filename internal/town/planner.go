package town

// Planner creates finite preparation plans from immutable demand and registered town assets.
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
		if origin.Act != OriginAct3 {
			return Plan{}, ReasonHubTransferUnsupported
		}
		steps = append(steps, PlanStep{Phase: PlanPhaseNormalize, Kind: StepEgress, Act: origin.Act}, PlanStep{Phase: PlanPhaseNormalize, Kind: StepHubTransfer, Act: OriginAct1})
	}
	if snapshot.Demand.Stash {
		steps = append(steps, PlanStep{Phase: PlanPhaseServices, Kind: StepStash, Act: OriginAct1})
	}
	for _, service := range []Service{ServicePotions, ServiceScrolls, ServiceIdentify, ServiceSell, ServiceRepair} {
		if demandNeeds(snapshot.Demand, service) {
			steps = append(steps, PlanStep{Phase: PlanPhaseServices, Kind: StepService, Service: service, Act: OriginAct1})
		}
	}
	if target.ID != "" {
		if target.ID != "countess" || target.Act != OriginAct1 {
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
		return d.Potions
	case ServiceScrolls:
		return d.Scrolls
	case ServiceIdentify:
		return d.Identify
	case ServiceSell:
		return d.Sell
	case ServiceRepair:
		return d.Repair
	}
	return false
}

// GraphAnchors converts a validated Act-1 plan into an unordered set of required
// service anchors bracketed by its confirmed start and final waypoint.
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
