package tasks

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/profile"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/telemetry"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

type pipelineTelemetry struct {
	events []telemetry.Event
	failAt int
}

func (p *pipelineTelemetry) Emit(event telemetry.Event) error {
	if p.failAt > 0 && len(p.events)+1 == p.failAt {
		return errors.New("disk unavailable")
	}
	p.events = append(p.events, event)
	return nil
}

func TestRunPipelineTelemetryCarriesDefinitionStepOutcomeAndActionIndex(t *testing.T) {
	trace := &pipelineTelemetry{}
	definition, _ := DefaultRunRegistry().Definition(RunIDCountess)
	pipeline := &runPipeline{definition: definition, encounterActionIndex: 0}
	runner := NewRunner(config.NewLogger("error"), RunSelection{Run: string(RunIDCountess)}, RunConfig{}, Deps{Telemetry: trace})
	runner.run = pipeline

	if err := runner.emitStep(telemetry.RunStepStarted, pipelineStepEngageBoss, RunOutcomeRunning, ""); err != nil {
		t.Fatal(err)
	}
	if len(trace.events) != 1 {
		t.Fatalf("events = %d, want 1", len(trace.events))
	}
	event := trace.events[0]
	if event.DefinitionID != "countess" || event.Step != pipelineStepEngageBoss || event.Stage != telemetry.HistoryStageCombat || event.Outcome != string(RunOutcomeRunning) || event.ActionIndex == nil || *event.ActionIndex != 0 {
		t.Fatalf("event = %+v", event)
	}
}

func TestPhase14AllPipelineStepsHaveExactlyOneStableStage(t *testing.T) {
	tests := map[telemetry.HistoryStage][]string{
		telemetry.HistoryStageTravel:     {pipelineStepPrecheck, pipelineStepApplyTownProfile, pipelineStepAcquireTownWaypoint, pipelineStepOpenWaypoint, pipelineStepSelectRunWaypoint, pipelineStepWaitEntryArea, pipelineStepPlayRoute},
		telemetry.HistoryStageCombat:     {pipelineStepAcquireBoss, pipelineStepEngageBoss},
		telemetry.HistoryStageLoot:       {pipelineStepRepositionForLoot, pipelineStepWaitForDrops, pipelineStepScanLoot, pipelineStepPickLoot},
		telemetry.HistoryStageReturnTown: {pipelineStepCastTownPortal, pipelineStepEnterTownPortal, pipelineStepWaitOriginTown, pipelineStepPlayTownEgress, pipelineStepOpenOriginWaypoint, pipelineStepSelectHubWaypoint, pipelineStepWaitHubArea, pipelineStepOpenStash, pipelineStepStashItems, pipelineStepCloseStash, pipelineStepPrepareTown, pipelineStepComplete},
	}
	seen := make(map[string]telemetry.HistoryStage)
	for wantStage, steps := range tests {
		for _, step := range steps {
			stage, ok := RunStageForStep(step)
			if !ok || stage != wantStage {
				t.Fatalf("step %q stage=%q ok=%t, want %q", step, stage, ok, wantStage)
			}
			if previous, duplicate := seen[step]; duplicate {
				t.Fatalf("step %q mapped twice to %q and %q", step, previous, stage)
			}
			seen[step] = stage
		}
	}
	if len(seen) != 25 {
		t.Fatalf("mapped steps=%d, want 25", len(seen))
	}
	if _, ok := RunStageForStep("unknown"); ok {
		t.Fatal("unknown step received a stage")
	}
}

func TestRunPipelineTelemetryFailureStopsBeforeFollowingInput(t *testing.T) {
	trace := &pipelineTelemetry{failAt: 2}
	townWalk := &mockTownWalker{}
	runner := NewRunner(config.NewLogger("error"), RunSelection{Run: string(RunIDCountess)}, RunConfig{StepTimeout: time.Second}, Deps{Telemetry: trace, TownWalk: townWalk})

	result := runner.Tick(context.Background(), healthy(townState()), time.Now())
	if result.Outcome != RunOutcomeFailed || result.Reason != "telemetry_failed" {
		t.Fatalf("result = %+v", result)
	}
	if townWalk.calls != 0 {
		t.Fatalf("town-walk calls = %d, want no input after failed transition telemetry", townWalk.calls)
	}
}

