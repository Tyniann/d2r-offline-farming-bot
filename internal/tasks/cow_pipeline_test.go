package tasks

import (
	"context"
	"testing"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/profile"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/telemetry"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

func TestCowLegRouteDisablesOnlyItsPrivateCombatCopy(t *testing.T) {
	definition, ok := DefaultRunRegistry().Definition(RunIDCows)
	if !ok {
		t.Fatal("Cow definition missing")
	}
	cfg := RunConfig{SetupRouteID: "leg-route", RouteCombat: RouteCombatConfig{Enabled: true}}
	pipeline := newCowPipeline(definition, cfg)
	if pipeline.legRoute.core.routeID != "leg-route" {
		t.Fatalf("setup route=%q", pipeline.legRoute.core.routeID)
	}
	if pipeline.legRoute.core.routeCombat.Enabled {
		t.Fatal("Wirt-Leg route combat remained enabled")
	}
	if !pipeline.config.RouteCombat.Enabled || !cfg.RouteCombat.Enabled {
		t.Fatal("Cow-Sweep combat config was mutated while disabling Wirt-Leg combat")
	}
	if !pipeline.cowSweep.core.routeCombat.Enabled || !pipeline.cowSweep.core.requireTerminalSafe {
		t.Fatal("Cow-Sweep lost combat or terminal safe-snapshot enforcement")
	}
	if !pipeline.legRoute.core.suppressRouteLoot {
		t.Fatal("setup route may not consume inventory capacity with incidental loot")
	}
}

func TestCowSweepUsesCorpseExplosionWrapperOnlyForProfileCapability(t *testing.T) {
	definition, ok := DefaultRunRegistry().Definition(RunIDCows)
	if !ok {
		t.Fatal("Cow definition missing")
	}
	now := time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)
	route := controllerRoute(RouteProgress{RouteID: "cow-route", Mode: RouteProgressMovement})
	clear := &routeClearMock{}
	deps := Deps{Route: route, RouteClear: clear}

	hammerdin := newCowPipeline(definition, RunConfig{
		RouteID: "cow-route", Combat: CombatConfig{Profile: hammerdinCombatProfileID},
	})
	if result := tickCowSweepAfterArrivalSettle(t, hammerdin, deps, now); result.failed {
		t.Fatalf("profile-only Cow sweep failed: %+v", result)
	}

	necro := newCowPipeline(definition, RunConfig{
		RouteID: "cow-route", Combat: CombatConfig{Profile: "necro_bone_spear", UseCorpseExplosion: true},
	})
	if result := tickCowSweepAfterArrivalSettle(t, necro, deps, now); !result.failed || result.reason != CowReasonCapabilityMissing {
		t.Fatalf("CE Cow sweep accepted route clear without CE capability: %+v", result)
	}
}

func tickCowSweepAfterArrivalSettle(t *testing.T, pipeline *cowPipeline, deps Deps, now time.Time) stepResult {
	t.Helper()
	pipeline.onStepEnter(cowStepSweep)
	var result stepResult
	for index := 0; index < entryAreaArriveSnapshots; index++ {
		at := now.Add(time.Duration(index) * time.Second)
		if index == entryAreaArriveSnapshots-1 {
			at = now.Add(entryAreaArriveSettle)
		}
		state := world.State{
			At: at, Generation: uint64(index + 1), Valid: true, Phase: world.GamePhaseInGame,
			Area: world.LookupArea(world.MooMooFarm),
		}
		result = pipeline.onTick(context.Background(), deps, cowStepSweep, state, at, now, 0)
		if result.failed && index < entryAreaArriveSnapshots-1 {
			t.Fatalf("settle tick %d failed: %+v", index, result)
		}
	}
	return result
}

func TestCowRetryReturnUsesSharedStandardPipeline(t *testing.T) {
	machine, err := newRunMachine(RunSelection{Run: string(RunIDCows), Phase: RunPhaseRetryReturn}, RunConfig{})
	if err != nil {
		t.Fatal(err)
	}
	pipeline, ok := machine.(*runPipeline)
	if !ok || pipeline.phase != RunPhaseRetryReturn || pipeline.definition.ID != RunIDCows {
		t.Fatalf("Cow retry-return machine=%T pipeline=%+v", machine, pipeline)
	}
}

func TestCowRetryReturnAcceptsPrimarySweepArea(t *testing.T) {
	definition, ok := DefaultRunRegistry().Definition(RunIDCows)
	if !ok {
		t.Fatal("Cow definition missing")
	}
	pipeline := &runPipeline{definition: definition, phase: RunPhaseRetryReturn}
	now := time.Date(2026, 8, 9, 0, 29, 38, 0, time.UTC)
	result := pipeline.onRetryReturnTick(context.Background(), Deps{}, pipelineStepPrecheck, world.State{
		Valid: true, Phase: world.GamePhaseInGame, Area: world.LookupArea(world.MooMooFarm),
	}, now, now)
	if !result.complete || result.failed {
		t.Fatalf("Cow retry-return precheck=%+v", result)
	}
}

