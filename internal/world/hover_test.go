package world

import (
	"testing"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/memory"
)

func TestHoverInfoMatches(t *testing.T) {
	h := HoverInfo{IsHovered: true, UnitType: HoverUnitTypeObject, UnitID: 42}
	if !h.Matches(HoverUnitTypeObject, 42) {
		t.Fatal("expected match for same type and ID")
	}
	if h.Matches(HoverUnitTypeEntrance, 42) {
		t.Fatal("type mismatch must not match")
	}
	if h.Matches(HoverUnitTypeObject, 43) {
		t.Fatal("ID mismatch must not match")
	}
	if (HoverInfo{}).Matches(HoverUnitTypeObject, 42) {
		t.Fatal("not-hovered must never match")
	}
}

func TestFromSnapshotHoverEntityMatching(t *testing.T) {
	snap := memory.Snapshot{
		Valid:  true,
		Phase:  memory.GamePhaseInGame,
		AreaID: 1,
		Objects: []memory.ObjectUnit{
			{TxtFileNo: 119, UnitID: 10, PosX: 100, PosY: 100},
		},
		Entrances: []memory.EntranceUnit{
			{TxtFileNo: 10, UnitID: 20, PosX: 110, PosY: 110},
		},
		Monsters: []memory.MonsterUnit{
			{NPCID: 45, UnitID: 20, PosX: 120, PosY: 120},
		},
		Hover: memory.HoverState{IsHovered: true, UnitType: memory.HoverUnitTypeEntrance, UnitID: 20},
	}

	st := FromSnapshot(snap)
	if !st.Hover.IsHovered || st.Hover.UnitType != HoverUnitTypeEntrance || st.Hover.UnitID != 20 {
		t.Fatalf("State.Hover = %+v, want entrance unit 20", st.Hover)
	}
	if !st.Entrances[0].IsHovered {
		t.Fatal("entrance with matching UnitID must be hovered")
	}
	// Same UnitID on a monster must not match because the unit type differs.
	if st.Monsters[0].IsHovered {
		t.Fatal("monster must not be hovered despite matching UnitID")
	}
	if st.Objects[0].IsHovered {
		t.Fatal("object must not be hovered")
	}
}

func TestFromSnapshotNoHover(t *testing.T) {
	snap := memory.Snapshot{
		Valid:  true,
		Phase:  memory.GamePhaseInGame,
		AreaID: 1,
		Objects: []memory.ObjectUnit{
			{TxtFileNo: 119, UnitID: 10},
		},
	}
	st := FromSnapshot(snap)
	if st.Hover.IsHovered {
		t.Fatalf("State.Hover = %+v, want not hovered", st.Hover)
	}
	if st.Objects[0].IsHovered {
		t.Fatal("object must not be hovered without hover data")
	}
}
