// Package hammerdin provides the Paladin Hammerdin profile strategy module.
package hammerdin

import (
	"fmt"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/memory"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/profile"
)

const profileID = "paladin_hammerdin"

// NewMephistoFactory returns the first registered Hammerdin strategy.
func NewMephistoFactory() profile.StrategyFactory {
	return func() profile.RunStrategy { return &mephistoStrategy{} }
}

type mephistoStrategy struct{}

func (s *mephistoStrategy) ProfileID() string { return profileID }
func (s *mephistoStrategy) RunID() string     { return "mephisto" }
func (s *mephistoStrategy) RequiredSkills() []string {
	return []string{"teleport", "town_portal", "blessed_hammer", "concentration", "holy_shield"}
}
func (s *mephistoStrategy) Configure(exec *profile.Executor, standardAttackID uint16, _ profile.RouteCombatActions) error {
	if exec == nil || standardAttackID != memory.MustSkillID("blessed_hammer") {
		return fmt.Errorf("hammerdin mephisto strategy requires executor and Blessed Hammer standard attack")
	}
	// Town-ready CTA/Holy Shield is owned by the app-layer town_ready wrapper.
	// Standard attack (close teleport, then confirmed LMB) is owned by
	// the shared Mephisto pipeline, not by encounter hooks.
	return nil
}
