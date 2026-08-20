package tasks

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestPhase22StandardPipelineGolden freezes the observable transition graph
// before Phase 22 splits the standard pipeline across domain files.
func TestPhase22StandardPipelineGolden(t *testing.T) {
	var trace strings.Builder
	for _, runID := range []RunID{RunIDCountess, RunIDMephisto, RunIDSummoner, RunIDNihlathak, RunIDLowerKurast} {
		definition, ok := DefaultRunRegistry().Definition(runID)
		if !ok {
			t.Fatalf("definition %q missing", runID)
		}
		pipeline := &runPipeline{definition: definition}
		fmt.Fprintf(&trace, "%s.full=%s\n", runID, strings.Join(characterizationSteps(t, pipeline, pipeline.firstStep()), ">"))
	}

	countess, _ := DefaultRunRegistry().Definition(RunIDCountess)
	lootRecovery := &runPipeline{definition: countess, loot: pipelineLootState{lootScanHasTarget: true}}
	fmt.Fprintf(&trace, "recovery.loot_target=%s\n", strings.Join(characterizationSteps(t, lootRecovery, pipelineStepScanLoot), ">"))

	summoner, _ := DefaultRunRegistry().Definition(RunIDSummoner)
	postKillRecovery := &runPipeline{definition: summoner, boss: pipelineBossState{bossKillEmitted: true}}
	fmt.Fprintf(&trace, "recovery.boss_already_killed=%s\n", strings.Join(characterizationSteps(t, postKillRecovery, pipelineStepAcquireBoss), ">"))

	retryAct1 := &runPipeline{definition: countess, phase: RunPhaseRetryReturn}
	fmt.Fprintf(&trace, "recovery.retry_return_act1=%s\n", strings.Join(characterizationSteps(t, retryAct1, retryAct1.firstStep()), ">"))

	mephisto, _ := DefaultRunRegistry().Definition(RunIDMephisto)
	retryForeignTown := &runPipeline{definition: mephisto, phase: RunPhaseRetryReturn}
	fmt.Fprintf(&trace, "recovery.retry_return_foreign_town=%s\n", strings.Join(characterizationSteps(t, retryForeignTown, retryForeignTown.firstStep()), ">"))
	fmt.Fprintf(&trace, "failure.play_route_timeout=%s\n", retryForeignTown.timeoutReason(pipelineStepPlayRoute))
	fmt.Fprintf(&trace, "failure.waypoint_timeout=%s\n", retryForeignTown.timeoutReason(pipelineStepWaitEntryArea))

	want, err := os.ReadFile(filepath.Join("testdata", "phase22-standard-pipeline.golden"))
	if err != nil {
		t.Fatal(err)
	}
	if got := trace.String(); got != string(want) {
		t.Fatalf("Phase-22 pipeline characterization changed:\n--- want\n%s--- got\n%s", want, got)
	}
}

func characterizationSteps(t *testing.T, pipeline *runPipeline, first string) []string {
	t.Helper()
	steps := make([]string, 0, 32)
	for step := first; step != ""; step = pipeline.nextStep(step) {
		steps = append(steps, step)
		if len(steps) > 64 {
			t.Fatalf("pipeline transition loop after %q", step)
		}
	}
	return steps
}
