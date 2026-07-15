package world

// EntranceKind classifies registered dungeon and wilderness transitions.
type EntranceKind int

// EntranceKind values for tower and cellar transitions.
const (
	EntranceKindUnknown EntranceKind = iota
	EntranceKindTowerCellarUp
	EntranceKindTowerCellarDown
	EntranceKindTowerToWilderness
	EntranceKindWildernessToTower
	EntranceKindCatacombsUp
	EntranceKindCatacombsDown
	EntranceKindDuranceUp
	EntranceKindDuranceDown
)

// String returns a stable label for logging.
func (k EntranceKind) String() string {
	switch k {
	case EntranceKindTowerCellarUp:
		return "tower_cellar_up"
	case EntranceKindTowerCellarDown:
		return "tower_cellar_down"
	case EntranceKindWildernessToTower:
		return "wilderness_to_tower"
	case EntranceKindTowerToWilderness:
		return "tower_to_wilderness"
	case EntranceKindCatacombsUp:
		return "catacombs_up"
	case EntranceKindCatacombsDown:
		return "catacombs_down"
	case EntranceKindDuranceUp:
		return "durance_up"
	case EntranceKindDuranceDown:
		return "durance_down"
	default:
		return "unknown"
	}
}

// ParseEntranceKind resolves a stable route-contract label to its registered kind.
func ParseEntranceKind(value string) (EntranceKind, bool) {
	for kind := EntranceKindUnknown; kind <= EntranceKindDuranceDown; kind++ {
		if kind.String() == value {
			return kind, true
		}
	}
	return EntranceKindUnknown, false
}

// Entrance is an interpreted world entrance with resolved kind and display name.
type Entrance struct {
	Kind      EntranceKind
	ID        uint32
	UnitID    uint32
	Position  Position
	Name      string
	IsHovered bool // True when the hover buffer confirms this unit under the cursor.
}

// Entrance IDs from d2go entrance catalog @ 16d248a53591.
const (
	EntranceTowerCellarUp     uint32 = 8
	EntranceTowerCellarDown   uint32 = 9
	EntranceWildernessToTower uint32 = 10
	EntranceTowerToWilderness uint32 = 11
	EntranceCatacombsUp       uint32 = 17
	EntranceCatacombsDown     uint32 = 18
	EntranceDuranceUpLeft     uint32 = 65
	EntranceDuranceUpRight    uint32 = 66
	EntranceDuranceDownLeft   uint32 = 67
	EntranceDuranceDownRight  uint32 = 68
)

var entranceIDs = []uint32{
	EntranceTowerCellarUp,
	EntranceTowerCellarDown,
	EntranceWildernessToTower,
	EntranceTowerToWilderness,
	EntranceCatacombsUp,
	EntranceCatacombsDown,
	EntranceDuranceUpLeft,
	EntranceDuranceUpRight,
	EntranceDuranceDownLeft,
	EntranceDuranceDownRight,
}

var entranceNames = map[uint32]string{
	EntranceTowerCellarUp:     "Act 1 Tower Cellar Up",
	EntranceTowerCellarDown:   "Act 1 Tower Cellar Down",
	EntranceWildernessToTower: "Act 1 Wilderness to Tower",
	EntranceTowerToWilderness: "Act 1 Tower to Wilderness",
	EntranceCatacombsUp:       "Act 1 Catacombs Up",
	EntranceCatacombsDown:     "Act 1 Catacombs Down",
	EntranceDuranceUpLeft:     "Act 3 Durance Up Left",
	EntranceDuranceUpRight:    "Act 3 Durance Up Right",
	EntranceDuranceDownLeft:   "Act 3 Durance Down Left",
	EntranceDuranceDownRight:  "Act 3 Durance Down Right",
}

// LookupEntranceKind resolves an entrance ID to its semantic kind.
func LookupEntranceKind(id uint32) EntranceKind {
	switch id {
	case EntranceTowerCellarUp:
		return EntranceKindTowerCellarUp
	case EntranceTowerCellarDown:
		return EntranceKindTowerCellarDown
	case EntranceWildernessToTower:
		return EntranceKindWildernessToTower
	case EntranceTowerToWilderness:
		return EntranceKindTowerToWilderness
	case EntranceCatacombsUp:
		return EntranceKindCatacombsUp
	case EntranceCatacombsDown:
		return EntranceKindCatacombsDown
	case EntranceDuranceUpLeft, EntranceDuranceUpRight:
		return EntranceKindDuranceUp
	case EntranceDuranceDownLeft, EntranceDuranceDownRight:
		return EntranceKindDuranceDown
	default:
		return EntranceKindUnknown
	}
}

// LookupEntranceName returns a display name for a known entrance ID.
func LookupEntranceName(id uint32) string {
	return entranceNames[id]
}

// AllEntranceIDs returns every semantically classified run entrance ID.
func AllEntranceIDs() []uint32 {
	return append([]uint32(nil), entranceIDs...)
}
