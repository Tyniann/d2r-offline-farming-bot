package app

import (
	"context"
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
	if plan.RouteLayoutFingerprint != "56035675f9c30f9c11bfdea89e1da882d48e95f8423822bd2e95c01291619e37" {
		t.Fatalf("resolved Countess layout fingerprint = %q", plan.RouteLayoutFingerprint)
	}
}

func TestPhase10CharacterizationSuccessfulActiveGameExitsExactlyOnce(t *testing.T) {
	driver := &fakeSessionDriver{}
	result, err := newSessionCycleOrchestrator(driver).execute(context.Background(), true)
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != sessionCycleSuccess {
		t.Fatalf("cycle result = %+v", result)
	}
	if got := countCall(driver.calls, "exit:success"); got != 1 {
		t.Fatalf("successful exits = %d, want exactly 1; calls=%v", got, driver.calls)
	}
	if got := countCall(driver.calls, "start"); got != 0 {
		t.Fatalf("active game unexpectedly started a new game %d time(s); calls=%v", got, driver.calls)
	}
	if reset, exit := indexOf(driver.calls, "run.reset:cycle_evaluate"), indexOf(driver.calls, "exit:success"); reset < 0 || exit < 0 || reset >= exit {
		t.Fatalf("run reset must precede Save & Exit: %v", driver.calls)
	}
}

func countCall(calls []string, target string) int {
	count := 0
	for _, call := range calls {
		if call == target {
			count++
		}
	}
	return count
}
