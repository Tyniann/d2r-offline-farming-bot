package tasks

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/memory"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/profile"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/telemetry"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

func mephistoState(monsters ...world.Monster) world.State {
	state := areaState(world.DuranceOfHateLevel3)
	state.Player.Position = world.Position{X: 17570, Y: 8069}
	state.Monsters = monsters
	return healthy(state)
}

func mephistoMonster(unitID uint32) world.Monster {
	return world.Monster{
		NPCID:           242,
		UnitID:          unitID,
		Position:        world.Position{X: 17565, Y: 8065},
		MonsterTypeFlag: world.SuperUniqueMonsterFlag,
	}
}

func TestMephistoExecutesTwoIndexedBossActionsBeforeCombat(t *testing.T) {
	definition, _ := DefaultRunRegistry().Definition(RunIDMephisto)
	target := mephistoMonster(4242)
	trace := &pipelineTelemetry{}
	profiles := &mockProfileActions{hookResults: []profile.Result{
		{Status: profile.StatusComplete, Hook: profile.HookBossEngage},
		{Status: profile.StatusComplete, Hook: profile.HookBossEngage},
	}}
	combat := &mockCombatActions{}
	pipeline := &runPipeline{
		definition: definition,
		boss: pipelineBossState{targetSeen: true,
			targetUnitID: target.UnitID}, core: pipelineCoreState{combat: killRunConfig().Combat},
	}
	deps := Deps{Telemetry: trace, Profile: profiles, Combat: combat}

	first := pipeline.onBossTick(context.Background(), deps, pipelineStepEngageBoss, mephistoState(target), time.Now())
	if first.failed || combat.castCalls != 0 || pipeline.boss.encounterActionIndex != 1 {
		t.Fatalf("first action=%+v index=%d combat=%d", first, pipeline.boss.encounterActionIndex, combat.castCalls)
	}
	second := pipeline.onBossTick(context.Background(), deps, pipelineStepEngageBoss, mephistoState(target), time.Now())
	if second.failed || profiles.hookCalls != 2 || combat.castCalls != 1 || pipeline.boss.encounterActionIndex != 2 {
		t.Fatalf("second action=%+v hooks=%d index=%d combat=%d", second, profiles.hookCalls, pipeline.boss.encounterActionIndex, combat.castCalls)
	}
	for i, pinned := range profiles.targets {
		if pinned.UnitID != target.UnitID {
			t.Fatalf("action %d target UnitID=%d, want %d", i, pinned.UnitID, target.UnitID)
		}
		if pinned.ActionIndex != i {
			t.Fatalf("action %d target index=%d", i, pinned.ActionIndex)
		}
	}
	if len(trace.events) != 4 {
		t.Fatalf("encounter events=%d, want 4", len(trace.events))
	}
	wantIndexes := []int{0, 0, 1, 1}
	for i, event := range trace.events {
		if event.ActionIndex == nil || *event.ActionIndex != wantIndexes[i] {
			t.Fatalf("event %d index=%v, want %d", i, event.ActionIndex, wantIndexes[i])
		}
		if event.UnitID != target.UnitID {
			t.Fatalf("event %d UnitID=%d, want %d", i, event.UnitID, target.UnitID)
		}
		wantEvent := telemetry.RunEncounterActionStarted
		if i%2 == 1 {
			wantEvent = telemetry.RunEncounterActionCompleted
		}
		if event.Event != wantEvent {
			t.Fatalf("event %d=%s, want %s", i, event.Event, wantEvent)
		}
	}
}

