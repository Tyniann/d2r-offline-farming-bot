package tasks

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/pathing"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/profile"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/telemetry"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

type routeClearMock struct {
	requests []profile.RouteClearRequest
	result   profile.Result
	resets   int
}

func (m *routeClearMock) TickRouteClear(_ context.Context, request profile.RouteClearRequest, _ time.Time) profile.Result {
	m.requests = append(m.requests, request)
	if m.result.Status == "" {
		return profile.Result{Status: profile.StatusPending}
	}
	return m.result
}

func (m *routeClearMock) ResetRouteClear() { m.resets++ }

func controllerRoute(progress RouteProgress) *mockRoutePlayback {
	return &mockRoutePlayback{progress: progress, progressOK: true}
}

func controllerDefinition(t *testing.T) RunDefinition {
	t.Helper()
	definition, ok := DefaultRunRegistry().Definition(RunIDSummoner)
	if !ok {
		t.Fatal("Summoner definition missing")
	}
	return definition
}

func controllerState(at time.Time, monsters ...world.Monster) world.State {
	state := phase17ThreatState(monsters...)
	state.At = at
	state.Area = world.LookupArea(world.ArcaneSanctuary)
	state.Identity = world.GameIdentity{Valid: true, CharacterName: "MrBones", Class: world.CharacterClassNecromancer}
	state.MonsterCoverage.EligibleMonsterCount = len(monsters)
	return state
}

func controllerTick(t *testing.T, controller *RouteThreatController, route *mockRoutePlayback, clear *routeClearMock, state world.State, progress RouteProgress, cfg RouteCombatConfig) RouteThreatTickResult {
	t.Helper()
	definition := controllerDefinition(t)
	assessment := assessThreats(state, progress, definition.RouteHostileNPCIDs, cfg)
	return controller.Tick(context.Background(), route, clear, state, progress, assessment, definition, cfg, "necro_bone_spear", state.At)
}

func TestRouteThreatControllerHoldsAndClearsImmediateThreat(t *testing.T) {
	base := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	progress, cfg := phase17ThreatProgress(), phase17ThreatConfig()
	cfg.NoProgressTimeout = 12 * time.Second
	cfg.NoProgressTimeout = 12 * time.Second
	state := controllerState(base, world.Monster{NPCID: world.ArcaneSpecter, UnitID: 7, Position: world.Position{X: 110, Y: 100}})
	route, clear := controllerRoute(progress), &routeClearMock{result: profile.Result{Status: profile.StatusAction}}
	var controller RouteThreatController

	result := controllerTick(t, &controller, route, clear, state, progress, cfg)
	if result.Failed || result.AllowMovement || result.State != RouteThreatClearing {
		t.Fatalf("result = %+v", result)
	}
	if route.holdCalls != 1 || route.tickCalls != 0 || len(clear.requests) != 1 || clear.requests[0].Target.UnitID != 7 {
		t.Fatalf("holds=%d ticks=%d requests=%+v", route.holdCalls, route.tickCalls, clear.requests)
	}
}

func TestRouteThreatControllerRequiresThreeClearSnapshotsAndFreshResume(t *testing.T) {
	base := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	progress, cfg := phase17ThreatProgress(), phase17ThreatConfig()
	cfg.NoProgressTimeout = 12 * time.Second
	route, clear := controllerRoute(progress), &routeClearMock{}
	var controller RouteThreatController
	threat := controllerState(base, world.Monster{NPCID: world.ArcaneSpecter, UnitID: 7, Position: world.Position{X: 110, Y: 100}})
	_ = controllerTick(t, &controller, route, clear, threat, progress, cfg)

	for i := 1; i <= Phase17StableClearSnapshots; i++ {
		safe := controllerState(base.Add(time.Duration(i) * time.Second))
		result := controllerTick(t, &controller, route, clear, safe, progress, cfg)
		if result.AllowMovement || result.Failed {
			t.Fatalf("clear snapshot %d = %+v", i, result)
		}
	}
	same := controllerState(base.Add(Phase17StableClearSnapshots * time.Second))
	if result := controllerTick(t, &controller, route, clear, same, progress, cfg); result.AllowMovement {
		t.Fatal("movement resumed on the stable-clear snapshot")
	}
	fresh := controllerState(base.Add((Phase17StableClearSnapshots + 1) * time.Second))
	if result := controllerTick(t, &controller, route, clear, fresh, progress, cfg); !result.AllowMovement || result.Failed {
		t.Fatalf("fresh resume = %+v", result)
	}
	if clear.resets != 1 {
		t.Fatalf("clear resets = %d", clear.resets)
	}
}

func TestRouteThreatControllerNoProgressAndProgressBeyondTwelveSeconds(t *testing.T) {
	base := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	progress, cfg := phase17ThreatProgress(), phase17ThreatConfig()
	cfg.NoProgressTimeout = 12 * time.Second
	monster := world.Monster{NPCID: world.ArcaneSpecter, UnitID: 7, Position: world.Position{X: 110, Y: 100}}

	route, clear := controllerRoute(progress), &routeClearMock{result: profile.Result{Status: profile.StatusAction}}
	var stuck RouteThreatController
	for _, elapsed := range []time.Duration{0, 4 * time.Second, 8 * time.Second, 12 * time.Second} {
		result := controllerTick(t, &stuck, route, clear, controllerState(base.Add(elapsed), monster), progress, cfg)
		if elapsed < 12*time.Second && result.Failed {
			t.Fatalf("premature failure at %s: %+v", elapsed, result)
		}
		if elapsed == 12*time.Second && (!result.Failed || result.Reason != RouteThreatReasonClearNoProgress) {
			t.Fatalf("no-progress result = %+v", result)
		}
	}

	route, clear = controllerRoute(progress), &routeClearMock{result: profile.Result{Status: profile.StatusAction}}
	var progressing RouteThreatController
	for i := 0; i < 25; i++ {
		state := controllerState(base.Add(time.Duration(i)*time.Second), monster)
		state.MonsterCoverage.EligibleMonsterCount = 100 - i
		result := controllerTick(t, &progressing, route, clear, state, progress, cfg)
		if result.Failed {
			t.Fatalf("progressing clear failed at %d: %+v", i, result)
		}
	}
	if len(clear.requests) != 25 {
		t.Fatalf("clear actions = %d", len(clear.requests))
	}
}

