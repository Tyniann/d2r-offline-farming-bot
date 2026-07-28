package memory

import "testing"

func TestCountessMonsterCandidate(t *testing.T) {
	if !IsCountessMonsterCandidate(46, SuperUniqueMonsterFlag) {
		t.Fatal("super-unique Black Rogue 46 should match")
	}
	if !IsCountessMonsterCandidate(45, SuperUniqueMonsterFlag) {
		t.Fatal("super-unique Dark Stalker should match")
	}
	if IsCountessMonsterCandidate(45, 0) {
		t.Fatal("normal Dark Stalker without super-unique flag should not match")
	}
}

func TestRuntimeMonsterCandidateIncludesAct1TownNPCs(t *testing.T) {
	for _, id := range []uint32{148, 154, 265} {
		if !IsRuntimeMonsterCandidate(id, 0) {
			t.Fatalf("town NPC %d should match", id)
		}
	}
	if IsRuntimeMonsterCandidate(149, 0) {
		t.Fatal("unregistered normal monster should not match")
	}
}

func TestPostBossCleanupNPCIDs(t *testing.T) {
	for _, id := range []uint32{21, 38, 43, 44, 45, 46, 47, 55, 162, 40, 56, 131} {
		if !IsPostBossCleanupNPCID(id) || !IsRuntimeMonsterCandidate(id, 0) {
			t.Fatalf("cleanup hostile %d should be enumerated", id)
		}
	}
	for _, id := range []uint32{148, 265, 291} {
		if IsPostBossCleanupNPCID(id) {
			t.Fatalf("non-hostile or summon %d must not be a cleanup candidate", id)
		}
	}
}

func TestCountessTowerNPCIDs(t *testing.T) {
	for _, id := range []uint32{43, 44, 45, 46, 47} {
		if !IsCountessTowerNPCID(id) {
			t.Fatalf("tower npc %d should match", id)
		}
	}
}

func TestCountessFilterObjects(t *testing.T) {
	for _, id := range []uint32{584, 59, 119, 157} {
		if !IsRuntimeObjectID(id) {
			t.Fatalf("object %d should match", id)
		}
	}
	if IsRuntimeObjectID(999) {
		t.Fatal("unexpected object match")
	}
}

func TestCountessFilterEntrances(t *testing.T) {
	for _, id := range []uint32{10, 11, 17, 18} {
		if !IsCountessEntranceID(id) {
			t.Fatalf("entrance %d should match", id)
		}
	}
}
