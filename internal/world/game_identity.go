package world

// CharacterClass identifies one of D2R's seven playable classes.
type CharacterClass uint8

// CharacterClass values follow D2UnitStrc.dwClassId.
const (
	CharacterClassAmazon CharacterClass = iota
	CharacterClassSorceress
	CharacterClassNecromancer
	CharacterClassPaladin
	CharacterClassBarbarian
	CharacterClassDruid
	CharacterClassAssassin
)

// String returns a stable lowercase class label.
func (c CharacterClass) String() string {
	switch c {
	case CharacterClassAmazon:
		return "amazon"
	case CharacterClassSorceress:
		return "sorceress"
	case CharacterClassNecromancer:
		return "necromancer"
	case CharacterClassPaladin:
		return "paladin"
	case CharacterClassBarbarian:
		return "barbarian"
	case CharacterClassDruid:
		return "druid"
	case CharacterClassAssassin:
		return "assassin"
	default:
		return "unknown"
	}
}

// GameIdentity is the Memory-confirmed active offline character identity.
// MapSeed is diagnostic only and must not authorize route playback.
type GameIdentity struct {
	Valid         bool
	CharacterName string
	Class         CharacterClass
	MapSeed       uint32
}
