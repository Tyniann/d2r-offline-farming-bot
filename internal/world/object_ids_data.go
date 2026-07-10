// Code generated from D2R 3.2.92777 local data/global/excel/objects.txt; DO NOT EDIT.
package world

const (
	// TownPortalID is the player-cast portal object for this generated D2R version.
	TownPortalID uint32 = 59
	// GoodChestID is the unique chest placement object for this generated D2R version.
	GoodChestID uint32 = 584
	// PersonalStashID is the character stash object for this generated D2R version.
	PersonalStashID uint32 = 267
)

var waypointIDs = []uint32{
	119, // WaypointOutsideAct1
	145, // InnerHellWaypoint
	156, // WaypointAct2
	157, // WaypointInsideAct1
	237, // WaypointAct3
	238, // WaypointOutsideAct4
	288, // WaypointCellar
	323, // SewerWaypoint
	324, // TravincalWaypoint
	398, // WaypointAct4Town
	402, // WaypointDesertValley
	429, // WaypointExp
	494, // WaypointBaal
	496, // WaypointWilderness
	511, // WaypointIceCave
	539, // TempleWaypoint
}

// IsWaypointID reports whether id is a waypoint object in the generated catalog.
func IsWaypointID(id uint32) bool {
	for _, waypointID := range waypointIDs {
		if id == waypointID {
			return true
		}
	}
	return false
}

// AllWaypointIDs returns a copy of generated waypoint object IDs.
func AllWaypointIDs() []uint32 { return append([]uint32(nil), waypointIDs...) }

var objectNames = map[uint32]string{
	TownPortalID:    "Town Portal",
	GoodChestID:     "Good Chest",
	PersonalStashID: "Personal Stash",
	119:             "Waypoint",
	145:             "Waypoint",
	156:             "Waypoint",
	157:             "Waypoint",
	237:             "Waypoint",
	238:             "Waypoint",
	288:             "Waypoint",
	323:             "Waypoint",
	324:             "Waypoint",
	398:             "Waypoint",
	402:             "Waypoint",
	429:             "Waypoint",
	494:             "Waypoint",
	496:             "Waypoint",
	511:             "Waypoint",
	539:             "Waypoint",
}
