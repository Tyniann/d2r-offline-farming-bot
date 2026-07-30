package config

import (
	"math"
	"strings"
	"testing"
)

func TestRouteCombatDefaultsAreRunIDAwareAndPreserveExplicitFalse(t *testing.T) {
	summoner := RunConfig{}
	summoner.Combat.applyDefaults()
	summoner.RouteCombat.applyDefaults("summoner")
	if !summoner.RouteCombat.EnabledValue() {
		t.Fatal("missing Summoner route_combat did not default enabled")
	}
	if summoner.RouteCombat.ImmediateRadiusTiles != 18 ||
		summoner.RouteCombat.CorridorWidthTiles != 7 ||
		summoner.RouteCombat.LandingRadiusTiles != 10 ||
		summoner.RouteCombat.AttackDistanceTiles != 30 ||
		summoner.RouteCombat.NoProgressTimeoutMs != 12000 ||
		summoner.RouteCombat.TeleportManaReservePercent != 20 ||
		summoner.RouteCombat.ResumeManaPercent != 35 ||
		summoner.RouteCombat.EmergencyManaPercent != 10 ||
		summoner.RouteCombat.ManaRecoveryTimeoutMs != 5000 {
		t.Fatalf("Summoner defaults = %+v", summoner.RouteCombat)
	}

	countess := RunConfig{}
	countess.Combat.applyDefaults()
	countess.RouteCombat.applyDefaults("countess")
	if countess.RouteCombat.EnabledValue() {
		t.Fatal("non-Summoner route_combat defaulted enabled")
	}
	disabled := false
	explicit := RouteCombatConfig{Enabled: &disabled}
	explicit.applyDefaults("summoner")
	if explicit.EnabledValue() {
		t.Fatal("explicit enabled:false was overwritten")
	}
}

func TestRouteCombatValidationMatrix(t *testing.T) {
	valid := RouteCombatConfig{}
	valid.applyDefaults("summoner")
	if err := valid.validate("summoner", "route_combat"); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name   string
		runID  string
		mutate func(*RouteCombatConfig)
		want   string
	}{
		{"unsupported run", "countess", func(c *RouteCombatConfig) { enabled := true; c.Enabled = &enabled }, "capable"},
		{"nan radius", "summoner", func(c *RouteCombatConfig) { c.ImmediateRadiusTiles = math.NaN() }, "finite"},
		{"corridor ordering", "summoner", func(c *RouteCombatConfig) { c.CorridorWidthTiles = c.ImmediateRadiusTiles }, "corridor_width"},
		{"landing ordering", "summoner", func(c *RouteCombatConfig) { c.LandingRadiusTiles = c.AttackDistanceTiles }, "landing_radius"},
		{"emergency ordering", "summoner", func(c *RouteCombatConfig) { c.EmergencyManaPercent = c.TeleportManaReservePercent }, "emergency_mana"},
		{"percent range", "summoner", func(c *RouteCombatConfig) { c.ResumeManaPercent = 101 }, "1..100"},
		{"clear timeout low", "summoner", func(c *RouteCombatConfig) { c.NoProgressTimeoutMs = 2999 }, "3000..30000"},
		{"mana timeout high", "summoner", func(c *RouteCombatConfig) { c.ManaRecoveryTimeoutMs = 15001 }, "1000..15000"},
		{"timeout ordering", "summoner", func(c *RouteCombatConfig) { c.NoProgressTimeoutMs = 4000; c.ManaRecoveryTimeoutMs = 5000 }, "<= no_progress"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := valid
			tc.mutate(&cfg)
			if err := cfg.validate(tc.runID, "route_combat"); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want containing %q", err, tc.want)
			}
		})
	}
}

func TestConfigExampleResolvesOnlySummonerRouteCombatEnabled(t *testing.T) {
	cfg, err := Load("../../configs/config.example.yaml")
	if err != nil {
		t.Fatal(err)
	}
	for id, run := range cfg.Runs.Definitions {
		want := id == "summoner"
		if run.RouteCombat.EnabledValue() != want {
			t.Fatalf("%s enabled=%t want=%t", id, run.RouteCombat.EnabledValue(), want)
		}
	}
}

func TestRouteCombatRejectsUnsupportedProfileBeforeRuntime(t *testing.T) {
	cfg, err := Load("../../configs/config.example.yaml")
	if err != nil {
		t.Fatal(err)
	}
	alternate := cfg.Profiles["necro_bone_spear"]
	alternate.Setup.Default = false
	cfg.Profiles["alternate_necro"] = alternate
	run := cfg.Runs.Definitions["summoner"]
	run.Combat.Profile = "alternate_necro"
	cfg.Runs.Definitions["summoner"] = run
	if err := cfg.validate(); err == nil || !strings.Contains(err.Error(), "requires profile necro_bone_spear") {
		t.Fatalf("error = %v", err)
	}
}
