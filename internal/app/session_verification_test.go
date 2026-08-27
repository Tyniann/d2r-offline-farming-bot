package app

import (
	"errors"
	"testing"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/pathing"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

func sessionTownState(at time.Time, character string) world.State {
	return world.State{
		At: at, Valid: true, Phase: world.GamePhaseInGame,
		Area:     world.Area{ID: world.RogueEncampment, Name: "Rogue Encampment"},
		Identity: world.GameIdentity{Valid: true, CharacterName: character, Class: world.CharacterClassNecromancer},
	}
}

func TestSessionGameVerifierRequiresResetAndThreeFreshTicks(t *testing.T) {
	verifier := newSessionGameVerifier(sessionGameExpectation{Character: "MrBones", GameVersion: "3.2.92777", StartArea: world.RogueEncampment})
	if _, _, err := verifier.Observe(sessionTownState(time.Now(), "MrBones"), "3.2.92777"); err == nil {
		t.Fatal("expected reset requirement")
	}
	verifier.ResetForNextGame()
	start := time.Now()
	for tick := 0; tick < sessionGameStableTicks; tick++ {
		verification, ready, err := verifier.Observe(sessionTownState(start.Add(time.Duration(tick)*time.Millisecond), "MrBones"), "3.2.92777")
		if err != nil {
			t.Fatal(err)
		}
		if ready != (tick == sessionGameStableTicks-1) {
			t.Fatalf("tick %d ready=%t", tick, ready)
		}
		if ready && verification.Generation != 1 {
			t.Fatalf("verification = %+v", verification)
		}
	}
	verifier.ResetForNextGame()
	if _, ready, err := verifier.Observe(sessionTownState(start.Add(time.Second), "MrBones"), "3.2.92777"); err != nil || ready {
		t.Fatalf("new generation reused old stability: ready=%t err=%v", ready, err)
	}
}

func TestSessionGameVerifierWaitsForMenuAndUnknownInsteadOfFailing(t *testing.T) {
	verifier := newSessionGameVerifier(sessionGameExpectation{Character: "MrBones", GameVersion: "3.2.92777", StartArea: world.RogueEncampment})
	verifier.ResetForNextGame()
	menu := world.State{At: time.Now(), Valid: true, Phase: world.GamePhaseMenu, Identity: world.GameIdentity{Valid: true, CharacterName: "MrBones"}}
	if _, ready, err := verifier.Observe(menu, "3.2.92777"); err != nil || ready {
		t.Fatalf("menu should wait: ready=%t err=%v", ready, err)
	}
	unknown := world.State{At: time.Now(), Valid: true, Phase: world.GamePhaseUnknown}
	if _, ready, err := verifier.Observe(unknown, "3.2.92777"); err != nil || ready {
		t.Fatalf("unknown should wait: ready=%t err=%v", ready, err)
	}
	zeroArea := sessionTownState(time.Now(), "MrBones")
	zeroArea.Area = world.Area{}
	if _, ready, err := verifier.Observe(zeroArea, "3.2.92777"); err != nil || ready {
		t.Fatalf("area 0 should wait: ready=%t err=%v", ready, err)
	}
}

func TestSessionGameVerifierRejectsWrongContextBeforeRun(t *testing.T) {
	tests := []struct {
		name    string
		state   world.State
		version string
	}{
		{"character", sessionTownState(time.Now(), "MrHammer"), "3.2.92777"},
		{"version", sessionTownState(time.Now(), "MrBones"), "old"},
		{"fresh act 3 start", func() world.State {
			s := sessionTownState(time.Now(), "MrBones")
			s.Area = world.Area{ID: world.KurastDocks, Name: "Kurast Docks"}
			return s
		}(), "3.2.92777"},
		{"ui", func() world.State { s := sessionTownState(time.Now(), "MrBones"); s.UI.StashOpen = true; return s }(), "3.2.92777"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			verifier := newSessionGameVerifier(sessionGameExpectation{Character: "MrBones", GameVersion: "3.2.92777", StartArea: world.RogueEncampment})
			verifier.ResetForNextGame()
			if _, _, err := verifier.Observe(tc.state, tc.version); err == nil {
				t.Fatal("expected context mismatch")
			}
		})
	}
}

