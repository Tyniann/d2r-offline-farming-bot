package memory

// Countess-relevant entity ID allowlists. IDs must stay in sync with internal/world/*_ids.go.

// SuperUniqueMonsterFlag is the unitData type flag for super-unique monsters (d2go getMonsterType).
const SuperUniqueMonsterFlag uint8 = 10

// IsCountessNPCID reports whether id is the Dark Stalker base type (d2go npc iota 45).
// Use [IsCountessMonsterCandidate] for probe enumeration (super-unique flag).
func IsCountessNPCID(id uint32) bool {
	return id == 45
}

// IsCountessMonsterCandidate reports whether a living monster unit should appear in the Countess probe snapshot.
// Super-uniques (flag 10) are included regardless of NPC id so The Countess is found even when her base type is not 51.
func IsCountessMonsterCandidate(_ uint32, monsterTypeFlag uint8) bool {
	return monsterTypeFlag == SuperUniqueMonsterFlag
}

// IsRuntimeMonsterCandidate reports whether a living unit is required by current run or Town features.
func IsRuntimeMonsterCandidate(id uint32, monsterTypeFlag uint8) bool {
	if IsCountessMonsterCandidate(id, monsterTypeFlag) {
		return true
	}
	if IsPostBossCleanupNPCID(id) {
		return true
	}
	if _, ok := runtimeBossNPCIDs[id]; ok {
		return true
	}
	switch id {
	case 148, 150, 154, 265:
		return true
	}
	return false
}

func isRuntimePriorityMonsterCandidate(id uint32, monsterTypeFlag uint8) bool {
	if IsCountessMonsterCandidate(id, monsterTypeFlag) {
		return true
	}
	if _, ok := runtimeBossNPCIDs[id]; ok {
		return true
	}
	switch id {
	case 148, 150, 154, 265:
		return true
	default:
		return false
	}
}

// IsPostBossCleanupNPCID reports whether id is a hostile base type that can
// accompany Summoner or Nihlathak post-boss cleanup. Countess skips cleanup
// because many Tower Cellar hostiles sit behind walls and waste the budget.
// The allowlist intentionally excludes player summons and unrelated monster
// units.
func IsPostBossCleanupNPCID(id uint32) bool {
	switch id {
	case 40, 56, 131: // Arcane Sanctuary.
		return true
	case 295, 438, 458, 472, 546, 547, 551, 552, 553, 554, 555, 578, 597, 631, 656, 659, 662, 682, 685: // Halls of Vaught and spawned minions.
		return true
	default:
		return false
	}
}

// IsCountessTowerNPCID reports tower-area trash monster NPC ids (Forgotten Tower cellars).
func IsCountessTowerNPCID(id uint32) bool {
	switch id {
	case 43, 44, 45, 46, 47: // DarkHunter … FleshHunter (d2go npc iota)
		return true
	default:
		return false
	}
}

// IsRuntimeObjectID reports whether id is needed by the current run and recovery features.
func IsRuntimeObjectID(id uint32) bool {
	_, ok := runtimeObjectIDs[id]
	return ok
}

// IsCountessEntranceID reports whether id is a Countess-route entrance candidate.
// Entrance units are a small segment and some transition units in fixed rooms
// use IDs outside the known d2go tower/stairs constants, so probes keep all
// entrance IDs and let the world layer classify the known subset.
func IsCountessEntranceID(id uint32) bool {
	return id > 0
}
