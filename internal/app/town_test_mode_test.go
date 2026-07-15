package app

import "testing"

func TestValidateAkaraBulkProfile(t *testing.T) {
	cfg := fullCountessConfig()
	if err := validateAkaraBulkProfile(cfg); err != nil {
		t.Fatal(err)
	}
	runCfg, _ := cfg.Runs.Run("countess")
	profileCfg := cfg.Profiles[runCfg.Combat.Profile]
	profileCfg.Resources.Rejuvenation.BeltSlots = nil
	cfg.Profiles[runCfg.Combat.Profile] = profileCfg
	if err := validateAkaraBulkProfile(cfg); err == nil {
		t.Fatal("incomplete belt profile accepted")
	}
}

func TestValidateAkaraBulkProfileProtectsRejuvenationSlot(t *testing.T) {
	cfg := fullCountessConfig()
	runCfg, _ := cfg.Runs.Run("countess")
	profileCfg := cfg.Profiles[runCfg.Combat.Profile]
	profileCfg.Resources.Rejuvenation.BeltSlots = []int{3}
	profileCfg.Resources.Mana.BeltSlots = []int{2, 4}
	cfg.Profiles[runCfg.Combat.Profile] = profileCfg
	if err := validateAkaraBulkProfile(cfg); err == nil {
		t.Fatal("unprotected slot 4 accepted")
	}
}
