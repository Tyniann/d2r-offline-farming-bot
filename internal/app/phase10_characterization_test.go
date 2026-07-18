package app

import (
	"strings"
	"testing"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
)

func TestPhase10CharacterizationCountessSessionPreflightBinding(t *testing.T) {
	cfg, err := config.Load("../../configs/config.example.yaml")
	if err != nil {
		t.Fatal(err)
	}
	cfg.Input.Enabled = true
	cfg.Runs.Active = ""
	countess, _ := cfg.Runs.Run("countess")
	countess.RouteID = "black-marsh-cellar5-nightmare-mrbones"
	cfg.Runs.Definitions["countess"] = countess
	cfg.Session.Enabled = true
	cfg.Session.Run = "countess"
	cfg.Session.Character = "MrBones"
	cfg.Session.Difficulty = "nightmare"

	plan, err := ResolveSessionPlan(cfg, Options{SessionInspect: true})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Status != "ready" || plan.Run != "countess" || plan.Character != "MrBones" || plan.Difficulty != "nightmare" {
		t.Fatalf("resolved Countess plan = %+v", plan)
	}
	if plan.RouteID != "black-marsh-cellar5-nightmare-mrbones" || !strings.HasSuffix(plan.RoutePath, "black-marsh-cellar5-nightmare-mrbones.yaml") {
		t.Fatalf("resolved Countess route binding = %+v", plan)
	}
	if plan.RouteLayoutFingerprint != "e6020b03a517d9aab52964cb0d8fb5fb362f17606408ac65cfa6f68ed5c519e3" {
		t.Fatalf("resolved Countess layout fingerprint = %q", plan.RouteLayoutFingerprint)
	}
}
