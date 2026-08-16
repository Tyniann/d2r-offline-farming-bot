package replay

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/pathing"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/profile"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/tasks"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/telemetry"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

func TestReplayReachesIdenticalTerminalFailure(t *testing.T) {
	bundle := captureBossFocusLossFixture(t)
	report, err := Replay(bundle)
	if err != nil {
		t.Fatalf("Replay() error = %v", err)
	}
	if report.Ticks != len(bundle.Frames) || report.Step != bundle.Terminal.Step || report.Outcome != bundle.Terminal.Outcome || report.Reason != bundle.Terminal.Reason {
		t.Fatalf("Replay() report = %+v, terminal = %+v", report, bundle.Terminal)
	}
}

func TestReplayReportsFirstDependencyOrderDivergence(t *testing.T) {
	bundle := captureBossFocusLossFixture(t)
	mutated := false
	for frameIndex := range bundle.Frames {
		deps := bundle.Frames[frameIndex].Dependencies
		for index := 0; index+1 < len(deps); index++ {
			if deps[index].Name != deps[index+1].Name {
				deps[index], deps[index+1] = deps[index+1], deps[index]
				mutated = true
				break
			}
		}
		if mutated {
			break
		}
	}
	if !mutated {
		t.Fatal("fixture has no frame with two dependency calls")
	}
	assertReplayDivergence(t, bundle, "dependency call order")
}

func TestReplayReportsFirstDependencyAnswerDivergence(t *testing.T) {
	bundle := captureBossFocusLossFixture(t)
	mutated := false
	for frameIndex := range bundle.Frames {
		for callIndex := range bundle.Frames[frameIndex].Dependencies {
			call := &bundle.Frames[frameIndex].Dependencies[callIndex]
			if call.Name == "combat.cast_world" || call.Name == "combat.cast_monster" {
				call.Error = ""
				call.Result["sent"] = true
				mutated = true
				break
			}
		}
	}
	if !mutated {
		t.Fatalf("fixture has no combat.cast_monster call: terminal=%+v dependencies=%v", bundle.Terminal, dependencyNames(bundle))
	}
	assertReplayDivergence(t, bundle, "")
}

func dependencyNames(bundle Bundle) []string {
	var names []string
	for _, frame := range bundle.Frames {
		for _, call := range frame.Dependencies {
			names = append(names, call.Name)
		}
	}
	return names
}

func TestReplayReportsFirstClockDivergence(t *testing.T) {
	bundle := captureWaypointSettleFixture(t)
	mutated := false
	for index := range bundle.Frames {
		if bundle.Frames[index].Before.Step == "select_run_waypoint" && !frameHasDependency(bundle.Frames[index], "waypoint.select") {
			for shifted := index; shifted < len(bundle.Frames); shifted++ {
				bundle.Frames[shifted].ElapsedNS += int64(time.Second)
			}
			mutated = true
			break
		}
	}
	if !mutated {
		t.Fatal("fixture has no waypoint settle frame")
	}
	assertReplayDivergence(t, bundle, "dependency call order")
}

func frameHasDependency(frame Frame, name string) bool {
	for _, call := range frame.Dependencies {
		if call.Name == name {
			return true
		}
	}
	return false
}

func assertReplayDivergence(t *testing.T, bundle Bundle, kind string) {
	t.Helper()
	_, err := Replay(bundle)
	var divergence *Divergence
	if !errors.As(err, &divergence) {
		t.Fatalf("Replay() error = %v, want Divergence", err)
	}
	if divergence.Tick == 0 || (kind != "" && divergence.Kind != kind) {
		t.Fatalf("divergence = %+v, want kind %q", divergence, kind)
	}
}

