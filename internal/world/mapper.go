package world

import (
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/memory"
)

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
	livingMonsterIDs := make(map[uint32]struct{}, len(snap.Monsters))
	for _, m := range snap.Monsters {
		monsters = append(monsters, mapMonster(m, hover))
		livingMonsterIDs[m.UnitID] = struct{}{}
	}
	cowCorpses := make([]CowCorpse, 0, len(snap.CowCorpses))
	cowCorpsesComplete := snap.Valid && snap.Phase == memory.GamePhaseInGame && snap.CowCorpsesComplete
	corpseIDs := make(map[uint32]struct{}, len(snap.CowCorpses))
	for _, corpse := range snap.CowCorpses {
		_, living := livingMonsterIDs[corpse.UnitID]
		_, duplicate := corpseIDs[corpse.UnitID]
		if corpse.UnitID == 0 || corpse.PosX == 0 || corpse.PosY == 0 || living || duplicate ||
			(corpse.NPCID != HellBovine && corpse.NPCID != CowKing) {
			cowCorpsesComplete = false
			continue
		}
		corpseIDs[corpse.UnitID] = struct{}{}
		cowCorpses = append(cowCorpses, mapCowCorpse(corpse, snap.At, snap.Generation))
	}
	items := make([]Item, 0, len(snap.Items))
	for _, i := range snap.Items {
		items = append(items, mapItem(i, hover))
	}

	if !snap.Valid {
		return State{
			At:                 snap.At,
			Generation:         snap.Generation,
			Valid:              false,
			Reason:             snap.Reason,
			Phase:              phase,
			Objects:            objects,
			Entrances:          entrances,
			Monsters:           monsters,
			CowCorpses:         cowCorpses,
			CowCorpsesComplete: false,
			MonsterCoverage:    mapMonsterCoverage(snap.MonsterCoverage),
			Items:              items,
			Hover:              hover,
			UI:                 mapUIState(snap.UI),
		}
	}

	return State{
		At:         snap.At,
		Generation: snap.Generation,
		Valid:      true,
		Phase:      phase,
		Area:       LookupArea(AreaID(snap.AreaID)),
		Player: Player{
			ActiveWeaponSet:        mapWeaponSet(snap.ActiveWeaponSet),
			Position:               Position{X: snap.PosX, Y: snap.PosY},
			HP:                     snap.HP,
			MaxHP:                  snap.MaxHP,
			Mana:                   snap.Mana,
			MaxMana:                snap.MaxMana,
			Gold:                   snap.Gold,
			PrivateStashGold:       snap.PrivateStashGold,
			GoldKnown:              snap.GoldKnown,
			PrivateStashGoldKnown:  snap.PrivateStashGoldKnown,
			LeftSkillID:            snap.PlayerSkills.LeftSkill,
			RightSkillID:           snap.PlayerSkills.RightSkill,
			SkillsKnown:            cloneSkillKnown(snap.PlayerSkills.SkillsKnown),
			SkillsComplete:         snap.PlayerSkills.Complete,
			SkillsIncompleteReason: snap.PlayerSkills.IncompleteReason,
		},
		Mercenary:          mapMercenary(snap.Mercenary),
		Identity:           mapGameIdentity(snap.Identity),
		Objects:            objects,
		Entrances:          entrances,
		Monsters:           monsters,
		CowCorpses:         cowCorpses,
		CowCorpsesComplete: cowCorpsesComplete,
		MonsterCoverage:    mapMonsterCoverage(snap.MonsterCoverage),
		Items:              items,
		Hover:              hover,
		UI:                 mapUIState(snap.UI),
	}
}

func mapWeaponSet(active memory.WeaponSetSnapshot) WeaponSetState {
	if !active.Available || active.Value > uint8(WeaponSetSecondary) {
		return WeaponSetState{}
	}
	return WeaponSetState{Set: WeaponSet(active.Value), Available: true}
}

func mapMercenary(mercenary memory.MercenarySnapshot) Mercenary {
	if !mercenary.HiredKnown {
		return Mercenary{}
	}
	if !mercenary.Hired {
		return Mercenary{HiredKnown: true}
	}
	if mercenary.Alive == mercenary.Dead {
		return Mercenary{}
	}
	result := Mercenary{
		HiredKnown: true,
		Hired:      true,
		Alive:      mercenary.Alive,
		Dead:       mercenary.Dead,
		UnitID:     mercenary.UnitID,
		NPCID:      mercenary.NPCID,
	}
	if !result.Alive || !mercenary.VitalsKnown || mercenary.MaxHP == 0 {
		return result
	}
	result.VitalsKnown = true
	result.HP = mercenary.HP
	result.MaxHP = mercenary.MaxHP
	return result
}

func mapMonsterCoverage(coverage memory.MonsterCoverage) MonsterCoverage {
	return MonsterCoverage{
		EligibleMonsterCount:       coverage.EligibleMonsterCount,
		MonstersTruncated:          coverage.MonstersTruncated,
		MonsterCoverageRadiusTiles: coverage.MonsterCoverageRadiusTiles,
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
	return UIState{InventoryOpen: ui.InventoryOpen, NPCInteractOpen: ui.NPCInteractOpen, NPCShopOpen: ui.NPCShopOpen, WaypointOpen: ui.WaypointOpen, StashOpen: ui.StashOpen, QuitMenuOpen: ui.QuitMenuOpen, CubeOpen: ui.CubeOpenKnown && ui.CubeOpen, CubeOpenKnown: ui.CubeOpenKnown}
}

func mapCowCorpse(corpse memory.CowCorpseUnit, observedAt time.Time, generation uint64) CowCorpse {
	return CowCorpse{
		NPCID: corpse.NPCID, UnitID: corpse.UnitID,
		Position: Position{X: corpse.PosX, Y: corpse.PosY},
		Name:     LookupNPCName(corpse.NPCID), MonsterTypeFlag: corpse.MonsterTypeFlag,
		ObservedAt: observedAt, SnapshotGeneration: generation,
		Consumed: corpse.Consumed, ConsumptionKnown: corpse.ConsumptionKnown,
	}
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
		Mode:      o.Mode,
		ModeKnown: o.ModeKnown,
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
