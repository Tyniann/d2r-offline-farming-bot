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

// NewSummonerFactory returns the Hammerdin Summoner strategy. Travel uses the
// shared route-clear hold; attacks stay on the Blessed Hammer standard-attack
// path instead of a Necromancer curse opener.
func NewSummonerFactory() profile.StrategyFactory {
	return func() profile.RunStrategy {
		return &summonerStrategy{}
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
	// the shared boss pipeline, not by encounter hooks. Countess, Mephisto,
	// and Nihlathak do not bind RouteClear: those runs have no travel combat.
	return nil
}

type summonerStrategy struct{}

func (s *summonerStrategy) ProfileID() string { return profileID }
func (s *summonerStrategy) RunID() string     { return "summoner" }
func (s *summonerStrategy) RequiredSkills() []string {
	return []string{"teleport", "town_portal", "blessed_hammer", "concentration", "holy_shield"}
}
func (s *summonerStrategy) RequiresRouteClear() bool { return true }
func (s *summonerStrategy) Configure(exec *profile.Executor, standardAttackID uint16, routeClear profile.RouteCombatActions) error {
	if exec == nil || routeClear == nil || standardAttackID != memory.MustSkillID("blessed_hammer") {
		return fmt.Errorf("hammerdin summoner strategy requires executor, route clear and Blessed Hammer standard attack")
	}
	// No curse opener: Blessed Hammer is the only route-clear attack. The
	// combat adapter holds LMB after a close teleport, matching Mephisto.
	return exec.ConfigureRouteClear(profile.RouteClearSingleTarget, 0, standardAttackID, routeClear)
}
