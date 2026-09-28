package replay

import (
	"context"
	"io"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/input"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/tasks"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

func TestReplayCowPreflightAndExternalAbort(t *testing.T) {
	for _, abort := range []bool{false, true} {
		t.Run(map[bool]string{false: "preflight", true: "operator-stop"}[abort], func(t *testing.T) {
			start := time.Now()
			cfg := tasks.RunConfig{RouteID: "sweep", SetupRouteID: "leg", Cow: tasks.CowConfig{
				Character: "Fixture", ExpectedClassKnown: true, Difficulty: "hell", ClientWidth: 1280, ClientHeight: 720, HasTownServices: true,
			}}
			contract := ContractSnapshot{RunID: "cows", Dependencies: []string{"input", "telemetry"}, Policy: map[string]any{"run_config": cfg}}
			dir := t.TempDir()
			r := newTestRecorder(t, dir, start, Config{Enabled: true, Label: "cow-contract", SaveSuccessful: true}, contract)
			deps := InstrumentDeps(tasks.Deps{Input: cowTraceInput{}, Telemetry: replayTelemetryFixture{}}, r)
			runner := tasks.NewRunner(slog.New(slog.NewTextHandler(io.Discard, nil)), tasks.RunSelection{Run: "cows"}, cfg, deps)
			state := world.State{Valid: true, Phase: world.GamePhaseInGame, Area: world.LookupArea(world.RogueEncampment)}
			state.Identity.Valid = true
			state.Identity.CharacterName = "Fixture"
			for i := 0; i < 3; i++ {
				now := start.Add(time.Duration(i) * time.Second)
				state.At, state.Generation = now, uint64(i+1)
				r.BeginTick(now, NormalizeWorld(state), state.Generation, RuntimeGates{}, traceStateFromResult(runner.Result()))
				var tick tasks.TickResult
				if abort && i == 1 {
					r.RecordDependency("runtime.control", map[string]any{"reason": "operator_stop", "stop_attack": false}, nil, nil)
					if err := runner.AbortOpenStep("operator_stop"); err != nil {
						t.Fatal(err)
					}
					tick = runner.Result()
				} else {
					tick = runner.Tick(context.Background(), state, now)
				}
				r.EndTick(traceStateFromResult(tick))
				if runner.Terminal() {
					break
				}
			}
			result := runner.Result()
			want := tasks.CowReasonCubeMissing
			if abort {
				want = "operator_stop"
			}
			if result.Reason != want {
				t.Fatalf("fixture reason = %q, want %q", result.Reason, want)
			}
			final, err := r.Finalize(Terminal{Step: result.Step, Outcome: string(result.Outcome), Reason: result.Reason})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := ReplayFile(filepath.Join(dir, final.Filename)); err != nil {
				t.Fatal(err)
			}
		})
	}
}

type cowTraceInput struct{}

func (cowTraceInput) Status() input.Status { return input.Status{Enabled: true} }
func (cowTraceInput) Bound() bool          { return true }
func (cowTraceInput) Window() (input.WindowInfo, bool) {
	return input.WindowInfo{ClientWidth: 1280, ClientHeight: 720}, true
}
