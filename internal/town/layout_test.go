package town

import (
	"testing"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

func layoutState(stash, waypoint world.Position) world.State {
	return world.State{Valid: true, Phase: world.GamePhaseInGame, Area: world.LookupArea(world.RogueEncampment), Objects: []world.Object{{UnitID: 1, Kind: world.ObjectKindPersonalStash, Position: stash}, {UnitID: 2, Kind: world.ObjectKindWaypoint, Position: waypoint}}, Monsters: []world.Monster{{NPCID: world.Akara, UnitID: 3, Position: world.Position{X: stash.X + 10, Y: stash.Y - 10}}, {NPCID: world.DeckardCain, UnitID: 4, Position: world.Position{X: stash.X + 20, Y: stash.Y - 20}}, {NPCID: world.Charsi, UnitID: 5, Position: world.Position{X: stash.X + 30, Y: stash.Y - 30}}}}
}

func TestTownLayoutFingerprintIsTranslationAndUnitIDIndependent(t *testing.T) {
	a, reason := InspectTownLayout(layoutState(world.Position{X: 100, Y: 100}, world.Position{X: 80, Y: 70}))
	if reason != "" {
		t.Fatal(reason)
	}
	bState := layoutState(world.Position{X: 500, Y: 400}, world.Position{X: 480, Y: 370})
	bState.Objects[0].UnitID, bState.Objects[1].UnitID = 99, 77
	bState.Player.Position = world.Position{X: 999, Y: 999}
	b, reason := InspectTownLayout(bState)
	if reason != "" || a.Hash != b.Hash || a.WaypointDeltaX != -20 || a.WaypointDeltaY != -30 {
		t.Fatalf("a=%+v b=%+v reason=%s", a, b, reason)
	}
}

func TestTownLayoutFingerprintSeparatesWaypointPreset(t *testing.T) {
	left, _ := InspectTownLayout(layoutState(world.Position{X: 100, Y: 100}, world.Position{X: 80, Y: 70}))
	right, _ := InspectTownLayout(layoutState(world.Position{X: 100, Y: 100}, world.Position{X: 130, Y: 70}))
	if left.Hash == right.Hash {
		t.Fatalf("left and right share hash %s", left.Hash)
	}
}

func TestTownLayoutFingerprintFailsClosedOnMissingOrAmbiguousAnchors(t *testing.T) {
	state := layoutState(world.Position{X: 100, Y: 100}, world.Position{X: 80, Y: 70})
	state.Objects = state.Objects[:1]
	if _, reason := InspectTownLayout(state); reason != ReasonTownLayoutUnavailable {
		t.Fatalf("missing reason=%s", reason)
	}
	state = layoutState(world.Position{X: 100, Y: 100}, world.Position{X: 80, Y: 70})
	state.Objects = append(state.Objects, world.Object{UnitID: 3, Kind: world.ObjectKindWaypoint, Position: world.Position{X: 130, Y: 70}})
	if _, reason := InspectTownLayout(state); reason != ReasonTownLayoutUnavailable {
		t.Fatalf("ambiguous reason=%s", reason)
	}
}

func TestTownLayoutFingerprintDoesNotRequireTownNPCs(t *testing.T) {
	state := layoutState(world.Position{X: 100, Y: 100}, world.Position{X: 80, Y: 70})
	state.Monsters = nil
	fingerprint, reason := InspectTownLayout(state)
	if reason != "" || fingerprint.Hash == "" || fingerprint.AkaraDeltaX != 0 || fingerprint.CainDeltaX != 0 || fingerprint.CharsiDeltaX != 0 {
		t.Fatalf("fingerprint=%+v reason=%s", fingerprint, reason)
	}
}
