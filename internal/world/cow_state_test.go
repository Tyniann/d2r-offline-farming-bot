package world

import (
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/memory"
)

func cowTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestCowCorpseProjectionIsCurrentDisjointAndDefensivelyCloned(t *testing.T) {
	at := time.Now()
	snap := memory.Snapshot{
		At: at, Generation: 12, Valid: true, Phase: memory.GamePhaseInGame,
		Monsters:           []memory.MonsterUnit{{NPCID: HellBovine, UnitID: 1, PosX: 10, PosY: 11}},
		CowCorpsesComplete: true,
		CowCorpses:         []memory.CowCorpseUnit{{NPCID: CowKing, UnitID: 2, PosX: 20, PosY: 21, MonsterTypeFlag: SuperUniqueMonsterFlag, ConsumptionKnown: true}},
	}
	model := NewModel(cowTestLogger())
	state := model.Update(snap)
	corpse, ok := state.FindCurrentCowCorpse(2)
	if !ok || corpse.ObservedAt != at || corpse.SnapshotGeneration != 12 || corpse.Position != (Position{X: 20, Y: 21}) {
		t.Fatalf("corpse=%+v ok=%t", corpse, ok)
	}
	state.CowCorpses[0].UnitID = 99
	if got := model.Current().CowCorpses[0].UnitID; got != 2 {
		t.Fatalf("model corpse alias changed to %d", got)
	}
}

func TestCowCorpseProjectionRejectsLivingDuplicateAndStaleAuthority(t *testing.T) {
	at := time.Now()
	snap := memory.Snapshot{
		At: at, Generation: 3, Valid: true, Phase: memory.GamePhaseInGame,
		Monsters:           []memory.MonsterUnit{{NPCID: HellBovine, UnitID: 7, PosX: 10, PosY: 10}},
		CowCorpsesComplete: true,
		CowCorpses:         []memory.CowCorpseUnit{{NPCID: HellBovine, UnitID: 7, PosX: 10, PosY: 10, ConsumptionKnown: true}},
	}
	state := FromSnapshot(snap)
	if state.CowCorpsesComplete || len(state.CowCorpses) != 0 {
		t.Fatalf("inconsistent state=%+v", state)
	}

	valid := snap
	valid.Monsters = nil
	valid.CowCorpses[0].UnitID = 8
	state = FromSnapshot(valid)
	state.Generation++
	if _, ok := state.FindCurrentCowCorpse(8); ok {
		t.Fatal("stale corpse retained authority after generation changed")
	}
}

func TestCubeOpenUnavailableRevokesWorldState(t *testing.T) {
	model := NewModel(cowTestLogger())
	open := memory.Snapshot{At: time.Now(), Generation: 1, Valid: true, Phase: memory.GamePhaseInGame, UI: memory.UIState{CubeOpen: true, CubeOpenKnown: true}}
	if got := model.Update(open); !got.UI.CubeOpen || !got.UI.CubeOpenKnown {
		t.Fatalf("open UI=%+v", got.UI)
	}
	unavailable := open
	unavailable.Generation++
	unavailable.UI = memory.UIState{}
	if got := model.Update(unavailable); got.UI.CubeOpen || got.UI.CubeOpenKnown {
		t.Fatalf("unavailable UI retained authority: %+v", got.UI)
	}
}