func TestRouteThreatControllerDensityReliefAndLocallyCompleteTruncation(t *testing.T) {
	base := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	progress, cfg := phase17ThreatProgress(), phase17ThreatConfig()
	cfg.NoProgressTimeout = 12 * time.Second
	density := world.Monster{NPCID: world.ArcaneSpecter, UnitID: 8, Position: world.Position{X: 125, Y: 108}}
	state := controllerState(base, density)
	state.MonsterCoverage = world.MonsterCoverage{EligibleMonsterCount: 513, MonstersTruncated: true, MonsterCoverageRadiusTiles: 40}
	route, clear := controllerRoute(progress), &routeClearMock{}
	var controller RouteThreatController
	result := controllerTick(t, &controller, route, clear, state, progress, cfg)
	if result.State != RouteThreatDensityRelief || len(clear.requests) != 1 || clear.requests[0].Mode != profile.RouteClearDensityRelief {
		t.Fatalf("density result=%+v requests=%+v", result, clear.requests)
	}

	locallyComplete := controllerState(base.Add(time.Second))
	locallyComplete.MonsterCoverage = world.MonsterCoverage{EligibleMonsterCount: 513, MonstersTruncated: true, MonsterCoverageRadiusTiles: 51}
	var safeController RouteThreatController
	safeRoute, safeClear := controllerRoute(progress), &routeClearMock{}
	safe := controllerTick(t, &safeController, safeRoute, safeClear, locallyComplete, progress, cfg)
	if !safe.AllowMovement || safeRoute.holdCalls != 0 || len(safeClear.requests) != 0 {
		t.Fatalf("locally complete = %+v holds=%d requests=%d", safe, safeRoute.holdCalls, len(safeClear.requests))
	}
}

func TestRouteThreatControllerFailsThirdOutOfRangeSnapshotAndResets(t *testing.T) {
	base := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	progress, cfg := phase17ThreatProgress(), phase17ThreatConfig()
	cfg.NoProgressTimeout = 12 * time.Second
	target := world.Monster{NPCID: world.ArcaneGhoulLord, UnitID: 9, Position: world.Position{X: 140, Y: 100}}
	route, clear := controllerRoute(progress), &routeClearMock{}
	var controller RouteThreatController
	for i := 0; i < Phase17StableClearSnapshots; i++ {
		result := controllerTick(t, &controller, route, clear, controllerState(base.Add(time.Duration(i)*time.Second), target), progress, cfg)
		if i < Phase17StableClearSnapshots-1 && result.Failed {
			t.Fatalf("premature range failure %d: %+v", i, result)
		}
		if i == Phase17StableClearSnapshots-1 && (!result.Failed || result.Reason != RouteThreatReasonOutOfRange) {
			t.Fatalf("range result = %+v", result)
		}
	}
	controller.Reset(clear)
	if controller.State() != RouteThreatMoving || clear.resets != 1 {
		t.Fatalf("reset state=%s resets=%d", controller.State(), clear.resets)
	}
}

func TestRouteThreatControllerFailsThirdUnprojectableSnapshotAsOutOfRange(t *testing.T) {
	base := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	progress, cfg := phase17ThreatProgress(), phase17ThreatConfig()
	cfg.NoProgressTimeout = 12 * time.Second
	target := world.Monster{NPCID: world.ArcaneGhoulLord, UnitID: 83, Position: world.Position{X: 130, Y: 100}}
	route := controllerRoute(progress)
	clear := &routeClearMock{result: profile.Result{
		Status: profile.StatusPending,
		Reason: profile.RouteClearReasonTargetUnprojectable,
	}}
	var controller RouteThreatController

	for i := 0; i < Phase17StableClearSnapshots; i++ {
		result := controllerTick(t, &controller, route, clear, controllerState(base.Add(time.Duration(i)*time.Second), target), progress, cfg)
		if i < Phase17StableClearSnapshots-1 && result.Failed {
			t.Fatalf("premature projection failure %d: %+v", i, result)
		}
		if i == Phase17StableClearSnapshots-1 && (!result.Failed || result.Reason != RouteThreatReasonOutOfRange) {
			t.Fatalf("projection result = %+v", result)
		}
	}
	if route.tickCalls != 0 || len(clear.requests) != Phase17StableClearSnapshots {
		t.Fatalf("route ticks=%d clear requests=%d", route.tickCalls, len(clear.requests))
	}
}

func TestSummonerRouteOutOfRangeForceMovesAndAcceptsMeasuredProgress(t *testing.T) {
	base := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	progress, cfg := phase17ThreatProgress(), phase17ThreatConfig()
	cfg.NoProgressTimeout = 12 * time.Second
	target := world.Monster{NPCID: world.ArcaneGhoulLord, UnitID: 117, Position: world.Position{X: 140, Y: 100}}
	route, clear, combat := controllerRoute(progress), &routeClearMock{}, &mockCombatActions{}
	trace := &pipelineTelemetry{}
	pipeline := &runPipeline{
		definition: controllerDefinition(t), phase: RunPhasePlayRoute, core: pipelineCoreState{routeID: "summoner-route",
			combat: CombatConfig{Profile: "necro_bone_spear"}, routeCombat: cfg},
	}
	deps := Deps{Route: route, RouteClear: clear, Combat: combat, Telemetry: trace}

	for i := 0; i < Phase17StableClearSnapshots; i++ {
		state := controllerState(base.Add(time.Duration(i)*100*time.Millisecond), target)
		if result := pipeline.onTravelTick(context.Background(), deps, pipelineStepPlayRoute, state, state.At, base); result.failed {
			t.Fatalf("approach trigger tick %d failed: %+v", i, result)
		}
	}
	if combat.forceMoveCalls != 1 || combat.lastForceMoveTarget != progress.MovementTarget || route.tickCalls != 0 {
		t.Fatalf("force moves=%d target=%+v route ticks=%d", combat.forceMoveCalls, combat.lastForceMoveTarget, route.tickCalls)
	}

	progressed := controllerState(base.Add(700*time.Millisecond), target)
	progressed.Player.Position = world.Position{X: 105, Y: 100}
	if result := pipeline.onTravelTick(context.Background(), deps, pipelineStepPlayRoute, progressed, progressed.At, base); result.failed {
		t.Fatalf("measured approach progress failed: %+v", result)
	}
	if pipeline.travel.routeApproachPending || pipeline.travel.routeApproachFailures != 0 || combat.forceMoveCalls != 1 ||
		pipeline.travel.routeThreat.outOfRangeTicks != 0 {
		t.Fatalf("pending=%t failures=%d force moves=%d out-of-range ticks=%d",
			pipeline.travel.routeApproachPending, pipeline.travel.routeApproachFailures, combat.forceMoveCalls, pipeline.travel.routeThreat.outOfRangeTicks)
	}
	var approachAction, approachProgress *telemetry.Event
	for i := range trace.events {
		event := &trace.events[i]
		if event.Event == telemetry.RouteClearAction && event.ActionKind == "force_move" {
			approachAction = event
		}
		if event.Event == telemetry.RouteClearProgress && event.ProgressKind == "approach" {
			approachProgress = event
		}
	}
	if approachAction == nil || approachAction.Attempt != 1 || approachAction.UnitID != target.UnitID {
		t.Fatalf("approach action = %+v events=%+v", approachAction, trace.events)
	}
	if approachProgress == nil || approachProgress.UnitID != target.UnitID ||
		approachProgress.PositionProgressTiles != 5 {
		t.Fatalf("approach progress = %+v events=%+v", approachProgress, trace.events)
	}
}

