package world

// ObjectKind classifies runtime-relevant objects.
type ObjectKind int

// ObjectKind values for waypoints, the Countess good chest, and Lower-Kurast Supertruhen.
const (
	ObjectKindUnknown ObjectKind = iota
	ObjectKindWaypoint
	ObjectKindGoodChest
	ObjectKindTownPortal
	ObjectKindPersonalStash
	ObjectKindPermanentPortal
	ObjectKindWirtsBody
	ObjectKindSuperChest
	ObjectKindRack
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
	case ObjectKindSuperChest:
		return "super_chest"
	case ObjectKindRack:
		return "rack"
	default:
		return "unknown"
	}
}

// ParseObjectKind maps a stable kind label back to [ObjectKind].
func ParseObjectKind(value string) (ObjectKind, bool) {
	for kind := ObjectKindUnknown; kind <= ObjectKindRack; kind++ {
		if kind.String() == value {
			return kind, true
		}
	}
	return ObjectKindUnknown, false
}

// ObjectModeClosed is UnitAny+0x0C for an unopened chest or rack.
const ObjectModeClosed uint32 = 0

// ObjectModeOpened is UnitAny+0x0C after a successful open. Locked is not a mode.
const ObjectModeOpened uint32 = 2

// Object is an interpreted world object with resolved kind and display name.
// Mode is UnitAny+0x0C. ModeKnown is false when that read failed; consumers
// must not treat Mode 0 as closed in that case.
type Object struct {
	Kind      ObjectKind
	ID        uint32 // TxtFileNo / object ID.
	UnitID    uint32
	Position  Position
	Name      string
	IsHovered bool // True when the hover buffer confirms this unit under the cursor.
	Mode      uint32
	ModeKnown bool
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
	if IsSuperChestID(id) {
		return ObjectKindSuperChest
	}
	if IsRackID(id) {
		return ObjectKindRack
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
