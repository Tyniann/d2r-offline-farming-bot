package tasks

import (
	"context"
	"testing"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

func townState() world.State {
	return world.State{
		Valid: true,
		Area:  world.LookupArea(world.AreaID(1)), // Rogue Encampment
	}
}

func blackMarshState() world.State {
	return world.State{
		Valid: true,
		Area:  world.LookupArea(world.AreaID(6)), // Black Marsh
	}
}

func newTestRunner(runName string) *Runner {
	return NewRunner(config.NewLogger("error"), runName, RunConfig{StepTimeout: 30 * time.Second}, Deps{})
}

func TestRunnerPassiveModeNoOp(t *testing.T) {
	r := newTestRunner("")
	now := time.Now()

	res := r.Tick(context.Background(), townState(), now)
	if res.Active {
		t.Fatal("expected inactive tick in passive mode")
	}
	if res.Outcome != RunOutcomeIdle {
		t.Fatalf("Outcome = %q, want idle", res.Outcome)
	}

	r.Reset("process_lost")
	if r.WasReset() {
		t.Fatal("Reset should be no-op in passive mode")
	}
}

func TestRunnerLazyStartAndSuccessInTown(t *testing.T) {
	r := newTestRunner("countess")
	now := time.Now()

	if r.started {
		t.Fatal("run should not start before first tick")
	}

	res := r.Tick(context.Background(), townState(), now)
	if !res.Active || res.Outcome != RunOutcomeRunning || res.Step != countessStepArmed {
		t.Fatalf("first tick = %+v, want armed after precheck completes same tick", res)
	}
	if !r.started {
		t.Fatal("expected started after first tick")
	}

	res = r.Tick(context.Background(), townState(), now.Add(time.Millisecond))
	if res.Step != countessStepArmed || !res.Active {
		t.Fatalf("second tick = %+v, want armed tick 1", res)
	}

	res = r.Tick(context.Background(), townState(), now.Add(2*time.Millisecond))
	if res.Step != countessStepComplete || !res.Active {
		t.Fatalf("third tick = %+v, want complete after armed", res)
	}

	res = r.Tick(context.Background(), townState(), now.Add(3*time.Millisecond))
	if !res.Active || res.Outcome != RunOutcomeSuccess {
		t.Fatalf("fourth tick = %+v, want success", res)
	}
	if !r.Terminal() {
		t.Fatal("expected terminal after success")
	}

	res = r.Tick(context.Background(), townState(), now.Add(4*time.Millisecond))
	if res.Active {
		t.Fatal("expected inactive after terminal")
	}
}

func TestRunnerPrecheckFailsOutsideTown(t *testing.T) {
	r := newTestRunner("countess")
	now := time.Now()
	st := blackMarshState()
	if st.Area.ID != world.AreaID(6) {
		t.Fatalf("AreaID = %d, want 6 (Black Marsh)", st.Area.ID)
	}

	res := r.Tick(context.Background(), st, now)
	if !res.Active || res.Outcome != RunOutcomeFailed {
		t.Fatalf("tick = %+v, want failed", res)
	}
	if res.Reason != "not_in_town" {
		t.Fatalf("Reason = %q, want not_in_town", res.Reason)
	}
	if !r.Terminal() {
		t.Fatal("expected terminal after failure")
	}

	res = r.Tick(context.Background(), st, now.Add(time.Millisecond))
	if res.Active {
		t.Fatal("expected inactive after terminal failure")
	}
}

func TestRunnerPrecheckSucceedsInTownAreaID1(t *testing.T) {
	st := townState()
	if st.Area.ID != world.AreaID(1) {
		t.Fatalf("AreaID = %d, want 1 (Rogue Encampment)", st.Area.ID)
	}
	if !st.Area.IsTown() {
		t.Fatal("expected town area for AreaID 1")
	}
}

// waitRun is a test-only run machine for timeout verification.
type waitRun struct{}

func (w *waitRun) firstStep() string { return "wait" }
func (w *waitRun) nextStep(string) string { return "" }
func (w *waitRun) usesTickTimeout(string) bool { return false }
func (w *waitRun) onTick(string, world.State, int) stepResult { return stepResult{} }

func TestRunnerWaitStepTimeout(t *testing.T) {
	r := NewRunner(config.NewLogger("error"), "wait", RunConfig{StepTimeout: 5 * time.Millisecond}, Deps{})
	r.run = &waitRun{}
	now := time.Now()

	res := r.Tick(context.Background(), townState(), now)
	if !res.Active || res.Step != "wait" {
		t.Fatalf("first tick = %+v, want active wait step", res)
	}

	res = r.Tick(context.Background(), townState(), now.Add(10*time.Millisecond))
	if res.Outcome != RunOutcomeFailed || res.Reason != "timeout" {
		t.Fatalf("expected timeout failure, got %+v", res)
	}
	if !r.Terminal() {
		t.Fatal("expected terminal after timeout")
	}

	res = r.Tick(context.Background(), townState(), now.Add(20*time.Millisecond))
	if res.Active {
		t.Fatal("expected inactive after terminal timeout")
	}
}

func TestRunnerResetBlocksRestart(t *testing.T) {
	r := newTestRunner("countess")
	now := time.Now()

	r.Reset("process_lost")
	if !r.WasReset() {
		t.Fatal("expected WasReset after Reset")
	}

	res := r.Tick(context.Background(), townState(), now)
	if res.Active {
		t.Fatal("expected inactive after reset")
	}
	if r.started {
		t.Fatal("run should not lazy-start after reset")
	}
}

func TestRunnerStepTimeoutNotUsedForTickSteps(t *testing.T) {
	r := NewRunner(config.NewLogger("error"), "countess", RunConfig{StepTimeout: 1 * time.Millisecond}, Deps{})
	now := time.Now()

	res := r.Tick(context.Background(), townState(), now)
	if res.Step != countessStepArmed {
		t.Fatalf("first tick step = %q, want armed", res.Step)
	}

	// Armed uses tick counter, not time timeout — still running after long elapsed time.
	res = r.Tick(context.Background(), townState(), now.Add(time.Hour))
	if res.Step != countessStepArmed || !res.Active {
		t.Fatalf("armed should not time out: %+v", res)
	}
}

func TestIsKnownRun(t *testing.T) {
	if !IsKnownRun("countess") {
		t.Fatal("countess should be known")
	}
	if IsKnownRun("mephisto") {
		t.Fatal("mephisto should not be known in 4.1")
	}
}

func TestKnownRunsStable(t *testing.T) {
	runs := KnownRuns()
	if len(runs) != 1 || runs[0] != "countess" {
		t.Fatalf("KnownRuns() = %v", runs)
	}
}
