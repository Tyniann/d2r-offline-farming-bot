package app

import (
	"testing"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

func TestConsumeMercenaryPreflightSkipsOfflineGameStart(t *testing.T) {
	rt := &Runtime{
		Config: &config.Config{
			Session:  config.SessionConfig{Run: "nihlathak"},
			Runs:     config.RunsConfig{Definitions: map[string]config.RunConfig{"nihlathak": {Combat: config.CombatConfig{Profile: "necro_bone_spear"}}}},
			Profiles: config.ProfilesConfig{"necro_bone_spear": {Resources: config.ProfileResourcesConfig{}}},
		},
		Options:              Options{OfflineDifficulty: "hell"},
		mercPreflightPending: true,
	}
	state := world.State{
		Valid: true, Phase: world.GamePhaseInGame, Area: world.LookupArea(world.RogueEncampment),
		Identity: world.GameIdentity{Valid: true, CharacterName: "MrBones"},
	}
	if err := rt.consumeMercenaryPreflight(state); err != nil {
		t.Fatalf("offline start must not fail merc preflight: %v", err)
	}
	if !rt.mercPreflightPending {
		t.Fatal("pending flag must remain set until a productive in-game tick")
	}
}

func TestConsumeMercenaryPreflightRequiresConfirmedTownIdentity(t *testing.T) {
	rt := &Runtime{
		Config: &config.Config{
			Session:  config.SessionConfig{Run: "nihlathak"},
			Runs:     config.RunsConfig{Definitions: map[string]config.RunConfig{"nihlathak": {Combat: config.CombatConfig{Profile: "necro_bone_spear"}}}},
			Profiles: config.ProfilesConfig{"necro_bone_spear": {Resources: config.ProfileResourcesConfig{}}},
		},
		mercPreflightPending: true,
	}
	loading := world.State{Valid: true, Phase: world.GamePhaseInGame, Area: world.LookupArea(world.RogueEncampment)}
	if err := rt.consumeMercenaryPreflight(loading); err != nil {
		t.Fatalf("unconfirmed identity must wait: %v", err)
	}
	if !rt.mercPreflightPending {
		t.Fatal("pending flag cleared too early")
	}
}
