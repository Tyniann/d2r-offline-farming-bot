package tasks

import (
	"context"
	"testing"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/memory"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

func TestHammerdinCountessStartsStandardAttackAfterEmptyEngage(t *testing.T) {
	definition, _ := DefaultRunRegistry().Definition(RunIDCountess)
	target := countessMonster(77, world.Position{X: 101, Y: 100})
	profiles := &mockProfileActions{}
	combat := &mockCombatActions{}
	pipeline := &runPipeline{
		definition: definition,
		boss:       pipelineBossState{targetSeen: true, targetUnitID: target.UnitID},
		core:       pipelineCoreState{combat: hammerdinMephistoCombat()},
	}

	result := pipeline.onBossTick(context.Background(), Deps{Profile: profiles, Combat: combat}, pipelineStepEngageBoss, healthy(cellar5State(target)), time.Now())
	if result.failed || combat.holdCalls != 1 || combat.castCalls != 0 || combat.teleportCalls != 0 ||
		combat.lastSkillID != memory.MustSkillID("blessed_hammer") || pipeline.boss.encounterActionIndex != 1 {
		t.Fatalf("empty countess engage=%+v index=%d holds=%d casts=%d teleports=%d skill=%d",
			result, pipeline.boss.encounterActionIndex, combat.holdCalls, combat.castCalls, combat.teleportCalls, combat.lastSkillID)
	}
}

func TestHammerdinNihlathakEngageUsesStandardHoldNotNecroApproach(t *testing.T) {
	definition, _ := DefaultRunRegistry().Definition(RunIDNihlathak)
	closeBoss := world.Monster{NPCID: world.Nihlathak, UnitID: 42, Position: world.Position{X: 101, Y: 100}}
	combat := &mockCombatActions{}
	pipeline := &runPipeline{
		definition: definition,
		boss:       pipelineBossState{targetSeen: true, targetUnitID: closeBoss.UnitID},
		core:       pipelineCoreState{combat: hammerdinMephistoCombat()},
	}

	closeTick := pipeline.onBossTick(context.Background(), Deps{Combat: combat}, pipelineStepEngageBoss, nihlathakBossState(world.Position{X: 100, Y: 100}, closeBoss), time.Unix(10, 0).UTC())
	if closeTick.failed || combat.holdCalls != 1 || combat.castCalls != 0 || combat.teleportCalls != 0 ||
		combat.lastSkillID != memory.MustSkillID("blessed_hammer") {
		t.Fatalf("close nihlathak=%+v holds=%d casts=%d teleports=%d skill=%d",
			closeTick, combat.holdCalls, combat.castCalls, combat.teleportCalls, combat.lastSkillID)
	}

	aimFalse := false
	farCombat := &mockCombatActions{aimProjectable: &aimFalse, farthestDistance: 18, farthestOK: boolPtr(true)}
	farBoss := world.Monster{NPCID: world.Nihlathak, UnitID: 42, Position: world.Position{X: 200, Y: 200}}
	farPipeline := &runPipeline{
		definition: definition,
		boss:       pipelineBossState{targetSeen: true, targetUnitID: farBoss.UnitID},
		core:       pipelineCoreState{combat: hammerdinMephistoCombat()},
	}
	farTick := farPipeline.onBossTick(context.Background(), Deps{Combat: farCombat}, pipelineStepEngageBoss, nihlathakBossState(world.Position{X: 100, Y: 100}, farBoss), time.Unix(10, 0).UTC())
	if farTick.failed || farCombat.teleportCalls != 1 || farCombat.lastDesired != 1 || farCombat.holdCalls != 0 || farCombat.castCalls != 0 {
		t.Fatalf("far nihlathak=%+v teleports=%d desired=%.0f holds=%d casts=%d, want melee teleport not Necro projection",
			farTick, farCombat.teleportCalls, farCombat.lastDesired, farCombat.holdCalls, farCombat.castCalls)
	}
}

func TestHammerdinNihlathakSkipsPostBossCleanup(t *testing.T) {
	nihlathak, _ := DefaultRunRegistry().Definition(RunIDNihlathak)
	pipeline := &runPipeline{
		definition: nihlathak,
		core:       pipelineCoreState{combat: hammerdinMephistoCombat()},
	}
	if got := pipeline.nextStep(pipelineStepEngageBoss); got != pipelineStepRepositionForLoot {
		t.Fatalf("Hammerdin Nihlathak engage successor = %q, want loot reposition", got)
	}
	pipeline.boss.bossKillEmitted = true
	if got := pipeline.nextStep(pipelineStepAcquireBoss); got != pipelineStepRepositionForLoot {
		t.Fatalf("Hammerdin Nihlathak acquire successor = %q, want loot reposition", got)
	}
}

func TestHammerdinSummonerStartsStandardAttackAfterEmptyEngage(t *testing.T) {
	definition, _ := DefaultRunRegistry().Definition(RunIDSummoner)
	target := world.Monster{NPCID: world.Summoner, UnitID: 77, Position: world.Position{X: 101, Y: 100}}
	combat := &mockCombatActions{}
	pipeline := &runPipeline{
		definition: definition,
		boss:       pipelineBossState{targetSeen: true, targetUnitID: target.UnitID},
		core:       pipelineCoreState{combat: hammerdinMephistoCombat()},
	}
	state := healthy(areaState(world.ArcaneSanctuary))
	state.Player.Position = world.Position{X: 100, Y: 100}
	state.Monsters = []world.Monster{target}

	result := pipeline.onBossTick(context.Background(), Deps{Combat: combat}, pipelineStepEngageBoss, state, time.Now())
	if result.failed || combat.holdCalls != 1 || combat.castCalls != 0 || combat.teleportCalls != 0 ||
		combat.lastSkillID != memory.MustSkillID("blessed_hammer") {
		t.Fatalf("summoner engage=%+v holds=%d casts=%d teleports=%d skill=%d",
			result, combat.holdCalls, combat.castCalls, combat.teleportCalls, combat.lastSkillID)
	}
}

func TestHammerdinSummonerSkipsPostBossCleanup(t *testing.T) {
	summoner, _ := DefaultRunRegistry().Definition(RunIDSummoner)
	pipeline := &runPipeline{
		definition: summoner,
		core:       pipelineCoreState{combat: hammerdinMephistoCombat()},
	}
	if got := pipeline.nextStep(pipelineStepEngageBoss); got != pipelineStepRepositionForLoot {
		t.Fatalf("Hammerdin Summoner engage successor = %q, want loot reposition", got)
	}
}
