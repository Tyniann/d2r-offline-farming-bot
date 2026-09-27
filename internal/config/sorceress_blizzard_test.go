package config

import "testing"

func TestSorceressBlizzardDefaultsAndExample(t *testing.T) {
	var defaults ProfilesConfig
	defaults.ApplyDefaults()
	cfg, err := Load("../../configs/config.example.yaml")
	if err != nil {
		t.Fatal(err)
	}
	for _, profiles := range []ProfilesConfig{defaults, cfg.Profiles} {
		if err := profiles.validate("sorceress_blizzard", "test"); err != nil {
			t.Fatal(err)
		}
		p := profiles["sorceress_blizzard"]
		if !p.Setup.Enabled || !p.Setup.Default || p.Combat.AttackIntervalMs != 1800 || len(p.Hooks.BossEngage) != 3 || p.Hooks.TownReady[0].OncePerGame {
			t.Fatalf("profile=%+v", p)
		}
	}
	for _, mutate := range []func(*ProfileConfig){
		func(p *ProfileConfig) { p.Combat.AttackIntervalMs = 350 },
		func(p *ProfileConfig) { p.RequiresMercenary = false },
		func(p *ProfileConfig) { p.Hooks.BossEngage = p.Hooks.BossEngage[:1] },
		func(p *ProfileConfig) { p.CharacterClass = "necromancer" },
	} {
		p := defaults["sorceress_blizzard"]
		mutate(&p)
		if err := (ProfilesConfig{"sorceress_blizzard": p}).validate("sorceress_blizzard", "test"); err == nil {
			t.Fatalf("invalid profile accepted: %+v", p)
		}
	}
}
