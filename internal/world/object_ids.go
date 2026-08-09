package world

// ObjectKind classifies runtime-relevant objects.
type ObjectKind int

// ObjectKind values for waypoints and the Countess good chest.
const (
	ObjectKindUnknown ObjectKind = iota
	ObjectKindWaypoint
	ObjectKindGoodChest
	ObjectKindTownPortal
	ObjectKindPersonalStash
	ObjectKindPermanentPortal
	ObjectKindWirtsBody
)

// String returns a stable label for logging.
func (k ObjectKind) String() string {
	switch k {
	case ObjectKindWaypoint:
		return "waypoint"
	case ObjectKindGoodChest:
		return "good_chest"
	case ObjectKindTownPortal:
		return "town_portal"
	case ObjectKindPersonalStash:
		return "personal_stash"
	case ObjectKindPermanentPortal:
		return "permanent_portal"
	case ObjectKindWirtsBody:
		return "wirts_body"
	default:
		return "unknown"
	}
}

// Object is an interpreted world object with resolved kind and display name.
type Object struct {
	Kind      ObjectKind
	ID        uint32 // TxtFileNo / object ID.
	UnitID    uint32
	Position  Position
	Name      string
	IsHovered bool // True when the hover buffer confirms this unit under the cursor.
}

// LookupObjectKind resolves an object ID to its semantic kind.
func LookupObjectKind(id uint32) ObjectKind {
	if IsWaypointID(id) {
		return ObjectKindWaypoint
	}
	if id == GoodChestID {
		return ObjectKindGoodChest
	}
	if id == TownPortalID {
		return ObjectKindTownPortal
	}
	if id == PersonalStashID {
		return ObjectKindPersonalStash
	}
	if id == PermanentPortalID {
		return ObjectKindPermanentPortal
	}
	if id == WirtsBodyID {
		return ObjectKindWirtsBody
	}
	return ObjectKindUnknown
}

// LookupObjectName returns a display name for a known object ID.
func LookupObjectName(id uint32) string {
	if name, ok := objectNames[id]; ok {
		return name
	}
	return ""
}
