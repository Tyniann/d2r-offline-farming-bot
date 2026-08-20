package config

import (
	"strings"
	"testing"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/memory"
)

func TestDefaultNecroProfileContract(t *testing.T) {
	var profiles ProfilesConfig
	profiles.applyDefaults()
	got := profiles["necro_bone_spear"]
	if got.CharacterClass != "necromancer" || got.DisplayName != "Knochen-Speer" || !got.Setup.Enabled || !got.Setup.Default ||
		len(got.Hooks.TownReady) != 1 || got.Hooks.TownReady[0].Skill != "bone_armor" {
		t.Fatalf("default profile = %+v", got)
	}
	if got.Combat.StandardAttack != "bone_spear" || got.Combat.AttackIntervalMs != 350 ||
		got.Combat.EngageDistanceTiles != 22 || got.Combat.RepositionDistanceTiles != 32 || got.Combat.KillConfirmTicks != 3 {
		t.Fatalf("default combat = %+v", got.Combat)
	}
	if len(got.RequiredSkills) != 7 || got.RequiredSkills[0].Skill != "teleport" || got.RequiredSkills[2].Skill != "bone_spear" {
		t.Fatalf("default required skills = %+v", got.RequiredSkills)
	}
	if got.Resources.Mana.UseBelowPercent != 35 || len(got.Resources.Mana.BeltSlots) != 2 || got.Resources.Mana.CooldownMs != 4000 {
		t.Fatalf("default mana policy = %+v", got.Resources.Mana)
	}
	enabled, merc := got.Resources.Mercenary.Resolve()
	if !enabled || merc.UseBelowPercent != 50 || len(merc.BeltSlots) != 1 || merc.BeltSlots[0] != 1 || merc.CooldownMs != 4000 {
		t.Fatalf("default mercenary policy = enabled=%v rule=%+v", enabled, merc)
	}
	if err := profiles.validate("necro_bone_spear", "test"); err != nil {
		t.Fatal(err)
	}
	if err := profiles.validateSetupMetadata(); err != nil {
		t.Fatal(err)
	}
}

func TestDefaultPaladinHammerdinProfileContract(t *testing.T) {
	var profiles ProfilesConfig
	profiles.applyDefaults()
	got := profiles["paladin_hammerdin"]
	if got.CharacterClass != "paladin" || got.DisplayName != "Hammerdin" || !got.Setup.Enabled || !got.Setup.Default || !got.RequiresMercenary {
		t.Fatalf("default Hammerdin profile = %+v", got)
	}
	wantSkills := []struct {
		key  string
		id   uint16
		slot string
	}{
		{key: "teleport", id: 54, slot: "right"},
		{key: "town_portal", id: 359, slot: "right"},
		{key: "blessed_hammer", id: 112, slot: "left"},
		{key: "concentration", id: 113, slot: "right"},
		{key: "holy_shield", id: 117, slot: "right"},
	}
	if len(got.RequiredSkills) != len(wantSkills) {
		t.Fatalf("required skills = %+v", got.RequiredSkills)
	}
	for index, want := range wantSkills {
		entry := got.RequiredSkills[index]
		catalog, ok := memory.LookupSkillByKey(entry.Skill)
		if !ok || entry.Skill != want.key || catalog.ID != want.id || entry.Slot != want.slot {
			t.Fatalf("required skill[%d] = %+v catalog=%+v", index, entry, catalog)
		}
	}
	if len(got.OptionalSkillPairs) != 1 || len(got.OptionalSkillPairs[0].Skills) != 2 {
		t.Fatalf("optional skill pairs = %+v", got.OptionalSkillPairs)
	}
	pair := got.OptionalSkillPairs[0].Skills
	for index, want := range []struct {
		key string
		id  uint16
	}{{"battle_command", 155}, {"battle_orders", 149}} {
		catalog, ok := memory.LookupSkillByKey(pair[index].Skill)
		if !ok || pair[index].Skill != want.key || catalog.ID != want.id || pair[index].Slot != "right" {
			t.Fatalf("optional skill[%d] = %+v catalog=%+v", index, pair[index], catalog)
		}
	}
	if err := profiles.validate("paladin_hammerdin", "test"); err != nil {
		t.Fatal(err)
	}
	if got.Combat.StandardAttack != "blessed_hammer" || got.Combat.AttackIntervalMs != 300 ||
		got.Combat.EngageDistanceTiles != 1 || got.Combat.RepositionDistanceTiles != 3 || got.Combat.KillConfirmTicks != 3 {
		t.Fatalf("default Hammerdin combat = %+v", got.Combat)
	}
}

