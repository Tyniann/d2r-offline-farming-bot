package app

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/replay"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/tasks"
)

func TestRuntimeTraceReplaysMercenaryHoldAndAbort(t *testing.T) {
	rt := testRuntimeWithInput(&mockProcess{}, &mockProbe{}, &mockInput{enabled: true, bound: true}, Options{})
	rt.World.Update(validSnapshot(100))
	rt.runConfig = tasks.RunConfig{Cow: tasks.CowConfig{Character: "Fixture", ExpectedClassKnown: true, Difficulty: "hell", HasTownServices: true}, Combat: tasks.CombatConfig{UseCorpseExplosion: true}}
	selection := tasks.RunSelection{Run: "cows"}
	contract := runtimeTraceContract(rt.Config, rt.Options, selection, rt.runConfig)
	contract.Dependencies = []string{"input", "combat"}
	dir := t.TempDir()
	recorder, err := replay.NewRecorder(replay.Config{Enabled: true, Directory: dir, Label: "mercenary-hold"}, replay.Metadata{}, contract)
	if err != nil {
		t.Fatal(err)
	}
	combat := &traceStopCombat{}
	rt.RuntimeTrace = recorder
	rt.taskDeps = replay.InstrumentDeps(tasks.Deps{Input: rt.Input, Combat: combat}, recorder)
	rt.Tasks = tasks.NewRunner(rt.Log, selection, rt.runConfig, rt.taskDeps)
	state := rt.World.Current()
	now := time.Now()
	recorder.BeginTick(now, replay.NormalizeWorld(state), state.Generation, replay.RuntimeGates{}, traceTickState(rt.Tasks.Result()))
	result := rt.Tasks.Tick(context.Background(), state, now)
	recorder.EndTick(traceTickState(result))
	rt.productiveRunActive = true
	for _, decision := range []mercenaryDeathDecision{mercenaryDeathPending, mercenaryDeathConfirmed} {
		if err := rt.holdRunForMercenaryDeath(mercenaryDeathObservation{Decision: decision}); err != nil {
			t.Fatal(err)
		}
	}
	if combat.stops != 2 || rt.Tasks.Result().Reason != reasonMercenaryDiedDuringRun {
		t.Fatalf("stops=%d result=%+v", combat.stops, rt.Tasks.Result())
	}
	result = rt.Tasks.Result()
	final, err := recorder.Finalize(replay.Terminal{Step: result.Step, Outcome: string(result.Outcome), Reason: result.Reason})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := replay.ReplayFile(filepath.Join(dir, final.Filename)); err != nil {
		t.Fatal(err)
	}
}

type traceStopCombat struct {
	tasks.CombatActions
	stops int
}

func (c *traceStopCombat) StopAttack() error { c.stops++; return nil }
func (c *traceStopCombat) Reset()            {}
