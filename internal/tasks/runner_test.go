package tasks

import (
	"context"
	"errors"
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

func areaState(area world.AreaID) world.State {
	return world.State{
		Valid: true,
		Phase: world.GamePhaseInGame,
		Area:  world.LookupArea(area),
	}
}

func cellar5State(monsters ...world.Monster) world.State {
	st := areaState(world.TowerCellarLevel5)
	st.Player.Position = world.Position{X: 100, Y: 100}
	st.Monsters = monsters
	return st
}

func cellar5WithGoodChest() world.State {
	st := cellar5State()
	st.Objects = []world.Object{{
		Kind:     world.ObjectKindGoodChest,
		UnitID:   50,
		Position: world.Position{X: 120, Y: 120},
		Name:     "Good Chest",
	}}
	return st
}

func countessMonster(unitID uint32, pos world.Position) world.Monster {
	return world.Monster{
		NPCID:           world.DarkStalker,
		UnitID:          unitID,
		Position:        pos,
		MonsterTypeFlag: world.SuperUniqueMonsterFlag,
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

type mockNavigator struct {
	startGoals  []pathing.Goal
	startErr    error
	tickResults []pathing.NavTickResult
	resetCalls  int
}

type mockCombatActions struct {
	castCalls     int
	teleportCalls int
	resetCalls    int
	lastSkillID   uint16
	lastDesired   float64
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

func (m *mockNavigator) Ready() bool { return true }

func (m *mockNavigator) Start(goal pathing.Goal) error {
	m.startGoals = append(m.startGoals, goal)
	return m.startErr
}

func (m *mockNavigator) Tick(context.Context, world.State) pathing.NavTickResult {
	if len(m.tickResults) == 0 {
		return pathing.NavTickResult{Status: pathing.NavArrived, Done: true}
	}
	res := m.tickResults[0]
	m.tickResults = m.tickResults[1:]
	return res
}

func (m *mockNavigator) Active() bool { return len(m.tickResults) > 0 }

func (m *mockNavigator) Reset() { m.resetCalls++ }

func (m *mockCombatActions) CastSkillAtWorld(_ time.Time, skillID uint16, _, _ world.Position) error {
	m.castCalls++
	m.lastSkillID = skillID
	return nil
}

func (m *mockCombatActions) TeleportToward(_ time.Time, _ world.Position, _ world.Position, desiredDistanceTiles float64) error {
	m.teleportCalls++
	m.lastDesired = desiredDistanceTiles
	return nil
}

func (m *mockCombatActions) Reset() { m.resetCalls++ }

func newTestRunner(runName string) *Runner {
	return NewRunner(config.NewLogger("error"), RunSelection{Run: runName}, RunConfig{StepTimeout: 30 * time.Second}, Deps{})
}

func killRunConfig() RunConfig {
	return RunConfig{
		StepTimeout: time.Second,
		CountessCombat: CountessCombatConfig{
			AttackSkillID:           84,
			AttackInterval:          350 * time.Millisecond,
			EngageDistanceTiles:     22,
			RepositionDistanceTiles: 32,
			KillConfirmTicks:        3,
		},
	}
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

func TestCountessTravelCellar5SuccessStartsExpectedGoals(t *testing.T) {
	wp := &mockWaypointActions{}
	nav := &mockNavigator{}
	r := NewRunner(config.NewLogger("error"), RunSelection{Run: "countess", Phase: CountessPhaseTravelCellar5}, RunConfig{StepTimeout: 5 * time.Second}, Deps{
		Waypoint: wp,
		TownWalk: &mockTownWalker{},
		Pathing:  nav,
	})
	now := time.Now()

	ticks := []world.State{
		townStateWithWaypoint(),
		townStateWithWaypoint(),
		townStateWithWaypoint(),
		townStateWithWaypoint(),
		townStateWithWaypoint(),
		blackMarshState(),
		blackMarshState(),
		areaState(world.ForgottenTower),
		areaState(world.TowerCellarLevel1),
		areaState(world.TowerCellarLevel2),
		areaState(world.TowerCellarLevel3),
		areaState(world.TowerCellarLevel4),
		areaState(world.TowerCellarLevel4),
	}

	var res TickResult
	for i, st := range ticks {
		res = r.Tick(context.Background(), st, now.Add(time.Duration(i)*200*time.Millisecond))
	}
	if res.Outcome != RunOutcomeSuccess || res.Step != countessStepEnterCellar5 {
		t.Fatalf("final tick = %+v, want success at enter_cellar_5", res)
	}

	if len(nav.startGoals) > 1 {
		t.Fatalf("Start calls = %d, want at most 1 when snapshots already reach target areas", len(nav.startGoals))
	}
}

func TestCountessTravelCellar5AllowsBlackMarshLoadingWait(t *testing.T) {
	r := NewRunner(config.NewLogger("error"), RunSelection{Run: "countess", Phase: CountessPhaseTravelCellar5}, RunConfig{StepTimeout: 5 * time.Second}, Deps{Waypoint: &mockWaypointActions{}, TownWalk: &mockTownWalker{}, Pathing: &mockNavigator{}})
	now := time.Now()
	_ = r.Tick(context.Background(), townStateWithWaypoint(), now)
	_ = r.Tick(context.Background(), townStateWithWaypoint(), now.Add(100*time.Millisecond))
	_ = r.Tick(context.Background(), townStateWithWaypoint(), now.Add(200*time.Millisecond))
	_ = r.Tick(context.Background(), townStateWithWaypoint(), now.Add(300*time.Millisecond))
	_ = r.Tick(context.Background(), townStateWithWaypoint(), now.Add(800*time.Millisecond))

	if !r.CurrentStepAllowsNonInputTick() {
		t.Fatal("travel-cellar5 wait_black_marsh should allow non-input ticks")
	}
}

func TestCountessTravelCellar5ResumesFromRouteAreas(t *testing.T) {
	cases := []struct {
		name     string
		area     world.AreaID
		wantStep string
	}{
		{"black marsh", world.BlackMarsh, countessStepFindTower},
		{"forgotten tower", world.ForgottenTower, countessStepEnterCellar1},
		{"cellar 1", world.TowerCellarLevel1, countessStepEnterCellar2},
		{"cellar 2", world.TowerCellarLevel2, countessStepEnterCellar3},
		{"cellar 3", world.TowerCellarLevel3, countessStepEnterCellar4},
		{"cellar 4", world.TowerCellarLevel4, countessStepEnterCellar5},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := NewRunner(config.NewLogger("error"), RunSelection{Run: "countess", Phase: CountessPhaseTravelCellar5}, RunConfig{StepTimeout: 5 * time.Second}, Deps{
				Waypoint: &mockWaypointActions{},
				TownWalk: &mockTownWalker{},
				Pathing:  &mockNavigator{tickResults: []pathing.NavTickResult{{Status: pathing.NavExploring}}},
			})

			res := r.Tick(context.Background(), areaState(tc.area), time.Now())
			if res.Outcome != RunOutcomeRunning || res.Step != tc.wantStep {
				t.Fatalf("tick = %+v, want running step %s", res, tc.wantStep)
			}
		})
	}
}

func TestCountessTravelCellar5ResumeAlreadyAtCellar5Completes(t *testing.T) {
	r := NewRunner(config.NewLogger("error"), RunSelection{Run: "countess", Phase: CountessPhaseTravelCellar5}, RunConfig{StepTimeout: 5 * time.Second}, Deps{
		Waypoint: &mockWaypointActions{},
		TownWalk: &mockTownWalker{},
		Pathing:  &mockNavigator{},
	})

	res := r.Tick(context.Background(), areaState(world.TowerCellarLevel5), time.Now())
	if res.Outcome != RunOutcomeSuccess || res.Step != countessStepPrecheck {
		t.Fatalf("tick = %+v, want success from precheck at cellar 5", res)
	}
}

func TestCountessKillPrecheckRequiresCellar5AndCombat(t *testing.T) {
	r := NewRunner(config.NewLogger("error"), RunSelection{Run: "countess", Phase: CountessPhaseKillCountess}, killRunConfig(), Deps{Combat: &mockCombatActions{}})
	res := r.Tick(context.Background(), areaState(world.TowerCellarLevel4), time.Now())
	if res.Outcome != RunOutcomeFailed || res.Reason != "not_cellar_5" {
		t.Fatalf("wrong area tick = %+v, want not_cellar_5", res)
	}

	r = NewRunner(config.NewLogger("error"), RunSelection{Run: "countess", Phase: CountessPhaseKillCountess}, killRunConfig(), Deps{})
	res = r.Tick(context.Background(), cellar5State(), time.Now())
	if res.Outcome != RunOutcomeFailed || res.Reason != "combat_not_wired" {
		t.Fatalf("missing combat tick = %+v, want combat_not_wired", res)
	}
}

func TestCountessKillTargetsDarkStalkerBeforeGenericSuperUnique(t *testing.T) {
	combat := &mockCombatActions{}
	r := NewRunner(config.NewLogger("error"), RunSelection{Run: "countess", Phase: CountessPhaseKillCountess}, killRunConfig(), Deps{Combat: combat})
	now := time.Now()
	generic := world.Monster{NPCID: 999, UnitID: 20, Position: world.Position{X: 101, Y: 101}, MonsterTypeFlag: world.SuperUniqueMonsterFlag}
	countess := countessMonster(10, world.Position{X: 110, Y: 100})
	st := cellar5State(generic, countess)

	_ = r.Tick(context.Background(), st, now)
	_ = r.Tick(context.Background(), st, now.Add(time.Millisecond))
	_ = r.Tick(context.Background(), cellar5State(generic), now.Add(2*time.Millisecond))
	_ = r.Tick(context.Background(), cellar5State(generic), now.Add(3*time.Millisecond))
	res := r.Tick(context.Background(), cellar5State(generic), now.Add(4*time.Millisecond))
	if res.Outcome != RunOutcomeSuccess {
		t.Fatalf("tick = %+v, want success after stored Countess target disappears", res)
	}
}

func TestCountessKillGoodChestFallbackStartsOnceAndTicksPathing(t *testing.T) {
	nav := &mockNavigator{tickResults: []pathing.NavTickResult{
		{Status: pathing.NavMoving},
		{Status: pathing.NavArrived, Done: true},
	}}
	r := NewRunner(config.NewLogger("error"), RunSelection{Run: "countess", Phase: CountessPhaseKillCountess}, killRunConfig(), Deps{
		Combat:  &mockCombatActions{},
		Pathing: nav,
	})
	now := time.Now()
	st := cellar5WithGoodChest()

	_ = r.Tick(context.Background(), st, now)
	_ = r.Tick(context.Background(), st, now.Add(time.Millisecond))
	_ = r.Tick(context.Background(), st, now.Add(2*time.Millisecond))
	if len(nav.startGoals) != 1 {
		t.Fatalf("Start calls = %d, want 1", len(nav.startGoals))
	}
	if nav.startGoals[0].Kind != pathing.GoalKindMoveToPosition || nav.startGoals[0].TargetPos != st.Objects[0].Position {
		t.Fatalf("goal = %+v, want move to good chest position", nav.startGoals[0])
	}
}

func TestCountessKillEntranceFallbackMovesInWhenChestMissing(t *testing.T) {
	nav := &mockNavigator{tickResults: []pathing.NavTickResult{
		{Status: pathing.NavMoving},
	}}
	r := NewRunner(config.NewLogger("error"), RunSelection{Run: "countess", Phase: CountessPhaseKillCountess}, killRunConfig(), Deps{
		Combat:  &mockCombatActions{},
		Pathing: nav,
	})
	now := time.Now()
	st := cellar5State()
	st.Entrances = []world.Entrance{{
		Kind:     world.EntranceKindTowerCellarDown,
		UnitID:   37,
		Position: world.Position{X: 12641, Y: 9556},
		Name:     "Act 1 Tower Cellar Down",
	}}

	_ = r.Tick(context.Background(), st, now)
	_ = r.Tick(context.Background(), st, now.Add(time.Millisecond))
	if len(nav.startGoals) != 1 {
		t.Fatalf("Start calls = %d, want 1", len(nav.startGoals))
	}
	if nav.startGoals[0].Kind != pathing.GoalKindMoveToPosition || nav.startGoals[0].TargetPos != st.Entrances[0].Position {
		t.Fatalf("goal = %+v, want move to cellar down search anchor", nav.startGoals[0])
	}
}

func TestCountessKillEngageCastsEveryTaskTickAndConfirmsKill(t *testing.T) {
	combat := &mockCombatActions{}
	r := NewRunner(config.NewLogger("error"), RunSelection{Run: "countess", Phase: CountessPhaseKillCountess}, killRunConfig(), Deps{Combat: combat})
	now := time.Now()
	target := countessMonster(10, world.Position{X: 110, Y: 100})
	visible := cellar5State(target)

	_ = r.Tick(context.Background(), visible, now)
	_ = r.Tick(context.Background(), visible, now.Add(time.Millisecond))
	_ = r.Tick(context.Background(), visible, now.Add(2*time.Millisecond))
	_ = r.Tick(context.Background(), visible, now.Add(3*time.Millisecond))
	if combat.castCalls != 2 || combat.lastSkillID != 84 {
		t.Fatalf("castCalls=%d skill=%d, want two task-level casts with Bone Spear", combat.castCalls, combat.lastSkillID)
	}

	absent := cellar5State()
	_ = r.Tick(context.Background(), absent, now.Add(4*time.Millisecond))
	_ = r.Tick(context.Background(), absent, now.Add(5*time.Millisecond))
	res := r.Tick(context.Background(), absent, now.Add(6*time.Millisecond))
	if res.Outcome != RunOutcomeSuccess {
		t.Fatalf("tick = %+v, want success after absent ticks", res)
	}
}

func TestCountessKillAbsenceResetAndAreaFailure(t *testing.T) {
	combat := &mockCombatActions{}
	r := NewRunner(config.NewLogger("error"), RunSelection{Run: "countess", Phase: CountessPhaseKillCountess}, killRunConfig(), Deps{Combat: combat})
	now := time.Now()
	target := countessMonster(10, world.Position{X: 110, Y: 100})
	visible := cellar5State(target)
	absent := cellar5State()

	_ = r.Tick(context.Background(), visible, now)
	_ = r.Tick(context.Background(), visible, now.Add(time.Millisecond))
	_ = r.Tick(context.Background(), absent, now.Add(2*time.Millisecond))
	_ = r.Tick(context.Background(), absent, now.Add(3*time.Millisecond))
	res := r.Tick(context.Background(), visible, now.Add(4*time.Millisecond))
	if res.Outcome == RunOutcomeSuccess {
		t.Fatalf("tick = %+v, absence should reset when target reappears", res)
	}
	res = r.Tick(context.Background(), areaState(world.TowerCellarLevel4), now.Add(5*time.Millisecond))
	if res.Outcome != RunOutcomeFailed || res.Reason != "unexpected_area" {
		t.Fatalf("area tick = %+v, want unexpected_area", res)
	}
}

func TestCountessKillTeleportRepositionUsesEngageDistance(t *testing.T) {
	combat := &mockCombatActions{}
	r := NewRunner(config.NewLogger("error"), RunSelection{Run: "countess", Phase: CountessPhaseKillCountess}, killRunConfig(), Deps{Combat: combat})
	now := time.Now()
	target := countessMonster(10, world.Position{X: 200, Y: 100})
	st := cellar5State(target)

	_ = r.Tick(context.Background(), st, now)
	_ = r.Tick(context.Background(), st, now.Add(time.Millisecond))
	_ = r.Tick(context.Background(), st, now.Add(2*time.Millisecond))
	if combat.teleportCalls != 1 || combat.lastDesired != 22 {
		t.Fatalf("teleports=%d desired=%.1f, want one teleport toward engage distance", combat.teleportCalls, combat.lastDesired)
	}
}

func TestCountessNavigateAreaStartsOnceWhilePending(t *testing.T) {
	nav := &mockNavigator{tickResults: []pathing.NavTickResult{
		{Status: pathing.NavExploring},
		{Status: pathing.NavExploring},
	}}
	c := &countessRun{}
	goal := pathing.Goal{Kind: pathing.GoalKindMoveToArea, TargetArea: world.ForgottenTower, ViaEntrance: world.EntranceKindWildernessToTower}

	if res := c.tickNavigateArea(context.Background(), Deps{Pathing: nav}, blackMarshState(), goal); res.complete || res.failed {
		t.Fatalf("first tick = %+v, want pending", res)
	}
	if res := c.tickNavigateArea(context.Background(), Deps{Pathing: nav}, blackMarshState(), goal); res.complete || res.failed {
		t.Fatalf("second tick = %+v, want pending", res)
	}
	if len(nav.startGoals) != 1 {
		t.Fatalf("Start calls = %d, want 1", len(nav.startGoals))
	}
}

func TestCountessNavigationGoalsStartExpectedPathingGoals(t *testing.T) {
	cases := []struct {
		step        string
		source      world.AreaID
		target      world.AreaID
		viaEntrance world.EntranceKind
	}{
		{countessStepFindTower, world.BlackMarsh, world.ForgottenTower, world.EntranceKindWildernessToTower},
		{countessStepEnterCellar1, world.ForgottenTower, world.TowerCellarLevel1, world.EntranceKindUnknown},
		{countessStepEnterCellar2, world.TowerCellarLevel1, world.TowerCellarLevel2, world.EntranceKindTowerCellarDown},
		{countessStepEnterCellar3, world.TowerCellarLevel2, world.TowerCellarLevel3, world.EntranceKindTowerCellarDown},
		{countessStepEnterCellar4, world.TowerCellarLevel3, world.TowerCellarLevel4, world.EntranceKindTowerCellarDown},
		{countessStepEnterCellar5, world.TowerCellarLevel4, world.TowerCellarLevel5, world.EntranceKindTowerCellarDown},
	}
	for _, tc := range cases {
		t.Run(tc.step, func(t *testing.T) {
			c := &countessRun{}
			nav := &mockNavigator{tickResults: []pathing.NavTickResult{{Status: pathing.NavExploring}}}
			goal, ok := countessNavigationGoal(tc.step)
			if !ok {
				t.Fatalf("missing goal for %s", tc.step)
			}
			res := c.tickNavigateArea(context.Background(), Deps{Pathing: nav}, areaState(tc.source), goal)
			if res.failed || res.complete {
				t.Fatalf("tick = %+v, want pending", res)
			}
			if len(nav.startGoals) != 1 {
				t.Fatalf("Start calls = %d, want 1", len(nav.startGoals))
			}
			got := nav.startGoals[0]
			if got.TargetArea != tc.target || got.ViaEntrance != tc.viaEntrance {
				t.Fatalf("goal = %+v, want target=%s via=%s", got, tc.target, tc.viaEntrance)
			}
		})
	}
}

func TestCountessEnterCellar1UsesForgottenTowerSpecialCase(t *testing.T) {
	c := &countessRun{}
	nav := &mockNavigator{tickResults: []pathing.NavTickResult{
		{Status: pathing.NavClicking},
	}}
	goal, ok := countessNavigationGoal(countessStepEnterCellar1)
	if !ok {
		t.Fatal("missing enter_cellar_1 goal")
	}

	res := c.tickNavigateArea(context.Background(), Deps{Pathing: nav}, areaState(world.ForgottenTower), goal)
	if res.failed || res.complete {
		t.Fatalf("tick = %+v, want pending", res)
	}
	if len(nav.startGoals) != 1 {
		t.Fatalf("Start calls = %d, want 1", len(nav.startGoals))
	}
	got := nav.startGoals[0]
	if got.Kind != pathing.GoalKindMoveToArea ||
		got.TargetArea != world.TowerCellarLevel1 ||
		got.ViaEntrance != world.EntranceKindUnknown {
		t.Fatalf("goal = %+v, want Tower Cellar Level 1 with special-case entrance selection", got)
	}
}

func TestCountessNavigateAreaCompletesWithoutStartWhenAlreadyInTarget(t *testing.T) {
	nav := &mockNavigator{}
	c := &countessRun{}
	goal := pathing.Goal{Kind: pathing.GoalKindMoveToArea, TargetArea: world.ForgottenTower, ViaEntrance: world.EntranceKindWildernessToTower}

	res := c.tickNavigateArea(context.Background(), Deps{Pathing: nav}, areaState(world.ForgottenTower), goal)
	if !res.complete || res.failed {
		t.Fatalf("tick = %+v, want complete", res)
	}
	if len(nav.startGoals) != 0 {
		t.Fatalf("Start calls = %d, want 0", len(nav.startGoals))
	}
}

func TestCountessNavigationSourceGuardFailsUnexpectedArea(t *testing.T) {
	goal, ok := countessNavigationGoal(countessStepFindTower)
	if !ok {
		t.Fatal("missing find_tower goal")
	}
	res := countessNavigationSourceGuard(countessStepFindTower, areaState(world.TamoeHighland), goal)
	if !res.failed || res.reason != "unexpected_area" {
		t.Fatalf("guard = %+v, want unexpected_area failure", res)
	}
}

func TestCountessNavigateAreaFailureReasons(t *testing.T) {
	cases := []struct {
		name string
		res  pathing.NavTickResult
		want string
	}{
		{"stuck", pathing.NavTickResult{Status: pathing.NavStuck, Reason: pathing.ReasonStuck, Done: true}, pathing.ReasonStuck},
		{"hover", pathing.NavTickResult{Status: pathing.NavFailed, Reason: pathing.ReasonHoverNotFound, Done: true}, pathing.ReasonHoverNotFound},
		{"projection", pathing.NavTickResult{Status: pathing.NavFailed, Reason: pathing.ReasonProjectionFailed, Done: true}, pathing.ReasonProjectionFailed},
		{"cancelled", pathing.NavTickResult{Status: pathing.NavFailed, Reason: pathing.ReasonCancelled, Done: true}, pathing.ReasonCancelled},
		{"invalid", pathing.NavTickResult{Status: pathing.NavFailed, Reason: pathing.ReasonInvalidGoal, Done: true}, pathing.ReasonInvalidGoal},
	}
	goal := pathing.Goal{Kind: pathing.GoalKindMoveToArea, TargetArea: world.ForgottenTower, ViaEntrance: world.EntranceKindWildernessToTower}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := &countessRun{}
			nav := &mockNavigator{tickResults: []pathing.NavTickResult{tc.res}}
			res := c.tickNavigateArea(context.Background(), Deps{Pathing: nav}, blackMarshState(), goal)
			if !res.failed || res.reason != tc.want {
				t.Fatalf("tick = %+v, want failure %q", res, tc.want)
			}
		})
	}
}

