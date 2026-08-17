// Package hammerdin provides the Paladin Hammerdin profile strategy module.
package hammerdin

import (
	"fmt"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/memory"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/profile"
)

const profileID = "paladin_hammerdin"

// NewBossFactory returns the Hammerdin strategy for the named run.
// Combat stays in the shared pipeline standard-attack path; this factory
// only registers the profile/run pair and required skills.
func NewBossFactory(runID string) profile.StrategyFactory {
	return func() profile.RunStrategy {
		return &bossStrategy{runID: runID}
	}
}

type bossStrategy struct {
	runID string
}

func (s *bossStrategy) ProfileID() string { return profileID }
func (s *bossStrategy) RunID() string     { return s.runID }
func (s *bossStrategy) RequiredSkills() []string {
	return []string{"teleport", "town_portal", "blessed_hammer", "concentration", "holy_shield"}
}
func (s *bossStrategy) Configure(exec *profile.Executor, standardAttackID uint16, _ profile.RouteCombatActions) error {
	if exec == nil || standardAttackID != memory.MustSkillID("blessed_hammer") {
		return fmt.Errorf("hammerdin strategy requires executor and Blessed Hammer standard attack")
	}
	// Town-ready CTA/Holy Shield is owned by the app-layer town_ready wrapper.
	// Standard attack (close teleport, then confirmed LMB) is owned by
	// the shared boss pipeline, not by encounter hooks. Nihlathak does not
	// bind RouteClear: Blessed Hammer already clears nearby hostiles.
	return nil
}
