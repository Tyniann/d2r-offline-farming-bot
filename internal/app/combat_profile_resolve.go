package app

import (
	"fmt"
	"strings"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/memory"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/tasks"
)

// resolveActiveCombatProfileID returns the character-owned combat profile.
// OperatorSettings is the productive authority when a character is set up.
// Pure config/CLI fixtures without setup fall back to the single enabled class default.
func resolveActiveCombatProfileID(cfg *config.Config, settings *OperatorSettings, characterName string) (string, error) {
	if cfg == nil {
		return "", fmt.Errorf("config is required to resolve combat profile")
	}
	character := strings.ToLower(strings.TrimSpace(characterName))
	if character == "" {
		character = strings.ToLower(strings.TrimSpace(cfg.Session.Character))
	}
	if settings != nil && character != "" {
		if value, ok := settings.Characters[character]; ok {
			profileID := strings.TrimSpace(value.CombatProfile)
			if profileID != "" {
				if _, exists := cfg.Profiles[profileID]; !exists {
					return "", fmt.Errorf("character %q combat profile %q is unknown", characterName, profileID)
				}
				return profileID, nil
			}
		}
	}
	return defaultEnabledCombatProfileID(cfg.Profiles)
}

// resolveRuntimeCombatProfileID binds productive runtime construction to the
// frozen loadout profile. Read-only and legacy test fixtures without a loadout
// retain the config default resolution.
func resolveRuntimeCombatProfileID(cfg *config.Config, loadout *CharacterLoadoutSnapshot) (string, error) {
	if cfg == nil {
		return "", fmt.Errorf("config is required to resolve runtime combat profile")
	}
	if loadout == nil {
		return resolveActiveCombatProfileID(cfg, nil, cfg.Session.Character)
	}
	profileID := strings.TrimSpace(loadout.ProfileID)
	if profileID == "" {
		return "", fmt.Errorf("frozen character loadout has no combat profile")
	}
	if _, ok := cfg.Profiles[profileID]; !ok {
		return "", fmt.Errorf("frozen character loadout combat profile %q is unknown", profileID)
	}
	if character := strings.TrimSpace(loadout.Character); character != "" &&
		strings.TrimSpace(cfg.Session.Character) != "" &&
		!strings.EqualFold(character, cfg.Session.Character) {
		return "", fmt.Errorf("frozen character loadout belongs to %q, runtime character is %q", character, cfg.Session.Character)
	}
	return profileID, nil
}

func (rt *Runtime) resolvedCombatProfileID() (string, error) {
	if rt == nil || rt.Config == nil {
		return "", fmt.Errorf("runtime config is required to resolve combat profile")
	}
	if profileID := strings.TrimSpace(rt.combatProfileID); profileID != "" {
		if _, ok := rt.Config.Profiles[profileID]; !ok {
			return "", fmt.Errorf("runtime combat profile %q is unknown", profileID)
		}
		return profileID, nil
	}
	// Hand-built read-only/unit-test Runtime fixtures predate the frozen field.
	return resolveActiveCombatProfileID(rt.Config, nil, rt.Config.Session.Character)
}

func defaultEnabledCombatProfileID(profiles config.ProfilesConfig) (string, error) {
	var defaults []string
	for id, profileCfg := range profiles {
		if profileCfg.Setup.Enabled && profileCfg.Setup.Default {
			defaults = append(defaults, id)
		}
	}
	if len(defaults) == 1 {
		return defaults[0], nil
	}
	if len(defaults) == 0 {
		if _, ok := profiles["necro_bone_spear"]; ok {
			return "necro_bone_spear", nil
		}
		return "", fmt.Errorf("no enabled default combat profile is configured")
	}
	// Classless read-only and legacy CLI fixtures have no character setup from
	// which to select a class default. Preserve their established Necro carrier;
	// productive runtimes use the frozen character-owned profile above.
	if profileCfg, ok := profiles["necro_bone_spear"]; ok && profileCfg.Setup.Enabled && profileCfg.Setup.Default {
		return "necro_bone_spear", nil
	}
	return "", fmt.Errorf("multiple enabled default combat profiles are configured without a class context")
}

func mapCombatConfigFromProfile(profileID string, profileCfg config.ProfileConfig) (tasks.CombatConfig, error) {
	attackSkillID, err := memory.ParseSkillTestName(profileCfg.Combat.StandardAttack)
	if err != nil {
		return tasks.CombatConfig{}, fmt.Errorf("combat_profiles.%s.combat.standard_attack: %w", profileID, err)
	}
	return tasks.CombatConfig{
		Profile:                 profileID,
		AttackSkillID:           attackSkillID,
		AttackInterval:          time.Duration(profileCfg.Combat.AttackIntervalMs) * time.Millisecond,
		EngageDistanceTiles:     profileCfg.Combat.EngageDistanceTiles,
		RepositionDistanceTiles: profileCfg.Combat.RepositionDistanceTiles,
		KillConfirmTicks:        profileCfg.Combat.KillConfirmTicks,
	}, nil
}