func captureBossFocusLossFixture(t *testing.T) Bundle {
	t.Helper()
	directory := t.TempDir()
	start := time.Date(2026, time.August, 14, 12, 0, 0, 0, time.UTC)
	contract := ContractSnapshot{
		RunID: "countess", Phase: tasks.RunPhaseBoss, ProfileID: "necro_bone_spear",
		Dependencies: []string{"combat", "telemetry"},
		Definition:   map[string]any{"fixture": "controlled_focus_loss"},
		Route:        map[string]any{"route_id": "countess-fixture"},
		Tuning: map[string]any{
			"step_timeout_ms": 10000, "loot_pickup_distance_tiles": 30,
			"attack_skill_id": 84, "attack_interval_ms": 1, "engage_distance_tiles": 8,
			"reposition_distance_tiles": 15, "kill_confirm_ticks": 3,
		},
	}
	recorder := newTestRecorder(t, directory, start, Config{Enabled: true, Label: "focus-loss", SaveSuccessful: true}, contract)
	combat := &focusLossCombatFixture{}
	deps := InstrumentDeps(tasks.Deps{Combat: combat, Telemetry: replayTelemetryFixture{}}, recorder)
	runner := tasks.NewRunner(slog.New(slog.NewTextHandler(io.Discard, nil)), tasks.RunSelection{Run: contract.RunID, Phase: contract.Phase}, tasks.RunConfig{StepTimeout: 10 * time.Second, Combat: tasks.CombatConfig{Profile: contract.ProfileID, AttackSkillID: 84, AttackInterval: time.Millisecond, EngageDistanceTiles: 8, RepositionDistanceTiles: 15, KillConfirmTicks: 3}}, deps)
	state := world.State{
		Phase: world.GamePhaseInGame, Valid: true, Area: world.LookupArea(world.TowerCellarLevel5),
		Player:   world.Player{Position: world.Position{X: 100, Y: 100}, HP: 1000, MaxHP: 1000, Mana: 500, MaxMana: 500},
		Monsters: []world.Monster{{NPCID: world.DarkStalker, UnitID: 431, Position: world.Position{X: 108, Y: 100}, MonsterTypeFlag: world.SuperUniqueMonsterFlag}},
	}
	for tick := 0; tick < 12; tick++ {
		now := start.Add(time.Duration(tick) * 50 * time.Millisecond)
		state.At = now
		state.Generation = uint64(tick + 1)
		before := traceStateFromResult(runner.Result())
		recorder.BeginTick(now, NormalizeWorld(state), state.Generation, RuntimeGates{InputEnabled: true, WindowBound: true}, before)
		result := runner.Tick(context.Background(), state, now)
		recorder.EndTick(traceStateFromResult(result))
		if runner.Terminal() {
			final, err := recorder.Finalize(Terminal{Step: result.Step, Outcome: string(result.Outcome), Reason: result.Reason})
			if err != nil {
				t.Fatalf("Finalize() error = %v", err)
			}
			bundle, err := ReadBundle(filepath.Join(directory, final.Filename), 1<<20)
			if err != nil {
				t.Fatalf("ReadBundle() error = %v", err)
			}
			return bundle
		}
	}
	t.Fatal("focus-loss fixture did not terminate")
	return Bundle{}
}

