package world

// HoverUnitType classifies which unit segment the hovered unit belongs to,
// matching the D2R hover buffer semantics (d2go: 1=monster, 2=object, 4=item, 5=entrance).
type HoverUnitType uint32

// HoverUnitType values relevant for entity matching.
const (
	HoverUnitTypeMonster  HoverUnitType = 1
	HoverUnitTypeObject   HoverUnitType = 2
	HoverUnitTypeItem     HoverUnitType = 4
	HoverUnitTypeEntrance HoverUnitType = 5
)

// String returns a stable label for structured logging.
func (t HoverUnitType) String() string {
	switch t {
	case HoverUnitTypeMonster:
		return "monster"
	case HoverUnitTypeObject:
		return "object"
	case HoverUnitTypeItem:
		return "item"
	case HoverUnitTypeEntrance:
		return "entrance"
	default:
		return "unknown"
	}
}

// HoverInfo describes which unit is currently under the mouse cursor.
// The zero value means nothing is hovered; UnitType and UnitID are only
// meaningful when IsHovered is true.
type HoverInfo struct {
	IsHovered bool
	UnitType  HoverUnitType
	UnitID    uint32
}

// Matches reports whether the hover data confirms the unit identified by
// unitType and unitID is under the cursor. Both type and ID must match so a
// monster standing in front of an entrance is never confused with it.
func (h HoverInfo) Matches(unitType HoverUnitType, unitID uint32) bool {
	return h.IsHovered && h.UnitType == unitType && h.UnitID == unitID
}