func TestRunPipelineCentralResetBarrierClearsGenerationOnce(t *testing.T) {
	definition, _ := DefaultRunRegistry().Definition(RunIDCountess)
	pipeline := &runPipeline{
		definition: definition, targetSeen: true, targetUnitID: 42, routeStarted: true,
		lootPickupActive: true, encounterActionIndex: 1,
	}
	profileActions := &mockProfileActions{}
	lootActions := &mockLootActions{}
	runner := NewRunner(config.NewLogger("error"), RunSelection{Run: string(RunIDCountess)}, RunConfig{}, Deps{Profile: profileActions, Loot: lootActions})
	runner.run = pipeline

	runner.Reset("process_lost")
	runner.Reset("duplicate")
	if pipeline.targetSeen || pipeline.targetUnitID != 0 || pipeline.routeStarted || pipeline.lootPickupActive || pipeline.encounterActionIndex != 0 {
		t.Fatalf("pipeline state crossed reset barrier: %+v", pipeline)
	}
	if profileActions.resetCalls != 1 || lootActions.resetCalls != 1 {
		t.Fatalf("reset calls: profile=%d loot=%d, want exactly one", profileActions.resetCalls, lootActions.resetCalls)
	}
}

func TestRunPipelineEncounterActionTelemetryUsesStableIndex(t *testing.T) {
	definition, _ := DefaultRunRegistry().Definition(RunIDCountess)
	trace := &pipelineTelemetry{}
	profiles := &mockProfileActions{hookResults: []profile.Result{{Status: profile.StatusComplete}}}
	combat := &mockCombatActions{}
	target := countessMonster(73, world.Position{X: 101, Y: 100})
	pipeline := &runPipeline{
		definition: definition, combat: killRunConfig().Combat, targetSeen: true, targetUnitID: target.UnitID,
	}

	result := pipeline.onBossTick(context.Background(), Deps{Telemetry: trace, Profile: profiles, Combat: combat}, pipelineStepEngageBoss, healthy(cellar5State(target)), time.Now())
	if result.failed || len(trace.events) != 2 || trace.events[0].Event != telemetry.RunEncounterActionStarted || trace.events[1].Event != telemetry.RunEncounterActionCompleted {
		t.Fatalf("result=%+v events=%+v", result, trace.events)
	}
	if trace.events[0].ActionIndex == nil || *trace.events[0].ActionIndex != 0 || trace.events[1].ActionIndex == nil || *trace.events[1].ActionIndex != 0 {
		t.Fatalf("action indexes = %+v, %+v", trace.events[0].ActionIndex, trace.events[1].ActionIndex)
	}
}

func TestRunPipelineEncounterTelemetryFailureBlocksProfileAndCombatInput(t *testing.T) {
	definition, _ := DefaultRunRegistry().Definition(RunIDCountess)
	trace := &pipelineTelemetry{failAt: 1}
	profiles := &mockProfileActions{}
	combat := &mockCombatActions{}
	target := countessMonster(74, world.Position{X: 101, Y: 100})
	pipeline := &runPipeline{
		definition: definition, combat: killRunConfig().Combat, targetSeen: true, targetUnitID: target.UnitID,
	}

	result := pipeline.onBossTick(context.Background(), Deps{Telemetry: trace, Profile: profiles, Combat: combat}, pipelineStepEngageBoss, healthy(cellar5State(target)), time.Now())
	if !result.failed || result.reason != "telemetry_failed" || profiles.hookCalls != 0 || combat.castCalls != 0 {
		t.Fatalf("result=%+v hookCalls=%d combatCalls=%d", result, profiles.hookCalls, combat.castCalls)
	}
}

func TestRunPipelineWaitEntryAreaIgnoresTransientAreaZero(t *testing.T) {
	definition, _ := DefaultRunRegistry().Definition(RunIDMephisto)
	pipeline := &runPipeline{definition: definition, phase: RunPhasePlayRoute}
	loading := world.State{Valid: true, Phase: world.GamePhaseInGame, Area: world.LookupArea(world.None)}
	result := pipeline.onTravelTick(context.Background(), Deps{}, pipelineStepWaitEntryArea, loading, time.Now(), time.Now())
	if result.complete || result.failed {
		t.Fatalf("transient Area 0 result = %+v, want pending", result)
	}

	wrong := loading
	wrong.Area = world.LookupArea(world.BlackMarsh)
	result = pipeline.onTravelTick(context.Background(), Deps{}, pipelineStepWaitEntryArea, wrong, time.Now(), time.Now())
	if !result.failed || result.reason != string(RunReasonUnexpectedArea) {
		t.Fatalf("confirmed wrong area result = %+v", result)
	}
}
