package app

import (
	"testing"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

func TestStorageTransferConfirmationRequiresCountAndBoundCube(t *testing.T) {
	before := world.State{At: time.Unix(100, 0), Generation: 1, Valid: true, Phase: world.GamePhaseInGame, UI: world.UIState{StashOpen: true, InventoryOpen: true}, Items: []world.Item{{UnitID: 1, Code: "box", Location: world.ItemLocationInventory, PlayerOwned: true}}, Collection: world.CollectionState{ScopeID: "fixture", ScopeKnown: true, Generation: 1, ObservedAt: time.Unix(100, 0), Tab: world.StorageTabGems, TabKnown: true, Counts: []world.MaterialCount{{Code: "gsr", Count: 22, Known: true}}}}
	for _, test := range []struct {
		name           string
		count          int
		code           string
		cursor         bool
		complete, fail bool
	}{
		{"pending_count", 22, "gsr", false, false, false},
		{"confirmed", 21, "gsr", false, true, false},
		{"wrong_material", 21, "gsg", false, false, true},
		{"cursor", 21, "gsr", true, false, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			s := before
			s.Collection.Counts = []world.MaterialCount{{Code: "gsr", Count: test.count, Known: true}}
			s.Items = append(append([]world.Item(nil), before.Items...), world.Item{UnitID: 2, Code: test.code, Location: world.ItemLocationCube, PlayerOwned: true, Width: 1, Height: 1})
			if test.cursor {
				s.Items = append(s.Items, world.Item{UnitID: 3, Location: world.ItemLocationCursor})
			}
			complete, err := storageTransferConfirmed(s, before, world.StorageTabGems, "gsr", 21, true)
			if complete != test.complete || (err != nil) != test.fail {
				t.Fatalf("confirmed=%t err=%v", complete, err)
			}
		})
	}
}
