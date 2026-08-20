package tasks

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/profile"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/telemetry"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/town"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

type fakeChestOperate struct {
	clicks []uint32
	resets int
	open   bool
	world  *world.State
}

func (f *fakeChestOperate) Tick(_ world.State, target world.Object, _ float64) ChestOperateResult {
	f.clicks = append(f.clicks, target.UnitID)
	if f.open && f.world != nil {
		for i := range f.world.Objects {
			if f.world.Objects[i].UnitID == target.UnitID {
				f.world.Objects[i].Mode = world.ObjectModeOpened
			}
		}
	}
	return ChestOperateResult{Status: ChestOperateClicked, Done: true}
}

func (f *fakeChestOperate) Reset() { f.resets++ }

type blockerChestOperate struct {
	world *world.State
	ticks int
}

func (f *blockerChestOperate) Tick(state world.State, target world.Object, _ float64) ChestOperateResult {
	f.ticks++
	if len(state.Monsters) > 0 {
		if f.ticks%2 == 1 {
			return ChestOperateResult{Status: ChestOperatePending, BlockerUnitID: state.Monsters[0].UnitID}
		}
		return ChestOperateResult{Status: ChestOperateHoverNotFound, Done: true, BlockerUnitID: state.Monsters[0].UnitID}
	}
	if f.world != nil {
		for i := range f.world.Objects {
			if f.world.Objects[i].UnitID == target.UnitID {
				f.world.Objects[i].Mode = world.ObjectModeOpened
			}
		}
	}
	return ChestOperateResult{Status: ChestOperateClicked, Done: true}
}

func (f *blockerChestOperate) Reset() {}

type hoverFailChestOperate struct {
	blockerUnitID uint32
	ticks         int
}

func (f *hoverFailChestOperate) Tick(world.State, world.Object, float64) ChestOperateResult {
	f.ticks++
	return ChestOperateResult{Status: ChestOperateHoverNotFound, Done: true, BlockerUnitID: f.blockerUnitID}
}

func (f *hoverFailChestOperate) Reset() {}

type sequencedChestOperate struct {
	world   *world.State
	results []ChestOperateResult
	ticks   int
}

func (f *sequencedChestOperate) Tick(_ world.State, target world.Object, _ float64) ChestOperateResult {
	idx := f.ticks
	if idx >= len(f.results) {
		idx = len(f.results) - 1
	}
	f.ticks++
	res := f.results[idx]
	if res.Status == ChestOperateClicked && f.world != nil {
		for i := range f.world.Objects {
			if f.world.Objects[i].UnitID == target.UnitID {
				f.world.Objects[i].Mode = world.ObjectModeOpened
			}
		}
	}
	return res
}

func (f *sequencedChestOperate) Reset() {}

type chestLocalClear struct {
	state  *world.State
	ticks  int
	resets int
	keep   bool
}

func (f *chestLocalClear) TickRouteClear(_ context.Context, _ profile.RouteClearRequest, _ time.Time) profile.Result {
	f.ticks++
	if f.state != nil && !f.keep {
		f.state.Monsters = nil
	}
	return profile.Result{Status: profile.StatusAction}
}

func (f *chestLocalClear) ResetRouteClear() { f.resets++ }

type teleportingCombat struct {
	mockCombatActions
	state *world.State
}

func (c *teleportingCombat) TeleportToward(_ time.Time, _ world.Player, target world.Position, _ float64) (bool, error) {
	if c.state != nil {
		c.state.Player.Position = target
	}
	return true, nil
}

type incrementalTeleportCombat struct {
	mockCombatActions
	state *world.State
	step  uint32
}

func (c *incrementalTeleportCombat) TeleportToward(_ time.Time, _ world.Player, target world.Position, _ float64) (bool, error) {
	if c.state == nil {
		return true, nil
	}
	position := c.state.Player.Position
	if position.X > target.X {
		delta := min(c.step, position.X-target.X)
		position.X -= delta
	} else if position.X < target.X {
		delta := min(c.step, target.X-position.X)
		position.X += delta
	}
	if position.Y > target.Y {
		delta := min(c.step, position.Y-target.Y)
		position.Y -= delta
	} else if position.Y < target.Y {
		delta := min(c.step, target.Y-position.Y)
		position.Y += delta
	}
	c.state.Player.Position = position
	return true, nil
}

func chestSweepDeps(chest ChestOperateActions, state *world.State, trace ...*pipelineTelemetry) pipelineChestDeps {
	deps := pipelineChestDeps{Chest: chest, Combat: &teleportingCombat{state: state}, Loot: &mockLootActions{}}
	if len(trace) > 0 {
		deps.Telemetry = trace[0]
	}
	return deps
}

func clickCounts(clicks []uint32) map[uint32]int {
	out := make(map[uint32]int, len(clicks))
	for _, id := range clicks {
		out[id]++
	}
	return out
}