func TestRouteApproachDirectionalProgressAcceptsRoundedForwardStep(t *testing.T) {
	progress := routeApproachDirectionalProgress(
		world.Position{X: 100, Y: 100},
		world.Position{X: 108, Y: 101},
		world.Position{X: 100, Y: 101},
	)
	if progress <= routeThreatApproachProgressEpsilonTiles || progress >= 1 {
		t.Fatalf("rounded diagonal progress = %.3f, want positive accepted sub-tile projection", progress)
	}
}

func TestSummonerRouteApproachExhaustionDefersToNoProgressWatchdog(t *testing.T) {
	base := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	progress, cfg := phase17ThreatProgress(), phase17ThreatConfig()
	cfg.NoProgressTimeout = 12 * time.Second
	target := world.Monster{NPCID: world.ArcaneHellClan, UnitID: 133, Position: world.Position{X: 140, Y: 100}}
	route, clear, combat := controllerRoute(progress), &routeClearMock{}, &mockCombatActions{}
	trace := &pipelineTelemetry{}
	pipeline := &runPipeline{
		definition: controllerDefinition(t), phase: RunPhasePlayRoute, core: pipelineCoreState{routeID: "summoner-route",
			combat: CombatConfig{Profile: "necro_bone_spear"}, routeCombat: cfg},
	}
	deps := Deps{Route: route, RouteClear: clear, Combat: combat, Telemetry: trace}

	for i := 0; i < Phase17StableClearSnapshots; i++ {
		state := controllerState(base.Add(time.Duration(i)*100*time.Millisecond), target)
		result := pipeline.onTravelTick(context.Background(), deps, pipelineStepPlayRoute, state, state.At, base)
		if result.failed {
			t.Fatalf("initial tick %d failed: %+v", i, result)
		}
	}
	for failure := 1; failure <= routeThreatApproachMaxFailures; failure++ {
		state := controllerState(base.Add(200*time.Millisecond+time.Duration(failure)*routeThreatApproachSettle), target)
		result := pipeline.onTravelTick(context.Background(), deps, pipelineStepPlayRoute, state, state.At, base)
		if result.failed {
			t.Fatalf("approach failure %d bypassed shared watchdog: %+v", failure, result)
		}
	}
	if combat.forceMoveCalls != routeThreatApproachMaxFailures || route.tickCalls != 0 ||
		pipeline.travel.routeApproachExhaustedUnitID != target.UnitID {
		t.Fatalf("force moves=%d route ticks=%d exhausted=%d", combat.forceMoveCalls, route.tickCalls, pipeline.travel.routeApproachExhaustedUnitID)
	}

	timedOut := controllerState(base.Add(cfg.NoProgressTimeout+time.Second), target)
	result := pipeline.onTravelTick(context.Background(), deps, pipelineStepPlayRoute, timedOut, timedOut.At, base)
	if !result.failed || result.reason != string(RouteThreatReasonClearNoProgress) {
		t.Fatalf("watchdog result = %+v, want %s", result, RouteThreatReasonClearNoProgress)
	}
	var attempts []int
	noProgressEvents := 0
	for _, event := range trace.events {
		if event.Event == telemetry.RouteClearAction && event.ActionKind == "force_move" {
			attempts = append(attempts, event.Attempt)
		}
		if event.Event == telemetry.RouteClearProgress && event.ProgressKind == "approach" {
			t.Fatalf("ineffective approach emitted progress: %+v", event)
		}
		if event.Event == telemetry.RouteClearProgress && event.ProgressKind == "approach_no_progress" {
			noProgressEvents++
		}
	}
	if !reflect.DeepEqual(attempts, []int{1, 2, 3}) {
		t.Fatalf("approach attempts = %v", attempts)
	}
	if noProgressEvents != routeThreatApproachMaxFailures {
		t.Fatalf("no-progress telemetry events = %d, want %d", noProgressEvents, routeThreatApproachMaxFailures)
	}
}

func TestSummonerRouteUnprojectableInRangeUsesForceMoveFallback(t *testing.T) {
	base := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	progress, cfg := phase17ThreatProgress(), phase17ThreatConfig()
	cfg.NoProgressTimeout = 12 * time.Second
	target := world.Monster{NPCID: world.ArcaneGhoulLord, UnitID: 83, Position: world.Position{X: 130, Y: 100}}
	route := controllerRoute(progress)
	clear := &routeClearMock{result: profile.Result{Status: profile.StatusPending, Reason: profile.RouteClearReasonTargetUnprojectable}}
	combat := &mockCombatActions{}
	pipeline := &runPipeline{
		definition: controllerDefinition(t), phase: RunPhasePlayRoute, core: pipelineCoreState{routeID: "summoner-route",
			combat: CombatConfig{Profile: "necro_bone_spear"}, routeCombat: cfg},
	}
	for i := 0; i < Phase17StableClearSnapshots; i++ {
		state := controllerState(base.Add(time.Duration(i)*100*time.Millisecond), target)
		result := pipeline.onTravelTick(context.Background(), Deps{Route: route, RouteClear: clear, Combat: combat}, pipelineStepPlayRoute, state, state.At, base)
		if result.failed {
			t.Fatalf("unprojectable tick %d failed: %+v", i, result)
		}
	}
	if combat.forceMoveCalls != 1 || len(clear.requests) != Phase17StableClearSnapshots || route.tickCalls != 0 {
		t.Fatalf("force moves=%d clear requests=%d route ticks=%d", combat.forceMoveCalls, len(clear.requests), route.tickCalls)
	}
}

