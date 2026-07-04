package app

import (
	"fmt"
	"log/slog"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/memory"
)

type runActionsAdapter struct {
	log      *slog.Logger
	input    inputController
	bindings configBindingSource
}

func newRunActionsAdapter(log *slog.Logger, in inputController, bindings configBindingSource) *runActionsAdapter {
	return &runActionsAdapter{
		log:      log.With("component", "run_actions"),
		input:    in,
		bindings: bindings,
	}
}

func (a *runActionsAdapter) CastBelt(slot int) error {
	if err := a.input.CastBelt(a.bindings, slot); err != nil {
		return fmt.Errorf("run cast belt slot %d: %w", slot, err)
	}
	a.log.Debug("run belt cast", "slot", slot)
	return nil
}

func (a *runActionsAdapter) CastTownPortal() error {
	win, ok := a.input.Window()
	if !ok {
		return fmt.Errorf("run town portal: window not bound")
	}
	clientX := win.ClientWidth / 2
	clientY := win.ClientHeight / 2
	if err := a.input.CastSkillAt(a.bindings, memory.SkillTownPortal, clientX, clientY); err != nil {
		return fmt.Errorf("run town portal: %w", err)
	}
	a.log.Debug("run town portal cast", "client_x", clientX, "client_y", clientY)
	return nil
}
