package tasks

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/town"
)

const updatePipelineTransitionGoldenEnv = "UPDATE_PIPELINE_TRANSITION_GOLDEN"

var (
	transitionPhases = []string{
		"full",
		RunPhaseTravelEntry,
		RunPhasePlayRoute,
		RunPhaseBoss,
		RunPhaseLootAndReturn,
		RunPhaseRetryReturn,
		RunPhaseStashPersonal,
		RunPhaseTownReady,
		"not-a-phase",
	}
	transitionCurrents = []string{
		pipelineStepPrecheck,
		pipelineStepApplyTownProfile,
		pipelineStepAcquireTownWaypoint,
		pipelineStepOpenWaypoint,
		pipelineStepSelectRunWaypoint,
		pipelineStepWaitEntryArea,
		pipelineStepPlayRoute,
		pipelineStepChestSweep,
		pipelineStepAcquireBoss,
		pipelineStepEngageBoss,
		pipelineStepClearNearbyHostiles,
		pipelineStepRepositionForLoot,
		pipelineStepWaitForDrops,
		pipelineStepScanLoot,
		pipelineStepPickLoot,
		pipelineStepWaitRecoveryArea,
		pipelineStepCastTownPortal,
		pipelineStepEnterTownPortal,
		pipelineStepWaitOriginTown,
		pipelineStepPlayTownEgress,
		pipelineStepOpenOriginWaypoint,
		pipelineStepSelectHubWaypoint,
		pipelineStepWaitHubArea,
		pipelineStepOpenStash,
		pipelineStepStashItems,
		pipelineStepCloseStash,
		pipelineStepPrepareTown,
		pipelineStepComplete,
		"-",
		"unknown_step",
		cowStepPreflight,
	}
	transitionResumes = []string{"off", pipelineStepPlayRoute, "empty"}
)

type transitionFlags struct {
	loot, kill, clear, foreign int
	resume                     string
}

func (f transitionFlags) String() string {
	return fmt.Sprintf("loot=%d,kill=%d,clear=%d,foreign=%d,resume=%s", f.loot, f.kill, f.clear, f.foreign, f.resume)
}

func (f transitionFlags) zero() bool {
	return f.loot == 0 && f.kill == 0 && f.clear == 0 && f.foreign == 0 && f.resume == "off"
}

func TestPipelineFirstStepIsPrecheck(t *testing.T) {
	for _, phase := range transitionPhases {
		pipeline := pipelineForTransition(phase, transitionFlags{resume: "off"})
		if got := pipeline.firstStep(); got != pipelineStepPrecheck {
			t.Fatalf("firstStep phase %q = %q, want %q", phase, got, pipelineStepPrecheck)
		}
	}
}

