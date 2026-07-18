package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/tasks"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/town"
	"gopkg.in/yaml.v3"
)

const testCountessFingerprint = "e6020b03a517d9aab52964cb0d8fb5fb362f17606408ac65cfa6f68ed5c519e3"

func availabilityConfig(t *testing.T) *config.Config {
	t.Helper()
	cfg, err := config.Load("../../configs/config.example.yaml")
	if err != nil {
		t.Fatal(err)
	}
	cfg.Session.Character = "MrBones"
	cfg.Session.Difficulty = "nightmare"
	cfg.Routes.LifecycleFile = filepath.Join(t.TempDir(), "route-lifecycle.local.yaml")
	cfg.Routes.AssignmentsFile = filepath.Join(t.TempDir(), "route-assignments.local.yaml")
	// Availability tests must not change when an operator records the real
	// Phase-10 Egress asset in the repository workspace.
	egress := cfg.Town.Egress[town.OriginAct3]
	egress.RoutesDirectory = t.TempDir()
	cfg.Town.Egress[town.OriginAct3] = egress
	writeTestRouteAssignments(t, cfg, map[tasks.RunID]string{tasks.RunIDCountess: "black-marsh-cellar5-nightmare-mrbones", tasks.RunIDMephisto: "durance-2-mephisto-nightmare-mrbones"})
	return cfg
}

