package app

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/input"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/memory"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/profile"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/tasks"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

type runActionsAdapter struct {
	log      *slog.Logger
	input    inputController
	bindings configBindingSource
	combat   *combatAdapter
}

func newRunActionsAdapter(log *slog.Logger, in inputController, bindings configBindingSource, combat *combatAdapter) *runActionsAdapter {
	return &runActionsAdapter{
		log:      log.With("component", "run_actions"),
		input:    in,
		bindings: bindings,
		combat:   combat,
	}
}

func (a *runActionsAdapter) CastBelt(slot int) error {
	if err := a.input.CastBelt(a.bindings, slot); err != nil {
		return fmt.Errorf("run cast belt slot %d: %w", slot, err)
	}
	a.log.Debug("run belt cast", "slot", slot)
	return nil
}

func (a *runActionsAdapter) CastTownPortal(now time.Time, state world.State) error {
	if townPortalTomeKnownEmpty(state) {
		return tasks.ErrTownPortalSupplyEmpty
	}
	player := state.Player
	win, ok := a.input.Window()
	if !ok {
		return fmt.Errorf("run town portal: window not bound")
	}
	clientX := win.ClientWidth / 2
	clientY := win.ClientHeight / 2
	if a.combat == nil || a.combat.selector == nil {
		return fmt.Errorf("run town portal: skill selector not wired")
	}
	combatInput, ok := a.input.(verifiedCombatInput)
	if !ok {
		return fmt.Errorf("run town portal: verified input not wired")
	}
	sent, err := a.combat.selector.EnsureAndCast(memory.SkillTownPortal, player.RightSkillID, now, func() error {
		if moveErr := a.input.MoveTo(clientX, clientY); moveErr != nil {
			return fmt.Errorf("run town portal aim: %w", moveErr)
		}
		if clickErr := combatInput.Click(input.MouseRight); clickErr != nil {
			return fmt.Errorf("run town portal click: %w", clickErr)
		}
		a.log.Debug("run town portal cast", "client_x", clientX, "client_y", clientY)
		return nil
	})
	if err != nil {
		return err
	}
	if !sent {
		return profile.ErrSkillSelectionPending
	}
	return nil
}

func townPortalTomeKnownEmpty(state world.State) bool {
	bookID := memory.MustSkillID("book_of_townportal")
	if state.Player.RightSkillID != bookID {
		return false
	}
	found := false
	total := 0
	for _, item := range state.InventoryItems() {
		if item.Code != "tbk" {
			continue
		}
		found = true
		if !item.QuantityKnown {
			return false
		}
		total += item.Quantity
	}
	return found && total <= 0
}
