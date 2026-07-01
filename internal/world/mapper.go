package world

import "github.com/Tyniann/d2r-offline-farming-bot/internal/memory"

// mapPhase converts memory.GamePhase to world.GamePhase. This is the only conversion site.
func mapPhase(phase memory.GamePhase) GamePhase {
	switch phase {
	case memory.GamePhaseMenu:
		return GamePhaseMenu
	case memory.GamePhaseLoading:
		return GamePhaseLoading
	case memory.GamePhaseInGame:
		return GamePhaseInGame
	default:
		return GamePhaseUnknown
	}
}

// FromSnapshot maps a memory snapshot into a semantic world State.
//
// Phase is always taken from snap.Phase via mapPhase, including when snap.Valid is false.
// Area and Player are populated only when snap.Valid is true; entity slices use empty
// non-nil slices when the probe did not enumerate entities.
func FromSnapshot(snap memory.Snapshot) State {
	phase := mapPhase(snap.Phase)

	objects := make([]Object, 0, len(snap.Objects))
	for _, o := range snap.Objects {
		objects = append(objects, mapObject(o))
	}
	entrances := make([]Entrance, 0, len(snap.Entrances))
	for _, e := range snap.Entrances {
		entrances = append(entrances, mapEntrance(e))
	}
	monsters := make([]Monster, 0, len(snap.Monsters))
	for _, m := range snap.Monsters {
		monsters = append(monsters, mapMonster(m))
	}

	if !snap.Valid {
		return State{
			At:        snap.At,
			Valid:     false,
			Reason:    snap.Reason,
			Phase:     phase,
			Objects:   objects,
			Entrances: entrances,
			Monsters:  monsters,
		}
	}

	return State{
		At:    snap.At,
		Valid: true,
		Phase: phase,
		Area:  LookupArea(AreaID(snap.AreaID)),
		Player: Player{
			Position: Position{X: snap.PosX, Y: snap.PosY},
			HP:       snap.HP,
			MaxHP:    snap.MaxHP,
			Mana:     snap.Mana,
			MaxMana:  snap.MaxMana,
		},
		Objects:   objects,
		Entrances: entrances,
		Monsters:  monsters,
	}
}

func mapObject(o memory.ObjectUnit) Object {
	name := LookupObjectName(o.TxtFileNo)
	if name == "" && LookupObjectKind(o.TxtFileNo) == ObjectKindWaypoint {
		name = "Waypoint"
	}
	return Object{
		Kind:     LookupObjectKind(o.TxtFileNo),
		ID:       o.TxtFileNo,
		UnitID:   o.UnitID,
		Position: Position{X: o.PosX, Y: o.PosY},
		Name:     name,
	}
}

func mapEntrance(e memory.EntranceUnit) Entrance {
	return Entrance{
		Kind:     LookupEntranceKind(e.TxtFileNo),
		ID:       e.TxtFileNo,
		UnitID:   e.UnitID,
		Position: Position{X: e.PosX, Y: e.PosY},
		Name:     LookupEntranceName(e.TxtFileNo),
	}
}

func mapMonster(m memory.MonsterUnit) Monster {
	return Monster{
		NPCID:           m.NPCID,
		UnitID:          m.UnitID,
		Position:        Position{X: m.PosX, Y: m.PosY},
		Name:            LookupNPCName(m.NPCID),
		MonsterTypeFlag: m.MonsterTypeFlag,
	}
}