func TestSummonerRouteThreatInterleaveNeverTicksRouteOnThreat(t *testing.T) {
	base := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	progress, cfg := phase17ThreatProgress(), phase17ThreatConfig()
	cfg.NoProgressTimeout = 12 * time.Second
	definition := controllerDefinition(t)
	route := controllerRoute(progress)
	clear := &routeClearMock{}
	pipeline := &runPipeline{
		definition: definition, phase: RunPhasePlayRoute, core: pipelineCoreState{routeID: "summoner-route",
			combat: CombatConfig{Profile: "necro_bone_spear"}, routeCombat: cfg},
	}
	state := controllerState(base, world.Monster{NPCID: world.ArcaneSpecter, UnitID: 7, Position: world.Position{X: 110, Y: 100}})
	result := pipeline.onTravelTick(context.Background(), Deps{Route: route, RouteClear: clear}, pipelineStepPlayRoute, state, base, base)
	if result.failed || route.tickCalls != 0 || route.holdCalls != 1 || len(clear.requests) != 1 {
		t.Fatalf("result=%+v ticks=%d holds=%d requests=%d", result, route.tickCalls, route.holdCalls, len(clear.requests))
	}
}

func TestSummonerRoutePickitHoldsBeforePickupAndCombatHasPriority(t *testing.T) {
	base := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	progress, cfg := phase17ThreatProgress(), phase17ThreatConfig()
	cfg.NoProgressTimeout = 12 * time.Second
	target := LootTarget{
		UnitID: 77, TxtFileNo: 1, Code: "r01", PickitAction: "keep",
		Position: world.Position{X: 106, Y: 100}, AreaID: world.ArcaneSanctuary,
	}
	newPipeline := func() *runPipeline {
		return &runPipeline{
			definition: controllerDefinition(t), phase: RunPhasePlayRoute, core: pipelineCoreState{routeID: "summoner-route",
				combat: CombatConfig{Profile: "necro_bone_spear"}, routeCombat: cfg},
		}
	}

	t.Run("safe route starts pickup before movement", func(t *testing.T) {
		route := controllerRoute(progress)
		lootActions := &mockLootActions{scans: []LootScanResult{{HasTarget: true, CandidateCount: 1, NextTarget: target}}}
		result := newPipeline().onTravelTick(context.Background(), Deps{
			Route: route, RouteClear: &routeClearMock{}, Loot: lootActions, Combat: &mockCombatActions{},
		}, pipelineStepPlayRoute, controllerState(base), base, base)
		if result.failed || route.tickCalls != 0 || route.holdCalls != 1 || lootActions.scanCalls != 1 ||
			len(lootActions.startCalls) != 1 || lootActions.startCalls[0].UnitID != target.UnitID || lootActions.tickCalls != 1 {
			t.Fatalf("result=%+v route=%+v loot=%+v", result, route, lootActions)
		}
	})

	t.Run("threat suppresses every loot action", func(t *testing.T) {
		route := controllerRoute(progress)
		lootActions := &mockLootActions{scans: []LootScanResult{{HasTarget: true, NextTarget: target}}}
		state := controllerState(base, world.Monster{NPCID: world.ArcaneSpecter, UnitID: 8, Position: world.Position{X: 106, Y: 100}})
		result := newPipeline().onTravelTick(context.Background(), Deps{
			Route: route, RouteClear: &routeClearMock{}, Loot: lootActions, Combat: &mockCombatActions{},
		}, pipelineStepPlayRoute, state, base, base)
		if result.failed || lootActions.scanCalls != 0 || len(lootActions.startCalls) != 0 || lootActions.tickCalls != 0 || route.tickCalls != 0 {
			t.Fatalf("result=%+v route=%+v loot=%+v", result, route, lootActions)
		}
	})
}

func TestSummonerRoutePickitCollectsAllKeepTargetsBeforeMovement(t *testing.T) {
	base := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	progress, cfg := phase17ThreatProgress(), phase17ThreatConfig()
	cfg.NoProgressTimeout = 12 * time.Second
	route := controllerRoute(progress)
	first := LootTarget{UnitID: 71, Code: "r01", PickitAction: "keep", Position: world.Position{X: 104, Y: 100}, AreaID: world.ArcaneSanctuary}
	second := LootTarget{UnitID: 72, Code: "r02", PickitAction: "keep", Position: world.Position{X: 105, Y: 100}, AreaID: world.ArcaneSanctuary}
	lootActions := &mockLootActions{
		scans: []LootScanResult{
			{HasTarget: true, CandidateCount: 2, NextTarget: first},
			{HasTarget: true, CandidateCount: 1, NextTarget: second},
			{},
		},
		ticks: []LootPickupResult{
			{Done: true, Status: LootPickupPickedUp, Target: first},
			{Done: true, Status: LootPickupPickedUp, Target: second},
		},
	}
	pipeline := &runPipeline{
		definition: controllerDefinition(t), phase: RunPhasePlayRoute, core: pipelineCoreState{routeID: "summoner-route",
			combat: CombatConfig{Profile: "necro_bone_spear"}, routeCombat: cfg},
	}
	deps := Deps{Route: route, RouteClear: &routeClearMock{}, Loot: lootActions, Combat: &mockCombatActions{}}
	for tick := 0; tick < 3; tick++ {
		state := controllerState(base.Add(time.Duration(tick) * time.Second))
		result := pipeline.onTravelTick(context.Background(), deps, pipelineStepPlayRoute, state, state.At, base)
		if result.failed {
			t.Fatalf("tick %d failed: %+v", tick, result)
		}
	}
	if len(lootActions.startCalls) != 2 || lootActions.startCalls[0].UnitID != first.UnitID ||
		lootActions.startCalls[1].UnitID != second.UnitID || route.holdCalls != 2 || route.tickCalls != 1 {
		t.Fatalf("starts=%+v holds=%d route ticks=%d scans=%d", lootActions.startCalls, route.holdCalls, route.tickCalls, lootActions.scanCalls)
	}
}