func TestStandardPipelinePhasePathsGolden(t *testing.T) {
	countess := mustDefinition(t, RunIDCountess)
	mephisto := mustDefinition(t, RunIDMephisto)
	paths := []struct {
		key   string
		build func() *runPipeline
		want  string
	}{
		{"town_ready", func() *runPipeline {
			return &runPipeline{definition: countess, phase: RunPhaseTownReady}
		}, "precheck>town_ready_profile>complete"},
		{"stash_personal", func() *runPipeline {
			return &runPipeline{definition: countess, phase: RunPhaseStashPersonal}
		}, "precheck>open_personal_stash>stash_items>close_personal_stash>complete"},
		{"travel_entry", func() *runPipeline {
			return &runPipeline{definition: countess, phase: RunPhaseTravelEntry}
		}, "precheck>acquire_town_waypoint>open_waypoint>select_run_waypoint>wait_entry_area"},
		{"play_route", func() *runPipeline {
			return &runPipeline{definition: countess, phase: RunPhasePlayRoute}
		}, "precheck>acquire_town_waypoint>open_waypoint>select_run_waypoint>wait_entry_area>play_bound_route"},
		{"play_route_resume_entry", func() *runPipeline {
			return &runPipeline{definition: countess, phase: RunPhasePlayRoute, travel: pipelineTravelState{
				resumeAfterPrecheckSet: true, resumeAfterPrecheck: pipelineStepPlayRoute,
			}}
		}, "precheck>play_bound_route"},
		{"play_route_resume_terminal", func() *runPipeline {
			return &runPipeline{definition: countess, phase: RunPhasePlayRoute, travel: pipelineTravelState{
				resumeAfterPrecheckSet: true,
			}}
		}, "precheck"},
		{"boss", func() *runPipeline {
			return &runPipeline{definition: countess, phase: RunPhaseBoss}
		}, "precheck>acquire_boss>engage_boss"},
		{"loot_return_act1", func() *runPipeline {
			return &runPipeline{definition: countess, phase: RunPhaseLootAndReturn}
		}, "precheck>wait_for_drops>scan_loot>cast_town_portal>enter_town_portal>wait_origin_town>open_personal_stash>stash_items>close_personal_stash>complete"},
		{"loot_return_act1_target", func() *runPipeline {
			return &runPipeline{definition: countess, phase: RunPhaseLootAndReturn, loot: pipelineLootState{lootScanHasTarget: true}}
		}, "precheck>wait_for_drops>scan_loot>pick_loot>cast_town_portal>enter_town_portal>wait_origin_town>open_personal_stash>stash_items>close_personal_stash>complete"},
		{"loot_return_foreign", func() *runPipeline {
			return &runPipeline{definition: mephisto, phase: RunPhaseLootAndReturn}
		}, "precheck>wait_for_drops>scan_loot>cast_town_portal>enter_town_portal>wait_origin_town>play_town_egress>open_origin_waypoint>select_hub_waypoint>wait_hub_area>open_personal_stash>stash_items>close_personal_stash>complete"},
		{"loot_return_foreign_target", func() *runPipeline {
			return &runPipeline{definition: mephisto, phase: RunPhaseLootAndReturn, loot: pipelineLootState{lootScanHasTarget: true}}
		}, "precheck>wait_for_drops>scan_loot>pick_loot>cast_town_portal>enter_town_portal>wait_origin_town>play_town_egress>open_origin_waypoint>select_hub_waypoint>wait_hub_area>open_personal_stash>stash_items>close_personal_stash>complete"},
		{"failure.hub_waypoint_timeout", func() *runPipeline {
			return &runPipeline{definition: countess}
		}, "waypoint_destination_timeout"},
		{"lower_kurast_full", func() *runPipeline {
			return &runPipeline{definition: mustDefinition(t, RunIDLowerKurast)}
		}, "precheck>acquire_town_waypoint>open_waypoint>select_run_waypoint>wait_entry_area>play_bound_route>chest_sweep>wait_for_drops>scan_loot>cast_town_portal>enter_town_portal>wait_origin_town>play_town_egress>open_origin_waypoint>select_hub_waypoint>wait_hub_area>open_personal_stash>stash_items>close_personal_stash>prepare_town_handoff>complete"},
		{"lower_kurast_boss", func() *runPipeline {
			return &runPipeline{definition: mustDefinition(t, RunIDLowerKurast), phase: RunPhaseBoss}
		}, "precheck>chest_sweep"},
	}

	var got strings.Builder
	for _, path := range paths {
		pipeline := path.build()
		var value string
		if path.key == "failure.hub_waypoint_timeout" {
			value = pipeline.timeoutReason(pipelineStepWaitHubArea)
		} else {
			value = strings.Join(walkTransitionSteps(t, pipeline, pipeline.firstStep()), ">")
		}
		if value != path.want {
			t.Fatalf("live nextStep %s = %q, spec wants %q", path.key, value, path.want)
		}
		fmt.Fprintf(&got, "%s=%s\n", path.key, value)
	}
	compareTransitionGolden(t, "standard-pipeline-phase-paths.golden", got.String())
}

func TestStandardPipelineTransitionMatrixGolden(t *testing.T) {
	var baseline, branches strings.Builder
	seenFamilies := map[string]bool{}
	for _, phase := range transitionPhases {
		zero := pipelineForTransition(phase, transitionFlags{resume: "off"})
		for _, current := range transitionCurrents {
			base := zero.nextStep(decodeTransitionStep(current))
			fmt.Fprintf(&baseline, "%s|%s|%s\n", phase, current, encodeTransitionStep(base))
			for _, flags := range allTransitionFlags() {
				if flags.zero() {
					continue
				}
				next := pipelineForTransition(phase, flags).nextStep(decodeTransitionStep(current))
				if next == base {
					continue
				}
				line := fmt.Sprintf("%s|%s|%s|%s", phase, current, flags.String(), encodeTransitionStep(next))
				family, ok := declaredTransitionFamily(phase, current, flags, encodeTransitionStep(next))
				if !ok {
					t.Fatalf("undeclared transition branch: %s", line)
				}
				seenFamilies[family] = true
				fmt.Fprintf(&branches, "%s\n", line)
			}
		}
	}
	for _, family := range declaredTransitionFamilies() {
		if !seenFamilies[family] {
			t.Fatalf("declared transition family %q missing from live nextStep", family)
		}
	}
	compareTransitionGolden(t, "standard-pipeline-transition-baseline.golden", baseline.String())
	compareTransitionGolden(t, "standard-pipeline-transition-branches.golden", branches.String())
}

func allTransitionFlags() []transitionFlags {
	out := make([]transitionFlags, 0, 48)
	for _, loot := range []int{0, 1} {
		for _, kill := range []int{0, 1} {
			for _, clear := range []int{0, 1} {
				for _, foreign := range []int{0, 1} {
					for _, resume := range transitionResumes {
						out = append(out, transitionFlags{loot: loot, kill: kill, clear: clear, foreign: foreign, resume: resume})
					}
				}
			}
		}
	}
	return out
}

