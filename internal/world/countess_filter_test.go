package world

import (
	"testing"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/memory"
)

func TestCountessFilterMatchesWorldIDs(t *testing.T) {
	for _, id := range AllWaypointIDs() {
		if !memory.IsCountessObjectID(id) {
			t.Fatalf("waypoint %d missing from countess filter", id)
		}
	}
	if !memory.IsCountessObjectID(GoodChestID) {
		t.Fatal("good chest missing from countess filter")
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
