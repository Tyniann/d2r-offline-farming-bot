package replay

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"reflect"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/tasks"
)

// ReplayReport summarizes a deterministic headless runtime replay.
type ReplayReport struct {
	Ticks   int    `json:"ticks"`
	Step    string `json:"step"`
	Outcome string `json:"outcome"`
	Reason  string `json:"reason,omitempty"`
}

// Divergence reports the first difference between the recorded run and the
// current productive task pipeline.
type Divergence struct {
	Tick     uint64
	Step     string
	Kind     string
	Expected any
	Actual   any
	AreaID   uint32
}

// Error returns a stable, operator-readable first-divergence report.
func (d *Divergence) Error() string {
	expected, _ := json.Marshal(d.Expected)
	actual, _ := json.Marshal(d.Actual)
	return fmt.Sprintf("runtime replay divergence at tick %d, step %s, area %d: %s: expected=%s actual=%s", d.Tick, d.Step, d.AreaID, d.Kind, expected, actual)
}

// ReplayFile reads and replays one bounded compressed trace without process,
// window, hotkey, Memory, or OS-input dependencies.
func ReplayFile(path string) (ReplayReport, error) {
	bundle, err := ReadBundle(path, defaultMaximumBundleBytes)
	if err != nil {
		return ReplayReport{}, err
	}
	return Replay(bundle)
}

// Replay executes the productive task runner against recorded World frames,
// a fake monotonic clock, and transcript-backed dependencies.
func Replay(bundle Bundle) (ReplayReport, error) {
	if err := bundle.Validate(); err != nil {
		return ReplayReport{}, err
	}
	if bundle.FramesTruncated {
		return ReplayReport{}, fmt.Errorf("runtime trace detailed frames were truncated and cannot be replayed from run start")
	}
	runConfig, err := replayRunConfig(bundle.Contract)
	if err != nil {
		return ReplayReport{}, err
	}
	transcript := &replayDependencies{}
	observer, err := NewRecorder(Config{Enabled: true, Directory: ".", Label: "headless-replay", MaximumFrames: len(bundle.Frames) + 1, SaveSuccessful: true, Now: func() time.Time { return time.Unix(0, 0) }}, bundle.Metadata, bundle.Contract)
	if err != nil {
		return ReplayReport{}, err
	}
	transcriptDeps, err := transcript.taskDeps(bundle.Contract.Dependencies)
	if err != nil {
		return ReplayReport{}, err
	}
	deps := InstrumentDeps(transcriptDeps, observer)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	runner := tasks.NewRunner(logger, tasks.RunSelection{Run: bundle.Contract.RunID, Phase: bundle.Contract.Phase}, runConfig, deps)
	clockBase := time.Unix(0, 0)

	for _, frame := range bundle.Frames {
		before := traceStateFromResult(runner.Result())
		if !reflect.DeepEqual(before, frame.Before) {
			return ReplayReport{}, divergence(frame, "state before tick", frame.Before, before)
		}
		transcript.beginFrame(frame)
		now := clockBase.Add(time.Duration(frame.ElapsedNS))
		state := worldStateFromFrame(frame, now)
		observer.BeginTick(now, frame.World, frame.Generation, frame.Gates, before)
		result := runner.Tick(context.Background(), state, now)
		after := traceStateFromResult(result)
		observer.EndTick(after)
		if err := transcript.endFrame(); err != nil {
			return ReplayReport{}, divergence(frame, "dependency call order", transcript.expectedCall(), err.Error())
		}
		actualFrame, ok := observer.LastFrame()
		if !ok {
			return ReplayReport{}, divergence(frame, "observer frame", "recorded frame", "missing")
		}
		if !canonicalEqual(frame.Dependencies, actualFrame.Dependencies) {
			return ReplayReport{}, divergence(frame, "dependency result", frame.Dependencies, actualFrame.Dependencies)
		}
		if !canonicalEqual(frame.Intents, actualFrame.Intents) {
			return ReplayReport{}, divergence(frame, "input intent", frame.Intents, actualFrame.Intents)
		}
		if !reflect.DeepEqual(after, frame.After) {
			return ReplayReport{}, divergence(frame, "state after tick", frame.After, after)
		}
	}

	result := runner.Result()
	actualTerminal := Terminal{Step: result.Step, Outcome: string(result.Outcome), Reason: result.Reason}
	if !reflect.DeepEqual(actualTerminal, bundle.Terminal) {
		last := bundle.Frames[len(bundle.Frames)-1]
		return ReplayReport{}, divergence(last, "terminal result", bundle.Terminal, actualTerminal)
	}
	return ReplayReport{Ticks: len(bundle.Frames), Step: actualTerminal.Step, Outcome: actualTerminal.Outcome, Reason: actualTerminal.Reason}, nil
}

