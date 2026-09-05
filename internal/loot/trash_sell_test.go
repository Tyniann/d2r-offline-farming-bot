package loot

import (
	"testing"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

func TestTrashSellEligibleRejectsKeepLockReservedAndUnidentified(t *testing.T) {
	filter := testDecisionFilter(t, [][]int{
		{1, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		{1, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		{0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		{0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
	}, `[name] == gpv`)

	trash := inventoryItem(1, "8ws", "swor", 2, 0, 1, 3)
	trash.Identified = true
	if !filter.TrashSellEligible(trash) {
		t.Fatal("unlocked unmatched identified weapon must be trash")
	}

	keep := inventoryItem(2, "gpv", "gem", 3, 0, 1, 1)
	keep.Identified = true
	if filter.TrashSellEligible(keep) {
		t.Fatal("keep-match must not be trash")
	}

	locked := inventoryItem(3, "8ws", "swor", 0, 0, 1, 2)
	locked.Identified = true
	if filter.TrashSellEligible(locked) {
		t.Fatal("locked cells must not be trash")
	}

	unidentified := trash
	unidentified.UnitID = 4
	unidentified.Identified = false
	if filter.TrashSellEligible(unidentified) {
		t.Fatal("unidentified items must not be trash")
	}

	for _, code := range []string{"box", "tbk", "ibk", "leg"} {
		reserved := inventoryItem(10, code, "misc", 4, 0, 1, 1)
		reserved.Identified = true
		if filter.TrashSellEligible(reserved) || !CowRecipeReservedCode(code) {
			t.Fatalf("reserved code %q must not be trash", code)
		}
	}
}

func TestTrashSellEligibleRejectsPickitSellMatches(t *testing.T) {
	pickit, err := CompilePickitRules("test", []PickitRuleSpec{
		{ProfileID: "mephisto-standard", RuleID: "candidate", Action: ActionSell, Expression: `[quality] == unique`},
	})
	if err != nil {
		t.Fatal(err)
	}
	filter := NewFilter(testLogger(), testLock(t, allFreeLock()), pickit)
	item := inventoryItem(5, "xap", "helm", 0, 0, 2, 2)
	item.Identified = true
	item.Quality = world.ItemQualityUnique
	if filter.TrashSellEligible(item) {
		t.Fatal("Pickit sell matches must stay on the vendor path")
	}
}
