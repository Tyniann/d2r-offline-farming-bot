package config

import (
	"fmt"
	"strings"
)

// ProfilesConfig maps stable profile IDs to character and encounter behavior.
type ProfilesConfig map[string]ProfileConfig

// ProfileConfig defines class-gated hooks and in-run resource policies.
type ProfileConfig struct {
	CharacterClass string                 `yaml:"character_class"`
	Hooks          ProfileHooksConfig     `yaml:"hooks"`
	Resources      ProfileResourcesConfig `yaml:"resources"`
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

func (c ProfilesConfig) validate(selected, source string) error {
	profileCfg, ok := c[selected]
	if !ok {
		return fmt.Errorf("combat_profiles.%s is required by %s", selected, source)
	}
	supportedClass := map[string]bool{"amazon": true, "sorceress": true, "necromancer": true, "paladin": true, "barbarian": true, "druid": true, "assassin": true}
	if !supportedClass[profileCfg.CharacterClass] {
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
