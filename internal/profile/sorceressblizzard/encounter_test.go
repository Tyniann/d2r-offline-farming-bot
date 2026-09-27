package sorceressblizzard

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/memory"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/profile"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

type actionsStub struct {
	casts     []uint16
	teleports int
	pending   bool
}

func (a *actionsStub) CastSkillAtWorld(_ time.Time, skill uint16, _ world.Player, _ world.Position) error {
	if a.pending {
		return profile.ErrSkillSelectionPending
	}
	a.casts = append(a.casts, skill)
	return nil
}
func (*actionsStub) CastBelt(int) error             { return nil }
func (*actionsStub) CastBeltForMercenary(int) error { return nil }
func (a *actionsStub) TeleportToward(time.Time, world.Player, world.Position, float64) (bool, error) {
	a.teleports++
	return true, nil
}
func (a *actionsStub) CastAttackAtMonster(_ time.Time, skill uint16, _ world.Player, _ world.Monster) (profile.MonsterCastResult, error) {
	a.casts = append(a.casts, skill)
	return profile.MonsterCastResult{Sent: true}, nil
}
func (*actionsStub) StopAttack() error { return nil }

func encounterFixture(t *testing.T) (*EncounterExecutor, *actionsStub, world.State, profile.EncounterTarget) {
	t.Helper()
	a := &actionsStub{}
	static := profile.Action{SkillID: memory.MustSkillID("static_field"), Target: profile.TargetSelf, OncePerEncounter: true, Settle: 500 * time.Millisecond}
	exec, err := profile.NewExecutor(slog.New(slog.NewTextHandler(io.Discard, nil)), profile.Definition{
		ID: ID, CharacterClass: world.CharacterClassSorceress,
		Hooks: map[profile.Hook][]profile.Action{
			profile.HookBossEngage: {static, static, static},
			profile.HookTownReady:  {{SkillID: memory.MustSkillID("frozen_armor"), Target: profile.TargetSelf}},
		},
	}, a)
	if err != nil {
		t.Fatal(err)
	}
	w := world.State{Valid: true, Phase: world.GamePhaseInGame, At: time.Now().Add(time.Hour),
		Identity: world.GameIdentity{Valid: true, Class: world.CharacterClassSorceress},
		Player:   world.Player{Position: world.Position{X: 100, Y: 100}},
		Monsters: []world.Monster{{UnitID: 7, NPCID: world.Mephisto, Position: world.Position{X: 120, Y: 100}}},
	}
	return NewEncounterExecutor(exec, a), a, w, profile.EncounterTarget{UnitID: 7, Position: w.Monsters[0].Position}
}

func TestStaticFieldApproachConfirmationThreeCastsAndReset(t *testing.T) {
	e, a, w, target := encounterFixture(t)
	tick := func() profile.Result {
		return e.TickHook(context.Background(), profile.HookBossEngage, w, target, w.At)
	}
	if got := tick(); got.Status != profile.StatusAction || a.teleports != 1 || len(a.casts) != 0 {
		t.Fatalf("approach=%+v actions=%+v", got, a)
	}
	w.Player.Position.X = 117
	if got := tick(); got.Status != profile.StatusPending || len(a.casts) != 0 {
		t.Fatalf("stale landing=%+v", got)
	}
	w.At = w.At.Add(time.Second)
	a.pending = true
	if got := tick(); got.Status != profile.StatusPending || len(a.casts) != 0 {
		t.Fatalf("unconfirmed selection=%+v", got)
	}
	a.pending = false
	for i := 0; i < 3; i++ {
		if got := tick(); got.Status != profile.StatusAction {
			t.Fatalf("cast %d=%+v", i, got)
		}
		w.At = w.At.Add(time.Second)
	}
	if got := tick(); got.Status != profile.StatusComplete || len(a.casts) != 3 {
		t.Fatalf("completed=%+v casts=%v", got, a.casts)
	}
	target.ActionIndex = 1
	if got := tick(); got.Status != profile.StatusComplete || len(a.casts) != 3 {
		t.Fatalf("repeated hook=%+v casts=%v", got, a.casts)
	}
	e.Reset()
	target.ActionIndex = 0
	if got := tick(); got.Status != profile.StatusAction || len(a.casts) != 4 {
		t.Fatalf("reset=%+v casts=%v", got, a.casts)
	}
}

