package app

import (
	"testing"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
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
	cfg.Runs.Countess.RouteID = "black-marsh-cellar5-nightmare-mrbones"
	cfg.Session.Enabled = true
	cfg.Session.Character = "MrBones"
	cfg.Session.Difficulty = "nightmare"
	cfg.Pathing.TownWalk.Difficulty = "nightmare"
	plan, err := ResolveSessionPlan(cfg, Options{SessionInspect: true})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Status != "ready" || plan.RouteLayoutFingerprint == "" || plan.RoutePath == "" {
		t.Fatalf("plan = %+v", plan)
	}
}

func TestResolveSessionPlanRejectsRouteDifficultyMismatch(t *testing.T) {
	cfg, err := config.Load("../../configs/config.example.yaml")
	if err != nil {
		t.Fatal(err)
	}
	cfg.Input.Enabled = true
	cfg.Runs.Countess.RouteID = "black-marsh-cellar5-nightmare-mrbones"
	cfg.Session.Enabled = true
	cfg.Session.Character = "MrBones"
	cfg.Session.Difficulty = "hell"
	cfg.Pathing.TownWalk.Difficulty = "hell"
	if _, err := ResolveSessionPlan(cfg, Options{SessionInspect: true}); err == nil {
		t.Fatal("expected route difficulty mismatch")
	}
}

func TestResolveSessionPlanRejectsModeConflict(t *testing.T) {
	cfg := &config.Config{}
	if _, err := ResolveSessionPlan(cfg, Options{SessionInspect: true, Run: "countess"}); err == nil {
		t.Fatal("expected mode conflict")
	}
}

func TestNewRejectsEnabledSessionExecution(t *testing.T) {
	_, err := New(&config.Config{Session: config.SessionConfig{Enabled: true}}, Options{})
	if err == nil {
		t.Fatal("expected Phase-7.5 execution guard")
	}
}
