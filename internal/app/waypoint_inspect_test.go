package app

import (
	"encoding/json"
	"testing"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/pathing"
)

func TestResolveWaypointTargetsInspectReportIsStableAndReadOnly(t *testing.T) {
	report, err := ResolveWaypointTargetsInspectReport(Options{WaypointTargetsInspect: true})
	if err != nil {
		t.Fatal(err)
	}
	if report.SchemaVersion != 1 || len(report.Targets) != 3 {
		t.Fatalf("report = %+v", report)
	}
	if report.Targets[0].ID != pathing.WaypointTargetBlackMarsh || report.Targets[1].ID != pathing.WaypointTargetDuranceOfHateLevel2 || report.Targets[2].ID != pathing.WaypointTargetRogueEncampment {
		t.Fatalf("target order = %+v", report.Targets)
	}
	first, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	second, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Fatalf("JSON changed: %s != %s", first, second)
	}
}

func TestResolveWaypointTargetsInspectReportRejectsConflicts(t *testing.T) {
	if _, err := ResolveWaypointTargetsInspectReport(Options{WaypointTargetsInspect: true, Run: "countess"}); err == nil {
		t.Fatal("expected conflicting run mode to fail")
	}
}
