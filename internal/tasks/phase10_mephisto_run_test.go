package tasks

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
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
		definition:   definition,
		combat:       killRunConfig().Combat,
		targetSeen:   true,
		targetUnitID: target.UnitID,
	}
	deps := Deps{Telemetry: trace, Profile: profiles, Combat: combat}

	first := pipeline.onBossTick(context.Background(), deps, pipelineStepEngageBoss, mephistoState(target), time.Now())
	if first.failed || combat.castCalls != 0 || pipeline.encounterActionIndex != 1 {
		t.Fatalf("first action=%+v index=%d combat=%d", first, pipeline.encounterActionIndex, combat.castCalls)
	}
	second := pipeline.onBossTick(context.Background(), deps, pipelineStepEngageBoss, mephistoState(target), time.Now())
	if second.failed || profiles.hookCalls != 2 || combat.castCalls != 1 || pipeline.encounterActionIndex != 2 {
		t.Fatalf("second action=%+v hooks=%d index=%d combat=%d", second, profiles.hookCalls, pipeline.encounterActionIndex, combat.castCalls)
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
		definition:   definition,
		combat:       killRunConfig().Combat,
		targetSeen:   true,
		targetUnitID: target.UnitID,
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
		definition:   definition,
		combat:       killRunConfig().Combat,
		targetSeen:   true,
		targetUnitID: 10,
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
		definition:           definition,
		combat:               killRunConfig().Combat,
		targetSeen:           true,
		targetUnitID:         10,
		encounterActionIndex: len(definition.BossEngageSequence),
	}
	combat := &mockCombatActions{}
	replacement := pipeline.onBossTick(context.Background(), Deps{Combat: combat}, pipelineStepEngageBoss, mephistoState(mephistoMonster(11)), time.Now())
	if !replacement.failed || replacement.reason != string(RunReasonBossPinLost) {
		t.Fatalf("replacement result=%+v, want boss_pin_lost", replacement)
	}
	if combat.stopCalls != 1 {
		t.Fatalf("StopAttack calls after pin replacement = %d, want 1", combat.stopCalls)
	}

	pipeline.targetAbsentTicks = 0
	for tick := 1; tick <= pipeline.combat.KillConfirmTicks; tick++ {
		result := pipeline.onBossTick(context.Background(), Deps{Combat: combat}, pipelineStepEngageBoss, mephistoState(), time.Now())
		if tick < pipeline.combat.KillConfirmTicks && (result.complete || result.failed) {
			t.Fatalf("absence tick %d=%+v, want pending", tick, result)
		}
		if tick == pipeline.combat.KillConfirmTicks && (!result.complete || result.failed) {
			t.Fatalf("final absence tick=%+v, want complete", result)
		}
	}
	if combat.stopCalls != 1+pipeline.combat.KillConfirmTicks {
		t.Fatalf("StopAttack calls = %d, want release on every absent-target tick", combat.stopCalls)
	}
}

func TestCountessRetainsExactlyOneBossAction(t *testing.T) {
	definition, _ := DefaultRunRegistry().Definition(RunIDCountess)
	target := countessMonster(77, world.Position{X: 110, Y: 100})
	profiles := &mockProfileActions{}
	combat := &mockCombatActions{}
	pipeline := &runPipeline{
		definition:   definition,
		combat:       killRunConfig().Combat,
		targetSeen:   true,
		targetUnitID: target.UnitID,
	}
	result := pipeline.onBossTick(context.Background(), Deps{Profile: profiles, Combat: combat}, pipelineStepEngageBoss, healthy(cellar5State(target)), time.Now())
	if result.failed || profiles.hookCalls != 1 || pipeline.encounterActionIndex != 1 || combat.castCalls != 1 {
		t.Fatalf("result=%+v hooks=%d index=%d combat=%d", result, profiles.hookCalls, pipeline.encounterActionIndex, combat.castCalls)
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

	pipeline.lootScanHasTarget = true
	full := pipeline.onLootTick(context.Background(), Deps{Loot: &mockLootActions{scans: []LootScanResult{{InventoryFull: true, InventoryFullCandidateCount: 1}}}}, pipelineStepScanLoot, mephistoState(), time.Now(), time.Now())
	if !full.complete || full.failed || pipeline.lootScanHasTarget {
		t.Fatalf("inventory-full result=%+v hasTarget=%t", full, pipeline.lootScanHasTarget)
	}
	if next := pipeline.nextStep(pipelineStepScanLoot); next != pipelineStepCastTownPortal {
		t.Fatalf("inventory-full successor=%q, want portal", next)
	}
}