func TestMephistoPipelineAndRealProfileCastTwoPrisonsBeforeBoneSpear(t *testing.T) {
	definition, _ := DefaultRunRegistry().Definition(RunIDMephisto)
	target := mephistoMonster(5252)
	actions := &mockCombatActions{}
	executor, err := profile.NewExecutor(config.NewLogger("error"), profile.Definition{
		ID:             "test-necro",
		CharacterClass: world.CharacterClassNecromancer,
		Hooks: map[profile.Hook][]profile.Action{
			profile.HookBossEngage: {{SkillID: 88, Target: profile.TargetBoss, OncePerEncounter: true}},
		},
	}, actions)
	if err != nil {
		t.Fatal(err)
	}
	pipeline := &runPipeline{
		definition: definition,
		boss: pipelineBossState{targetSeen: true,
			targetUnitID: target.UnitID}, core: pipelineCoreState{combat: killRunConfig().Combat},
	}
	state := mephistoState(target)
	state.Identity = world.GameIdentity{Valid: true, CharacterName: "MrBones", Class: world.CharacterClassNecromancer}
	deps := Deps{Profile: executor, Combat: actions}

	for tick := 0; tick < 4; tick++ {
		result := pipeline.onBossTick(context.Background(), deps, pipelineStepEngageBoss, state, time.Now().Add(time.Duration(tick)*time.Millisecond))
		if result.failed {
			t.Fatalf("tick %d=%+v", tick, result)
		}
	}
	want := []uint16{88, 88, 84}
	if !reflect.DeepEqual(actions.castSkills, want) {
		t.Fatalf("cast order=%v, want %v", actions.castSkills, want)
	}
}

func TestMephistoFailsWhenBossPinIsLostBeforeActionsComplete(t *testing.T) {
	definition, _ := DefaultRunRegistry().Definition(RunIDMephisto)
	pipeline := &runPipeline{
		definition: definition,
		boss: pipelineBossState{targetSeen: true,
			targetUnitID: 10}, core: pipelineCoreState{combat: killRunConfig().Combat},
	}
	result := pipeline.onBossTick(
		context.Background(),
		Deps{Profile: &mockProfileActions{}, Combat: &mockCombatActions{}},
		pipelineStepEngageBoss,
		mephistoState(),
		time.Now(),
	)
	if !result.failed || result.reason != string(RunReasonBossPinLost) {
		t.Fatalf("result=%+v, want boss_pin_lost", result)
	}
}

func TestMephistoAcquiresExactNPCWithoutSuperUniqueFlag(t *testing.T) {
	definition, _ := DefaultRunRegistry().Definition(RunIDMephisto)
	pipeline := &runPipeline{definition: definition}
	target := mephistoMonster(6262)
	target.MonsterTypeFlag = 0

	got, ok := pipeline.findBossTarget(mephistoState(target))
	if !ok || got.UnitID != target.UnitID || got.NPCID != world.Mephisto {
		t.Fatalf("target=%+v ok=%t, want exact Mephisto without super-unique flag", got, ok)
	}
}

func TestCountessStillRequiresSuperUniqueFlagForConfiguredBaseNPC(t *testing.T) {
	definition, _ := DefaultRunRegistry().Definition(RunIDCountess)
	pipeline := &runPipeline{definition: definition}
	trash := countessMonster(88, world.Position{X: 101, Y: 100})
	trash.MonsterTypeFlag = 0
	if got, ok := pipeline.findBossTarget(healthy(cellar5State(trash))); ok {
		t.Fatalf("ordinary Dark Stalker selected as Countess: %+v", got)
	}
}

func TestMephistoRejectsReplacementUnitAndConfirmsTrueAbsence(t *testing.T) {
	definition, _ := DefaultRunRegistry().Definition(RunIDMephisto)
	pipeline := &runPipeline{
		definition: definition,
		boss: pipelineBossState{targetSeen: true,
			targetUnitID:         10,
			encounterActionIndex: len(definition.BossEngageSequence)}, core: pipelineCoreState{combat: killRunConfig().Combat},
	}
	combat := &mockCombatActions{}
	trace := &pipelineTelemetry{}
	replacement := pipeline.onBossTick(context.Background(), Deps{Combat: combat, Telemetry: trace}, pipelineStepEngageBoss, mephistoState(mephistoMonster(11)), time.Now())
	if !replacement.failed || replacement.reason != string(RunReasonBossPinLost) {
		t.Fatalf("replacement result=%+v, want boss_pin_lost", replacement)
	}
	if combat.stopCalls != 1 {
		t.Fatalf("StopAttack calls after pin replacement = %d, want 1", combat.stopCalls)
	}

	pipeline.boss.targetAbsentTicks = 0
	for tick := 1; tick <= pipeline.core.combat.KillConfirmTicks; tick++ {
		result := pipeline.onBossTick(context.Background(), Deps{Combat: combat, Telemetry: trace}, pipelineStepEngageBoss, mephistoState(), time.Now())
		if tick < pipeline.core.combat.KillConfirmTicks && (result.complete || result.failed) {
			t.Fatalf("absence tick %d=%+v, want pending", tick, result)
		}
		if tick == pipeline.core.combat.KillConfirmTicks && (!result.complete || result.failed) {
			t.Fatalf("final absence tick=%+v, want complete", result)
		}
	}
	if combat.stopCalls != 1+pipeline.core.combat.KillConfirmTicks {
		t.Fatalf("StopAttack calls = %d, want release on every absent-target tick", combat.stopCalls)
	}
	if len(trace.events) != 1 || trace.events[0].Event != telemetry.BossKillConfirmed || trace.events[0].UnitID != 10 || trace.events[0].BossID != "mephisto" || trace.events[0].BossName != "Mephisto" || trace.events[0].Stage != telemetry.HistoryStageCombat {
		t.Fatalf("boss kill events=%+v", trace.events)
	}
	_ = pipeline.onBossTick(context.Background(), Deps{Combat: combat, Telemetry: trace}, pipelineStepEngageBoss, mephistoState(), time.Now())
	if len(trace.events) != 1 {
		t.Fatalf("boss kill duplicated: %+v", trace.events)
	}
}

