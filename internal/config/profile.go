package config

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/memory"
)

// ProfilesConfig maps stable profile IDs to character and encounter behavior.
type ProfilesConfig map[string]ProfileConfig

// ProfileConfig defines class-gated hooks, setup metadata and in-run resource policies.
type ProfileConfig struct {
	CharacterClass     string                        `yaml:"character_class"`
	DisplayName        string                        `yaml:"display_name,omitempty"`
	Setup              ProfileSetupConfig            `yaml:"setup,omitempty"`
	Combat             ProfileCombatConfig           `yaml:"combat,omitempty"`
	RequiredSkills     []RequiredSkillConfig         `yaml:"required_skills,omitempty"`
	OptionalSkillPairs []OptionalSkillPairConfig     `yaml:"optional_skill_pairs,omitempty"`
	RequiresMercenary  bool                          `yaml:"requires_mercenary,omitempty"`
	Hooks              ProfileHooksConfig            `yaml:"hooks"`
	Resources          ProfileResourcesConfig        `yaml:"resources"`
	RouteMaintenance   ProfileRouteMaintenanceConfig `yaml:"route_maintenance,omitempty"`
}

// ProfileCombatConfig holds build-owned combat metadata that no longer belongs on a run.
type ProfileCombatConfig struct {
	// StandardAttack is the canonical catalog key for the profile's primary attack skill.
	StandardAttack string `yaml:"standard_attack"`
	// AttackIntervalMs throttles real attack inputs.
	AttackIntervalMs int `yaml:"attack_interval_ms"`
	// EngageDistanceTiles is the desired distance after combat repositioning.
	EngageDistanceTiles float64 `yaml:"engage_distance_tiles"`
	// RepositionDistanceTiles triggers teleport repositioning when exceeded.
	RepositionDistanceTiles float64 `yaml:"reposition_distance_tiles"`
	// KillConfirmTicks confirms death after consecutive valid absence ticks.
	KillConfirmTicks int `yaml:"kill_confirm_ticks"`
}

// RequiredSkillConfig declares one ordered duty skill for bindings and live skill checks.
type RequiredSkillConfig struct {
	Skill       string `yaml:"skill"`
	DisplayName string `yaml:"display_name"`
	Slot        string `yaml:"slot,omitempty"`
}

// OptionalSkillPairConfig declares exactly two jointly optional profile skills.
// Operator bindings may omit both entries, but never configure only one.
type OptionalSkillPairConfig struct {
	Skills []ProfileSkillSlotConfig `yaml:"skills"`
}

// ProfileSkillSlotConfig binds one canonical CASC skill key to its forced mouse slot.
type ProfileSkillSlotConfig struct {
	Skill string `yaml:"skill"`
	Slot  string `yaml:"slot"`
}

// ProfileRouteMaintenanceConfig contains the deliberately narrow maintenance
// policy evaluated only while a combat route owns the current tick.
type ProfileRouteMaintenanceConfig struct {
	BoneArmor BoneArmorMaintenanceConfig `yaml:"bone_armor,omitempty"`
}

// BoneArmorMaintenanceConfig refreshes Bone Armor on a finite timer or after
// newly observed player damage below the configured HP threshold.
type BoneArmorMaintenanceConfig struct {
	Enabled                    *bool  `yaml:"enabled,omitempty"`
	Skill                      string `yaml:"skill"`
	RefreshIntervalMs          int    `yaml:"refresh_interval_ms"`
	RefreshAfterDamageBelowPct int    `yaml:"refresh_after_damage_below_percent"`
	MinimumRecastIntervalMs    int    `yaml:"minimum_recast_interval_ms"`
	SettleMs                   int    `yaml:"settle_ms"`
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
	Healing      ResourceRuleConfig      `yaml:"healing"`
	Mana         ResourceRuleConfig      `yaml:"mana"`
	Rejuvenation ResourceRuleConfig      `yaml:"rejuvenation"`
	Mercenary    MercenaryResourceConfig `yaml:"mercenary"`
	ThrottleMs   int                     `yaml:"throttle_ms"`
	VerifyMs     int                     `yaml:"verify_timeout_ms"`
}

// ResourceRuleConfig selects a percentage threshold and eligible belt columns.
type ResourceRuleConfig struct {
	UseBelowPercent int   `yaml:"use_below_percent"`
	BeltSlots       []int `yaml:"belt_slots"`
	CooldownMs      int   `yaml:"cooldown_ms"`
}

