package tasks

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/profile"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/telemetry"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/town"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

type pipelineTelemetry struct {
	events []telemetry.Event
	failAt int
}

func (p *pipelineTelemetry) Emit(event telemetry.Event) error {
	if p.failAt > 0 && len(p.events)+1 == p.failAt {
		return errors.New("disk unavailable")
	}
	p.events = append(p.events, event)
	return nil
}

func TestAbortOpenStepEmitsFailedAndIsIdempotent(t *testing.T) {
	trace := &pipelineTelemetry{}
	runner := NewRunner(config.NewLogger("error"), RunSelection{Run: string(RunIDCountess), Phase: RunPhaseLootAndReturn}, killRunConfig(), Deps{Telemetry: trace})
	runner.started = true
	runner.outcome = RunOutcomeRunning
	runner.tracker.begin(pipelineStepEnterTownPortal, time.Now(), time.Second)

	if err := runner.AbortOpenStep("emergency_stop_requested"); err != nil {
		t.Fatal(err)
	}
	if !runner.Terminal() || runner.Result().Reason != "emergency_stop_requested" {
		t.Fatalf("runner after abort = %+v", runner.Result())
	}
	if len(trace.events) != 1 || trace.events[0].Event != telemetry.RunStepFailed || trace.events[0].Step != pipelineStepEnterTownPortal {
		t.Fatalf("abort events = %+v", trace.events)
	}
	if err := runner.AbortOpenStep("emergency_stop_requested"); err != nil {
		t.Fatal(err)
	}
	if len(trace.events) != 1 {
		t.Fatalf("idempotent abort emitted extra events: %+v", trace.events)
	}
	for _, event := range trace.events {
		if event.Event == telemetry.RunAborted || event.Event == telemetry.RunFailed || event.Event == telemetry.RunCompleted {
			t.Fatalf("AbortOpenStep must not emit run terminals: %+v", event)
		}
	}
}

func TestRunPipelineTelemetryCarriesDefinitionStepOutcomeAndActionIndex(t *testing.T) {
	trace := &pipelineTelemetry{}
	definition, _ := DefaultRunRegistry().Definition(RunIDCountess)
	pipeline := &runPipeline{definition: definition, encounterActionIndex: 0}
	runner := NewRunner(config.NewLogger("error"), RunSelection{Run: string(RunIDCountess)}, RunConfig{}, Deps{Telemetry: trace})
	runner.run = pipeline

	if err := runner.emitStep(telemetry.RunStepStarted, pipelineStepEngageBoss, RunOutcomeRunning, ""); err != nil {
		t.Fatal(err)
	}
	if len(trace.events) != 1 {
		t.Fatalf("events = %d, want 1", len(trace.events))
	}
	event := trace.events[0]
	if event.DefinitionID != "countess" || event.Step != pipelineStepEngageBoss || event.Stage != telemetry.HistoryStageCombat || event.Outcome != string(RunOutcomeRunning) || event.ActionIndex == nil || *event.ActionIndex != 0 {
		t.Fatalf("event = %+v", event)
	}
}