func lowerKurastSweepState(objects []world.Object, keys int) world.State {
	state := world.State{
		Valid:   true,
		Phase:   world.GamePhaseInGame,
		Area:    world.LookupArea(world.LowerKurast),
		Player:  world.Player{Position: world.Position{X: 5032, Y: 2994}},
		Objects: objects,
	}
	if keys > 0 {
		state.Items = []world.Item{{
			Code: town.KeyItemCode, Location: world.ItemLocationInventory, PlayerOwned: true, Page: 0,
			Quantity: keys, QuantityKnown: true,
		}}
	}
	return state
}

func TestChestSweepFailsWhenNoSuperChestVisible(t *testing.T) {
	definition, _ := DefaultRunRegistry().Definition(RunIDLowerKurast)
	pipeline := &runPipeline{definition: definition}
	state := lowerKurastSweepState(nil, 0)
	res := pipeline.tickChestSweep(context.Background(), pipelineChestDeps{Chest: &fakeChestOperate{}, Loot: &mockLootActions{}}, state, time.Now())
	if !res.failed || res.reason != string(RunReasonChestSweepEmpty) {
		t.Fatalf("empty sweep = %+v, want failed %s", res, RunReasonChestSweepEmpty)
	}
}

func TestChestSweepIgnoresRackWithoutNearbySuperChest(t *testing.T) {
	definition, _ := DefaultRunRegistry().Definition(RunIDLowerKurast)
	pipeline := &runPipeline{definition: definition}
	chest := &fakeChestOperate{}
	state := lowerKurastSweepState([]world.Object{
		closedObject(world.WeaponRack2ID, world.ObjectKindRack, 99, 5032, 2994),
	}, 5)
	handled, res := pipeline.tickChestWork(context.Background(), pipelineChestDeps{Chest: chest, Loot: &mockLootActions{}}, state, time.Now(), chestSelectRoute)
	if handled || res.failed || len(chest.clicks) != 0 {
		t.Fatalf("route rack-only handled=%t result=%+v clicks=%v", handled, res, chest.clicks)
	}
	res = pipeline.tickChestSweep(context.Background(), pipelineChestDeps{Chest: chest, Loot: &mockLootActions{}}, state, time.Now())
	if !res.failed || res.reason != string(RunReasonChestSweepEmpty) || len(chest.clicks) != 0 {
		t.Fatalf("sweep rack-only = %+v clicks=%v", res, chest.clicks)
	}
}

func TestChestSweepSkipsLockedChestWithoutKeysAndDoesNotReclick(t *testing.T) {
	definition, _ := DefaultRunRegistry().Definition(RunIDLowerKurast)
	pipeline := &runPipeline{definition: definition}
	chest := &fakeChestOperate{}
	state := lowerKurastSweepState(liveHutCampObjects(), 0)
	trace := &pipelineTelemetry{}
	deps := chestSweepDeps(chest, &state, trace)
	now := time.Now()
	var terminal stepResult
	for i := 0; i < 200; i++ {
		tickAt := now.Add(time.Duration(i) * time.Second)
		state.At = tickAt
		terminal = pipeline.tickChestSweep(context.Background(), deps, state, tickAt)
		if terminal.failed || terminal.complete {
			break
		}
	}
	if terminal.failed {
		t.Fatalf("skip-without-key failed: %+v clicks=%v", terminal, chest.clicks)
	}
	if !terminal.complete {
		t.Fatalf("skip-without-key did not finish, phase=%q clicks=%v", pipeline.chest.phase, chest.clicks)
	}
	counts := clickCounts(chest.clicks)
	if counts[183] != 1 || counts[181] != 1 || counts[159] != 1 || counts[180] != 1 {
		t.Fatalf("superchest clicks = %v, want one each for 183/181/159/180", counts)
	}
	if counts[182] != 2 || counts[158] != 2 {
		t.Fatalf("hut rack clicks = %v, want two each (click + retry)", counts)
	}
	if pipeline.chest.skipped[183] != true || pipeline.chest.openedSuperChests != 0 {
		t.Fatalf("skip state skipped=%v opened=%d", pipeline.chest.skipped, pipeline.chest.openedSuperChests)
	}
	if got := countChestEvents(trace, telemetry.BossKillConfirmed); got != 0 {
		t.Fatalf("fake boss kills = %d", got)
	}
	if got := countChestEvents(trace, telemetry.ChestSkipped); got < 4 {
		t.Fatalf("chest_skipped events = %d, want at least 4: %+v", got, trace.events)
	}
	if got := countChestEvents(trace, telemetry.ChestOpened) + countChestEvents(trace, telemetry.RackOperated); got != 0 {
		t.Fatalf("opened events on skip run = chest=%d rack=%d", countChestEvents(trace, telemetry.ChestOpened), countChestEvents(trace, telemetry.RackOperated))
	}
}

