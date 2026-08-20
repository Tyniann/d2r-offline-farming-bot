package app

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/tasks"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

func TestPhase16D2SContractUsesCharacterizedV105Prefix(t *testing.T) {
	if Phase16D2SPrefixLength != Phase16D2SNameOffset+Phase16D2SNameLength {
		t.Fatalf("prefix=%d name_end=%d", Phase16D2SPrefixLength, Phase16D2SNameOffset+Phase16D2SNameLength)
	}
	if Phase16D2SPrefixLength != 315 || Phase16D2SMagic != 0xAA55AA55 || Phase16D2SClassOffset != 0x18 {
		t.Fatal("phase-16 D2S layout drifted")
	}
	if got := Phase16D2SAllowedVersions(); !reflect.DeepEqual(got, []uint32{105}) {
		t.Fatalf("versions=%v", got)
	}
}

func TestCharacterSetupConfigValidatesExactRegisteredChainsAndProfiles(t *testing.T) {
	cfg, err := config.Load(filepath.Join("..", "..", "configs", "config.example.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	profiles, err := NewPickitProfileService(filepath.Join("..", "..", "configs", "pickit", "profiles"))
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateCharacterSetupConfig(cfg, profiles); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name   string
		mutate func(map[string][]string)
	}{
		{name: "missing run", mutate: func(defaults map[string][]string) { delete(defaults, "mephisto") }},
		{name: "unknown run", mutate: func(defaults map[string][]string) { defaults["andariel"] = []string{"gems"} }},
		{name: "wrong order", mutate: func(defaults map[string][]string) {
			defaults["countess"] = []string{"keys", "gems", "countess-standard"}
		}},
		{name: "unknown profile", mutate: func(defaults map[string][]string) {
			defaults["countess"] = []string{"gems", "keys", "missing"}
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			copyCfg := *cfg
			copyCfg.CharacterSetup.PickitDefaults = make(map[string][]string, len(cfg.CharacterSetup.PickitDefaults))
			for runID, values := range cfg.CharacterSetup.PickitDefaults {
				copyCfg.CharacterSetup.PickitDefaults[runID] = append([]string(nil), values...)
			}
			test.mutate(copyCfg.CharacterSetup.PickitDefaults)
			if err := ValidateCharacterSetupConfig(&copyCfg, profiles); err == nil {
				t.Fatal("invalid character setup config was accepted")
			}
		})
	}
}

func TestPhase16D2SClassesReuseCanonicalWorldValues(t *testing.T) {
	mappings := Phase16D2SClasses()
	if len(mappings) != 8 {
		t.Fatalf("mappings=%+v", mappings)
	}
	for id, mapping := range mappings {
		if mapping.ID != byte(id) || mapping.Class != world.CharacterClass(id) {
			t.Fatalf("mapping[%d]=%+v", id, mapping)
		}
	}
	if mappings[7].Class != world.CharacterClassWarlock || mappings[7].Class.String() != "warlock" {
		t.Fatalf("warlock=%+v", mappings[7])
	}
}

func TestPhase16OwnershipReasonsDefaultsAndNonGoalsAreComplete(t *testing.T) {
	if owners := Phase16ContractOwners(); len(owners) != 8 || owners[0].Owner != "internal/app" || owners[len(owners)-1].Owner != "React reason projection" {
		t.Fatalf("owners=%+v", owners)
	}
	reasons := Phase16CharacterReasonCodes()
	if len(reasons) != 13 {
		t.Fatalf("reasons=%v", reasons)
	}
	seen := make(map[Phase16CharacterReasonCode]struct{}, len(reasons))
	for _, reason := range reasons {
		if reason == "" {
			t.Fatal("empty reason code")
		}
		if _, duplicate := seen[reason]; duplicate {
			t.Fatalf("duplicate reason=%q", reason)
		}
		seen[reason] = struct{}{}
	}
	defaults := Phase16DefaultPickitChains()
	if !reflect.DeepEqual(defaults[tasks.RunIDCountess], []string{"gems", "keys", "countess-standard"}) ||
		!reflect.DeepEqual(defaults[tasks.RunIDMephisto], []string{"gems", "mephisto-standard"}) ||
		!reflect.DeepEqual(defaults[tasks.RunIDLowerKurast], []string{"gems", "lk-superchests"}) {
		t.Fatalf("defaults=%v", defaults)
	}
	defaults[tasks.RunIDCountess][0] = "mutated"
	if Phase16DefaultPickitChains()[tasks.RunIDCountess][0] != "gems" {
		t.Fatal("default chains must be returned as independent values")
	}
	if nonGoals := Phase16NonGoals(); len(nonGoals) != 10 {
		t.Fatalf("non-goals=%v", nonGoals)
	}
}
