package app

import (
	"testing"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/pathing"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/tasks"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/telemetry"
)

func TestResolveSessionPlanDisabledWithoutRuntimeRequirements(t *testing.T) {
	cfg := &config.Config{Session: config.SessionConfig{Enabled: false, Run: "countess", Difficulty: "normal", MaxRuns: 3, MaxDurationMs: 1000}}
	plan, err := ResolveSessionPlan(cfg, Options{SessionInspect: true})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Status != "disabled" || plan.Enabled {
		t.Fatalf("plan = %+v", plan)
	}
}

func TestResolveSessionPlanReadyForRecordedNightmareRoute(t *testing.T) {
	cfg, err := config.Load("../../configs/config.example.yaml")
	if err != nil {
		t.Fatal(err)
	}
	cfg.Input.Enabled = true
	cfg.Runs.Active = ""
	writeTestRouteAssignments(t, cfg, map[tasks.RunID]string{tasks.RunIDCountess: "countess-mrbones-fd1756c208", tasks.RunIDMephisto: "durance-2-mephisto-nightmare-mrbones"})
	cfg.Session.Enabled = true
	cfg.Session.Character = "MrBones"
	cfg.Session.Difficulty = "nightmare"
	plan, err := ResolveSessionPlan(cfg, Options{SessionInspect: true})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Status != "ready" || plan.RouteLayoutFingerprint == "" || plan.RoutePath == "" {
		t.Fatalf("plan = %+v", plan)
	}
}

func TestBindSessionSetupRouteKeepsPrimaryAndAddsLegContext(t *testing.T) {
	plan := SessionPlan{RouteID: "cow-sweep", RouteLayoutFingerprint: "cow-layout"}
	availability := runAvailabilityResolution{
		routeSets: map[tasks.RunID]map[pathing.RouteRole]pathing.Route{
			tasks.RunIDCows: {
				pathing.RouteRoleLegAcquisition: {ID: "leg-acquisition", Binding: pathing.RouteBinding{LayoutFingerprint: pathing.RouteLayoutFingerprint{Hash: "leg-layout"}}},
			},
		},
		routePaths: map[string]string{"leg-acquisition": "routes/leg.yaml"},
	}
	if err := bindSessionSetupRoute(&plan, availability, tasks.RunIDCows); err != nil {
		t.Fatal(err)
	}
	if plan.RouteID != "cow-sweep" || plan.RouteLayoutFingerprint != "cow-layout" || plan.SetupRouteID != "leg-acquisition" || plan.SetupRouteLayoutFingerprint != "leg-layout" || plan.SetupRoutePath != "routes/leg.yaml" {
		t.Fatalf("route-set plan = %+v", plan)
	}
}

func TestResolveSessionPlanReadyForRecordedMephistoRouteAndEgress(t *testing.T) {
	cfg, err := config.Load("../../configs/config.example.yaml")
	if err != nil {
		t.Fatal(err)
	}
	cfg.Input.Enabled = true
	cfg.Runs.Active = ""
	cfg.Session.Enabled = true
	cfg.Session.Run = "mephisto"
	cfg.Session.Character = "MrBones"
	cfg.Session.Difficulty = "nightmare"

	plan, err := ResolveSessionPlan(cfg, Options{SessionInspect: true})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Status != "ready" || plan.RouteID != "durance-2-mephisto-nightmare-mrbones" || plan.RouteLayoutFingerprint == "" || plan.RoutePath == "" {
		t.Fatalf("Mephisto plan = %+v", plan)
	}
}

func TestMephistoSessionRunContextBindsDefinitionAssetsAndPolicies(t *testing.T) {
	cfg, err := config.Load("../../configs/config.example.yaml")
	if err != nil {
		t.Fatal(err)
	}
	cfg.Input.Enabled = true
	cfg.Runs.Active = ""
	cfg.Session.Enabled = true
	cfg.Session.Run = "mephisto"
	cfg.Session.Character = "MrBones"
	cfg.Session.Difficulty = "nightmare"
	runConfig, err := mapRunConfig(cfg, "mephisto", true)
	if err != nil {
		t.Fatal(err)
	}
	rt := &Runtime{Config: cfg, runConfig: runConfig}

	event, err := rt.sessionRunContextEvent()
	if err != nil {
		t.Fatal(err)
	}
	if event.Event != telemetry.RunContext || event.DefinitionID != "mephisto" || event.RouteID != "durance-2-mephisto-nightmare-mrbones" || event.RouteLayoutFingerprint == "" {
		t.Fatalf("context identity = %+v", event)
	}
	if event.WaypointTarget != "durance_of_hate_level_2" || event.TownOrigin != "act3" {
		t.Fatalf("context assets = %+v", event)
	}
}

func TestResolveSessionPlanRejectsRouteDifficultyMismatch(t *testing.T) {
	cfg, err := config.Load("../../configs/config.example.yaml")
	if err != nil {
		t.Fatal(err)
	}
	cfg.Input.Enabled = true
	writeTestRouteAssignments(t, cfg, map[tasks.RunID]string{tasks.RunIDCountess: "black-marsh-cellar5-nightmare-mrbones", tasks.RunIDMephisto: "durance-2-mephisto-nightmare-mrbones"})
	cfg.Session.Enabled = true
	cfg.Session.Character = "MrBones"
	cfg.Session.Difficulty = "hell"
	if _, err := ResolveSessionPlan(cfg, Options{SessionInspect: true}); err == nil {
		t.Fatal("expected route difficulty mismatch")
	}
}

func TestResolveSessionPlanRejectsModeConflict(t *testing.T) {
	cfg := &config.Config{}
	for _, opts := range []Options{
		{SessionInspect: true, Run: "countess"},
		{SessionInspect: true, CowProbe: "gate-20-0"},
	} {
		if _, err := ResolveSessionPlan(cfg, opts); err == nil {
			t.Fatalf("expected mode conflict for %+v", opts)
		}
	}
}

func TestNewRejectsEnabledSessionExecution(t *testing.T) {
	_, err := New(&config.Config{Session: config.SessionConfig{Enabled: true}}, Options{})
	if err == nil {
		t.Fatal("expected Phase-7.5 execution guard")
	}
}