func TestCowReturnTimeoutsRequireOneStableTerminalReason(t *testing.T) {
	pipeline := &cowPipeline{}
	for _, step := range []string{cowStepCastReturnTP, cowStepEnterReturnTP, cowStepWaitRogue} {
		if got := pipeline.timeoutReason(step); got != "cow_return_portal_failed" {
			t.Fatalf("step %s reason=%q", step, got)
		}
	}
}

func TestCowPipelineReturnsThroughSharedTownHandoffAfterSweep(t *testing.T) {
	definition, ok := DefaultRunRegistry().Definition(RunIDCows)
	if !ok {
		t.Fatal("Cow definition missing")
	}
	pipeline := newCowPipeline(definition, RunConfig{})
	want := []string{
		cowStepPreflight, cowStepTownReady, cowStepAcquireWaypoint, cowStepOpenWaypoint, cowStepSelectStony,
		cowStepWaitStony, cowStepPlayLegRoute, cowStepOpenWirt, cowStepPickupLeg,
		cowStepCastReturnTP, cowStepEnterReturnTP, cowStepWaitRogue, cowStepBuyTome,
		cowStepSetupComplete,
		cowStepPortalRecipe, cowStepRecipeComplete, cowStepSweep, cowStepSweepComplete,
		pipelineStepCastTownPortal, pipelineStepEnterTownPortal, pipelineStepWaitOriginTown,
		pipelineStepOpenStash, pipelineStepStashItems, pipelineStepCloseStash,
		pipelineStepPrepareTown, pipelineStepComplete,
	}
	step := pipeline.firstStep()
	for i, expected := range want {
		if step != expected {
			t.Fatalf("step %d=%q, want %q", i, step, expected)
		}
		step = pipeline.nextStep(step)
	}
	if step != "" {
		t.Fatalf("Cow pipeline continued beyond Town handoff to %q", step)
	}
}

func TestCowPipelineStepsHaveStableHistoryStages(t *testing.T) {
	tests := map[telemetry.HistoryStage][]string{
		telemetry.HistoryStageTravel: {
			cowStepPreflight, cowStepTownReady, cowStepAcquireWaypoint, cowStepOpenWaypoint, cowStepSelectStony,
			cowStepWaitStony, cowStepPlayLegRoute, cowStepOpenWirt, cowStepPortalRecipe, cowStepRecipeComplete,
		},
		telemetry.HistoryStageLoot: {cowStepPickupLeg},
		telemetry.HistoryStageReturnTown: {
			cowStepCastReturnTP, cowStepEnterReturnTP, cowStepWaitRogue, cowStepBuyTome,
			cowStepSafeFailure, cowStepSetupComplete,
			pipelineStepCastTownPortal, pipelineStepEnterTownPortal, pipelineStepWaitOriginTown,
			pipelineStepOpenStash, pipelineStepStashItems, pipelineStepCloseStash,
			pipelineStepPrepareTown, pipelineStepComplete,
		},
		telemetry.HistoryStageCombat: {cowStepSweep, cowStepSweepComplete},
	}
	seen := make(map[string]telemetry.HistoryStage)
	for wantStage, steps := range tests {
		for _, step := range steps {
			stage, ok := RunStageForStep(step)
			if !ok || stage != wantStage {
				t.Fatalf("step %q stage=%q ok=%t, want %q", step, stage, ok, wantStage)
			}
			if previous, duplicate := seen[step]; duplicate {
				t.Fatalf("step %q mapped twice to %q and %q", step, previous, stage)
			}
			seen[step] = stage
		}
	}
	if len(seen) != 27 {
		t.Fatalf("mapped Cow lifecycle steps=%d, want 27", len(seen))
	}
}

func TestCowFinalTownHandoffDelegatesToSharedActions(t *testing.T) {
	definition, ok := DefaultRunRegistry().Definition(RunIDCows)
	if !ok {
		t.Fatal("Cow definition missing")
	}
	pipeline := newCowPipeline(definition, RunConfig{})
	now := time.Now()
	ctx := context.Background()
	actions := &mockRunActions{}
	portal := &mockTownPortalActions{}
	stash := &mockPersonalStashActions{}
	loot := &mockLootActions{}
	town := &mockTownPreparationActions{}
	deps := Deps{Actions: actions, Portal: portal, Stash: stash, Loot: loot, Town: town}

	steps := []struct {
		step  string
		state world.State
	}{
		{pipelineStepCastTownPortal, areaState(world.MooMooFarm)},
		{pipelineStepEnterTownPortal, areaState(world.MooMooFarm)},
		{pipelineStepWaitOriginTown, townState()},
		{pipelineStepOpenStash, townState()},
		{pipelineStepStashItems, townState()},
		{pipelineStepCloseStash, townState()},
		{pipelineStepPrepareTown, townState()},
		{pipelineStepComplete, townState()},
	}
	for _, test := range steps {
		result := pipeline.onTick(ctx, deps, test.step, test.state, now, now, 0)
		if !result.complete || result.failed {
			t.Fatalf("step %s result=%+v", test.step, result)
		}
	}
	if actions.portalCalls != 1 || portal.calls != 1 || stash.calls != 1 || town.calls != 1 {
		t.Fatalf("shared calls portal-cast=%d portal-entry=%d stash=%d town=%d", actions.portalCalls, portal.calls, stash.calls, town.calls)
	}
}