func TestFreshSessionGameVerifierAcceptsEveryTownAndIgnoresStickyWaypointBit(t *testing.T) {
	areas := []world.AreaID{
		world.RogueEncampment,
		world.LutGholein,
		world.KurastDocks,
		world.ThePandemoniumFortress,
		world.Harrogath,
	}
	for _, area := range areas {
		t.Run(world.LookupArea(area).Name, func(t *testing.T) {
			verifier := newSessionGameVerifier(sessionGameExpectation{
				Character: "MrBones", GameVersion: "3.2.92777", AllowedStartAreas: areas,
			})
			verifier.ResetForNextGame()
			start := time.Now()
			for tick := 0; tick < sessionGameStableTicks; tick++ {
				state := sessionTownState(start.Add(time.Duration(tick)*time.Millisecond), "MrBones")
				state.Area = world.LookupArea(area)
				state.UI.WaypointOpen = true
				verification, ready, err := verifier.Observe(state, "3.2.92777")
				if err != nil {
					t.Fatal(err)
				}
				if ready != (tick == sessionGameStableTicks-1) {
					t.Fatalf("tick %d ready=%t", tick, ready)
				}
				if ready && verification.AreaID != area {
					t.Fatalf("verified area=%s want=%s", verification.AreaID, area)
				}
			}
		})
	}
}

func TestFreshSessionGameVerifierResetsStabilityWhenTownChanges(t *testing.T) {
	verifier := newSessionGameVerifier(sessionGameExpectation{
		Character: "MrBones", GameVersion: "3.2.92777",
		AllowedStartAreas: []world.AreaID{world.RogueEncampment, world.LutGholein},
	})
	verifier.ResetForNextGame()
	start := time.Now()
	rogue := sessionTownState(start, "MrBones")
	if _, ready, err := verifier.Observe(rogue, "3.2.92777"); err != nil || ready {
		t.Fatalf("first town tick ready=%t err=%v", ready, err)
	}
	lut := sessionTownState(start.Add(time.Millisecond), "MrBones")
	lut.Area = world.LookupArea(world.LutGholein)
	if _, ready, err := verifier.Observe(lut, "3.2.92777"); err != nil || ready {
		t.Fatalf("changed town tick ready=%t err=%v", ready, err)
	}
	for tick := 0; tick < sessionGameStableTicks-1; tick++ {
		lut.At = lut.At.Add(time.Millisecond)
		_, ready, err := verifier.Observe(lut, "3.2.92777")
		if err != nil {
			t.Fatal(err)
		}
		if ready != (tick == sessionGameStableTicks-2) {
			t.Fatalf("stable tick %d ready=%t", tick, ready)
		}
	}
}

func TestVerifySessionRouteStartRejectsFreshLayoutMismatch(t *testing.T) {
	route, err := pathing.LoadRoute("../../configs/routes/farming/mrbones/nightmare/black-marsh-cellar5-nightmare-mrbones.yaml")
	if err != nil {
		t.Fatal(err)
	}
	start := route.Segments[0].Points[0]
	state := world.State{
		Valid: true, Phase: world.GamePhaseInGame, Area: world.Area{ID: world.BlackMarsh},
		Player:   world.Player{Position: world.Position{X: start.X, Y: start.Y}},
		Identity: world.GameIdentity{Valid: true, CharacterName: "MrBones", Class: world.CharacterClassNecromancer},
		Objects:  []world.Object{{Kind: world.ObjectKindWaypoint, ID: 119, Position: world.Position{X: start.X + 50, Y: start.Y + 50}}},
	}
	_, err = verifySessionRouteStart(route, state, "3.2.92777")
	if !errors.Is(err, pathing.ErrRouteLayoutMismatch) {
		t.Fatalf("err = %v, want layout mismatch", err)
	}
}

type resetCounter struct{ calls int }

func (r *resetCounter) Reset() { r.calls++ }

func TestSessionResetBarrierClearsEveryComponentAndWorld(t *testing.T) {
	navigator, route, loot := &resetCounter{}, &resetCounter{}, &resetCounter{}
	worldCalls := 0
	barrier := sessionResetBarrier{
		components: []sessionNamedResetter{{name: "navigator", resetter: navigator}, {name: "route", resetter: route}, {name: "loot", resetter: loot}},
		resetWorld: func(time.Time, string) { worldCalls++ },
	}
	for cycle := 0; cycle < 3; cycle++ {
		if err := barrier.ResetForNextGame(time.Now(), "next_game"); err != nil {
			t.Fatal(err)
		}
	}
	if navigator.calls != 3 || route.calls != 3 || loot.calls != 3 || worldCalls != 3 {
		t.Fatalf("resets navigator=%d route=%d loot=%d world=%d", navigator.calls, route.calls, loot.calls, worldCalls)
	}
}