func TestStaticFieldGuardsAndTownPrebuff(t *testing.T) {
	t.Run("town", func(t *testing.T) {
		e, a, w, target := encounterFixture(t)
		got := e.TickHook(context.Background(), profile.HookTownReady, w, target, w.At)
		if got.Status != profile.StatusAction || len(a.casts) != 1 || a.casts[0] != memory.MustSkillID("frozen_armor") || a.teleports != 0 {
			t.Fatalf("town=%+v actions=%+v", got, a)
		}
	})
	for _, npc := range []uint32{world.DarkStalker, world.Summoner, world.Nihlathak, world.HellBovine} {
		e, a, w, target := encounterFixture(t)
		w.Monsters[0].NPCID = npc
		got := e.TickHook(context.Background(), profile.HookBossEngage, w, target, w.At)
		if got.Status != profile.StatusComplete || len(a.casts) != 0 || a.teleports != 0 {
			t.Fatalf("npc %d=%+v actions=%+v", npc, got, a)
		}
	}
	t.Run("timeout", func(t *testing.T) {
		e, a, w, target := encounterFixture(t)
		e.TickHook(context.Background(), profile.HookBossEngage, w, target, w.At)
		w.At = w.At.Add(20 * time.Second)
		got := e.TickHook(context.Background(), profile.HookBossEngage, w, target, w.At)
		if got.Status != profile.StatusFailed || a.teleports != 1 || len(a.casts) != 0 {
			t.Fatalf("timeout=%+v actions=%+v", got, a)
		}
	})
	t.Run("cancelled", func(t *testing.T) {
		e, a, w, target := encounterFixture(t)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		got := e.TickHook(ctx, profile.HookBossEngage, w, target, w.At)
		if got.Status != profile.StatusFailed || a.teleports != 0 {
			t.Fatalf("cancelled=%+v", got)
		}
	})
	t.Run("invalid snapshot", func(t *testing.T) {
		e, a, w, target := encounterFixture(t)
		w.Valid = false
		got := e.TickHook(context.Background(), profile.HookBossEngage, w, target, w.At)
		if got.Status != profile.StatusPending || a.teleports != 0 {
			t.Fatalf("invalid=%+v", got)
		}
	})
}

func TestAllRouteStrategiesUseBlizzardWithoutCurseOrCorpseExplosion(t *testing.T) {
	for _, run := range []string{"countess", "cows", "lower-kurast", "mephisto", "nihlathak", "summoner"} {
		e, a, w, _ := encounterFixture(t)
		s := NewFactory(run)()
		if _, ce := s.(profile.SupportsCorpseExplosion); ce {
			t.Fatal("Blizzard declares CE")
		}
		if s.(profile.SupportsRouteClear).RequiresRouteClear() != (run == "cows" || run == "summoner") {
			t.Fatalf("route clear %s", run)
		}
		if err := s.Configure(e.Executor, memory.MustSkillID("blizzard"), a); err != nil {
			t.Fatal(err)
		}
		got := e.TickRouteClear(context.Background(), profile.RouteClearRequest{RunID: run, DefinitionID: ID, Player: w.Player, Target: w.Monsters[0], Mode: profile.RouteClearThreat, AssessmentAt: w.At}, w.At)
		if got.Status != profile.StatusAction || got.SkillID != memory.MustSkillID("blizzard") || len(a.casts) != 1 {
			t.Fatalf("%s=%+v casts=%v", run, got, a.casts)
		}
	}
}
