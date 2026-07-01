package world

import (
	"io"
	"log/slog"
	"testing"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/memory"
)

func TestModelCurrentSliceIndependence(t *testing.T) {
	m := NewModel(slog.New(slog.NewTextHandler(io.Discard, nil)))
	snap := validSnapshot()
	snap.Objects = []memory.ObjectUnit{{TxtFileNo: 119, UnitID: 42, PosX: 1, PosY: 2}}
	m.Update(snap)

	c1 := m.Current()
	c1.Objects[0].UnitID = 0

	c2 := m.Current()
	if c2.Objects[0].UnitID == 0 {
		t.Fatal("mutating Current() slice should not affect stored state")
	}
	if len(m.Current().Objects) != 1 || m.Current().Objects[0].UnitID != 42 {
		t.Fatal("model should retain original object unit id")
	}
}

func TestModelUpdateClonesEntitySlices(t *testing.T) {
	m := NewModel(slog.New(slog.NewTextHandler(io.Discard, nil)))
	snap := validSnapshot()
	snap.Monsters = []memory.MonsterUnit{{NPCID: 51, UnitID: 7}}
	got := m.Update(snap)
	got.Monsters[0].UnitID = 99
	if m.Current().Monsters[0].UnitID != 7 {
		t.Fatal("mutating Update return monsters should not affect model")
	}
}
