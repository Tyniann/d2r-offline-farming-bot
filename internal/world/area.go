package world

import "fmt"

// AreaID identifies a D2R level/area, matching memory.Snapshot.AreaID.
type AreaID uint32

// Act identifies which game act an area belongs to.
type Act uint8

// Act values for the five acts plus an unknown sentinel.
const (
	ActUnknown Act = iota
	Act1
	Act2
	Act3
	Act4
	Act5
)

// String returns a stable label for structured logging.
func (a Act) String() string {
	switch a {
	case Act1:
		return "act1"
	case Act2:
		return "act2"
	case Act3:
		return "act3"
	case Act4:
		return "act4"
	case Act5:
		return "act5"
	default:
		return "unknown"
	}
}

// AreaKind classifies terrain semantics for pathing and run logic.
// Town detection uses AreaID.IsTown(), not AreaKind.
type AreaKind uint8

// AreaKind values. Towns keep AreaKindUnknown; use IsTown() instead.
const (
	AreaKindUnknown AreaKind = iota
	AreaKindOutdoor
	AreaKindDungeon
	AreaKindSpecial
)

// String returns a stable label for structured logging.
func (k AreaKind) String() string {
	switch k {
	case AreaKindOutdoor:
		return "outdoor"
	case AreaKindDungeon:
		return "dungeon"
	case AreaKindSpecial:
		return "special"
	default:
		return "unknown"
	}
}

// Area is a resolved area with display name, act, and kind.
// Act is always derived from the ID via AreaID.Act(), never stored in the catalog.
type Area struct {
	ID   AreaID   // D2R level identifier.
	Name string   // Display name from the embedded catalog or an unknown fallback.
	Act  Act      // Derived act; ActUnknown for ID 0.
	Kind AreaKind // Terrain classification; towns use AreaKindUnknown.
}

// LookupArea returns catalog metadata for id.
// Unknown or unnamed IDs get a synthetic name and AreaKindUnknown.
// ID 0 always yields ActUnknown because it appears during menu/load screens.
func LookupArea(id AreaID) Area {
	if id == 0 {
		return Area{
			ID:   id,
			Name: fmt.Sprintf("Unknown Area %d", id),
			Act:  ActUnknown,
			Kind: AreaKindUnknown,
		}
	}

	entry, ok := areaCatalog[id]
	if !ok || entry.name == "" {
		return Area{
			ID:   id,
			Name: fmt.Sprintf("Unknown Area %d", id),
			Act:  id.Act(),
			Kind: AreaKindUnknown,
		}
	}

	return Area{
		ID:   id,
		Name: entry.name,
		Act:  id.Act(),
		Kind: entry.kind,
	}
}

// Act returns the game act for id.
// ID 0 yields ActUnknown; other IDs follow d2go act ranges with that exception.
func (id AreaID) Act() Act {
	if id == 0 {
		return ActUnknown
	}
	if id < 40 {
		return Act1
	}
	if id < 75 {
		return Act2
	}
	if id < 103 {
		return Act3
	}
	if id < 109 {
		return Act4
	}
	return Act5
}

// IsTown reports whether id is one of the five town hubs.
func (id AreaID) IsTown() bool {
	switch id {
	case RogueEncampment, LutGholein, KurastDocks, ThePandemoniumFortress, Harrogath:
		return true
	}
	return false
}

// String returns a human-readable area label for logging.
func (id AreaID) String() string {
	a := LookupArea(id)
	if a.IsKnown() {
		return a.Name
	}
	return fmt.Sprintf("AreaID(%d)", id)
}

// IsKnown reports whether the area has a non-empty catalog name (IDs 1..136).
func (a Area) IsKnown() bool {
	entry, ok := areaCatalog[a.ID]
	return ok && entry.name != ""
}

// IsTown reports whether the area is a town hub.
func (a Area) IsTown() bool {
	return a.ID.IsTown()
}

// IsDungeon reports whether the area is classified as a dungeon.
func (a Area) IsDungeon() bool {
	return a.Kind == AreaKindDungeon
}