func captureWaypointSettleFixture(t *testing.T) Bundle {
	t.Helper()
	directory := t.TempDir()
	start := time.Date(2026, time.August, 14, 12, 30, 0, 0, time.UTC)
	contract := ContractSnapshot{
		RunID: "countess", Phase: tasks.RunPhaseTravelEntry, ProfileID: "necro_bone_spear",
		Dependencies: []string{"waypoint", "town_walk", "telemetry"},
		Definition:   map[string]any{"fixture": "waypoint_settle"},
		Tuning:       map[string]any{"step_timeout_ms": 10000},
	}
	recorder := newTestRecorder(t, directory, start, Config{Enabled: true, Label: "waypoint-settle", SaveSuccessful: true}, contract)
	waypoint := &waypointSettleFixture{}
	deps := InstrumentDeps(tasks.Deps{Waypoint: waypoint, TownWalk: waypoint, Telemetry: replayTelemetryFixture{}}, recorder)
	runner := tasks.NewRunner(slog.New(slog.NewTextHandler(io.Discard, nil)), tasks.RunSelection{Run: contract.RunID, Phase: contract.Phase}, tasks.RunConfig{StepTimeout: 10 * time.Second, Combat: tasks.CombatConfig{Profile: contract.ProfileID}}, deps)
	townState := world.State{Phase: world.GamePhaseInGame, Valid: true, Area: world.LookupArea(world.RogueEncampment), Player: world.Player{Position: world.Position{X: 100, Y: 100}, HP: 1000, MaxHP: 1000, Mana: 500, MaxMana: 500}, Objects: []world.Object{{Kind: world.ObjectKindWaypoint, ID: 119, UnitID: 50, Position: world.Position{X: 102, Y: 100}}}}
	times := []time.Duration{
		0, 50 * time.Millisecond, 100 * time.Millisecond, 200 * time.Millisecond,
		300 * time.Millisecond, 800 * time.Millisecond,
		time.Second, 2 * time.Second, 3 * time.Second, 4 * time.Second,
	}
	const blackMarshFrom = 6
	for index, elapsed := range times {
		now := start.Add(elapsed)
		state := townState
		if index >= blackMarshFrom {
			state.Area = world.LookupArea(world.BlackMarsh)
		}
		state.At = now
		state.Generation = uint64(index + 1)
		before := traceStateFromResult(runner.Result())
		recorder.BeginTick(now, NormalizeWorld(state), state.Generation, RuntimeGates{InputEnabled: true, WindowBound: true}, before)
		result := runner.Tick(context.Background(), state, now)
		recorder.EndTick(traceStateFromResult(result))
		if runner.Terminal() {
			final, err := recorder.Finalize(Terminal{Step: result.Step, Outcome: string(result.Outcome), Reason: result.Reason})
			if err != nil {
				t.Fatalf("Finalize() error = %v", err)
			}
			bundle, err := ReadBundle(filepath.Join(directory, final.Filename), 1<<20)
			if err != nil {
				t.Fatalf("ReadBundle() error = %v", err)
			}
			return bundle
		}
	}
	t.Fatal("waypoint fixture did not terminate")
	return Bundle{}
}

type waypointSettleFixture struct {
	openCalls int
}

func (*waypointSettleFixture) Reset() {}
func (*waypointSettleFixture) TickAct1Waypoint(context.Context, world.State) pathing.TownWalkResult {
	return pathing.TownWalkResult{Status: pathing.TownWalkWaypointVisible, Done: true}
}
func (f *waypointSettleFixture) TickTownWaypoint(context.Context, world.State) pathing.WaypointActionResult {
	f.openCalls++
	if f.openCalls == 1 {
		return pathing.WaypointActionResult{Status: pathing.WaypointActionPending}
	}
	return pathing.WaypointActionResult{Status: pathing.WaypointActionClicked, Done: true}
}
func (*waypointSettleFixture) SelectWaypointTarget(context.Context, world.State, pathing.WaypointTargetID, time.Time) pathing.WaypointActionResult {
	return pathing.WaypointActionResult{Status: pathing.WaypointActionClicked, Done: true}
}

type focusLossCombatFixture struct{}

type replayTelemetryFixture struct{}

func (replayTelemetryFixture) Emit(telemetry.Event) error { return nil }

func (*focusLossCombatFixture) CastAttackAtWorld(time.Time, uint16, world.Player, world.Position) (bool, error) {
	return false, errors.New("focus window: window not foreground")
}
func (*focusLossCombatFixture) HoldStandardAttack(time.Time, uint16, world.Player, world.Monster) (profile.MonsterCastResult, error) {
	return profile.MonsterCastResult{}, errors.New("focus window: window not foreground")
}
func (*focusLossCombatFixture) CastAttackAtMonster(time.Time, uint16, world.Player, world.Monster) (profile.MonsterCastResult, error) {
	return profile.MonsterCastResult{}, errors.New("focus window: window not foreground")
}
func (*focusLossCombatFixture) MonsterAimProjectable(world.Position, world.Position) bool {
	return true
}
func (*focusLossCombatFixture) FarthestProjectableMonsterApproach(_, target world.Position) (world.Position, float64, bool) {
	return target, 8, true
}
func (*focusLossCombatFixture) StopAttack() error { return nil }
func (*focusLossCombatFixture) TeleportToward(time.Time, world.Player, world.Position, float64) (bool, error) {
	return false, nil
}
func (*focusLossCombatFixture) ForceMoveToward(time.Time, world.Position, world.Position) (bool, error) {
	return false, nil
}
func (*focusLossCombatFixture) Reset() {}
