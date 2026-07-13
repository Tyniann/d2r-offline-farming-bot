package world

import "testing"

func TestDarkStalkerID(t *testing.T) {
	if DarkStalker != 45 {
		t.Fatalf("DarkStalker = %d, want 45 (d2go npc iota)", DarkStalker)
	}
}

func TestAct1TownNPCIDs(t *testing.T) {
	if DeckardCain != 265 || Akara != 148 || Charsi != 154 {
		t.Fatalf("town NPC IDs = %d/%d/%d", DeckardCain, Akara, Charsi)
	}
}
