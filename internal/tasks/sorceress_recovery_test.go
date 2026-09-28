package tasks

import (
	"context"
	"testing"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/profile"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

func TestSorceressRouteApproachOwnsSkillSelectionUntilMovement(t *testing.T) {
	p, _, _, combat, w := blizzardTaskFixture(t, RunIDCows)
	p.core.routeID, p.core.routeCombat = "cow", phase17ThreatConfig()
	p.core.routeCombat.NoProgressTimeout = 12 * time.Second
	cow := world.Monster{NPCID: world.HellBovine, UnitID: 7, Position: world.Position{X: 100, Y: 125}}
	w.Monsters = []world.Monster{cow}
	progress := phase17ThreatProgress()
	route := controllerRoute(progress)
	clear := &routeClearMock{result: profile.Result{Status: profile.StatusPending, Reason: profile.RouteClearReasonTargetUnprojectable}}
	ok := true
	combat.farthestOK, combat.farthestPosition, combat.farthestDistance = &ok, world.Position{X: 100, Y: 105}, 20
	combat.teleportSent = []bool{false, true}
	deps := Deps{Route: route, RouteClear: clear, Combat: combat}
	base := w.At
	for i := 0; i < 4; i++ {
		w.At = base.Add(time.Duration(i) * 100 * time.Millisecond)
		if got := p.onTravelTick(context.Background(), deps, pipelineStepPlayRoute, w, w.At, base); got.failed {
			t.Fatalf("tick %d: %+v", i, got)
		}
	}
	if combat.teleportCalls != 2 || len(clear.requests) != 3 || !p.travel.routeApproachPending {
		t.Fatalf("attack interrupted Teleport selection: teleports=%d attacks=%d pending=%v", combat.teleportCalls, len(clear.requests), p.travel.routeApproachPending)
	}
	w.At = base.Add(400 * time.Millisecond)
	w.Player.Position = world.Position{X: 100, Y: 105}
	if got := p.onTravelTick(context.Background(), deps, pipelineStepPlayRoute, w, w.At, base); got.failed || p.travel.routeApproachPending {
		t.Fatalf("movement confirmation: %+v pending=%v", got, p.travel.routeApproachPending)
	}
}

func TestSorceressRouteIdleTriggersAtTwoSecondsDespiteMercenaryKills(t *testing.T) {
	p, _, _, _, w := blizzardTaskFixture(t, RunIDCows)
	cfg, progress := phase17ThreatConfig(), phase17ThreatProgress()
	cfg.NoProgressTimeout = 12 * time.Second
	route, clear := controllerRoute(progress), &routeClearMock{}
	w.Monsters = cowLivingPack(3)
	var controller RouteThreatController
	base := w.At
	for _, ms := range []int{0, 1900, 2000} {
		w.At = base.Add(time.Duration(ms) * time.Millisecond)
		w.MonsterCoverage.EligibleMonsterCount = 100 - ms/100
		assessment := assessThreats(w, progress, p.definition.RouteHostileNPCIDs, cfg)
		got := controller.Tick(context.Background(), route, clear, w, progress, assessment, p.definition, cfg, p.core.combat, w.At)
		if got.Failed || got.AllowMovement || got.StopAttack != (ms == 2000) {
			t.Fatalf("%dms idle recovery: %+v", ms, got)
		}
	}
}

func TestSorceressIdleRetargetsThenBoundsUnsafeRecovery(t *testing.T) {
	p, _, _, combat, w := blizzardTaskFixture(t, RunIDCows)
	p.core.routeID, p.core.routeCombat = "cow", phase17ThreatConfig()
	p.core.routeCombat.NoProgressTimeout = 12 * time.Second
	w.Monsters = cowLivingPack(3)
	route, clear := controllerRoute(phase17ThreatProgress()), &routeClearMock{}
	deps := Deps{Route: route, RouteClear: clear, Combat: combat}
	base := w.At
	for _, ms := range []int{0, 1999, 2000, 2100, 4000, 4100, 6000} {
		w.At = base.Add(time.Duration(ms) * time.Millisecond)
		w.MonsterCoverage.EligibleMonsterCount = 100 - ms/100
		got := p.onTravelTick(context.Background(), deps, pipelineStepPlayRoute, w, w.At, base)
		if got.failed != (ms == 6000) || got.complete {
			t.Fatalf("%dms: %+v", ms, got)
		}
		if ms == 2100 && clear.requests[len(clear.requests)-1].Target.UnitID == w.Monsters[0].UnitID {
			t.Fatal("idle retarget repeated the ineffective target")
		}
	}
	if combat.teleportCalls != 0 || route.tickCalls != 0 {
		t.Fatalf("unsafe landing or uncleared route accepted: teleports=%d route=%d", combat.teleportCalls, route.tickCalls)
	}
}

func TestSorceressIdleAllowsAttacksAndManaRecovery(t *testing.T) {
	for _, recovery := range []bool{false, true} {
		t.Run(map[bool]string{false: "attacks", true: "mana"}[recovery], func(t *testing.T) {
			p, _, _, _, w := blizzardTaskFixture(t, RunIDCows)
			cfg, progress := phase17ThreatConfig(), phase17ThreatProgress()
			cfg.NoProgressTimeout, cfg.ManaRecoveryTimeout = 12*time.Second, 30*time.Second
			cfg.TeleportManaReservePercent, cfg.ResumeManaPercent = 30, 60
			clear := &routeClearMock{result: profile.Result{Status: profile.StatusAction, ActionKind: profile.RouteClearActionAttack}}
			if recovery {
				clear.result = profile.Result{Status: profile.StatusPending}
			}
			w.Monsters = cowLivingPack(1)
			route := controllerRoute(progress)
			var controller RouteThreatController
			base := w.At
			for i := 0; i < 8; i++ {
				w.At = base.Add(time.Duration(i) * time.Second)
				if recovery {
					w.Player.Mana = 10
				}
				assessment := assessThreats(w, progress, p.definition.RouteHostileNPCIDs, cfg)
				controller.ObserveResources(w, assessment, cfg, w.At)
				got := controller.Tick(context.Background(), route, clear, w, progress, assessment, p.definition, cfg, p.core.combat, w.At)
				if got.Failed || got.StopAttack {
					t.Fatalf("second %d: %+v", i, got)
				}
			}
		})
	}
}

type selectiveAimCombat struct{ mockCombatActions }

func (*selectiveAimCombat) MonsterAimProjectable(_ world.Position, target world.Position) bool {
	return target.Y == 100
}

func TestSorceressAimPreferenceKeepsThreatsAndDropsDeadTargets(t *testing.T) {
	p, _, _, _, w := blizzardTaskFixture(t, RunIDCows)
	cfg, progress := phase17ThreatConfig(), phase17ThreatProgress()
	w.Monsters = []world.Monster{
		{NPCID: world.HellBovine, UnitID: 7, Position: world.Position{X: 101, Y: 101}},
		{NPCID: world.HellBovine, UnitID: 8, Position: world.Position{X: 110, Y: 100}},
		{NPCID: world.HellBovine, UnitID: 9, Position: world.Position{X: 112, Y: 100}},
	}
	combat := &selectiveAimCombat{}
	assessment := assessThreats(w, progress, p.definition.RouteHostileNPCIDs, cfg)
	target, mode, found := selectAimableRouteTarget(w, progress, assessment, p.definition.RouteHostileNPCIDs, cfg, combat, 0)
	if !found || target.UnitID != 8 || mode != profile.RouteClearThreat || assessment.RelevantThreatCount != 3 {
		t.Fatalf("projectable preference: %+v %v %v assessment=%+v", target, mode, found, assessment)
	}
	// Blizzard killed the previously aimable monster between two Ice Blast ticks.
	w.Monsters = append(w.Monsters[:1], w.Monsters[2:]...)
	assessment = assessThreats(w, progress, p.definition.RouteHostileNPCIDs, cfg)
	target, _, found = selectAimableRouteTarget(w, progress, assessment, p.definition.RouteHostileNPCIDs, cfg, combat, 0)
	if !found || target.UnitID != 9 {
		t.Fatalf("dead target reused: %+v", target)
	}
	w.Monsters = w.Monsters[:1]
	assessment = assessThreats(w, progress, p.definition.RouteHostileNPCIDs, cfg)
	_, _, found = selectAimableRouteTarget(w, progress, assessment, p.definition.RouteHostileNPCIDs, cfg, combat, 0)
	if found || !assessment.RouteTargetFound {
		t.Fatal("off-screen blocker was discarded")
	}
}

func TestSorceressPendingApproachRequiresFreshLiveTargetAndHasDeadline(t *testing.T) {
	for _, lost := range []bool{false, true} {
		t.Run(map[bool]string{false: "deadline", true: "target_died"}[lost], func(t *testing.T) {
			p, _, _, combat, w := blizzardTaskFixture(t, RunIDCows)
			p.core.routeID, p.core.routeCombat = "cow", phase17ThreatConfig()
			p.core.routeCombat.NoProgressTimeout = 12 * time.Second
			cow := world.Monster{NPCID: world.HellBovine, UnitID: 7, Position: world.Position{X: 100, Y: 125}}
			w.Monsters = []world.Monster{cow}
			ok := true
			combat.farthestOK, combat.farthestPosition, combat.farthestDistance = &ok, world.Position{X: 100, Y: 105}, 20
			combat.teleportSent = []bool{false, false}
			progress := phase17ThreatProgress()
			route, clear := controllerRoute(progress), &routeClearMock{}
			deps := Deps{Combat: combat, Route: route, RouteClear: clear}
			base := w.At
			if got := p.tickRouteThreatApproach(narrowTravelDeps(deps), w, progress, cow, w.At); got.failed {
				t.Fatal(got)
			}
			if got := p.onTravelTick(context.Background(), deps, pipelineStepPlayRoute, w, w.At, base); got.failed || combat.teleportCalls != 1 || len(clear.requests) != 0 {
				t.Fatalf("stale snapshot: %+v", got)
			}
			w.At = base.Add(2 * time.Second)
			if lost {
				w.Monsters = nil
			}
			got := p.onTravelTick(context.Background(), deps, pipelineStepPlayRoute, w, w.At, base)
			if got.failed == lost || combat.teleportCalls != 1 || len(clear.requests) != 0 {
				t.Fatalf("pending recovery: %+v", got)
			}
			if lost && !p.travel.routeApproachSelectingAt.IsZero() {
				t.Fatal("dead recovery target retained")
			}
		})
	}
}

func TestSorceressPendingApproachWaitsForManaWithBoundedRecovery(t *testing.T) {
	p, _, _, combat, w := blizzardTaskFixture(t, RunIDCows)
	p.core.routeID, p.core.routeCombat = "cow", phase17ThreatConfig()
	p.core.routeCombat.NoProgressTimeout = 12 * time.Second
	p.core.routeCombat.ManaRecoveryTimeout = 5 * time.Second
	p.core.routeCombat.TeleportManaReservePercent, p.core.routeCombat.ResumeManaPercent = 30, 60
	cow := world.Monster{NPCID: world.HellBovine, UnitID: 7, Position: world.Position{X: 100, Y: 125}}
	w.Monsters = []world.Monster{cow}
	w.Player.Mana = 10
	base := w.At
	p.travel.routeApproachTargetUnitID = cow.UnitID
	p.travel.routeApproachSelectingAt = base
	p.travel.routeApproachSnapshotAt = base
	deps := Deps{Combat: combat, Route: controllerRoute(phase17ThreatProgress()), RouteClear: &routeClearMock{}, Profile: &mockProfileActions{}}
	for _, seconds := range []int{1, 4, 6} {
		w.At = base.Add(time.Duration(seconds) * time.Second)
		got := p.onTravelTick(context.Background(), deps, pipelineStepPlayRoute, w, w.At, base)
		if got.failed != (seconds == 6) || combat.teleportCalls != 0 {
			t.Fatalf("second %d: %+v teleports=%d", seconds, got, combat.teleportCalls)
		}
		if seconds == 6 && got.reason != string(RouteThreatReasonManaRecoveryFailed) {
			t.Fatalf("wrong deadline: %+v", got)
		}
	}
}
