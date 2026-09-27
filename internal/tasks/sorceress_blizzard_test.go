package tasks

import (
	"context"
	"testing"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/memory"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/profile"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/profile/sorceressblizzard"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/telemetry"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

func blizzardTaskFixture(t *testing.T, runID RunID) (*runPipeline, *profile.Executor, *sorceressblizzard.EncounterExecutor, *mockCombatActions, world.State) {
	t.Helper()
	var profiles config.ProfilesConfig
	profiles.ApplyDefaults()
	cfg := profiles[sorceressblizzard.ID]
	definition, ok := DefaultRunRegistry().Definition(runID)
	if !ok {
		t.Fatal("missing route", runID)
	}
	combat := &mockCombatActions{}
	hooks := map[profile.Hook][]profile.Action{}
	for hook, actions := range map[profile.Hook][]config.ProfileActionConfig{profile.HookTownReady: cfg.Hooks.TownReady, profile.HookBossEngage: cfg.Hooks.BossEngage} {
		for _, action := range actions {
			hooks[hook] = append(hooks[hook], profile.Action{SkillID: memory.MustSkillID(action.Skill), Target: profile.TargetKind(action.Target), OncePerEncounter: action.OncePerEncounter, OncePerGame: action.OncePerGame, Delay: time.Duration(action.DelayMs) * time.Millisecond, Settle: time.Duration(action.SettleMs) * time.Millisecond})
		}
	}
	exec, err := profile.NewExecutor(config.NewLogger("error"), profile.Definition{ID: sorceressblizzard.ID, CharacterClass: world.CharacterClassSorceress, Hooks: hooks}, combat)
	if err != nil {
		t.Fatal(err)
	}
	if err := sorceressblizzard.NewFactory(string(runID))().Configure(exec, memory.MustSkillID(cfg.Combat.StandardAttack), combat); err != nil {
		t.Fatal(err)
	}
	pipeline := &runPipeline{definition: definition, core: pipelineCoreState{combat: CombatConfig{Profile: sorceressblizzard.ID, AttackSkillID: memory.MustSkillID(cfg.Combat.StandardAttack), AttackInterval: time.Duration(cfg.Combat.AttackIntervalMs) * time.Millisecond, EngageDistanceTiles: cfg.Combat.EngageDistanceTiles, RepositionDistanceTiles: cfg.Combat.RepositionDistanceTiles, KillConfirmTicks: cfg.Combat.KillConfirmTicks}}}
	w := world.State{Valid: true, Phase: world.GamePhaseInGame, At: time.Now().Add(time.Hour), Generation: 1, Area: world.LookupArea(definition.RouteTerminalArea), Identity: world.GameIdentity{Valid: true, Class: world.CharacterClassSorceress}, Player: world.Player{Position: world.Position{X: 100, Y: 100}, HP: 100, MaxHP: 100, Mana: 100, MaxMana: 100}}
	return pipeline, exec, sorceressblizzard.NewEncounterExecutor(exec, combat), combat, w
}

func TestBlizzardBossRoutesAcquireEngageConfirmKillAndChooseCleanup(t *testing.T) {
	for _, run := range []RunID{RunIDCountess, RunIDMephisto, RunIDSummoner, RunIDNihlathak} {
		t.Run(string(run), func(t *testing.T) {
			p, exec, hooks, combat, w := blizzardTaskFixture(t, run)
			boss := world.Monster{UnitID: 7, NPCID: p.definition.Boss.NPCID, Position: world.Position{X: 140, Y: 100}, MonsterTypeFlag: world.SuperUniqueMonsterFlag}
			w.Monsters = []world.Monster{boss}
			deps := Deps{Combat: combat, Profile: hooks, RouteClear: exec}
			if got := p.onBossTick(context.Background(), deps, pipelineStepAcquireBoss, w, w.At); got.failed || !got.complete || p.boss.targetUnitID != 7 {
				t.Fatalf("acquire=%+v", got)
			}
			for i := 0; i < 15; i++ {
				w.At = w.At.Add(600 * time.Millisecond)
				got := p.onBossTick(context.Background(), deps, pipelineStepEngageBoss, w, w.At)
				if got.failed {
					t.Fatalf("engage=%+v", got)
				}
				if combat.teleportCalls > 0 {
					w.Player.Position = world.Position{X: 137, Y: 100}
				}
				if combat.lastSkillID == memory.MustSkillID("blizzard") {
					break
				}
			}
			static := 0
			for _, skill := range combat.castSkills {
				if skill == memory.MustSkillID("static_field") {
					static++
				} else if skill != memory.MustSkillID("blizzard") {
					t.Fatalf("unexpected skill %d", skill)
				}
			}
			wantStatic := 0
			if run == RunIDMephisto {
				wantStatic = 3
			}
			if static != wantStatic || combat.lastSkillID != memory.MustSkillID("blizzard") || combat.holdCalls != 0 {
				t.Fatalf("casts=%v static=%d", combat.castSkills, static)
			}
			if run == RunIDNihlathak && (combat.teleportCalls != 0 || combat.lastMonsterUnitID != boss.UnitID) {
				t.Fatalf("Nihlathak lost recorded anchor: %+v", combat)
			}
			if run == RunIDMephisto && (combat.teleportCalls != 1 || combat.lastDesired != 3) {
				t.Fatalf("Static Field approach=%+v", combat)
			}
			if run == RunIDCountess && (combat.teleportCalls != 1 || combat.lastDesired != 15) {
				t.Fatalf("Countess approach=%+v", combat)
			}
			w.Monsters = nil
			for i := 1; i <= 3; i++ {
				w.At = w.At.Add(time.Second)
				got := p.onBossTick(context.Background(), deps, pipelineStepEngageBoss, w, w.At)
				if got.failed || got.complete != (i == 3) {
					t.Fatalf("kill confirmation %d=%+v", i, got)
				}
			}
			wantNext := pipelineStepRepositionForLoot
			if run == RunIDSummoner || run == RunIDNihlathak {
				wantNext = pipelineStepClearNearbyHostiles
			}
			if got := p.nextStep(pipelineStepEngageBoss); got != wantNext {
				t.Fatalf("next=%s want=%s", got, wantNext)
			}
			if got := p.nextStep(pipelineStepCloseStash); got != pipelineStepPrepareTown {
				t.Fatalf("town handoff=%s", got)
			}
		})
	}
}

func TestBlizzardNihlathakApproachIsSingleFreshAndBounded(t *testing.T) {
	p, exec, hooks, combat, w := blizzardTaskFixture(t, RunIDNihlathak)
	boss := world.Monster{UnitID: 7, NPCID: world.Nihlathak, Position: world.Position{X: 150, Y: 100}}
	w.Monsters = []world.Monster{boss}
	p.storeBossTarget(boss)
	projectable := false
	combat.aimProjectable = &projectable
	combat.farthestDistance = 18
	combat.farthestOK = boolPtr(true)
	deps := Deps{Combat: combat, Profile: hooks, RouteClear: exec}
	got := p.onBossTick(context.Background(), deps, pipelineStepEngageBoss, w, w.At)
	if got.failed || combat.teleportCalls != 1 || combat.lastDesired != 18 || combat.castCalls != 0 {
		t.Fatalf("approach=%+v combat=%+v", got, combat)
	}
	projectable = true
	got = p.onBossTick(context.Background(), deps, pipelineStepEngageBoss, w, w.At.Add(time.Second))
	if got.failed || combat.castCalls != 0 {
		t.Fatalf("stale landing cast=%+v", got)
	}
	w.At = w.At.Add(time.Second)
	got = p.onBossTick(context.Background(), deps, pipelineStepEngageBoss, w, w.At)
	if got.failed || combat.lastMonsterUnitID != 7 || combat.lastSkillID != memory.MustSkillID("blizzard") {
		t.Fatalf("landed=%+v combat=%+v", got, combat)
	}
	projectable = false
	w.At = w.At.Add(time.Second)
	got = p.onBossTick(context.Background(), deps, pipelineStepEngageBoss, w, w.At)
	if !got.failed || got.reason != "boss_combat_unprojectable" || combat.teleportCalls != 1 {
		t.Fatalf("second approach=%+v calls=%d", got, combat.teleportCalls)
	}
}

func TestBlizzardPostBossCleanupRespectsRadiusBudgetAndUnprojectableTargets(t *testing.T) {
	for _, run := range []RunID{RunIDSummoner, RunIDNihlathak} {
		t.Run(string(run), func(t *testing.T) {
			p, exec, hooks, combat, w := blizzardTaskFixture(t, run)
			p.boss.targetUnitID = 7
			inside := world.Monster{UnitID: 8, NPCID: world.ArcaneGhoulLord, Position: world.Position{X: 105, Y: 100}}
			outside := world.Monster{UnitID: 9, NPCID: world.ArcaneGhoulLord, Position: world.Position{X: 100 + uint32(p.postBossCleanupRadiusTiles()) + 1, Y: 100}}
			w.Monsters = []world.Monster{outside, inside}
			deps := Deps{Combat: combat, RouteClear: exec, Profile: hooks}
			for i := 0; i < p.postBossCleanupMaxCasts(); i++ {
				w.At = w.At.Add(2 * time.Second)
				got := p.onBossTick(context.Background(), deps, pipelineStepClearNearbyHostiles, w, w.At)
				if got.failed || got.complete || combat.lastMonsterUnitID != inside.UnitID || combat.lastSkillID != memory.MustSkillID("blizzard") {
					t.Fatalf("cleanup %d=%+v combat=%+v", i, got, combat)
				}
			}
			got := p.onBossTick(context.Background(), deps, pipelineStepClearNearbyHostiles, w, w.At.Add(time.Second))
			if got.failed || !got.complete {
				t.Fatalf("bounded cleanup=%+v", got)
			}
			p.resetPostBossCleanup()
			combat.castMonsterErr = nil
			if run == RunIDNihlathak {
				combat.castMonsterErr = profile.ErrRouteClearTargetUnprojectable
				w.At = w.At.Add(2 * time.Second)
				if got := p.onBossTick(context.Background(), deps, pipelineStepClearNearbyHostiles, w, w.At); got.failed {
					t.Fatal(got)
				}
				if !p.boss.cleanupSkippedUnitIDs[inside.UnitID] {
					t.Fatal("unprojectable target was not skipped")
				}
			} else {
				w.Monsters = []world.Monster{outside}
			}
			for i := 1; i <= 3; i++ {
				w.At = w.At.Add(100 * time.Millisecond)
				got := p.onBossTick(context.Background(), deps, pipelineStepClearNearbyHostiles, w, w.At)
				if got.failed || got.complete != (i == 3) {
					t.Fatalf("empty cleanup %d=%+v", i, got)
				}
			}
		})
	}
}

func TestBlizzardLowerKurastBlockerClearRetriesChestOnce(t *testing.T) {
	p, exec, _, combat, _ := blizzardTaskFixture(t, RunIDLowerKurast)
	rack := closedObject(world.ArmorStand1ID, world.ObjectKindRack, 127, 5012, 2983)
	rack.Mode = world.ObjectModeOpened
	w := lowerKurastSweepState([]world.Object{closedObject(world.JungleChest2ID, world.ObjectKindSuperChest, 126, 5027, 3012), rack}, 5)
	w.Player.Position = world.Position{X: 5027, Y: 3012}
	w.Monsters = []world.Monster{{UnitID: 77, NPCID: world.Zakarumite, Position: world.Position{X: 5028, Y: 3012}, IsHovered: true}}
	chest := &blockerChestOperate{world: &w}
	trace := &pipelineTelemetry{}
	deps := pipelineChestDeps{Chest: chest, Combat: combat, Loot: &mockLootActions{}, RouteClear: exec, Telemetry: trace}
	base := time.Now().Add(time.Hour)
	var got stepResult
	for i := 0; i < 100; i++ {
		w.At = base.Add(time.Duration(i) * 100 * time.Millisecond)
		got = p.tickChestSweep(context.Background(), deps, w, w.At)
		if combat.castCalls > 0 {
			w.Monsters = nil
		}
		if got.failed || got.complete {
			break
		}
	}
	if got.failed || !got.complete || combat.castCalls != 1 || combat.lastSkillID != memory.MustSkillID("blizzard") || countChestEvents(trace, telemetry.ChestOpened) != 1 || countChestEvents(trace, telemetry.ChestSkipped) != 0 {
		t.Fatalf("sweep=%+v combat=%+v events=%+v", got, combat, trace.events)
	}
}

type blizzardDoneRoute struct{ mockRoutePlayback }

func (r *blizzardDoneRoute) Tick(context.Context, world.State) (bool, error) {
	r.tickCalls++
	return true, nil
}

func TestBlizzardSummonerAndCowSweepWaitForClearAndCompleteCoverage(t *testing.T) {
	for _, run := range []RunID{RunIDSummoner, RunIDCows} {
		t.Run(string(run), func(t *testing.T) {
			p, exec, hooks, combat, w := blizzardTaskFixture(t, run)
			cfg := phase17ThreatConfig()
			cfg.Enabled = true
			cfg.NoProgressTimeout = 12 * time.Second
			p.core.routeCombat = cfg
			p.core.routeID = "blizzard-test"
			p.core.suppressRouteLoot = true
			route := &blizzardDoneRoute{mockRoutePlayback: mockRoutePlayback{progressOK: true, progress: RouteProgress{RouteID: "blizzard-test", Mode: RouteProgressTransition}}}
			deps := Deps{Route: route, RouteClear: exec, Combat: combat, Profile: hooks}
			var cow *cowPipeline
			if run == RunIDCows {
				cow = newCowPipeline(p.definition, RunConfig{RouteID: "blizzard-test", SetupRouteID: "leg-test", Combat: p.core.combat, RouteCombat: cfg})
				cow.cowSweep.travel.fieldReadyComplete = true
				cow.cowSweep.core.suppressRouteLoot = true
				if cow.legRoute.core.routeCombat.Enabled || cow.config.Combat.UseCorpseExplosion {
					t.Fatal("Cow setup activated combat or CE")
				}
			}
			tick := func() stepResult {
				w.At = w.At.Add(100 * time.Millisecond)
				if cow != nil {
					return cow.onTick(context.Background(), deps, cowStepSweep, w, w.At, w.At, 0)
				}
				return p.onTravelTick(context.Background(), deps, pipelineStepPlayRoute, w, w.At, w.At)
			}
			npc := world.ArcaneSpecter
			secondX := uint32(110)
			if run == RunIDCows {
				npc = world.HellBovine
				secondX = 125
			}
			w.Monsters = []world.Monster{{UnitID: 7, NPCID: npc, Position: world.Position{X: 105, Y: 100}}, {UnitID: 8, NPCID: npc, Position: world.Position{X: secondX, Y: 100}}}
			for len(w.Monsters) > 0 {
				got := tick()
				if got.failed || got.complete || route.tickCalls != 0 || combat.lastSkillID != memory.MustSkillID("blizzard") {
					t.Fatalf("threat hold=%+v route=%+v combat=%+v", got, route, combat)
				}
				w.Monsters = w.Monsters[1:]
			}
			w.MonsterCoverage = world.MonsterCoverage{MonstersTruncated: true, MonsterCoverageRadiusTiles: 1}
			for i := 0; i < 4; i++ {
				if got := tick(); got.failed || got.complete || route.tickCalls != 0 {
					t.Fatalf("incomplete coverage=%+v", got)
				}
			}
			w.MonsterCoverage = world.MonsterCoverage{}
			for i := 0; i < 2; i++ {
				if got := tick(); got.failed || got.complete || route.tickCalls != 0 {
					t.Fatalf("premature resume=%+v", got)
				}
			}
			got := tick()
			if !got.complete {
				got = tick()
			}
			if got.failed || !got.complete || route.tickCalls != 1 || combat.castCalls != 2 {
				t.Fatalf("resume=%+v ticks=%d casts=%v", got, route.tickCalls, combat.castSkills)
			}
		})
	}
}
