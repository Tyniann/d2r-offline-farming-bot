package necrobonespear

import (
	"context"
	"testing"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/memory"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/profile"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

type routeClearActionsStub struct {
	skills []uint16
	sent   bool
}

func (s *routeClearActionsStub) CastAttackAtMonster(_ time.Time, skillID uint16, _ world.Player, _ world.Monster) (profile.MonsterCastResult, error) {
	s.skills = append(s.skills, skillID)
	return profile.MonsterCastResult{Sent: s.sent}, nil
}

func (s *routeClearActionsStub) StopAttack() error { return nil }

type profileActionsStub struct{}

func (profileActionsStub) CastSkillAtWorld(time.Time, uint16, world.Player, world.Position) error {
	return nil
}
func (profileActionsStub) CastBelt(int) error             { return nil }
func (profileActionsStub) CastBeltForMercenary(int) error { return nil }

func TestNihlathakStrategyConfiguresPostBossRouteClear(t *testing.T) {
	strategy := NewNihlathakFactory()()
	clear, ok := strategy.(profile.SupportsRouteClear)
	if !ok {
		t.Fatal("nihlathak strategy must expose route clear for post-boss cleanup")
	}
	if clear.RequiresRouteClear() {
		t.Fatal("nihlathak must not require travel route_clear capability")
	}
	foundAD := false
	for _, skill := range strategy.RequiredSkills() {
		if skill == "amplify_damage" {
			foundAD = true
			break
		}
	}
	if !foundAD {
		t.Fatalf("required skills = %v", strategy.RequiredSkills())
	}

	definition := profile.Definition{ID: "necro_bone_spear", CharacterClass: world.CharacterClassNecromancer}
	executor, err := profile.NewExecutor(config.NewLogger("error"), definition, profileActionsStub{})
	if err != nil {
		t.Fatal(err)
	}
	actions := &routeClearActionsStub{sent: true}
	if configureErr := strategy.Configure(executor, memory.SkillBoneSpear, actions); configureErr != nil {
		t.Fatal(configureErr)
	}
	now := time.Now()
	result := executor.TickRouteClear(context.Background(), profile.RouteClearRequest{
		RunID:        "nihlathak",
		DefinitionID: "necro_bone_spear",
		Player:       world.Player{Position: world.Position{X: 100, Y: 100}},
		Target:       world.Monster{UnitID: 9, NPCID: 1, Position: world.Position{X: 110, Y: 100}},
		Mode:         profile.RouteClearThreat,
		AssessmentAt: now,
	}, now)
	if result.Status != profile.StatusAction || result.SkillID != memory.SkillAmplifyDamage ||
		result.Reason == "route_clear_strategy_unavailable" {
		t.Fatalf("opener result = %+v", result)
	}
	if len(actions.skills) != 1 || actions.skills[0] != memory.SkillAmplifyDamage {
		t.Fatalf("skills = %v", actions.skills)
	}
}

func TestLowerKurastStrategyConfiguresLocalClearWithoutTravelRouteClear(t *testing.T) {
	strategy := NewLowerKurastFactory()()
	if strategy.ProfileID() != "necro_bone_spear" || strategy.RunID() != "lower-kurast" {
		t.Fatalf("strategy identity = %s/%s", strategy.ProfileID(), strategy.RunID())
	}
	clear, ok := strategy.(profile.SupportsRouteClear)
	if !ok || clear.RequiresRouteClear() {
		t.Fatal("lower-kurast must wire local clear without travel route_clear")
	}
	definition := profile.Definition{ID: "necro_bone_spear", CharacterClass: world.CharacterClassNecromancer}
	executor, err := profile.NewExecutor(config.NewLogger("error"), definition, profileActionsStub{})
	if err != nil {
		t.Fatal(err)
	}
	actions := &routeClearActionsStub{sent: true}
	if configureErr := strategy.Configure(executor, memory.SkillBoneSpear, actions); configureErr != nil {
		t.Fatal(configureErr)
	}
	now := time.Now()
	result := executor.TickRouteClear(context.Background(), profile.RouteClearRequest{
		RunID:        "lower-kurast",
		DefinitionID: "necro_bone_spear",
		Player:       world.Player{Position: world.Position{X: 100, Y: 100}},
		Target:       world.Monster{UnitID: 9, NPCID: 1, Position: world.Position{X: 110, Y: 100}},
		Mode:         profile.RouteClearThreat,
		AssessmentAt: now,
	}, now)
	if result.Status != profile.StatusAction || result.SkillID != memory.SkillAmplifyDamage {
		t.Fatalf("opener result = %+v, want Amplify Damage", result)
	}
	result = executor.TickRouteClear(context.Background(), profile.RouteClearRequest{
		RunID:        "lower-kurast",
		DefinitionID: "necro_bone_spear",
		Player:       world.Player{Position: world.Position{X: 100, Y: 100}},
		Target:       world.Monster{UnitID: 9, NPCID: 1, Position: world.Position{X: 110, Y: 100}},
		Mode:         profile.RouteClearThreat,
		AssessmentAt: now.Add(time.Second),
	}, now.Add(time.Second))
	if result.Status != profile.StatusAction || result.SkillID != memory.SkillBoneSpear {
		t.Fatalf("attack result = %+v, want Bone Spear", result)
	}
}
