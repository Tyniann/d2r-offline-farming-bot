package replay

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/pathing"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/profile"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/tasks"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

func TestReplayPortalPreservesTargetAndBlockerEvidence(t *testing.T) {
	deps := &replayDependencies{}
	deps.beginFrame(Frame{Dependencies: []DependencyCall{{
		Name: "portal.tick",
		Result: map[string]any{
			"status": "hover_not_found", "reason": "hover_not_found", "done": true,
			"portal_unit_id": float64(77), "blocker_unit_id": float64(99),
		},
	}}})

	result := deps.TickPortal(context.Background(), world.State{}, time.Time{})
	if result.Status != pathing.TownPortalActionHoverNotFound || result.PortalUnitID != 77 || result.BlockerUnitID != 99 {
		t.Fatalf("result = %+v, want hover_not_found with portal 77 and blocker 99", result)
	}
	if err := deps.endFrame(); err != nil {
		t.Fatal(err)
	}
}

func TestInstrumentDepsForwardsOnceAndOnlyObservesIntent(t *testing.T) {
	directory := t.TempDir()
	now := time.Date(2026, time.August, 14, 11, 0, 0, 0, time.UTC)
	recorder := newTestRecorder(t, directory, now, Config{Enabled: true, Label: "dependency", SaveSuccessful: true}, ContractSnapshot{RunID: "mephisto", Definition: map[string]any{}})
	fixture := &combatDependencyFixture{teleportSent: true}
	deps := InstrumentDeps(tasks.Deps{Combat: fixture}, recorder)
	recorder.BeginTick(now, WorldFrame{Phase: "in_game", Valid: true}, 3, RuntimeGates{InputEnabled: true, WindowBound: true}, TickState{Step: "engage_boss", Outcome: "running", Active: true})

	sent, err := deps.Combat.TeleportToward(now, world.Player{Position: world.Position{X: 10, Y: 20}}, world.Position{X: 30, Y: 40}, 8)
	if err != nil || !sent {
		t.Fatalf("TeleportToward() = (%t, %v), want (true, nil)", sent, err)
	}
	recorder.EndTick(TickState{Step: "engage_boss", Outcome: "failure", Reason: "focus_lost"})
	result, err := recorder.Finalize(Terminal{Step: "engage_boss", Outcome: "failure", Reason: "focus_lost"})
	if err != nil {
		t.Fatalf("Finalize() error = %v", err)
	}
	if fixture.teleportCalls != 1 {
		t.Fatalf("underlying dependency calls = %d, want exactly 1", fixture.teleportCalls)
	}
	bundle, err := ReadBundle(filepath.Join(directory, result.Filename), 1<<20)
	if err != nil {
		t.Fatalf("ReadBundle() error = %v", err)
	}
	frame := bundle.Frames[0]
	if len(frame.Dependencies) != 1 || frame.Dependencies[0].Name != "combat.teleport" {
		t.Fatalf("dependencies = %+v", frame.Dependencies)
	}
	if len(frame.Intents) != 1 || frame.Intents[0].Name != "teleport" || frame.Intents[0].Outcome != "sent" {
		t.Fatalf("intents = %+v", frame.Intents)
	}
}

type combatDependencyFixture struct {
	teleportSent  bool
	teleportCalls int
}

func (f *combatDependencyFixture) CastAttackAtWorld(time.Time, uint16, world.Player, world.Position) (bool, error) {
	return false, nil
}
func (f *combatDependencyFixture) HoldStandardAttack(time.Time, uint16, world.Player, world.Monster) (profile.MonsterCastResult, error) {
	return profile.MonsterCastResult{}, nil
}
func (f *combatDependencyFixture) CastAttackAtMonster(time.Time, uint16, world.Player, world.Monster) (profile.MonsterCastResult, error) {
	return profile.MonsterCastResult{}, nil
}
func (f *combatDependencyFixture) MonsterAimProjectable(world.Position, world.Position) bool {
	return true
}
func (f *combatDependencyFixture) FarthestProjectableMonsterApproach(_, target world.Position) (world.Position, float64, bool) {
	return target, 0, true
}
func (f *combatDependencyFixture) StopAttack() error { return nil }
func (f *combatDependencyFixture) TeleportToward(time.Time, world.Player, world.Position, float64) (bool, error) {
	f.teleportCalls++
	return f.teleportSent, nil
}
func (f *combatDependencyFixture) ForceMoveToward(time.Time, world.Position, world.Position) (bool, error) {
	return false, nil
}
func (f *combatDependencyFixture) Reset() {}
