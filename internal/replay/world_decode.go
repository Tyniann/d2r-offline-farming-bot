package replay

import (
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

func worldStateFromFrame(frame Frame, at time.Time) world.State {
	state := world.State{
		At: at, Generation: frame.Generation, Phase: parseGamePhase(frame.World.Phase), Valid: frame.World.Valid, Reason: frame.World.Reason,
		Area:               world.LookupArea(world.AreaID(frame.World.AreaID)),
		Player:             world.Player{Position: world.Position{X: frame.World.Player.X, Y: frame.World.Player.Y}, HP: frame.World.Player.HP, MaxHP: frame.World.Player.MaxHP, Mana: frame.World.Player.Mana, MaxMana: frame.World.Player.MaxMana, Gold: frame.World.Player.Gold, PrivateStashGold: frame.World.Player.PrivateStashGold, GoldKnown: frame.World.Player.GoldKnown, PrivateStashGoldKnown: frame.World.Player.PrivateStashGoldKnown, LeftSkillID: frame.World.Player.LeftSkillID, RightSkillID: frame.World.Player.RightSkillID, ActiveWeaponSet: parseWeaponSet(frame.World.Player.ActiveWeaponSet, frame.World.Player.WeaponSetAvailable), SkillsKnown: make(map[uint16]bool, len(frame.World.Player.SkillsKnown)), SkillsComplete: frame.World.Player.SkillsComplete, SkillsIncompleteReason: frame.World.Player.SkillsIncompleteReason},
		Mercenary:          world.Mercenary{HiredKnown: frame.World.Mercenary.HiredKnown, Hired: frame.World.Mercenary.Hired, Alive: frame.World.Mercenary.Alive, Dead: frame.World.Mercenary.Dead, VitalsKnown: frame.World.Mercenary.VitalsKnown, UnitID: frame.World.Mercenary.UnitID, NPCID: frame.World.Mercenary.NPCID, HP: frame.World.Mercenary.HP, MaxHP: frame.World.Mercenary.MaxHP},
		Identity:           world.GameIdentity{Valid: frame.World.Identity.Valid, CharacterName: frame.World.Identity.CharacterName, Class: parseCharacterClass(frame.World.Identity.Class)},
		UI:                 world.UIState{InventoryOpen: frame.World.UI.InventoryOpen, NPCInteractOpen: frame.World.UI.NPCInteractOpen, NPCShopOpen: frame.World.UI.NPCShopOpen, WaypointOpen: frame.World.UI.WaypointOpen, StashOpen: frame.World.UI.StashOpen, QuitMenuOpen: frame.World.UI.QuitMenuOpen, CubeOpen: frame.World.UI.CubeOpen, CubeOpenKnown: frame.World.UI.CubeOpenKnown},
		Hover:              world.HoverInfo{IsHovered: frame.World.Hover.Hovered, UnitType: parseHoverUnitType(frame.World.Hover.UnitType), UnitID: frame.World.Hover.UnitID},
		CowCorpsesComplete: frame.World.Evidence["cow_corpses_complete"],
		MonsterCoverage:    world.MonsterCoverage{EligibleMonsterCount: frame.World.MonsterCoverage.EligibleCount, MonstersTruncated: frame.World.MonsterCoverage.Truncated, MonsterCoverageRadiusTiles: frame.World.MonsterCoverage.RadiusTiles},
	}
	for _, skillID := range frame.World.Player.SkillsKnown {
		state.Player.SkillsKnown[skillID] = true
	}
	for _, object := range frame.World.Objects {
		state.Objects = append(state.Objects, world.Object{Kind: parseObjectKind(object), ID: object.ID, UnitID: object.UnitID, Position: world.Position{X: object.X, Y: object.Y}, IsHovered: object.Hovered, Mode: object.Mode, ModeKnown: object.ModeKnown})
	}
	for _, entrance := range frame.World.Entrances {
		state.Entrances = append(state.Entrances, world.Entrance{Kind: parseEntranceKind(entrance), ID: entrance.ID, UnitID: entrance.UnitID, Position: world.Position{X: entrance.X, Y: entrance.Y}, IsHovered: entrance.Hovered})
	}
	for _, monster := range frame.World.Monsters {
		state.Monsters = append(state.Monsters, world.Monster{NPCID: monster.NPCID, UnitID: monster.UnitID, Position: world.Position{X: monster.X, Y: monster.Y}, MonsterTypeFlag: monster.TypeFlag, IsHovered: monster.Hovered})
	}
	for _, corpse := range frame.World.CowCorpses {
		state.CowCorpses = append(state.CowCorpses, world.CowCorpse{NPCID: corpse.NPCID, UnitID: corpse.UnitID, Position: world.Position{X: corpse.X, Y: corpse.Y}, MonsterTypeFlag: corpse.TypeFlag, ObservedAt: at, SnapshotGeneration: frame.Generation, Consumed: corpse.Consumed, ConsumptionKnown: corpse.ConsumptionKnown})
	}
	for _, item := range frame.World.Items {
		state.Items = append(state.Items, world.Item{TxtFileNo: item.TxtFileNo, UnitID: item.UnitID, Code: item.Code, Quality: parseItemQuality(item.Quality), IdentityKind: world.ItemIdentityKind(item.IdentityKind), IdentityKey: item.IdentityKey, IdentityValid: item.IdentityValid, Location: world.ItemLocation(item.Location), OwnerID: item.OwnerID, PlayerOwned: item.PlayerOwned, Page: item.Page, GridX: item.GridX, GridY: item.GridY, Width: item.Width, Height: item.Height, Position: world.Position{X: item.X, Y: item.Y}, Identified: item.Identified, Ethereal: item.Ethereal, IsHovered: item.Hovered, Sockets: item.Sockets, SocketsAvailable: item.SocketsAvailable, Socketed: item.Socketed, Quantity: item.Quantity, QuantityKnown: item.QuantityKnown})
	}
	return state
}

func parseWeaponSet(value string, available bool) world.WeaponSetState {
	if !available {
		return world.WeaponSetState{}
	}
	switch value {
	case "primary":
		return world.WeaponSetState{Set: world.WeaponSetPrimary, Available: true}
	case "secondary":
		return world.WeaponSetState{Set: world.WeaponSetSecondary, Available: true}
	default:
		return world.WeaponSetState{}
	}
}

func parseGamePhase(value string) world.GamePhase {
	switch value {
	case "menu":
		return world.GamePhaseMenu
	case "loading":
		return world.GamePhaseLoading
	case "in_game":
		return world.GamePhaseInGame
	default:
		return world.GamePhaseUnknown
	}
}

func parseCharacterClass(value string) world.CharacterClass {
	for class := world.CharacterClassAmazon; class <= world.CharacterClassWarlock; class++ {
		if class.String() == value {
			return class
		}
	}
	return ^world.CharacterClass(0)
}

func parseHoverUnitType(value string) world.HoverUnitType {
	for _, unitType := range []world.HoverUnitType{world.HoverUnitTypeMonster, world.HoverUnitTypeObject, world.HoverUnitTypeItem, world.HoverUnitTypeEntrance} {
		if unitType.String() == value {
			return unitType
		}
	}
	return 0
}

func parseObjectKind(entity EntityFrame) world.ObjectKind {
	if kind := world.LookupObjectKind(entity.ID); kind.String() == entity.Kind {
		return kind
	}
	if kind, ok := world.ParseObjectKind(entity.Kind); ok {
		return kind
	}
	return world.ObjectKindUnknown
}

func parseEntranceKind(entity EntityFrame) world.EntranceKind {
	if kind := world.LookupEntranceKind(entity.ID); kind.String() == entity.Kind {
		return kind
	}
	if kind, ok := world.ParseEntranceKind(entity.Kind); ok {
		return kind
	}
	return world.EntranceKindUnknown
}

func parseItemQuality(value string) world.ItemQuality {
	for quality := world.ItemQualityUnknown; quality <= world.ItemQualityCrafted; quality++ {
		if quality.String() == value {
			return quality
		}
	}
	return world.ItemQualityUnknown
}
