package tasks

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/pathing"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/profile"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/town"
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

func healthy(st world.State) world.State {
	st.Player.HP = 100
	st.Player.MaxHP = 100
	return st
}

func hpState(hp, maxHP uint32) world.State {
	st := townState()
	st.Player.HP = hp
	st.Player.MaxHP = maxHP
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
	results        []pathing.WaypointActionResult
	selectResult   pathing.WaypointActionResult
	tickCalls      int
	selectCalls    int
	selectedTarget pathing.WaypointTargetID
	resetCalls     int
	lastTickState  world.State
}

type mockTownWalker struct {
	results []pathing.TownWalkResult
	resets  int
	calls   int
}

type mockProfileActions struct {
	hookResults     []profile.Result
	hookCalls       int
	hooks           []profile.Hook
	targets         []profile.EncounterTarget
	resourceResults []profile.Result
	resetCalls      int
}

type mockTownPortalActions struct {
	results []pathing.TownPortalActionResult
	calls   int
	resets  int
}

type mockPersonalStashActions struct {
	results []pathing.PersonalStashResult
	calls   int
	resets  int
}

type mockNavigator struct {
	startGoals  []pathing.Goal
	startErr    error
	tickResults []pathing.NavTickResult
	resetCalls  int
}

type mockRoutePlayback struct {
	startedID  string
	startErr   error
	tickErr    error
	resetCalls int
}

func (m *mockRoutePlayback) Start(routeID string, _ world.State) error {
	m.startedID = routeID
	return m.startErr
}
func (m *mockRoutePlayback) Tick(_ context.Context, state world.State) (bool, error) {
	return state.Area.ID == world.TowerCellarLevel5, m.tickErr
}
func (m *mockRoutePlayback) Reset() { m.resetCalls++ }

type mockCombatActions struct {
	castCalls          int
	castSkills         []uint16
	teleportCalls      int
	stopCalls          int
	resetCalls         int
	lastSkillID        uint16
	lastDesired        float64
	lastTeleportTarget world.Position
	teleportSent       []bool
}

type mockRunActions struct {
	beltCalls   []int
	portalCalls int
	beltErr     error
	portalErr   error
}

type mockLootActions struct {
	scans       []LootScanResult
	ticks       []LootPickupResult
	startErr    error
	resetCalls  int
	scanCalls   int
	startCalls  []LootTarget
	tickCalls   int
	lastTickNow time.Time
	stashTicks  []LootStashResult
	closeTicks  []LootStashResult
}

type countingRun struct {
	onTickCalls int
	result      stepResult
}

type tickRun struct{}

func (m *mockTownWalker) Reset() { m.resets++ }

func (m *mockTownPortalActions) Reset() { m.resets++ }

func (m *mockPersonalStashActions) Reset() { m.resets++ }

func (m *mockPersonalStashActions) Tick(context.Context, world.State) pathing.PersonalStashResult {
	m.calls++
	if len(m.results) == 0 {
		return pathing.PersonalStashResult{Status: pathing.PersonalStashOpened, Done: true}
	}
	res := m.results[0]
	m.results = m.results[1:]
	return res
}

func (m *mockTownPortalActions) Tick(context.Context, world.State, time.Time) pathing.TownPortalActionResult {
	m.calls++
	if len(m.results) == 0 {
		return pathing.TownPortalActionResult{Status: pathing.TownPortalActionClicked, Done: true}
	}
	res := m.results[0]
	m.results = m.results[1:]
	return res
}

func (m *mockTownWalker) TickAct1Waypoint(context.Context, world.State) pathing.TownWalkResult {
	m.calls++
	if len(m.results) == 0 {
		return pathing.TownWalkResult{Status: pathing.TownWalkWaypointVisible, Done: true}
	}
	res := m.results[0]
	m.results = m.results[1:]
	return res
}

func (m *mockProfileActions) TickHook(_ context.Context, hook profile.Hook, _ world.State, target profile.EncounterTarget, _ time.Time) profile.Result {
	m.hookCalls++
	m.hooks = append(m.hooks, hook)
	m.targets = append(m.targets, target)
	if len(m.hookResults) == 0 {
		return profile.Result{Status: profile.StatusComplete}
	}
	res := m.hookResults[0]
	m.hookResults = m.hookResults[1:]
	return res
}

func (m *mockProfileActions) TickResources(world.State, time.Time) profile.Result {
	if len(m.resourceResults) == 0 {
		return profile.Result{Status: profile.StatusComplete}
	}
	res := m.resourceResults[0]
	m.resourceResults = m.resourceResults[1:]
	return res
}

func (m *mockProfileActions) Reset() { m.resetCalls++ }

func (m *mockWaypointActions) Reset() { m.resetCalls++ }

type mockTownPreparationActions struct{ calls, resets int }

type mockTownEgressPlayback struct {
	starts []town.OriginAct
	ticks  int
	resets int
	done   bool
	err    error
}

func (m *mockTownEgressPlayback) Start(act town.OriginAct, _ world.State) error {
	m.starts = append(m.starts, act)
	return m.err
}
func (m *mockTownEgressPlayback) Tick(context.Context, world.State) (bool, error) {
	m.ticks++
	return m.done, m.err
}
func (m *mockTownEgressPlayback) Reset() { m.resets++ }

func (m *mockTownPreparationActions) Tick(context.Context, world.State) TownPreparationResult {
	m.calls++
	return TownPreparationResult{Status: "complete", Done: true}
}
func (m *mockTownPreparationActions) Reset() { m.resets++ }

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

