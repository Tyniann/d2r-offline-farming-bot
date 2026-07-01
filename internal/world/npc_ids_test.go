package world

import "testing"

func TestDarkStalkerID(t *testing.T) {
	if DarkStalker != 45 {
		t.Fatalf("DarkStalker = %d, want 45 (d2go npc iota)", DarkStalker)
	}
}
