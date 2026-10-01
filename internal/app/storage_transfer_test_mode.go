package app

import (
	"context"
	"fmt"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/input"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/loot"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

// runStorageTransferTest is only the Gate 25.4 diagnostic: select the other
// material tab, observe readiness, withdraw exactly one unit and return that
// same unit. It never transmutes, retries a click or invokes a farming task.
// Every confirmation has three seconds of active time; pause freezes that
// budget, stop revokes input. An unconfirmed partial transfer stays untouched.
func (rt *Runtime) runStorageTransferTest(ctx context.Context, run *runState, hotkeys <-chan input.HotkeyEvent, ticker *time.Ticker, cancel context.CancelFunc, code string) error {
	in, ok := rt.Input.(loot.CollectionInput)
	if !ok {
		return fmt.Errorf("storage transfer: positioned input unavailable")
	}
	tab := world.StorageTabGems
	if code == "r01" {
		tab = world.StorageTabRunes
	}
	before := rt.World.Current()
	stock, known := before.CollectionCount(code)
	if !known || stock < 1 || !before.Collection.TabKnown || !storageTransferUnchanged(before, before) {
		return fmt.Errorf("Materialbestand, leerer Cursor oder leerer Truhenwürfel nicht bestätigt")
	}
	if before.Collection.Tab == tab {
		return fmt.Errorf("für die Tabkalibrierung bitte zuerst den anderen Materialtab öffnen")
	}
	layout := loot.DefaultCollectionLayout()
	actions := loot.NewCollectionActions(rt.Log, in, layout)
	if err := actions.SelectTab(before, tab); err != nil {
		return fmt.Errorf("storage transfer tab: %w", err)
	}
	// Visibility/activation and unchanged stock must agree in two fresh frames.
	// Their actual elapsed time supplies this diagnostic's settle; no guessed
	// fixed delay enables a transfer. The product layout remains uncalibrated
	// until the operator's live results are recorded in Gate 25.4.
	var firstReady uint64
	ready, err := rt.waitStorageTransfer(ctx, run, hotkeys, ticker, cancel, before.Collection.ScopeID, before.Generation, func(s world.State) (bool, error) {
		count, valid := s.CollectionCount(code)
		if !valid {
			firstReady = 0
			return false, nil
		}
		if count != stock || !storageTransferUnchanged(s, before) {
			return false, fmt.Errorf("Bestand oder Inventar während der Tabwahl verändert")
		}
		if !s.Collection.TabKnown || s.Collection.Tab != tab {
			firstReady = 0
			return false, nil
		}
		if firstReady == 0 {
			firstReady = s.Generation
			return false, nil
		}
		return s.Generation > firstReady, nil
	})
	if err != nil {
		return err
	}
	layout.Settle = ready.At.Sub(before.At)
	if layout.Settle <= 0 {
		return fmt.Errorf("Tabwartezeit nicht messbar")
	}
	rt.Log.Info("storage transfer tab calibrated", "item_code", code, "tab", tab, "settle_ms", layout.Settle.Milliseconds(), "scope", ready.Collection.ScopeID, "generation", ready.Generation)
	// Rebind only software state on the now-confirmed tab. SelectTab sends no
	// second click, and the measured delay plus another fresh frame still apply.
	actions = loot.NewCollectionActions(rt.Log, in, layout)
	if err = actions.SelectTab(ready, tab); err != nil {
		return err
	}
	selected := ready
	ready, err = rt.waitStorageTransfer(ctx, run, hotkeys, ticker, cancel, before.Collection.ScopeID, ready.Generation, func(s world.State) (bool, error) {
		count, valid := s.CollectionCount(code)
		if !valid || !s.Collection.TabKnown {
			return false, nil
		}
		if count != stock || s.Collection.Tab != tab || !storageTransferUnchanged(s, before) {
			return false, fmt.Errorf("Materialzustand vor der Entnahme verändert")
		}
		return s.At.Sub(selected.At) >= layout.Settle, nil
	})
	if err != nil {
		return err
	}
	if err = actions.WithdrawOne(ready, code); err != nil {
		return fmt.Errorf("storage transfer withdraw: %w", err)
	}
	loaded, err := rt.waitStorageTransfer(ctx, run, hotkeys, ticker, cancel, before.Collection.ScopeID, ready.Generation, func(s world.State) (bool, error) {
		return storageTransferConfirmed(s, before, tab, code, stock-1, true)
	})
	if err != nil {
		return err
	}
	unit := loaded.ItemsByLocation(world.ItemLocationCube)[0]
	rt.Log.Info("storage transfer withdraw confirmed", "item_code", code, "count_before", stock, "count_after", stock-1, "cube_unit_id", unit.UnitID, "cube_x", unit.GridX, "cube_y", unit.GridY, "generation", loaded.Generation)
	if err = actions.StoreOutput(loaded, unit.UnitID, code); err != nil {
		return fmt.Errorf("storage transfer return: %w", err)
	}
	returned, err := rt.waitStorageTransfer(ctx, run, hotkeys, ticker, cancel, before.Collection.ScopeID, loaded.Generation, func(s world.State) (bool, error) {
		confirmed, e := storageTransferConfirmed(s, before, tab, code, stock, false)
		if e != nil {
			return false, e
		}
		for _, item := range s.ItemsByLocation(world.ItemLocationCube) {
			if item.UnitID != unit.UnitID {
				return false, fmt.Errorf("Würfelinhalt während der Rückgabe verändert")
			}
		}
		return confirmed, nil
	})
	if err != nil {
		return err
	}
	rt.Log.Info("storage transfer test completed", "item_code", code, "count_before", stock, "count_after", stock, "cube_unit_id", unit.UnitID, "cube_empty", true, "cursor_empty", true, "inventory_unchanged", true, "stash_open", returned.UI.StashOpen, "settle_ms", layout.Settle.Milliseconds(), "generation", returned.Generation)
	return nil
}

func (rt *Runtime) waitStorageTransfer(ctx context.Context, run *runState, hotkeys <-chan input.HotkeyEvent, ticker *time.Ticker, cancel context.CancelFunc, scope string, after uint64, observe func(world.State) (bool, error)) (world.State, error) {
	remaining := 3 * time.Second
	last := time.Now()
	for {
		select {
		case <-ctx.Done():
			return world.State{}, context.Canceled
		case event := <-hotkeys:
			rt.handleHotkeyEvent(event, cancel)
		case <-ticker.C:
			now := time.Now()
			status := rt.Input.Status()
			if status.Stopped {
				return world.State{}, context.Canceled
			}
			if !status.Paused {
				remaining -= now.Sub(last)
			}
			last = now
			if remaining <= 0 {
				return world.State{}, fmt.Errorf("Materialaktion nicht bestätigt; keine Wiederholung, bitte Truhe und Würfel prüfen")
			}
			if err := rt.runTick(ctx, run); err != nil {
				return world.State{}, fmt.Errorf("storage transfer observation: %w", err)
			}
			if run.hasEverAttached && !run.attached {
				return world.State{}, fmt.Errorf("process lost during storage transfer")
			}
			if status.Paused {
				continue
			}
			s := rt.World.Current()
			if !s.CollectionAvailable() || s.Generation <= after {
				continue
			}
			if s.Collection.ScopeID != scope {
				return world.State{}, fmt.Errorf("storage scope changed during transfer")
			}
			if !s.UI.StashOpen || !s.UI.InventoryOpen || s.UI.NPCShopOpen || s.UI.NPCInteractOpen || s.UI.QuitMenuOpen {
				return world.State{}, fmt.Errorf("Truhenansicht während des Tests verändert")
			}
			complete, err := observe(s)
			if err != nil {
				return world.State{}, err
			}
			if complete {
				return s, nil
			}
		}
	}
}

func storageTransferUnchanged(s, before world.State) bool {
	return len(s.ItemsByLocation(world.ItemLocationCube)) == 0 && storageTransferInventoryUnchanged(s, before)
}

func storageTransferInventoryUnchanged(s, before world.State) bool {
	if len(s.ItemsByLocation(world.ItemLocationCursor)) != 0 {
		return false
	}
	items, original := s.InventoryItems(), before.InventoryItems()
	if len(items) != len(original) {
		return false
	}
	for _, item := range items {
		found := false
		for _, old := range original {
			if item.UnitID == old.UnitID && item.Code == old.Code && item.GridX == old.GridX && item.GridY == old.GridY {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func storageTransferConfirmed(s, before world.State, tab world.StorageTab, code string, wantCount int, loaded bool) (bool, error) {
	if !storageTransferInventoryUnchanged(s, before) {
		return false, fmt.Errorf("Cursor oder persönliches Inventar während des Transfers verändert")
	}
	if !s.Collection.TabKnown {
		return false, nil
	}
	if s.Collection.Tab != tab {
		return false, fmt.Errorf("Materialtab während des Transfers verändert")
	}
	count, known := s.CollectionCount(code)
	if !known {
		return false, nil
	}
	items := s.ItemsByLocation(world.ItemLocationCube)
	if len(items) > 1 {
		return false, fmt.Errorf("unerwarteter Würfelinhalt")
	}
	if len(items) == 1 {
		item := items[0]
		if item.UnitID == 0 || !item.PlayerOwned || item.Code != code || item.Width != 1 || item.Height != 1 || item.GridX < 0 || item.GridX >= 3 || item.GridY < 0 || item.GridY >= 4 {
			return false, fmt.Errorf("unerwartetes Material im Würfel")
		}
	}
	return count == wantCount && ((loaded && len(items) == 1) || (!loaded && len(items) == 0)), nil
}