func TestBossKillTelemetryFailurePreventsKillCompletion(t *testing.T) {
	definition, _ := DefaultRunRegistry().Definition(RunIDMephisto)
	pipeline := &runPipeline{
		definition: definition, boss: pipelineBossState{targetSeen: true, targetUnitID: 10,
			encounterActionIndex: len(definition.BossEngageSequence), targetAbsentTicks: killRunConfig().Combat.KillConfirmTicks - 1}, core: pipelineCoreState{combat: killRunConfig().Combat},
	}
	trace := &pipelineTelemetry{failAt: 1}
	result := pipeline.onBossTick(context.Background(), Deps{Combat: &mockCombatActions{}, Telemetry: trace}, pipelineStepEngageBoss, mephistoState(), time.Now())
	if !result.failed || result.complete || result.reason != "telemetry_failed" || pipeline.boss.bossKillEmitted {
		t.Fatalf("result=%+v emitted=%t", result, pipeline.boss.bossKillEmitted)
	}
}

func TestCountessRetainsExactlyOneBossAction(t *testing.T) {
	definition, _ := DefaultRunRegistry().Definition(RunIDCountess)
	target := countessMonster(77, world.Position{X: 110, Y: 100})
	profiles := &mockProfileActions{}
	combat := &mockCombatActions{}
	pipeline := &runPipeline{
		definition: definition,
		boss: pipelineBossState{targetSeen: true,
			targetUnitID: target.UnitID}, core: pipelineCoreState{combat: killRunConfig().Combat},
	}
	result := pipeline.onBossTick(context.Background(), Deps{Profile: profiles, Combat: combat}, pipelineStepEngageBoss, healthy(cellar5State(target)), time.Now())
	if result.failed || profiles.hookCalls != 1 || pipeline.boss.encounterActionIndex != 1 || combat.castCalls != 1 {
		t.Fatalf("result=%+v hooks=%d index=%d combat=%d", result, profiles.hookCalls, pipeline.boss.encounterActionIndex, combat.castCalls)
	}
}