func TestChestSweepRetriesClickWithoutModeChangeThenSkips(t *testing.T) {
	definition, _ := DefaultRunRegistry().Definition(RunIDLowerKurast)
	pipeline := &runPipeline{definition: definition}
	chest := &fakeChestOperate{}
	objects := []world.Object{
		closedObject(world.JungleChest2ID, world.ObjectKindSuperChest, 183, 5032, 2994),
		closedObject(world.ArmorStand1ID, world.ObjectKindRack, 182, 5012, 2983),
	}
	state := lowerKurastSweepState(objects, 5)
	deps := chestSweepDeps(chest, &state)
	now := time.Now()
	var terminal stepResult
	for i := 0; i < 200; i++ {
		tickAt := now.Add(time.Duration(i) * time.Second)
		state.At = tickAt
		terminal = pipeline.tickChestSweep(context.Background(), deps, state, tickAt)
		if terminal.failed || terminal.complete {
			break
		}
	}
	if terminal.failed || !terminal.complete {
		t.Fatalf("retry sweep = %+v clicks=%v", terminal, chest.clicks)
	}
	counts := clickCounts(chest.clicks)
	if counts[183] != 2 {
		t.Fatalf("chest 183 clicks = %d, want 2 (click + retry)", counts[183])
	}
	if counts[182] != 2 {
		t.Fatalf("rack 182 clicks = %d, want 2", counts[182])
	}
}

func TestChestSweepClearsObservedMonsterBlockerThenRetriesObject(t *testing.T) {
	definition, _ := DefaultRunRegistry().Definition(RunIDLowerKurast)
	pipeline := &runPipeline{definition: definition}
	state := lowerKurastSweepState([]world.Object{
		closedObject(world.JungleChest2ID, world.ObjectKindSuperChest, 126, 5027, 3012),
		closedObject(world.ArmorStand1ID, world.ObjectKindRack, 127, 5012, 2983),
	}, 5)
	state.Player.Position = world.Position{X: 5027, Y: 3012}
	state.Monsters = []world.Monster{{
		UnitID: 77, NPCID: 1, Position: world.Position{X: 5028, Y: 3012}, IsHovered: true,
	}}
	chest := &blockerChestOperate{world: &state}
	clear := &chestLocalClear{state: &state}
	trace := &pipelineTelemetry{}
	deps := pipelineChestDeps{
		Chest: chest, Combat: &teleportingCombat{state: &state}, Loot: &mockLootActions{},
		RouteClear: clear, Telemetry: trace,
	}
	now := time.Now()
	var terminal stepResult
	for i := 0; i < 100; i++ {
		tickAt := now.Add(time.Duration(i) * 100 * time.Millisecond)
		state.At = tickAt
		terminal = pipeline.tickChestSweep(context.Background(), deps, state, tickAt)
		if terminal.failed || terminal.complete {
			break
		}
	}
	if terminal.failed || !terminal.complete {
		t.Fatalf("blocker recovery = %+v phase=%q", terminal, pipeline.chest.phase)
	}
	if clear.ticks != 1 || clear.resets == 0 {
		t.Fatalf("local clear ticks/resets = %d/%d", clear.ticks, clear.resets)
	}
	if got := countChestEvents(trace, telemetry.ChestOpened); got != 1 {
		t.Fatalf("chest_opened = %d, events=%+v", got, trace.events)
	}
	if got := countChestEvents(trace, telemetry.ChestSkipped); got != 0 {
		t.Fatalf("chest_skipped = %d, events=%+v", got, trace.events)
	}
}

func TestChestSweepDoesNotClearWithoutLocalMonsterEvidence(t *testing.T) {
	definition, _ := DefaultRunRegistry().Definition(RunIDLowerKurast)
	pipeline := &runPipeline{definition: definition}
	rack := closedObject(world.ArmorStand1ID, world.ObjectKindRack, 127, 5012, 2983)
	rack.Mode = world.ObjectModeOpened
	state := lowerKurastSweepState([]world.Object{
		closedObject(world.JungleChest2ID, world.ObjectKindSuperChest, 126, 5027, 3012),
		rack,
	}, 5)
	clear := &chestLocalClear{state: &state}
	trace := &pipelineTelemetry{}
	deps := pipelineChestDeps{
		Chest: &hoverFailChestOperate{}, Combat: &teleportingCombat{state: &state},
		Loot: &mockLootActions{}, RouteClear: clear, Telemetry: trace,
	}
	now := time.Now()
	var terminal stepResult
	for i := 0; i < 20; i++ {
		tickAt := now.Add(time.Duration(i) * 100 * time.Millisecond)
		state.At = tickAt
		terminal = pipeline.tickChestSweep(context.Background(), deps, state, tickAt)
		if terminal.failed || terminal.complete {
			break
		}
	}
	if terminal.failed || !terminal.complete {
		t.Fatalf("no-evidence sweep = %+v phase=%q", terminal, pipeline.chest.phase)
	}
	if clear.ticks != 0 {
		t.Fatalf("local clear ran %d times without monster evidence", clear.ticks)
	}
	if got := countChestEvents(trace, telemetry.ChestSkipped); got != 1 {
		t.Fatalf("chest_skipped = %d, events=%+v", got, trace.events)
	}
}