func TestCountessNavigateAreaStartFailureReasons(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want string
	}{
		{"not wired", pathing.ErrNavigatorNotWired, "pathing_not_wired"},
		{"invalid", errors.New(pathing.ReasonInvalidGoal + ": target area required"), pathing.ReasonInvalidGoal},
		{"other", errors.New("boom"), "pathing_start_failed"},
	}
	goal := pathing.Goal{Kind: pathing.GoalKindMoveToArea, TargetArea: world.ForgottenTower, ViaEntrance: world.EntranceKindWildernessToTower}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := &countessRun{}
			nav := &mockNavigator{startErr: tc.err}
			res := c.tickNavigateArea(context.Background(), Deps{Pathing: nav}, blackMarshState(), goal)
			if !res.failed || res.reason != tc.want {
				t.Fatalf("tick = %+v, want failure %q", res, tc.want)
			}
		})
	}
}

func TestRunnerResetsPathingLifecycle(t *testing.T) {
	nav := &mockNavigator{}
	combat := &mockCombatActions{}
	r := NewRunner(config.NewLogger("error"), RunSelection{Run: "countess"}, RunConfig{StepTimeout: time.Second}, Deps{Pathing: nav, Combat: combat})

	r.Reset("process_lost")
	if nav.resetCalls != 1 {
		t.Fatalf("Reset calls after Runner.Reset = %d, want 1", nav.resetCalls)
	}
	if combat.resetCalls != 1 {
		t.Fatalf("Combat Reset calls after Runner.Reset = %d, want 1", combat.resetCalls)
	}

	nav.resetCalls = 0
	combat.resetCalls = 0
	r = NewRunner(config.NewLogger("error"), RunSelection{Run: "countess"}, RunConfig{StepTimeout: time.Second}, Deps{Pathing: nav, Combat: combat})
	_ = r.Tick(context.Background(), townState(), time.Now())
	if nav.resetCalls != 2 {
		t.Fatalf("Reset calls after beginStep chain = %d, want 2", nav.resetCalls)
	}
	if combat.resetCalls != 2 {
		t.Fatalf("Combat Reset calls after beginStep chain = %d, want 2", combat.resetCalls)
	}

	nav.resetCalls = 0
	combat.resetCalls = 0
	r = NewRunner(config.NewLogger("error"), RunSelection{Run: "countess"}, RunConfig{StepTimeout: time.Second}, Deps{Pathing: nav, Combat: combat})
	_ = r.Tick(context.Background(), blackMarshState(), time.Now())
	if nav.resetCalls != 2 {
		t.Fatalf("Reset calls after failed step = %d, want 2 (begin + failure)", nav.resetCalls)
	}
	if combat.resetCalls != 2 {
		t.Fatalf("Combat Reset calls after failed step = %d, want 2 (begin + failure)", combat.resetCalls)
	}
}
