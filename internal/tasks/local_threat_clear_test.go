package tasks

import (
	"context"
	"testing"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/profile"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

func localClearState(at time.Time, monsters ...world.Monster) world.State {
	return world.State{
		At: at, Valid: true, Phase: world.GamePhaseInGame,
		Player:   world.Player{Position: world.Position{X: 100, Y: 100}},
		Monsters: monsters,
	}
}

func TestLocalThreatClearReportsExhaustedWhenActionBudgetEndsWithThreat(t *testing.T) {
	now := time.Now()
	clear := &localThreatClear{}
	clear.start(world.Position{X: 100, Y: 100}, 7, now)
	executor := &routeClearMock{result: profile.Result{Status: profile.StatusAction}}
	state := localClearState(now, world.Monster{UnitID: 7, Position: world.Position{X: 101, Y: 100}})

	for tick := 0; tick < localThreatClearMaxActions; tick++ {
		state.At = now.Add(time.Duration(tick) * time.Millisecond)
		if result := clear.tick(context.Background(), executor, state, state.At, "countess", "necro_bone_spear"); result.outcome != localThreatClearPending {
			t.Fatalf("tick %d outcome = %q, want pending", tick, result.outcome)
		}
	}
	result := clear.tick(context.Background(), executor, state, now.Add(time.Second), "countess", "necro_bone_spear")
	if result.outcome != localThreatClearExhausted {
		t.Fatalf("outcome = %q, want exhausted", result.outcome)
	}
}

func TestLocalThreatClearRequiresCompleteCoverageForCleared(t *testing.T) {
	now := time.Now()
	clear := &localThreatClear{}
	clear.start(world.Position{X: 100, Y: 100}, 0, now)
	executor := &routeClearMock{}

	for tick := 0; tick < localThreatClearStableSnapshots; tick++ {
		state := localClearState(now.Add(time.Duration(tick) * time.Millisecond))
		state.MonsterCoverage = world.MonsterCoverage{MonstersTruncated: true, MonsterCoverageRadiusTiles: localThreatClearRadiusTiles - 1}
		if result := clear.tick(context.Background(), executor, state, state.At, "countess", "necro_bone_spear"); result.outcome != localThreatClearPending {
			t.Fatalf("incomplete tick %d outcome = %q, want pending", tick, result.outcome)
		}
	}

	for tick := 0; tick < localThreatClearStableSnapshots; tick++ {
		state := localClearState(now.Add(time.Duration(tick+10) * time.Millisecond))
		state.MonsterCoverage = world.MonsterCoverage{MonstersTruncated: true, MonsterCoverageRadiusTiles: localThreatClearRadiusTiles}
		result := clear.tick(context.Background(), executor, state, state.At, "countess", "necro_bone_spear")
		want := localThreatClearPending
		if tick == localThreatClearStableSnapshots-1 {
			want = localThreatClearCleared
		}
		if result.outcome != want {
			t.Fatalf("complete tick %d outcome = %q, want %q", tick, result.outcome, want)
		}
	}
}

func TestLocalThreatClearReportsStableFailureReason(t *testing.T) {
	now := time.Now()
	clear := &localThreatClear{}
	clear.start(world.Position{X: 100, Y: 100}, 7, now)
	executor := &routeClearMock{result: profile.Result{Status: profile.StatusFailed, Reason: "profile_input_failed"}}
	state := localClearState(now, world.Monster{UnitID: 7, Position: world.Position{X: 101, Y: 100}})

	result := clear.tick(context.Background(), executor, state, now, "countess", "necro_bone_spear")
	if result.outcome != localThreatClearFailed || result.reason != "profile_input_failed" {
		t.Fatalf("result = %+v, want failed/profile_input_failed", result)
	}
}