func TestPhase14AllPipelineStepsHaveExactlyOneStableStage(t *testing.T) {
	tests := map[telemetry.HistoryStage][]string{
		telemetry.HistoryStageTravel:     {pipelineStepPrecheck, pipelineStepApplyTownProfile, pipelineStepAcquireTownWaypoint, pipelineStepOpenWaypoint, pipelineStepSelectRunWaypoint, pipelineStepWaitEntryArea, pipelineStepPlayRoute},
		telemetry.HistoryStageCombat:     {pipelineStepAcquireBoss, pipelineStepEngageBoss, pipelineStepClearNearbyHostiles},
		telemetry.HistoryStageLoot:       {pipelineStepRepositionForLoot, pipelineStepWaitForDrops, pipelineStepScanLoot, pipelineStepPickLoot},
		telemetry.HistoryStageReturnTown: {pipelineStepCastTownPortal, pipelineStepEnterTownPortal, pipelineStepWaitOriginTown, pipelineStepPlayTownEgress, pipelineStepOpenOriginWaypoint, pipelineStepSelectHubWaypoint, pipelineStepWaitHubArea, pipelineStepOpenStash, pipelineStepStashItems, pipelineStepCloseStash, pipelineStepPrepareTown, pipelineStepComplete},
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
	if len(seen) != 26 {
		t.Fatalf("mapped steps=%d, want 26", len(seen))
	}
	if _, ok := RunStageForStep("unknown"); ok {
		t.Fatal("unknown step received a stage")
	}
}

func TestRunPipelineTelemetryFailureStopsBeforeFollowingInput(t *testing.T) {
	trace := &pipelineTelemetry{failAt: 2}
	townWalk := &mockTownWalker{}
	runner := NewRunner(config.NewLogger("error"), RunSelection{Run: string(RunIDCountess)}, RunConfig{StepTimeout: time.Second}, Deps{Telemetry: trace, TownWalk: townWalk})

	result := runner.Tick(context.Background(), healthy(townState()), time.Now())
	if result.Outcome != RunOutcomeFailed || result.Reason != "telemetry_failed" {
		t.Fatalf("result = %+v", result)
	}
	if townWalk.calls != 0 {
		t.Fatalf("town-walk calls = %d, want no input after failed transition telemetry", townWalk.calls)
	}
}

func TestRunPipelineCentralResetBarrierClearsGenerationOnce(t *testing.T) {
	definition, _ := DefaultRunRegistry().Definition(RunIDCountess)
	pipeline := &runPipeline{
		definition: definition, targetSeen: true, targetUnitID: 42, routeStarted: true,
		lootPickupActive: true, encounterActionIndex: 1,
		postKillTeleportAttempts: 2, lootApproachTargetSet: true,
		lootApproachTarget: LootTarget{UnitID: 99}, lootApproachAttempts: 2,
		lootPickupRecovered: map[uint32]bool{7: true}, lootRecoveryPending: true,
		lootRecoveryTarget: LootTarget{UnitID: 7}, lootRecoveryTeleportSent: true,
		portalRecovered: map[uint32]bool{59: true}, portalRecoveryPending: true,
		portalRecoveryUnitID: 59, portalRecoveryTeleportSent: true,
	}
	profileActions := &mockProfileActions{}
	lootActions := &mockLootActions{}
	runner := NewRunner(config.NewLogger("error"), RunSelection{Run: string(RunIDCountess)}, RunConfig{}, Deps{Profile: profileActions, Loot: lootActions})
	runner.run = pipeline

	runner.Reset("process_lost")
	runner.Reset("duplicate")
	if pipeline.targetSeen || pipeline.targetUnitID != 0 || pipeline.routeStarted || pipeline.lootPickupActive || pipeline.encounterActionIndex != 0 ||
		pipeline.postKillTeleportAttempts != 0 || pipeline.lootApproachTargetSet || pipeline.lootApproachTarget.UnitID != 0 || pipeline.lootApproachAttempts != 0 ||
		pipeline.lootPickupRecovered != nil || pipeline.lootRecoveryPending || pipeline.lootRecoveryTarget.UnitID != 0 || pipeline.lootRecoveryTeleportSent ||
		pipeline.portalRecovered != nil || pipeline.portalRecoveryPending || pipeline.portalRecoveryUnitID != 0 || pipeline.portalRecoveryTeleportSent {
		t.Fatalf("pipeline state crossed reset barrier: %+v", pipeline)
	}
	if profileActions.resetCalls != 1 || lootActions.resetCalls != 1 {
		t.Fatalf("reset calls: profile=%d loot=%d, want exactly one", profileActions.resetCalls, lootActions.resetCalls)
	}
}

func TestRunPipelineEncounterActionTelemetryUsesStableIndex(t *testing.T) {
	definition, _ := DefaultRunRegistry().Definition(RunIDCountess)
	trace := &pipelineTelemetry{}
	profiles := &mockProfileActions{hookResults: []profile.Result{{Status: profile.StatusComplete}}}
	combat := &mockCombatActions{}
	target := countessMonster(73, world.Position{X: 101, Y: 100})
	pipeline := &runPipeline{
		definition: definition, combat: killRunConfig().Combat, targetSeen: true, targetUnitID: target.UnitID,
	}

	result := pipeline.onBossTick(context.Background(), Deps{Telemetry: trace, Profile: profiles, Combat: combat}, pipelineStepEngageBoss, healthy(cellar5State(target)), time.Now())
	if result.failed || len(trace.events) != 2 || trace.events[0].Event != telemetry.RunEncounterActionStarted || trace.events[1].Event != telemetry.RunEncounterActionCompleted {
		t.Fatalf("result=%+v events=%+v", result, trace.events)
	}
	if trace.events[0].ActionIndex == nil || *trace.events[0].ActionIndex != 0 || trace.events[1].ActionIndex == nil || *trace.events[1].ActionIndex != 0 {
		t.Fatalf("action indexes = %+v, %+v", trace.events[0].ActionIndex, trace.events[1].ActionIndex)
	}
}

func TestRunPipelineEncounterTelemetryFailureBlocksProfileAndCombatInput(t *testing.T) {
	definition, _ := DefaultRunRegistry().Definition(RunIDCountess)
	trace := &pipelineTelemetry{failAt: 1}
	profiles := &mockProfileActions{}
	combat := &mockCombatActions{}
	target := countessMonster(74, world.Position{X: 101, Y: 100})
	pipeline := &runPipeline{
		definition: definition, combat: killRunConfig().Combat, targetSeen: true, targetUnitID: target.UnitID,
	}

	result := pipeline.onBossTick(context.Background(), Deps{Telemetry: trace, Profile: profiles, Combat: combat}, pipelineStepEngageBoss, healthy(cellar5State(target)), time.Now())
	if !result.failed || result.reason != "telemetry_failed" || profiles.hookCalls != 0 || combat.castCalls != 0 {
		t.Fatalf("result=%+v hookCalls=%d combatCalls=%d", result, profiles.hookCalls, combat.castCalls)
	}
}

func TestRunPipelineEmptyEngageSequenceStartsRegularCombatWithoutProfileHook(t *testing.T) {
	tests := []struct {
		runID RunID
		area  world.AreaID
		npcID uint32
	}{
		{runID: RunIDSummoner, area: world.ArcaneSanctuary, npcID: world.Summoner},
		{runID: RunIDNihlathak, area: world.HallsOfVaught, npcID: world.Nihlathak},
	}
	for _, test := range tests {
		t.Run(string(test.runID), func(t *testing.T) {
			definition, _ := DefaultRunRegistry().Definition(test.runID)
			target := world.Monster{NPCID: test.npcID, UnitID: 75, Position: world.Position{X: 104, Y: 100}}
			state := healthy(areaState(test.area))
			state.Player.Position = world.Position{X: 100, Y: 100}
			state.Monsters = []world.Monster{target}
			profiles := &mockProfileActions{}
			combat := &mockCombatActions{}
			pipeline := &runPipeline{
				definition:   definition,
				combat:       killRunConfig().Combat,
				targetSeen:   true,
				targetUnitID: target.UnitID,
			}

			result := pipeline.onBossTick(context.Background(), Deps{Profile: profiles, Combat: combat}, pipelineStepEngageBoss, state, time.Now())
			if result.failed || result.complete || profiles.hookCalls != 0 || combat.castCalls != 1 {
				t.Fatalf("result=%+v hooks=%d combat=%d", result, profiles.hookCalls, combat.castCalls)
			}
			if combat.lastSkillID != killRunConfig().Combat.AttackSkillID {
				t.Fatalf("attack skill=%d, want %d", combat.lastSkillID, killRunConfig().Combat.AttackSkillID)
			}
		})
	}
}

func TestPostBossCleanupUsesStandardAttackForCountessAndSummoner(t *testing.T) {
	tests := []struct {
		runID     RunID
		area      world.AreaID
		hostileID uint32
	}{
		{runID: RunIDCountess, area: world.TowerCellarLevel5, hostileID: 55},
		{runID: RunIDSummoner, area: world.ArcaneSanctuary, hostileID: 131},
	}
	for _, test := range tests {
		t.Run(string(test.runID), func(t *testing.T) {
			definition, _ := DefaultRunRegistry().Definition(test.runID)
			pipeline := &runPipeline{definition: definition, combat: killRunConfig().Combat, targetUnitID: 99}
			state := healthy(areaState(test.area))
			state.Player.Position = world.Position{X: 100, Y: 100}
			state.Monsters = []world.Monster{
				{NPCID: test.hostileID, UnitID: 10, Position: world.Position{X: 110, Y: 100}},
				{NPCID: test.hostileID, UnitID: 11, Position: world.Position{X: 105, Y: 100}},
			}
			combat := &mockCombatActions{}

			result := pipeline.onBossTick(context.Background(), Deps{Combat: combat}, pipelineStepClearNearbyHostiles, state, time.Now())
			if result.failed || result.complete || combat.castCalls != 1 || pipeline.cleanupCastCount != 1 || pipeline.cleanupTargetUnitID != 11 {
				t.Fatalf("result=%+v casts=%d cleanup=%+v", result, combat.castCalls, pipeline)
			}
			if combat.lastSkillID != killRunConfig().Combat.AttackSkillID {
				t.Fatalf("cleanup skill=%d, want standard attack %d", combat.lastSkillID, killRunConfig().Combat.AttackSkillID)
			}

			// Selection is recomputed from the current living snapshot. A
			// previously selected unit must not remain pinned when another
			// living hostile becomes nearer.
			state.Monsters[0].Position = world.Position{X: 101, Y: 100}
			state.Monsters[1].Position = world.Position{X: 115, Y: 100}
			result = pipeline.onBossTick(context.Background(), Deps{Combat: combat}, pipelineStepClearNearbyHostiles, state, time.Now())
			if result.failed || combat.lastMonsterUnitID != 10 || pipeline.cleanupTargetUnitID != 10 {
				t.Fatalf("nearest target was not refreshed: result=%+v combat_target=%d cleanup_target=%d", result, combat.lastMonsterUnitID, pipeline.cleanupTargetUnitID)
			}
		})
	}
}

func TestPostBossCleanupCompletesWhenStableClearOrBudgetExhausted(t *testing.T) {
	definition, _ := DefaultRunRegistry().Definition(RunIDCountess)
	state := healthy(areaState(world.TowerCellarLevel5))
	state.Player.Position = world.Position{X: 100, Y: 100}
	combat := &mockCombatActions{}
	pipeline := &runPipeline{definition: definition, combat: killRunConfig().Combat}

	for tick := 1; tick <= postBossCleanupStableTicks; tick++ {
		result := pipeline.onBossTick(context.Background(), Deps{Combat: combat}, pipelineStepClearNearbyHostiles, state, time.Now())
		if result.failed || result.complete != (tick == postBossCleanupStableTicks) {
			t.Fatalf("clear tick %d result=%+v", tick, result)
		}
	}

	pipeline.cleanupCastCount = postBossCleanupMaxCasts
	state.Monsters = []world.Monster{{NPCID: 55, UnitID: 10, Position: world.Position{X: 101, Y: 100}}}
	result := pipeline.onBossTick(context.Background(), Deps{Combat: combat}, pipelineStepClearNearbyHostiles, state, time.Now())
	if result.failed || !result.complete || combat.castCalls != 0 {
		t.Fatalf("budget result=%+v casts=%d, want best-effort completion", result, combat.castCalls)
	}
}

func TestRunPipelineWaitEntryAreaIgnoresTransientAreaZero(t *testing.T) {
	definition, _ := DefaultRunRegistry().Definition(RunIDMephisto)
	pipeline := &runPipeline{definition: definition, phase: RunPhasePlayRoute}
	loading := world.State{Valid: true, Phase: world.GamePhaseInGame, Area: world.LookupArea(world.None)}
	result := pipeline.onTravelTick(context.Background(), Deps{}, pipelineStepWaitEntryArea, loading, time.Now(), time.Now())
	if result.complete || result.failed {
		t.Fatalf("transient Area 0 result = %+v, want pending", result)
	}

	wrong := loading
	wrong.Area = world.LookupArea(world.BlackMarsh)
	result = pipeline.onTravelTick(context.Background(), Deps{}, pipelineStepWaitEntryArea, wrong, time.Now(), time.Now())
	if !result.failed || result.reason != string(RunReasonUnexpectedArea) {
		t.Fatalf("confirmed wrong area result = %+v", result)
	}
}

func TestRetryReturnPhaseUsesOnlyPortalAndTownNormalization(t *testing.T) {
	t.Parallel()

	pipeline := &runPipeline{
		definition: RunDefinition{
			RouteTerminalArea: world.ArcaneSanctuary,
			ReturnOrigin:      town.OriginAct2,
		},
		phase: RunPhaseRetryReturn,
	}
	want := []string{
		pipelineStepPrecheck,
		pipelineStepCastTownPortal,
		pipelineStepEnterTownPortal,
		pipelineStepWaitOriginTown,
		pipelineStepPlayTownEgress,
		pipelineStepOpenOriginWaypoint,
		pipelineStepSelectHubWaypoint,
		pipelineStepWaitHubArea,
		pipelineStepComplete,
	}
	var got []string
	for step := pipelineStepPrecheck; step != ""; step = pipeline.nextStep(step) {
		got = append(got, step)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("retry-return steps = %v, want %v", got, want)
	}
}

func TestRetryReturnPhaseCompletesDirectlyFromActOneTown(t *testing.T) {
	t.Parallel()

	pipeline := &runPipeline{
		definition: RunDefinition{ReturnOrigin: town.OriginAct1},
		phase:      RunPhaseRetryReturn,
	}
	if got := pipeline.nextStep(pipelineStepWaitOriginTown); got != pipelineStepComplete {
		t.Fatalf("next step = %q, want %q", got, pipelineStepComplete)
	}
}

func TestRetryReturnPrecheckRequiresRouteTerminalArea(t *testing.T) {
	t.Parallel()

	pipeline := &runPipeline{
		definition: RunDefinition{
			RouteTerminalArea: world.TowerCellarLevel5,
			Recording: RecordingContract{AllowedRouteAreas: []world.AreaID{
				world.BlackMarsh,
				world.ForgottenTower,
				world.TowerCellarLevel5,
			}},
		},
		phase: RunPhaseRetryReturn,
	}
	now := time.Now()
	valid := pipeline.onRetryReturnTick(context.Background(), Deps{}, pipelineStepPrecheck, world.State{
		Valid: true,
		Phase: world.GamePhaseInGame,
		Area:  world.LookupArea(world.ForgottenTower),
	}, now, now)
	if !valid.complete || valid.failed {
		t.Fatalf("valid retry precheck = %+v", valid)
	}
	enter := pipeline.tickEnterTownPortal(context.Background(), Deps{Portal: &mockTownPortalActions{}}, world.State{
		Valid: true,
		Phase: world.GamePhaseInGame,
		Area:  world.LookupArea(world.ForgottenTower),
	}, now)
	if !enter.complete || enter.failed {
		t.Fatalf("route-area portal entry = %+v", enter)
	}

	wrongArea := pipeline.onRetryReturnTick(context.Background(), Deps{}, pipelineStepPrecheck, world.State{
		Valid: true,
		Phase: world.GamePhaseInGame,
		Area:  world.LookupArea(world.ArcaneSanctuary),
	}, now, now)
	if !wrongArea.failed || wrongArea.reason != string(RunReasonUnexpectedArea) {
		t.Fatalf("wrong-area retry precheck = %+v", wrongArea)
	}
}

func nihlathakBossState(player world.Position, boss world.Monster) world.State {
	state := healthy(areaState(world.HallsOfVaught))
	state.At = time.Unix(1, 0).UTC()
	state.Player.Position = player
	state.Monsters = []world.Monster{boss}
	return state
}

func TestNihlathakEngageUsesOneApproachThenRetryableProjectionLoss(t *testing.T) {
	definition, _ := DefaultRunRegistry().Definition(RunIDNihlathak)
	boss := world.Monster{NPCID: world.Nihlathak, UnitID: 42, Position: world.Position{X: 200, Y: 200}}
	aimFalse := false
	combat := &mockCombatActions{aimProjectable: &aimFalse, farthestDistance: 18, farthestOK: boolPtr(true)}
	pipeline := &runPipeline{
		definition:   definition,
		combat:       killRunConfig().Combat,
		targetSeen:   true,
		targetUnitID: boss.UnitID,
	}
	now := time.Unix(10, 0).UTC()
	state := nihlathakBossState(world.Position{X: 100, Y: 100}, boss)

	first := pipeline.onBossTick(context.Background(), Deps{Combat: combat}, pipelineStepEngageBoss, state, now)
	if first.failed || first.complete || combat.teleportCalls != 1 || !pipeline.bossApproachAttempted {
		t.Fatalf("first approach = %+v teleports=%d attempted=%t", first, combat.teleportCalls, pipeline.bossApproachAttempted)
	}

	settled := state
	settled.At = state.At.Add(time.Second)
	lost := pipeline.onBossTick(context.Background(), Deps{Combat: combat}, pipelineStepEngageBoss, settled, now.Add(bossApproachSettle+time.Millisecond))
	if !lost.failed || lost.reason != "boss_combat_unprojectable" || combat.teleportCalls != 1 || pipeline.bossApproachPending {
		t.Fatalf("projection loss = %+v teleports=%d pending=%t, want boss_combat_unprojectable without second approach", lost, combat.teleportCalls, pipeline.bossApproachPending)
	}
}

func TestNihlathakEngageUnprojectableCastAfterApproachIsRetryable(t *testing.T) {
	definition, _ := DefaultRunRegistry().Definition(RunIDNihlathak)
	boss := world.Monster{NPCID: world.Nihlathak, UnitID: 42, Position: world.Position{X: 120, Y: 100}, IsHovered: true}
	combat := &mockCombatActions{castMonsterErr: profile.ErrRouteClearTargetUnprojectable}
	pipeline := &runPipeline{
		definition:           definition,
		combat:               killRunConfig().Combat,
		targetSeen:           true,
		targetUnitID:         boss.UnitID,
		bossApproachAttempted: true,
	}
	result := pipeline.onBossTick(context.Background(), Deps{Combat: combat}, pipelineStepEngageBoss, nihlathakBossState(world.Position{X: 100, Y: 100}, boss), time.Unix(10, 0).UTC())
	if !result.failed || result.reason != "boss_combat_unprojectable" {
		t.Fatalf("result=%+v, want boss_combat_unprojectable", result)
	}
}

func boolPtr(value bool) *bool { return &value }