func TestMephistoLootBranchesToPortalAndAct3Normalization(t *testing.T) {
	definition, _ := DefaultRunRegistry().Definition(RunIDMephisto)
	pipeline := &runPipeline{definition: definition, phase: RunPhaseLootAndReturn}
	lootActions := &mockLootActions{scans: []LootScanResult{{}, {}, {}}}
	for tick := 0; tick < lootNoTargetStableTicks; tick++ {
		result := pipeline.onLootTick(context.Background(), Deps{Loot: lootActions}, pipelineStepScanLoot, mephistoState(), time.Now(), time.Now())
		if tick < lootNoTargetStableTicks-1 && (result.complete || result.failed) {
			t.Fatalf("no-drop tick %d=%+v, want pending", tick, result)
		}
		if tick == lootNoTargetStableTicks-1 && (!result.complete || result.failed) {
			t.Fatalf("stable no-drop=%+v, want complete", result)
		}
	}
	if next := pipeline.nextStep(pipelineStepScanLoot); next != pipelineStepCastTownPortal {
		t.Fatalf("no-drop successor=%q, want %q", next, pipelineStepCastTownPortal)
	}
	if next := pipeline.nextStep(pipelineStepWaitOriginTown); next != pipelineStepPlayTownEgress {
		t.Fatalf("Act-3 successor=%q, want %q", next, pipelineStepPlayTownEgress)
	}

	pipeline.loot.lootScanHasTarget = true
	full := pipeline.onLootTick(context.Background(), Deps{Loot: &mockLootActions{scans: []LootScanResult{{InventoryFull: true, InventoryFullCandidateCount: 1}}}}, pipelineStepScanLoot, mephistoState(), time.Now(), time.Now())
	if !full.complete || full.failed || pipeline.loot.lootScanHasTarget {
		t.Fatalf("inventory-full result=%+v hasTarget=%t", full, pipeline.loot.lootScanHasTarget)
	}
	if next := pipeline.nextStep(pipelineStepScanLoot); next != pipelineStepCastTownPortal {
		t.Fatalf("inventory-full successor=%q, want portal", next)
	}
}

func hammerdinMephistoCombat() CombatConfig {
	return CombatConfig{
		Profile:                 "paladin_hammerdin",
		AttackSkillID:           memory.MustSkillID("blessed_hammer"),
		AttackInterval:          350 * time.Millisecond,
		EngageDistanceTiles:     1,
		RepositionDistanceTiles: 3,
		KillConfirmTicks:        3,
	}
}

func farMephistoState(monsters ...world.Monster) world.State {
	state := mephistoState(monsters...)
	state.Player.Position = world.Position{X: 17620, Y: 8069}
	return state
}

func closeMephistoState(monsters ...world.Monster) world.State {
	state := mephistoState(monsters...)
	state.Player.Position = world.Position{X: 17566, Y: 8066}
	return state
}

func TestHammerdinMephistoStartsStandardAttackAfterEmptyEngage(t *testing.T) {
	definition, _ := DefaultRunRegistry().Definition(RunIDMephisto)
	target := mephistoMonster(4242)
	profiles := &mockProfileActions{}
	combat := &mockCombatActions{}
	pipeline := &runPipeline{
		definition: definition,
		boss:       pipelineBossState{targetSeen: true, targetUnitID: target.UnitID},
		core:       pipelineCoreState{combat: hammerdinMephistoCombat()},
	}
	deps := Deps{Profile: profiles, Combat: combat}
	now := time.Now()

	first := pipeline.onBossTick(context.Background(), deps, pipelineStepEngageBoss, closeMephistoState(target), now)
	if first.failed || first.complete || combat.holdCalls != 0 || pipeline.boss.encounterActionIndex != 1 {
		t.Fatalf("first empty engage=%+v index=%d holds=%d", first, pipeline.boss.encounterActionIndex, combat.holdCalls)
	}
	second := pipeline.onBossTick(context.Background(), deps, pipelineStepEngageBoss, closeMephistoState(target), now.Add(time.Millisecond))
	if second.failed || combat.holdCalls != 1 || combat.teleportCalls != 0 || combat.lastSkillID != memory.MustSkillID("blessed_hammer") {
		t.Fatalf("second empty engage=%+v holds=%d teleports=%d skill=%d", second, combat.holdCalls, combat.teleportCalls, combat.lastSkillID)
	}
}

func TestHammerdinEngageTeleportsWhenBeyondTolerance(t *testing.T) {
	definition, _ := DefaultRunRegistry().Definition(RunIDMephisto)
	target := mephistoMonster(4242)
	combat := &mockCombatActions{}
	pipeline := &runPipeline{
		definition: definition,
		boss: pipelineBossState{
			targetSeen:           true,
			targetUnitID:         target.UnitID,
			encounterActionIndex: len(definition.BossEngageSequence),
		},
		core: pipelineCoreState{combat: hammerdinMephistoCombat()},
	}

	result := pipeline.onBossTick(context.Background(), Deps{Combat: combat}, pipelineStepEngageBoss, farMephistoState(target), time.Now())
	if result.failed || combat.teleportCalls != 1 || combat.holdCalls != 0 || combat.lastDesired != 1 {
		t.Fatalf("result=%+v teleports=%d holds=%d desired=%.0f", result, combat.teleportCalls, combat.holdCalls, combat.lastDesired)
	}
}

