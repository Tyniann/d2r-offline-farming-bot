package memory

// Countess-relevant entity ID allowlists. IDs must stay in sync with internal/world/*_ids.go.

// SuperUniqueMonsterFlag is the unitData type flag for super-unique monsters (d2go getMonsterType).
const SuperUniqueMonsterFlag uint8 = 10

// IsCountessNPCID reports whether id is the Dark Stalker base type (d2go npc iota 45).
// Use [IsCountessMonsterCandidate] for probe enumeration (super-unique flag).
func IsCountessNPCID(id uint32) bool {
	return id == 45
}

// IsCountessMonsterCandidate reports whether a living monster unit should appear in the Countess probe snapshot.
// Super-uniques (flag 10) are included regardless of NPC id so The Countess is found even when her base type is not 51.
func IsCountessMonsterCandidate(_ uint32, monsterTypeFlag uint8) bool {
	return monsterTypeFlag == SuperUniqueMonsterFlag
}

// IsCountessTowerNPCID reports tower-area trash monster NPC ids (Forgotten Tower cellars).
func IsCountessTowerNPCID(id uint32) bool {
	switch id {
	case 43, 44, 45, 46, 47: // DarkHunter … FleshHunter (d2go npc iota)
		return true
	default:
		return false
	}
}

// IsCountessObjectID reports whether id is a Countess-route object (waypoints, good chest).
func IsCountessObjectID(id uint32) bool {
	switch id {
	case 580: // Good chest (PlaceUniqueChest)
		return true
	case 119, 145, 156, 157, 237, 238, 288, 323, 324, 398, 402, 429, 494, 496, 511, 539:
		return true
	default:
		return false
	}
}

// IsCountessEntranceID reports whether id is a Countess-route entrance (tower + cellar stairs).
func IsCountessEntranceID(id uint32) bool {
	switch id {
	case 10, 11, 17, 18:
		return true
	default:
		return false
	}
}
