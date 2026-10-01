package tasks

import (
	"context"
	"testing"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/crafting"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/telemetry"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

type taskCompactionMock struct {
	calls, resets int
	request       crafting.Request
	failure       *crafting.Failure
	pending       bool
}

func (m *taskCompactionMock) Tick(_ world.State, _ time.Time, r crafting.Request) crafting.Result {
	m.calls++
	m.request = r
	if m.failure != nil {
		return crafting.Result{Done: true, Failure: m.failure}
	}
	return crafting.Result{Done: !m.pending, Success: !m.pending}
}
func (m *taskCompactionMock) Reset() { m.resets++ }
func fullStashState() world.State {
	s := world.State{Valid: true, Phase: world.GamePhaseInGame, At: time.Unix(100, 0), Generation: 1, Area: world.LookupArea(world.RogueEncampment), UI: world.UIState{StashOpen: true, InventoryOpen: true}, Collection: world.CollectionState{ScopeID: "test", ScopeKnown: true, Generation: 1, ObservedAt: time.Unix(100, 0), Tab: world.StorageTabGems, TabKnown: true, Counts: []world.MaterialCount{{Code: "glr", Count: 99, Known: true}}}}
	return s
}
func fullStashFailure() LootStashResult {
	return LootStashResult{Status: LootStashFailed, Done: true, Reason: "verify_timeout", CompactionCandidate: true, UnitID: 809, Code: "glr", GridX: 9, GridY: 2, Attempt: 3}
}

func TestStorageCompactionFullUncraftableMaterialStopsWithoutInput(t *testing.T) {
	for _, tc := range []struct{ code, reason string }{{"gpr", crafting.ReasonGemFull}, {"r10", crafting.ReasonRuneFull}} {
		s := fullStashState()
		s.Collection.Counts = []world.MaterialCount{{Code: tc.code, Count: 99, Known: true}}
		failure := fullStashFailure()
		failure.Code = tc.code
		compact := &taskCompactionMock{}
		pipeline := &runPipeline{}
		result := pipeline.tickPersonalStashWorkflow(context.Background(), pipelineReturnDeps{Loot: &mockLootActions{stashTicks: []LootStashResult{failure}}, Compaction: compact}, pipelineStepStashItems, s, s.At)
		if !result.failed || result.reason != tc.reason || result.reasonParams["material_code"] != tc.code || compact.calls != 0 || pipeline.ret.pendingCompaction != nil {
			t.Fatalf("result=%+v", result)
		}
	}
}

func TestStorageCompactionSharedReturnMatrix(t *testing.T) {
	cases := []RunSelection{{Run: "countess"}, {Run: "summoner"}, {Run: "mephisto"}, {Run: "nihlathak"}, {Run: "lower-kurast"}, {Run: "cows"}, {Run: "countess", Phase: RunPhaseStashPersonal}, {Run: "countess", Phase: RunPhaseLootAndReturn}}
	for _, selection := range cases {
		t.Run(selection.Run+"/"+selection.Phase, func(t *testing.T) {
			registry := DefaultRunRegistry()
			definition, ok := registry.Definition(RunID(selection.Run))
			if !ok {
				t.Fatal("missing definition")
			}
			pipeline := &runPipeline{definition: definition, phase: selection.Phase}
			var machine runMachine = pipeline
			if selection.Run == "cows" {
				cow := newCowPipeline(definition, RunConfig{})
				machine = cow
				pipeline = &cow.cowSweep
			}
			looting := &mockLootActions{stashTicks: []LootStashResult{fullStashFailure(), {Status: LootStashPending, Transferred: true, UnitID: 809, Code: "glr"}, {Status: LootStashSuccess, Done: true}}}
			compaction := &taskCompactionMock{}
			deps := Deps{Loot: looting, Compaction: compaction}
			r := NewRunner(config.NewLogger("error"), selection, RunConfig{StepTimeout: time.Millisecond}, deps)
			r.run = machine
			r.started = true
			r.outcome = RunOutcomeRunning
			s := fullStashState()
			if err := r.beginStep(pipelineStepStashItems, s.At); err != nil {
				t.Fatal(err)
			}
			res := r.Tick(context.Background(), s, s.At)
			if res.Step != pipelineStepCompactStorage || res.Outcome != RunOutcomeRunning {
				t.Fatalf("full stash=%+v", res)
			}
			if !machine.usesTickTimeout(pipelineStepCompactStorage) || !machine.allowsNonInputTick(pipelineStepCompactStorage) {
				t.Fatal("compaction uses ordinary timeout/world gating")
			}
			if stage, ok := RunStageForStep(pipelineStepCompactStorage); !ok || stage != telemetry.HistoryStageReturnTown {
				t.Fatal("history stage missing")
			}
			s.At = s.At.Add(time.Second)
			s.Collection.Counts[0].Count = 96
			res = r.Tick(context.Background(), s, s.At)
			if res.Step != pipelineStepStashItems || compaction.calls != 1 || compaction.request.UnitID != 809 {
				t.Fatalf("compaction=%+v calls=%d", res, compaction.calls)
			}
			r.Tick(context.Background(), s, s.At)
			res = r.Tick(context.Background(), s, s.At)
			if res.Step != pipelineStepCloseStash || pipeline.ret.retryCompaction != nil {
				t.Fatalf("retry did not verify trigger: %+v", res)
			}
			pipeline.onStepEnter(pipelineStepOpenStash)
			if len(pipeline.ret.compactedUnits) != 0 || pipeline.ret.stashGeneration != 0 {
				t.Fatal("new stash visit retained handled items")
			}
			r.Reset("test")
			if pipeline.ret.pendingCompaction != nil || pipeline.ret.retryCompaction != nil || len(pipeline.ret.compactedUnits) != 0 || compaction.resets != 2 {
				t.Fatal("run reset retained compaction")
			}
		})
	}
}

func TestStorageCompactionReturnRejectsRetryAndIneligibleFailure(t *testing.T) {
	for _, kind := range []string{"second_failure", "not_full", "unknown", "input_failure", "policy_failure", "telemetry_failure", "full_target"} {
		t.Run(kind, func(t *testing.T) {
			failure := fullStashFailure()
			s := fullStashState()
			compaction := &taskCompactionMock{}
			switch kind {
			case "not_full":
				s.Collection.Counts[0].Count = 98
			case "unknown":
				s.Collection.Counts = nil
			case "input_failure":
				failure.Reason = "ctrl_click_failed"
				failure.CompactionCandidate = false
			case "policy_failure":
				failure.Reason = "policy_changed"
				failure.CompactionCandidate = false
			case "telemetry_failure":
				failure.Status = LootStashTelemetryFailed
				failure.CompactionCandidate = false
			case "full_target":
				compaction.failure = &crafting.Failure{Reason: crafting.ReasonGemFull, TriggerCode: "glr", MaterialCode: "gpr"}
			}
			looting := &mockLootActions{stashTicks: []LootStashResult{failure, failure}}
			pipeline := &runPipeline{}
			deps := pipelineReturnDeps{Loot: looting, Compaction: compaction}
			res := pipeline.tickPersonalStashWorkflow(context.Background(), deps, pipelineStepStashItems, s, s.At)
			if kind == "second_failure" || kind == "full_target" {
				if !res.complete {
					t.Fatal("full candidate refused")
				}
				res = pipeline.tickPersonalStashWorkflow(context.Background(), deps, pipelineStepCompactStorage, s, s.At)
				if kind == "second_failure" {
					if !res.complete {
						t.Fatal("first compaction failed")
					}
					res = pipeline.tickPersonalStashWorkflow(context.Background(), deps, pipelineStepStashItems, s, s.At)
				}
			} else if compaction.calls != 0 || pipeline.ret.pendingCompaction != nil {
				t.Fatal("ineligible failure invoked crafting")
			}
			if !res.failed || res.reasonParams["item_code"] != "glr" {
				t.Fatalf("failure/context=%+v", res)
			}
			if kind == "full_target" && (res.reason != crafting.ReasonGemFull || res.reasonParams["material_code"] != "gpr") {
				t.Fatal("full target context lost")
			}
			if kind == "second_failure" && compaction.calls != 1 {
				t.Fatal("same trigger compacted twice")
			}
		})
	}
}
