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
		"WaypointOutsideAct1\tWaypoint\twaypoint portal\t811",
		"Expansion\t\t\t",
		"Bank\tbank\tbank\t901",
		"PlaceUniqueChest\tPlaceUniqueChest\tPlaces a Unique Chest\t999",
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
	if selected.TownPortal.ID != 701 || selected.GoodChest.ID != 999 {
		t.Fatalf("selected IDs = portal %d chest %d, want 701/999", selected.TownPortal.ID, selected.GoodChest.ID)
	}
	if selected.Stash.ID != 901 {
		t.Fatalf("stash ID = %d, want 901", selected.Stash.ID)
	}
	if len(selected.Waypoints) != 1 || selected.Waypoints[0].ID != 811 {
		t.Fatalf("waypoints = %+v, want ID 811", selected.Waypoints)
	}
}

func TestRenderedCatalogsKeepMemoryAndWorldIDsInSync(t *testing.T) {
	selected := selectedObjects{
		TownPortal: objectRow{ID: 701, Class: "TownPortal"},
		GoodChest:  objectRow{ID: 999, Class: "PlaceUniqueChest"},
		Stash:      objectRow{ID: 901, Class: "Bank"},
		Waypoints:  []objectRow{{ID: 811, Class: "WaypointOutsideAct1"}},
	}
	memorySource := string(renderMemory("test-version", selected))
	worldSource := string(renderWorld("test-version", selected))
	for _, want := range []string{"701", "999", "901", "811", "test-version"} {
		if !strings.Contains(memorySource, want) || !strings.Contains(worldSource, want) {
			t.Fatalf("generated sources do not both contain %q", want)
		}
	}
}