func TestChestSweepClearsNearestHostileWhenHoveredUnitIsMissing(t *testing.T) {
	definition, _ := DefaultRunRegistry().Definition(RunIDLowerKurast)
	pipeline := &runPipeline{definition: definition}
	rack := closedObject(world.ArmorStand1ID, world.ObjectKindRack, 127, 5012, 2983)
	rack.Mode = world.ObjectModeOpened
	state := lowerKurastSweepState([]world.Object{
		closedObject(world.JungleChest2ID, world.ObjectKindSuperChest, 126, 5027, 3012),
		rack,
	}, 5)
	state.Player.Position = world.Position{X: 5027, Y: 3012}
	state.Monsters = []world.Monster{{
		UnitID: 77, NPCID: 1, Position: world.Position{X: 5028, Y: 3012},
	}}
	chest := &sequencedChestOperate{
		world: &state,
		results: []ChestOperateResult{
			{Status: ChestOperateHoverNotFound, Done: true, BlockerUnitID: 46},
			{Status: ChestOperateClicked, Done: true},
		},
	}
	clear := &chestLocalClear{state: &state}
	trace := &pipelineTelemetry{}
	deps := pipelineChestDeps{
		Chest: chest, Combat: &teleportingCombat{state: &state}, Loot: &mockLootActions{},
		RouteClear: clear, Telemetry: trace,
	}
	now := time.Now()
	var terminal stepResult
	for i := 0; i < 100; i++ {
		tickAt := now.Add(time.Duration(i) * 100 * time.Millisecond)
		state.At = tickAt
		terminal = pipeline.tickChestSweep(context.Background(), deps, state, tickAt)
		if terminal.failed || terminal.complete {
			break
		}
	}
	if terminal.failed || !terminal.complete {
		t.Fatalf("missing-hover-id recovery = %+v phase=%q ticks=%d", terminal, pipeline.chest.phase, chest.ticks)
	}
	if clear.ticks == 0 {
		t.Fatal("local clear did not run for nearest living hostile")
	}
	if got := countChestEvents(trace, telemetry.ChestOpened); got != 1 {
		t.Fatalf("chest_opened = %d, events=%+v", got, trace.events)
	}
}

func TestChestSweepSkipsWhenHoveredMonsterHasNoLivingHostileNearby(t *testing.T) {
	definition, _ := DefaultRunRegistry().Definition(RunIDLowerKurast)
	pipeline := &runPipeline{definition: definition}
	rack := closedObject(world.ArmorStand1ID, world.ObjectKindRack, 127, 5012, 2983)
	rack.Mode = world.ObjectModeOpened
	state := lowerKurastSweepState([]world.Object{
		closedObject(world.JungleChest2ID, world.ObjectKindSuperChest, 126, 5027, 3012),
		rack,
	}, 5)
	state.Player.Position = world.Position{X: 5027, Y: 3012}
	state.Monsters = []world.Monster{{
		UnitID: 77, NPCID: 1, Position: world.Position{X: 5200, Y: 3200},
	}}
	clear := &chestLocalClear{state: &state}
	trace := &pipelineTelemetry{}
	deps := pipelineChestDeps{
		Chest: &hoverFailChestOperate{blockerUnitID: 46}, Combat: &teleportingCombat{state: &state},
		Loot: &mockLootActions{}, RouteClear: clear, Telemetry: trace,
	}
	now := time.Now()
	var terminal stepResult
	for i := 0; i < 20; i++ {
		tickAt := now.Add(time.Duration(i) * 100 * time.Millisecond)
		state.At = tickAt
		terminal = pipeline.tickChestSweep(context.Background(), deps, state, tickAt)
		if terminal.failed || terminal.complete {
			break
		}
	}
	if terminal.failed || !terminal.complete {
		t.Fatalf("far hostile skip = %+v phase=%q", terminal, pipeline.chest.phase)
	}
	if clear.ticks != 0 {
		t.Fatalf("local clear ran %d times for a hostile outside the object radius", clear.ticks)
	}
	if got := countChestEvents(trace, telemetry.ChestSkipped); got != 1 {
		t.Fatalf("chest_skipped = %d, events=%+v", got, trace.events)
	}
}

