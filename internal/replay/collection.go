package replay

import (
	"slices"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

func normalizeCollection(state world.State) CollectionFrame {
	if !state.CollectionAvailable() {
		return CollectionFrame{}
	}
	c := state.Collection
	return CollectionFrame{ScopeID: c.ScopeID, ScopeKnown: c.ScopeKnown, Generation: c.Generation, Counts: slices.Clone(c.Counts), Tab: c.Tab, TabKnown: c.TabKnown}
}

func decodeCollection(frame Frame, at time.Time) world.CollectionState {
	c := frame.World.Collection
	if !frame.World.Valid || frame.World.Phase != "in_game" || !frame.World.UI.StashOpen || !c.ScopeKnown || c.ScopeID == "" || c.Generation == 0 || c.Generation != frame.Generation {
		return world.CollectionState{}
	}
	seen := make(map[string]bool)
	for _, count := range c.Counts {
		if count.Code == "" || seen[count.Code] || (count.Known && (count.Count < 0 || count.Count > 99)) {
			return world.CollectionState{}
		}
		seen[count.Code] = true
	}
	result := world.CollectionState{ScopeID: c.ScopeID, ScopeKnown: true, Generation: c.Generation, ObservedAt: at, Counts: slices.Clone(c.Counts)}
	if c.TabKnown && (c.Tab == world.StorageTabBasic || c.Tab == world.StorageTabMaterials || c.Tab == world.StorageTabGems || c.Tab == world.StorageTabRunes) {
		result.Tab, result.TabKnown = c.Tab, true
	}
	return result
}