func (m *mockWaypointActions) SelectWaypointTarget(_ context.Context, _ world.State, target pathing.WaypointTargetID, _ time.Time) pathing.WaypointActionResult {
	m.selectCalls++
	m.selectedTarget = target
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

func (m *mockCombatActions) CastAttackAtWorld(_ time.Time, skillID uint16, _ world.Player, _ world.Position) error {
	m.castCalls++
	m.castSkills = append(m.castSkills, skillID)
	m.lastSkillID = skillID
	return nil
}

func (m *mockCombatActions) CastSkillAtWorld(_ time.Time, skillID uint16, _, _ world.Position) error {
	m.castCalls++
	m.castSkills = append(m.castSkills, skillID)
	m.lastSkillID = skillID
	return nil
}

func (m *mockCombatActions) CastBelt(int) error { return nil }

func (m *mockCombatActions) StopAttack() error {
	m.stopCalls++
	return nil
}

func (m *mockCombatActions) TeleportToward(_ time.Time, _ world.Position, target world.Position, desiredDistanceTiles float64) (bool, error) {
	m.teleportCalls++
	m.lastDesired = desiredDistanceTiles
	m.lastTeleportTarget = target
	if len(m.teleportSent) == 0 {
		return true, nil
	}
	sent := m.teleportSent[0]
	m.teleportSent = m.teleportSent[1:]
	return sent, nil
}

func (m *mockCombatActions) Reset() { m.resetCalls++ }

func (m *mockRunActions) CastBelt(slot int) error {
	m.beltCalls = append(m.beltCalls, slot)
	return m.beltErr
}

func (m *mockRunActions) CastTownPortal() error {
	m.portalCalls++
	return m.portalErr
}

func (m *mockLootActions) Scan(world.State) LootScanResult {
	m.scanCalls++
	if len(m.scans) == 0 {
		return LootScanResult{}
	}
	res := m.scans[0]
	m.scans = m.scans[1:]
	return res
}

func (m *mockLootActions) StartPickup(target LootTarget) error {
	m.startCalls = append(m.startCalls, target)
	return m.startErr
}

func (m *mockLootActions) TickPickup(_ world.State, now time.Time) LootPickupResult {
	m.tickCalls++
	m.lastTickNow = now
	if len(m.ticks) == 0 {
		return LootPickupResult{Status: LootPickupPending}
	}
	res := m.ticks[0]
	m.ticks = m.ticks[1:]
	return res
}

func (m *mockLootActions) Reset() { m.resetCalls++ }

func (m *mockLootActions) TickStash(world.State, time.Time) LootStashResult {
	if len(m.stashTicks) == 0 {
		return LootStashResult{Status: LootStashSuccess, Done: true}
	}
	res := m.stashTicks[0]
	m.stashTicks = m.stashTicks[1:]
	return res
}

func (m *mockLootActions) TickCloseStash(world.State, time.Time) LootStashResult {
	if len(m.closeTicks) == 0 {
		return LootStashResult{Status: LootStashClosed, Done: true}
	}
	res := m.closeTicks[0]
	m.closeTicks = m.closeTicks[1:]
	return res
}

func (r *countingRun) firstStep() string              { return "count" }
func (r *countingRun) nextStep(string) string         { return "" }
func (r *countingRun) usesTickTimeout(string) bool    { return false }
func (r *countingRun) allowsNonInputTick(string) bool { return false }
func (r *countingRun) onStepEnter(string)             {}
func (r *countingRun) onTick(context.Context, Deps, string, world.State, time.Time, time.Time, int) stepResult {
	r.onTickCalls++
	return r.result
}

func (r *tickRun) firstStep() string              { return "tick" }
func (r *tickRun) nextStep(string) string         { return "" }
func (r *tickRun) usesTickTimeout(string) bool    { return true }
func (r *tickRun) allowsNonInputTick(string) bool { return false }
func (r *tickRun) onStepEnter(string)             {}
func (r *tickRun) onTick(context.Context, Deps, string, world.State, time.Time, time.Time, int) stepResult {
	return stepResult{}
}

func newTestRunner(runName string) *Runner {
	return NewRunner(config.NewLogger("error"), RunSelection{Run: runName}, RunConfig{StepTimeout: 30 * time.Second}, Deps{})
}

func killRunConfig() RunConfig {
	return RunConfig{
		StepTimeout: time.Second,
		RouteID:     "test-countess-route",
		Combat: CombatConfig{
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

func TestRunnerLazyStartBeginsFullCountessRun(t *testing.T) {
	r := newTestRunner("countess")
	now := time.Now()

	if r.started {
		t.Fatal("run should not start before first tick")
	}

	res := r.Tick(context.Background(), townState(), now)
	if !res.Active || res.Outcome != RunOutcomeRunning || res.Step != pipelineStepAcquireTownWaypoint {
		t.Fatalf("first tick = %+v, want acquire_town_waypoint after precheck completes same tick", res)
	}
	if !r.started {
		t.Fatal("expected started after first tick")
	}
}

func TestCountessStashPersonalTransfersClosesAndSucceeds(t *testing.T) {
	stash := &mockPersonalStashActions{results: []pathing.PersonalStashResult{
		{Status: pathing.PersonalStashPending},
		{Status: pathing.PersonalStashOpened, Done: true},
	}}
	lootActions := &mockLootActions{}
	r := NewRunner(config.NewLogger("error"), RunSelection{Run: "countess", Phase: RunPhaseStashPersonal}, RunConfig{StepTimeout: time.Second}, Deps{Stash: stash, Loot: lootActions})
	now := time.Now()

	res := r.Tick(context.Background(), townState(), now)
	if res.Step != pipelineStepOpenStash || res.Outcome != RunOutcomeRunning {
		t.Fatalf("precheck tick = %+v, want open_personal_stash", res)
	}
	res = r.Tick(context.Background(), townState(), now.Add(time.Millisecond))
	if res.Step != pipelineStepOpenStash || res.Outcome != RunOutcomeRunning {
		t.Fatalf("pending tick = %+v, want open_personal_stash", res)
	}
	res = r.Tick(context.Background(), townState(), now.Add(2*time.Millisecond))
	if res.Step != pipelineStepStashItems || res.Outcome != RunOutcomeRunning {
		t.Fatalf("opened tick = %+v, want stash_items", res)
	}
	res = r.Tick(context.Background(), townState(), now.Add(3*time.Millisecond))
	if res.Step != pipelineStepCloseStash || res.Outcome != RunOutcomeRunning {
		t.Fatalf("stash tick = %+v, want close_personal_stash", res)
	}
	res = r.Tick(context.Background(), townState(), now.Add(4*time.Millisecond))
	if res.Step != pipelineStepComplete || res.Outcome != RunOutcomeRunning {
		t.Fatalf("close tick = %+v, want complete", res)
	}
	res = r.Tick(context.Background(), townState(), now.Add(5*time.Millisecond))
	if res.Outcome != RunOutcomeSuccess || stash.calls != 2 {
		t.Fatalf("final tick = %+v calls=%d, want success after two stash ticks", res, stash.calls)
	}
}

func TestCountessStashPersonalPreservesStableFailureReason(t *testing.T) {
	stash := &mockPersonalStashActions{results: []pathing.PersonalStashResult{{
		Status: pathing.PersonalStashUnsupportedResolution, Done: true,
	}}}
	r := NewRunner(config.NewLogger("error"), RunSelection{Run: "countess", Phase: RunPhaseStashPersonal}, RunConfig{StepTimeout: time.Second}, Deps{Stash: stash, Loot: &mockLootActions{}})
	now := time.Now()
	_ = r.Tick(context.Background(), townState(), now)
	res := r.Tick(context.Background(), townState(), now.Add(time.Millisecond))
	if res.Outcome != RunOutcomeFailed || res.Reason != "unsupported_resolution" {
		t.Fatalf("tick = %+v, want unsupported_resolution", res)
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
	if res.Reason != "not_act1_town" {
		t.Fatalf("Reason = %q, want not_act1_town", res.Reason)
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
	r := NewRunner(config.NewLogger("error"), RunSelection{Run: "tick"}, RunConfig{StepTimeout: 1 * time.Millisecond}, Deps{})
	r.run = &tickRun{}
	now := time.Now()

	res := r.Tick(context.Background(), townState(), now)
	if res.Step != "tick" {
		t.Fatalf("first tick step = %q, want tick", res.Step)
	}

	// Armed uses tick counter, not time timeout — still running after long elapsed time.
	res = r.Tick(context.Background(), townState(), now.Add(time.Hour))
	if res.Step != "tick" || !res.Active {
		t.Fatalf("tick step should not time out: %+v", res)
	}
}

func TestIsKnownRun(t *testing.T) {
	if !IsKnownRun("countess") {
		t.Fatal("countess should be known")
	}
	if !IsKnownRun("mephisto") {
		t.Fatal("mephisto metadata should be registered in Phase 10.1")
	}
}

func TestKnownRunsStable(t *testing.T) {
	runs := KnownRuns()
	if !reflect.DeepEqual(runs, []string{"countess", "mephisto"}) {
		t.Fatalf("KnownRuns() = %v", runs)
	}
}

func TestCountessFullRunSuccessCastsTownPortal(t *testing.T) {
	actions := &mockRunActions{}
	portals := &mockTownPortalActions{}
	preparation := &mockTownPreparationActions{}
	cfg := killRunConfig()
	cfg.StepTimeout = 30 * time.Second
	r := NewRunner(config.NewLogger("error"), RunSelection{Run: "countess"}, cfg, Deps{
		Waypoint: &mockWaypointActions{},
		TownWalk: &mockTownWalker{},
		Pathing:  &mockNavigator{},
		Combat:   &mockCombatActions{},
		Actions:  actions,
		Portal:   portals,
		Stash:    &mockPersonalStashActions{},
		Loot:     &mockLootActions{scans: []LootScanResult{{GroundItemCount: 0, CandidateCount: 0}}},
		Route:    &mockRoutePlayback{},
		Town:     preparation,
	})
	now := time.Now()
	target := countessMonster(10, world.Position{X: 110, Y: 100})
	arrivedAtCountess := healthy(cellar5State())
	arrivedAtCountess.Player.Position = target.Position

	ticks := []world.State{
		healthy(townStateWithWaypoint()),
		healthy(townStateWithWaypoint()),
		healthy(townStateWithWaypoint()),
		healthy(townStateWithWaypoint()),
		healthy(townStateWithWaypoint()),
		healthy(blackMarshState()),
		healthy(blackMarshState()),
		healthy(areaState(world.ForgottenTower)),
		healthy(areaState(world.TowerCellarLevel1)),
		healthy(areaState(world.TowerCellarLevel2)),
		healthy(areaState(world.TowerCellarLevel3)),
		healthy(areaState(world.TowerCellarLevel4)),
		healthy(cellar5State(target)),
		healthy(cellar5State(target)),
		healthy(cellar5State()),
		healthy(cellar5State()),
		healthy(cellar5State()),
		arrivedAtCountess,
		healthy(cellar5State()),
		healthy(cellar5State()),
		healthy(cellar5State()),
		healthy(cellar5State()),
		healthy(cellar5State()),
		healthy(cellar5State()),
		healthy(cellar5State()),
		healthy(cellar5State()),
		healthy(townState()),
		healthy(townState()),
		healthy(townState()),
		healthy(townState()),
		healthy(townState()),
		healthy(townState()),
		healthy(townState()),
		healthy(townState()),
	}

	var res TickResult
	for i, st := range ticks {
		res = r.Tick(context.Background(), st, now.Add(time.Duration(i)*time.Second))
	}
	if res.Outcome != RunOutcomeSuccess || res.Step != pipelineStepComplete {
		t.Fatalf("final tick = %+v, want success at complete", res)
	}
	if actions.portalCalls != 1 {
		t.Fatalf("portal calls = %d, want 1", actions.portalCalls)
	}
	if portals.calls != 1 {
		t.Fatalf("portal entry calls = %d, want 1", portals.calls)
	}
	if preparation.calls != 1 {
		t.Fatalf("town preparation calls = %d, want 1 after stash", preparation.calls)
	}
}

func TestCountessFullRunAllowsBlackMarshLoadingWait(t *testing.T) {
	r := NewRunner(config.NewLogger("error"), RunSelection{Run: "countess"}, RunConfig{StepTimeout: 5 * time.Second}, Deps{
		Waypoint: &mockWaypointActions{},
		TownWalk: &mockTownWalker{},
		Pathing:  &mockNavigator{},
	})
	now := time.Now()
	_ = r.Tick(context.Background(), healthy(townStateWithWaypoint()), now)
	_ = r.Tick(context.Background(), healthy(townStateWithWaypoint()), now.Add(100*time.Millisecond))
	_ = r.Tick(context.Background(), healthy(townStateWithWaypoint()), now.Add(200*time.Millisecond))
	_ = r.Tick(context.Background(), healthy(townStateWithWaypoint()), now.Add(300*time.Millisecond))
	_ = r.Tick(context.Background(), healthy(townStateWithWaypoint()), now.Add(800*time.Millisecond))

	if !r.CurrentStepAllowsNonInputTick() {
		t.Fatal("full run wait_entry_area should allow non-input ticks")
	}
}

func TestCountessTownReadyHookCompletesBeforeTownWalk(t *testing.T) {
	profileActions := &mockProfileActions{hookResults: []profile.Result{
		{Status: profile.StatusAction, Hook: profile.HookTownReady},
		{Status: profile.StatusPending, Hook: profile.HookTownReady},
		{Status: profile.StatusComplete, Hook: profile.HookTownReady},
	}}
	townWalk := &mockTownWalker{}
	r := NewRunner(config.NewLogger("error"), RunSelection{Run: "countess"}, RunConfig{StepTimeout: 5 * time.Second}, Deps{Profile: profileActions, TownWalk: townWalk})
	now := time.Now()
	state := healthy(townState())
	_ = r.Tick(context.Background(), state, now) // precheck
	_ = r.Tick(context.Background(), state, now.Add(100*time.Millisecond))
	_ = r.Tick(context.Background(), state, now.Add(200*time.Millisecond))
	if townWalk.calls != 0 {
		t.Fatalf("town walk calls before hook completion = %d", townWalk.calls)
	}
	_ = r.Tick(context.Background(), state, now.Add(300*time.Millisecond))
	if profileActions.hookCalls != 3 || townWalk.calls != 1 {
		t.Fatalf("hook calls=%d town walk calls=%d, want 3 and 1", profileActions.hookCalls, townWalk.calls)
	}
}

func TestCountessIsolatedTownReadyRunsHookWithoutTownWalk(t *testing.T) {
	profileActions := &mockProfileActions{hookResults: []profile.Result{
		{Status: profile.StatusAction, Hook: profile.HookTownReady},
		{Status: profile.StatusPending, Hook: profile.HookTownReady},
		{Status: profile.StatusComplete, Hook: profile.HookTownReady},
	}}
	townWalk := &mockTownWalker{}
	r := NewRunner(config.NewLogger("error"), RunSelection{Run: "countess", Phase: RunPhaseTownReady}, RunConfig{StepTimeout: 5 * time.Second}, Deps{Profile: profileActions, TownWalk: townWalk})
	state := healthy(townState())
	state.Identity = world.GameIdentity{Valid: true, CharacterName: "MrBones", Class: world.CharacterClassNecromancer}
	now := time.Now()
	for i := 0; i < 6 && !r.Terminal(); i++ {
		_ = r.Tick(context.Background(), state, now.Add(time.Duration(i)*100*time.Millisecond))
	}
	if !r.Terminal() || r.Result().Outcome != RunOutcomeSuccess {
		t.Fatalf("result=%+v", r.Result())
	}
	if profileActions.hookCalls != 3 || townWalk.calls != 0 {
		t.Fatalf("hook calls=%d town walk=%d", profileActions.hookCalls, townWalk.calls)
	}
}

func TestProfileTelemetryFailureTerminatesRunWithExactReasonAndReset(t *testing.T) {
	profileActions := &mockProfileActions{resourceResults: []profile.Result{{Status: profile.StatusFailed, Reason: "profile_telemetry_failed"}}}
	r := NewRunner(config.NewLogger("error"), RunSelection{Run: "count"}, RunConfig{StepTimeout: time.Second}, Deps{Profile: profileActions})
	r.run = &countingRun{}
	result := r.Tick(context.Background(), healthy(townState()), time.Now())
	if result.Outcome != RunOutcomeFailed || result.Reason != "profile_telemetry_failed" {
		t.Fatalf("result=%+v", result)
	}
	r.Reset("cycle_evaluate")
	if profileActions.resetCalls != 1 {
		t.Fatalf("profile reset calls=%d", profileActions.resetCalls)
	}
}

func TestCountessFullRunPortalRequiresRunActions(t *testing.T) {
	r := NewRunner(config.NewLogger("error"), RunSelection{Run: "countess"}, killRunConfig(), Deps{})
	r.tracker.begin(pipelineStepCastTownPortal, time.Now(), time.Second)
	r.started = true
	r.outcome = RunOutcomeRunning

	res := r.Tick(context.Background(), healthy(cellar5State()), time.Now())
	if res.Outcome != RunOutcomeFailed || res.Reason != "run_actions_not_wired" {
		t.Fatalf("tick = %+v, want run_actions_not_wired failure", res)
	}
}

func TestRunnerSafetyPotionGuard(t *testing.T) {
	actions := &mockRunActions{}
	run := &countingRun{}
	r := NewRunner(config.NewLogger("error"), RunSelection{Run: "count"}, RunConfig{StepTimeout: 10 * time.Second}, Deps{Actions: actions})
	r.run = run
	now := time.Now()

	res := r.Tick(context.Background(), hpState(65, 100), now)
	if res.Outcome != RunOutcomeRunning || len(actions.beltCalls) != 1 || actions.beltCalls[0] != 1 {
		t.Fatalf("healing tick = %+v belt=%v, want slot 1", res, actions.beltCalls)
	}
	if run.onTickCalls != 0 {
		t.Fatalf("onTick calls = %d, want 0 when potion consumes tick", run.onTickCalls)
	}

	res = r.Tick(context.Background(), hpState(20, 100), now.Add(time.Second))
	if len(actions.beltCalls) != 1 {
		t.Fatalf("belt calls after throttled tick = %v, want unchanged", actions.beltCalls)
	}

	res = r.Tick(context.Background(), hpState(20, 100), now.Add(2*time.Second))
	if res.Outcome != RunOutcomeRunning || len(actions.beltCalls) != 2 || actions.beltCalls[1] != 4 {
		t.Fatalf("full rejuv tick = %+v belt=%v, want slot 4", res, actions.beltCalls)
	}
}

func TestRunnerSafetyPotionGuardsMissingDataAndActions(t *testing.T) {
	run := &countingRun{}
	r := NewRunner(config.NewLogger("error"), RunSelection{Run: "count"}, RunConfig{StepTimeout: time.Second}, Deps{})
	r.run = run
	now := time.Now()

	_ = r.Tick(context.Background(), hpState(0, 0), now)
	_ = r.Tick(context.Background(), hpState(20, 100), now.Add(time.Second))
	if run.onTickCalls != 2 {
		t.Fatalf("onTick calls = %d, want 2 when MaxHP is zero or RunActions missing", run.onTickCalls)
	}
}

func TestRunnerSafetyPotionFailure(t *testing.T) {
	actions := &mockRunActions{beltErr: errors.New("boom")}
	r := NewRunner(config.NewLogger("error"), RunSelection{Run: "count"}, RunConfig{StepTimeout: time.Second}, Deps{Actions: actions})
	r.run = &countingRun{}

	res := r.Tick(context.Background(), hpState(20, 100), time.Now())
	if res.Outcome != RunOutcomeFailed || res.Reason != "safety_potion_failed" {
		t.Fatalf("tick = %+v, want safety_potion_failed", res)
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
	r := NewRunner(config.NewLogger("error"), RunSelection{Run: "countess", Phase: RunPhaseTravelEntry}, RunConfig{StepTimeout: time.Second}, Deps{Waypoint: wp, TownWalk: tw})
	now := time.Now()

	res := r.Tick(context.Background(), townStateWithWaypoint(), now)
	if res.Step != pipelineStepAcquireTownWaypoint || res.Outcome != RunOutcomeRunning {
		t.Fatalf("precheck tick = %+v, want acquire_town_waypoint running", res)
	}
	res = r.Tick(context.Background(), townStateWithWaypoint(), now.Add(50*time.Millisecond))
	if res.Step != pipelineStepOpenWaypoint {
		t.Fatalf("acquire tick = %+v, want open_waypoint", res)
	}
	res = r.Tick(context.Background(), townStateWithWaypoint(), now.Add(100*time.Millisecond))
	if res.Step != pipelineStepOpenWaypoint || wp.tickCalls != 1 {
		t.Fatalf("open pending = %+v tickCalls=%d", res, wp.tickCalls)
	}
	res = r.Tick(context.Background(), townStateWithWaypoint(), now.Add(200*time.Millisecond))
	if res.Step != pipelineStepSelectRunWaypoint {
		t.Fatalf("open clicked = %+v, want select_run_waypoint", res)
	}
	res = r.Tick(context.Background(), townStateWithWaypoint(), now.Add(300*time.Millisecond))
	if wp.selectCalls != 0 || res.Step != pipelineStepSelectRunWaypoint {
		t.Fatalf("settle tick = %+v selectCalls=%d, want no click", res, wp.selectCalls)
	}
	res = r.Tick(context.Background(), townStateWithWaypoint(), now.Add(800*time.Millisecond))
	if wp.selectCalls != 1 || res.Step != pipelineStepWaitEntryArea {
		t.Fatalf("select tick = %+v selectCalls=%d, want wait_entry_area", res, wp.selectCalls)
	}
	if wp.selectedTarget != pathing.WaypointTargetBlackMarsh {
		t.Fatalf("selected target = %q, want Black Marsh", wp.selectedTarget)
	}
	if !r.CurrentStepAllowsNonInputTick() {
		t.Fatal("wait_entry_area should allow non-input ticks")
	}
	loading := world.State{Phase: world.GamePhaseLoading, Valid: false}
	res = r.Tick(context.Background(), loading, now.Add(900*time.Millisecond))
	if res.Step != pipelineStepWaitEntryArea || res.Outcome != RunOutcomeRunning {
		t.Fatalf("loading wait tick = %+v, want still waiting", res)
	}
	res = r.Tick(context.Background(), blackMarshState(), now.Add(time.Second))
	if res.Outcome != RunOutcomeSuccess {
		t.Fatalf("black marsh tick = %+v, want success", res)
	}
}

func TestRunWaypointWaitRejectsWrongAreaAndUsesStableTimeoutReason(t *testing.T) {
	now := time.Now()
	newWaitingRunner := func() *Runner {
		r := NewRunner(config.NewLogger("error"), RunSelection{Run: "countess", Phase: RunPhaseTravelEntry}, RunConfig{StepTimeout: time.Second}, Deps{})
		r.started = true
		r.outcome = RunOutcomeRunning
		r.tracker.begin(pipelineStepWaitEntryArea, now, time.Second)
		return r
	}

	r := newWaitingRunner()
	res := r.Tick(context.Background(), areaState(world.TamoeHighland), now)
	if res.Outcome != RunOutcomeFailed || res.Reason != string(RunReasonUnexpectedArea) {
		t.Fatalf("wrong area tick = %+v", res)
	}

	r = newWaitingRunner()
	res = r.Tick(context.Background(), areaState(world.RogueEncampment), now.Add(time.Second))
	if res.Outcome != RunOutcomeFailed || res.Reason != string(RunReasonWaypointDestinationTimeout) {
		t.Fatalf("timeout tick = %+v", res)
	}
}

func TestMephistoTownNormalizationUsesBoundEgressAndRegisteredHubTarget(t *testing.T) {
	definition, _ := DefaultRunRegistry().Definition(RunIDMephisto)
	pipeline := &runPipeline{definition: definition}
	if got := pipeline.nextStep(pipelineStepWaitOriginTown); got != pipelineStepPlayTownEgress {
		t.Fatalf("wait origin successor = %q", got)
	}
	egress := &mockTownEgressPlayback{done: true}
	waypoint := &mockWaypointActions{}
	deps := Deps{TownEgress: egress, Waypoint: waypoint}
	now := time.Now()
	kurast := areaState(world.KurastDocks)

	res := pipeline.onTownNormalizationTick(context.Background(), deps, pipelineStepPlayTownEgress, kurast, now, now)
	if !res.complete || res.failed || len(egress.starts) != 1 || egress.starts[0] != town.OriginAct3 {
		t.Fatalf("egress tick=%+v starts=%v", res, egress.starts)
	}
	res = pipeline.onTownNormalizationTick(context.Background(), deps, pipelineStepOpenOriginWaypoint, kurast, now, now)
	if !res.complete || waypoint.tickCalls != 1 {
		t.Fatalf("open waypoint tick=%+v calls=%d", res, waypoint.tickCalls)
	}
	res = pipeline.onTownNormalizationTick(context.Background(), deps, pipelineStepSelectHubWaypoint, kurast, now, now.Add(-time.Second))
	if !res.complete || waypoint.selectedTarget != pathing.WaypointTargetRogueEncampment {
		t.Fatalf("hub select=%+v target=%s", res, waypoint.selectedTarget)
	}
	res = pipeline.onTownNormalizationTick(context.Background(), deps, pipelineStepWaitHubArea, areaState(world.RogueEncampment), now, now)
	if !res.complete || res.failed {
		t.Fatalf("hub arrival=%+v", res)
	}
}

func TestMephistoTownNormalizationFailsClosedBeforeMovement(t *testing.T) {
	definition, _ := DefaultRunRegistry().Definition(RunIDMephisto)
	pipeline := &runPipeline{definition: definition}
	now := time.Now()
	if res := pipeline.onTownNormalizationTick(context.Background(), Deps{}, pipelineStepPlayTownEgress, areaState(world.KurastDocks), now, now); !res.failed || res.reason != string(RunReasonTownEgressMissing) {
		t.Fatalf("missing egress=%+v", res)
	}
	egress := &mockTownEgressPlayback{done: true}
	if res := pipeline.onTownNormalizationTick(context.Background(), Deps{TownEgress: egress}, pipelineStepPlayTownEgress, areaState(world.RogueEncampment), now, now); !res.failed || res.reason != string(RunReasonUnexpectedArea) || len(egress.starts) != 0 {
		t.Fatalf("wrong start=%+v starts=%v", res, egress.starts)
	}
	if res := pipeline.onTownNormalizationTick(context.Background(), Deps{}, pipelineStepWaitHubArea, areaState(world.TamoeHighland), now, now); !res.failed || res.reason != string(RunReasonUnexpectedArea) {
		t.Fatalf("wrong hub area=%+v", res)
	}
}

func TestCountessTravelMarshPrecheckFailsOutsideAct1Town(t *testing.T) {
	r := NewRunner(config.NewLogger("error"), RunSelection{Run: "countess", Phase: RunPhaseTravelEntry}, RunConfig{StepTimeout: time.Second}, Deps{})
	res := r.Tick(context.Background(), blackMarshState(), time.Now())
	if res.Outcome != RunOutcomeFailed || res.Reason != "not_act1_town" {
		t.Fatalf("tick = %+v, want not_act1_town failure", res)
	}
}

func TestCountessTravelMarshWaypointFailureReason(t *testing.T) {
	wp := &mockWaypointActions{
		results: []pathing.WaypointActionResult{{Status: pathing.WaypointActionHoverNotFound, Reason: string(pathing.WaypointActionHoverNotFound), Done: true}},
	}
	r := NewRunner(config.NewLogger("error"), RunSelection{Run: "countess", Phase: RunPhaseTravelEntry}, RunConfig{StepTimeout: time.Second}, Deps{Waypoint: wp, TownWalk: &mockTownWalker{}})
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
	r := NewRunner(config.NewLogger("error"), RunSelection{Run: "countess", Phase: RunPhaseTravelEntry}, RunConfig{StepTimeout: time.Second}, Deps{Waypoint: wp, TownWalk: tw})
	now := time.Now()
	_ = r.Tick(context.Background(), townStateWithFarWaypoint(), now)
	res := r.Tick(context.Background(), townStateWithFarWaypoint(), now.Add(time.Millisecond))
	if res.Step != pipelineStepAcquireTownWaypoint || res.Outcome != RunOutcomeRunning {
		t.Fatalf("pending acquire = %+v", res)
	}
	res = r.Tick(context.Background(), townStateWithFarWaypoint(), now.Add(2*time.Millisecond))
	if res.Step != pipelineStepOpenWaypoint {
		t.Fatalf("visible acquire = %+v, want open_waypoint", res)
	}
}

func TestCountessTravelCellar5SuccessStartsExpectedGoals(t *testing.T) {
	wp := &mockWaypointActions{}
	nav := &mockNavigator{}
	route := &mockRoutePlayback{}
	r := NewRunner(config.NewLogger("error"), RunSelection{Run: "countess", Phase: RunPhasePlayRoute}, RunConfig{StepTimeout: 5 * time.Second, RouteID: "test-countess-route"}, Deps{
		Waypoint: wp,
		TownWalk: &mockTownWalker{},
		Pathing:  nav,
		Route:    route,
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
		areaState(world.TowerCellarLevel5),
	}

	var res TickResult
	for i, st := range ticks {
		res = r.Tick(context.Background(), st, now.Add(time.Duration(i)*200*time.Millisecond))
	}
	if res.Outcome != RunOutcomeSuccess || res.Step != pipelineStepPlayRoute {
		t.Fatalf("final tick = %+v, want success at play_bound_route", res)
	}

	if len(nav.startGoals) > 1 {
		t.Fatalf("Start calls = %d, want at most 1 when snapshots already reach target areas", len(nav.startGoals))
	}
	if route.startedID != "test-countess-route" {
		t.Fatalf("route started = %q", route.startedID)
	}
}

func TestCountessTravelCellar5AllowsBlackMarshLoadingWait(t *testing.T) {
	r := NewRunner(config.NewLogger("error"), RunSelection{Run: "countess", Phase: RunPhasePlayRoute}, RunConfig{StepTimeout: 5 * time.Second}, Deps{Waypoint: &mockWaypointActions{}, TownWalk: &mockTownWalker{}, Pathing: &mockNavigator{}})
	now := time.Now()
	_ = r.Tick(context.Background(), townStateWithWaypoint(), now)
	_ = r.Tick(context.Background(), townStateWithWaypoint(), now.Add(100*time.Millisecond))
	_ = r.Tick(context.Background(), townStateWithWaypoint(), now.Add(200*time.Millisecond))
	_ = r.Tick(context.Background(), townStateWithWaypoint(), now.Add(300*time.Millisecond))
	_ = r.Tick(context.Background(), townStateWithWaypoint(), now.Add(800*time.Millisecond))

	if !r.CurrentStepAllowsNonInputTick() {
		t.Fatal("play-route wait_entry_area should allow non-input ticks")
	}
}

func TestCountessTravelCellar5ResumesFromRouteAreas(t *testing.T) {
	cases := []struct {
		name     string
		area     world.AreaID
		wantStep string
	}{
		{"black marsh", world.BlackMarsh, pipelineStepPlayRoute},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := NewRunner(config.NewLogger("error"), RunSelection{Run: "countess", Phase: RunPhasePlayRoute}, RunConfig{StepTimeout: 5 * time.Second, RouteID: "test-countess-route"}, Deps{
				Waypoint: &mockWaypointActions{},
				TownWalk: &mockTownWalker{},
				Pathing:  &mockNavigator{tickResults: []pathing.NavTickResult{{Status: pathing.NavExploring}}},
				Route:    &mockRoutePlayback{},
			})

			res := r.Tick(context.Background(), areaState(tc.area), time.Now())
			if res.Outcome != RunOutcomeRunning || res.Step != tc.wantStep {
				t.Fatalf("tick = %+v, want running step %s", res, tc.wantStep)
			}
		})
	}
}

func TestCountessTravelCellar5RejectsIntermediateResume(t *testing.T) {
	for _, area := range []world.AreaID{world.ForgottenTower, world.TowerCellarLevel1, world.TowerCellarLevel4} {
		r := NewRunner(config.NewLogger("error"), RunSelection{Run: "countess", Phase: RunPhasePlayRoute}, RunConfig{StepTimeout: time.Second, RouteID: "test-countess-route"}, Deps{Route: &mockRoutePlayback{}})
		res := r.Tick(context.Background(), areaState(area), time.Now())
		if res.Outcome != RunOutcomeFailed || res.Reason != "not_act1_town" {
			t.Fatalf("area %d result = %+v", area, res)
		}
	}
}

func TestCountessTravelCellar5ResumeAlreadyAtCellar5Completes(t *testing.T) {
	r := NewRunner(config.NewLogger("error"), RunSelection{Run: "countess", Phase: RunPhasePlayRoute}, RunConfig{StepTimeout: 5 * time.Second}, Deps{
		Waypoint: &mockWaypointActions{},
		TownWalk: &mockTownWalker{},
		Pathing:  &mockNavigator{},
	})

	res := r.Tick(context.Background(), areaState(world.TowerCellarLevel5), time.Now())
	if res.Outcome != RunOutcomeSuccess || res.Step != pipelineStepPrecheck {
		t.Fatalf("tick = %+v, want success from precheck at cellar 5", res)
	}
}

func TestCountessKillPrecheckRequiresCellar5AndCombat(t *testing.T) {
	r := NewRunner(config.NewLogger("error"), RunSelection{Run: "countess", Phase: RunPhaseBoss}, killRunConfig(), Deps{Combat: &mockCombatActions{}})
	res := r.Tick(context.Background(), areaState(world.TowerCellarLevel4), time.Now())
	if res.Outcome != RunOutcomeFailed || res.Reason != "not_cellar_5" {
		t.Fatalf("wrong area tick = %+v, want not_cellar_5", res)
	}

	r = NewRunner(config.NewLogger("error"), RunSelection{Run: "countess", Phase: RunPhaseBoss}, killRunConfig(), Deps{})
	res = r.Tick(context.Background(), cellar5State(), time.Now())
	if res.Outcome != RunOutcomeFailed || res.Reason != "combat_not_wired" {
		t.Fatalf("missing combat tick = %+v, want combat_not_wired", res)
	}
}

func TestCountessKillTargetsDarkStalkerBeforeGenericSuperUnique(t *testing.T) {
	combat := &mockCombatActions{}
	r := NewRunner(config.NewLogger("error"), RunSelection{Run: "countess", Phase: RunPhaseBoss}, killRunConfig(), Deps{Combat: combat})
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
	r := NewRunner(config.NewLogger("error"), RunSelection{Run: "countess", Phase: RunPhaseBoss}, killRunConfig(), Deps{
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
	r := NewRunner(config.NewLogger("error"), RunSelection{Run: "countess", Phase: RunPhaseBoss}, killRunConfig(), Deps{
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
	r := NewRunner(config.NewLogger("error"), RunSelection{Run: "countess", Phase: RunPhaseBoss}, killRunConfig(), Deps{Combat: combat})
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

func TestCountessBossHookCompletesBeforeFirstAttack(t *testing.T) {
	combat := &mockCombatActions{}
	profileActions := &mockProfileActions{hookResults: []profile.Result{
		{Status: profile.StatusAction, Hook: profile.HookBossEngage},
		{Status: profile.StatusPending, Hook: profile.HookBossEngage},
		{Status: profile.StatusComplete, Hook: profile.HookBossEngage},
	}, resourceResults: []profile.Result{
		{Status: profile.StatusAction, Resource: profile.ResourceMana, BeltSlot: 2},
		{Status: profile.StatusPending, Resource: profile.ResourceMana, BeltSlot: 2},
		{Status: profile.StatusComplete, Resource: profile.ResourceMana, BeltSlot: 2},
	}}
	r := NewRunner(config.NewLogger("error"), RunSelection{Run: "countess", Phase: RunPhaseBoss}, killRunConfig(), Deps{Combat: combat, Profile: profileActions})
	now := time.Now()
	target := countessMonster(77, world.Position{X: 110, Y: 100})
	visible := healthy(cellar5State(target))

	_ = r.Tick(context.Background(), visible, now)                       // mana action
	_ = r.Tick(context.Background(), visible, now.Add(time.Millisecond)) // mana verify pending
	_ = r.Tick(context.Background(), visible, now.Add(2*time.Millisecond))
	_ = r.Tick(context.Background(), visible, now.Add(3*time.Millisecond)) // locate
	_ = r.Tick(context.Background(), visible, now.Add(4*time.Millisecond)) // prison action
	_ = r.Tick(context.Background(), visible, now.Add(5*time.Millisecond)) // prison settle
	if combat.castCalls != 0 {
		t.Fatalf("combat casts before hook completion = %d", combat.castCalls)
	}
	_ = r.Tick(context.Background(), visible, now.Add(6*time.Millisecond))
	if combat.castCalls != 1 || profileActions.hookCalls != 3 || profileActions.hooks[0] != profile.HookBossEngage {
		t.Fatalf("combat=%d hooks=%v calls=%d", combat.castCalls, profileActions.hooks, profileActions.hookCalls)
	}
	if profileActions.targets[0].UnitID != 77 || profileActions.targets[0].Position != target.Position {
		t.Fatalf("target=%+v", profileActions.targets[0])
	}
}

func TestCountessKillAbsenceResetAndAreaFailure(t *testing.T) {
	combat := &mockCombatActions{}
	r := NewRunner(config.NewLogger("error"), RunSelection{Run: "countess", Phase: RunPhaseBoss}, killRunConfig(), Deps{Combat: combat})
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
	r := NewRunner(config.NewLogger("error"), RunSelection{Run: "countess", Phase: RunPhaseBoss}, killRunConfig(), Deps{Combat: combat})
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

func TestBossRunRepositionsAtLastBossPositionBeforeLoot(t *testing.T) {
	combat := &mockCombatActions{}
	definition, _ := DefaultRunRegistry().Definition(RunIDCountess)
	position := world.Position{X: 130, Y: 100}
	pipeline := &runPipeline{
		definition: definition, combat: killRunConfig().Combat,
		targetSeen: true, targetUnitID: 10, targetPosition: position, targetPositionSet: true,
	}
	now := time.Now()
	state := cellar5State()
	state.Player.Position = world.Position{X: 100, Y: 100}

	res := pipeline.onBossTick(context.Background(), Deps{Combat: combat}, pipelineStepRepositionForLoot, state, now)
	if res.complete || res.failed || combat.teleportCalls != 1 || combat.lastDesired != 0 {
		t.Fatalf("first reposition = %+v teleports=%d desired=%.1f", res, combat.teleportCalls, combat.lastDesired)
	}
	res = pipeline.onBossTick(context.Background(), Deps{Combat: combat}, pipelineStepRepositionForLoot, state, now.Add(time.Millisecond))
	if res.complete || res.failed || combat.teleportCalls != 1 {
		t.Fatalf("stale snapshot = %+v teleports=%d, want wait without another cast", res, combat.teleportCalls)
	}
	state.Player.Position = position
	res = pipeline.onBossTick(context.Background(), Deps{Combat: combat}, pipelineStepRepositionForLoot, state, now.Add(2*time.Millisecond))
	if !res.complete || res.failed || combat.teleportCalls != 1 {
		t.Fatalf("arrival = %+v teleports=%d, want complete without another cast", res, combat.teleportCalls)
	}
}

func TestBossLootRepositionRetriesOnlyAfterFreshSnapshotsThenContinuesToItemScan(t *testing.T) {
	combat := &mockCombatActions{}
	definition, _ := DefaultRunRegistry().Definition(RunIDCountess)
	position := world.Position{X: 130, Y: 100}
	pipeline := &runPipeline{
		definition: definition, combat: killRunConfig().Combat,
		targetSeen: true, targetUnitID: 10, targetPosition: position, targetPositionSet: true,
	}
	now := time.Now()
	state := cellar5State()
	state.At = now
	state.Player.Position = world.Position{X: 100, Y: 100}

	res := pipeline.onBossTick(context.Background(), Deps{Combat: combat}, pipelineStepRepositionForLoot, state, now)
	if res.complete || res.failed || combat.teleportCalls != 1 || pipeline.postKillTeleportAttempts != 1 {
		t.Fatalf("first attempt=%+v calls=%d state=%+v", res, combat.teleportCalls, pipeline)
	}
	res = pipeline.onBossTick(context.Background(), Deps{Combat: combat}, pipelineStepRepositionForLoot, state, now.Add(time.Second))
	if res.complete || res.failed || combat.teleportCalls != 1 {
		t.Fatalf("stale snapshot retried: result=%+v calls=%d", res, combat.teleportCalls)
	}
	for attempt := 2; attempt <= lootRepositionMaxAttempts; attempt++ {
		state.At = now.Add(time.Duration(attempt) * lootRepositionRetryDelay)
		res = pipeline.onBossTick(context.Background(), Deps{Combat: combat}, pipelineStepRepositionForLoot, state, state.At)
		if res.complete || res.failed || combat.teleportCalls != attempt {
			t.Fatalf("attempt %d result=%+v calls=%d", attempt, res, combat.teleportCalls)
		}
	}
	state.At = state.At.Add(lootRepositionRetryDelay)
	res = pipeline.onBossTick(context.Background(), Deps{Combat: combat}, pipelineStepRepositionForLoot, state, state.At)
	if !res.complete || res.failed || combat.teleportCalls != lootRepositionMaxAttempts {
		t.Fatalf("bounded fallback=%+v calls=%d", res, combat.teleportCalls)
	}
}

func TestBossLootRepositionDoesNotConsumeRetryForThrottledNoOp(t *testing.T) {
	combat := &mockCombatActions{teleportSent: []bool{false, true}}
	definition, _ := DefaultRunRegistry().Definition(RunIDCountess)
	pipeline := &runPipeline{
		definition: definition, targetPosition: world.Position{X: 130, Y: 100}, targetPositionSet: true,
	}
	state := cellar5State()
	state.At = time.Now()
	state.Player.Position = world.Position{X: 100, Y: 100}

	_ = pipeline.onBossTick(context.Background(), Deps{Combat: combat}, pipelineStepRepositionForLoot, state, state.At)
	if pipeline.postKillTeleportAttempts != 0 {
		t.Fatalf("throttled call consumed retry: %+v", pipeline)
	}
	_ = pipeline.onBossTick(context.Background(), Deps{Combat: combat}, pipelineStepRepositionForLoot, state, state.At.Add(time.Millisecond))
	if pipeline.postKillTeleportAttempts != 1 || combat.teleportCalls != 2 {
		t.Fatalf("real retry attempts=%d calls=%d", pipeline.postKillTeleportAttempts, combat.teleportCalls)
	}
}

func TestLootCandidateRepositionsToCurrentItemBeforePickup(t *testing.T) {
	combat := &mockCombatActions{}
	definition, _ := DefaultRunRegistry().Definition(RunIDCountess)
	target := LootTarget{
		UnitID: 10, TxtFileNo: 1, Code: "r01", Name: "El Rune",
		Position: world.Position{X: 130, Y: 100}, AreaID: world.TowerCellarLevel5,
	}
	lootActions := &mockLootActions{
		scans: []LootScanResult{{GroundItemCount: 1, CandidateCount: 1, HasTarget: true, NextTarget: target}},
	}
	pipeline := &runPipeline{definition: definition, lootPickupDistanceTiles: 8}
	now := time.Now()
	state := cellar5State()
	state.At = now
	state.Player.Position = world.Position{X: 100, Y: 100}
	state.Items = []world.Item{{
		UnitID: target.UnitID, TxtFileNo: target.TxtFileNo, Code: target.Code, Name: target.Name,
		Location: world.ItemLocationGround, Position: target.Position,
	}}

	res := pipeline.onLootTick(context.Background(), Deps{Loot: lootActions, Combat: combat}, pipelineStepPickLoot, state, now, now)
	if res.complete || res.failed || combat.teleportCalls != 1 || len(lootActions.startCalls) != 0 || combat.lastTeleportTarget != target.Position {
		t.Fatalf("approach result=%+v teleports=%d target=%+v starts=%d", res, combat.teleportCalls, combat.lastTeleportTarget, len(lootActions.startCalls))
	}
	arrived := state
	arrived.At = now.Add(100 * time.Millisecond)
	arrived.Player.Position = target.Position
	res = pipeline.onLootTick(context.Background(), Deps{Loot: lootActions, Combat: combat}, pipelineStepPickLoot, arrived, arrived.At, now)
	if res.complete || res.failed || combat.teleportCalls != 1 || len(lootActions.startCalls) != 1 || lootActions.startCalls[0].UnitID != target.UnitID {
		t.Fatalf("arrival result=%+v teleports=%d starts=%+v", res, combat.teleportCalls, lootActions.startCalls)
	}
}

func TestAllRegisteredRunsRepositionBeforeLoot(t *testing.T) {
	countess, _ := DefaultRunRegistry().Definition(RunIDCountess)
	mephisto, _ := DefaultRunRegistry().Definition(RunIDMephisto)
	if got := (&runPipeline{definition: countess}).nextStep(pipelineStepEngageBoss); got != pipelineStepRepositionForLoot {
		t.Fatalf("countess successor = %q, want %q", got, pipelineStepRepositionForLoot)
	}
	if got := (&runPipeline{definition: mephisto}).nextStep(pipelineStepEngageBoss); got != pipelineStepRepositionForLoot {
		t.Fatalf("mephisto successor = %q, want %q", got, pipelineStepRepositionForLoot)
	}
}

func TestCountessLootWaitForDropsRequiresThreeStableTicks(t *testing.T) {
	lootActions := &mockLootActions{}
	actions := &mockRunActions{}
	r := NewRunner(config.NewLogger("error"), RunSelection{Run: "countess", Phase: RunPhaseLootAndReturn}, killRunConfig(), Deps{
		Loot:    lootActions,
		Actions: actions,
	})
	now := time.Now()
	st := cellar5State()

	res := r.Tick(context.Background(), st, now)
	if res.Step != pipelineStepWaitForDrops || res.Outcome != RunOutcomeRunning {
		t.Fatalf("precheck tick = %+v, want wait_for_drops running", res)
	}
	res = r.Tick(context.Background(), st, now.Add(time.Millisecond))
	if res.Step != pipelineStepWaitForDrops || res.Outcome != RunOutcomeRunning {
		t.Fatalf("stable tick 1 = %+v, want wait_for_drops running", res)
	}
	res = r.Tick(context.Background(), world.State{}, now.Add(2*time.Millisecond))
	if res.Step != pipelineStepWaitForDrops || res.Outcome != RunOutcomeRunning {
		t.Fatalf("invalid tick = %+v, want wait_for_drops running", res)
	}
	_ = r.Tick(context.Background(), st, now.Add(3*time.Millisecond))
	_ = r.Tick(context.Background(), st, now.Add(4*time.Millisecond))
	res = r.Tick(context.Background(), st, now.Add(5*time.Millisecond))
	if res.Step != pipelineStepScanLoot || res.Outcome != RunOutcomeRunning {
		t.Fatalf("stable tick 3 = %+v, want scan_loot running", res)
	}
}

func TestCountessLootNoCandidatesCastsTownPortalAndSucceeds(t *testing.T) {
	lootActions := &mockLootActions{scans: []LootScanResult{{GroundItemCount: 2}}}
	actions := &mockRunActions{}
	r := NewRunner(config.NewLogger("error"), RunSelection{Run: "countess", Phase: RunPhaseLootAndReturn}, killRunConfig(), Deps{
		Loot:    lootActions,
		Actions: actions,
		Portal:  &mockTownPortalActions{},
		Stash:   &mockPersonalStashActions{},
	})
	now := time.Now()
	st := cellar5State()

	_ = r.Tick(context.Background(), st, now)
	_ = r.Tick(context.Background(), st, now.Add(time.Millisecond))
	_ = r.Tick(context.Background(), st, now.Add(2*time.Millisecond))
	_ = r.Tick(context.Background(), st, now.Add(3*time.Millisecond))
	_ = r.Tick(context.Background(), st, now.Add(4*time.Millisecond))
	_ = r.Tick(context.Background(), st, now.Add(5*time.Millisecond))
	res := r.Tick(context.Background(), st, now.Add(6*time.Millisecond))
	if res.Step != pipelineStepCastTownPortal || res.Outcome != RunOutcomeRunning {
		t.Fatalf("stable empty scan = %+v, want cast_town_portal running", res)
	}
	res = r.Tick(context.Background(), st, now.Add(7*time.Millisecond))
	if res.Step != pipelineStepEnterTownPortal || res.Outcome != RunOutcomeRunning {
		t.Fatalf("portal tick = %+v, want enter_town_portal running", res)
	}
	res = r.Tick(context.Background(), st, now.Add(8*time.Millisecond))
	if res.Step != pipelineStepWaitOriginTown || res.Outcome != RunOutcomeRunning {
		t.Fatalf("entry tick = %+v, want wait_origin_town running", res)
	}
	res = r.Tick(context.Background(), townState(), now.Add(9*time.Millisecond))
	if res.Step != pipelineStepOpenStash || res.Outcome != RunOutcomeRunning {
		t.Fatalf("town tick = %+v, want open_personal_stash", res)
	}
	res = r.Tick(context.Background(), townState(), now.Add(10*time.Millisecond))
	if res.Step != pipelineStepStashItems || res.Outcome != RunOutcomeRunning {
		t.Fatalf("open tick = %+v, want stash_items", res)
	}
	res = r.Tick(context.Background(), townState(), now.Add(11*time.Millisecond))
	if res.Step != pipelineStepCloseStash || res.Outcome != RunOutcomeRunning {
		t.Fatalf("stash tick = %+v, want close_personal_stash", res)
	}
	res = r.Tick(context.Background(), townState(), now.Add(12*time.Millisecond))
	if res.Step != pipelineStepComplete || res.Outcome != RunOutcomeRunning {
		t.Fatalf("close tick = %+v, want complete", res)
	}
	res = r.Tick(context.Background(), townState(), now.Add(13*time.Millisecond))
	if res.Outcome != RunOutcomeSuccess || actions.portalCalls != 1 {
		t.Fatalf("complete tick = %+v portalCalls=%d, want success with one portal", res, actions.portalCalls)
	}
}

func TestCountessLootDoesNotFinishOnTransientNoTargetScan(t *testing.T) {
	target := LootTarget{UnitID: 10, TxtFileNo: 1, Code: "r01", Name: "El Rune", Position: world.Position{X: 101, Y: 100}, AreaID: world.TowerCellarLevel5}
	lootActions := &mockLootActions{scans: []LootScanResult{
		{GroundItemCount: 1},
		{GroundItemCount: 1, CandidateCount: 1, HasTarget: true, NextTarget: target},
	}}
	r := NewRunner(config.NewLogger("error"), RunSelection{Run: "countess", Phase: RunPhaseLootAndReturn}, killRunConfig(), Deps{Loot: lootActions})
	r.started = true
	r.outcome = RunOutcomeRunning
	r.tracker.begin(pipelineStepScanLoot, time.Now(), time.Second)
	r.run.onStepEnter(pipelineStepScanLoot)

	res := r.Tick(context.Background(), cellar5State(), time.Now())
	if res.Step != pipelineStepScanLoot || res.Outcome != RunOutcomeRunning {
		t.Fatalf("empty tick = %+v, want scan_loot running", res)
	}
	res = r.Tick(context.Background(), cellar5State(), time.Now().Add(time.Millisecond))
	if res.Step != pipelineStepPickLoot || res.Outcome != RunOutcomeRunning {
		t.Fatalf("reappeared target tick = %+v, want pick_loot running", res)
	}
}

func TestCountessLootTelemetryFailureAbortsBeforePickup(t *testing.T) {
	lootActions := &mockLootActions{scans: []LootScanResult{{TelemetryFailed: true}}}
	r := NewRunner(config.NewLogger("error"), RunSelection{Run: "countess", Phase: RunPhaseLootAndReturn}, killRunConfig(), Deps{Loot: lootActions})
	r.started = true
	r.outcome = RunOutcomeRunning
	r.tracker.begin(pipelineStepScanLoot, time.Now(), time.Second)
	r.run.onStepEnter(pipelineStepScanLoot)
	res := r.Tick(context.Background(), cellar5State(), time.Now())
	if res.Outcome != RunOutcomeFailed || res.Reason != "telemetry_failed" || len(lootActions.startCalls) != 0 {
		t.Fatalf("tick=%+v starts=%v, want fail-closed telemetry_failed", res, lootActions.startCalls)
	}
}

func TestCountessLootInventoryFullStopsPickupAndRecoversToTown(t *testing.T) {
	lootActions := &mockLootActions{scans: []LootScanResult{{
		GroundItemCount:             1,
		InventoryFullCandidateCount: 1,
		InventoryFull:               true,
	}}}
	actions := &mockRunActions{}
	portals := &mockTownPortalActions{}
	r := NewRunner(config.NewLogger("error"), RunSelection{Run: "countess", Phase: RunPhaseLootAndReturn}, killRunConfig(), Deps{
		Loot:    lootActions,
		Actions: actions,
		Portal:  portals,
		Stash:   &mockPersonalStashActions{},
	})
	now := time.Now()
	st := cellar5State()
	states := []world.State{st, st, st, st, st, st, st, townState(), townState(), townState(), townState(), townState()}
	var res TickResult
	for i, state := range states {
		res = r.Tick(context.Background(), state, now.Add(time.Duration(i)*time.Millisecond))
	}
	if res.Outcome != RunOutcomeSuccess {
		t.Fatalf("final tick = %+v, want success in town", res)
	}
	if len(lootActions.startCalls) != 0 || lootActions.tickCalls != 0 {
		t.Fatalf("pickup start/tick = %d/%d, want none after inventory_full", len(lootActions.startCalls), lootActions.tickCalls)
	}
	if actions.portalCalls != 1 || portals.calls != 1 {
		t.Fatalf("portal cast/entry = %d/%d, want 1/1", actions.portalCalls, portals.calls)
	}
}

func TestCountessTownPortalFailuresHaveStableReasons(t *testing.T) {
	cases := []struct {
		name   string
		status pathing.TownPortalActionStatus
		reason string
	}{
		{"not found", pathing.TownPortalActionNotFound, "town_portal_not_found"},
		{"hover failed", pathing.TownPortalActionHoverNotFound, "town_portal_enter_failed"},
		{"too far", pathing.TownPortalActionTooFar, "town_portal_enter_failed"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := NewRunner(config.NewLogger("error"), RunSelection{Run: "countess", Phase: RunPhaseLootAndReturn}, killRunConfig(), Deps{
				Loot:    &mockLootActions{},
				Actions: &mockRunActions{},
				Portal: &mockTownPortalActions{results: []pathing.TownPortalActionResult{{
					Status: tc.status,
					Done:   true,
				}}},
			})
			r.started = true
			r.outcome = RunOutcomeRunning
			r.tracker.begin(pipelineStepEnterTownPortal, time.Now(), time.Second)
			res := r.Tick(context.Background(), cellar5State(), time.Now())
			if res.Outcome != RunOutcomeFailed || res.Reason != tc.reason {
				t.Fatalf("tick = %+v, want reason %s", res, tc.reason)
			}
		})
	}
}

func TestCountessWaitTownAllowsLoadingAndRejectsWrongArea(t *testing.T) {
	r := NewRunner(config.NewLogger("error"), RunSelection{Run: "countess", Phase: RunPhaseLootAndReturn}, killRunConfig(), Deps{})
	r.started = true
	r.outcome = RunOutcomeRunning
	r.tracker.begin(pipelineStepWaitOriginTown, time.Now(), time.Second)
	if !r.CurrentStepAllowsNonInputTick() {
		t.Fatal("wait_origin_town should allow loading ticks")
	}
	res := r.Tick(context.Background(), world.State{Phase: world.GamePhaseLoading}, time.Now())
	if res.Outcome != RunOutcomeRunning {
		t.Fatalf("loading tick = %+v, want running", res)
	}
	res = r.Tick(context.Background(), areaState(world.BlackMarsh), time.Now())
	if res.Outcome != RunOutcomeFailed || res.Reason != "unexpected_area" {
		t.Fatalf("wrong-area tick = %+v, want unexpected_area", res)
	}
}

func TestCountessLootPickSkipsFailedCandidateAndFinishesWhenNoTargetsRemain(t *testing.T) {
	target := LootTarget{UnitID: 10, TxtFileNo: 1, Code: "r01", Name: "El Rune", Position: world.Position{X: 101, Y: 100}, AreaID: world.TowerCellarLevel5}
	lootActions := &mockLootActions{
		scans: []LootScanResult{
			{GroundItemCount: 1, CandidateCount: 1, HasTarget: true, NextTarget: target},
			{GroundItemCount: 1, CandidateCount: 1, HasTarget: true, NextTarget: target},
			{GroundItemCount: 1, CandidateCount: 0},
		},
		ticks: []LootPickupResult{{Status: LootPickupMonsterNearby, Done: true, Target: target}},
	}
	actions := &mockRunActions{}
	r := NewRunner(config.NewLogger("error"), RunSelection{Run: "countess", Phase: RunPhaseLootAndReturn}, killRunConfig(), Deps{
		Loot:    lootActions,
		Actions: actions,
	})
	now := time.Now()
	st := cellar5State()

	for i := 0; i < 7; i++ {
		_ = r.Tick(context.Background(), st, now.Add(time.Duration(i)*time.Millisecond))
	}
	if len(lootActions.startCalls) != 1 {
		t.Fatalf("StartPickup calls = %d, want 1", len(lootActions.startCalls))
	}
	if lootActions.scanCalls != 3 {
		t.Fatalf("Scan calls = %d, want initial scan, pickup scan, rescan", lootActions.scanCalls)
	}
}

func TestCountessLootHardPickupStatusFailsRun(t *testing.T) {
	target := LootTarget{UnitID: 10, TxtFileNo: 1, Code: "r01", Name: "El Rune", Position: world.Position{X: 101, Y: 100}, AreaID: world.TowerCellarLevel5}
	lootActions := &mockLootActions{
		scans: []LootScanResult{
			{GroundItemCount: 1, CandidateCount: 1, HasTarget: true, NextTarget: target},
			{GroundItemCount: 1, CandidateCount: 1, HasTarget: true, NextTarget: target},
		},
		ticks: []LootPickupResult{{Status: LootPickupInputBlocked, Done: true, Target: target}},
	}
	r := NewRunner(config.NewLogger("error"), RunSelection{Run: "countess", Phase: RunPhaseLootAndReturn}, killRunConfig(), Deps{
		Loot:    lootActions,
		Actions: &mockRunActions{},
	})
	now := time.Now()
	st := cellar5State()
	var res TickResult
	for i := 0; i < 6; i++ {
		res = r.Tick(context.Background(), st, now.Add(time.Duration(i)*time.Millisecond))
	}
	if res.Outcome != RunOutcomeFailed || res.Reason != string(LootPickupInputBlocked) {
		t.Fatalf("tick = %+v, want input_blocked failure", res)
	}
}

func TestCountessLootStartPickupErrorFailsRun(t *testing.T) {
	target := LootTarget{UnitID: 10, TxtFileNo: 1, Code: "r01", Name: "El Rune", Position: world.Position{X: 101, Y: 100}, AreaID: world.TowerCellarLevel5}
	lootActions := &mockLootActions{
		scans: []LootScanResult{
			{GroundItemCount: 1, CandidateCount: 1, HasTarget: true, NextTarget: target},
			{GroundItemCount: 1, CandidateCount: 1, HasTarget: true, NextTarget: target},
		},
		startErr: errors.New("not wired"),
	}
	r := NewRunner(config.NewLogger("error"), RunSelection{Run: "countess", Phase: RunPhaseLootAndReturn}, killRunConfig(), Deps{
		Loot:    lootActions,
		Actions: &mockRunActions{},
	})
	now := time.Now()
	st := cellar5State()
	var res TickResult
	for i := 0; i < 6; i++ {
		res = r.Tick(context.Background(), st, now.Add(time.Duration(i)*time.Millisecond))
	}
	if res.Outcome != RunOutcomeFailed || res.Reason != "loot_pickup_start_failed" {
		t.Fatalf("tick = %+v, want loot_pickup_start_failed", res)
	}
}

func TestCountessLootFailsOnNilLootActionsAndAreaChange(t *testing.T) {
	r := NewRunner(config.NewLogger("error"), RunSelection{Run: "countess", Phase: RunPhaseLootAndReturn}, killRunConfig(), Deps{})
	res := r.Tick(context.Background(), cellar5State(), time.Now())
	if res.Outcome != RunOutcomeFailed || res.Reason != "loot_actions_not_wired" {
		t.Fatalf("nil loot tick = %+v, want loot_actions_not_wired", res)
	}

	r = NewRunner(config.NewLogger("error"), RunSelection{Run: "countess", Phase: RunPhaseLootAndReturn}, killRunConfig(), Deps{
		Loot:    &mockLootActions{},
		Actions: &mockRunActions{},
	})
	now := time.Now()
	_ = r.Tick(context.Background(), cellar5State(), now)
	res = r.Tick(context.Background(), areaState(world.TowerCellarLevel4), now.Add(time.Millisecond))
	if res.Outcome != RunOutcomeFailed || res.Reason != "unexpected_area" {
		t.Fatalf("area tick = %+v, want unexpected_area", res)
	}
}

func TestCountessLootFailsOnAreaChangeDuringPickLoot(t *testing.T) {
	target := LootTarget{UnitID: 10, TxtFileNo: 1, Code: "r01", Name: "El Rune", Position: world.Position{X: 101, Y: 100}, AreaID: world.TowerCellarLevel5}
	lootActions := &mockLootActions{
		scans: []LootScanResult{{GroundItemCount: 1, CandidateCount: 1, HasTarget: true, NextTarget: target}},
	}
	r := NewRunner(config.NewLogger("error"), RunSelection{Run: "countess", Phase: RunPhaseLootAndReturn}, killRunConfig(), Deps{
		Loot:    lootActions,
		Actions: &mockRunActions{},
	})
	now := time.Now()
	st := cellar5State()
	for i := 0; i < 5; i++ {
		_ = r.Tick(context.Background(), st, now.Add(time.Duration(i)*time.Millisecond))
	}
	res := r.Tick(context.Background(), areaState(world.TowerCellarLevel4), now.Add(5*time.Millisecond))
	if res.Outcome != RunOutcomeFailed || res.Reason != "unexpected_area" {
		t.Fatalf("pick area tick = %+v, want unexpected_area", res)
	}
}

func TestCountessKillStillEndsAfterKillWithoutPortalOrLoot(t *testing.T) {
	combat := &mockCombatActions{}
	lootActions := &mockLootActions{}
	actions := &mockRunActions{}
	r := NewRunner(config.NewLogger("error"), RunSelection{Run: "countess", Phase: RunPhaseBoss}, killRunConfig(), Deps{
		Combat:  combat,
		Loot:    lootActions,
		Actions: actions,
	})
	now := time.Now()
	target := countessMonster(10, world.Position{X: 110, Y: 100})
	visible := cellar5State(target)
	absent := cellar5State()

	_ = r.Tick(context.Background(), visible, now)
	_ = r.Tick(context.Background(), visible, now.Add(time.Millisecond))
	_ = r.Tick(context.Background(), absent, now.Add(2*time.Millisecond))
	_ = r.Tick(context.Background(), absent, now.Add(3*time.Millisecond))
	res := r.Tick(context.Background(), absent, now.Add(4*time.Millisecond))
	if res.Outcome != RunOutcomeSuccess || actions.portalCalls != 0 || lootActions.scanCalls != 0 {
		t.Fatalf("tick = %+v portalCalls=%d scanCalls=%d, want kill-only success", res, actions.portalCalls, lootActions.scanCalls)
	}
}

func TestCountessNavigateAreaStartsOnceWhilePending(t *testing.T) {
	nav := &mockNavigator{tickResults: []pathing.NavTickResult{
		{Status: pathing.NavExploring},
		{Status: pathing.NavExploring},
	}}
	c := &runPipeline{}
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
		{pipelineStepFindTower, world.BlackMarsh, world.ForgottenTower, world.EntranceKindWildernessToTower},
		{pipelineStepEnterCellar1, world.ForgottenTower, world.TowerCellarLevel1, world.EntranceKindUnknown},
		{pipelineStepEnterCellar2, world.TowerCellarLevel1, world.TowerCellarLevel2, world.EntranceKindTowerCellarDown},
		{pipelineStepEnterCellar3, world.TowerCellarLevel2, world.TowerCellarLevel3, world.EntranceKindTowerCellarDown},
		{pipelineStepEnterCellar4, world.TowerCellarLevel3, world.TowerCellarLevel4, world.EntranceKindTowerCellarDown},
		{pipelineStepEnterCellar5, world.TowerCellarLevel4, world.TowerCellarLevel5, world.EntranceKindTowerCellarDown},
	}
	for _, tc := range cases {
		t.Run(tc.step, func(t *testing.T) {
			c := &runPipeline{}
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
	c := &runPipeline{}
	nav := &mockNavigator{tickResults: []pathing.NavTickResult{
		{Status: pathing.NavClicking},
	}}
	goal, ok := countessNavigationGoal(pipelineStepEnterCellar1)
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
	c := &runPipeline{}
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
	goal, ok := countessNavigationGoal(pipelineStepFindTower)
	if !ok {
		t.Fatal("missing find_tower goal")
	}
	res := countessNavigationSourceGuard(pipelineStepFindTower, areaState(world.TamoeHighland), goal)
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
			c := &runPipeline{}
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
			c := &runPipeline{}
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
