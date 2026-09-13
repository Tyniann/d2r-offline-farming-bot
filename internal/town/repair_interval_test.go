package town

import "testing"

func TestIntervalRepairDue(t *testing.T) {
	cases := []struct {
		name                     string
		started, last, interval  int
		want                     bool
	}{
		{"before first", 9, 0, 10, false},
		{"first due", 10, 0, 10, true},
		{"same after latch", 10, 10, 10, false},
		{"before second", 19, 10, 10, false},
		{"second due", 20, 10, 10, true},
		{"catch-up after miss", 11, 0, 10, true},
		{"zero interval", 10, 0, 0, false},
		{"negative interval", 10, 0, -1, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IntervalRepairDue(tc.started, tc.last, tc.interval); got != tc.want {
				t.Fatalf("IntervalRepairDue(%d,%d,%d)=%v want %v", tc.started, tc.last, tc.interval, got, tc.want)
			}
		})
	}
}

func TestRepairIntervalRunsDefaultsToTen(t *testing.T) {
	cfg := validTownConfig()
	cfg.RepairIntervalRuns = 0
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	if cfg.RepairIntervalRuns != DefaultRepairIntervalRuns {
		t.Fatalf("RepairIntervalRuns=%d want %d", cfg.RepairIntervalRuns, DefaultRepairIntervalRuns)
	}
}

func TestPlannerRepairOnlyDemandIncludesCharsi(t *testing.T) {
	planner, err := NewPlanner(validTownConfig())
	if err != nil {
		t.Fatal(err)
	}
	snapshot := InspectDemand(SupplySnapshot{
		Healing: 2, Mana: 4, TownPortalScrolls: 5, IdentifyScrolls: 3, RepairRequired: true,
	}, Thresholds{Healing: 2, Mana: 4, TownPortalScrolls: 5, IdentifyScrolls: 3}, "countess")
	plan, reason := planner.Plan(Origin{Act: OriginAct1, Anchor: AnchorStash}, snapshot, NextRunTarget{ID: "countess", Act: OriginAct1})
	if reason != "" {
		t.Fatal(reason)
	}
	if len(plan.Steps) < 3 || plan.Steps[0].Kind != StepService || plan.Steps[0].Service != ServiceRepair {
		t.Fatalf("steps=%+v want repair then handoff", plan.Steps)
	}
	start, sequence, end, reason := planner.GraphAnchorSequence(plan)
	if reason != "" || start != AnchorStash || end != AnchorWaypoint {
		t.Fatalf("anchors=%s/%v/%s reason=%s", start, sequence, end, reason)
	}
	if len(sequence) != 1 || sequence[0] != AnchorCharsi {
		t.Fatalf("sequence=%v want [charsi]", sequence)
	}
}