func TestCowTownReadyStepExecutesExistingProfileHook(t *testing.T) {
	actions := &cowProfileCounter{result: profile.Result{Status: profile.StatusComplete}}
	pipeline := &cowPipeline{}
	now := time.Now()
	result := pipeline.onTick(context.Background(), Deps{Profile: actions}, cowStepTownReady, world.State{Valid: true, Phase: world.GamePhaseInGame}, now, now, 0)
	if !result.complete || result.failed || actions.calls != 1 {
		t.Fatalf("town-ready result=%+v calls=%d", result, actions.calls)
	}
}

func TestCowWirtFailureReturnsToTownBeforeTerminalStop(t *testing.T) {
	pipeline := &cowPipeline{pendingFailure: "cow_leg_spawn_failed"}
	if got := pipeline.nextStep(cowStepOpenWirt); got != cowStepCastReturnTP {
		t.Fatalf("Wirt failure next=%q", got)
	}
	if got := pipeline.nextStep(cowStepWaitRogue); got != cowStepSafeFailure {
		t.Fatalf("Town-confirmed failure next=%q", got)
	}
}

func TestCowLegPickupFinishesActiveExecutorAfterInventoryConfirmation(t *testing.T) {
	pipeline := &cowPipeline{legUnitID: 42, pickupStarted: true}
	loot := &mockLootActions{ticks: []LootPickupResult{{Status: LootPickupPickedUp, Done: true}}}
	state := world.State{Items: []world.Item{{
		UnitID: 42, Code: "leg", Location: world.ItemLocationInventory, PlayerOwned: true, Page: 0,
	}}}

	result := pipeline.tickLegPickup(Deps{Loot: loot}, state, time.Now())
	if !result.complete || result.failed || !pipeline.pickupFinished || pipeline.pendingFailure != "" {
		t.Fatalf("result=%+v pickupFinished=%t", result, pipeline.pickupFinished)
	}
	if loot.tickCalls != 1 {
		t.Fatalf("TickPickup calls=%d, want 1", loot.tickCalls)
	}
}

func TestCowWaitStonyFieldSettlesBeforeLegRoute(t *testing.T) {
	definition, ok := DefaultRunRegistry().Definition(RunIDCows)
	if !ok {
		t.Fatal("Cow definition missing")
	}
	pipeline := newCowPipeline(definition, RunConfig{})
	pipeline.onStepEnter(cowStepWaitStony)
	now := time.Date(2026, 8, 18, 21, 0, 0, 0, time.UTC)
	arrived := world.State{Valid: true, Phase: world.GamePhaseInGame, Area: world.LookupArea(world.StonyField), At: now}

	loading := arrived
	loading.Phase = world.GamePhaseLoading
	if result := pipeline.onTick(context.Background(), Deps{}, cowStepWaitStony, loading, now, now, 0); result.complete || result.failed {
		t.Fatalf("loading Stony Field result=%+v, want pending", result)
	}

	open := arrived
	open.UI.WaypointOpen = true
	if result := pipeline.onTick(context.Background(), Deps{}, cowStepWaitStony, open, now, now, 0); result.complete || result.failed {
		t.Fatalf("first Stony Field snapshot result=%+v, want pending", result)
	}

	early := open
	early.At = now.Add(time.Second)
	if result := pipeline.onTick(context.Background(), Deps{}, cowStepWaitStony, early, now.Add(time.Second), now, 0); result.complete || result.failed {
		t.Fatalf("unsettled Stony Field result=%+v, want pending", result)
	}

	settled := open
	settled.At = now.Add(entryAreaArriveSettle)
	if result := pipeline.onTick(context.Background(), Deps{}, cowStepWaitStony, settled, now.Add(entryAreaArriveSettle), now, 0); !result.complete || result.failed {
		t.Fatalf("settled Stony Field result=%+v, want complete", result)
	}
}

func TestCowSweepSettlesMooMooArrivalBeforePlayback(t *testing.T) {
	definition, ok := DefaultRunRegistry().Definition(RunIDCows)
	if !ok {
		t.Fatal("Cow definition missing")
	}
	pipeline := newCowPipeline(definition, RunConfig{RouteID: "cow-route"})
	pipeline.onStepEnter(cowStepSweep)
	now := time.Date(2026, 8, 18, 21, 5, 0, 0, time.UTC)
	state := world.State{
		At: now, Valid: true, Phase: world.GamePhaseInGame, Area: world.LookupArea(world.MooMooFarm),
	}
	if result := pipeline.onTick(context.Background(), Deps{}, cowStepSweep, state, now, now, 0); result.complete || result.failed {
		t.Fatalf("first Moo Moo snapshot result=%+v, want pending settle", result)
	}
}