func TestSummonerRoutePickitReusesBoundedTeleportApproach(t *testing.T) {
	base := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	progress, cfg := phase17ThreatProgress(), phase17ThreatConfig()
	cfg.NoProgressTimeout = 12 * time.Second
	route := controllerRoute(progress)
	target := LootTarget{
		UnitID: 73, Code: "r03", PickitAction: "keep",
		Position: world.Position{X: 120, Y: 100}, AreaID: world.ArcaneSanctuary,
	}
	lootActions := &mockLootActions{scans: []LootScanResult{{HasTarget: true, CandidateCount: 1, NextTarget: target}}}
	combat := &mockCombatActions{}
	pipeline := &runPipeline{
		definition: controllerDefinition(t), phase: RunPhasePlayRoute, core: pipelineCoreState{routeID: "summoner-route",
			combat: CombatConfig{Profile: "necro_bone_spear"}, routeCombat: cfg},
	}
	result := pipeline.onTravelTick(context.Background(), Deps{
		Route: route, RouteClear: &routeClearMock{}, Loot: lootActions, Combat: combat,
	}, pipelineStepPlayRoute, controllerState(base), base, base)
	if result.failed || combat.teleportCalls != 1 || combat.lastTeleportTarget != target.Position ||
		len(lootActions.startCalls) != 0 || route.holdCalls != 1 || route.tickCalls != 0 {
		t.Fatalf("result=%+v teleports=%d starts=%d holds=%d ticks=%d", result, combat.teleportCalls, len(lootActions.startCalls), route.holdCalls, route.tickCalls)
	}
}

func TestRouteManaHysteresisHoldsBelowResumeAfterEntry(t *testing.T) {
	base := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	progress, cfg := phase17ThreatProgress(), phase17ThreatConfig()
	cfg.NoProgressTimeout = 12 * time.Second
	cfg.TeleportManaReservePercent = 20
	cfg.ResumeManaPercent = 35
	cfg.EmergencyManaPercent = 10
	cfg.ManaRecoveryTimeout = 5 * time.Second
	route, clear := controllerRoute(progress), &routeClearMock{}
	var controller RouteThreatController

	state := controllerState(base)
	state.Player.Mana, state.Player.MaxMana = 25, 100
	assessment := assessThreats(state, progress, controllerDefinition(t).RouteHostileNPCIDs, cfg)
	if context := controller.ObserveResources(state, assessment, cfg, base); context.MobilityCritical {
		t.Fatalf("25%% unexpectedly started hold: %+v", context)
	}
	if result := controllerTick(t, &controller, route, clear, state, progress, cfg); !result.AllowMovement {
		t.Fatalf("25%% movement = %+v", result)
	}

	for i, mana := range []uint32{19, 34} {
		state = controllerState(base.Add(time.Duration(i+1) * time.Second))
		state.Player.Mana, state.Player.MaxMana = mana, 100
		assessment = assessThreats(state, progress, controllerDefinition(t).RouteHostileNPCIDs, cfg)
		context := controller.ObserveResources(state, assessment, cfg, state.At)
		if !context.MobilityCritical {
			t.Fatalf("%d%% did not retain hold: %+v", mana, context)
		}
		if result := controllerTick(t, &controller, route, clear, state, progress, cfg); result.AllowMovement || result.Failed {
			t.Fatalf("%d%% result = %+v", mana, result)
		}
	}

	state = controllerState(base.Add(3 * time.Second))
	state.Player.Mana, state.Player.MaxMana = 35, 100
	assessment = assessThreats(state, progress, controllerDefinition(t).RouteHostileNPCIDs, cfg)
	if context := controller.ObserveResources(state, assessment, cfg, state.At); context.MobilityCritical {
		t.Fatalf("35%% did not release hold: %+v", context)
	}
}

func TestRouteManaEmergencyContextAndFiveSecondTimeout(t *testing.T) {
	base := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	progress, cfg := phase17ThreatProgress(), phase17ThreatConfig()
	cfg.NoProgressTimeout = 12 * time.Second
	cfg.TeleportManaReservePercent = 20
	cfg.ResumeManaPercent = 35
	cfg.EmergencyManaPercent = 10
	cfg.ManaRecoveryTimeout = 5 * time.Second
	monster := world.Monster{NPCID: world.ArcaneSpecter, UnitID: 7, Position: world.Position{X: 110, Y: 100}}
	route, clear := controllerRoute(progress), &routeClearMock{}
	var controller RouteThreatController

	for _, elapsed := range []time.Duration{0, 4 * time.Second, 5 * time.Second} {
		state := controllerState(base.Add(elapsed), monster)
		state.Player.Mana, state.Player.MaxMana = 10, 100
		assessment := assessThreats(state, progress, controllerDefinition(t).RouteHostileNPCIDs, cfg)
		resourceContext := controller.ObserveResources(state, assessment, cfg, state.At)
		if !resourceContext.MobilityCritical || !resourceContext.Threatened || !resourceContext.EmergencyMana {
			t.Fatalf("context at %s = %+v", elapsed, resourceContext)
		}
		result := controller.Tick(context.Background(), route, clear, state, progress, assessment, controllerDefinition(t), cfg, "necro_bone_spear", state.At)
		if elapsed < 5*time.Second && result.Failed {
			t.Fatalf("premature failure at %s: %+v", elapsed, result)
		}
		if elapsed == 5*time.Second && (!result.Failed || result.Reason != RouteThreatReasonManaRecoveryFailed) {
			t.Fatalf("timeout result = %+v", result)
		}
	}
}

