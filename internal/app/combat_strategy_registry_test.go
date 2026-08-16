package app

import (
	"strings"
	"testing"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/profile"
)

func TestCombatStrategyRegistryResolvesBoneSpearMatrix(t *testing.T) {
	registry := NewCombatStrategyRegistry()
	runs := []string{"countess", "mephisto", "summoner", "nihlathak", "cows"}
	for _, runID := range runs {
		factory, ok := registry.Resolve("necro_bone_spear", runID)
		if !ok || factory == nil {
			t.Fatalf("missing factory for %s", runID)
		}
		strategy := factory()
		if strategy.ProfileID() != "necro_bone_spear" || strategy.RunID() != runID {
			t.Fatalf("strategy identity = %s/%s", strategy.ProfileID(), strategy.RunID())
		}
	}
	nihlathak := registry.factories["necro_bone_spear"]["nihlathak"]()
	clear, ok := nihlathak.(profile.SupportsRouteClear)
	if !ok || clear.RequiresRouteClear() {
		t.Fatal("nihlathak must wire post-boss route clear without travel capability")
	}
	if _, ok := registry.Resolve("unknown_profile", "countess"); ok {
		t.Fatal("unknown profile resolved")
	}
	if _, ok := registry.Resolve("necro_bone_spear", "baal"); ok {
		t.Fatal("unknown run resolved")
	}
	got := registry.SupportedRuns("necro_bone_spear")
	if len(got) != 5 {
		t.Fatalf("supported runs = %v", got)
	}
}

func TestCombatStrategyRegistryExposesOnlyHammerdinMephisto(t *testing.T) {
	registry := NewCombatStrategyRegistry()
	factory, ok := registry.Resolve("paladin_hammerdin", "mephisto")
	if !ok || factory == nil {
		t.Fatal("Hammerdin Mephisto strategy is missing")
	}
	strategy := factory()
	if strategy.ProfileID() != "paladin_hammerdin" || strategy.RunID() != "mephisto" {
		t.Fatalf("strategy identity = %s/%s", strategy.ProfileID(), strategy.RunID())
	}
	if got := registry.SupportedRuns("paladin_hammerdin"); len(got) != 1 || got[0] != "mephisto" {
		t.Fatalf("Hammerdin supported runs = %v", got)
	}
	for _, runID := range []string{"countess", "summoner", "nihlathak", "cows"} {
		if _, exists := registry.Resolve("paladin_hammerdin", runID); exists {
			t.Fatalf("Hammerdin unexpectedly supports %s", runID)
		}
	}
	if got := registry.SupportedRuns("necro_bone_spear"); len(got) != 5 {
		t.Fatalf("existing Bone-Spear registry changed: %v", got)
	}
}

func TestCombatStrategyRegistryValidateAgainstProfiles(t *testing.T) {
	cfg, err := config.Load("../../configs/config.example.yaml")
	if err != nil {
		t.Fatal(err)
	}
	registry := NewCombatStrategyRegistry()
	if validateErr := registry.ValidateAgainstProfiles(cfg.Profiles); validateErr != nil {
		t.Fatal(validateErr)
	}
	broken := cfg.Profiles
	value := broken["necro_bone_spear"]
	filtered := make([]config.RequiredSkillConfig, 0, len(value.RequiredSkills))
	for _, skill := range value.RequiredSkills {
		if skill.Skill != "corpse_explosion" {
			filtered = append(filtered, skill)
		}
	}
	value.RequiredSkills = filtered
	broken["necro_bone_spear"] = value
	err = registry.ValidateAgainstProfiles(broken)
	if err == nil || !strings.Contains(err.Error(), "corpse_explosion") {
		t.Fatalf("error = %v", err)
	}
}

func TestCombatStrategyRegistryRejectsNilFactory(t *testing.T) {
	registry := &CombatStrategyRegistry{factories: map[string]map[string]profile.StrategyFactory{}}
	if err := registry.Register(nil); err == nil {
		t.Fatal("nil factory accepted")
	}
	if err := registry.Register(func() profile.RunStrategy { return nil }); err == nil {
		t.Fatal("nil strategy accepted")
	}
}
