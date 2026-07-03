package tasks

import (
	"context"
	"testing"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/pathing"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

func townState() world.State {
	return world.State{
		Valid: true,
		Phase: world.GamePhaseInGame,
		Area:  world.LookupArea(world.AreaID(1)), // Rogue Encampment
	}
}

func townStateWithWaypoint() world.State {
	st := townState()
	st.Player.Position = world.Position{X: 100, Y: 100}
	st.Objects = []world.Object{{Kind: world.ObjectKindWaypoint, UnitID: 10, Position: world.Position{X: 105, Y: 105}, Name: "Waypoint"}}
	return st
}

func townStateWithFarWaypoint() world.State {
	st := townState()
	st.Player.Position = world.Position{X: 100, Y: 100}
	st.Objects = []world.Object{{Kind: world.ObjectKindWaypoint, UnitID: 10, Position: world.Position{X: 150, Y: 150}, Name: "Waypoint"}}
	return st
}

func blackMarshState() world.State {
	return world.State{
		Valid: true,
		Phase: world.GamePhaseInGame,
		Area:  world.LookupArea(world.AreaID(6)), // Black Marsh
	}
}

type mockWaypointActions struct {
	results       []pathing.WaypointActionResult
	selectResult  pathing.WaypointActionResult
	tickCalls     int
	selectCalls   int
	resetCalls    int
	lastTickState world.State
}

type mockTownWalker struct {
	results []pathing.TownWalkResult
	resets  int
}

func (m *mockTownWalker) Reset() { m.resets++ }

func (m *mockTownWalker) TickAct1Waypoint(context.Context, world.State) pathing.TownWalkResult {
	if len(m.results) == 0 {
		return pathing.TownWalkResult{Status: pathing.TownWalkWaypointVisible, Done: true}
	}
	res := m.results[0]
	m.results = m.results[1:]
	return res
}

func (m *mockWaypointActions) Reset() { m.resetCalls++ }

func (m *mockWaypointActions) TickTownWaypoint(_ context.Context, st world.State) pathing.WaypointActionResult {
	m.tickCalls++
	m.lastTickState = st
	if len(m.results) == 0 {
		return pathing.WaypointActionResult{Status: pathing.WaypointActionClicked, Done: true}
	}
	res := m.results[0]
	m.results = m.results[1:]
	return res
}

func (m *mockWaypointActions) SelectBlackMarsh(context.Context) pathing.WaypointActionResult {
	m.selectCalls++
	if m.selectResult.Status == "" {
		return pathing.WaypointActionResult{Status: pathing.WaypointActionClicked, Done: true}
	}
	return m.selectResult
}

func newTestRunner(runName string) *Runner {
	return NewRunner(config.NewLogger("error"), RunSelection{Run: runName}, RunConfig{StepTimeout: 30 * time.Second}, Deps{})
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

func (w *waitRun) firstStep() string              { return "wait" }
func (w *waitRun) nextStep(string) string         { return "" }
func (w *waitRun) usesTickTimeout(string) bool    { return false }
func (w *waitRun) allowsNonInputTick(string) bool { return false }
func (w *waitRun) onStepEnter(string)             {}
func (w *waitRun) onTick(context.Context, Deps, string, world.State, time.Time, time.Time, int) stepResult {
	return stepResult{}
}

func TestRunnerWaitStepTimeout(t *testing.T) {
	r := NewRunner(config.NewLogger("error"), RunSelection{Run: "wait"}, RunConfig{StepTimeout: 5 * time.Millisecond}, Deps{})
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
	r := NewRunner(config.NewLogger("error"), RunSelection{Run: "countess"}, RunConfig{StepTimeout: 1 * time.Millisecond}, Deps{})
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

func TestCountessTravelMarshSuccessThroughLoading(t *testing.T) {
	wp := &mockWaypointActions{
		results: []pathing.WaypointActionResult{
			{Status: pathing.WaypointActionPending},
			{Status: pathing.WaypointActionClicked, Done: true},
		},
	}
	tw := &mockTownWalker{}
	r := NewRunner(config.NewLogger("error"), RunSelection{Run: "countess", Phase: CountessPhaseTravelMarsh}, RunConfig{StepTimeout: time.Second}, Deps{Waypoint: wp, TownWalk: tw})
	now := time.Now()

	res := r.Tick(context.Background(), townStateWithWaypoint(), now)
	if res.Step != countessStepAcquireTownWP || res.Outcome != RunOutcomeRunning {
		t.Fatalf("precheck tick = %+v, want acquire_town_waypoint running", res)
	}
	res = r.Tick(context.Background(), townStateWithWaypoint(), now.Add(50*time.Millisecond))
	if res.Step != countessStepOpenWaypoint {
		t.Fatalf("acquire tick = %+v, want open_waypoint", res)
	}
	res = r.Tick(context.Background(), townStateWithWaypoint(), now.Add(100*time.Millisecond))
	if res.Step != countessStepOpenWaypoint || wp.tickCalls != 1 {
		t.Fatalf("open pending = %+v tickCalls=%d", res, wp.tickCalls)
	}
	res = r.Tick(context.Background(), townStateWithWaypoint(), now.Add(200*time.Millisecond))
	if res.Step != countessStepSelectMarsh {
		t.Fatalf("open clicked = %+v, want select_black_marsh", res)
	}
	res = r.Tick(context.Background(), townStateWithWaypoint(), now.Add(300*time.Millisecond))
	if wp.selectCalls != 0 || res.Step != countessStepSelectMarsh {
		t.Fatalf("settle tick = %+v selectCalls=%d, want no click", res, wp.selectCalls)
	}
	res = r.Tick(context.Background(), townStateWithWaypoint(), now.Add(800*time.Millisecond))
	if wp.selectCalls != 1 || res.Step != countessStepWaitBlackMarsh {
		t.Fatalf("select tick = %+v selectCalls=%d, want wait_black_marsh", res, wp.selectCalls)
	}
	if !r.CurrentStepAllowsNonInputTick() {
		t.Fatal("wait_black_marsh should allow non-input ticks")
	}
	loading := world.State{Phase: world.GamePhaseLoading, Valid: false}
	res = r.Tick(context.Background(), loading, now.Add(900*time.Millisecond))
	if res.Step != countessStepWaitBlackMarsh || res.Outcome != RunOutcomeRunning {
		t.Fatalf("loading wait tick = %+v, want still waiting", res)
	}
	res = r.Tick(context.Background(), blackMarshState(), now.Add(time.Second))
	if res.Outcome != RunOutcomeSuccess {
		t.Fatalf("black marsh tick = %+v, want success", res)
	}
}

func TestCountessTravelMarshPrecheckFailsOutsideAct1Town(t *testing.T) {
	r := NewRunner(config.NewLogger("error"), RunSelection{Run: "countess", Phase: CountessPhaseTravelMarsh}, RunConfig{StepTimeout: time.Second}, Deps{})
	res := r.Tick(context.Background(), blackMarshState(), time.Now())
	if res.Outcome != RunOutcomeFailed || res.Reason != "not_act1_town" {
		t.Fatalf("tick = %+v, want not_act1_town failure", res)
	}
}

func TestCountessTravelMarshWaypointFailureReason(t *testing.T) {
	wp := &mockWaypointActions{
		results: []pathing.WaypointActionResult{{Status: pathing.WaypointActionHoverNotFound, Reason: string(pathing.WaypointActionHoverNotFound), Done: true}},
	}
	r := NewRunner(config.NewLogger("error"), RunSelection{Run: "countess", Phase: CountessPhaseTravelMarsh}, RunConfig{StepTimeout: time.Second}, Deps{Waypoint: wp, TownWalk: &mockTownWalker{}})
	now := time.Now()
	_ = r.Tick(context.Background(), townStateWithWaypoint(), now)
	_ = r.Tick(context.Background(), townStateWithWaypoint(), now.Add(time.Millisecond))
	res := r.Tick(context.Background(), townStateWithWaypoint(), now.Add(2*time.Millisecond))
	if res.Outcome != RunOutcomeFailed || res.Reason != string(pathing.WaypointActionHoverNotFound) {
		t.Fatalf("tick = %+v, want hover_not_found failure", res)
	}
}

func TestCountessTravelMarshAcquiresWaypointByTownWalk(t *testing.T) {
	wp := &mockWaypointActions{}
	tw := &mockTownWalker{results: []pathing.TownWalkResult{
		{Status: pathing.TownWalkPending},
		{Status: pathing.TownWalkWaypointVisible, Done: true},
	}}
	r := NewRunner(config.NewLogger("error"), RunSelection{Run: "countess", Phase: CountessPhaseTravelMarsh}, RunConfig{StepTimeout: time.Second}, Deps{Waypoint: wp, TownWalk: tw})
	now := time.Now()
	_ = r.Tick(context.Background(), townStateWithFarWaypoint(), now)
	res := r.Tick(context.Background(), townStateWithFarWaypoint(), now.Add(time.Millisecond))
	if res.Step != countessStepAcquireTownWP || res.Outcome != RunOutcomeRunning {
		t.Fatalf("pending acquire = %+v", res)
	}
	res = r.Tick(context.Background(), townStateWithFarWaypoint(), now.Add(2*time.Millisecond))
	if res.Step != countessStepOpenWaypoint {
		t.Fatalf("visible acquire = %+v, want open_waypoint", res)
	}
}
