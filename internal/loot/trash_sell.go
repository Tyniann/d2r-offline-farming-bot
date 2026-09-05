package loot

import (
	"strings"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

// CowRecipeReservedCode reports item codes that Cow town dump must never sell.
func CowRecipeReservedCode(code string) bool {
	switch strings.ToLower(strings.TrimSpace(code)) {
	case "box", "tbk", "ibk", "leg":
		return true
	default:
		return false
	}
}

// TrashSellEligible reports whether item may be sold as Cow town-dump trash.
// Authorization is independent of Pickit sell matches: identified personal
// inventory, unlocked footprint, not a keep-match, and not a recipe reserved code.
func (f *Filter) TrashSellEligible(item world.Item) bool {
	if f == nil || !item.Identified || CowRecipeReservedCode(item.Code) || !stashEligible(f.inventoryLock, item) {
		return false
	}
	result := f.evaluate(item)
	if result.Matched && (result.Action == ActionKeep || result.Action == ActionSell) {
		return false
	}
	return true
}