func TestSummonerRoutePendingResourceStillClearsButActionConsumesTick(t *testing.T) {
	base := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	progress, cfg := phase17ThreatProgress(), phase17ThreatConfig()
	cfg.NoProgressTimeout = 12 * time.Second
	cfg.TeleportManaReservePercent = 20
	cfg.ResumeManaPercent = 35
	cfg.EmergencyManaPercent = 10
	cfg.ManaRecoveryTimeout = 5 * time.Second
	definition := controllerDefinition(t)
	state := controllerState(base, world.Monster{NPCID: world.ArcaneSpecter, UnitID: 7, Position: world.Position{X: 110, Y: 100}})
	state.Player.Mana, state.Player.MaxMana = 50, 100

	t.Run("pending", func(t *testing.T) {
		route, clear := controllerRoute(progress), &routeClearMock{}
		resources := &mockProfileActions{resourceResults: []profile.Result{{Status: profile.StatusPending}}}
		pipeline := &runPipeline{
			definition: definition, phase: RunPhasePlayRoute, core: pipelineCoreState{routeID: "summoner-route",
				combat: CombatConfig{Profile: "necro_bone_spear"}, routeCombat: cfg},
		}
		result := pipeline.onTravelTick(context.Background(), Deps{Route: route, RouteClear: clear, Profile: resources}, pipelineStepPlayRoute, state, base, base)
		if result.failed || route.tickCalls != 0 || route.holdCalls != 1 || len(clear.requests) != 1 {
			t.Fatalf("result=%+v ticks=%d holds=%d clears=%d", result, route.tickCalls, route.holdCalls, len(clear.requests))
		}
	})

	t.Run("action", func(t *testing.T) {
		route, clear := controllerRoute(progress), &routeClearMock{}
		resources := &mockProfileActions{resourceResults: []profile.Result{{Status: profile.StatusAction, Resource: profile.ResourceMana}}}
		pipeline := &runPipeline{
			definition: definition, phase: RunPhasePlayRoute, core: pipelineCoreState{routeID: "summoner-route",
				combat: CombatConfig{Profile: "necro_bone_spear"}, routeCombat: cfg},
		}
		result := pipeline.onTravelTick(context.Background(), Deps{Route: route, RouteClear: clear, Profile: resources}, pipelineStepPlayRoute, state, base, base)
		if result.failed || route.tickCalls != 0 || route.holdCalls != 0 || len(clear.requests) != 0 {
			t.Fatalf("result=%+v ticks=%d holds=%d clears=%d", result, route.tickCalls, route.holdCalls, len(clear.requests))
		}
	})
}

func TestRunnerDelegatesOptInRoutePendingResourceExactlyOnce(t *testing.T) {
	base := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	progress, cfg := phase17ThreatProgress(), phase17ThreatConfig()
	cfg.NoProgressTimeout = 12 * time.Second
	cfg.TeleportManaReservePercent = 20
	cfg.ResumeManaPercent = 35
	cfg.EmergencyManaPercent = 10
	cfg.ManaRecoveryTimeout = 5 * time.Second
	route, clear := controllerRoute(progress), &routeClearMock{}
	resources := &mockProfileActions{resourceResults: []profile.Result{{Status: profile.StatusPending}}}
	runner := NewRunner(
		config.NewLogger("error"),
		RunSelection{Run: string(RunIDSummoner), Phase: RunPhasePlayRoute},
		RunConfig{
			StepTimeout: 30 * time.Second, RouteID: "summoner-route",
			Combat: CombatConfig{Profile: "necro_bone_spear"}, RouteCombat: cfg,
		},
		Deps{Route: route, RouteClear: clear, Profile: resources},
	)
	runner.started = true
	runner.outcome = RunOutcomeRunning
	runner.tracker.begin(pipelineStepPlayRoute, base, 30*time.Second)
	state := controllerState(base, world.Monster{NPCID: world.ArcaneSpecter, UnitID: 7, Position: world.Position{X: 110, Y: 100}})
	state.Player.Mana, state.Player.MaxMana = 50, 100

	result := runner.Tick(context.Background(), state, base)
	if result.Outcome != RunOutcomeRunning || resources.resourceCalls != 1 || len(clear.requests) != 1 || route.tickCalls != 0 {
		t.Fatalf("result=%+v resource calls=%d clears=%d route ticks=%d", result, resources.resourceCalls, len(clear.requests), route.tickCalls)
	}
	if len(resources.resourceContexts) != 1 || !resources.resourceContexts[0].Threatened {
		t.Fatalf("resource contexts = %+v", resources.resourceContexts)
	}
}

func TestSummonerRouteMissingManaResourceFailsImmediately(t *testing.T) {
	base := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	progress, cfg := phase17ThreatProgress(), phase17ThreatConfig()
	cfg.NoProgressTimeout = 12 * time.Second
	cfg.TeleportManaReservePercent = 20
	cfg.ResumeManaPercent = 35
	cfg.EmergencyManaPercent = 10
	cfg.ManaRecoveryTimeout = 5 * time.Second
	definition := controllerDefinition(t)
	state := controllerState(base, world.Monster{NPCID: world.ArcaneSpecter, UnitID: 7, Position: world.Position{X: 110, Y: 100}})
	state.Player.Mana, state.Player.MaxMana = 10, 100
	route, clear := controllerRoute(progress), &routeClearMock{}
	resources := &mockProfileActions{resourceResults: []profile.Result{{
		Status: profile.StatusComplete, Resource: profile.ResourceMana, Reason: "mana_potion_unavailable",
	}}}
	pipeline := &runPipeline{
		definition: definition, phase: RunPhasePlayRoute, core: pipelineCoreState{routeID: "summoner-route",
			combat: CombatConfig{Profile: "necro_bone_spear"}, routeCombat: cfg},
	}
	result := pipeline.onTravelTick(context.Background(), Deps{Route: route, RouteClear: clear, Profile: resources}, pipelineStepPlayRoute, state, base, base)
	if !result.failed || result.reason != string(RouteThreatReasonManaRecoveryFailed) || route.tickCalls != 0 || len(clear.requests) != 0 {
		t.Fatalf("result=%+v ticks=%d clears=%d", result, route.tickCalls, len(clear.requests))
	}
}

func TestRouteRecoveryGuardStopsSecondIneffectiveInput(t *testing.T) {
	base := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	cfg := phase17ThreatConfig()
	cfg.NoProgressTimeout = 12 * time.Second
	progress := phase17ThreatProgress()
	progress.Mode = RouteProgressRecovery
	progress.MovementTarget = progress.PreviousConfirmed
	progress.RecoveryInputSent = true
	progress.RecoveryInputAt = base
	progress.RecoveryInputOrigin = world.Position{X: 140, Y: 100}
	progress.RecoveryNextInputAt = base.Add(250 * time.Millisecond)
	progress.RecoveryOutcomeAt = base.Add(700 * time.Millisecond)
	progress.RecoveryProgressTiles = 3
	route, clear := controllerRoute(progress), &routeClearMock{}
	var controller RouteThreatController

	// Vor dem Teleport-Settle bleibt Movement erlaubt; wirkungslos ist der Cast erst danach.
	waiting := controllerState(base.Add(250 * time.Millisecond))
	waiting.Player.Position = progress.RecoveryInputOrigin
	if result := controllerTick(t, &controller, route, clear, waiting, progress, cfg); result.Failed || !result.AllowMovement || result.State != RouteThreatRecoveryGuard {
		t.Fatalf("settle wait = %+v", result)
	}

	stuck := controllerState(base.Add(700 * time.Millisecond))
	stuck.Player.Position = progress.RecoveryInputOrigin
	result := controllerTick(t, &controller, route, clear, stuck, progress, cfg)
	if !result.Failed || result.Reason != RouteThreatReasonRecoveryUnsafe || result.AllowMovement {
		t.Fatalf("stuck recovery = %+v", result)
	}
}