func TestChestSweepClearsLootHoverBlockerThenRetriesPickup(t *testing.T) {
	definition, _ := DefaultRunRegistry().Definition(RunIDLowerKurast)
	pipeline := &runPipeline{definition: definition}
	rack := closedObject(world.ArmorStand1ID, world.ObjectKindRack, 127, 5012, 2983)
	rack.Mode = world.ObjectModeOpened
	state := lowerKurastSweepState([]world.Object{
		closedObject(world.JungleChest2ID, world.ObjectKindSuperChest, 126, 5027, 3012),
		rack,
	}, 5)
	state.Player.Position = world.Position{X: 5027, Y: 3012}
	gem := LootTarget{
		UnitID: 295, Code: "gzv", Name: "Flawless Amethyst",
		Position: world.Position{X: 5027, Y: 3012}, AreaID: world.LowerKurast,
	}
	state.Hover = world.HoverInfo{IsHovered: true, UnitType: world.HoverUnitTypeMonster, UnitID: 43}
	state.Monsters = []world.Monster{{
		UnitID: 77, NPCID: 1, Position: world.Position{X: 5028, Y: 3012},
	}}
	chest := &fakeChestOperate{open: true, world: &state}
	loot := &mockLootActions{
		scans: []LootScanResult{
			{HasTarget: true, NextTarget: gem},
			{HasTarget: true, NextTarget: gem},
		},
		ticks: []LootPickupResult{
			{Status: LootPickupHoverNotFound, Done: true, Target: gem},
			{Status: LootPickupPickedUp, Done: true, Target: gem},
		},
	}
	clear := &chestLocalClear{state: &state}
	deps := pipelineChestDeps{
		Chest: chest, Combat: &teleportingCombat{state: &state}, Loot: loot,
		RouteClear: clear,
	}
	now := time.Now()
	var terminal stepResult
	for i := 0; i < 80; i++ {
		tickAt := now.Add(time.Duration(i) * 100 * time.Millisecond)
		state.At = tickAt
		terminal = pipeline.tickChestSweep(context.Background(), deps, state, tickAt)
		if terminal.failed || terminal.complete {
			break
		}
	}
	if terminal.failed || !terminal.complete {
		t.Fatalf("loot blocker recovery = %+v phase=%q start=%d ticks=%d", terminal, pipeline.chest.phase, len(loot.startCalls), loot.tickCalls)
	}
	if clear.ticks == 0 {
		t.Fatal("local clear did not run for loot hover blocker")
	}
	if loot.tickCalls != 2 || len(loot.startCalls) != 2 {
		t.Fatalf("pickup start/tick = %d/%d, want 2/2 after clear retry", len(loot.startCalls), loot.tickCalls)
	}
	if len(loot.clearSkipIDs) == 0 || loot.clearSkipIDs[0] != 295 {
		t.Fatalf("ClearSkippedPickup = %v, want item 295 after clear", loot.clearSkipIDs)
	}
}

func TestChestSweepRetriesRouteHoverMissOnceFromLeftoverSweep(t *testing.T) {
	definition, _ := DefaultRunRegistry().Definition(RunIDLowerKurast)
	pipeline := &runPipeline{definition: definition}
	rack := closedObject(world.ArmorStand1ID, world.ObjectKindRack, 127, 5012, 2983)
	rack.Mode = world.ObjectModeOpened
	state := lowerKurastSweepState([]world.Object{
		closedObject(world.JungleChest2ID, world.ObjectKindSuperChest, 126, 5027, 3012),
		rack,
	}, 5)
	state.Player.Position = world.Position{X: 5027, Y: 3012}
	chest := &sequencedChestOperate{
		world: &state,
		results: []ChestOperateResult{
			{Status: ChestOperateHoverNotFound, Done: true},
			{Status: ChestOperateClicked, Done: true},
		},
	}
	trace := &pipelineTelemetry{}
	deps := chestSweepDeps(chest, &state, trace)
	now := time.Now()
	handled, routeRes := pipeline.tickChestWork(context.Background(), deps, state, now, chestSelectRoute)
	if !handled || routeRes.failed || !pipeline.chest.skipped[126] {
		t.Fatalf("route hover miss handled=%t result=%+v skipped=%v", handled, routeRes, pipeline.chest.skipped)
	}
	if chest.ticks != 1 {
		t.Fatalf("route operate ticks = %d, want 1", chest.ticks)
	}
	var terminal stepResult
	for i := 0; i < 40; i++ {
		tickAt := now.Add(time.Duration(i+1) * 100 * time.Millisecond)
		state.At = tickAt
		terminal = pipeline.tickChestSweep(context.Background(), deps, state, tickAt)
		if terminal.failed || terminal.complete {
			break
		}
	}
	if terminal.failed || !terminal.complete {
		t.Fatalf("leftover retry = %+v phase=%q ticks=%d", terminal, pipeline.chest.phase, chest.ticks)
	}
	if chest.ticks != 2 {
		t.Fatalf("operate ticks = %d, want route miss plus leftover retry", chest.ticks)
	}
	if got := countChestEvents(trace, telemetry.ChestOpened); got != 1 {
		t.Fatalf("chest_opened = %d, events=%+v", got, trace.events)
	}
}

