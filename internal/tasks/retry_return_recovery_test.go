package tasks

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/pathing"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/profile"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/telemetry"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

func TestRetryReturnPreClickFailureStartsLocalClearBeforeTeleport(t *testing.T) {
	definition, _ := DefaultRunRegistry().Definition(RunIDCountess)
	pipeline := &runPipeline{definition: definition, phase: RunPhaseRetryReturn}
	combat := &mockCombatActions{}
	clear := &routeClearMock{}
	portals := &mockTownPortalActions{results: []pathing.TownPortalActionResult{{
		Status: pathing.TownPortalActionTooFar, Done: true, PortalUnitID: 77, BlockerUnitID: 88,
	}}}
	now := time.Now()
	state := portalCellarState(77, world.Position{X: 140, Y: 100})
	state.At = now

	result := pipeline.tickEnterTownPortal(context.Background(), Deps{Portal: portals, Combat: combat, RouteClear: clear, Profile: &mockProfileActions{}}, state, now)
	if result.failed || result.complete || pipeline.ret.destinationPhase != portalDestinationClear ||
		!pipeline.ret.destinationRecoveryUsed || pipeline.ret.destinationClear.preferredUnitID != 88 {
		t.Fatalf("result=%+v return=%+v", result, pipeline.ret)
	}
	if combat.teleportCalls != 0 || len(clear.requests) != 0 {
		t.Fatalf("teleports=%d clear requests=%d before recovery tick", combat.teleportCalls, len(clear.requests))
	}
}

func TestRetryReturnPostClickTimeoutStartsOneRecoveryClear(t *testing.T) {
	definition, _ := DefaultRunRegistry().Definition(RunIDCountess)
	pipeline := &runPipeline{definition: definition, phase: RunPhaseRetryReturn}
	trace := &pipelineTelemetry{}
	deps := pipelineReturnDeps{Combat: &mockCombatActions{}, RouteClear: &routeClearMock{}, Profile: &mockProfileActions{}, Telemetry: trace}
	base := time.Now()
	state := portalCellarState(77, world.Position{X: 100, Y: 100})
	for _, offset := range []time.Duration{0, 500 * time.Millisecond, 1100 * time.Millisecond} {
		state.At = base.Add(offset)
		result := pipeline.tickPortalDestinationObservation(deps, state, state.At)
		if result.failed {
			t.Fatalf("offset %s: %+v", offset, result)
		}
	}
	if !pipeline.ret.destinationRecoveryUsed || pipeline.ret.destinationPhase != portalDestinationClear {
		t.Fatalf("return state = %+v", pipeline.ret)
	}
	if len(trace.events) != 2 || trace.events[1].Event != telemetry.LocalRecoveryClearStarted ||
		trace.events[1].RequiredRadiusTiles != localThreatClearRadiusTiles || trace.events[1].ActionBudget != localThreatClearMaxActions {
		t.Fatalf("start telemetry = %+v", trace.events)
	}
}

