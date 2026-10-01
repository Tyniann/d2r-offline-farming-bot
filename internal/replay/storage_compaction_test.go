package replay

import (
	"context"
	"io"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/crafting"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/pathing"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/tasks"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

func TestStorageCompactionTaskTraceResumesOriginalStash(t *testing.T) {
	start := time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC)
	cfg := tasks.RunConfig{StepTimeout: 10 * time.Second}
	contract := ContractSnapshot{RunID: "countess", Phase: tasks.RunPhaseStashPersonal, Dependencies: []string{"input", "stash", "loot", "compaction", "telemetry"}, Policy: map[string]any{"run_config": cfg}}
	dir := t.TempDir()
	recorder := newTestRecorder(t, dir, start, Config{Enabled: true, Label: "compaction", SaveSuccessful: true}, contract)
	state := world.State{Valid: true, Phase: world.GamePhaseInGame, Area: world.LookupArea(world.RogueEncampment), UI: world.UIState{StashOpen: true, InventoryOpen: true}, Collection: world.CollectionState{ScopeID: "fixture", ScopeKnown: true, Tab: world.StorageTabGems, TabKnown: true, Counts: []world.MaterialCount{{Code: "glr", Count: 99, Known: true}, {Code: "gpr", Count: 5, Known: true}}}}
	loot := &compactionTraceLoot{}
	compact := &compactionTraceFixture{}
	deps := InstrumentDeps(tasks.Deps{Input: cowTraceInput{}, Stash: compactionTraceStash{}, Loot: loot, Compaction: compact, Telemetry: replayTelemetryFixture{}}, recorder)
	runner := tasks.NewRunner(slog.New(slog.NewTextHandler(io.Discard, nil)), tasks.RunSelection{Run: contract.RunID, Phase: contract.Phase}, cfg, deps)
	for tick := 0; tick < 20; tick++ {
		now := start.Add(time.Duration(tick) * 100 * time.Millisecond)
		state.At, state.Generation = now, uint64(tick+1)
		state.Collection.ObservedAt, state.Collection.Generation = now, state.Generation
		if compact.calls >= 2 {
			state.Collection.Counts[0].Count = 96
		}
		recorder.BeginTick(now, NormalizeWorld(state), state.Generation, RuntimeGates{InputEnabled: true, WindowBound: true}, traceStateFromResult(runner.Result()))
		result := runner.Tick(context.Background(), state, now)
		recorder.EndTick(traceStateFromResult(result))
		if runner.Terminal() {
			if result.Outcome != tasks.RunOutcomeSuccess || compact.calls != 2 || loot.stashCalls != 3 {
				t.Fatalf("result=%+v compaction=%d stash=%d", result, compact.calls, loot.stashCalls)
			}
			final, err := recorder.Finalize(Terminal{Step: result.Step, Outcome: string(result.Outcome), Reason: result.Reason})
			if err != nil {
				t.Fatal(err)
			}
			if _, err = ReplayFile(filepath.Join(dir, final.Filename)); err != nil {
				t.Fatal(err)
			}
			return
		}
	}
	t.Fatal("task trace did not terminate")
}

type compactionTraceFixture struct{ calls int }

func (f *compactionTraceFixture) Tick(world.State, time.Time, crafting.Request) crafting.Result {
	f.calls++
	return crafting.Result{Done: f.calls == 2, Success: f.calls == 2}
}
func (*compactionTraceFixture) Reset() {}

type compactionTraceStash struct{}

func (compactionTraceStash) Tick(context.Context, world.State) pathing.PersonalStashResult {
	return pathing.PersonalStashResult{Status: pathing.PersonalStashOpened, Done: true}
}
func (compactionTraceStash) Reset() {}

type compactionTraceLoot struct {
	replayNoLootFixture
	stashCalls int
}

func (f *compactionTraceLoot) TickStash(world.State, time.Time) tasks.LootStashResult {
	f.stashCalls++
	if f.stashCalls == 1 {
		return tasks.LootStashResult{Status: tasks.LootStashFailed, Reason: "verify_timeout", Done: true, UnitID: 101, Code: "glr", GridX: 9, GridY: 2, CompactionCandidate: true}
	}
	return tasks.LootStashResult{Status: tasks.LootStashSuccess, Done: f.stashCalls == 3, Transferred: f.stashCalls == 2, UnitID: 101, Code: "glr"}
}
func (*compactionTraceLoot) TickCloseStash(world.State, time.Time) tasks.LootStashResult {
	return tasks.LootStashResult{Status: tasks.LootStashClosed, Done: true}
}
