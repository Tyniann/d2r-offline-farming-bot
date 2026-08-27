package hammerdin

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

func TestBossStrategyContract(t *testing.T) {
	wantSkills := []string{"teleport", "town_portal", "blessed_hammer", "concentration", "holy_shield"}
	for _, runID := range []string{"countess", "mephisto", "nihlathak"} {
		strategy := NewBossFactory(runID)()
		if strategy.ProfileID() != "paladin_hammerdin" || strategy.RunID() != runID {
			t.Fatalf("strategy identity = %s/%s", strategy.ProfileID(), strategy.RunID())
		}
		got := strategy.RequiredSkills()
		if len(got) != len(wantSkills) {
			t.Fatalf("%s required skills = %v", runID, got)
		}
		for index := range wantSkills {
			if got[index] != wantSkills[index] {
				t.Fatalf("%s required skills = %v", runID, got)
			}
		}
		clear, ok := strategy.(profile.SupportsRouteClear)
		if !ok || clear.RequiresRouteClear() {
			t.Fatalf("%s must wire local clear without travel route clear", runID)
		}
		if _, ok := strategy.(profile.SupportsLocalRecoveryClear); !ok {
			t.Fatalf("%s must expose local recovery clear", runID)
		}
		executor, err := profile.NewExecutor(config.NewLogger("error"), profile.Definition{ID: "paladin_hammerdin", CharacterClass: world.CharacterClassPaladin}, profileActionsStub{})
		if err != nil {
			t.Fatal(err)
		}
		actions := &routeClearActionsStub{sent: true}
		if err := strategy.Configure(executor, memory.MustSkillID("blessed_hammer"), actions); err != nil {
			t.Fatal(err)
		}
		now := time.Now()
		result := executor.TickRouteClear(context.Background(), profile.RouteClearRequest{
			RunID: runID, DefinitionID: "paladin_hammerdin",
			Player: world.Player{Position: world.Position{X: 100, Y: 100}},
			Target: world.Monster{UnitID: 9, NPCID: 1, Position: world.Position{X: 102, Y: 100}},
			Mode:   profile.RouteClearThreat, AssessmentAt: now,
		}, now)
		if result.Status != profile.StatusAction || result.SkillID != memory.MustSkillID("blessed_hammer") {
			t.Fatalf("%s local clear result = %+v", runID, result)
		}
		if err := strategy.Configure(executor, memory.MustSkillID("bone_spear"), &routeClearActionsStub{}); err == nil {
			t.Fatalf("%s accepted wrong standard attack", runID)
		}
	}
}

func TestLowerKurastStrategyWiresLocalClearWithoutTravelRouteClear(t *testing.T) {
	strategy := NewLowerKurastFactory()()
	if strategy.ProfileID() != "paladin_hammerdin" || strategy.RunID() != "lower-kurast" {
		t.Fatalf("strategy identity = %s/%s", strategy.ProfileID(), strategy.RunID())
	}
	clear, ok := strategy.(profile.SupportsRouteClear)
	if !ok || clear.RequiresRouteClear() {
		t.Fatal("lower-kurast must wire local clear without travel route_clear")
	}
	definition := profile.Definition{ID: "paladin_hammerdin", CharacterClass: world.CharacterClassPaladin}
	executor, err := profile.NewExecutor(config.NewLogger("error"), definition, profileActionsStub{})
	if err != nil {
		t.Fatal(err)
	}
	actions := &routeClearActionsStub{sent: true}
	if configureErr := strategy.Configure(executor, memory.MustSkillID("blessed_hammer"), actions); configureErr != nil {
		t.Fatal(configureErr)
	}
	now := time.Now()
	result := executor.TickRouteClear(context.Background(), profile.RouteClearRequest{
		RunID:        "lower-kurast",
		DefinitionID: "paladin_hammerdin",
		Player:       world.Player{Position: world.Position{X: 100, Y: 100}},
		Target:       world.Monster{UnitID: 9, NPCID: 1, Position: world.Position{X: 102, Y: 100}},
		Mode:         profile.RouteClearThreat,
		AssessmentAt: now,
	}, now)
	if result.Status != profile.StatusAction || result.SkillID != memory.MustSkillID("blessed_hammer") ||
		result.ActionKind != profile.RouteClearActionAttack {
		t.Fatalf("local clear result = %+v, want Blessed Hammer without opener", result)
	}
}

