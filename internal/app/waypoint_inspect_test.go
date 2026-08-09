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
	if report.SchemaVersion != 1 || len(report.Targets) != 6 {
		t.Fatalf("report = %+v", report)
	}
	if report.Targets[0].ID != pathing.WaypointTargetArcaneSanctuary || report.Targets[1].ID != pathing.WaypointTargetBlackMarsh || report.Targets[2].ID != pathing.WaypointTargetDuranceOfHateLevel2 || report.Targets[3].ID != pathing.WaypointTargetHallsOfPain || report.Targets[4].ID != pathing.WaypointTargetRogueEncampment || report.Targets[5].ID != pathing.WaypointTargetStonyField {
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
	for _, opts := range []Options{
		{WaypointTargetsInspect: true, Run: "countess"},
		{WaypointTargetsInspect: true, CowProbe: "gate-20-0"},
	} {
		if _, err := ResolveWaypointTargetsInspectReport(opts); err == nil {
			t.Fatalf("expected conflicting mode to fail for %+v", opts)
		}
	}
}
