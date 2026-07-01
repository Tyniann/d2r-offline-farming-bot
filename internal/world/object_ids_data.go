package world

// Waypoint object IDs from d2go objects.go entries with Name == "Waypoint" @ 16d248a53591.
const (
	WaypointAct1Town              uint32 = 119
	WaypointAct1ColdPlains        uint32 = 145
	WaypointAct1StonyField        uint32 = 156
	WaypointAct1Wilderness        uint32 = 157 // Black Marsh
	WaypointAct2Town              uint32 = 237
	WaypointAct2Sewers            uint32 = 238
	WaypointAct2DryHills          uint32 = 288
	WaypointAct2HallsOfDead       uint32 = 323
	WaypointAct2FarOasis          uint32 = 324
	WaypointAct3Town              uint32 = 398
	WaypointAct3KurastBazaar      uint32 = 402
	WaypointAct3Travincal         uint32 = 429
	WaypointAct4Town              uint32 = 494
	WaypointAct4RiverOfFlame      uint32 = 496
	WaypointAct5Town              uint32 = 511
	WaypointAct5GlacialPeak       uint32 = 539
)

// GoodChestID is the unique chest object used by the Countess (d2go PlaceUniqueChest).
const GoodChestID uint32 = 580

// waypointIDs lists all waypoint IDs for filter-sync tests.
var waypointIDs = []uint32{
	WaypointAct1Town,
	WaypointAct1ColdPlains,
	WaypointAct1StonyField,
	WaypointAct1Wilderness,
	WaypointAct2Town,
	WaypointAct2Sewers,
	WaypointAct2DryHills,
	WaypointAct2HallsOfDead,
	WaypointAct2FarOasis,
	WaypointAct3Town,
	WaypointAct3KurastBazaar,
	WaypointAct3Travincal,
	WaypointAct4Town,
	WaypointAct4RiverOfFlame,
	WaypointAct5Town,
	WaypointAct5GlacialPeak,
}

// IsWaypointID reports whether id is a waypoint object.
func IsWaypointID(id uint32) bool {
	for _, wp := range waypointIDs {
		if id == wp {
			return true
		}
	}
	return false
}

// AllWaypointIDs returns a copy of known waypoint IDs.
func AllWaypointIDs() []uint32 {
	return append([]uint32(nil), waypointIDs...)
}

var objectNames = map[uint32]string{
	GoodChestID:              "Good Chest",
	WaypointAct1Town:         "Waypoint",
	WaypointAct1ColdPlains:   "Waypoint",
	WaypointAct1StonyField:   "Waypoint",
	WaypointAct1Wilderness:   "Waypoint",
	WaypointAct2Town:         "Waypoint",
	WaypointAct2Sewers:       "Waypoint",
	WaypointAct2DryHills:     "Waypoint",
	WaypointAct2HallsOfDead:  "Waypoint",
	WaypointAct2FarOasis:     "Waypoint",
	WaypointAct3Town:         "Waypoint",
	WaypointAct3KurastBazaar: "Waypoint",
	WaypointAct3Travincal:    "Waypoint",
	WaypointAct4Town:         "Waypoint",
	WaypointAct4RiverOfFlame: "Waypoint",
	WaypointAct5Town:         "Waypoint",
	WaypointAct5GlacialPeak:  "Waypoint",
}
