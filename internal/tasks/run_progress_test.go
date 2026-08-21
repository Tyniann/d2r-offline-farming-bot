package tasks

import (
	"strings"
	"testing"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

func TestProjectRunProgressCoversEverySupportedRun(t *testing.T) {
	tests := []struct {
		run, step, label string
		area             world.AreaID
		current, total   int
	}{
		{string(RunIDCountess), pipelineStepPlayRoute, "Kellergeschoss 3 von 5", world.TowerCellarLevel3, 6, 13},
		{string(RunIDMephisto), pipelineStepEngageBoss, "Bosskampf", world.DuranceOfHateLevel3, 4, 8},
		{string(RunIDSummoner), pipelineStepPlayRoute, "Reise zum Beschwörer", world.ArcaneSanctuary, 3, 8},
		{string(RunIDNihlathak), pipelineStepWaitForDrops, "Beute", world.HallsOfVaught, 5, 8},
		{string(RunIDLowerKurast), pipelineStepChestSweep, "Supertruhen", world.LowerKurast, 4, 8},
		{string(RunIDCows), cowStepPortalRecipe, "Portal ins Kuh-Level", world.RogueEncampment, 8, 12},
	}
	for _, test := range tests {
		t.Run(test.run, func(t *testing.T) {
			got, ok := ProjectRunProgress(test.run, test.step, test.area)
			if !ok || got.Label != test.label || got.Current != test.current || got.Total != test.total {
				t.Fatalf("ProjectRunProgress() = %+v, %v; want %q, %d/%d", got, ok, test.label, test.current, test.total)
			}
			if strings.Contains(got.Label, test.step) {
				t.Fatalf("label %q exposes internal step %q", got.Label, test.step)
			}
		})
	}
}

func TestProjectRunProgressRejectsUnknownOrUnmappedState(t *testing.T) {
	for _, test := range []struct{ run, step string }{{"unknown", pipelineStepPrecheck}, {string(RunIDCountess), "mouse_action"}} {
		if got, ok := ProjectRunProgress(test.run, test.step, world.None); ok || got != (RunProgress{}) {
			t.Fatalf("ProjectRunProgress(%q, %q) = %+v, %v", test.run, test.step, got, ok)
		}
	}
}

func TestRunnerProgressPublishesOnlyActiveValidProgress(t *testing.T) {
	runner := &Runner{selection: RunSelection{Run: string(RunIDCountess)}, outcome: RunOutcomeRunning}
	if _, ok := runner.Progress(world.TowerCellarLevel3); ok {
		t.Fatal("inactive runner published progress")
	}
	runner.started = true
	runner.tracker.name = pipelineStepPlayRoute
	progress, ok := runner.Progress(world.TowerCellarLevel3)
	if !ok || progress.Label != "Kellergeschoss 3 von 5" {
		t.Fatalf("active progress = %+v, %v", progress, ok)
	}
	progress, ok = runner.Progress(world.None)
	if !ok || progress.Label != "Kellergeschoss 3 von 5" {
		t.Fatalf("loading snapshot regressed progress = %+v, %v", progress, ok)
	}
	runner.terminal = true
	if _, ok := runner.Progress(world.TowerCellarLevel3); ok {
		t.Fatal("terminal runner published progress")
	}
}

func TestEveryProductivePipelineStepHasAValidProjection(t *testing.T) {
	standardSteps := []string{
		pipelineStepPrecheck, pipelineStepApplyTownProfile, pipelineStepAcquireTownWaypoint, pipelineStepOpenWaypoint,
		pipelineStepSelectRunWaypoint, pipelineStepWaitEntryArea, pipelineStepPlayRoute, pipelineStepAcquireBoss,
		pipelineStepEngageBoss, pipelineStepClearNearbyHostiles, pipelineStepRepositionForLoot, pipelineStepWaitForDrops,
		pipelineStepScanLoot, pipelineStepPickLoot, pipelineStepCastTownPortal, pipelineStepEnterTownPortal,
		pipelineStepWaitOriginTown, pipelineStepPlayTownEgress, pipelineStepOpenOriginWaypoint, pipelineStepSelectHubWaypoint,
		pipelineStepWaitHubArea, pipelineStepOpenStash, pipelineStepStashItems, pipelineStepCloseStash,
		pipelineStepPrepareTown, pipelineStepComplete,
	}
	for _, run := range []RunID{RunIDCountess, RunIDMephisto, RunIDSummoner, RunIDNihlathak} {
		assertValidProgressForSteps(t, string(run), standardSteps)
	}
	lowerKurastSteps := []string{
		pipelineStepPrecheck, pipelineStepApplyTownProfile, pipelineStepAcquireTownWaypoint, pipelineStepOpenWaypoint,
		pipelineStepSelectRunWaypoint, pipelineStepWaitEntryArea, pipelineStepPlayRoute, pipelineStepChestSweep,
		pipelineStepRepositionForLoot, pipelineStepWaitForDrops, pipelineStepScanLoot, pipelineStepPickLoot,
		pipelineStepCastTownPortal, pipelineStepEnterTownPortal, pipelineStepWaitOriginTown, pipelineStepPlayTownEgress,
		pipelineStepOpenOriginWaypoint, pipelineStepSelectHubWaypoint, pipelineStepWaitHubArea, pipelineStepOpenStash,
		pipelineStepStashItems, pipelineStepCloseStash, pipelineStepPrepareTown, pipelineStepComplete,
	}
	assertValidProgressForSteps(t, string(RunIDLowerKurast), lowerKurastSteps)
	assertValidProgressForSteps(t, string(RunIDCows), []string{
		cowStepPreflight, cowStepTownReady, cowStepAcquireWaypoint, cowStepOpenWaypoint, cowStepSelectStony, cowStepWaitStony,
		cowStepPlayLegRoute, cowStepOpenWirt, cowStepPickupLeg, cowStepCastReturnTP, cowStepEnterReturnTP, cowStepWaitRogue,
		cowStepBuyTome, cowStepSafeFailure, cowStepSetupComplete, cowStepPortalRecipe, cowStepRecipeComplete, cowStepSweep,
		cowStepSweepComplete, pipelineStepCastTownPortal, pipelineStepEnterTownPortal, pipelineStepWaitOriginTown,
		pipelineStepOpenStash, pipelineStepStashItems, pipelineStepCloseStash, pipelineStepPrepareTown, pipelineStepComplete,
	})
}

func assertValidProgressForSteps(t *testing.T, run string, steps []string) {
	t.Helper()
	for _, step := range steps {
		progress, ok := ProjectRunProgress(run, step, world.None)
		if !ok || progress.Label == "" || progress.Current < 1 || progress.Current > progress.Total {
			t.Errorf("run %q step %q has invalid progress %+v, %v", run, step, progress, ok)
		}
	}
}
