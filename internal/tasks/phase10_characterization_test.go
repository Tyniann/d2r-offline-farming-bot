package tasks

import (
	"reflect"
	"testing"
)

func TestPhase10CharacterizationCountessFullRunTransitionsWithoutLoot(t *testing.T) {
	run := &runPipeline{}
	got := []string{run.firstStep()}
	for step := run.firstStep(); ; {
		step = run.nextStep(step)
		if step == "" {
			break
		}
		got = append(got, step)
	}
	want := []string{
		pipelineStepPrecheck,
		pipelineStepAcquireTownWaypoint,
		pipelineStepOpenWaypoint,
		pipelineStepSelectRunWaypoint,
		pipelineStepWaitEntryArea,
		pipelineStepPlayRoute,
		pipelineStepAcquireBoss,
		pipelineStepEngageBoss,
		pipelineStepRepositionForLoot,
		pipelineStepWaitForDrops,
		pipelineStepScanLoot,
		pipelineStepCastTownPortal,
		pipelineStepEnterTownPortal,
		pipelineStepWaitOriginTown,
		pipelineStepOpenStash,
		pipelineStepStashItems,
		pipelineStepCloseStash,
		pipelineStepPrepareTown,
		pipelineStepComplete,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Countess full-run transitions = %v, want %v", got, want)
	}
}

func TestPhase10CharacterizationCountessLootBranchReturnsToSharedCompletion(t *testing.T) {
	run := &runPipeline{loot: pipelineLootState{lootScanHasTarget: true}}
	if got := run.nextStep(pipelineStepScanLoot); got != pipelineStepPickLoot {
		t.Fatalf("scan_loot successor = %q, want %q", got, pipelineStepPickLoot)
	}
	if got := run.nextStep(pipelineStepPickLoot); got != pipelineStepCastTownPortal {
		t.Fatalf("pick_loot successor = %q, want %q", got, pipelineStepCastTownPortal)
	}
	if got := run.nextStep(pipelineStepCloseStash); got != pipelineStepPrepareTown {
		t.Fatalf("close stash successor = %q, want %q", got, pipelineStepPrepareTown)
	}
	if got := run.nextStep(pipelineStepPrepareTown); got != pipelineStepComplete {
		t.Fatalf("town handoff successor = %q, want %q", got, pipelineStepComplete)
	}
}

func TestPhase10CharacterizationCountessPersistentStateResetsAtExistingStepBoundaries(t *testing.T) {
	run := &runPipeline{travel: pipelineTravelState{navStarted: true,
		routeStarted: true}, boss: pipelineBossState{chestFallbackStarted: true,
		targetSeen:        true,
		targetUnitID:      42,
		targetAbsentTicks: 2},
	}
	run.onStepEnter(pipelineStepAcquireBoss)
	if run.travel.navStarted || run.travel.routeStarted || run.boss.chestFallbackStarted || run.boss.targetSeen || run.boss.targetUnitID != 0 || run.boss.targetAbsentTicks != 0 {
		t.Fatalf("acquire_boss did not reset persistent state: %+v", run)
	}
}
