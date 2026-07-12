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

func TestSessionGameVerifierRejectsWrongContextBeforeRun(t *testing.T) {
	tests := []struct {
		name    string
		state   world.State
		version string
	}{
		{"character", sessionTownState(time.Now(), "MrHammer"), "3.2.92777"},
		{"version", sessionTownState(time.Now(), "MrBones"), "old"},
		{"area", func() world.State {
			s := sessionTownState(time.Now(), "MrBones")
			s.Area = world.Area{ID: world.BlackMarsh, Name: "Black Marsh"}
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

func TestVerifySessionRouteStartRejectsFreshLayoutMismatch(t *testing.T) {
	route, err := pathing.LoadRoute("../../configs/routes/recordings/black-marsh-cellar5-nightmare-mrbones.yaml")
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
