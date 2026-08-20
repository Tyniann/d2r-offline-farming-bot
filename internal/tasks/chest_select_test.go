package tasks

import (
	"testing"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/town"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

// Live closed capture 20260820T083741.908526300Z-closed.json, Area 79 camp.
func liveHutCampObjects() []world.Object {
	return []world.Object{
		closedObject(world.JungleChest2ID, world.ObjectKindSuperChest, 183, 5032, 2994),
		closedObject(world.JungleChest2ID, world.ObjectKindSuperChest, 181, 5027, 3012),
		closedObject(world.JungleChestID, world.ObjectKindSuperChest, 180, 5012, 3017),
		closedObject(world.JungleChestID, world.ObjectKindSuperChest, 159, 5060, 2972),
		closedObject(world.ArmorStand1ID, world.ObjectKindRack, 182, 5012, 2983),
		closedObject(world.WeaponRack2ID, world.ObjectKindRack, 158, 5048, 2972),
		{ID: 160, Kind: world.ObjectKindUnknown, UnitID: 157, Position: world.Position{X: 5056, Y: 3011}, Mode: 2, ModeKnown: true},
	}
}

func closedObject(id uint32, kind world.ObjectKind, unitID, x, y uint32) world.Object {
	return world.Object{
		ID: id, Kind: kind, UnitID: unitID, Position: world.Position{X: x, Y: y},
		Mode: world.ObjectModeClosed, ModeKnown: true,
	}
}

func TestHutEligibleSuperChestsUsesLiveCampProximity(t *testing.T) {
	got := hutEligibleSuperChests(liveHutCampObjects())
	ids := eligibleUnitIDs(got)
	if !ids[183] || !ids[181] || !ids[159] {
		t.Fatalf("eligible = %v, want west pair 183/181 and east 159", ids)
	}
	if !ids[180] {
		t.Fatal("extra JungleChest 180 beside the west hut must stay eligible so the far west pair is not dropped")
	}
	if len(got) != 4 {
		t.Fatalf("eligible count = %d, want 4", len(got))
	}
}

func TestHutEligibleIncludesWestPairWhenSeedIsRackAdjacent(t *testing.T) {
	// Productive leftover sweep 2026-08-20: opened 183 at 5033,2983 next to the
	// armor stand; the second west 183 sat ~30 tiles further and was skipped at 22.
	objects := []world.Object{
		closedObject(world.JungleChestID, world.ObjectKindSuperChest, 120, 5060, 2972),
		closedObject(world.WeaponRack2ID, world.ObjectKindRack, 119, 5048, 2972),
		closedObject(world.JungleChest2ID, world.ObjectKindSuperChest, 128, 5033, 2983),
		closedObject(world.ArmorStand1ID, world.ObjectKindRack, 127, 5012, 2983),
		closedObject(world.JungleChest2ID, world.ObjectKindSuperChest, 200, 5027, 3012),
	}
	ids := eligibleUnitIDs(hutEligibleSuperChests(objects))
	if !ids[120] || !ids[128] || !ids[200] {
		t.Fatalf("eligible = %v, want east 120 and both west 128/200", ids)
	}
}

func TestHutEligibleSuperChestsIgnoresCatalogIDWithoutHutRack(t *testing.T) {
	objects := []world.Object{
		closedObject(world.JungleChestID, world.ObjectKindSuperChest, 180, 5012, 3017),
		closedObject(world.WeaponRack2ID, world.ObjectKindRack, 1, 4800, 2800),
	}
	if got := hutEligibleSuperChests(objects); len(got) != 0 {
		t.Fatalf("lonely extra chest seeded = %+v", got)
	}
}

func TestSelectChestOperateTargetIgnoresRackWithoutNearbySuperChest(t *testing.T) {
	player := world.Position{X: 5050, Y: 3000}
	objects := []world.Object{
		closedObject(world.WeaponRack2ID, world.ObjectKindRack, 99, 5050, 3000),
		closedObject(world.JungleChestID, world.ObjectKindSuperChest, 180, 4800, 2800),
	}
	if _, ok := selectChestOperateTarget(player, objects, nil, world.Object{}, chestSelectRoute); ok {
		t.Fatal("route selected a rack with no nearby Supertruhe")
	}
	if _, ok := selectChestOperateTarget(player, objects, nil, world.Object{}, chestSelectSweep); ok {
		t.Fatal("sweep selected a rack with no nearby Supertruhe")
	}
}

func TestSelectChestOperateTargetWaitsUntilRouteIsNearCamp(t *testing.T) {
	objects := []world.Object{
		closedObject(world.JungleChest2ID, world.ObjectKindSuperChest, 132, 5008, 2963),
		closedObject(world.ArmorStand1ID, world.ObjectKindRack, 131, 5012, 2983),
	}
	far := world.Position{X: 5060, Y: 2963}
	if target, ok := selectChestOperateTarget(far, objects, nil, world.Object{}, chestSelectRoute); ok {
		t.Fatalf("route selected distant chest %+v before entering the camp", target)
	}
	if target, ok := selectChestOperateTarget(far, objects, nil, world.Object{}, chestSelectSweep); !ok || target.UnitID != 132 {
		t.Fatalf("terminal sweep target = %+v, ok=%t, want chest 132", target, ok)
	}
	near := world.Position{X: 5048, Y: 2963}
	if target, ok := selectChestOperateTarget(near, objects, nil, world.Object{}, chestSelectRoute); !ok || target.UnitID != 132 {
		t.Fatalf("near route target = %+v, ok=%t, want chest 132", target, ok)
	}
}

func TestSelectChestOperateTargetSkipsProcessedUnitIDs(t *testing.T) {
	player := world.Position{X: 5032, Y: 2994}
	objects := liveHutCampObjects()
	skipped := map[uint32]bool{183: true, 181: true, 159: true}
	target, ok := selectChestOperateTarget(player, objects, skipped, world.Object{UnitID: 183, Position: world.Position{X: 5032, Y: 2994}}, chestSelectRoute)
	if !ok || target.UnitID != 182 {
		t.Fatalf("after chests skipped, want west rack 182, got ok=%t %+v", ok, target)
	}
	skipped[182] = true
	skipped[158] = true
	if _, again := selectChestOperateTarget(player, objects, skipped, world.Object{UnitID: 183}, chestSelectSweep); again {
		t.Fatal("processed UnitIDs were selected again")
	}
}

func TestInventoryKeyCountTreatsUnknownQuantityAsZero(t *testing.T) {
	state := world.State{Items: []world.Item{
		{Code: town.KeyItemCode, Location: world.ItemLocationInventory, PlayerOwned: true, Page: 0, Quantity: 7, QuantityKnown: true},
		{Code: town.KeyItemCode, Location: world.ItemLocationInventory, PlayerOwned: true, Page: 0, Quantity: 4, QuantityKnown: false},
		{Code: "pk1", Location: world.ItemLocationInventory, PlayerOwned: true, Page: 0, Quantity: 1, QuantityKnown: true},
		{Code: town.KeyItemCode, Location: world.ItemLocationStash, PlayerOwned: true, Page: 0, Quantity: 12, QuantityKnown: true},
	}}
	if got := inventoryKeyCount(state); got != 7 {
		t.Fatalf("key count = %d, want 7", got)
	}
}

func TestObjectIsClosedRequiresKnownMode(t *testing.T) {
	if objectIsClosed(world.Object{Mode: 0}) {
		t.Fatal("unknown mode must not count as closed")
	}
	if !objectIsClosed(world.Object{Mode: world.ObjectModeClosed, ModeKnown: true}) {
		t.Fatal("known mode 0 should count as closed")
	}
	if objectIsClosed(world.Object{Mode: world.ObjectModeOpened, ModeKnown: true}) {
		t.Fatal("opened must not count as closed")
	}
}