func writeTestRouteAssignments(t *testing.T, cfg *config.Config, routes map[tasks.RunID]string) {
	t.Helper()
	if cfg.Routes.AssignmentsFile == "" {
		cfg.Routes.AssignmentsFile = filepath.Join(t.TempDir(), "route-assignments.local.yaml")
	}
	character := strings.ToLower(cfg.Session.Character)
	if character == "" {
		character = "mrbones"
	}
	manifest := RouteAssignmentManifest{SchemaVersion: 1, Revision: 1, Assignments: map[string]map[tasks.RunID]string{character: routes}}
	data, err := yaml.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(cfg.ResolvePath(cfg.Routes.AssignmentsFile)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfg.ResolvePath(cfg.Routes.AssignmentsFile), data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestResolveRunAvailabilitiesGoldenOrderAndReasons(t *testing.T) {
	cfg := availabilityConfig(t)
	report, err := ResolveRunAvailabilities(cfg, RunAvailabilityContext{
		Character: "MrBones", CharacterClass: "necromancer", Difficulty: "nightmare", GameVersion: "3.2.92777",
	})
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"context":{"character":"MrBones","character_class":"necromancer","difficulty":"nightmare","game_version":"3.2.92777"},"runs":[{"run_id":"countess","display_name":"Countess","status":"runtime_validation_required","reasons":["route_runtime_validation_required"],"route":{"route_id":"black-marsh-cellar5-nightmare-mrbones","reason":"route_runtime_validation_required"}},{"run_id":"mephisto","display_name":"Mephisto","status":"unavailable","reasons":["town_egress_missing"],"route":{"route_id":"durance-2-mephisto-nightmare-mrbones"}}]}`
	if string(encoded) != want {
		t.Fatalf("availability JSON:\n%s\nwant:\n%s", encoded, want)
	}
}

func TestResolveRunAvailabilitiesCountessAvailableWithLiveFingerprint(t *testing.T) {
	cfg := availabilityConfig(t)
	seed := uint32(466817790)
	report, err := ResolveRunAvailabilities(cfg, RunAvailabilityContext{
		Character: "MrBones", CharacterClass: "necromancer", Difficulty: "nightmare", GameVersion: "3.2.92777",
		MapSeed: &seed, LayoutFingerprint: testCountessFingerprint,
	})
	if err != nil {
		t.Fatal(err)
	}
	countess, ok := findRunAvailability(report.Runs, tasks.RunIDCountess)
	if !ok || countess.Status != tasks.RunAvailabilityAvailable || len(countess.Reasons) != 0 {
		t.Fatalf("Countess availability = %+v", countess)
	}
}

func TestResolveRunAvailabilitiesUsesStableMismatchReasons(t *testing.T) {
	cfg := availabilityConfig(t)
	report, err := ResolveRunAvailabilities(cfg, RunAvailabilityContext{
		Character: "MrBones", CharacterClass: "sorceress", Difficulty: "hell", GameVersion: "3.2.92777",
	})
	if err != nil {
		t.Fatal(err)
	}
	countess, _ := findRunAvailability(report.Runs, tasks.RunIDCountess)
	if countess.Status != tasks.RunAvailabilityUnavailable || len(countess.Reasons) != 2 || countess.Reasons[0] != tasks.RunReasonProfileClassMismatch || countess.Reasons[1] != tasks.RunReasonRouteBindingMismatch {
		t.Fatalf("Countess mismatch reasons = %+v", countess)
	}
}

func TestResolveSessionPlanBlocksUnavailableRunBeforeRuntime(t *testing.T) {
	cfg := availabilityConfig(t)
	cfg.Session.Enabled = true
	cfg.Session.Run = string(tasks.RunIDMephisto)
	cfg.Input.Enabled = true
	_, err := ResolveSessionPlan(cfg, Options{SessionInspect: true})
	if err == nil || !strings.Contains(err.Error(), string(tasks.RunReasonTownEgressMissing)) {
		t.Fatalf("session error = %v", err)
	}
}

func TestResolveRunAvailabilitiesReportsMissingForeignTownEgress(t *testing.T) {
	cfg := availabilityConfig(t)
	delete(cfg.Town.Egress, town.OriginAct3)
	report, err := ResolveRunAvailabilities(cfg, RunAvailabilityContext{Character: "MrBones", Difficulty: "nightmare", GameVersion: "3.2.92777"})
	if err != nil {
		t.Fatal(err)
	}
	mephisto, _ := findRunAvailability(report.Runs, tasks.RunIDMephisto)
	if !containsRunReason(mephisto.Reasons, tasks.RunReasonTownEgressMissing) {
		t.Fatalf("Mephisto reasons = %v", mephisto.Reasons)
	}
}

func TestResolveRunAvailabilitiesAcceptsBoundForeignTownEgress(t *testing.T) {
	cfg := availabilityConfig(t)
	directory := t.TempDir()
	egress := cfg.Town.Egress[town.OriginAct3]
	egress.RoutesDirectory = directory
	cfg.Town.Egress[town.OriginAct3] = egress
	state, fingerprint := egressTestState(t)
	saveEgressTestRoute(t, directory, state, fingerprint)
	seed := state.Identity.MapSeed
	report, err := ResolveRunAvailabilities(cfg, RunAvailabilityContext{
		Character: "MrBones", CharacterClass: "necromancer", Difficulty: "nightmare", GameVersion: "3.2.92777", MapSeed: &seed,
	})
	if err != nil {
		t.Fatal(err)
	}
	mephisto, _ := findRunAvailability(report.Runs, tasks.RunIDMephisto)
	if containsRunReason(mephisto.Reasons, tasks.RunReasonTownEgressMissing) || containsRunReason(mephisto.Reasons, tasks.RunReasonTownEgressBindingMismatch) {
		t.Fatalf("bound Egress rejected: %v", mephisto.Reasons)
	}
}

func TestResolveRunsInspectReportRejectsRuntimeModeConflict(t *testing.T) {
	cfg := availabilityConfig(t)
	if _, err := ResolveRunsInspectReport(cfg, Options{RunsInspect: true, Run: "countess"}); err == nil {
		t.Fatal("expected --runs-inspect conflict")
	}
}

func containsRunReason(reasons []tasks.RunReason, want tasks.RunReason) bool {
	for _, reason := range reasons {
		if reason == want {
			return true
		}
	}
	return false
}