func TestRetryReturnClearedAndExhaustedAdvanceToOnePortalRetry(t *testing.T) {
	definition, _ := DefaultRunRegistry().Definition(RunIDCountess)
	for _, outcome := range []string{"cleared", "exhausted"} {
		t.Run(outcome, func(t *testing.T) {
			pipeline := &runPipeline{definition: definition, phase: RunPhaseRetryReturn}
			base := time.Now()
			pipeline.ret.destinationRecoveryUsed = true
			pipeline.ret.destinationPhase = portalDestinationClear
			pipeline.ret.destinationPortalUnitID = 77
			pipeline.ret.destinationClear.start(world.Position{X: 100, Y: 100}, 0, base)
			if outcome == "cleared" {
				pipeline.ret.destinationClear.noTargetSnapshots = localThreatClearStableSnapshots - 1
			} else {
				pipeline.ret.destinationClear.actions = localThreatClearMaxActions
			}
			combat := &mockCombatActions{}
			clear := &routeClearMock{}
			trace := &pipelineTelemetry{}
			portals := &mockTownPortalActions{results: []pathing.TownPortalActionResult{{Status: pathing.TownPortalActionClicked, Done: true}}}
			deps := pipelineReturnDeps{Profile: &mockProfileActions{}, Combat: combat, RouteClear: clear, Portal: portals, Telemetry: trace}
			state := portalCellarState(77, world.Position{X: 100, Y: 100})
			state.At = base.Add(time.Millisecond)
			state.MonsterCoverage = world.MonsterCoverage{MonstersTruncated: true, MonsterCoverageRadiusTiles: localThreatClearRadiusTiles}

			result := pipeline.tickPortalDestinationClear(context.Background(), deps, state, state.At)
			if result.failed || pipeline.ret.destinationPhase != portalDestinationTeleport || combat.stopCalls != 1 {
				t.Fatalf("clear result=%+v phase=%q stops=%d", result, pipeline.ret.destinationPhase, combat.stopCalls)
			}
			if len(trace.events) != 1 || trace.events[0].Event != telemetry.LocalRecoveryClearFinished || trace.events[0].Outcome != outcome {
				t.Fatalf("clear telemetry = %+v", trace.events)
			}
			result = pipeline.tickPortalDestinationTeleport(deps, state, state.At.Add(time.Millisecond))
			if result.failed || pipeline.ret.destinationPhase != portalDestinationSettle || combat.teleportCalls != 1 {
				t.Fatalf("teleport result=%+v phase=%q calls=%d", result, pipeline.ret.destinationPhase, combat.teleportCalls)
			}
			state.At = state.At.Add(lootRepositionRetryDelay + 2*time.Millisecond)
			result = pipeline.tickPortalDestinationSettle(deps, state, state.At)
			if result.failed || pipeline.ret.destinationPhase != portalDestinationRetryClick {
				t.Fatalf("settle result=%+v phase=%q", result, pipeline.ret.destinationPhase)
			}
			result = pipeline.tickPortalDestinationRetryClick(context.Background(), deps, state, state.At.Add(time.Millisecond), true)
			if result.failed || !result.complete || portals.calls != 1 || !pipeline.ret.destinationRecoveryUsed {
				t.Fatalf("retry result=%+v calls=%d state=%+v", result, portals.calls, pipeline.ret)
			}
			if len(trace.events) != 2 || trace.events[1].Event != telemetry.ReturnPortalRetry || trace.events[1].Attempt != 1 || trace.events[1].Outcome != "success" {
				t.Fatalf("retry telemetry = %+v", trace.events)
			}
		})
	}
}

func TestRetryReturnLocalClearChecksFailClosedResourcesBeforeCombat(t *testing.T) {
	definition, _ := DefaultRunRegistry().Definition(RunIDCountess)
	pipeline := &runPipeline{definition: definition, phase: RunPhaseRetryReturn}
	now := time.Now()
	pipeline.ret.destinationRecoveryUsed = true
	pipeline.ret.destinationPhase = portalDestinationClear
	pipeline.ret.destinationClear.start(world.Position{X: 100, Y: 100}, 0, now)
	resources := &mockProfileActions{resourceResults: []profile.Result{{Status: profile.StatusFailed, Reason: "critical_resource_unavailable"}}}
	combat := &mockCombatActions{}
	clear := &routeClearMock{result: profile.Result{Status: profile.StatusAction}}
	state := portalCellarState(77, world.Position{X: 100, Y: 100})
	state.At = now
	state.Monsters = []world.Monster{{UnitID: 9, NPCID: 1, Position: world.Position{X: 101, Y: 100}}}

	result := pipeline.tickPortalDestinationClear(context.Background(), pipelineReturnDeps{Profile: resources, Combat: combat, RouteClear: clear}, state, now)
	if !result.failed || result.reason != "critical_resource_unavailable" || len(clear.requests) != 0 || combat.stopCalls != 1 {
		t.Fatalf("result=%+v clear=%d stops=%d", result, len(clear.requests), combat.stopCalls)
	}
	if len(resources.resourceContexts) != 1 || !resources.resourceContexts[0].FailOnUnavailable {
		t.Fatalf("resource contexts = %+v", resources.resourceContexts)
	}
}

