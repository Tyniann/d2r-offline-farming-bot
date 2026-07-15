package town

import (
	"testing"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

type transferInputMock struct{ targets []world.AreaID }

func (m *transferInputMock) SelectDestination(id world.AreaID) error {
	m.targets = append(m.targets, id)
	return nil
}

func TestRepairRemainsUnavailableWithoutReliableEvidence(t *testing.T) {
	if ok, reason := PlanRepair(RepairAssessment{Required: true}); ok || reason != ReasonRepairStateUnavailable {
		t.Fatalf("repair=%v reason=%s", ok, reason)
	}
	if ok, reason := PlanRepair(RepairAssessment{Required: true, DurabilityKnown: true, CostKnown: true, UIKnown: true}); !ok || reason != "" {
		t.Fatalf("validated repair=%v reason=%s", ok, reason)
	}
}

func TestWaypointTransfersVerifyHubAndCountessHandoff(t *testing.T) {
	in := &transferInputMock{}
	hub, reason := NewWaypointTransferExecutor(in, WaypointTransfer{FromAct: OriginAct3, ToArea: world.RogueEncampment}, 2)
	if reason != "" {
		t.Fatal(reason)
	}
	kurast := world.State{Valid: true, Area: world.LookupArea(world.KurastDocks)}
	if got := hub.Tick(kurast); got.Action != "waypoint_transfer" || len(in.targets) != 1 {
		t.Fatalf("hub action=%+v targets=%v", got, in.targets)
	}
	if got := hub.Tick(world.State{Valid: true, Area: world.LookupArea(world.RogueEncampment)}); got.Status != InteractionComplete {
		t.Fatalf("hub complete=%+v", got)
	}
	handoff, reason := NewWaypointTransferExecutor(in, WaypointTransfer{FromAct: OriginAct1, ToArea: world.BlackMarsh}, 2)
	if reason != "" || handoff.Tick(world.State{Valid: true, Area: world.LookupArea(world.RogueEncampment)}).Action != "waypoint_transfer" {
		t.Fatalf("handoff reason=%s", reason)
	}
}

func TestWaypointTransferNegativeReasonsAndNoRepeat(t *testing.T) {
	in := &transferInputMock{}
	if _, reason := NewWaypointTransferExecutor(in, WaypointTransfer{FromAct: OriginAct2, ToArea: world.RogueEncampment}, 1); reason != ReasonHubTransferUnsupported {
		t.Fatalf("hub reason=%s", reason)
	}
	if _, reason := NewWaypointTransferExecutor(in, WaypointTransfer{FromAct: OriginAct1, ToArea: world.LutGholein}, 1); reason != ReasonNextTargetUnsupported {
		t.Fatalf("target reason=%s", reason)
	}
	exec, _ := NewWaypointTransferExecutor(in, WaypointTransfer{FromAct: OriginAct3, ToArea: world.RogueEncampment}, 1)
	state := world.State{Valid: true, Area: world.LookupArea(world.KurastDocks)}
	_ = exec.Tick(state)
	if got := exec.Tick(state); got.Reason != string(ReasonTransferVerifyTimeout) || len(in.targets) != 1 {
		t.Fatalf("timeout=%+v targets=%v", got, in.targets)
	}
}

func TestPlannerStableNegativeReasons(t *testing.T) {
	cfg := validTownConfig()
	cfg.Egress[OriginAct2] = EgressConfig{Area: "lut_gholein", RouteID: "act2-egress", Anchors: []Anchor{AnchorPortalArrival, AnchorWaypoint}, RoutesDirectory: "routes/town/act2/egress"}
	planner, err := NewPlanner(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if _, reason := planner.Plan(Origin{Act: OriginAct2}, DemandSnapshot{}, NextRunTarget{}); reason != ReasonHubTransferUnsupported {
		t.Fatalf("hub planner reason=%s", reason)
	}
	if _, reason := planner.Plan(Origin{Act: OriginAct1}, DemandSnapshot{}, NextRunTarget{ID: "unknown", Act: OriginAct2}); reason != ReasonNextTargetUnsupported {
		t.Fatalf("target planner reason=%s", reason)
	}
}
