package app

import (
	"strings"
	"testing"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
)

func TestResolveRuntimeCombatProfileUsesFrozenLoadout(t *testing.T) {
	cfg := &config.Config{
		Session: config.SessionConfig{Character: "MrBones"},
		Profiles: config.ProfilesConfig{
			"necro_bone_spear":  {Setup: config.ProfileSetupConfig{Enabled: true, Default: true}},
			"necro_bone_spirit": {},
		},
	}
	loadout := &CharacterLoadoutSnapshot{Character: "MrBones", ProfileID: "necro_bone_spirit"}

	got, err := resolveRuntimeCombatProfileID(cfg, loadout)
	if err != nil || got != "necro_bone_spirit" {
		t.Fatalf("runtime profile=%q err=%v", got, err)
	}
	rt := &Runtime{Config: cfg, combatProfileID: got}
	if resolved, resolveErr := rt.resolvedCombatProfileID(); resolveErr != nil || resolved != "necro_bone_spirit" {
		t.Fatalf("frozen runtime profile=%q err=%v", resolved, resolveErr)
	}
}

func TestResolveRuntimeCombatProfileRejectsInvalidFreeze(t *testing.T) {
	cfg := &config.Config{
		Session:  config.SessionConfig{Character: "MrBones"},
		Profiles: config.ProfilesConfig{"necro_bone_spear": {}},
	}
	tests := []struct {
		name    string
		loadout CharacterLoadoutSnapshot
		want    string
	}{
		{name: "missing profile", loadout: CharacterLoadoutSnapshot{Character: "MrBones"}, want: "no combat profile"},
		{name: "unknown profile", loadout: CharacterLoadoutSnapshot{Character: "MrBones", ProfileID: "missing"}, want: "unknown"},
		{name: "wrong character", loadout: CharacterLoadoutSnapshot{Character: "MrHammer", ProfileID: "necro_bone_spear"}, want: "belongs to"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := resolveRuntimeCombatProfileID(cfg, &test.loadout); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error=%v, want %q", err, test.want)
			}
		})
	}
}