func TestPaladinHammerdinProfileRejectsContractDrift(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(ProfileConfig) ProfileConfig
	}{
		{name: "Blessed Hammer on right", mutate: func(value ProfileConfig) ProfileConfig {
			value.RequiredSkills[2].Slot = "right"
			return value
		}},
		{name: "Mercenary optional", mutate: func(value ProfileConfig) ProfileConfig {
			value.RequiresMercenary = false
			return value
		}},
		{name: "partial CTA description", mutate: func(value ProfileConfig) ProfileConfig {
			value.OptionalSkillPairs[0].Skills = value.OptionalSkillPairs[0].Skills[:1]
			return value
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var profiles ProfilesConfig
			profiles.applyDefaults()
			value := test.mutate(profiles["paladin_hammerdin"])
			copyProfiles := ProfilesConfig{"paladin_hammerdin": value}
			if err := copyProfiles.validate("paladin_hammerdin", "test"); err == nil {
				t.Fatal("invalid Hammerdin contract accepted")
			}
		})
	}
}

func TestMercenaryResourceConfigPresenceDefaultsAndValidation(t *testing.T) {
	base := func() ProfilesConfig {
		var profiles ProfilesConfig
		profiles.applyDefaults()
		return profiles
	}
	t.Run("missing block resolves enabled", func(t *testing.T) {
		profiles := base()
		value := profiles["necro_bone_spear"]
		value.Resources.Mercenary = MercenaryResourceConfig{}
		profiles["necro_bone_spear"] = value
		if err := profiles.validate("necro_bone_spear", "run.combat.profile"); err != nil {
			t.Fatal(err)
		}
		enabled, rule := value.Resources.Mercenary.Resolve()
		if !enabled || rule.UseBelowPercent != 50 || len(rule.BeltSlots) != 1 || rule.BeltSlots[0] != 1 {
			t.Fatalf("resolve = %v %+v", enabled, rule)
		}
	})
	t.Run("explicit false skips runtime requirements", func(t *testing.T) {
		profiles := base()
		value := profiles["necro_bone_spear"]
		disabled := false
		value.Resources.Mercenary = MercenaryResourceConfig{Enabled: &disabled}
		profiles["necro_bone_spear"] = value
		if err := profiles.validate("necro_bone_spear", "run.combat.profile"); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("slot outside healing rejected", func(t *testing.T) {
		profiles := base()
		value := profiles["necro_bone_spear"]
		value.Resources.Mercenary.BeltSlots = []int{2}
		profiles["necro_bone_spear"] = value
		if err := profiles.validate("necro_bone_spear", "run.combat.profile"); err == nil {
			t.Fatal("expected subset validation error")
		}
	})
}

func TestProfileSetupMetadataMatrix(t *testing.T) {
	base := func() ProfilesConfig {
		var profiles ProfilesConfig
		profiles.applyDefaults()
		return profiles
	}
	t.Run("no enabled profile", func(t *testing.T) {
		profiles := base()
		value := profiles["necro_bone_spear"]
		value.Setup = ProfileSetupConfig{}
		profiles["necro_bone_spear"] = value
		if err := profiles.validateSetupMetadata(); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("multiple enabled with one default", func(t *testing.T) {
		profiles := base()
		second := profiles["necro_bone_spear"]
		second.DisplayName = "Knochen-Geist"
		second.Setup.Default = false
		profiles["necro_bone_spirit"] = second
		if err := profiles.validateSetupMetadata(); err != nil {
			t.Fatal(err)
		}
	})
	tests := []struct {
		name   string
		mutate func(ProfilesConfig)
	}{
		{name: "enabled without default", mutate: func(profiles ProfilesConfig) {
			value := profiles["necro_bone_spear"]
			value.Setup.Default = false
			profiles["necro_bone_spear"] = value
		}},
		{name: "two defaults", mutate: func(profiles ProfilesConfig) {
			profiles["necro_bone_spirit"] = profiles["necro_bone_spear"]
		}},
		{name: "default without enabled", mutate: func(profiles ProfilesConfig) {
			value := profiles["necro_bone_spear"]
			value.Setup.Enabled = false
			profiles["necro_bone_spear"] = value
		}},
		{name: "missing display name", mutate: func(profiles ProfilesConfig) {
			value := profiles["necro_bone_spear"]
			value.DisplayName = ""
			profiles["necro_bone_spear"] = value
		}},
		{name: "long display name", mutate: func(profiles ProfilesConfig) {
			value := profiles["necro_bone_spear"]
			value.DisplayName = strings.Repeat("ä", 65)
			profiles["necro_bone_spear"] = value
		}},
		{name: "control character", mutate: func(profiles ProfilesConfig) {
			value := profiles["necro_bone_spear"]
			value.DisplayName = "Knochen\nSpeer"
			profiles["necro_bone_spear"] = value
		}},
		{name: "unknown class", mutate: func(profiles ProfilesConfig) {
			value := profiles["necro_bone_spear"]
			value.CharacterClass = "monk"
			profiles["necro_bone_spear"] = value
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			profiles := base()
			test.mutate(profiles)
			if err := profiles.validateSetupMetadata(); err == nil {
				t.Fatal("invalid setup metadata was accepted")
			}
		})
	}
	t.Run("disabled experimental profile remains valid", func(t *testing.T) {
		profiles := base()
		experimental := profiles["necro_bone_spear"]
		experimental.DisplayName = "Experiment"
		experimental.Setup = ProfileSetupConfig{}
		profiles["experimental"] = experimental
		if err := profiles.validateSetupMetadata(); err != nil {
			t.Fatal(err)
		}
	})
}

func TestCharacterSetupDefaultsAndStructuralValidation(t *testing.T) {
	var setup CharacterSetupConfig
	setup.applyDefaults()
	if got := setup.PickitDefaults["countess"]; strings.Join(got, ",") != "gems,keys,countess-standard" {
		t.Fatalf("countess=%v", got)
	}
	if got := setup.PickitDefaults["mephisto"]; strings.Join(got, ",") != "gems,mephisto-standard" {
		t.Fatalf("mephisto=%v", got)
	}
	if got := setup.PickitDefaults["lower-kurast"]; strings.Join(got, ",") != "gems,lk-superchests" {
		t.Fatalf("lower-kurast=%v", got)
	}
	for _, runID := range []string{"summoner", "nihlathak"} {
		if got := setup.PickitDefaults[runID]; strings.Join(got, ",") != "gems,keys" {
			t.Fatalf("%s=%v", runID, got)
		}
	}
	if err := setup.validate(); err != nil {
		t.Fatal(err)
	}
	setup.PickitDefaults["countess"] = []string{"gems", "gems"}
	if err := setup.validate(); err == nil {
		t.Fatal("duplicate default profile was accepted")
	}
}

func TestDefaultCharacterSetupPickitChainsReturnsIndependentCopies(t *testing.T) {
	first := DefaultCharacterSetupPickitChains()
	first["summoner"][0] = "mutated"
	if got := DefaultCharacterSetupPickitChains()["summoner"][0]; got != "gems" {
		t.Fatalf("default chain leaked caller mutation: %q", got)
	}
}

func TestProfileRequiredSkillsAndCombatValidation(t *testing.T) {
	base := func() ProfilesConfig {
		var profiles ProfilesConfig
		profiles.applyDefaults()
		return profiles
	}
	tests := []struct {
		name    string
		mutate  func(ProfilesConfig)
		wantSub string
	}{
		{name: "missing standard attack", mutate: func(profiles ProfilesConfig) {
			value := profiles["necro_bone_spear"]
			value.Combat.StandardAttack = ""
			profiles["necro_bone_spear"] = value
		}, wantSub: "standard_attack"},
		{name: "standard attack missing from catalog", mutate: func(profiles ProfilesConfig) {
			value := profiles["necro_bone_spear"]
			value.Combat.StandardAttack = "not_a_real_skill"
			profiles["necro_bone_spear"] = value
		}, wantSub: "skill catalog"},
		{name: "standard attack not required", mutate: func(profiles ProfilesConfig) {
			value := profiles["necro_bone_spear"]
			value.Combat.StandardAttack = "bone_wall"
			profiles["necro_bone_spear"] = value
		}, wantSub: "required_skills"},
		{name: "teleport missing", mutate: func(profiles ProfilesConfig) {
			value := profiles["necro_bone_spear"]
			value.RequiredSkills = value.RequiredSkills[1:]
			profiles["necro_bone_spear"] = value
		}, wantSub: "teleport"},
		{name: "town portal missing", mutate: func(profiles ProfilesConfig) {
			value := profiles["necro_bone_spear"]
			filtered := make([]RequiredSkillConfig, 0, len(value.RequiredSkills))
			for _, skill := range value.RequiredSkills {
				if skill.Skill != "town_portal" {
					filtered = append(filtered, skill)
				}
			}
			value.RequiredSkills = filtered
			profiles["necro_bone_spear"] = value
		}, wantSub: "town_portal"},
		{name: "duplicate required", mutate: func(profiles ProfilesConfig) {
			value := profiles["necro_bone_spear"]
			value.RequiredSkills = append(value.RequiredSkills, RequiredSkillConfig{Skill: "teleport", DisplayName: "Teleport"})
			profiles["necro_bone_spear"] = value
		}, wantSub: "duplicate"},
		{name: "more than eight", mutate: func(profiles ProfilesConfig) {
			value := profiles["necro_bone_spear"]
			value.RequiredSkills = append(value.RequiredSkills, RequiredSkillConfig{Skill: "bone_wall", DisplayName: "Knochenwand"})
			value.RequiredSkills = append(value.RequiredSkills, RequiredSkillConfig{Skill: "attack", DisplayName: "Angriff"})
			profiles["necro_bone_spear"] = value
		}, wantSub: "at most 8"},
		{name: "hook skill missing from required", mutate: func(profiles ProfilesConfig) {
			value := profiles["necro_bone_spear"]
			filtered := make([]RequiredSkillConfig, 0, len(value.RequiredSkills))
			for _, skill := range value.RequiredSkills {
				if skill.Skill != "bone_armor" {
					filtered = append(filtered, skill)
				}
			}
			value.RequiredSkills = filtered
			profiles["necro_bone_spear"] = value
		}, wantSub: "hooks.town_ready"},
		{name: "maintenance skill missing from required", mutate: func(profiles ProfilesConfig) {
			value := profiles["necro_bone_spear"]
			filtered := make([]RequiredSkillConfig, 0, len(value.RequiredSkills))
			for _, skill := range value.RequiredSkills {
				if skill.Skill != "bone_armor" {
					filtered = append(filtered, skill)
				}
			}
			value.RequiredSkills = filtered
			value.Hooks.TownReady = nil
			profiles["necro_bone_spear"] = value
		}, wantSub: "route_maintenance"},
		{name: "invalid german label", mutate: func(profiles ProfilesConfig) {
			value := profiles["necro_bone_spear"]
			value.RequiredSkills[0].DisplayName = " Teleport"
			profiles["necro_bone_spear"] = value
		}, wantSub: "display_name"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			profiles := base()
			test.mutate(profiles)
			err := profiles.validate("necro_bone_spear", "test")
			if err == nil || !strings.Contains(err.Error(), test.wantSub) {
				t.Fatalf("error = %v, want substring %q", err, test.wantSub)
			}
		})
	}
}

func TestProfileValidationRejectsMissingResourceCooldown(t *testing.T) {
	var profiles ProfilesConfig
	profiles.applyDefaults()
	got := profiles["necro_bone_spear"]
	got.Resources.Healing.CooldownMs = 0
	profiles["necro_bone_spear"] = got
	if err := profiles.validate("necro_bone_spear", "test"); err == nil {
		t.Fatal("expected resource cooldown error")
	}
}

func TestProfileValidationRejectsInvalidSlot(t *testing.T) {
	var profiles ProfilesConfig
	profiles.applyDefaults()
	got := profiles["necro_bone_spear"]
	got.Resources.Mana.BeltSlots = []int{2, 2}
	profiles["necro_bone_spear"] = got
	if err := profiles.validate("necro_bone_spear", "test"); err == nil {
		t.Fatal("expected duplicate belt slot error")
	}
}
