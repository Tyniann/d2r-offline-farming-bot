package config

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// ProfilesConfig maps stable profile IDs to character and encounter behavior.
type ProfilesConfig map[string]ProfileConfig

// ProfileConfig defines class-gated hooks, setup metadata and in-run resource policies.
type ProfileConfig struct {
	CharacterClass string                 `yaml:"character_class"`
	DisplayName    string                 `yaml:"display_name,omitempty"`
	Setup          ProfileSetupConfig     `yaml:"setup,omitempty"`
	Hooks          ProfileHooksConfig     `yaml:"hooks"`
	Resources      ProfileResourcesConfig `yaml:"resources"`
}

// ProfileSetupConfig steuert die explizite Freigabe und den Entwickler-Default im Charakter-Setup.
type ProfileSetupConfig struct {
	Enabled bool `yaml:"enabled"`
	Default bool `yaml:"default"`
}

// CharacterSetupConfig enthält die festen Pickit-Defaultketten für neue Charakterzuordnungen.
type CharacterSetupConfig struct {
	PickitDefaults map[string][]string `yaml:"pickit_defaults"`
}

// ProfileHooksConfig groups semantic hook actions by lifecycle event.
type ProfileHooksConfig struct {
	TownReady  []ProfileActionConfig `yaml:"town_ready"`
	BossEngage []ProfileActionConfig `yaml:"boss_engage"`
}

// ProfileActionConfig is one ordered skill action attached to a hook.
type ProfileActionConfig struct {
	Skill            string `yaml:"skill"`
	Target           string `yaml:"target"`
	OncePerGame      bool   `yaml:"once_per_game"`
	OncePerEncounter bool   `yaml:"once_per_encounter"`
	DelayMs          int    `yaml:"delay_ms"`
	SettleMs         int    `yaml:"settle_ms"`
}

// ProfileResourcesConfig configures prioritized potion consumption.
type ProfileResourcesConfig struct {
	Healing      ResourceRuleConfig `yaml:"healing"`
	Mana         ResourceRuleConfig `yaml:"mana"`
	Rejuvenation ResourceRuleConfig `yaml:"rejuvenation"`
	ThrottleMs   int                `yaml:"throttle_ms"`
	VerifyMs     int                `yaml:"verify_timeout_ms"`
}

// ResourceRuleConfig selects a percentage threshold and eligible belt columns.
type ResourceRuleConfig struct {
	UseBelowPercent int   `yaml:"use_below_percent"`
	BeltSlots       []int `yaml:"belt_slots"`
	CooldownMs      int   `yaml:"cooldown_ms"`
}

func (c *ProfilesConfig) applyDefaults() {
	if *c == nil {
		*c = ProfilesConfig{}
	}
	if _, ok := (*c)["necro_bone_spear"]; ok {
		return
	}
	(*c)["necro_bone_spear"] = ProfileConfig{
		CharacterClass: "necromancer",
		DisplayName:    "Knochen-Speer",
		Setup:          ProfileSetupConfig{Enabled: true, Default: true},
		Hooks: ProfileHooksConfig{
			TownReady:  []ProfileActionConfig{{Skill: "bone_armor", Target: "self", OncePerGame: true, DelayMs: 5000, SettleMs: 1500}},
			BossEngage: []ProfileActionConfig{{Skill: "bone_prison", Target: "boss", OncePerEncounter: true, DelayMs: 750, SettleMs: 1500}},
		},
		Resources: ProfileResourcesConfig{
			Healing:      ResourceRuleConfig{UseBelowPercent: 65, BeltSlots: []int{1}, CooldownMs: 4000},
			Mana:         ResourceRuleConfig{UseBelowPercent: 35, BeltSlots: []int{2, 3}, CooldownMs: 4000},
			Rejuvenation: ResourceRuleConfig{UseBelowPercent: 35, BeltSlots: []int{4}, CooldownMs: 1500},
			ThrottleMs:   1500, VerifyMs: 1500,
		},
	}
}

func (c *CharacterSetupConfig) applyDefaults() {
	if c.PickitDefaults != nil {
		return
	}
	c.PickitDefaults = DefaultCharacterSetupPickitChains()
}

// DefaultCharacterSetupPickitChains returns independent copies of the
// developer-owned Pickit defaults for every product run.
func DefaultCharacterSetupPickitChains() map[string][]string {
	return map[string][]string{
		"countess":  {"gems", "keys", "countess-standard"},
		"mephisto":  {"gems", "mephisto-standard"},
		"summoner":  {"gems", "keys"},
		"nihlathak": {"gems", "keys"},
	}
}

func (c CharacterSetupConfig) validate() error {
	if c.PickitDefaults == nil {
		return fmt.Errorf("character_setup.pickit_defaults is required")
	}
	for rawRunID, profiles := range c.PickitDefaults {
		runID := strings.TrimSpace(rawRunID)
		if runID == "" || runID != rawRunID {
			return fmt.Errorf("character_setup.pickit_defaults contains a non-canonical run id")
		}
		if len(profiles) == 0 {
			return fmt.Errorf("character_setup.pickit_defaults.%s must not be empty", runID)
		}
		seen := make(map[string]struct{}, len(profiles))
		for _, rawProfileID := range profiles {
			profileID := strings.TrimSpace(rawProfileID)
			if profileID == "" || profileID != rawProfileID {
				return fmt.Errorf("character_setup.pickit_defaults.%s contains a non-canonical profile id", runID)
			}
			if _, duplicate := seen[profileID]; duplicate {
				return fmt.Errorf("character_setup.pickit_defaults.%s contains duplicate profile %q", runID, profileID)
			}
			seen[profileID] = struct{}{}
		}
	}
	return nil
}

