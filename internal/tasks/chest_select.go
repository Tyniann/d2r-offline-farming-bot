package tasks

import (
	"github.com/Tyniann/d2r-offline-farming-bot/internal/town"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

const (
	// chestRackSeedTiles must cover both west Supertruhen from the ArmorStand
	// (live leftover sweep 20 Aug 2026: far west 183 was ~33 tiles from the stand).
	chestRackSeedTiles float64 = 34
	// chestPairTiles covers the west pair when the seed is the rack-adjacent chest
	// (~30 tiles to the second 183). Extra JungleChest 181 beside the hut may join.
	chestPairTiles                float64 = 32
	chestDropEvidenceTiles        float64 = 8
	chestApproachMaxDistanceTiles float64 = 40
	// A hut chest may need more projected teleports than ordinary loot because
	// walls and the screen edge shorten an otherwise valid approach.
	chestApproachMaxAttempts = 6
)

func objectIsClosed(object world.Object) bool {
	return object.ModeKnown && object.Mode == world.ObjectModeClosed
}

func objectIsOpened(object world.Object) bool {
	return object.ModeKnown && object.Mode == world.ObjectModeOpened
}

func inventoryKeyCount(state world.State) int {
	total := 0
	for _, item := range state.InventoryItems() {
		if item.Code != town.KeyItemCode {
			continue
		}
		if !item.QuantityKnown {
			continue
		}
		total += item.Quantity
	}
	return total
}

func groundItemIDs(state world.State) map[uint32]bool {
	ids := make(map[uint32]bool)
	for _, item := range state.GroundItems() {
		ids[item.UnitID] = true
	}
	return ids
}

func objectByUnitID(objects []world.Object, unitID uint32) (world.Object, bool) {
	if unitID == 0 {
		return world.Object{}, false
	}
	for _, object := range objects {
		if object.UnitID == unitID {
			return object, true
		}
	}
	return world.Object{}, false
}

func closestObject(player world.Position, objects []world.Object) (world.Object, bool) {
	best := world.Object{}
	bestDistance := 0.0
	found := false
	for _, object := range objects {
		if object.UnitID == 0 {
			continue
		}
		distance := world.Distance(player, object.Position)
		if !found || distance < bestDistance {
			best = object
			bestDistance = distance
			found = true
		}
	}
	return best, found
}

// hutEligibleSuperChests returns catalog Supertruhen that belong to a hut camp.
// A Supertruhe seeds only when a rack is within chestRackSeedTiles. A second
// Supertruhe joins when it is within chestPairTiles of a seed. The extra
// JungleChest 181 beside the west hut may join so the far west pair is not dropped.
func hutEligibleSuperChests(objects []world.Object) []world.Object {
	var chests []world.Object
	var racks []world.Object
	for _, object := range objects {
		if object.UnitID == 0 {
			continue
		}
		switch object.Kind {
		case world.ObjectKindSuperChest:
			chests = append(chests, object)
		case world.ObjectKindRack:
			racks = append(racks, object)
		}
	}
	if len(chests) == 0 || len(racks) == 0 {
		return nil
	}
	seed := make(map[uint32]bool, len(chests))
	var seeds []world.Object
	for _, chest := range chests {
		for _, rack := range racks {
			if world.Distance(chest.Position, rack.Position) <= chestRackSeedTiles {
				if !seed[chest.UnitID] {
					seed[chest.UnitID] = true
					seeds = append(seeds, chest)
				}
				break
			}
		}
	}
	if len(seeds) == 0 {
		return nil
	}
	selected := make([]world.Object, 0, len(chests))
	seen := make(map[uint32]bool, len(chests))
	for _, chest := range seeds {
		selected = append(selected, chest)
		seen[chest.UnitID] = true
	}
	for _, chest := range chests {
		if seen[chest.UnitID] {
			continue
		}
		for _, seedChest := range seeds {
			if world.Distance(chest.Position, seedChest.Position) <= chestPairTiles {
				selected = append(selected, chest)
				seen[chest.UnitID] = true
				break
			}
		}
	}
	return selected
}

func eligibleUnitIDs(chests []world.Object) map[uint32]bool {
	ids := make(map[uint32]bool, len(chests))
	for _, chest := range chests {
		ids[chest.UnitID] = true
	}
	return ids
}

func closedEligibleChests(objects []world.Object, skipped map[uint32]bool) []world.Object {
	var out []world.Object
	for _, chest := range hutEligibleSuperChests(objects) {
		if skipped[chest.UnitID] || !objectIsClosed(chest) {
			continue
		}
		out = append(out, chest)
	}
	return out
}

func closedRacksNear(objects []world.Object, chest world.Object, skipped map[uint32]bool) []world.Object {
	if chest.UnitID == 0 {
		return nil
	}
	var out []world.Object
	for _, object := range objects {
		if object.Kind != world.ObjectKindRack || object.UnitID == 0 {
			continue
		}
		if skipped[object.UnitID] || !objectIsClosed(object) {
			continue
		}
		if world.Distance(object.Position, chest.Position) > chestRackSeedTiles {
			continue
		}
		out = append(out, object)
	}
	return out
}

func closedRacksNearAnyEligible(objects []world.Object, skipped map[uint32]bool) []world.Object {
	eligible := hutEligibleSuperChests(objects)
	if len(eligible) == 0 {
		return nil
	}
	seen := make(map[uint32]bool)
	var out []world.Object
	for _, chest := range eligible {
		for _, rack := range closedRacksNear(objects, chest, skipped) {
			if seen[rack.UnitID] {
				continue
			}
			seen[rack.UnitID] = true
			out = append(out, rack)
		}
	}
	return out
}

type chestSelectMode int

const (
	chestSelectRoute chestSelectMode = iota
	chestSelectSweep
)

func selectChestOperateTarget(player world.Position, objects []world.Object, skipped map[uint32]bool, clusterChest world.Object, mode chestSelectMode) (world.Object, bool) {
	if clusterChest.UnitID != 0 {
		if racks := closedRacksNear(objects, clusterChest, skipped); len(racks) > 0 {
			return closestObject(player, racks)
		}
		return world.Object{}, false
	}
	if chests := closedEligibleChests(objects, skipped); len(chests) > 0 {
		target, found := closestObject(player, chests)
		if mode == chestSelectRoute && found && world.Distance(player, target.Position) > chestApproachMaxDistanceTiles {
			// Visibility spans several screens. Let playback enter the camp
			// before Hold so approach input does not burn its finite budget.
			return world.Object{}, false
		}
		return target, found
	}
	if mode == chestSelectSweep {
		if racks := closedRacksNearAnyEligible(objects, skipped); len(racks) > 0 {
			return closestObject(player, racks)
		}
	}
	return world.Object{}, false
}

func dropEvidenceNear(state world.State, target world.Object, previous map[uint32]bool) bool {
	if target.UnitID == 0 {
		return false
	}
	for _, item := range state.GroundItems() {
		if previous[item.UnitID] {
			continue
		}
		if world.Distance(item.Position, target.Position) <= chestDropEvidenceTiles {
			return true
		}
	}
	return false
}