func TestChestSweepDoesNotLoopLeftoverHoverMissForever(t *testing.T) {
	definition, _ := DefaultRunRegistry().Definition(RunIDLowerKurast)
	pipeline := &runPipeline{definition: definition}
	rack := closedObject(world.ArmorStand1ID, world.ObjectKindRack, 127, 5012, 2983)
	rack.Mode = world.ObjectModeOpened
	state := lowerKurastSweepState([]world.Object{
		closedObject(world.JungleChest2ID, world.ObjectKindSuperChest, 126, 5027, 3012),
		rack,
	}, 5)
	state.Player.Position = world.Position{X: 5027, Y: 3012}
	chest := &hoverFailChestOperate{}
	trace := &pipelineTelemetry{}
	deps := chestSweepDeps(chest, &state, trace)
	now := time.Now()
	if _, res := pipeline.tickChestWork(context.Background(), deps, state, now, chestSelectRoute); res.failed {
		t.Fatalf("route hover miss failed: %+v", res)
	}
	var terminal stepResult
	for i := 0; i < 40; i++ {
		tickAt := now.Add(time.Duration(i+1) * 100 * time.Millisecond)
		state.At = tickAt
		terminal = pipeline.tickChestSweep(context.Background(), deps, state, tickAt)
		if terminal.failed || terminal.complete {
			break
		}
	}
	if terminal.failed || !terminal.complete {
		t.Fatalf("bounded leftover retry = %+v ticks=%d", terminal, chest.ticks)
	}
	if chest.ticks != 2 {
		t.Fatalf("operate ticks = %d, want one leftover retry then skip", chest.ticks)
	}
	if got := countChestEvents(trace, telemetry.ChestSkipped); got != 2 {
		t.Fatalf("chest_skipped = %d, want route miss plus leftover miss: %+v", got, trace.events)
	}
}

func TestChestSweepPersistentBlockerClearIsBoundedAndRetriesOnce(t *testing.T) {
	definition, _ := DefaultRunRegistry().Definition(RunIDLowerKurast)
	pipeline := &runPipeline{definition: definition}
	rack := closedObject(world.ArmorStand1ID, world.ObjectKindRack, 127, 5012, 2983)
	rack.Mode = world.ObjectModeOpened
	state := lowerKurastSweepState([]world.Object{
		closedObject(world.JungleChest2ID, world.ObjectKindSuperChest, 126, 5027, 3012),
		rack,
	}, 5)
	state.Player.Position = world.Position{X: 5027, Y: 3012}
	state.Monsters = []world.Monster{{
		UnitID: 77, NPCID: 1, Position: world.Position{X: 5028, Y: 3012}, IsHovered: true,
	}}
	chest := &blockerChestOperate{world: &state}
	clear := &chestLocalClear{state: &state, keep: true}
	trace := &pipelineTelemetry{}
	deps := pipelineChestDeps{
		Chest: chest, Combat: &teleportingCombat{state: &state}, Loot: &mockLootActions{},
		RouteClear: clear, Telemetry: trace,
	}
	now := time.Now()
	var terminal stepResult
	for i := 0; i < 100; i++ {
		tickAt := now.Add(time.Duration(i) * 100 * time.Millisecond)
		state.At = tickAt
		terminal = pipeline.tickChestSweep(context.Background(), deps, state, tickAt)
		if terminal.failed || terminal.complete {
			break
		}
	}
	if terminal.failed || !terminal.complete {
		t.Fatalf("persistent blocker sweep = %+v phase=%q", terminal, pipeline.chest.phase)
	}
	if clear.ticks != chestBlockerMaxActions {
		t.Fatalf("local clear actions = %d, want bounded %d", clear.ticks, chestBlockerMaxActions)
	}
	if got := countChestEvents(trace, telemetry.ChestSkipped); got != 1 {
		t.Fatalf("chest_skipped = %d, events=%+v", got, trace.events)
	}
}

func TestLowerKurastBlockerClearHoldsRoute(t *testing.T) {
	definition, _ := DefaultRunRegistry().Definition(RunIDLowerKurast)
	state := lowerKurastSweepState([]world.Object{
		closedObject(world.JungleChest2ID, world.ObjectKindSuperChest, 126, 5027, 3012),
		closedObject(world.ArmorStand1ID, world.ObjectKindRack, 127, 5012, 2983),
	}, 5)
	state.Player.Position = world.Position{X: 5027, Y: 3012}
	state.Monsters = []world.Monster{{
		UnitID: 77, NPCID: 1, Position: world.Position{X: 5028, Y: 3012}, IsHovered: true,
	}}
	route := &mockRoutePlayback{progressOK: true}
	clear := &chestLocalClear{state: &state}
	pipeline := &runPipeline{definition: definition, core: pipelineCoreState{routeID: "lk-route"}}
	deps := Deps{
		Route: route, Chest: &blockerChestOperate{world: &state},
		Combat: &teleportingCombat{state: &state}, Loot: &mockLootActions{}, RouteClear: clear,
	}
	now := time.Now()
	for i := 0; i < 3; i++ {
		state.At = now.Add(time.Duration(i) * 100 * time.Millisecond)
		result := pipeline.onTravelTick(context.Background(), deps, pipelineStepPlayRoute, state, state.At, now)
		if result.failed || result.complete {
			t.Fatalf("route blocker tick %d = %+v", i, result)
		}
	}
	if clear.ticks != 1 || route.tickCalls != 0 || route.holdCalls != 3 {
		t.Fatalf("clear=%d route ticks/holds=%d/%d", clear.ticks, route.tickCalls, route.holdCalls)
	}
}

