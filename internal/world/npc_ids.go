package world

// Monster is an interpreted living monster entity.
type Monster struct {
	NPCID           uint32
	UnitID          uint32
	Position        Position
	Name            string
	MonsterTypeFlag uint8
	IsHovered       bool // True when the hover buffer confirms this unit under the cursor.
}

// DarkStalker is the Countess base NPC type (d2go npc.ID iota 45; file line ≠ iota value).
const DarkStalker uint32 = 45

const (
	// DeckardCain is the live-validated Rogue Encampment `cain5` NPC ID.
	DeckardCain uint32 = 265
	// Akara is the Act-1 potion, scroll, and sell vendor NPC ID.
	Akara uint32 = 148
	// Charsi is the Act-1 repair vendor NPC ID.
	Charsi uint32 = 154
)

// SuperUniqueMonsterFlag is the unitData flag for super-unique monsters in d2go.
const SuperUniqueMonsterFlag uint8 = 10

// LookupNPCName returns a display name for known Countess-route NPC IDs.
func LookupNPCName(id uint32) string {
	switch id {
	case DarkStalker:
		return "Dark Stalker"
	case DeckardCain:
		return "Deckard Cain"
	case Akara:
		return "Akara"
	case Charsi:
		return "Charsi"
	}
	return ""
}
