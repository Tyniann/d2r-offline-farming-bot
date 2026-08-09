package profile

import (
	"context"
	"testing"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

func configuredCorpseExplosionExecutor(t *testing.T, actions *actionMock) *Executor {
	t.Helper()
	definition := testDefinition()
	definition.ID = "necro_bone_spear"
	executor, err := NewExecutor(config.NewLogger("error"), definition, actions)
	if err != nil {
		t.Fatal(err)
	}
	if err := executor.ConfigureCorpseExplosion(74); err != nil {
		t.Fatal(err)
	}
	return executor
}

func currentCorpseState(at time.Time) world.State {
	return world.State{
		At: at, Generation: 7, Valid: true, Phase: world.GamePhaseInGame,
		Player:             world.Player{Position: world.Position{X: 100, Y: 100}},
		CowCorpsesComplete: true,
		CowCorpses: []world.CowCorpse{{
			NPCID: world.HellBovine, UnitID: 42, Position: world.Position{X: 104, Y: 103},
			ObservedAt: at, SnapshotGeneration: 7, ConsumptionKnown: true,
		}},
	}
}

func TestAuthorizedCorpseExplosionRequiresCurrentConcreteUnitID(t *testing.T) {
	actions := &actionMock{}
	executor := configuredCorpseExplosionExecutor(t, actions)
	now := time.Now().Add(2 * time.Second)
	state := currentCorpseState(now)

	for _, unitID := range []uint32{0, 41} {
		if got := executor.TickAuthorizedCorpseExplosion(context.Background(), state, unitID, now); got.Status != StatusComplete || got.Reason != CorpseExplosionReasonTargetUnavailable {
			t.Fatalf("unit %d result=%+v", unitID, got)
		}
	}
	stale := state
	stale.CowCorpses = append([]world.CowCorpse(nil), state.CowCorpses...)
	stale.CowCorpses[0].SnapshotGeneration--
	if got := executor.TickAuthorizedCorpseExplosion(context.Background(), stale, 42, now); got.Status != StatusComplete || got.Reason != CorpseExplosionReasonTargetUnavailable {
		t.Fatalf("stale result=%+v", got)
	}
	unavailable := state
	unavailable.CowCorpsesComplete = false
	if got := executor.TickAuthorizedCorpseExplosion(context.Background(), unavailable, 42, now); got.Status != StatusPending || got.Reason != CorpseExplosionReasonSnapshotUnavailable {
		t.Fatalf("unavailable result=%+v", got)
	}
	if len(actions.skills) != 0 {
		t.Fatalf("unauthorized inputs=%v", actions.skills)
	}
}

func TestAuthorizedCorpseExplosionCastsOnceThenRequiresSettleAndFreshSnapshot(t *testing.T) {
	actions := &actionMock{}
	executor := configuredCorpseExplosionExecutor(t, actions)
	now := time.Now().Add(2 * time.Second)
	state := currentCorpseState(now)
	if got := executor.TickAuthorizedCorpseExplosion(context.Background(), state, 42, now); got.Status != StatusAction || got.SkillID != 74 {
		t.Fatalf("cast result=%+v", got)
	}
	if len(actions.skills) != 1 || actions.skills[0] != 74 || actions.targets[0] != state.CowCorpses[0].Position {
		t.Fatalf("skills=%v targets=%v", actions.skills, actions.targets)
	}

	earlyFresh := state
	earlyFresh.At = now.Add(500 * time.Millisecond)
	earlyFresh.Generation++
	if got := executor.TickAuthorizedCorpseExplosion(context.Background(), earlyFresh, 42, earlyFresh.At); got.Status != StatusPending || got.Reason != CorpseExplosionReasonSettling {
		t.Fatalf("early result=%+v", got)
	}
	boundaryEarly := state
	boundaryEarly.At = now.Add(899 * time.Millisecond)
	boundaryEarly.Generation++
	if got := executor.TickAuthorizedCorpseExplosion(context.Background(), boundaryEarly, 42, boundaryEarly.At); got.Status != StatusPending || got.Reason != CorpseExplosionReasonSettling {
		t.Fatalf("899ms result=%+v", got)
	}
	sameGeneration := state
	if got := executor.TickAuthorizedCorpseExplosion(context.Background(), sameGeneration, 42, now.Add(1100*time.Millisecond)); got.Status != StatusPending {
		t.Fatalf("same generation result=%+v", got)
	}
	fresh := state
	fresh.At = now.Add(900 * time.Millisecond)
	fresh.Generation++
	fresh.CowCorpses = []world.CowCorpse{}
	if got := executor.TickAuthorizedCorpseExplosion(context.Background(), fresh, 42, fresh.At); got.Status != StatusComplete || got.Reason != CorpseExplosionReasonSettled {
		t.Fatalf("settled result=%+v", got)
	}
	if len(actions.skills) != 1 {
		t.Fatalf("back-to-back inputs=%v", actions.skills)
	}
}

func TestAuthorizedCorpseExplosionRejectsConsumedOrInconsistentCorpse(t *testing.T) {
	for _, mutate := range []func(*world.State){
		func(state *world.State) { state.CowCorpses[0].Consumed = true },
		func(state *world.State) { state.CowCorpses[0].ConsumptionKnown = false },
	} {
		actions := &actionMock{}
		executor := configuredCorpseExplosionExecutor(t, actions)
		now := time.Now().Add(2 * time.Second)
		state := currentCorpseState(now)
		mutate(&state)
		if got := executor.TickAuthorizedCorpseExplosion(context.Background(), state, 42, now); got.Status != StatusComplete || got.Reason != CorpseExplosionReasonTargetUnavailable {
			t.Fatalf("result=%+v", got)
		}
		if len(actions.skills) != 0 {
			t.Fatalf("inputs=%v", actions.skills)
		}
	}
}

func TestAuthorizedCorpseExplosionReportsUnprojectableCorpseWithoutPendingCast(t *testing.T) {
	actions := &actionMock{castErr: ErrCorpseExplosionTargetUnprojectable}
	executor := configuredCorpseExplosionExecutor(t, actions)
	now := time.Now().Add(2 * time.Second)
	state := currentCorpseState(now)
	result := executor.TickAuthorizedCorpseExplosion(context.Background(), state, 42, now)
	if result.Status != StatusComplete || result.Reason != CorpseExplosionReasonTargetUnprojectable || result.TargetUnitID != 42 {
		t.Fatalf("result=%+v", result)
	}
	actions.castErr = nil
	if retry := executor.TickAuthorizedCorpseExplosion(context.Background(), state, 42, now.Add(time.Second)); retry.Status != StatusAction {
		t.Fatalf("projection miss left a pending cast: %+v", retry)
	}
}