func pipelineForTransition(phaseLabel string, flags transitionFlags) *runPipeline {
	phase := phaseLabel
	if phaseLabel == "full" {
		phase = ""
	}
	return &runPipeline{
		definition: RunDefinition{
			ClearNearbyAfterBoss: flags.clear == 1,
			ReturnOrigin:         originForTransition(flags.foreign),
		},
		phase: phase,
		travel: pipelineTravelState{
			resumeAfterPrecheckSet: flags.resume != "off",
			resumeAfterPrecheck:    resumeTarget(flags.resume),
		},
		boss: pipelineBossState{bossKillEmitted: flags.kill == 1},
		loot: pipelineLootState{lootScanHasTarget: flags.loot == 1},
	}
}

func originForTransition(foreign int) town.OriginAct {
	if foreign == 1 {
		return town.OriginAct3
	}
	return town.OriginAct1
}

func resumeTarget(resume string) string {
	if resume == pipelineStepPlayRoute {
		return pipelineStepPlayRoute
	}
	return ""
}

func decodeTransitionStep(step string) string {
	if step == "-" {
		return ""
	}
	return step
}

func encodeTransitionStep(step string) string {
	if step == "" {
		return "-"
	}
	return step
}

func walkTransitionSteps(t *testing.T, pipeline *runPipeline, first string) []string {
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

func compareTransitionGolden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name)
	if os.Getenv(updatePipelineTransitionGoldenEnv) == "1" {
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	got = strings.ReplaceAll(got, "\r\n", "\n")
	wantText := strings.ReplaceAll(string(want), "\r\n", "\n")
	if got != wantText {
		t.Fatalf("%s changed:\n--- want\n%s--- got\n%s", name, wantText, got)
	}
}

func mustDefinition(t *testing.T, id RunID) RunDefinition {
	t.Helper()
	definition, ok := DefaultRunRegistry().Definition(id)
	if !ok {
		t.Fatalf("definition %q missing", id)
	}
	return definition
}

func declaredTransitionFamilies() []string {
	return []string{
		"full/not-a-phase scan_loot→pick_loot",
		"full/not-a-phase wait_origin_town→play_town_egress",
		"full/not-a-phase acquire_boss→reposition_for_loot",
		"full/not-a-phase acquire_boss→clear_nearby_hostiles",
		"full/not-a-phase engage_boss→clear_nearby_hostiles",
		"loot-and-return scan_loot→pick_loot",
		"loot-and-return wait_origin_town→play_town_egress",
		"retry-return wait_origin_town→play_town_egress",
		"travel precheck→play_bound_route",
		"travel precheck→-",
	}
}

func declaredTransitionFamily(phase, current string, flags transitionFlags, next string) (string, bool) {
	fullLike := phase == "full" || phase == "not-a-phase"
	travel := phase == RunPhaseTravelEntry || phase == RunPhasePlayRoute
	switch {
	case fullLike && current == pipelineStepScanLoot && flags.loot == 1 && next == pipelineStepPickLoot:
		return "full/not-a-phase scan_loot→pick_loot", true
	case fullLike && current == pipelineStepWaitOriginTown && flags.foreign == 1 && next == pipelineStepPlayTownEgress:
		return "full/not-a-phase wait_origin_town→play_town_egress", true
	case fullLike && current == pipelineStepAcquireBoss && flags.kill == 1 && flags.clear == 0 && next == pipelineStepRepositionForLoot:
		return "full/not-a-phase acquire_boss→reposition_for_loot", true
	case fullLike && current == pipelineStepAcquireBoss && flags.kill == 1 && flags.clear == 1 && next == pipelineStepClearNearbyHostiles:
		return "full/not-a-phase acquire_boss→clear_nearby_hostiles", true
	case fullLike && current == pipelineStepEngageBoss && flags.clear == 1 && next == pipelineStepClearNearbyHostiles:
		return "full/not-a-phase engage_boss→clear_nearby_hostiles", true
	case phase == RunPhaseLootAndReturn && current == pipelineStepScanLoot && flags.loot == 1 && next == pipelineStepPickLoot:
		return "loot-and-return scan_loot→pick_loot", true
	case phase == RunPhaseLootAndReturn && current == pipelineStepWaitOriginTown && flags.foreign == 1 && next == pipelineStepPlayTownEgress:
		return "loot-and-return wait_origin_town→play_town_egress", true
	case phase == RunPhaseRetryReturn && current == pipelineStepWaitOriginTown && flags.foreign == 1 && next == pipelineStepPlayTownEgress:
		return "retry-return wait_origin_town→play_town_egress", true
	case travel && current == pipelineStepPrecheck && flags.resume == pipelineStepPlayRoute && next == pipelineStepPlayRoute:
		return "travel precheck→play_bound_route", true
	case travel && current == pipelineStepPrecheck && flags.resume == "empty" && next == "-":
		return "travel precheck→-", true
	default:
		return "", false
	}
}
