package config

import "testing"

func TestDefaultNecroProfileContract(t *testing.T) {
	var profiles ProfilesConfig
	profiles.applyDefaults()
	got := profiles["necro_bone_spear"]
	if got.CharacterClass != "necromancer" || len(got.Hooks.TownReady) != 1 || got.Hooks.TownReady[0].Skill != "bone_armor" {
		t.Fatalf("default profile = %+v", got)
	}
	if got.Resources.Mana.UseBelowPercent != 35 || len(got.Resources.Mana.BeltSlots) != 2 || got.Resources.Mana.CooldownMs != 4000 {
		t.Fatalf("default mana policy = %+v", got.Resources.Mana)
	}
}

func TestProfileValidationRejectsMissingResourceCooldown(t *testing.T) {
	var profiles ProfilesConfig
	profiles.applyDefaults()
	got := profiles["necro_bone_spear"]
	got.Resources.Healing.CooldownMs = 0
	profiles["necro_bone_spear"] = got
	if err := profiles.validate("necro_bone_spear"); err == nil {
		t.Fatal("expected resource cooldown error")
	}
}

func TestProfileValidationRejectsInvalidSlot(t *testing.T) {
	var profiles ProfilesConfig
	profiles.applyDefaults()
	got := profiles["necro_bone_spear"]
	got.Resources.Mana.BeltSlots = []int{2, 2}
	profiles["necro_bone_spear"] = got
	if err := profiles.validate("necro_bone_spear"); err == nil {
		t.Fatal("expected duplicate belt slot error")
	}
}
