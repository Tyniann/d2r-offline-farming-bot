package app

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/loot"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/tasks"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

func TestPrepareSessionRunBindsSharedPipelineTelemetry(t *testing.T) {
	rt := &Runtime{
		Config: &config.Config{
			Telemetry: config.TelemetryConfig{Directory: t.TempDir()},
			Session:   config.SessionConfig{Run: string(tasks.RunIDCountess)},
		},
		Log:              config.NewLogger("error"),
		sessionSelection: tasks.RunSelection{Run: string(tasks.RunIDCountess)},
		routePlayback:    &routePlaybackAdapter{},
		lootActions:      &lootActionsAdapter{},
	}

	if _, err := rt.prepareSessionRun(); err != nil {
		t.Fatal(err)
	}
	telemetryPath := rt.Telemetry.Path()
	if rt.taskDeps.Telemetry == nil {
		t.Fatal("shared pipeline telemetry is nil after session run preparation")
	}

	result := rt.Tasks.Tick(context.Background(), world.State{}, time.Now())
	if result.Reason == "telemetry_failed" {
		t.Fatalf("first session tick failed because pipeline telemetry was not bound: %+v", result)
	}
	if err := rt.closeSessionRunTelemetry(); err != nil {
		t.Fatal(err)
	}
	if rt.taskDeps.Telemetry != nil {
		t.Fatal("shared pipeline telemetry remains bound after session run close")
	}

	data, err := os.ReadFile(telemetryPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"event":"run_step_started"`) {
		t.Fatalf("session run telemetry does not contain the first shared pipeline transition: %s", data)
	}
}

func TestPickitReloadFailureKeepsActiveRunSnapshotImmutable(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(root+"/profiles", 0o755); err != nil {
		t.Fatal(err)
	}
	profiles, err := NewPickitProfileService(root + "/profiles")
	if err != nil {
		t.Fatal(err)
	}
	_, err = profiles.Create(PickitProfileDocument{SchemaVersion: 1, Revision: 1, ID: "base", Name: "Base", Rules: []PickitProfileRuleDocument{{ID: "rune", Action: loot.ActionKeep, Expression: `[type] == "rune"`}}})
	if err != nil {
		t.Fatal(err)
	}
	assignments, err := NewPickitAssignmentStore(root+"/assignments.yaml", profiles)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = assignments.Initialize(map[string]map[tasks.RunID][]string{"MrBones": {tasks.RunIDCountess: {"base"}}}); err != nil {
		t.Fatal(err)
	}
	empty, _ := loot.CompilePickitRules("empty", nil)
	rt := &Runtime{Config: &config.Config{Session: config.SessionConfig{Character: "MrBones", Run: string(tasks.RunIDCountess)}}, Loot: loot.NewFilter(config.NewLogger("error"), loot.InventoryLock{}, empty), PickitAssignments: assignments}
	if err := rt.reloadPickitPolicy(); err != nil {
		t.Fatal(err)
	}
	activePolicy, activeRevision := rt.Loot.Pickit(), rt.ActivePickit.AssignmentRevision
	if err := os.WriteFile(root+"/profiles/base.yaml", []byte("schema_version: 1\nrevision: 2\nid: base\nname: kaputt\nrules: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := rt.reloadPickitPolicy(); err == nil || !strings.Contains(err.Error(), "reload pickit policy") {
		t.Fatalf("reload err=%v", err)
	}
	if rt.Loot.Pickit() != activePolicy || rt.ActivePickit.AssignmentRevision != activeRevision {
		t.Fatal("failed reload mutated active Pickit snapshot")
	}
}
