// Package necrobonespear provides the first concrete combat-profile strategy module.
package necrobonespear

import (
	"fmt"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/memory"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/profile"
)

const profileID = "necro_bone_spear"

// NewBossFactory returns a Bone-Spear boss strategy for the named run.
func NewBossFactory(runID string) profile.StrategyFactory {
	return func() profile.RunStrategy {
		return &bossStrategy{runID: runID}
	}
}

// NewLowerKurastFactory returns the Bone-Spear Lower-Kurast strategy. It binds
// Amplify Damage followed by Bone Spear only for local object-blocker recovery;
// travel remains without route-clear capability.
func NewLowerKurastFactory() profile.StrategyFactory {
	return func() profile.RunStrategy {
		return &lowerKurastStrategy{}
	}
}

// NewNihlathakFactory returns the Bone-Spear boss strategy with post-boss AD/Bone-Spear clear.
// Travel remains without route_clear capability; only clear_nearby_hostiles uses RouteClear.
func NewNihlathakFactory() profile.StrategyFactory {
	return func() profile.RunStrategy {
		return &nihlathakStrategy{}
	}
}

// NewSummonerFactory returns the AD/Bone-Spear route-clear + boss strategy.
func NewSummonerFactory() profile.StrategyFactory {
	return func() profile.RunStrategy {
		return &summonerStrategy{}
	}
}

// NewCowsFactory returns the AD/Bone-Spear/CE Cow-hold strategy.
func NewCowsFactory() profile.StrategyFactory {
	return func() profile.RunStrategy {
		return &cowsStrategy{}
	}
}

type bossStrategy struct {
	runID string
}

type lowerKurastStrategy struct{}

func (s *lowerKurastStrategy) ProfileID() string { return profileID }
func (s *lowerKurastStrategy) RunID() string     { return "lower-kurast" }
func (s *lowerKurastStrategy) RequiredSkills() []string {
	return []string{"teleport", "town_portal", "bone_spear", "amplify_damage", "bone_armor"}
}

// RequiresRouteClear reports false because Lower Kurast uses combat only after
// a Memory-confirmed monster blocks an object or loot hover probe.
func (s *lowerKurastStrategy) RequiresRouteClear() bool { return false }

func (s *lowerKurastStrategy) Configure(exec *profile.Executor, standardAttackID uint16, routeClear profile.RouteCombatActions) error {
	if exec == nil || routeClear == nil || standardAttackID != memory.SkillBoneSpear {
		return fmt.Errorf("necro bone spear lower-kurast strategy requires executor, route clear and Bone Spear standard attack")
	}
	return exec.ConfigureRouteClear(profile.RouteClearSingleTarget, memory.SkillAmplifyDamage, standardAttackID, routeClear)
}

func (s *bossStrategy) ProfileID() string { return profileID }
func (s *bossStrategy) RunID() string     { return s.runID }
func (s *bossStrategy) RequiredSkills() []string {
	return []string{"teleport", "town_portal", "bone_spear", "bone_armor", "bone_prison"}
}
func (s *bossStrategy) Configure(exec *profile.Executor, _ uint16, _ profile.RouteCombatActions) error {
	if exec == nil {
		return fmt.Errorf("necro bone spear boss strategy requires executor")
	}
	return nil
}

type nihlathakStrategy struct{}

func (s *nihlathakStrategy) ProfileID() string { return profileID }
func (s *nihlathakStrategy) RunID() string     { return "nihlathak" }
func (s *nihlathakStrategy) RequiredSkills() []string {
	return []string{"teleport", "town_portal", "bone_spear", "amplify_damage", "bone_armor", "bone_prison"}
}

// RequiresRouteClear reports false because Nihlathak has no travel route_clear
// capability; Configure still binds RouteClear for post-boss cleanup only.
func (s *nihlathakStrategy) RequiresRouteClear() bool { return false }

func (s *nihlathakStrategy) Configure(exec *profile.Executor, standardAttackID uint16, routeClear profile.RouteCombatActions) error {
	if exec == nil || routeClear == nil || standardAttackID == 0 {
		return fmt.Errorf("necro bone spear nihlathak strategy requires executor, route clear and standard attack")
	}
	return exec.ConfigureRouteClear(profile.RouteClearSingleTarget, memory.SkillAmplifyDamage, standardAttackID, routeClear)
}

type summonerStrategy struct{}

func (s *summonerStrategy) ProfileID() string { return profileID }
func (s *summonerStrategy) RunID() string     { return "summoner" }
func (s *summonerStrategy) RequiredSkills() []string {
	return []string{"teleport", "town_portal", "bone_spear", "amplify_damage", "bone_armor"}
}
func (s *summonerStrategy) RequiresRouteClear() bool { return true }
func (s *summonerStrategy) Configure(exec *profile.Executor, standardAttackID uint16, routeClear profile.RouteCombatActions) error {
	if exec == nil || routeClear == nil || standardAttackID == 0 {
		return fmt.Errorf("necro bone spear summoner strategy requires executor, route clear and standard attack")
	}
	return exec.ConfigureRouteClear(profile.RouteClearSingleTarget, memory.SkillAmplifyDamage, standardAttackID, routeClear)
}

type cowsStrategy struct{}

func (s *cowsStrategy) ProfileID() string { return profileID }
func (s *cowsStrategy) RunID() string     { return "cows" }
func (s *cowsStrategy) RequiredSkills() []string {
	return []string{"teleport", "town_portal", "bone_spear", "amplify_damage", "corpse_explosion", "bone_armor"}
}
func (s *cowsStrategy) RequiresRouteClear() bool      { return true }
func (s *cowsStrategy) RequiresCorpseExplosion() bool { return true }
func (s *cowsStrategy) Configure(exec *profile.Executor, standardAttackID uint16, routeClear profile.RouteCombatActions) error {
	if exec == nil || routeClear == nil || standardAttackID == 0 {
		return fmt.Errorf("necro bone spear cows strategy requires executor, route clear and standard attack")
	}
	if err := exec.ConfigureCorpseExplosion(memory.SkillCorpseExplosion); err != nil {
		return err
	}
	return exec.ConfigureRouteClear(profile.RouteClearSingleTarget, memory.SkillAmplifyDamage, standardAttackID, routeClear)
}
