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

// TrashSellEligible reports whether item may be queued as Cow town-dump trash.
// Unidentified items are eligible: Cain identifies them before Akara sells.
// Keep-matches, locked cells, recipe reserved codes, and Pickit sell-matches
// stay off this path; sell-matches remain on the existing vendor path.
func (f *Filter) TrashSellEligible(item world.Item) bool {
	return f.trashDumpAllowed(item, true)
}

// TrashSellStillAuthorized reports whether a queued dump order may still run.
// After identification a former unmatched item may become a keep-match and
// must not be sold. A newly revealed Pickit sell-match is still sold.
func (f *Filter) TrashSellStillAuthorized(item world.Item) bool {
	return f.trashDumpAllowed(item, false)
}

func (f *Filter) trashDumpAllowed(item world.Item, refuseSellMatch bool) bool {
	if f == nil || CowRecipeReservedCode(item.Code) || !stashEligible(f.inventoryLock, item) {
		return false
	}
	result := f.evaluate(item)
	if result.Matched && result.Action == ActionKeep {
		return false
	}
	if refuseSellMatch && result.Matched && result.Action == ActionSell {
		return false
	}
	return true
}