func TestChestSweepReachesLiveWestChestBeyondLootRetryBudget(t *testing.T) {
	definition, _ := DefaultRunRegistry().Definition(RunIDLowerKurast)
	pipeline := &runPipeline{definition: definition}
	state := lowerKurastSweepState([]world.Object{
		closedObject(world.JungleChest2ID, world.ObjectKindSuperChest, 132, 5008, 2963),
		closedObject(world.ArmorStand1ID, world.ObjectKindRack, 131, 5012, 2983),
	}, 5)
	state.Player.Position = world.Position{X: 5060, Y: 2963}
	chest := &fakeChestOperate{open: true, world: &state}
	trace := &pipelineTelemetry{}
	deps := pipelineChestDeps{
		Chest:     chest,
		Combat:    &incrementalTeleportCombat{state: &state, step: 10},
		Loot:      &mockLootActions{},
		Telemetry: trace,
	}
	now := time.Now()
	var terminal stepResult
	for i := 0; i < 100; i++ {
		tickAt := now.Add(time.Duration(i) * time.Second)
		state.At = tickAt
		terminal = pipeline.tickChestSweep(context.Background(), deps, state, tickAt)
		if terminal.failed || terminal.complete {
			break
		}
	}
	if terminal.failed || !terminal.complete {
		t.Fatalf("live west sweep = %+v position=%+v clicks=%v", terminal, state.Player.Position, chest.clicks)
	}
	if got := countChestEvents(trace, telemetry.ChestOpened); got != 1 {
		t.Fatalf("chest_opened = %d, events=%+v", got, trace.events)
	}
	if got := countChestEvents(trace, telemetry.ChestSkipped); got != 0 {
		t.Fatalf("chest_skipped = %d, events=%+v", got, trace.events)
	}
}

func TestChestSweepOpensCampAndDoesNotReclick(t *testing.T) {
	definition, _ := DefaultRunRegistry().Definition(RunIDLowerKurast)
	pipeline := &runPipeline{definition: definition}
	state := lowerKurastSweepState(liveHutCampObjects(), 5)
	chest := &fakeChestOperate{open: true, world: &state}
	trace := &pipelineTelemetry{}
	deps := chestSweepDeps(chest, &state, trace)
	now := time.Now()
	var terminal stepResult
	for i := 0; i < 200; i++ {
		tickAt := now.Add(time.Duration(i) * time.Second)
		state.At = tickAt
		terminal = pipeline.tickChestSweep(context.Background(), deps, state, tickAt)
		if terminal.failed || terminal.complete {
			break
		}
	}
	if terminal.failed || !terminal.complete {
		t.Fatalf("open sweep = %+v clicks=%v opened=%d", terminal, chest.clicks, pipeline.chest.openedSuperChests)
	}
	if pipeline.chest.openedSuperChests != 4 {
		t.Fatalf("opened superchests = %d, want 4", pipeline.chest.openedSuperChests)
	}
	counts := clickCounts(chest.clicks)
	if counts[183] != 1 || counts[181] != 1 || counts[159] != 1 || counts[180] != 1 || counts[182] != 1 || counts[158] != 1 {
		t.Fatalf("open clicks = %v, want one each hut object including extra 180", counts)
	}
	if got := countChestEvents(trace, telemetry.BossKillConfirmed); got != 0 {
		t.Fatalf("fake boss kills = %d", got)
	}
	if got := countChestEvents(trace, telemetry.ChestOpened); got != 4 {
		t.Fatalf("chest_opened = %d, want 4: %+v", got, trace.events)
	}
	if got := countChestEvents(trace, telemetry.RackOperated); got != 2 {
		t.Fatalf("rack_operated = %d, want 2: %+v", got, trace.events)
	}
}

func countChestEvents(trace *pipelineTelemetry, name telemetry.EventName) int {
	count := 0
	for _, event := range trace.events {
		if event.Event == name {
			count++
		}
	}
	return count
}

func TestLowerKurastFullPathDoesNotAcquireBoss(t *testing.T) {
	pipeline := &runPipeline{definition: mustDefinition(t, RunIDLowerKurast)}
	path := walkTransitionSteps(t, pipeline, pipeline.firstStep())
	joined := ""
	for i, step := range path {
		if i > 0 {
			joined += ">"
		}
		joined += step
		if step == pipelineStepAcquireBoss || step == pipelineStepEngageBoss {
			t.Fatalf("lower-kurast path includes boss step %q: %v", step, path)
		}
	}
	if path[5] != pipelineStepPlayRoute || path[6] != pipelineStepChestSweep {
		t.Fatalf("lower-kurast path = %s", joined)
	}
}

func TestTickBossFailsClosedOnChestSweepRun(t *testing.T) {
	pipeline := &runPipeline{definition: mustDefinition(t, RunIDLowerKurast)}
	res := pipeline.tickBoss(context.Background(), pipelineBossDeps{}, pipelineStepAcquireBoss, world.State{}, time.Now())
	if !res.failed || res.reason != "chest_sweep_used_boss_path" {
		t.Fatalf("boss path on LK = %+v", res)
	}
}

