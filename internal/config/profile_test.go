package config

import (
	"strings"
	"testing"
)

func TestDefaultNecroProfileContract(t *testing.T) {
	var profiles ProfilesConfig
	profiles.applyDefaults()
	got := profiles["necro_bone_spear"]
	if got.CharacterClass != "necromancer" || got.DisplayName != "Knochen-Speer" || !got.Setup.Enabled || !got.Setup.Default ||
		len(got.Hooks.TownReady) != 1 || got.Hooks.TownReady[0].Skill != "bone_armor" {
		t.Fatalf("default profile = %+v", got)
	}
	if got.Resources.Mana.UseBelowPercent != 35 || len(got.Resources.Mana.BeltSlots) != 2 || got.Resources.Mana.CooldownMs != 4000 {
		t.Fatalf("default mana policy = %+v", got.Resources.Mana)
	}
	enabled, merc := got.Resources.Mercenary.Resolve()
	if !enabled || merc.UseBelowPercent != 50 || len(merc.BeltSlots) != 1 || merc.BeltSlots[0] != 1 || merc.CooldownMs != 4000 {
		t.Fatalf("default mercenary policy = enabled=%v rule=%+v", enabled, merc)
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
