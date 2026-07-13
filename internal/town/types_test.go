package town

import "testing"

func TestPlanContract(t *testing.T) {
	cases := []struct {
		name    string
		plan    Plan
		wantErr bool
	}{
		{"act1 direct", Plan{Origin: Origin{Act: OriginAct1, Anchor: AnchorSpawn}, Steps: []PlanStep{{Phase: PlanPhaseServices, Kind: StepStash, Act: OriginAct1}}}, false},
		{"foreign egress", Plan{Origin: Origin{Act: OriginAct3, Anchor: AnchorPortalArrival}, Steps: []PlanStep{{Phase: PlanPhaseNormalize, Kind: StepEgress, Act: OriginAct3}, {Phase: PlanPhaseNormalize, Kind: StepHubTransfer, Act: OriginAct1}, {Phase: PlanPhaseServices, Kind: StepService, Service: ServicePotions, Act: OriginAct1}}}, false},
		{"empty services", Plan{Origin: Origin{Act: OriginAct1, Anchor: AnchorSpawn}}, false},
		{"unknown origin", Plan{Origin: Origin{Act: OriginActUnknown}}, true},
		{"service before egress", Plan{Origin: Origin{Act: OriginAct3, Anchor: AnchorPortalArrival}, Steps: []PlanStep{{Phase: PlanPhaseServices, Kind: StepStash, Act: OriginAct1}}}, true},
		{"transfer before egress", Plan{Origin: Origin{Act: OriginAct3, Anchor: AnchorPortalArrival}, Steps: []PlanStep{{Phase: PlanPhaseNormalize, Kind: StepHubTransfer, Act: OriginAct1}}}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.plan.Validate()
			if (err != nil) != tc.wantErr {
				t.Fatalf("Validate() error = %v, want error %v", err, tc.wantErr)
			}
		})
	}
}

func TestBudgetsValidate(t *testing.T) {
	if err := (Budgets{InputAttempts: 1, VerifyAttempts: 1, TotalSteps: 1}).Validate(); err != nil {
		t.Fatal(err)
	}
	if err := (Budgets{}).Validate(); err == nil {
		t.Fatal("zero budgets accepted")
	}
}