// MercenaryResourceConfig is the presence-sensitive combat-profile Merc potion
// policy. A missing block resolves to enabled=true with defaults; only an
// explicit `enabled: false` disables Preflight, Combat and Town Merc actions.
type MercenaryResourceConfig struct {
	Enabled         *bool `yaml:"enabled,omitempty"`
	UseBelowPercent int   `yaml:"use_below_percent"`
	BeltSlots       []int `yaml:"belt_slots"`
	CooldownMs      int   `yaml:"cooldown_ms"`
}

// Resolve returns the effective Merc resource switch and rule after defaults.
// Missing Enabled resolves to true. Zero thresholds, slots and cooldown fill
// with 50, `[1]` and `4000` respectively.
func (m MercenaryResourceConfig) Resolve() (enabled bool, rule ResourceRuleConfig) {
	enabled = true
	if m.Enabled != nil {
		enabled = *m.Enabled
	}
	rule = ResourceRuleConfig{
		UseBelowPercent: m.UseBelowPercent,
		BeltSlots:       append([]int(nil), m.BeltSlots...),
		CooldownMs:      m.CooldownMs,
	}
	if rule.UseBelowPercent == 0 {
		rule.UseBelowPercent = 50
	}
	if len(rule.BeltSlots) == 0 {
		rule.BeltSlots = []int{1}
	}
	if rule.CooldownMs == 0 {
		rule.CooldownMs = 4000
	}
	return enabled, rule
}

// ApplyDefaults materialisiert produktive Necro-Defaults für Tests und Loader.
func (c *ProfilesConfig) ApplyDefaults() {
	c.applyDefaults()
}

func (c *ProfilesConfig) applyDefaults() {
	if *c == nil {
		*c = ProfilesConfig{}
	}
	c.applyNecroBoneSpearDefaults()
	c.applyPaladinHammerdinDefaults()
}

func (c *ProfilesConfig) applyNecroBoneSpearDefaults() {
	if existing, ok := (*c)["necro_bone_spear"]; ok {
		if existing.DisplayName == "" {
			existing.DisplayName = "Knochen-Speer"
		}
		if !existing.Setup.Enabled && !existing.Setup.Default {
			existing.Setup = ProfileSetupConfig{Enabled: true, Default: true}
		}
		existing.Combat.applyDefaults()
		if len(existing.RequiredSkills) == 0 {
			existing.RequiredSkills = defaultNecroBoneSpearRequiredSkills()
		}
		existing.RouteMaintenance.BoneArmor.applyDefaults()
		(*c)["necro_bone_spear"] = existing
		return
	}
	enabled := true
	(*c)["necro_bone_spear"] = ProfileConfig{
		CharacterClass: "necromancer",
		DisplayName:    "Knochen-Speer",
		Setup:          ProfileSetupConfig{Enabled: true, Default: true},
		Combat: ProfileCombatConfig{
			StandardAttack:          "bone_spear",
			AttackIntervalMs:        350,
			EngageDistanceTiles:     22,
			RepositionDistanceTiles: 32,
			KillConfirmTicks:        3,
		},
		RequiredSkills: defaultNecroBoneSpearRequiredSkills(),
		Hooks: ProfileHooksConfig{
			TownReady:  []ProfileActionConfig{{Skill: "bone_armor", Target: "self", OncePerGame: true, DelayMs: 5000, SettleMs: 1500}},
			BossEngage: []ProfileActionConfig{{Skill: "bone_prison", Target: "boss", OncePerEncounter: true, DelayMs: 750, SettleMs: 1500}},
		},
		Resources: ProfileResourcesConfig{
			Healing:      ResourceRuleConfig{UseBelowPercent: 65, BeltSlots: []int{1}, CooldownMs: 4000},
			Mana:         ResourceRuleConfig{UseBelowPercent: 35, BeltSlots: []int{2, 3}, CooldownMs: 4000},
			Rejuvenation: ResourceRuleConfig{UseBelowPercent: 35, BeltSlots: []int{4}, CooldownMs: 1500},
			Mercenary:    MercenaryResourceConfig{UseBelowPercent: 50, BeltSlots: []int{1}, CooldownMs: 4000},
			ThrottleMs:   1500, VerifyMs: 1500,
		},
		RouteMaintenance: ProfileRouteMaintenanceConfig{BoneArmor: BoneArmorMaintenanceConfig{
			Enabled: &enabled, Skill: "bone_armor", RefreshIntervalMs: 60000,
			RefreshAfterDamageBelowPct: 65, MinimumRecastIntervalMs: 10000, SettleMs: 750,
		}},
	}
}

