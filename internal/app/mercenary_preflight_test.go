package app

import (
	"context"
	"testing"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/tasks"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

type readinessTownMock struct {
	results []tasks.TownPreparationResult
	resets  int
}

func (m *readinessTownMock) Tick(context.Context, world.State) tasks.TownPreparationResult {
	result := m.results[0]
	m.results = m.results[1:]
	return result
}

func (m *readinessTownMock) Reset() { m.resets++ }

func TestConsumeMercenaryPreflightSkipsOfflineGameStart(t *testing.T) {
	rt := &Runtime{
		Config: &config.Config{
			Session:  config.SessionConfig{Run: "nihlathak"},
			Runs:     config.RunsConfig{Definitions: map[string]config.RunConfig{"nihlathak": {}}},
			Profiles: config.ProfilesConfig{"necro_bone_spear": {Resources: config.ProfileResourcesConfig{}}},
		},
		Options:             Options{OfflineDifficulty: "hell"},
		runReadinessPending: true,
		productiveRunActive: true,
	}
	state := world.State{
		Valid: true, Phase: world.GamePhaseInGame, Area: world.LookupArea(world.RogueEncampment),
		Identity: world.GameIdentity{Valid: true, CharacterName: "MrBones"},
	}
	if _, err := rt.consumeRunReadiness(context.Background(), state); err != nil {
		t.Fatalf("offline start must not fail merc preflight: %v", err)
	}
	if !rt.runReadinessPending {
		t.Fatal("pending flag must remain set until a productive in-game tick")
	}
}

func TestConsumeMercenaryPreflightRequiresConfirmedTownIdentity(t *testing.T) {
	rt := &Runtime{
		Config: &config.Config{
			Session:  config.SessionConfig{Run: "nihlathak"},
			Runs:     config.RunsConfig{Definitions: map[string]config.RunConfig{"nihlathak": {}}},
			Profiles: config.ProfilesConfig{"necro_bone_spear": {Resources: config.ProfileResourcesConfig{}}},
		},
		runReadinessPending: true,
		productiveRunActive: true,
	}
	loading := world.State{Valid: true, Phase: world.GamePhaseInGame, Area: world.LookupArea(world.RogueEncampment)}
	ready, err := rt.consumeRunReadiness(context.Background(), loading)
	if err != nil {
		t.Fatalf("unconfirmed identity must wait: %v", err)
	}
	if ready {
		t.Fatal("unconfirmed identity must block productive task ticks")
	}
	if !rt.runReadinessPending {
		t.Fatal("pending flag cleared too early")
	}
}

func TestConsumeMercenaryPreflightRecoversDeadHiredMercenary(t *testing.T) {
	townActions := &readinessTownMock{results: []tasks.TownPreparationResult{{Status: "pending"}, {Status: "complete", Done: true}}}
	rt := &Runtime{
		Config: &config.Config{
			Session:  config.SessionConfig{Run: "nihlathak"},
			Runs:     config.RunsConfig{Definitions: map[string]config.RunConfig{"nihlathak": {}}},
			Profiles: config.ProfilesConfig{"necro_bone_spear": {Resources: config.ProfileResourcesConfig{}}},
		},
		taskDeps: tasks.Deps{Town: townActions}, runReadinessPending: true, productiveRunActive: true,
	}
	state := world.State{
		Valid: true, Phase: world.GamePhaseInGame, Area: world.LookupArea(world.RogueEncampment),
		Identity:  world.GameIdentity{Valid: true, CharacterName: "MrBones"},
		Mercenary: world.Mercenary{HiredKnown: true, Hired: true, Dead: true},
	}
	ready, err := rt.consumeRunReadiness(context.Background(), state)
	if err != nil || ready {
		t.Fatalf("pending recovery ready=%t err=%v", ready, err)
	}
	ready, err = rt.consumeRunReadiness(context.Background(), state)
	if err != nil || !ready || rt.runReadinessPending || townActions.resets != 1 {
		t.Fatalf("completed recovery ready=%t err=%v pending=%t resets=%d", ready, err, rt.runReadinessPending, townActions.resets)
	}
}

func TestConsumeRunReadinessLeavesStandardRunWithLivingMercInputFree(t *testing.T) {
	rt := &Runtime{
		Config: &config.Config{
			Session:  config.SessionConfig{Run: "summoner"},
			Runs:     config.RunsConfig{Definitions: map[string]config.RunConfig{"summoner": {}}},
			Profiles: config.ProfilesConfig{"necro_bone_spear": {Resources: config.ProfileResourcesConfig{}}},
		},
		runReadinessPending: true,
		productiveRunActive: true,
	}
	state := world.State{
		Valid: true, Phase: world.GamePhaseInGame, Area: world.LookupArea(world.RogueEncampment),
		Identity: world.GameIdentity{Valid: true, CharacterName: "MrBones"},
		Mercenary: world.Mercenary{
			HiredKnown: true, Hired: true, Alive: true, VitalsKnown: true, HP: 100, MaxHP: 100,
		},
	}
	ready, err := rt.consumeRunReadiness(context.Background(), state)
	if err != nil || !ready || rt.runReadinessPending {
		t.Fatalf("living standard readiness ready=%t err=%v pending=%t", ready, err, rt.runReadinessPending)
	}
}

func TestConsumeRunReadinessRequiresHammerdinMercenaryWhenPotionPolicyDisabled(t *testing.T) {
	disabled := false
	rt := &Runtime{
		Config: &config.Config{
			Session: config.SessionConfig{Run: "mephisto"},
			Runs:    config.RunsConfig{Definitions: map[string]config.RunConfig{"mephisto": {}}},
			Profiles: config.ProfilesConfig{"paladin_hammerdin": {
				RequiresMercenary: true,
				Resources:         config.ProfileResourcesConfig{Mercenary: config.MercenaryResourceConfig{Enabled: &disabled}},
			}},
		},
		combatProfileID:     "paladin_hammerdin",
		runReadinessPending: true,
		productiveRunActive: true,
	}
	state := world.State{
		Valid: true, Phase: world.GamePhaseInGame, Area: world.LookupArea(world.RogueEncampment),
		Identity:  world.GameIdentity{Valid: true, CharacterName: "MrHammer"},
		Mercenary: world.Mercenary{HiredKnown: true, Hired: false},
	}
	ready, err := rt.consumeRunReadiness(context.Background(), state)
	if err == nil || err.Error() != "mercenary_not_hired" || ready || rt.runReadinessPending {
		t.Fatalf("Hammerdin merc gate ready=%t err=%v pending=%t", ready, err, rt.runReadinessPending)
	}
}
