package app

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/town"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

func layoutPinState(waypoint world.Position) world.State {
	return world.State{
		Valid: true, Phase: world.GamePhaseInGame, Area: world.LookupArea(world.RogueEncampment),
		Identity: world.GameIdentity{Valid: true, CharacterName: "MrBones", Class: world.CharacterClassNecromancer, MapSeed: 123},
		Objects: []world.Object{
			{UnitID: 1, Kind: world.ObjectKindPersonalStash, Position: world.Position{X: 100, Y: 100}},
			{UnitID: 2, Kind: world.ObjectKindWaypoint, Position: waypoint},
		},
	}
}

func TestTownLayoutPinSurvivesAnchorUnloadingAndRevalidates(t *testing.T) {
	pin := &townLayoutPin{}
	anchored := layoutPinState(world.Position{X: 80, Y: 70})
	want, reason, observed := pin.Resolve(anchored)
	if reason != "" || !observed || want.Hash == "" {
		t.Fatalf("initial resolve = %+v reason=%s observed=%t", want, reason, observed)
	}

	unloaded := anchored
	unloaded.Objects = nil
	got, reason, observed := pin.Resolve(unloaded)
	if reason != "" || observed || got.Hash != want.Hash || got.StashX != want.StashX || got.StashY != want.StashY {
		t.Fatalf("unloaded resolve = %+v reason=%s observed=%t", got, reason, observed)
	}

	changed := layoutPinState(world.Position{X: 130, Y: 80})
	if _, reason, observed := pin.Resolve(changed); reason != town.ReasonTownLayoutMismatch || !observed {
		t.Fatalf("changed resolve reason=%s observed=%t", reason, observed)
	}
}

func TestTownLayoutPinRejectsTranslatedOriginWithoutReset(t *testing.T) {
	pin := &townLayoutPin{}
	state := layoutPinState(world.Position{X: 80, Y: 70})
	if _, reason, _ := pin.Resolve(state); reason != "" {
		t.Fatal(reason)
	}
	state.Objects[0].Position = world.Position{X: 200, Y: 200}
	state.Objects[1].Position = world.Position{X: 180, Y: 170}
	if _, reason, observed := pin.Resolve(state); reason != town.ReasonTownLayoutMismatch || !observed {
		t.Fatalf("translated origin reason=%s observed=%t", reason, observed)
	}
}

func TestTownLayoutPinRejectsIdentityChangeAndReset(t *testing.T) {
	pin := &townLayoutPin{}
	state := layoutPinState(world.Position{X: 80, Y: 70})
	if _, reason, _ := pin.Resolve(state); reason != "" {
		t.Fatal(reason)
	}
	state.Objects = nil
	state.Identity.CharacterName = "Other"
	if _, reason, _ := pin.Resolve(state); reason != town.ReasonTownLayoutUnavailable {
		t.Fatalf("identity change reason=%s", reason)
	}
	pin.Reset()
	state.Identity.CharacterName = "MrBones"
	if _, reason, _ := pin.Resolve(state); reason != town.ReasonTownLayoutUnavailable {
		t.Fatalf("reset reason=%s", reason)
	}
}

func TestTownLayoutTestPinIsProcessIdentityAndTimeBound(t *testing.T) {
	state := layoutPinState(world.Position{X: 80, Y: 70})
	fingerprint, reason := town.InspectTownLayout(state)
	if reason != "" {
		t.Fatal(reason)
	}
	path := filepath.Join(t.TempDir(), "layout-pin.json")
	now := time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC)
	if err := saveTownLayoutTestPin(path, 42, state, fingerprint, now); err != nil {
		t.Fatal(err)
	}
	got, err := loadTownLayoutTestPin(path, 42, state, now.Add(time.Minute))
	if err != nil || got.Hash != fingerprint.Hash {
		t.Fatalf("load = %+v err=%v", got, err)
	}
	if _, err := loadTownLayoutTestPin(path, 43, state, now.Add(time.Minute)); err == nil {
		t.Fatal("different process accepted")
	}
	changed := state
	changed.Identity.MapSeed++
	if _, err := loadTownLayoutTestPin(path, 42, changed, now.Add(time.Minute)); err == nil {
		t.Fatal("different identity accepted")
	}
	if _, err := loadTownLayoutTestPin(path, 42, state, now.Add(townLayoutTestPinMaxAge+time.Second)); err == nil {
		t.Fatal("expired pin accepted")
	}
}