// completedRouteHold matches live Hold after play_bound_route returns done:
// the adapter still exists, but Progress is no longer active.
type completedRouteHold struct {
	holds int
}

func (r *completedRouteHold) Start(string, world.State) error { return nil }
func (r *completedRouteHold) Progress(world.State) (RouteProgress, bool) {
	return RouteProgress{}, false
}
func (r *completedRouteHold) Hold(world.State) error {
	r.holds++
	return fmt.Errorf("run route hold has no active progress")
}
func (r *completedRouteHold) Tick(context.Context, world.State) (bool, error) { return true, nil }
func (r *completedRouteHold) Reset()                                          {}

func TestTerminalChestSweepDoesNotHoldCompletedRoute(t *testing.T) {
	definition, _ := DefaultRunRegistry().Definition(RunIDLowerKurast)
	pipeline := &runPipeline{definition: definition}
	state := lowerKurastSweepState(liveHutCampObjects(), 5)
	route := &completedRouteHold{}
	deps := Deps{
		Chest:  &fakeChestOperate{open: true, world: &state},
		Combat: &teleportingCombat{state: &state},
		Loot:   &mockLootActions{},
		Route:  route,
	}
	now := time.Now()
	res := pipeline.onTick(context.Background(), deps, pipelineStepChestSweep, state, now, now, 0)
	if res.failed && res.reason == string(RouteThreatReasonStateInvalid) {
		t.Fatalf("terminal sweep failed on completed-route Hold: %+v holds=%d", res, route.holds)
	}
	if route.holds != 0 {
		t.Fatalf("terminal chest_sweep held completed route %d times", route.holds)
	}
}

func TestRouteChestOperateHoldsActiveRoute(t *testing.T) {
	definition, _ := DefaultRunRegistry().Definition(RunIDLowerKurast)
	pipeline := &runPipeline{definition: definition}
	state := lowerKurastSweepState(liveHutCampObjects(), 5)
	route := &mockRoutePlayback{}
	handled, res := pipeline.tickChestWork(context.Background(), pipelineChestDeps{
		Chest:  &fakeChestOperate{},
		Combat: &teleportingCombat{state: &state},
		Loot:   &mockLootActions{},
		Route:  route,
	}, state, time.Now(), chestSelectRoute)
	if !handled || res.failed || route.holdCalls != 1 {
		t.Fatalf("in-route operate handled=%t result=%+v holds=%d", handled, res, route.holdCalls)
	}
}

func TestLowerKurastPlayRouteOperatesWithoutRouteClear(t *testing.T) {
	definition, _ := DefaultRunRegistry().Definition(RunIDLowerKurast)
	if definition.HasCapability(RunCapabilityRouteClear) {
		t.Fatal("lower-kurast must not require route-clear for operate-on-sight")
	}
	state := lowerKurastSweepState(liveHutCampObjects(), 5)
	route := &mockRoutePlayback{progressOK: true}
	chest := &fakeChestOperate{}
	pipeline := &runPipeline{definition: definition, core: pipelineCoreState{routeID: "lk-route"}}
	result := pipeline.onTravelTick(context.Background(), Deps{
		Route:  route,
		Chest:  chest,
		Combat: &teleportingCombat{state: &state},
		Loot:   &mockLootActions{},
	}, pipelineStepPlayRoute, state, time.Now(), time.Now())
	if result.failed || result.complete {
		t.Fatalf("play route = %+v", result)
	}
	if route.holdCalls != 1 || route.tickCalls != 0 || len(chest.clicks) == 0 {
		t.Fatalf("operate-on-sight holds=%d ticks=%d clicks=%v", route.holdCalls, route.tickCalls, chest.clicks)
	}
}

func TestLowerKurastPlayRouteSkipsOperateWhenProgressUnavailable(t *testing.T) {
	definition, _ := DefaultRunRegistry().Definition(RunIDLowerKurast)
	state := lowerKurastSweepState(liveHutCampObjects(), 5)
	route := &mockRoutePlayback{progressOK: false}
	chest := &fakeChestOperate{}
	pipeline := &runPipeline{definition: definition, core: pipelineCoreState{routeID: "lk-route"}}
	result := pipeline.onTravelTick(context.Background(), Deps{
		Route:  route,
		Chest:  chest,
		Combat: &teleportingCombat{state: &state},
		Loot:   &mockLootActions{},
	}, pipelineStepPlayRoute, state, time.Now(), time.Now())
	if result.failed || result.complete {
		t.Fatalf("play route = %+v", result)
	}
	if route.holdCalls != 0 || route.tickCalls != 1 || len(chest.clicks) != 0 {
		t.Fatalf("unavailable progress holds=%d ticks=%d clicks=%v", route.holdCalls, route.tickCalls, chest.clicks)
	}
}
