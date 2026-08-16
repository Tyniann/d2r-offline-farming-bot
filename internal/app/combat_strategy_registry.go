package app

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/profile"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/profile/hammerdin"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/profile/necrobonespear"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/tasks"
)

const (
	// ReasonProfileRunStrategyUnavailable reports that the active combat profile
	// has no registered executable strategy for the requested run.
	ReasonProfileRunStrategyUnavailable = "profile_run_strategy_unavailable"
)

// CombatStrategyRegistry maps (profileID, runID) to an executable strategy factory.
// It owns the only productive run-support authority for combat profiles.
type CombatStrategyRegistry struct {
	factories map[string]map[string]profile.StrategyFactory
}

// NewCombatStrategyRegistry builds the product registry with all released profile/run pairs.
func NewCombatStrategyRegistry() *CombatStrategyRegistry {
	registry := &CombatStrategyRegistry{factories: map[string]map[string]profile.StrategyFactory{}}
	registry.mustRegister(necrobonespear.NewBossFactory(string(tasks.RunIDCountess)))
	registry.mustRegister(necrobonespear.NewBossFactory(string(tasks.RunIDMephisto)))
	registry.mustRegister(necrobonespear.NewNihlathakFactory())
	registry.mustRegister(necrobonespear.NewSummonerFactory())
	registry.mustRegister(necrobonespear.NewCowsFactory())
	registry.mustRegister(hammerdin.NewMephistoFactory())
	return registry
}

func (r *CombatStrategyRegistry) mustRegister(factory profile.StrategyFactory) {
	if err := r.Register(factory); err != nil {
		panic(err)
	}
}

// Register adds one strategy factory after validating its self-description.
func (r *CombatStrategyRegistry) Register(factory profile.StrategyFactory) error {
	if r == nil {
		return fmt.Errorf("combat strategy registry is nil")
	}
	if factory == nil {
		return fmt.Errorf("combat strategy factory is nil")
	}
	strategy := factory()
	if strategy == nil {
		return fmt.Errorf("combat strategy factory returned nil")
	}
	profileID := strings.TrimSpace(strategy.ProfileID())
	runID := strings.TrimSpace(strategy.RunID())
	if profileID == "" || runID == "" {
		return fmt.Errorf("combat strategy factory must declare profile and run ids")
	}
	if _, ok := tasks.DefaultRunRegistry().Definition(tasks.RunID(runID)); !ok {
		return fmt.Errorf("combat strategy run %q is unknown", runID)
	}
	if r.factories[profileID] == nil {
		r.factories[profileID] = map[string]profile.StrategyFactory{}
	}
	if _, exists := r.factories[profileID][runID]; exists {
		return fmt.Errorf("combat strategy for profile %q run %q is already registered", profileID, runID)
	}
	r.factories[profileID][runID] = factory
	return nil
}

// Resolve returns the factory for one profile/run pair.
func (r *CombatStrategyRegistry) Resolve(profileID, runID string) (profile.StrategyFactory, bool) {
	if r == nil {
		return nil, false
	}
	byRun, ok := r.factories[strings.TrimSpace(profileID)]
	if !ok {
		return nil, false
	}
	factory, ok := byRun[strings.TrimSpace(runID)]
	return factory, ok
}

// SupportedRuns returns sorted run IDs registered for profileID.
func (r *CombatStrategyRegistry) SupportedRuns(profileID string) []string {
	byRun := r.factories[strings.TrimSpace(profileID)]
	out := make([]string, 0, len(byRun))
	for runID := range byRun {
		out = append(out, runID)
	}
	sort.Strings(out)
	return out
}

// DefaultCombatStrategyRegistry returns the process-wide product strategy registry.
func DefaultCombatStrategyRegistry() *CombatStrategyRegistry {
	return defaultCombatStrategyRegistry
}

var defaultCombatStrategyRegistry = NewCombatStrategyRegistry()

// ValidateAgainstProfiles checks registered factories against profile contracts.
func (r *CombatStrategyRegistry) ValidateAgainstProfiles(profiles config.ProfilesConfig) error {
	if r == nil {
		return fmt.Errorf("combat strategy registry is nil")
	}
	for profileID, byRun := range r.factories {
		profileCfg, ok := profiles[profileID]
		if !ok {
			return fmt.Errorf("combat strategy profile %q is missing from combat_profiles", profileID)
		}
		required := map[string]struct{}{}
		for _, entry := range profileCfg.RequiredSkills {
			required[entry.Skill] = struct{}{}
		}
		for runID, factory := range byRun {
			if factory == nil {
				return fmt.Errorf("combat strategy factory for profile %q run %q is nil", profileID, runID)
			}
			strategy := factory()
			if strategy == nil {
				return fmt.Errorf("combat strategy factory for profile %q run %q returned nil", profileID, runID)
			}
			if strategy.ProfileID() != profileID || strategy.RunID() != runID {
				return fmt.Errorf("combat strategy factory for profile %q run %q returned mismatched identity", profileID, runID)
			}
			for _, skill := range strategy.RequiredSkills() {
				if _, ok := required[skill]; !ok {
					return fmt.Errorf("combat strategy profile %q run %q requires skill %q outside required_skills", profileID, runID, skill)
				}
			}
			if needsClear, ok := strategy.(profile.SupportsRouteClear); ok && needsClear.RequiresRouteClear() {
				definition, definitionOK := tasks.DefaultRunRegistry().Definition(tasks.RunID(runID))
				if !definitionOK || !definition.HasCapability(tasks.RunCapabilityRouteClear) {
					return fmt.Errorf("combat strategy profile %q run %q requires route clear capability", profileID, runID)
				}
			}
		}
	}
	return nil
}
