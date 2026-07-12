package world

import "testing"

func TestFindSuperUniqueNearestWithFlag(t *testing.T) {
	state := State{
		Valid: true,
		Player: Player{
			Position: Position{X: 100, Y: 100},
		},
		Monsters: []Monster{
			{NPCID: DarkStalker, UnitID: 1, Position: Position{X: 200, Y: 200}, MonsterTypeFlag: SuperUniqueMonsterFlag},
			{NPCID: DarkStalker, UnitID: 2, Position: Position{X: 110, Y: 110}, MonsterTypeFlag: SuperUniqueMonsterFlag},
			{NPCID: DarkStalker, UnitID: 3, Position: Position{X: 105, Y: 105}, MonsterTypeFlag: 0},
		},
	}

	got, ok := state.FindSuperUnique(DarkStalker)
	if !ok {
		t.Fatal("expected super unique match")
	}
	if got.UnitID != 2 {
		t.Fatalf("UnitID = %d, want nearest super-unique 2", got.UnitID)
	}
}

func TestFindSuperUniqueIgnoresWrongFlag(t *testing.T) {
	state := State{
		Valid:  true,
		Player: Player{Position: Position{X: 0, Y: 0}},
		Monsters: []Monster{
			{NPCID: DarkStalker, UnitID: 1, MonsterTypeFlag: 0},
		},
	}
	if _, ok := state.FindSuperUnique(DarkStalker); ok {
		t.Fatal("expected no super-unique without flag 10")
	}
}

func TestFindSuperUniqueAnyNPCID(t *testing.T) {
	state := State{
		Valid:  true,
		Player: Player{Position: Position{X: 0, Y: 0}},
		Monsters: []Monster{
			{NPCID: 52, UnitID: 9, Position: Position{X: 5, Y: 0}, MonsterTypeFlag: SuperUniqueMonsterFlag},
		},
	}
	got, ok := state.FindSuperUnique(0)
	if !ok || got.UnitID != 9 {
		t.Fatalf("FindSuperUnique(0) = %+v, ok=%v", got, ok)
	}
}

func TestNearestObjectWaypoint(t *testing.T) {
	state := State{
		Valid:  true,
		Player: Player{Position: Position{X: 0, Y: 0}},
		Objects: []Object{
			{Kind: ObjectKindWaypoint, UnitID: 1, Position: Position{X: 50, Y: 0}},
			{Kind: ObjectKindWaypoint, UnitID: 2, Position: Position{X: 10, Y: 0}},
		},
	}
	got, ok := state.NearestObject(ObjectKindWaypoint)
	if !ok || got.UnitID != 2 {
		t.Fatalf("nearest waypoint = %+v, ok=%v", got, ok)
	}
}