func TestRetryReturnStopAttackFailureBlocksPortalRetry(t *testing.T) {
	definition, _ := DefaultRunRegistry().Definition(RunIDCountess)
	pipeline := &runPipeline{definition: definition, phase: RunPhaseRetryReturn}
	now := time.Now()
	pipeline.ret.destinationRecoveryUsed = true
	pipeline.ret.destinationPhase = portalDestinationClear
	pipeline.ret.destinationClear.start(world.Position{X: 100, Y: 100}, 0, now)
	pipeline.ret.destinationClear.noTargetSnapshots = localThreatClearStableSnapshots - 1
	combat := &mockCombatActions{stopErr: errors.New("release failed")}
	state := portalCellarState(77, world.Position{X: 100, Y: 100})
	state.At = now.Add(time.Millisecond)
	state.MonsterCoverage = world.MonsterCoverage{MonstersTruncated: true, MonsterCoverageRadiusTiles: localThreatClearRadiusTiles}

	result := pipeline.tickPortalDestinationClear(context.Background(), pipelineReturnDeps{
		Profile: &mockProfileActions{}, Combat: combat, RouteClear: &routeClearMock{},
	}, state, state.At)
	if !result.failed || result.reason != "portal_recovery_combat_stop_failed" || pipeline.ret.destinationPhase == portalDestinationTeleport {
		t.Fatalf("result=%+v phase=%q", result, pipeline.ret.destinationPhase)
	}
}

func TestRetryReturnPortalRetryIsSingleAndReportsFailure(t *testing.T) {
	definition, _ := DefaultRunRegistry().Definition(RunIDCountess)
	for _, tc := range []struct {
		name       string
		status     pathing.TownPortalActionStatus
		wantFailed bool
	}{
		{name: "clicked", status: pathing.TownPortalActionClicked},
		{name: "second miss", status: pathing.TownPortalActionHoverNotFound, wantFailed: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pipeline := &runPipeline{definition: definition, phase: RunPhaseRetryReturn}
			pipeline.ret.destinationRecoveryUsed = true
			pipeline.ret.destinationPhase = portalDestinationRetryClick
			pipeline.ret.destinationPortalUnitID = 77
			state := portalCellarState(77, world.Position{X: 100, Y: 100})
			result := pipeline.tickPortalDestinationRetryClick(context.Background(), pipelineReturnDeps{
				Portal: &mockTownPortalActions{results: []pathing.TownPortalActionResult{{Status: tc.status, Done: true}}},
			}, state, time.Now(), false)
			if result.failed != tc.wantFailed || (!tc.wantFailed && pipeline.ret.destinationPhase != portalDestinationRetryWait) {
				t.Fatalf("result=%+v phase=%q", result, pipeline.ret.destinationPhase)
			}
		})
	}
}

func TestRunnerProjectsCurrentRecoveryStep(t *testing.T) {
	definition, _ := DefaultRunRegistry().Definition(RunIDCountess)
	pipeline := &runPipeline{definition: definition, phase: RunPhaseRetryReturn}
	runner := &Runner{selection: RunSelection{Run: string(RunIDCountess), Phase: RunPhaseRetryReturn}, run: pipeline}

	tests := []struct {
		step             string
		destinationPhase portalDestinationPhase
		want             string
	}{
		{step: pipelineStepWaitRecoveryArea, want: "retry_return"},
		{step: pipelineStepWaitOriginTown, destinationPhase: portalDestinationClear, want: "local_recovery_clear"},
		{step: pipelineStepWaitOriginTown, destinationPhase: portalDestinationRetryClick, want: "return_portal_retry"},
		{step: pipelineStepPlayTownEgress, want: "return_to_act1"},
	}
	for _, test := range tests {
		runner.tracker.name = test.step
		pipeline.ret.destinationPhase = test.destinationPhase
		if got := runner.RecoveryStep(); got != test.want {
			t.Fatalf("step=%q phase=%q recovery=%q, want %q", test.step, test.destinationPhase, got, test.want)
		}
	}
}
