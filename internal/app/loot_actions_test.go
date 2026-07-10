package app

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/loot"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/tasks"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/telemetry"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

type telemetryEmitterMock struct {
	events []telemetry.Event
	err    error
}

func (m *telemetryEmitterMock) Emit(event telemetry.Event) error {
	if m.err != nil {
		return m.err
	}
	m.events = append(m.events, event)
	return nil
}

func TestCountInventoryFullCandidatesCountsOnlyExplicitNoFit(t *testing.T) {
	report := loot.DecisionReport{Decisions: []loot.ItemDecision{
		{Stage: loot.DecisionStageFail, Kind: loot.DecisionKindFail, Reason: loot.DecisionReasonInventoryFull},
		{Stage: loot.DecisionStageFail, Kind: loot.DecisionKindFail, Reason: loot.DecisionReasonCapacityUnsafe},
		{Stage: loot.DecisionStagePickCandidate, Kind: loot.DecisionKindPickCandidate, Reason: loot.DecisionReasonPickitMatch},
	}}
	if got := countInventoryFullCandidates(report); got != 1 {
		t.Fatalf("countInventoryFullCandidates() = %d, want 1", got)
	}
}

func TestLootScanEmitsDropAndPickitTelemetry(t *testing.T) {
	dir := t.TempDir()
	pickitPath := filepath.Join(dir, "test.nip")
	if err := os.WriteFile(pickitPath, []byte("[type] == rune\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	pickit, err := loot.LoadPickit(pickitPath)
	if err != nil {
		t.Fatal(err)
	}
	lock, err := loot.NewInventoryLock([][]int{{0, 0, 0, 0, 0, 0, 0, 0, 0, 0}, {0, 0, 0, 0, 0, 0, 0, 0, 0, 0}, {0, 0, 0, 0, 0, 0, 0, 0, 0, 0}, {0, 0, 0, 0, 0, 0, 0, 0, 0, 0}})
	if err != nil {
		t.Fatal(err)
	}
	emitter := &telemetryEmitterMock{}
	adapter := &lootActionsAdapter{log: config.NewLogger("error"), filter: loot.NewFilter(config.NewLogger("error"), lock, pickit), skipped: map[uint32]bool{}, telemetry: emitter}
	state := world.State{At: time.Now(), Valid: true, Phase: world.GamePhaseInGame, Area: world.LookupArea(world.TowerCellarLevel5), Items: []world.Item{{UnitID: 7, TxtFileNo: 627, Code: "r03", Name: "Tir Rune", Type: "rune", Location: world.ItemLocationGround, Width: 1, Height: 1}}}
	result := adapter.Scan(state)
	if result.TelemetryFailed || len(emitter.events) != 2 || emitter.events[0].Event != telemetry.DropSeen || emitter.events[1].Event != telemetry.PickitMatch {
		t.Fatalf("result=%+v events=%+v", result, emitter.events)
	}
}

func TestTelemetryFailureIsStickyAndBlocksFollowingLootInput(t *testing.T) {
	emitter := &telemetryEmitterMock{err: errors.New("disk full")}
	adapter := &lootActionsAdapter{log: config.NewLogger("error"), filter: &loot.Filter{}, skipped: map[uint32]bool{}, telemetry: emitter}
	result := adapter.Scan(world.State{Items: []world.Item{{UnitID: 7, Location: world.ItemLocationGround}}})
	if !result.TelemetryFailed || adapter.telemetryErr == nil {
		t.Fatalf("result=%+v telemetryErr=%v", result, adapter.telemetryErr)
	}
	if err := adapter.StartPickup(tasks.LootTarget{UnitID: 7, AreaID: world.TowerCellarLevel5}); err == nil {
		t.Fatal("StartPickup() after telemetry failure error=nil")
	}
	stash := adapter.TickStash(world.State{}, time.Now())
	if stash.Status != tasks.LootStashTelemetryFailed || !stash.Done {
		t.Fatalf("TickStash()=%+v, want telemetry_failed", stash)
	}
}