func TestSummonerStrategyWiresRouteClearWithoutCurseOpener(t *testing.T) {
	strategy := NewSummonerFactory()()
	if strategy.ProfileID() != "paladin_hammerdin" || strategy.RunID() != "summoner" {
		t.Fatalf("strategy identity = %s/%s", strategy.ProfileID(), strategy.RunID())
	}
	clear, ok := strategy.(profile.SupportsRouteClear)
	if !ok || !clear.RequiresRouteClear() {
		t.Fatal("summoner must require travel route_clear")
	}
	wantSkills := []string{"teleport", "town_portal", "blessed_hammer", "concentration", "holy_shield"}
	got := strategy.RequiredSkills()
	if len(got) != len(wantSkills) {
		t.Fatalf("required skills = %v", got)
	}
	for index := range wantSkills {
		if got[index] != wantSkills[index] {
			t.Fatalf("required skills = %v", got)
		}
	}

	definition := profile.Definition{ID: "paladin_hammerdin", CharacterClass: world.CharacterClassPaladin}
	executor, err := profile.NewExecutor(config.NewLogger("error"), definition, profileActionsStub{})
	if err != nil {
		t.Fatal(err)
	}
	actions := &routeClearActionsStub{sent: true}
	if configureErr := strategy.Configure(executor, memory.MustSkillID("blessed_hammer"), actions); configureErr != nil {
		t.Fatal(configureErr)
	}
	if err := strategy.Configure(executor, memory.MustSkillID("blessed_hammer"), nil); err == nil {
		t.Fatal("summoner accepted missing route clear")
	}
	now := time.Now()
	result := executor.TickRouteClear(context.Background(), profile.RouteClearRequest{
		RunID:        "summoner",
		DefinitionID: "paladin_hammerdin",
		Player:       world.Player{Position: world.Position{X: 100, Y: 100}},
		Target:       world.Monster{UnitID: 9, NPCID: world.ArcaneSpecter, Position: world.Position{X: 102, Y: 100}},
		Mode:         profile.RouteClearThreat,
		AssessmentAt: now,
	}, now)
	if result.Status != profile.StatusAction || result.SkillID != memory.MustSkillID("blessed_hammer") ||
		result.ActionKind != profile.RouteClearActionAttack {
		t.Fatalf("attack result = %+v, want Blessed Hammer without curse opener", result)
	}
	if len(actions.skills) != 1 || actions.skills[0] != memory.MustSkillID("blessed_hammer") {
		t.Fatalf("skills = %v", actions.skills)
	}
}

func TestCowsStrategyWiresRouteClearWithoutCorpseExplosion(t *testing.T) {
	strategy := NewCowsFactory()()
	if strategy.ProfileID() != "paladin_hammerdin" || strategy.RunID() != "cows" {
		t.Fatalf("strategy identity = %s/%s", strategy.ProfileID(), strategy.RunID())
	}
	clear, ok := strategy.(profile.SupportsRouteClear)
	if !ok || !clear.RequiresRouteClear() {
		t.Fatal("cows must require travel route_clear")
	}
	if _, ok := strategy.(profile.SupportsCorpseExplosion); ok {
		t.Fatal("cows must not declare Corpse Explosion")
	}
	wantSkills := []string{"teleport", "town_portal", "blessed_hammer", "concentration", "holy_shield"}
	got := strategy.RequiredSkills()
	if len(got) != len(wantSkills) {
		t.Fatalf("required skills = %v", got)
	}
	for index := range wantSkills {
		if got[index] != wantSkills[index] {
			t.Fatalf("required skills = %v", got)
		}
	}

	definition := profile.Definition{ID: "paladin_hammerdin", CharacterClass: world.CharacterClassPaladin}
	executor, err := profile.NewExecutor(config.NewLogger("error"), definition, profileActionsStub{})
	if err != nil {
		t.Fatal(err)
	}
	actions := &routeClearActionsStub{sent: true}
	if configureErr := strategy.Configure(executor, memory.MustSkillID("blessed_hammer"), actions); configureErr != nil {
		t.Fatal(configureErr)
	}
	if err := strategy.Configure(executor, memory.MustSkillID("blessed_hammer"), nil); err == nil {
		t.Fatal("cows accepted missing route clear")
	}
	now := time.Now()
	result := executor.TickRouteClear(context.Background(), profile.RouteClearRequest{
		RunID:        "cows",
		DefinitionID: "paladin_hammerdin",
		Player:       world.Player{Position: world.Position{X: 100, Y: 100}},
		Target:       world.Monster{UnitID: 9, NPCID: world.HellBovine, Position: world.Position{X: 102, Y: 100}},
		Mode:         profile.RouteClearThreat,
		AssessmentAt: now,
	}, now)
	if result.Status != profile.StatusAction || result.SkillID != memory.MustSkillID("blessed_hammer") ||
		result.ActionKind != profile.RouteClearActionAttack {
		t.Fatalf("attack result = %+v, want Blessed Hammer without curse opener", result)
	}
	if len(actions.skills) != 1 || actions.skills[0] != memory.MustSkillID("blessed_hammer") {
		t.Fatalf("skills = %v", actions.skills)
	}
}