func TestHammerdinEngageAttacksWhenWithinTolerance(t *testing.T) {
	definition, _ := DefaultRunRegistry().Definition(RunIDMephisto)
	target := mephistoMonster(4242)
	combat := &mockCombatActions{}
	pipeline := &runPipeline{
		definition: definition,
		boss: pipelineBossState{
			targetSeen:           true,
			targetUnitID:         target.UnitID,
			encounterActionIndex: len(definition.BossEngageSequence),
		},
		core: pipelineCoreState{combat: hammerdinMephistoCombat()},
	}

	result := pipeline.onBossTick(context.Background(), Deps{Combat: combat}, pipelineStepEngageBoss, closeMephistoState(target), time.Now())
	if result.failed || combat.holdCalls != 1 || combat.teleportCalls != 0 || combat.lastSkillID != memory.MustSkillID("blessed_hammer") {
		t.Fatalf("result=%+v holds=%d teleports=%d skill=%d", result, combat.holdCalls, combat.teleportCalls, combat.lastSkillID)
	}
	if !pipeline.boss.hammerdinAttackHeld {
		t.Fatal("expected LMB hold after in-range engage")
	}
}

func TestHammerdinEngageAcceptsOverlayHoverAfterAim(t *testing.T) {
	definition, _ := DefaultRunRegistry().Definition(RunIDMephisto)
	boss := mephistoMonster(4242)
	overlay := mephistoMonster(99)
	overlay.IsHovered = true
	combat := &mockCombatActions{holdResults: []profile.MonsterCastResult{
		{AimRequested: true},
		{Sent: true, TargetingMode: profile.MonsterTargetingHoverConfirmed},
	}}
	pipeline := &runPipeline{
		definition: definition,
		boss: pipelineBossState{
			targetSeen:           true,
			targetUnitID:         boss.UnitID,
			encounterActionIndex: len(definition.BossEngageSequence),
		},
		core: pipelineCoreState{combat: hammerdinMephistoCombat()},
	}
	now := time.Now()
	aim := pipeline.onBossTick(context.Background(), Deps{Combat: combat}, pipelineStepEngageBoss, closeMephistoState(boss), now)
	if aim.failed || combat.holdCalls != 1 || pipeline.boss.hammerdinAttackHeld {
		t.Fatalf("aim=%+v holds=%d held=%t", aim, combat.holdCalls, pipeline.boss.hammerdinAttackHeld)
	}

	overlayState := closeMephistoState(boss, overlay)
	overlayState.At = now.Add(time.Millisecond)
	hold := pipeline.onBossTick(context.Background(), Deps{Combat: combat}, pipelineStepEngageBoss, overlayState, now.Add(time.Millisecond))
	if hold.failed || combat.holdCalls != 2 || combat.lastMonsterUnitID != overlay.UnitID || !pipeline.boss.hammerdinAttackHeld {
		t.Fatalf("overlay hold=%+v holds=%d unit=%d held=%t", hold, combat.holdCalls, combat.lastMonsterUnitID, pipeline.boss.hammerdinAttackHeld)
	}
}

