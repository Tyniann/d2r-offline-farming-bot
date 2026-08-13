package app

import (
	"fmt"
	"strings"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
)

// NewCharacterWorkflowRuntime builds an isolated desktop route workflow runtime
// with a frozen Schema-3 loadout for the currently selected character.
func NewCharacterWorkflowRuntime(cfg *config.Config, resolver *CharacterLoadoutResolver, character string) (*Runtime, error) {
	if cfg == nil {
		return nil, fmt.Errorf("character workflow runtime requires config")
	}
	if resolver == nil {
		return nil, fmt.Errorf("character workflow runtime requires loadout resolver")
	}
	character = strings.TrimSpace(character)
	if character == "" {
		return nil, fmt.Errorf("character workflow runtime requires a selected character")
	}

	loadout, err := resolver.Resolve(character)
	if err != nil {
		return nil, fmt.Errorf("resolve character workflow loadout: %w", err)
	}

	workflowConfig := *cfg
	workflowConfig.Session = cfg.Session
	workflowConfig.Session.Character = character
	workflowConfig.Session.Enabled = false
	frozenLoadout := CloneCharacterLoadoutSnapshot(loadout)
	runtime, err := New(&workflowConfig, Options{Desktop: true, Loadout: &frozenLoadout})
	if err != nil {
		return nil, fmt.Errorf("build character workflow runtime: %w", err)
	}
	return runtime, nil
}
