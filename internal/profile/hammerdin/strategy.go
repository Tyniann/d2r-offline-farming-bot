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

// NewLowerKurastFactory returns the Hammerdin Lower-Kurast strategy. It wires
// stationary Blessed Hammer combat only for object-blocker recovery; travel
// remains without route-clear capability.
func NewLowerKurastFactory() profile.StrategyFactory {
	return func() profile.RunStrategy {
		return &lowerKurastStrategy{}
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

// NewCowsFactory returns the Hammerdin Cow strategy. Shared Cow preflight,
// Wirt setup and cube recipe stay in the Cow pipeline; sweep combat reuses
// the Blessed Hammer route-clear hold and does not declare Corpse Explosion.
func NewCowsFactory() profile.StrategyFactory {
	return func() profile.RunStrategy {
		return &cowsStrategy{}
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

// RequiresRouteClear reports false because these runs use local recovery
// combat without authorizing attacks during route playback.
func (s *bossStrategy) RequiresRouteClear() bool { return false }

func (s *bossStrategy) SupportsLocalRecoveryClear() {}

func (s *bossStrategy) Configure(exec *profile.Executor, standardAttackID uint16, routeClear profile.RouteCombatActions) error {
	if exec == nil || routeClear == nil || standardAttackID != memory.MustSkillID("blessed_hammer") {
		return fmt.Errorf("hammerdin strategy requires executor, local recovery clear and Blessed Hammer standard attack")
	}
	// Town-ready CTA/Holy Shield is owned by the app-layer town_ready wrapper.
	// Standard attack (close teleport, then confirmed LMB) is owned by
	// the shared boss pipeline, not by encounter hooks. Countess, Mephisto,
	// and Nihlathak do not authorize route playback combat.
	return exec.ConfigureRouteClear(profile.RouteClearSingleTarget, 0, standardAttackID, routeClear)
}

type lowerKurastStrategy struct{}

func (s *lowerKurastStrategy) ProfileID() string { return profileID }
func (s *lowerKurastStrategy) RunID() string     { return "lower-kurast" }
func (s *lowerKurastStrategy) RequiredSkills() []string {
	return []string{"teleport", "town_portal", "blessed_hammer", "concentration", "holy_shield"}
}

// RequiresRouteClear reports false because Lower Kurast uses stationary combat
// only after a monster blocks a chest hover probe.
func (s *lowerKurastStrategy) RequiresRouteClear() bool { return false }

func (s *lowerKurastStrategy) Configure(exec *profile.Executor, standardAttackID uint16, routeClear profile.RouteCombatActions) error {
	if exec == nil || routeClear == nil || standardAttackID != memory.MustSkillID("blessed_hammer") {
		return fmt.Errorf("hammerdin lower-kurast strategy requires executor, route clear and Blessed Hammer standard attack")
	}
	return exec.ConfigureRouteClear(profile.RouteClearSingleTarget, 0, standardAttackID, routeClear)
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

type cowsStrategy struct{}

func (s *cowsStrategy) ProfileID() string { return profileID }
func (s *cowsStrategy) RunID() string     { return "cows" }
func (s *cowsStrategy) RequiredSkills() []string {
	return []string{"teleport", "town_portal", "blessed_hammer", "concentration", "holy_shield"}
}
func (s *cowsStrategy) RequiresRouteClear() bool { return true }
func (s *cowsStrategy) Configure(exec *profile.Executor, standardAttackID uint16, routeClear profile.RouteCombatActions) error {
	if exec == nil || routeClear == nil || standardAttackID != memory.MustSkillID("blessed_hammer") {
		return fmt.Errorf("hammerdin cows strategy requires executor, route clear and Blessed Hammer standard attack")
	}
	// Cow setup (Wirt, leg, cube recipe) is owned by the shared cow pipeline.
	// Sweep combat matches Summoner: Blessed Hammer LMB hold, no curse opener
	// and no Corpse Explosion capability for the CE cow-hold wrapper.
	return exec.ConfigureRouteClear(profile.RouteClearSingleTarget, 0, standardAttackID, routeClear)
}
