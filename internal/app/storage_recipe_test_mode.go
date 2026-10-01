package app

import (
	"context"
	"fmt"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/crafting"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/input"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/loot"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

// runStorageRecipeTest is the Gate 25.8 acceptance path, bounded to one real
// recipe. It drives the productive executor, keeps hotkeys active and never
// starts a farming task or performs cleanup after an unconfirmed action.
func (rt *Runtime) runStorageRecipeTest(ctx context.Context, run *runState, hotkeys <-chan input.HotkeyEvent, ticker *time.Ticker, cancel context.CancelFunc, action inputTestAction) error {
	in, ok := rt.Input.(loot.CollectionInput)
	if !ok {
		return fmt.Errorf("storage recipe: positioned input unavailable")
	}
	e, err := crafting.NewRecipeTestExecutor(rt.Log, loot.NewCollectionActions(rt.Log, in, loot.DefaultCollectionLayout()), action.code)
	if err != nil {
		return err
	}
	before := rt.World.Current()
	request := crafting.Request{Code: action.code, RunGeneration: before.Generation}
	recipe, _ := crafting.LookupRecipe(action.code)
	source, sourceKnown := before.CollectionCount(action.code)
	target, targetKnown := before.CollectionCount(recipe.OutputCode)
	rt.Log.Info("storage recipe test started", "input_code", action.code, "output_code", recipe.OutputCode, "input_before", source, "input_known", sourceKnown, "output_before", target, "output_known", targetKnown, "scope", before.Collection.ScopeID, "initial_tab", before.Collection.Tab, "tab_known", before.Collection.TabKnown, "pause_before_transmute", action.pauseBeforeTransmute)
	pausedOnce := false
	for {
		select {
		case <-ctx.Done():
			return context.Canceled
		case event := <-hotkeys:
			rt.handleHotkeyEvent(event, cancel)
		case <-ticker.C:
			if err := rt.runTick(ctx, run); err != nil {
				return fmt.Errorf("storage recipe observation: %w", err)
			}
			if run.hasEverAttached && !run.attached {
				return fmt.Errorf("storage recipe: process lost; no cleanup input")
			}
			state := rt.World.Current()
			status := rt.Input.Status()
			result := e.Tick(state, time.Now(), request, status.Paused, status.Stopped)
			if result.Done {
				if result.Failure != nil {
					return fmt.Errorf("storage recipe test: %w", result.Failure)
				}
				rt.Log.Info("storage recipe test completed", "input_code", action.code, "output_code", recipe.OutputCode, "input_before", source, "input_after", source-3, "output_before", target, "output_after", target+1, "pause_confirmed", pausedOnce, "cube_empty", len(state.ItemsByLocation(world.ItemLocationCube)) == 0, "cursor_empty", len(state.ItemsByLocation(world.ItemLocationCursor)) == 0, "inventory_unchanged", true, "scope", state.Collection.ScopeID, "generation", state.Generation)
				return nil
			}
			if action.pauseBeforeTransmute && !pausedOnce && !status.Paused && e.ReadyToTransmute() {
				if !rt.Input.TogglePause("storage_recipe_before_transmute") {
					return fmt.Errorf("storage recipe: pause revoked")
				}
				pausedOnce = true
				rt.Log.Info("storage recipe test paused before transmute", "hint", "Drei Zutaten bestätigt. Zum Fortsetzen die konfigurierte Pause-Taste drücken; zum Abbrechen F11.", "input_code", action.code)
			}
		}
	}
}
