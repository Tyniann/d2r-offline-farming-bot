package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadAndSelectObjectsUsesExplicitExportIDs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "objects.txt")
	data := strings.Join([]string{
		"Class\tName\t*Description\t*ID",
		"Null\tDummy\ttest data\t0",
		"TownPortal\tPortal\tTown portal\t701",
		"PortalPermanent\tPortal\tPermanent town portal\t702",
		"Wirt\twirt's body\twirt's body\t703",
		"WaypointOutsideAct1\tWaypoint\twaypoint portal\t811",
		"Expansion\t\t\t",
		"Bank\tbank\tbank\t901",
		"PlaceUniqueChest\tPlaceUniqueChest\tPlaces a Unique Chest\t999",
		"JungleChest\tchest\tjungle chest act 3\t181",
		"JungleChest2\tchest\tchest-L-med jungle\t183",
		"ArmorStand1\tArmorStand\tArmor Stand 1R\t104",
		"ArmorStand2\tArmorStand\tArmor Stand 2L\t105",
		"WeaponRack2\tWeaponRack\tWeapon Rack 2L\t107",
		"Chest5\tgchest\tgeneric chest\t240",
	}, "\n")
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}

	rows, err := readObjectRows(path)
	if err != nil {
		t.Fatalf("readObjectRows() error = %v", err)
	}
	selected, err := selectObjects(rows)
	if err != nil {
		t.Fatalf("selectObjects() error = %v", err)
	}
	if selected.TownPortal.ID != 701 || selected.PermanentPortal.ID != 702 || selected.WirtsBody.ID != 703 || selected.GoodChest.ID != 999 {
		t.Fatalf("selected IDs = portal %d chest %d, want 701/999", selected.TownPortal.ID, selected.GoodChest.ID)
	}
	if selected.Stash.ID != 901 {
		t.Fatalf("stash ID = %d, want 901", selected.Stash.ID)
	}
	if len(selected.Waypoints) != 1 || selected.Waypoints[0].ID != 811 {
		t.Fatalf("waypoints = %+v, want ID 811", selected.Waypoints)
	}
	if len(selected.SuperChests) != 2 || selected.SuperChests[0].ID != 181 || selected.SuperChests[1].ID != 183 {
		t.Fatalf("super chests = %+v, want 181 then 183", selected.SuperChests)
	}
	if len(selected.Racks) != 2 || selected.Racks[0].ID != 104 || selected.Racks[1].ID != 107 {
		t.Fatalf("racks = %+v, want 104 then 107", selected.Racks)
	}
}

func TestRenderedCatalogsKeepMemoryAndWorldIDsInSync(t *testing.T) {
	selected := selectedObjects{
		TownPortal:      objectRow{ID: 701, Class: "TownPortal"},
		PermanentPortal: objectRow{ID: 702, Class: "PortalPermanent"},
		WirtsBody:       objectRow{ID: 703, Class: "Wirt"},
		GoodChest:       objectRow{ID: 999, Class: "PlaceUniqueChest"},
		Stash:           objectRow{ID: 901, Class: "Bank"},
		Waypoints:       []objectRow{{ID: 811, Class: "WaypointOutsideAct1"}},
		SuperChests:     []objectRow{{ID: 181, Class: "JungleChest"}, {ID: 183, Class: "JungleChest2"}},
		Racks:           []objectRow{{ID: 104, Class: "ArmorStand1"}, {ID: 107, Class: "WeaponRack2"}},
	}
	memorySource := string(renderMemory("test-version", selected))
	worldSource := string(renderWorld("test-version", selected))
	for _, want := range []string{"701", "702", "703", "999", "901", "811", "181", "183", "104", "107", "test-version"} {
		if !strings.Contains(memorySource, want) || !strings.Contains(worldSource, want) {
			t.Fatalf("generated sources do not both contain %q", want)
		}
	}
	if strings.Contains(memorySource, "240") || strings.Contains(worldSource, "105") {
		t.Fatal("generated catalogs must not include generic chests or unused rack facings")
	}
}

func TestSelectObjectsRejectsMissingLiveCampfireClasses(t *testing.T) {
	rows := []objectRow{
		{ID: 701, Class: "TownPortal"},
		{ID: 702, Class: "PortalPermanent"},
		{ID: 703, Class: "Wirt"},
		{ID: 811, Class: "WaypointOutsideAct1", Name: "Waypoint"},
		{ID: 901, Class: "Bank"},
		{ID: 999, Class: "PlaceUniqueChest"},
		{ID: 181, Class: "JungleChest"},
	}
	if _, err := selectObjects(rows); err == nil {
		t.Fatal("selectObjects() accepted a catalog without JungleChest2 and hut racks")
	}
}
