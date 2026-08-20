package api

import (
	"testing"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
)

func TestBootstrapBackendIsReadOnlyAndDeterministic(t *testing.T) {
	cfg, err := config.Load("../../configs/config.example.yaml")
	if err != nil {
		t.Fatal(err)
	}
	experimental := cfg.Profiles["necro_bone_spear"]
	experimental.CharacterClass = "paladin"
	experimental.Setup.Enabled = false
	experimental.Setup.Default = false
	cfg.Profiles["experimental_paladin"] = experimental
	backend, err := NewBootstrapBackend(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if status := backend.Status(); status.State != "idle" || status.D2R.State != "detached" {
		t.Fatalf("bootstrap status = %+v", status)
	}
	first := backend.Catalog()
	if len(first.Runs) != 6 {
		t.Fatalf("bootstrap runs = %+v", first.Runs)
	}
	if len(first.Profiles) != 2 || first.Profiles[0].ID != "necro_bone_spear" || first.Profiles[1].ID != "paladin_hammerdin" {
		t.Fatalf("bootstrap setup profiles = %+v", first.Profiles)
	}
	for _, run := range first.Runs {
		if run.RunID == "summoner" || run.RunID == "cows" {
			if !run.RouteCombat.Enabled || run.RouteCombat.NoProgressTimeoutMs != 12000 || run.RouteCombat.ManaRecoveryTimeoutMs != 5000 {
				t.Fatalf("effective summoner route combat = %+v", run.RouteCombat)
			}
		} else if run.RouteCombat.Enabled {
			t.Fatalf("route combat unexpectedly enabled for %s", run.RunID)
		}
	}
	first.Runs[0].RunID = "mutated"
	if second := backend.Catalog(); second.Runs[0].RunID == "mutated" {
		t.Fatal("catalog caller mutation changed backend state")
	}
	if _, err := backend.Command("start_queue", CommandRequest{CommandID: "forbidden"}); err == nil {
		t.Fatal("bootstrap backend accepted a command")
	}
}