func (c *ProfilesConfig) applyPaladinHammerdinDefaults() {
	if _, ok := (*c)["paladin_hammerdin"]; ok {
		return
	}
	(*c)["paladin_hammerdin"] = ProfileConfig{
		CharacterClass:    "paladin",
		DisplayName:       "Hammerdin",
		Setup:             ProfileSetupConfig{Enabled: true, Default: true},
		RequiresMercenary: true,
		Combat: ProfileCombatConfig{
			StandardAttack: "blessed_hammer",
			// Blessed Hammer needs a finished cast frame. 100 ms interrupts
			// the windup so the game shows no hammer and the Paladin walks.
			AttackIntervalMs:        300,
			EngageDistanceTiles:     1,
			RepositionDistanceTiles: 3,
			KillConfirmTicks:        3,
		},
		RequiredSkills: []RequiredSkillConfig{
			{Skill: "teleport", DisplayName: "Teleport", Slot: "right"},
			{Skill: "town_portal", DisplayName: "Stadtportal", Slot: "right"},
			{Skill: "blessed_hammer", DisplayName: "Gesegneter Hammer", Slot: "left"},
			{Skill: "concentration", DisplayName: "Konzentration", Slot: "right"},
			{Skill: "holy_shield", DisplayName: "Heiliger Schild", Slot: "right"},
		},
		OptionalSkillPairs: []OptionalSkillPairConfig{{Skills: []ProfileSkillSlotConfig{
			{Skill: "battle_command", Slot: "right"},
			{Skill: "battle_orders", Slot: "right"},
		}}},
		Resources: ProfileResourcesConfig{
			Healing:      ResourceRuleConfig{UseBelowPercent: 65, BeltSlots: []int{1}, CooldownMs: 4000},
			Mana:         ResourceRuleConfig{UseBelowPercent: 35, BeltSlots: []int{2, 3}, CooldownMs: 4000},
			Rejuvenation: ResourceRuleConfig{UseBelowPercent: 35, BeltSlots: []int{4}, CooldownMs: 1500},
			Mercenary:    MercenaryResourceConfig{UseBelowPercent: 50, BeltSlots: []int{1}, CooldownMs: 4000},
			ThrottleMs:   1500,
			VerifyMs:     1500,
		},
	}
}

func defaultNecroBoneSpearRequiredSkills() []RequiredSkillConfig {
	return []RequiredSkillConfig{
		{Skill: "teleport", DisplayName: "Teleport"},
		{Skill: "town_portal", DisplayName: "Stadtportal"},
		{Skill: "bone_spear", DisplayName: "Knochen-Speer"},
		{Skill: "amplify_damage", DisplayName: "Verstärkter Schaden"},
		{Skill: "corpse_explosion", DisplayName: "Kadaverexplosion"},
		{Skill: "bone_armor", DisplayName: "Knochenrüstung"},
		{Skill: "bone_prison", DisplayName: "Knochengefängnis"},
	}
}

func (c *ProfileCombatConfig) applyDefaults() {
	if c.StandardAttack == "" {
		c.StandardAttack = "bone_spear"
	}
	if c.AttackIntervalMs == 0 {
		c.AttackIntervalMs = 350
	}
	if c.EngageDistanceTiles == 0 {
		c.EngageDistanceTiles = 22
	}
	if c.RepositionDistanceTiles == 0 {
		c.RepositionDistanceTiles = 32
	}
	if c.KillConfirmTicks == 0 {
		c.KillConfirmTicks = 3
	}
}

