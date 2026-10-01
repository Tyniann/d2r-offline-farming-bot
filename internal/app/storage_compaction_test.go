package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/crafting"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/input"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/loot"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/telemetry"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

type compactionInput struct {
	buttons []input.MouseButton
	status  input.Status
}

func TestStorageCompactionFailuresNeverExitOrRetry(t *testing.T) {
	for _, reason := range []string{crafting.ReasonGemFull, crafting.ReasonRuneFull, crafting.ReasonUnavailable, crafting.ReasonCubeNotEmpty, crafting.ReasonUnconfirmed, crafting.ReasonTimeout} {
		calls := 0
		result, err := classifyFailedQueueRun(context.Background(), "cows", reason, []string{reason}, world.State{Valid: true, Phase: world.GamePhaseInGame, Area: world.LookupArea(world.RogueEncampment)}, func(context.Context) error { calls++; return nil })
		if err != nil || result.Disposition != QueueRunStop || result.ExitAuthorization != ExitAuthorizationNone || calls != 0 {
			t.Fatalf("reason=%s result=%+v err=%v", reason, result, err)
		}
	}
}

func (i *compactionInput) Window() (input.WindowInfo, bool) {
	return input.WindowInfo{ClientWidth: 1280, ClientHeight: 720}, true
}
func (i *compactionInput) Status() input.Status  { return i.status }
func (i *compactionInput) MoveTo(int, int) error { return nil }
func (i *compactionInput) ClickWithModifier(_ string, b input.MouseButton) error {
	i.buttons = append(i.buttons, b)
	return nil
}
func (i *compactionInput) PressKey(string) error { return nil }
func (i *compactionInput) ClickAt(_ int, _ int, b input.MouseButton) error {
	i.buttons = append(i.buttons, b)
	return nil
}
func (i *compactionInput) ClickAtWithCtrlShift(_ int, _ int, b input.MouseButton) error {
	i.buttons = append(i.buttons, b)
	return nil
}

func TestStorageCompactionAdapterKeepsPolicyAndPauseBudget(t *testing.T) {
	log := config.NewLogger("error")
	pickit, err := loot.CompilePickitRules("storage", []loot.PickitRuleSpec{{ProfileID: "runes", RuleID: "keep", Action: loot.ActionKeep, Expression: `[type] == "rune"`}})
	if err != nil {
		t.Fatal(err)
	}
	lock, err := loot.NewInventoryLock([][]int{{0, 0, 0, 0, 0, 0, 0, 0, 0, 0}, {0, 0, 0, 0, 0, 0, 0, 0, 0, 0}, {0, 0, 0, 0, 0, 0, 0, 0, 0, 0}, {0, 0, 0, 0, 0, 0, 0, 0, 0, 0}})
	if err != nil {
		t.Fatal(err)
	}
	controller := &compactionInput{status: input.Status{Enabled: true}}
	emitter := &telemetryEmitterMock{}
	filter := loot.NewFilter(log, lock, pickit)
	stash, err := loot.NewStashExecutor(log, filter, controller, loot.StashConfig{MaxRetries: 1, VerifyTimeout: 100 * time.Millisecond, CloseTimeout: time.Second, InventoryLeft: 847, InventoryTop: 369, InventoryCellW: 33, InventoryCellH: 33})
	if err != nil {
		t.Fatal(err)
	}
	looting := &lootActionsAdapter{log: log, filter: filter, stash: stash, telemetry: emitter}
	s := world.State{Valid: true, Phase: world.GamePhaseInGame, At: time.Unix(100, 0), Generation: 1, Area: world.LookupArea(world.RogueEncampment), UI: world.UIState{InventoryOpen: true, StashOpen: true}, Items: []world.Item{{UnitID: 1, Code: "box", Location: world.ItemLocationInventory, PlayerOwned: true, Width: 2, Height: 2}, {UnitID: 2, Code: "r03", Type: "rune", Location: world.ItemLocationInventory, PlayerOwned: true, Width: 1, Height: 1, GridX: 9, GridY: 2}}, Collection: world.CollectionState{ScopeID: "fixture", ScopeKnown: true, Generation: 1, ObservedAt: time.Unix(100, 0), TabKnown: true, Tab: world.StorageTabGems, Counts: []world.MaterialCount{{Code: "r03", Count: 99, Known: true}, {Code: "r04", Count: 98, Known: true}}}}
	fresh := func() {
		s.At = s.At.Add(100 * time.Millisecond)
		s.Generation++
		s.Collection.Generation = s.Generation
		s.Collection.ObservedAt = s.At
	}
	looting.TickStash(s, s.At)
	fresh()
	failure := looting.TickStash(s, s.At)
	if !failure.CompactionCandidate || failure.Reason != "verify_timeout" || failure.GridX != 9 || failure.GridY != 2 || looting.compactionScope != "fixture" {
		t.Fatalf("candidate=%+v", failure)
	}
	a, err := newStorageCompactionAdapter(log, controller, looting)
	if err != nil {
		t.Fatal(err)
	}
	r := crafting.Request{UnitID: failure.UnitID, Code: failure.Code, GridX: failure.GridX, GridY: failure.GridY, RunGeneration: 1}
	if result := a.Tick(s, s.At, r); result.Done {
		t.Fatal(result.Failure)
	}
	s.Collection.Tab = world.StorageTabRunes
	for tick := 0; tick < 4; tick++ {
		fresh()
		if result := a.Tick(s, s.At, r); result.Done {
			t.Fatal(result.Failure)
		}
	}
	if len(controller.buttons) != 3 || controller.buttons[2] != input.MouseRight {
		t.Fatalf("buttons=%v", controller.buttons)
	}
	before := len(controller.buttons)
	controller.status.Paused = true
	s.At = s.At.Add(time.Minute)
	a.observePause(s, s.At)
	controller.status.Paused = false
	fresh()
	if result := a.Tick(s, s.At, r); result.Done || len(controller.buttons) != before {
		t.Fatal("pause observer failed or replayed ingredient click")
	}
	s.Items[1].GridX = 8
	fresh()
	if result := a.Tick(s, s.At, r); !result.Done || result.Failure == nil || result.Failure.Reason != crafting.ReasonUnconfirmed || len(controller.buttons) != before {
		t.Fatal("changed policy binding authorized input")
	}
	a.Reset()
	s.Items[1].GridX = 9
	s.Collection.ScopeID = "other"
	if result := a.Tick(s, s.At, r); !result.Done || len(controller.buttons) != before {
		t.Fatal("new storage scope inherited old stash authorization")
	}
	s.Collection.ScopeID = "fixture"
	fresh()
	if result := a.Tick(s, s.At, r); !result.Done || len(controller.buttons) != before {
		t.Fatal("failed context resumed without Reset")
	}
	if emitter.events[1].Event != telemetry.StorageCompactionStarted || emitter.events[len(emitter.events)-1].Event != telemetry.StorageCompactionFailed {
		t.Fatalf("events=%+v", emitter.events)
	}
	a.Reset()
	emitter.err = errors.New("write failed")
	for tick := 0; tick < 2; tick++ {
		result := a.Tick(s, s.At, r)
		if !result.Done || result.Failure.Reason != "telemetry_failed" || len(controller.buttons) != before {
			t.Fatalf("telemetry result=%+v", result)
		}
	}
	a.Reset()
	looting.Reset()
	if a.request.UnitID != 0 || looting.compactionTrigger != nil || looting.compactionScope != "" || len(controller.buttons) != before {
		t.Fatal("reset retained binding or sent input")
	}
}