func TestRouteRecoveryGuardAcceptsConfirmedPositionProgress(t *testing.T) {
	base := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	cfg := phase17ThreatConfig()
	cfg.NoProgressTimeout = 12 * time.Second
	progress := phase17ThreatProgress()
	progress.Mode = RouteProgressRecovery
	progress.MovementTarget = progress.PreviousConfirmed
	progress.RecoveryInputSent = true
	progress.RecoveryInputAt = base
	progress.RecoveryInputOrigin = world.Position{X: 140, Y: 100}
	progress.RecoveryNextInputAt = base.Add(250 * time.Millisecond)
	progress.RecoveryOutcomeAt = base.Add(700 * time.Millisecond)
	progress.RecoveryProgressTiles = 3
	route, clear := controllerRoute(progress), &routeClearMock{}
	var controller RouteThreatController
	state := controllerState(base.Add(250 * time.Millisecond))
	state.Player.Position = world.Position{X: 137, Y: 100}

	result := controllerTick(t, &controller, route, clear, state, progress, cfg)
	if result.Failed || !result.AllowMovement || result.State != RouteThreatRecoveryGuard {
		t.Fatalf("progressed recovery = %+v", result)
	}
}

func TestThreatAtRecoveryTargetClearsBeforeRecoveryMovement(t *testing.T) {
	base := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	cfg := phase17ThreatConfig()
	cfg.NoProgressTimeout = 12 * time.Second
	progress := phase17ThreatProgress()
	progress.Mode = RouteProgressRecovery
	progress.MovementTarget = world.Position{X: 100, Y: 100}
	progress.PreviousConfirmed = progress.MovementTarget
	progress.LocalRecoveryAttempts = 1
	state := controllerState(base, world.Monster{
		NPCID: world.ArcaneSpecter, UnitID: 17, Position: world.Position{X: 101, Y: 100},
	})
	state.Player.Position = world.Position{X: 120, Y: 100}
	route, clear := controllerRoute(progress), &routeClearMock{}
	var controller RouteThreatController

	result := controllerTick(t, &controller, route, clear, state, progress, cfg)
	if result.Failed || result.AllowMovement || route.holdCalls != 1 || len(clear.requests) != 1 {
		t.Fatalf("result=%+v holds=%d clears=%d", result, route.holdCalls, len(clear.requests))
	}
	if clear.requests[0].Target.UnitID != 17 || progress.LocalRecoveryAttempts != 1 {
		t.Fatalf("request=%+v corrections=%d", clear.requests[0], progress.LocalRecoveryAttempts)
	}
}

func TestSummonerPipelineNeverTicksSecondIneffectiveRecoveryInput(t *testing.T) {
	base := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	cfg := phase17ThreatConfig()
	cfg.NoProgressTimeout = 12 * time.Second
	definition := controllerDefinition(t)
	progress := phase17ThreatProgress()
	progress.Mode = RouteProgressRecovery
	progress.MovementTarget = progress.PreviousConfirmed
	route, clear := controllerRoute(progress), &routeClearMock{}
	pipeline := &runPipeline{
		definition: definition, phase: RunPhasePlayRoute, core: pipelineCoreState{routeID: "summoner-route",
			combat: CombatConfig{Profile: "necro_bone_spear"}, routeCombat: cfg},
	}
	state := controllerState(base)
	state.Player.Position = world.Position{X: 140, Y: 100}
	if result := pipeline.onTravelTick(context.Background(), Deps{Route: route, RouteClear: clear}, pipelineStepPlayRoute, state, base, base); result.failed {
		t.Fatalf("first recovery = %+v", result)
	}
	if route.tickCalls != 1 {
		t.Fatalf("first route ticks = %d", route.tickCalls)
	}

	route.progress.RecoveryInputSent = true
	route.progress.RecoveryInputAt = base
	route.progress.RecoveryInputOrigin = state.Player.Position
	route.progress.RecoveryNextInputAt = base.Add(250 * time.Millisecond)
	route.progress.RecoveryOutcomeAt = base.Add(700 * time.Millisecond)
	route.progress.RecoveryProgressTiles = 3
	state.At = base.Add(700 * time.Millisecond)
	result := pipeline.onTravelTick(context.Background(), Deps{Route: route, RouteClear: clear}, pipelineStepPlayRoute, state, state.At, base)
	if !result.failed || result.reason != string(RouteThreatReasonRecoveryUnsafe) || route.tickCalls != 1 {
		t.Fatalf("second recovery=%+v route ticks=%d", result, route.tickCalls)
	}
}

func TestRouteThreatTelemetryIsTransitionAndActionBound(t *testing.T) {
	base := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	progress, cfg := phase17ThreatProgress(), phase17ThreatConfig()
	cfg.NoProgressTimeout = 12 * time.Second
	definition := controllerDefinition(t)
	monster := world.Monster{NPCID: world.ArcaneSpecter, UnitID: 7, Position: world.Position{X: 110, Y: 100}}
	route := controllerRoute(progress)
	clear := &routeClearMock{result: profile.Result{Status: profile.StatusAction, SkillID: 84}}
	trace := &pipelineTelemetry{}
	var controller RouteThreatController
	controller.SetTelemetry(trace)

	for i := 0; i < 2; i++ {
		state := controllerState(base.Add(time.Duration(i)*time.Second), monster)
		assessment := assessThreats(state, progress, definition.RouteHostileNPCIDs, cfg)
		result := controller.Tick(context.Background(), route, clear, state, progress, assessment, definition, cfg, "necro_bone_spear", state.At)
		if result.Failed {
			t.Fatalf("threat tick %d = %+v", i, result)
		}
	}
	for i := 2; i < 2+Phase17StableClearSnapshots; i++ {
		state := controllerState(base.Add(time.Duration(i) * time.Second))
		assessment := assessThreats(state, progress, definition.RouteHostileNPCIDs, cfg)
		result := controller.Tick(context.Background(), route, clear, state, progress, assessment, definition, cfg, "necro_bone_spear", state.At)
		if result.Failed {
			t.Fatalf("clear tick %d = %+v", i, result)
		}
	}

	counts := map[telemetry.EventName]int{}
	for _, event := range trace.events {
		counts[event.Event]++
	}
	if counts[telemetry.RouteThreatDetected] != 1 ||
		counts[telemetry.RouteClearStarted] != 1 ||
		counts[telemetry.RouteClearAction] != 2 ||
		counts[telemetry.RouteClearProgress] != 1 ||
		counts[telemetry.RouteClearCompleted] != 1 {
		t.Fatalf("event counts = %+v events=%+v", counts, trace.events)
	}
	completed := trace.events[len(trace.events)-1]
	if completed.Event != telemetry.RouteClearCompleted || completed.CombatActionsSent != 2 ||
		completed.TargetsSeen != 1 || completed.HoldMs != 4_000 {
		t.Fatalf("completed = %+v", completed)
	}
}

