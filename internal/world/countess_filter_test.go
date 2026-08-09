package world

import (
	"testing"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/memory"
)

func TestCountessFilterMatchesWorldIDs(t *testing.T) {
	for _, id := range AllWaypointIDs() {
		if !memory.IsRuntimeObjectID(id) {
			t.Fatalf("waypoint %d missing from countess filter", id)
		}
	}
	if !memory.IsRuntimeObjectID(GoodChestID) {
		t.Fatal("good chest missing from countess filter")
	}
	if !memory.IsRuntimeObjectID(TownPortalID) {
		t.Fatal("town portal missing from countess filter")
	}
	if !memory.IsRuntimeObjectID(PersonalStashID) {
		t.Fatal("personal stash missing from runtime filter")
	}
	if !memory.IsRuntimeObjectID(PermanentPortalID) || !memory.IsRuntimeObjectID(WirtsBodyID) {
		t.Fatal("Phase-20 portal or Wirt object missing from runtime filter")
	}
	for _, id := range AllEntranceIDs() {
		if !memory.IsCountessEntranceID(id) {
			t.Fatalf("entrance %d missing from countess filter", id)
		}
	}
	if !memory.IsCountessNPCID(DarkStalker) {
		t.Fatal("Dark Stalker missing from countess filter")
	}
}