func (c *BoneArmorMaintenanceConfig) applyDefaults() {
	if c.Enabled == nil {
		enabled := true
		c.Enabled = &enabled
	}
	if !*c.Enabled {
		return
	}
	if c.Skill == "" {
		c.Skill = "bone_armor"
	}
	if c.RefreshIntervalMs == 0 {
		c.RefreshIntervalMs = 60000
	}
	if c.RefreshAfterDamageBelowPct == 0 {
		c.RefreshAfterDamageBelowPct = 65
	}
	if c.MinimumRecastIntervalMs == 0 {
		c.MinimumRecastIntervalMs = 10000
	}
	if c.SettleMs == 0 {
		c.SettleMs = 750
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
		"countess":     {"gems", "keys", "countess-standard"},
		"mephisto":     {"gems", "mephisto-standard"},
		"lower-kurast": {"gems", "lk-superchests"},
		"summoner":     {"gems", "keys"},
		"nihlathak":    {"gems", "keys"},
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
		if err := validateProfileCombatAndRequiredSkills(id, profileCfg); err != nil {
			return err
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
	if err := validateProfileCombatAndRequiredSkills(selected, profileCfg); err != nil {
		return err
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
		if err := validateResourceRule(selected, name, rule); err != nil {
			return err
		}
	}
	if err := validateMercenaryResource(selected, profileCfg.Resources); err != nil {
		return err
	}
	if profileCfg.Resources.ThrottleMs <= 0 || profileCfg.Resources.VerifyMs <= 0 {
		return fmt.Errorf("combat_profiles.%s.resources throttle and verify timeouts must be > 0", selected)
	}
	maintenance := profileCfg.RouteMaintenance.BoneArmor
	if maintenance.Enabled != nil && *maintenance.Enabled {
		if strings.TrimSpace(maintenance.Skill) == "" {
			return fmt.Errorf("combat_profiles.%s.route_maintenance.bone_armor.skill is required", selected)
		}
		if maintenance.RefreshIntervalMs <= 0 || maintenance.MinimumRecastIntervalMs <= 0 || maintenance.SettleMs < 0 {
			return fmt.Errorf("combat_profiles.%s.route_maintenance.bone_armor intervals must be > 0 and settle_ms >= 0", selected)
		}
		if maintenance.RefreshAfterDamageBelowPct <= 0 || maintenance.RefreshAfterDamageBelowPct > 100 {
			return fmt.Errorf("combat_profiles.%s.route_maintenance.bone_armor.refresh_after_damage_below_percent must be within 1..100", selected)
		}
	}
	return nil
}

func validateProfileCombatAndRequiredSkills(profileID string, profileCfg ProfileConfig) error {
	if strings.TrimSpace(profileCfg.Combat.StandardAttack) == "" {
		return fmt.Errorf("combat_profiles.%s.combat.standard_attack is required", profileID)
	}
	if profileCfg.Combat.StandardAttack != strings.TrimSpace(profileCfg.Combat.StandardAttack) {
		return fmt.Errorf("combat_profiles.%s.combat.standard_attack must be trimmed", profileID)
	}
	if _, ok := memory.LookupSkillByKey(profileCfg.Combat.StandardAttack); !ok {
		return fmt.Errorf("combat_profiles.%s.combat.standard_attack %q is not in the skill catalog", profileID, profileCfg.Combat.StandardAttack)
	}
	if profileCfg.Combat.AttackIntervalMs <= 0 {
		return fmt.Errorf("combat_profiles.%s.combat.attack_interval_ms must be > 0", profileID)
	}
	if profileCfg.Combat.EngageDistanceTiles <= 0 || profileCfg.Combat.RepositionDistanceTiles <= 0 {
		return fmt.Errorf("combat_profiles.%s.combat engage/reposition distances must be > 0", profileID)
	}
	if profileCfg.Combat.EngageDistanceTiles >= profileCfg.Combat.RepositionDistanceTiles {
		return fmt.Errorf("combat_profiles.%s.combat.engage_distance_tiles must be < reposition_distance_tiles", profileID)
	}
	if profileCfg.Combat.KillConfirmTicks <= 0 {
		return fmt.Errorf("combat_profiles.%s.combat.kill_confirm_ticks must be > 0", profileID)
	}
	if len(profileCfg.RequiredSkills) == 0 {
		return fmt.Errorf("combat_profiles.%s.required_skills is required", profileID)
	}
	if len(profileCfg.RequiredSkills) > 8 {
		return fmt.Errorf("combat_profiles.%s.required_skills must contain at most 8 entries", profileID)
	}
	required := make(map[string]struct{}, len(profileCfg.RequiredSkills))
	requiredSlots := make(map[string]string, len(profileCfg.RequiredSkills))
	for i, entry := range profileCfg.RequiredSkills {
		skill := strings.TrimSpace(entry.Skill)
		if skill == "" || skill != entry.Skill {
			return fmt.Errorf("combat_profiles.%s.required_skills[%d].skill must be a canonical catalog key", profileID, i)
		}
		if _, duplicate := required[skill]; duplicate {
			return fmt.Errorf("combat_profiles.%s.required_skills contains duplicate skill %q", profileID, skill)
		}
		if _, ok := memory.LookupSkillByKey(skill); !ok {
			return fmt.Errorf("combat_profiles.%s.required_skills[%d].skill %q is not in the skill catalog", profileID, i, skill)
		}
		if err := validateRequiredSkillDisplayName(profileID, i, entry.DisplayName); err != nil {
			return err
		}
		if entry.Slot != "" {
			if err := validateProfileSkillSlot(profileID, fmt.Sprintf("required_skills[%d]", i), skill, entry.Slot); err != nil {
				return err
			}
		}
		required[skill] = struct{}{}
		requiredSlots[skill] = entry.Slot
	}
	optionalSkills := make(map[string]struct{})
	for pairIndex, pair := range profileCfg.OptionalSkillPairs {
		if len(pair.Skills) != 2 {
			return fmt.Errorf("combat_profiles.%s.optional_skill_pairs[%d].skills must contain exactly two entries", profileID, pairIndex)
		}
		for skillIndex, entry := range pair.Skills {
			skill := strings.TrimSpace(entry.Skill)
			if skill == "" || skill != entry.Skill {
				return fmt.Errorf("combat_profiles.%s.optional_skill_pairs[%d].skills[%d].skill must be a canonical catalog key", profileID, pairIndex, skillIndex)
			}
			if _, exists := required[skill]; exists {
				return fmt.Errorf("combat_profiles.%s optional skill %q is already required", profileID, skill)
			}
			if _, duplicate := optionalSkills[skill]; duplicate {
				return fmt.Errorf("combat_profiles.%s optional skill %q is duplicated", profileID, skill)
			}
			if err := validateProfileSkillSlot(profileID, fmt.Sprintf("optional_skill_pairs[%d].skills[%d]", pairIndex, skillIndex), skill, entry.Slot); err != nil {
				return err
			}
			optionalSkills[skill] = struct{}{}
		}
	}
	for _, skill := range []string{"teleport", "town_portal"} {
		if _, ok := required[skill]; !ok {
			return fmt.Errorf("combat_profiles.%s.required_skills must include %s", profileID, skill)
		}
	}
	if _, ok := required[profileCfg.Combat.StandardAttack]; !ok {
		return fmt.Errorf("combat_profiles.%s.combat.standard_attack %q must be listed in required_skills", profileID, profileCfg.Combat.StandardAttack)
	}
	for hook, actions := range map[string][]ProfileActionConfig{"town_ready": profileCfg.Hooks.TownReady, "boss_engage": profileCfg.Hooks.BossEngage} {
		for i, action := range actions {
			skill := strings.TrimSpace(action.Skill)
			if skill == "" {
				continue
			}
			if _, ok := required[skill]; !ok {
				return fmt.Errorf("combat_profiles.%s.hooks.%s[%d].skill %q must be listed in required_skills", profileID, hook, i, skill)
			}
		}
	}
	maintenance := profileCfg.RouteMaintenance.BoneArmor
	if maintenance.Enabled != nil && *maintenance.Enabled {
		skill := strings.TrimSpace(maintenance.Skill)
		if skill != "" {
			if _, ok := required[skill]; !ok {
				return fmt.Errorf("combat_profiles.%s.route_maintenance.bone_armor.skill %q must be listed in required_skills", profileID, skill)
			}
		}
	}
	if profileID == "paladin_hammerdin" {
		return validatePaladinHammerdinContract(profileCfg, requiredSlots)
	}
	return nil
}

func validateProfileSkillSlot(profileID, field, skill, slot string) error {
	entry, ok := memory.LookupSkillByKey(skill)
	if !ok {
		return fmt.Errorf("combat_profiles.%s.%s.skill %q is not in the skill catalog", profileID, field, skill)
	}
	switch slot {
	case "left":
		if !entry.LeftSkill {
			return fmt.Errorf("combat_profiles.%s.%s slot left is not supported by CASC skill %q", profileID, field, skill)
		}
	case "right":
		// TownPortal is the existing RMB portal action. Its CASC row deliberately
		// has no ordinary skill-menu slot flags, so only catalog identity applies.
		if !entry.RightSkill && skill != "town_portal" {
			return fmt.Errorf("combat_profiles.%s.%s slot right is not supported by CASC skill %q", profileID, field, skill)
		}
	default:
		return fmt.Errorf("combat_profiles.%s.%s.slot must be left or right", profileID, field)
	}
	return nil
}

func validatePaladinHammerdinContract(profileCfg ProfileConfig, slots map[string]string) error {
	if profileCfg.CharacterClass != "paladin" {
		return fmt.Errorf("combat_profiles.paladin_hammerdin.character_class must be paladin")
	}
	wantSlots := map[string]string{
		"teleport":       "right",
		"town_portal":    "right",
		"blessed_hammer": "left",
		"concentration":  "right",
		"holy_shield":    "right",
	}
	if len(slots) != len(wantSlots) {
		return fmt.Errorf("combat_profiles.paladin_hammerdin.required_skills must contain exactly the Hammerdin duty skills")
	}
	for skill, wantSlot := range wantSlots {
		if slots[skill] != wantSlot {
			return fmt.Errorf("combat_profiles.paladin_hammerdin required skill %q must use slot %s", skill, wantSlot)
		}
	}
	if !profileCfg.RequiresMercenary {
		return fmt.Errorf("combat_profiles.paladin_hammerdin.requires_mercenary must be true")
	}
	if len(profileCfg.OptionalSkillPairs) != 1 {
		return fmt.Errorf("combat_profiles.paladin_hammerdin must declare one optional Battle Command/Battle Orders pair")
	}
	pair := profileCfg.OptionalSkillPairs[0].Skills
	if len(pair) != 2 || pair[0].Skill != "battle_command" || pair[0].Slot != "right" || pair[1].Skill != "battle_orders" || pair[1].Slot != "right" {
		return fmt.Errorf("combat_profiles.paladin_hammerdin optional pair must be battle_command and battle_orders on right")
	}
	return nil
}

func validateRequiredSkillDisplayName(profileID string, index int, displayName string) error {
	if displayName == "" {
		return fmt.Errorf("combat_profiles.%s.required_skills[%d].display_name is required", profileID, index)
	}
	if displayName != strings.TrimSpace(displayName) {
		return fmt.Errorf("combat_profiles.%s.required_skills[%d].display_name must be trimmed", profileID, index)
	}
	if utf8.RuneCountInString(displayName) > 64 {
		return fmt.Errorf("combat_profiles.%s.required_skills[%d].display_name must contain at most 64 characters", profileID, index)
	}
	for _, value := range displayName {
		if unicode.IsControl(value) {
			return fmt.Errorf("combat_profiles.%s.required_skills[%d].display_name must not contain control characters", profileID, index)
		}
	}
	return nil
}

func validateResourceRule(selected, name string, rule ResourceRuleConfig) error {
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
	return nil
}

func validateMercenaryResource(selected string, resources ProfileResourcesConfig) error {
	raw := resources.Mercenary
	enabled, rule := raw.Resolve()
	if raw.Enabled != nil && !*raw.Enabled {
		if raw.UseBelowPercent != 0 && (raw.UseBelowPercent <= 0 || raw.UseBelowPercent > 100) {
			return fmt.Errorf("combat_profiles.%s.resources.mercenary.use_below_percent must be within 1..100", selected)
		}
		if raw.CooldownMs < 0 {
			return fmt.Errorf("combat_profiles.%s.resources.mercenary.cooldown_ms must be > 0", selected)
		}
		for _, slot := range raw.BeltSlots {
			if slot < 1 || slot > 4 {
				return fmt.Errorf("combat_profiles.%s.resources.mercenary.belt_slots must contain unique slots 1..4", selected)
			}
		}
		return nil
	}
	if !enabled {
		return nil
	}
	if err := validateResourceRule(selected, "mercenary", rule); err != nil {
		return err
	}
	healingSlots := map[int]bool{}
	for _, slot := range resources.Healing.BeltSlots {
		healingSlots[slot] = true
	}
	for _, slot := range rule.BeltSlots {
		if !healingSlots[slot] {
			return fmt.Errorf("combat_profiles.%s.resources.mercenary.belt_slots must be a subset of healing.belt_slots", selected)
		}
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