func (c ProfilesConfig) validateSetupMetadata() error {
	enabledByClass := map[string]int{}
	defaultsByClass := map[string]int{}
	for id, profileCfg := range c {
		if strings.TrimSpace(id) == "" || strings.TrimSpace(id) != id {
			return fmt.Errorf("combat_profiles contains a non-canonical profile id")
		}
		if !supportedProfileClass(profileCfg.CharacterClass) {
			return fmt.Errorf("combat_profiles.%s.character_class is unsupported", id)
		}
		if profileCfg.DisplayName != "" {
			if profileCfg.DisplayName != strings.TrimSpace(profileCfg.DisplayName) {
				return fmt.Errorf("combat_profiles.%s.display_name must be trimmed", id)
			}
			if utf8.RuneCountInString(profileCfg.DisplayName) > 64 {
				return fmt.Errorf("combat_profiles.%s.display_name must contain at most 64 characters", id)
			}
			for _, value := range profileCfg.DisplayName {
				if unicode.IsControl(value) {
					return fmt.Errorf("combat_profiles.%s.display_name must not contain control characters", id)
				}
			}
		}
		if profileCfg.Setup.Default && !profileCfg.Setup.Enabled {
			return fmt.Errorf("combat_profiles.%s.setup.default requires setup.enabled", id)
		}
		if !profileCfg.Setup.Enabled {
			continue
		}
		if profileCfg.DisplayName == "" {
			return fmt.Errorf("combat_profiles.%s.display_name is required for setup.enabled", id)
		}
		enabledByClass[profileCfg.CharacterClass]++
		if profileCfg.Setup.Default {
			defaultsByClass[profileCfg.CharacterClass]++
		}
	}
	for class, count := range enabledByClass {
		if count > 0 && defaultsByClass[class] != 1 {
			return fmt.Errorf("combat_profiles class %s must have exactly one enabled setup default", class)
		}
	}
	return nil
}

func (c ProfilesConfig) validate(selected, source string) error {
	profileCfg, ok := c[selected]
	if !ok {
		return fmt.Errorf("combat_profiles.%s is required by %s", selected, source)
	}
	if !supportedProfileClass(profileCfg.CharacterClass) {
		return fmt.Errorf("combat_profiles.%s.character_class is unsupported", selected)
	}
	for hook, actions := range map[string][]ProfileActionConfig{"town_ready": profileCfg.Hooks.TownReady, "boss_engage": profileCfg.Hooks.BossEngage} {
		for i, action := range actions {
			if strings.TrimSpace(action.Skill) == "" {
				return fmt.Errorf("combat_profiles.%s.hooks.%s[%d].skill is required", selected, hook, i)
			}
			if action.Target != "self" && action.Target != "boss" {
				return fmt.Errorf("combat_profiles.%s.hooks.%s[%d].target must be self or boss", selected, hook, i)
			}
			if action.DelayMs < 0 || action.SettleMs < 0 {
				return fmt.Errorf("combat_profiles.%s.hooks.%s[%d] delay and settle must be >= 0", selected, hook, i)
			}
		}
	}
	for name, rule := range map[string]ResourceRuleConfig{"healing": profileCfg.Resources.Healing, "mana": profileCfg.Resources.Mana, "rejuvenation": profileCfg.Resources.Rejuvenation} {
		if rule.UseBelowPercent <= 0 || rule.UseBelowPercent > 100 {
			return fmt.Errorf("combat_profiles.%s.resources.%s.use_below_percent must be within 1..100", selected, name)
		}
		if len(rule.BeltSlots) == 0 {
			return fmt.Errorf("combat_profiles.%s.resources.%s.belt_slots is required", selected, name)
		}
		if rule.CooldownMs <= 0 {
			return fmt.Errorf("combat_profiles.%s.resources.%s.cooldown_ms must be > 0", selected, name)
		}
		seen := map[int]bool{}
		for _, slot := range rule.BeltSlots {
			if slot < 1 || slot > 4 || seen[slot] {
				return fmt.Errorf("combat_profiles.%s.resources.%s.belt_slots must contain unique slots 1..4", selected, name)
			}
			seen[slot] = true
		}
	}
	if profileCfg.Resources.ThrottleMs <= 0 || profileCfg.Resources.VerifyMs <= 0 {
		return fmt.Errorf("combat_profiles.%s.resources throttle and verify timeouts must be > 0", selected)
	}
	return nil
}

func supportedProfileClass(class string) bool {
	switch class {
	case "amazon", "sorceress", "necromancer", "paladin", "barbarian", "druid", "assassin", "warlock":
		return true
	default:
		return false
	}
}