func TestRouteThreatTelemetryIdentifiesSetupRouteByRole(t *testing.T) {
	base := time.Date(2026, 8, 1, 18, 26, 34, 0, time.UTC)
	progress := phase17ThreatProgress()
	progress.RouteID = "cows-leg-acquisition"
	progress.RouteRole = pathing.RouteRoleLegAcquisition

	event := routeTelemetryEvent(telemetry.RouteThreatDetected, controllerState(base), progress, base, telemetry.Event{RouteID: "wrong-setup-id"})
	if event.RouteID != "" || event.RouteRole != string(pathing.RouteRoleLegAcquisition) {
		t.Fatalf("setup-route threat telemetry = %+v", event)
	}
}

func TestRouteThreatTelemetryPreservesExecutorActionContext(t *testing.T) {
	base := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	progress, cfg := phase17ThreatProgress(), phase17ThreatConfig()
	cfg.NoProgressTimeout = 12 * time.Second
	definition := controllerDefinition(t)
	monster := world.Monster{NPCID: world.ArcaneSpecter, UnitID: 7, Position: world.Position{X: 110, Y: 100}}
	route := controllerRoute(progress)
	clear := &routeClearMock{result: profile.Result{
		Status: profile.StatusAction, SkillID: 66, ActionKind: profile.RouteClearActionCurse,
		TargetingMode:        profile.MonsterTargetingWorldProjected,
		CowGroupAnchorUnitID: 7, CowGroupLivingCount: 5,
		CowCorpseAnchorDistanceTiles: 3, CowCorpseCoverageCount: 4,
	}}
	trace := &pipelineTelemetry{}
	var controller RouteThreatController
	controller.SetTelemetry(trace)
	state := controllerState(base, monster)
	assessment := assessThreats(state, progress, definition.RouteHostileNPCIDs, cfg)

	result := controller.Tick(context.Background(), route, clear, state, progress, assessment, definition, cfg, "necro_bone_spear", base)
	if result.Failed {
		t.Fatalf("threat tick = %+v", result)
	}
	for _, event := range trace.events {
		if event.Event == telemetry.RouteClearAction {
			if event.ActionKind != "curse" || event.SkillID != 66 || event.UnitID != monster.UnitID ||
				event.TargetingMode != string(profile.MonsterTargetingWorldProjected) ||
				event.HoverConfirmed == nil || *event.HoverConfirmed ||
				event.CowGroupAnchorUnitID != 7 || event.CowGroupLivingCount != 5 ||
				event.CowCorpseAnchorDistanceTiles != 3 || event.CowCorpseCoverageCount != 4 {
				t.Fatalf("curse action = %+v", event)
			}
			return
		}
	}
	t.Fatalf("missing route-clear action: %+v", trace.events)
}

func TestRouteSaturationAndManaTelemetryAvoidPerTickFlood(t *testing.T) {
	base := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	progress, cfg := phase17ThreatProgress(), phase17ThreatConfig()
	definition := controllerDefinition(t)
	trace := &pipelineTelemetry{}
	var controller RouteThreatController
	controller.SetTelemetry(trace)

	state := controllerState(base)
	state.MonsterCoverage = world.MonsterCoverage{
		EligibleMonsterCount: 513, MonstersTruncated: true, MonsterCoverageRadiusTiles: 40,
	}
	assessment := assessThreats(state, progress, definition.RouteHostileNPCIDs, cfg)
	if err := controller.observeSnapshotTelemetry(state, progress, assessment, definition, base); err != nil {
		t.Fatal(err)
	}
	if err := controller.observeSnapshotTelemetry(state, progress, assessment, definition, base.Add(time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	state.At = base.Add(time.Second)
	state.MonsterCoverage.MonsterCoverageRadiusTiles = 60
	assessment = assessThreats(state, progress, definition.RouteHostileNPCIDs, cfg)
	if err := controller.observeSnapshotTelemetry(state, progress, assessment, definition, state.At); err != nil {
		t.Fatal(err)
	}
	state.At = base.Add(2 * time.Second)
	state.MonsterCoverage.MonstersTruncated = false
	assessment = assessThreats(state, progress, definition.RouteHostileNPCIDs, cfg)
	if err := controller.observeSnapshotTelemetry(state, progress, assessment, definition, state.At); err != nil {
		t.Fatal(err)
	}

	resourceState := controllerState(base)
	resourceState.Player.Mana, resourceState.Player.MaxMana = 10, 100
	active := profile.ResourceContext{MobilityCritical: true}
	if err := controller.ObserveResourceResult(resourceState, progress, active, profile.Result{Resource: profile.ResourceMana}, base); err != nil {
		t.Fatal(err)
	}
	if err := controller.ObserveResourceResult(resourceState, progress, active, profile.Result{Resource: profile.ResourceMana}, base.Add(time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	active.Threatened = true
	if err := controller.ObserveResourceResult(resourceState, progress, active, profile.Result{Resource: profile.ResourceMana}, base.Add(2*time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	if err := controller.ObserveResourceResult(resourceState, progress, profile.ResourceContext{}, profile.Result{}, base.Add(3*time.Millisecond)); err != nil {
		t.Fatal(err)
	}

	counts := map[telemetry.EventName]int{}
	for _, event := range trace.events {
		counts[event.Event]++
	}
	if counts[telemetry.RouteMonsterSnapshotSaturated] != 3 || counts[telemetry.RouteManaHold] != 3 {
		t.Fatalf("counts=%+v events=%+v", counts, trace.events)
	}
}
