package app

import (
	"context"
	"testing"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/pathing"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/town"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

type freshGameEgressStub struct {
	startAct    town.OriginAct
	startAnchor town.Anchor
	startCalls  int
	tickCalls   int
	tickDone    bool
}

func (s *freshGameEgressStub) StartFrom(act town.OriginAct, anchor town.Anchor, _ world.State) error {
	s.startAct, s.startAnchor = act, anchor
	s.startCalls++
	return nil
}

func (s *freshGameEgressStub) Tick(context.Context, world.State) (bool, error) {
	s.tickCalls++
	return s.tickDone, nil
}

func (*freshGameEgressStub) Reset() {}

type freshGameWaypointStub struct {
	townCalls   int
	selectCalls int
}

func (*freshGameWaypointStub) Reset() {}

func (s *freshGameWaypointStub) TickTownWaypoint(context.Context, world.State) pathing.WaypointActionResult {
	s.townCalls++
	return pathing.WaypointActionResult{Status: pathing.WaypointActionClicked, Done: true}
}

func (s *freshGameWaypointStub) SelectWaypointTarget(context.Context, world.State, pathing.WaypointTargetID, time.Time) pathing.WaypointActionResult {
	s.selectCalls++
	return pathing.WaypointActionResult{Status: pathing.WaypointActionClicked, Done: true}
}

func freshGameTownState(area world.AreaID) world.State {
	return world.State{
		At: time.Now(), Valid: true, Phase: world.GamePhaseInGame, Area: world.LookupArea(area),
		Identity: world.GameIdentity{Valid: true, CharacterName: "MrBones", Class: world.CharacterClassNecromancer},
	}
}

func TestFreshGameNormalizerUsesSpawnEgressForActsTwoThroughFive(t *testing.T) {
	tests := []struct {
		area world.AreaID
		act  town.OriginAct
	}{
		{world.RogueEncampment, town.OriginActUnknown},
		{world.LutGholein, town.OriginAct2},
		{world.KurastDocks, town.OriginAct3},
		{world.ThePandemoniumFortress, town.OriginAct4},
		{world.Harrogath, town.OriginAct5},
	}
	for _, tc := range tests {
		t.Run(world.LookupArea(tc.area).Name, func(t *testing.T) {
			egress := &freshGameEgressStub{}
			waypoint := &freshGameWaypointStub{}
			normalizer := newFreshGameNormalizer(egress, waypoint)
			done, err := normalizer.Start(freshGameTownState(tc.area))
			if err != nil {
				t.Fatal(err)
			}
			if tc.area == world.RogueEncampment {
				if !done || egress.startCalls != 0 {
					t.Fatalf("Act 1 no-op done=%t starts=%d", done, egress.startCalls)
				}
				return
			}
			if done || egress.startCalls != 1 || egress.startAct != tc.act || egress.startAnchor != town.AnchorSpawn {
				t.Fatalf("done=%t start=%d act=%s anchor=%s", done, egress.startCalls, egress.startAct, egress.startAnchor)
			}
		})
	}
}

func TestFreshGameNormalizerCompletesOnlyAfterMemoryConfirmedRogueEncampment(t *testing.T) {
	egress := &freshGameEgressStub{tickDone: true}
	waypoint := &freshGameWaypointStub{}
	normalizer := newFreshGameNormalizer(egress, waypoint)
	foreign := freshGameTownState(world.LutGholein)
	if done, err := normalizer.Start(foreign); err != nil || done {
		t.Fatalf("start done=%t err=%v", done, err)
	}
	if done, err := normalizer.Tick(t.Context(), foreign); err != nil || done {
		t.Fatalf("walk done=%t err=%v", done, err)
	}
	if done, err := normalizer.Tick(t.Context(), foreign); err != nil || done {
		t.Fatalf("waypoint done=%t err=%v", done, err)
	}
	menu := foreign
	menu.UI.WaypointOpen = true
	if done, err := normalizer.Tick(t.Context(), menu); err != nil || done {
		t.Fatalf("selection done=%t err=%v", done, err)
	}
	transition := foreign
	transition.Area = world.Area{}
	if done, err := normalizer.Tick(t.Context(), transition); err != nil || done {
		t.Fatalf("waypoint transition done=%t err=%v", done, err)
	}
	if done, err := normalizer.Tick(t.Context(), foreign); err != nil || done {
		t.Fatalf("foreign wait done=%t err=%v", done, err)
	}
	if done, err := normalizer.Tick(t.Context(), freshGameTownState(world.RogueEncampment)); err != nil || !done {
		t.Fatalf("rogue confirmation done=%t err=%v", done, err)
	}
	if egress.tickCalls != 1 || waypoint.townCalls != 1 || waypoint.selectCalls != 1 {
		t.Fatalf("calls egress=%d town=%d select=%d", egress.tickCalls, waypoint.townCalls, waypoint.selectCalls)
	}
}

func TestFreshGameNormalizerBlocksUnsafeContextBeforeAction(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*world.State)
	}{
		{name: "wrong area", mutate: func(state *world.State) { state.Area = world.LookupArea(world.KurastDocks) }},
		{name: "inventory", mutate: func(state *world.State) { state.UI.InventoryOpen = true }},
		{name: "npc", mutate: func(state *world.State) { state.UI.NPCInteractOpen = true }},
		{name: "shop", mutate: func(state *world.State) { state.UI.NPCShopOpen = true }},
		{name: "stash", mutate: func(state *world.State) { state.UI.StashOpen = true }},
		{name: "quit", mutate: func(state *world.State) { state.UI.QuitMenuOpen = true }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			egress := &freshGameEgressStub{}
			waypoint := &freshGameWaypointStub{}
			normalizer := newFreshGameNormalizer(egress, waypoint)
			foreign := freshGameTownState(world.LutGholein)
			if done, err := normalizer.Start(foreign); err != nil || done {
				t.Fatalf("start done=%t err=%v", done, err)
			}
			unsafe := foreign
			tc.mutate(&unsafe)
			if _, err := normalizer.Tick(t.Context(), unsafe); err == nil {
				t.Fatal("unsafe context accepted")
			}
			if egress.tickCalls != 0 || waypoint.townCalls != 0 || waypoint.selectCalls != 0 {
				t.Fatalf("action before rejection: egress=%d town=%d select=%d", egress.tickCalls, waypoint.townCalls, waypoint.selectCalls)
			}
		})
	}
}
