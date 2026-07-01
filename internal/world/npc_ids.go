package world

// Monster is an interpreted living monster entity.
type Monster struct {
	NPCID           uint32
	UnitID          uint32
	Position        Position
	Name            string
	MonsterTypeFlag uint8
}

// DarkStalker is the Countess base NPC type (d2go npc.ID iota 45; file line ≠ iota value).
const DarkStalker uint32 = 45

// SuperUniqueMonsterFlag is the unitData flag for super-unique monsters in d2go.
const SuperUniqueMonsterFlag uint8 = 10

// LookupNPCName returns a display name for known Countess-route NPC IDs.
func LookupNPCName(id uint32) string {
	if id == DarkStalker {
		return "Dark Stalker"
	}
	return ""
}
