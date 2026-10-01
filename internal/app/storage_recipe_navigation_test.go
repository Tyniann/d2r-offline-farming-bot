package app

import (
	"testing"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/crafting"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/input"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/loot"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/memory"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

func TestStorageRecipeNavigatesFromNonMaterialTab(t *testing.T) {
	for _, tab := range []string{"basic", "materials"} {
		t.Run(tab, func(t *testing.T) {
			now := time.Unix(100, 0)
			snap := memory.Snapshot{Valid: true, Phase: memory.GamePhaseInGame, Generation: 1, At: now, AreaID: uint32(world.RogueEncampment), UI: memory.UIState{StashOpen: true, InventoryOpen: true}, Collection: memory.CollectionSnapshot{Available: true, ScopeID: "fixture", Generation: 1, Tab: tab, TabKnown: true}}
			for _, entry := range world.ItemCatalogEntries() {
				count, found := map[string]uint32{"gsr": 19, "glr": 95}[entry.Code]
				if found {
					snap.Collection.Counts = append(snap.Collection.Counts, memory.CollectionCount{TxtFileNo: entry.TxtFileNo, UnitID: entry.TxtFileNo + 1, Count: count})
				}
			}
			s := world.FromSnapshot(snap)
			s.Items = []world.Item{{UnitID: 1, Code: "box", Location: world.ItemLocationInventory, PlayerOwned: true, Width: 2, Height: 2}}
			log := config.NewLogger("error")
			in := &compactionInput{status: input.Status{Enabled: true}}
			e, err := crafting.NewRecipeTestExecutor(log, loot.NewCollectionActions(log, in, loot.DefaultCollectionLayout()), "gsr")
			if err != nil {
				t.Fatal(err)
			}
			request := crafting.Request{Code: "gsr", RunGeneration: 1}
			if result := e.Tick(s, s.At, request, false, false); result.Done || len(in.buttons) != 1 || in.buttons[0] != input.MouseLeft {
				t.Fatalf("tab selection failed: result=%+v buttons=%v", result, in.buttons)
			}
			fresh := func() {
				s.Generation++
				s.At = s.At.Add(350 * time.Millisecond)
				s.Collection.Generation, s.Collection.ObservedAt = s.Generation, s.At
			}
			// Time alone cannot authorize ingredient input while the switch is unknown.
			fresh()
			s.Collection.TabKnown = false
			if result := e.Tick(s, s.At, request, false, false); result.Done || len(in.buttons) != 1 {
				t.Fatal("unconfirmed tab sent input or stopped early")
			}
			fresh()
			s.Collection.Tab, s.Collection.TabKnown = world.StorageTabGems, true
			e.Tick(s, s.At, request, false, false)
			fresh()
			if result := e.Tick(s, s.At, request, false, false); result.Done || len(in.buttons) != 2 || in.buttons[1] != input.MouseRight {
				t.Fatalf("confirmed tab did not load ingredients: result=%+v buttons=%v", result, in.buttons)
			}
		})
	}
}