func traceStateFromResult(result tasks.TickResult) TickState {
	return TickState{Step: result.Step, Outcome: string(result.Outcome), Reason: result.Reason, Active: result.Active}
}

func divergence(frame Frame, kind string, expected, actual any) *Divergence {
	step := frame.Before.Step
	if step == "" {
		step = frame.After.Step
	}
	return &Divergence{Tick: frame.Tick, Step: step, Kind: kind, Expected: expected, Actual: actual, AreaID: frame.World.AreaID}
}

func canonicalEqual(left, right any) bool {
	leftJSON, leftErr := json.Marshal(left)
	rightJSON, rightErr := json.Marshal(right)
	return leftErr == nil && rightErr == nil && string(leftJSON) == string(rightJSON)
}

func replayRunConfig(contract ContractSnapshot) (tasks.RunConfig, error) {
	if contract.RunID == "" {
		return tasks.RunConfig{}, fmt.Errorf("runtime replay run_id is required")
	}
	config := tasks.RunConfig{
		RouteID:                 stringValue(contract.Route, "route_id"),
		SetupRouteID:            stringValue(contract.Route, "setup_route_id"),
		StepTimeout:             time.Duration(int64Value(contract.Tuning, "step_timeout_ms")) * time.Millisecond,
		LootPickupDistanceTiles: float64Value(contract.Tuning, "loot_pickup_distance_tiles"),
		Combat: tasks.CombatConfig{
			Profile:                 contract.ProfileID,
			AttackSkillID:           uint16(uint64Value(contract.Tuning, "attack_skill_id")),
			AttackInterval:          time.Duration(int64Value(contract.Tuning, "attack_interval_ms")) * time.Millisecond,
			EngageDistanceTiles:     float64Value(contract.Tuning, "engage_distance_tiles"),
			RepositionDistanceTiles: float64Value(contract.Tuning, "reposition_distance_tiles"),
			KillConfirmTicks:        int(int64Value(contract.Tuning, "kill_confirm_ticks")),
		},
		RouteCombat: tasks.RouteCombatConfig{
			Enabled:                    boolValue(contract.Tuning, "route_combat_enabled"),
			ImmediateRadiusTiles:       float64Value(contract.Tuning, "route_immediate_radius_tiles"),
			CorridorWidthTiles:         float64Value(contract.Tuning, "route_corridor_width_tiles"),
			LandingRadiusTiles:         float64Value(contract.Tuning, "route_landing_radius_tiles"),
			AttackDistanceTiles:        float64Value(contract.Tuning, "route_attack_distance_tiles"),
			NoProgressTimeout:          time.Duration(int64Value(contract.Tuning, "route_no_progress_timeout_ms")) * time.Millisecond,
			TeleportManaReservePercent: int(int64Value(contract.Tuning, "teleport_mana_reserve_percent")),
			ResumeManaPercent:          int(int64Value(contract.Tuning, "route_resume_mana_percent")),
			EmergencyManaPercent:       int(int64Value(contract.Tuning, "route_emergency_mana_percent")),
			ManaRecoveryTimeout:        time.Duration(int64Value(contract.Tuning, "route_mana_recovery_timeout_ms")) * time.Millisecond,
		},
	}
	return config, nil
}

func valueAt(values map[string]any, key string) any {
	if values == nil {
		return nil
	}
	return values[key]
}
func stringValue(values map[string]any, key string) string {
	value, _ := valueAt(values, key).(string)
	return value
}
func boolValue(values map[string]any, key string) bool {
	value, _ := valueAt(values, key).(bool)
	return value
}
func float64Value(values map[string]any, key string) float64 {
	switch value := valueAt(values, key).(type) {
	case float64:
		return value
	case float32:
		return float64(value)
	case int:
		return float64(value)
	case int64:
		return float64(value)
	case uint64:
		return float64(value)
	default:
		return 0
	}
}
func int64Value(values map[string]any, key string) int64   { return int64(float64Value(values, key)) }
func uint64Value(values map[string]any, key string) uint64 { return uint64(float64Value(values, key)) }