func TestHammerdinEngageKeepsHoldUntilRecheckThenTeleports(t *testing.T) {
	definition, _ := DefaultRunRegistry().Definition(RunIDMephisto)
	target := mephistoMonster(4242)
	combat := &mockCombatActions{}
	pipeline := &runPipeline{
		definition: definition,
		boss: pipelineBossState{
			targetSeen:           true,
			targetUnitID:         target.UnitID,
			encounterActionIndex: len(definition.BossEngageSequence),
		},
		core: pipelineCoreState{combat: hammerdinMephistoCombat()},
	}
	now := time.Now()
	closeTick := pipeline.onBossTick(context.Background(), Deps{Combat: combat}, pipelineStepEngageBoss, closeMephistoState(target), now)
	if closeTick.failed || combat.holdCalls != 1 || combat.teleportCalls != 0 || combat.stopCalls != 0 {
		t.Fatalf("hold start=%+v holds=%d teleports=%d stops=%d", closeTick, combat.holdCalls, combat.teleportCalls, combat.stopCalls)
	}

	for tick := 1; tick < hammerdinHoldRecheckSnapshots; tick++ {
		held := pipeline.onBossTick(context.Background(), Deps{Combat: combat}, pipelineStepEngageBoss, closeMephistoState(target), now.Add(time.Duration(tick)*time.Millisecond))
		if held.failed || combat.holdCalls != 1 || combat.teleportCalls != 0 || combat.stopCalls != 0 {
			t.Fatalf("hold tick %d=%+v holds=%d teleports=%d stops=%d", tick, held, combat.holdCalls, combat.teleportCalls, combat.stopCalls)
		}
	}

	far := farMephistoState(target)
	recheck := pipeline.onBossTick(context.Background(), Deps{Combat: combat}, pipelineStepEngageBoss, far, now.Add(time.Second))
	if recheck.failed || combat.holdCalls != 1 || combat.stopCalls != 1 || combat.teleportCalls != 0 {
		t.Fatalf("distance recheck=%+v holds=%d stops=%d teleports=%d, want release without same-tick teleport", recheck, combat.holdCalls, combat.stopCalls, combat.teleportCalls)
	}
	if pipeline.boss.hammerdinAttackHeld {
		t.Fatal("hold should be released after a failed distance recheck")
	}
	follow := pipeline.onBossTick(context.Background(), Deps{Combat: combat}, pipelineStepEngageBoss, far, now.Add(time.Second+time.Millisecond))
	if follow.failed || combat.teleportCalls != 1 {
		t.Fatalf("follow-up=%+v teleports=%d, want teleport after the release snapshot", follow, combat.teleportCalls)
	}
}

func TestHammerdinEngageRepositionsThroughOtherMonstersWhenLandingBlocks(t *testing.T) {
	definition, _ := DefaultRunRegistry().Definition(RunIDMephisto)
	target := mephistoMonster(4242)
	next := world.Monster{NPCID: 243, UnitID: 5001, Position: world.Position{X: 17575, Y: 8066}}
	third := world.Monster{NPCID: 244, UnitID: 5002, Position: world.Position{X: 17582, Y: 8066}}
	combat := &mockCombatActions{}
	pipeline := &runPipeline{
		definition: definition,
		boss: pipelineBossState{
			targetSeen:           true,
			targetUnitID:         target.UnitID,
			encounterActionIndex: len(definition.BossEngageSequence),
		},
		core: pipelineCoreState{combat: hammerdinMephistoCombat()},
	}
	base := time.Date(2026, 8, 17, 13, 0, 0, 0, time.UTC)
	state := closeMephistoState(target, next, third)
	state.At = base
	if result := pipeline.onBossTick(context.Background(), Deps{Combat: combat}, pipelineStepEngageBoss, state, base); result.failed {
		t.Fatalf("initial hold failed: %+v", result)
	}

	state.At = base.Add(hammerdinAttackWindow)
	if result := pipeline.onBossTick(context.Background(), Deps{Combat: combat}, pipelineStepEngageBoss, state, state.At); result.failed {
		t.Fatalf("first fallback start failed: %+v", result)
	}
	if combat.stopCalls != 1 || combat.teleportCalls != 0 {
		t.Fatalf("first fallback stops=%d teleports=%d, want release-only tick", combat.stopCalls, combat.teleportCalls)
	}
	state.At = state.At.Add(time.Millisecond)
	if result := pipeline.onBossTick(context.Background(), Deps{Combat: combat}, pipelineStepEngageBoss, state, state.At); result.failed {
		t.Fatalf("first fallback teleport failed: %+v", result)
	}
	if combat.teleportCalls != 1 || combat.lastDesired != 1 || combat.lastTeleportTarget != next.Position {
		t.Fatalf("first fallback teleports=%d desired=%.0f target=%+v, want %+v",
			combat.teleportCalls, combat.lastDesired, combat.lastTeleportTarget, next.Position)
	}

	// No player movement at settle means the terrain rejected candidate 1.
	state.At = state.At.Add(routeThreatApproachSettle)
	if result := pipeline.onBossTick(context.Background(), Deps{Combat: combat}, pipelineStepEngageBoss, state, state.At); result.failed {
		t.Fatalf("blocked landing settle failed: %+v", result)
	}
	state.At = state.At.Add(time.Millisecond)
	if result := pipeline.onBossTick(context.Background(), Deps{Combat: combat}, pipelineStepEngageBoss, state, state.At); result.failed {
		t.Fatalf("second hold failed: %+v", result)
	}
	state.At = state.At.Add(hammerdinAttackWindow)
	if result := pipeline.onBossTick(context.Background(), Deps{Combat: combat}, pipelineStepEngageBoss, state, state.At); result.failed {
		t.Fatalf("second fallback start failed: %+v", result)
	}
	state.At = state.At.Add(time.Millisecond)
	if result := pipeline.onBossTick(context.Background(), Deps{Combat: combat}, pipelineStepEngageBoss, state, state.At); result.failed {
		t.Fatalf("second fallback teleport failed: %+v", result)
	}
	if combat.teleportCalls != 2 || combat.lastDesired != 1 || combat.lastTeleportTarget != third.Position {
		t.Fatalf("second fallback teleports=%d desired=%.0f target=%+v, want %+v",
			combat.teleportCalls, combat.lastDesired, combat.lastTeleportTarget, third.Position)
	}
}

