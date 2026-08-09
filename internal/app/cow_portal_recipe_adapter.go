package app

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/input"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/loot"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/pathing"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/tasks"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

const cowPermanentPortalClickDistance = 15

// cowPortalRecipeAdapter binds the narrow loot executor to guarded input and
// the established Memory-hover entity clicker.
type cowPortalRecipeAdapter struct {
	controller inputController
	clicker    *pathing.EntityClicker
	executor   *loot.CowPortalRecipe
}

func newCowPortalRecipeAdapter(log *slog.Logger, controller inputController, pathCfg pathing.Config, cfg config.LootConfig) (*cowPortalRecipeAdapter, error) {
	adapter := &cowPortalRecipeAdapter{
		controller: controller,
		clicker:    pathing.NewEntityClicker(log, controller, pathCfg.Projector(), pathCfg.Click),
	}
	recipe, err := loot.NewCowPortalRecipe(log, adapter, loot.CowPortalRecipeConfig{
		CubeOpenTimeout: time.Duration(cfg.CowPortalRecipe.CubeOpenTimeoutMs) * time.Millisecond,
		TransferTimeout: time.Duration(cfg.CowPortalRecipe.TransferTimeoutMs) * time.Millisecond,
		ResultTimeout:   time.Duration(cfg.CowPortalRecipe.ResultTimeoutMs) * time.Millisecond,
		PortalTimeout:   time.Duration(cfg.CowPortalRecipe.PortalTimeoutMs) * time.Millisecond,
		CloseTimeout:    time.Duration(cfg.CowPortalRecipe.CloseTimeoutMs) * time.Millisecond,
		EntryTimeout:    time.Duration(cfg.CowPortalRecipe.EntryTimeoutMs) * time.Millisecond,
		InventoryLeft:   cfg.Stash.InventoryLeft,
		InventoryTop:    cfg.Stash.InventoryTop,
		InventoryCellW:  cfg.Stash.InventoryCellW,
		InventoryCellH:  cfg.Stash.InventoryCellH,
		TransmuteX:      cfg.CowPortalRecipe.TransmuteX,
		TransmuteY:      cfg.CowPortalRecipe.TransmuteY,
	})
	if err != nil {
		return nil, err
	}
	adapter.executor = recipe
	return adapter, nil
}

func (a *cowPortalRecipeAdapter) Tick(state world.State, now time.Time, legUnitID, tomeUnitID, cubeUnitID uint32) tasks.CowSetupActionResult {
	result := a.executor.Tick(state, now, loot.CowPortalBinding{LegUnitID: legUnitID, TomeUnitID: tomeUnitID, CubeUnitID: cubeUnitID})
	return tasks.CowSetupActionResult{Done: result.Done, Reason: result.Reason, UnitID: result.PortalUnitID, ProgressKind: result.ProgressKind}
}

func (a *cowPortalRecipeAdapter) Reset() {
	if a != nil && a.executor != nil {
		a.executor.Reset()
	}
}

func (a *cowPortalRecipeAdapter) Window() (input.WindowInfo, bool) { return a.controller.Window() }
func (a *cowPortalRecipeAdapter) Focus() error                     { return a.controller.Focus() }
func (a *cowPortalRecipeAdapter) MoveTo(x, y int) error            { return a.controller.MoveTo(x, y) }
func (a *cowPortalRecipeAdapter) Click(button input.MouseButton) error {
	return a.controller.Click(button)
}
func (a *cowPortalRecipeAdapter) ClickWithModifier(modifier string, button input.MouseButton) error {
	return a.controller.ClickWithModifier(modifier, button)
}
func (a *cowPortalRecipeAdapter) PressKey(key string) error { return a.controller.PressKey(key) }

func (a *cowPortalRecipeAdapter) TickPermanentPortal(state world.State, portal world.Object) (loot.CowPortalClickResult, error) {
	if portal.UnitID == 0 || portal.Kind != world.ObjectKindPermanentPortal {
		return loot.CowPortalClickResult{Done: true, Reason: "binding_invalid"}, nil
	}
	result, err := a.clicker.Tick(state, pathing.ClickTarget{
		UnitID: portal.UnitID, UnitType: world.HoverUnitTypeObject, Position: portal.Position, Name: portal.Name,
	}, cowPermanentPortalClickDistance)
	if err != nil {
		return loot.CowPortalClickResult{}, fmt.Errorf("click Cow portal %d: %w", portal.UnitID, err)
	}
	if result.Status == pathing.ClickHit {
		return loot.CowPortalClickResult{Clicked: true, Done: true}, nil
	}
	if result.Done {
		return loot.CowPortalClickResult{Done: true, Reason: string(result.Status)}, nil
	}
	return loot.CowPortalClickResult{}, nil
}

func (a *cowPortalRecipeAdapter) ResetPermanentPortal() {
	if a != nil && a.clicker != nil {
		a.clicker.Reset()
	}
}

var _ tasks.CowPortalRecipeActions = (*cowPortalRecipeAdapter)(nil)
var _ loot.CowPortalRecipeInput = (*cowPortalRecipeAdapter)(nil)
