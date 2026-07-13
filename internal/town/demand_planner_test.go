package town

import "testing"

func TestDemandInspectorAndPlannerScenarios(t *testing.T) {
	planner, err := NewPlanner(validTownConfig())
	if err != nil {
		t.Fatal(err)
	}
	thresholds := Thresholds{Healing: 2, Mana: 4, TownPortalScrolls: 5, IdentifyScrolls: 3}
	cases := []struct {
		name       string
		origin     Origin
		supply     SupplySnapshot
		target     NextRunTarget
		wantReason Reason
		wantKinds  []StepKind
	}{
		{"act1 no-op", Origin{Act: OriginAct1, Anchor: AnchorSpawn}, SupplySnapshot{Healing: 2, Mana: 4, TownPortalScrolls: 5, IdentifyScrolls: 3, BeltLayoutComplete: true}, NextRunTarget{}, "", nil},
		{"act3 return", Origin{Act: OriginAct3, Anchor: AnchorPortalArrival}, SupplySnapshot{Healing: 1, Mana: 4, TownPortalScrolls: 5, IdentifyScrolls: 3}, NextRunTarget{}, "", []StepKind{StepEgress, StepHubTransfer, StepService}},
		{"at threshold", Origin{Act: OriginAct1}, SupplySnapshot{Healing: 2, Mana: 4, TownPortalScrolls: 5, IdentifyScrolls: 3}, NextRunTarget{}, "", nil},
		{"one scroll missing", Origin{Act: OriginAct1}, SupplySnapshot{Healing: 2, Mana: 4, TownPortalScrolls: 4, IdentifyScrolls: 3}, NextRunTarget{}, "", []StepKind{StepService}},
		{"incomplete belt stays planable", Origin{Act: OriginAct1}, SupplySnapshot{Healing: 1, Mana: 4, TownPortalScrolls: 5, IdentifyScrolls: 3, BeltLayoutComplete: false}, NextRunTarget{}, "", []StepKind{StepService}},
		{"countess handoff", Origin{Act: OriginAct1}, SupplySnapshot{Healing: 2, Mana: 4, TownPortalScrolls: 5, IdentifyScrolls: 3}, NextRunTarget{ID: "countess", Act: OriginAct1}, "", []StepKind{StepAct1Waypoint, StepHandoff}},
		{"missing egress", Origin{Act: OriginAct2, Anchor: AnchorPortalArrival}, SupplySnapshot{}, NextRunTarget{}, ReasonEgressMissing, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			snapshot := InspectDemand(tc.supply, thresholds)
			plan, reason := planner.Plan(tc.origin, snapshot, tc.target)
			if reason != tc.wantReason {
				t.Fatalf("reason = %q, want %q", reason, tc.wantReason)
			}
			if len(plan.Steps) != len(tc.wantKinds) {
				t.Fatalf("steps = %+v", plan.Steps)
			}
			for i, kind := range tc.wantKinds {
				if plan.Steps[i].Kind != kind {
					t.Fatalf("step %d = %s, want %s", i, plan.Steps[i].Kind, kind)
				}
			}
		})
	}
}

func TestPlannerGraphAnchorsDeduplicatesSharedAkaraServices(t *testing.T) {
	planner, err := NewPlanner(validTownConfig())
	if err != nil {
		t.Fatal(err)
	}
	snapshot := DemandSnapshot{Demand: Demand{Potions: true, Scrolls: true, Identify: true}}
	plan, reason := planner.Plan(Origin{Act: OriginAct1, Anchor: AnchorPortalArrival}, snapshot, NextRunTarget{ID: "countess", Act: OriginAct1})
	if reason != "" {
		t.Fatal(reason)
	}
	start, required, end, reason := planner.GraphAnchors(plan)
	if reason != "" || start != AnchorPortalArrival || end != AnchorWaypoint {
		t.Fatalf("anchors = %s/%v/%s reason=%s", start, required, end, reason)
	}
	if len(required) != 2 || required[0] != AnchorAkara || required[1] != AnchorCain {
		t.Fatalf("required = %v, want akara/cain once", required)
	}
}