func TestHammerdinEngageAttacksPinnedBossBeforeReturningAfterReposition(t *testing.T) {
	definition, _ := DefaultRunRegistry().Definition(RunIDMephisto)
	target := mephistoMonster(4242)
	next := world.Monster{NPCID: 243, UnitID: 5001, Position: world.Position{X: 17575, Y: 8066}}
	combat := &mockCombatActions{}
	pipeline := &runPipeline{
		definition: definition,
		boss: pipelineBossState{
			targetSeen:           true,
			targetUnitID:         target.UnitID,
			encounterActionIndex: len(definition.BossEngageSequence),
		},
		core: pipelineCoreState{combat: hammerdinMephistoCombat()},
	}
	base := time.Date(2026, 8, 17, 13, 0, 0, 0, time.UTC)
	state := closeMephistoState(target, next)
	state.At = base
	if result := pipeline.onBossTick(context.Background(), Deps{Combat: combat}, pipelineStepEngageBoss, state, state.At); result.failed {
		t.Fatalf("initial hold failed: %+v", result)
	}
	state.At = base.Add(hammerdinAttackWindow)
	if result := pipeline.onBossTick(context.Background(), Deps{Combat: combat}, pipelineStepEngageBoss, state, state.At); result.failed {
		t.Fatalf("fallback start failed: %+v", result)
	}
	state.At = state.At.Add(time.Millisecond)
	if result := pipeline.onBossTick(context.Background(), Deps{Combat: combat}, pipelineStepEngageBoss, state, state.At); result.failed {
		t.Fatalf("fallback teleport failed: %+v", result)
	}

	state.At = base.Add(hammerdinAttackWindow + 100*time.Millisecond)
	state.Player.Position = world.Position{X: 17574, Y: 8066}
	if result := pipeline.onBossTick(context.Background(), Deps{Combat: combat}, pipelineStepEngageBoss, state, state.At); result.failed {
		t.Fatalf("fallback progress failed: %+v", result)
	}
	state.At = base.Add(hammerdinAttackWindow + 200*time.Millisecond)
	if result := pipeline.onBossTick(context.Background(), Deps{Combat: combat}, pipelineStepEngageBoss, state, state.At); result.failed {
		t.Fatalf("pinned boss attack failed: %+v", result)
	}
	if combat.teleportCalls != 1 || combat.holdCalls != 2 || combat.lastMonsterUnitID != target.UnitID {
		t.Fatalf("teleports=%d holds=%d target=%d, want pinned boss attacked before return", combat.teleportCalls, combat.holdCalls, combat.lastMonsterUnitID)
	}
}

