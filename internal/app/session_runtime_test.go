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
			Session:   config.SessionConfig{Run: string(tasks.RunIDCountess), Character: "MrBones", Difficulty: "nightmare"},
			Memory:    config.MemoryConfig{GameVersion: "3.2.92777"},
		},
		Log:              config.NewLogger("error"),
		sessionSelection: tasks.RunSelection{Run: string(tasks.RunIDCountess)},
		routePlayback:    &routePlaybackAdapter{},
		lootActions:      &lootActionsAdapter{},
		runConfig:        tasks.RunConfig{RouteID: "countess-route"},
	}

	if _, err := rt.prepareSessionRun(SupervisorRunRequest{ExecutionID: "countess-test-run", SessionID: "session-test", GameID: "game-1", QueueIndex: 0, Cycle: 0}); err != nil {
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
	if err := rt.finishSessionRunTelemetry(SupervisorRunResult{Disposition: QueueRunAdvance, SafeToExit: true}); err != nil {
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
	if !strings.Contains(string(data), `"event":"run_completed"`) {
		t.Fatalf("session run telemetry does not contain its terminal event: %s", data)
	}
}

func TestSessionRunContextHonorsFrozenCombatProfile(t *testing.T) {
	cfg := availabilityConfig(t)
	cfg.Session.Enabled = true
	cfg.Session.Run = string(tasks.RunIDCountess)
	cfg.Input.Enabled = true
	loadout := &CharacterLoadoutSnapshot{Character: "MrBones", ProfileID: "paladin_hammerdin"}
	rt := &Runtime{Config: cfg, Options: Options{Loadout: loadout}}

	_, err := rt.sessionRunContextEvent()
	if err == nil || !strings.Contains(err.Error(), string(tasks.RunReasonProfileClassMismatch)) {
		t.Fatalf("session run context ignored frozen Paladin profile: %v", err)
	}
}

func TestRunTaskCancelClosesOpenStepBeforeTerminal(t *testing.T) {
	rt := &Runtime{
		Config: &config.Config{
			Telemetry: config.TelemetryConfig{Directory: t.TempDir()},
			Session:   config.SessionConfig{Run: string(tasks.RunIDCountess), Character: "MrBones", Difficulty: "nightmare"},
			Memory:    config.MemoryConfig{GameVersion: "3.2.92777"},
		},
		Log:              config.NewLogger("error"),
		sessionSelection: tasks.RunSelection{Run: string(tasks.RunIDCountess)},
		routePlayback:    &routePlaybackAdapter{},
		lootActions:      &lootActionsAdapter{},
		runConfig:        tasks.RunConfig{RouteID: "countess-route"},
	}
	if _, err := rt.prepareSessionRun(SupervisorRunRequest{ExecutionID: "countess-abort-run", SessionID: "session-abort", GameID: "game-abort", QueueIndex: 0, Cycle: 0}); err != nil {
		t.Fatal(err)
	}
	telemetryPath := rt.Telemetry.Path()
	result := rt.Tasks.Tick(context.Background(), world.State{}, time.Now())
	if result.Reason == "telemetry_failed" {
		t.Fatalf("open step tick failed: %+v", result)
	}
	if err := rt.Tasks.AbortOpenStep(string(SupervisorReasonEmergencyStopRequested)); err != nil {
		t.Fatal(err)
	}
	if err := rt.finishSessionRunTelemetry(SupervisorRunResult{
		Disposition: QueueRunStop,
		Reason:      string(SupervisorReasonEmergencyStopRequested),
	}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(telemetryPath)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	stepFailed := strings.Index(content, `"event":"run_step_failed"`)
	aborted := strings.Index(content, `"event":"run_aborted"`)
	if stepFailed < 0 || aborted < 0 || stepFailed > aborted {
		t.Fatalf("want run_step_failed before single run_aborted, got: %s", content)
	}
	if strings.Count(content, `"event":"run_aborted"`) != 1 || strings.Contains(content, `"event":"run_failed"`) {
		t.Fatalf("unexpected terminals in abort telemetry: %s", content)
	}
	if strings.Count(content, `"event":"run_step_started"`) != strings.Count(content, `"event":"run_step_failed"`)+strings.Count(content, `"event":"run_step_completed"`) {
		t.Fatalf("open step remained unterminated: %s", content)
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
