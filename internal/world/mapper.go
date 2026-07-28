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
	hover := mapHover(snap.Hover)

	objects := make([]Object, 0, len(snap.Objects))
	for _, o := range snap.Objects {
		objects = append(objects, mapObject(o, hover))
	}
	entrances := make([]Entrance, 0, len(snap.Entrances))
	for _, e := range snap.Entrances {
		entrances = append(entrances, mapEntrance(e, hover))
	}
	monsters := make([]Monster, 0, len(snap.Monsters))
	for _, m := range snap.Monsters {
		monsters = append(monsters, mapMonster(m, hover))
	}
	items := make([]Item, 0, len(snap.Items))
	for _, i := range snap.Items {
		items = append(items, mapItem(i, hover))
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
			Items:     items,
			Hover:     hover,
			UI:        mapUIState(snap.UI),
		}
	}

	return State{
		At:    snap.At,
		Valid: true,
		Phase: phase,
		Area:  LookupArea(AreaID(snap.AreaID)),
		Player: Player{
			Position:              Position{X: snap.PosX, Y: snap.PosY},
			HP:                    snap.HP,
			MaxHP:                 snap.MaxHP,
			Mana:                  snap.Mana,
			MaxMana:               snap.MaxMana,
			Gold:                  snap.Gold,
			PrivateStashGold:      snap.PrivateStashGold,
			GoldKnown:             snap.GoldKnown,
			PrivateStashGoldKnown: snap.PrivateStashGoldKnown,
			LeftSkillID:           snap.PlayerSkills.LeftSkill,
			RightSkillID:          snap.PlayerSkills.RightSkill,
		},
		Identity:  mapGameIdentity(snap.Identity),
		Objects:   objects,
		Entrances: entrances,
		Monsters:  monsters,
		Items:     items,
		Hover:     hover,
		UI:        mapUIState(snap.UI),
	}
}

func mapGameIdentity(identity memory.IdentityProbe) GameIdentity {
	if !identity.Valid || !identity.Confirmed || identity.ClassID > uint32(CharacterClassWarlock) {
		return GameIdentity{}
	}
	return GameIdentity{
		Valid:         true,
		CharacterName: identity.CharacterName,
		Class:         CharacterClass(identity.ClassID),
		MapSeed:       identity.MapSeed,
	}
}

func mapUIState(ui memory.UIState) UIState {
	return UIState{InventoryOpen: ui.InventoryOpen, NPCInteractOpen: ui.NPCInteractOpen, NPCShopOpen: ui.NPCShopOpen, WaypointOpen: ui.WaypointOpen, StashOpen: ui.StashOpen, QuitMenuOpen: ui.QuitMenuOpen}
}

// mapHover converts the raw memory hover buffer into the world hover type.
func mapHover(h memory.HoverState) HoverInfo {
	if !h.IsHovered {
		return HoverInfo{}
	}
	return HoverInfo{
		IsHovered: true,
		UnitType:  HoverUnitType(h.UnitType),
		UnitID:    h.UnitID,
	}
}

func mapObject(o memory.ObjectUnit, hover HoverInfo) Object {
	name := LookupObjectName(o.TxtFileNo)
	if name == "" && LookupObjectKind(o.TxtFileNo) == ObjectKindWaypoint {
		name = "Waypoint"
	}
	return Object{
		Kind:      LookupObjectKind(o.TxtFileNo),
		ID:        o.TxtFileNo,
		UnitID:    o.UnitID,
		Position:  Position{X: o.PosX, Y: o.PosY},
		Name:      name,
		IsHovered: hover.Matches(HoverUnitTypeObject, o.UnitID),
	}
}

func mapEntrance(e memory.EntranceUnit, hover HoverInfo) Entrance {
	return Entrance{
		Kind:      LookupEntranceKind(e.TxtFileNo),
		ID:        e.TxtFileNo,
		UnitID:    e.UnitID,
		Position:  Position{X: e.PosX, Y: e.PosY},
		Name:      LookupEntranceName(e.TxtFileNo),
		IsHovered: hover.Matches(HoverUnitTypeEntrance, e.UnitID),
	}
}

func mapMonster(m memory.MonsterUnit, hover HoverInfo) Monster {
	return Monster{
		NPCID:           m.NPCID,
		UnitID:          m.UnitID,
		Position:        Position{X: m.PosX, Y: m.PosY},
		Name:            LookupNPCName(m.NPCID),
		MonsterTypeFlag: m.MonsterTypeFlag,
		IsHovered:       hover.Matches(HoverUnitTypeMonster, m.UnitID),
	}
}