func TestHammerdinEngageInRangeHoldDoesNotRefreshNoProgressWatchdog(t *testing.T) {
	definition, _ := DefaultRunRegistry().Definition(RunIDMephisto)
	target := mephistoMonster(4242)
	combat := &mockCombatActions{}
	pipeline := &runPipeline{
		definition: definition,
		boss: pipelineBossState{
			targetSeen:           true,
			targetUnitID:         target.UnitID,
			encounterActionIndex: len(definition.BossEngageSequence),
		},
		core: pipelineCoreState{combat: hammerdinMephistoCombat()},
	}
	base := time.Date(2026, 8, 17, 13, 0, 0, 0, time.UTC)
	state := closeMephistoState(target)
	state.At = base
	if result := pipeline.onBossTick(context.Background(), Deps{Combat: combat}, pipelineStepEngageBoss, state, base); result.failed {
		t.Fatalf("initial hold failed: %+v", result)
	}
	for snapshot := 1; snapshot <= hammerdinHoldRecheckSnapshots; snapshot++ {
		state.At = base.Add(time.Duration(snapshot) * 100 * time.Millisecond)
		if result := pipeline.onBossTick(context.Background(), Deps{Combat: combat}, pipelineStepEngageBoss, state, state.At); result.failed {
			t.Fatalf("hold recheck %d failed: %+v", snapshot, result)
		}
	}
	state.At = base.Add(hammerdinEngageNoProgressTimeout)
	result := pipeline.onBossTick(context.Background(), Deps{Combat: combat}, pipelineStepEngageBoss, state, state.At)
	if !result.failed || result.reason != string(RunReasonBossCombatNoProgress) || combat.stopCalls != 1 {
		t.Fatalf("timeout result=%+v stops=%d, want boss_combat_no_progress", result, combat.stopCalls)
	}
}

func TestHammerdinEngageFailsAfterTeleportBudget(t *testing.T) {
	definition, _ := DefaultRunRegistry().Definition(RunIDMephisto)
	target := mephistoMonster(4242)
	combat := &mockCombatActions{}
	pipeline := &runPipeline{
		definition: definition,
		boss: pipelineBossState{
			targetSeen:           true,
			targetUnitID:         target.UnitID,
			encounterActionIndex: len(definition.BossEngageSequence),
		},
		core: pipelineCoreState{combat: hammerdinMephistoCombat()},
	}
	now := time.Now()
	for tick := 1; tick <= hammerdinEngageMaxTeleports; tick++ {
		result := pipeline.onBossTick(context.Background(), Deps{Combat: combat}, pipelineStepEngageBoss, farMephistoState(target), now.Add(time.Duration(tick)*time.Millisecond))
		if tick < hammerdinEngageMaxTeleports && (result.failed || result.complete) {
			t.Fatalf("teleport tick %d=%+v, want pending", tick, result)
		}
		if tick == hammerdinEngageMaxTeleports && (!result.failed || result.reason != string(RunReasonBossCombatNoProgress)) {
			t.Fatalf("final teleport tick=%+v, want boss_combat_no_progress", result)
		}
	}
	if combat.teleportCalls != hammerdinEngageMaxTeleports || combat.holdCalls != 0 {
		t.Fatalf("teleports=%d holds=%d", combat.teleportCalls, combat.holdCalls)
	}
}

func TestHammerdinEngageFailsWhenNoHammerProgress(t *testing.T) {
	definition, _ := DefaultRunRegistry().Definition(RunIDMephisto)
	target := mephistoMonster(4242)
	combat := &mockCombatActions{teleportSent: make([]bool, 4)}
	pipeline := &runPipeline{
		definition: definition,
		boss: pipelineBossState{
			targetSeen:           true,
			targetUnitID:         target.UnitID,
			encounterActionIndex: len(definition.BossEngageSequence),
		},
		core: pipelineCoreState{combat: hammerdinMephistoCombat()},
	}
	now := time.Now()
	first := pipeline.onBossTick(context.Background(), Deps{Combat: combat}, pipelineStepEngageBoss, farMephistoState(target), now)
	if first.failed || combat.teleportCalls != 1 {
		t.Fatalf("first result=%+v teleports=%d", first, combat.teleportCalls)
	}
	later := pipeline.onBossTick(context.Background(), Deps{Combat: combat}, pipelineStepEngageBoss, farMephistoState(target), now.Add(hammerdinEngageNoProgressTimeout))
	if !later.failed || later.reason != string(RunReasonBossCombatNoProgress) {
		t.Fatalf("timeout result=%+v, want boss_combat_no_progress", later)
	}
}
