package pathing

import (
	"errors"
	"testing"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

func fingerprintState() world.State {
	return world.State{
		Valid:  true,
		Phase:  world.GamePhaseInGame,
		Area:   world.LookupArea(world.BlackMarsh),
		Player: world.Player{Position: world.Position{X: 100, Y: 200}},
		Objects: []world.Object{
			{Kind: world.ObjectKindWaypoint, ID: 119, UnitID: 10, Position: world.Position{X: 101, Y: 202}},
			{Kind: world.ObjectKindTownPortal, ID: 59, UnitID: 11, Position: world.Position{X: 99, Y: 199}},
		},
		Entrances: []world.Entrance{{ID: 10, UnitID: 12, Kind: world.EntranceKindWildernessToTower, Position: world.Position{X: 300, Y: 400}}},
	}
}

func TestBuildLayoutFingerprintStableAcrossOrderAndUnitIDs(t *testing.T) {
	a := fingerprintState()
	b := fingerprintState()
	b.Objects[0].UnitID = 999
	b.Entrances[0].UnitID = 998
	b.Objects[0], b.Objects[1] = b.Objects[1], b.Objects[0]
	fa, err := BuildLayoutFingerprint(a)
	if err != nil {
		t.Fatal(err)
	}
	fb, err := BuildLayoutFingerprint(b)
	if err != nil {
		t.Fatal(err)
	}
	if fa.Hash != fb.Hash || fa.AnchorCount != 2 {
		t.Fatalf("fingerprints differ: %+v vs %+v", fa, fb)
	}
}

func TestBuildLayoutFingerprintIgnoresPlayerPosition(t *testing.T) {
	a := fingerprintState()
	b := fingerprintState()
	b.Player.Position = world.Position{X: 125, Y: 225}
	fa, _ := BuildLayoutFingerprint(a)
	fb, _ := BuildLayoutFingerprint(b)
	if fa.Hash != fb.Hash {
		t.Fatal("player movement changed stable layout fingerprint")
	}
}

func TestBuildLayoutFingerprintChangesWithLayout(t *testing.T) {
	a := fingerprintState()
	b := fingerprintState()
	b.Objects[0].Position.X++
	fa, _ := BuildLayoutFingerprint(a)
	fb, _ := BuildLayoutFingerprint(b)
	if fa.Hash == fb.Hash {
		t.Fatal("layout change did not change fingerprint")
	}
}

func TestBuildLayoutFingerprintRequiresStableAnchor(t *testing.T) {
	state := fingerprintState()
	state.Objects = nil
	state.Entrances = nil
	_, err := BuildLayoutFingerprint(state)
	if !errors.Is(err, ErrLayoutAnchorsUnavailable) {
		t.Fatalf("error = %v", err)
	}
}
