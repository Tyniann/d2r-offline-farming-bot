package telemetry

import (
	"math"
	"sort"
	"testing"
	"time"
)

type analyzerItemFixture struct {
	unitID                             uint32
	key, name, action                  string
	seen, matched, picked, stash, sold bool
}

func TestHistoryAnalyzerMatchesHandCalculatedFarmingMatrix(t *testing.T) {
	start := time.Date(2026, 7, 20, 10, 0, 0, 0, time.UTC)
	runs := []HistoryRun{
		analyzerRun("countess-a", start, 60, HistoryOutcomeSuccess, "countess", "countess-route-a", true, [4]int64{10, 10, 20, 15}, "", "", []analyzerItemFixture{
			{unitID: 101, key: "base:r01:normal", name: "El Rune", action: "keep", seen: true, matched: true, picked: true, stash: true},
			{unitID: 102, key: "unique:The Gavel of Pain", name: "The Gavel of Pain", action: "sell", seen: true, matched: true, picked: true, sold: true},
		}),
		analyzerRun("countess-failed", start.Add(time.Hour), 120, HistoryOutcomeFailed, "countess", "countess-route-a", false, [4]int64{80, 0, 0, 0}, "acquire_boss", "boss_not_found", nil),
		analyzerRun("countess-route-b", start.Add(2*time.Hour), 90, HistoryOutcomeSuccess, "countess", "countess-route-b", true, [4]int64{30, 20, 20, 15}, "", "", []analyzerItemFixture{
			{unitID: 201, key: "base:r02:normal", name: "Eld Rune", action: "keep", seen: true, matched: true, picked: true, stash: true},
		}),
		analyzerRun("mephisto-failed", start.Add(3*time.Hour), 60, HistoryOutcomeFailed, "mephisto", "mephisto-route-a", true, [4]int64{20, 20, 10, 5}, "prepare_town_handoff", "town_timeout", []analyzerItemFixture{
			{unitID: 301, key: "base:r03:normal", name: "Tir Rune", action: "keep", seen: true, matched: true},
			{unitID: 302, key: "unique:Skin of the Vipermagi", name: "Skin of the Vipermagi", action: "keep", seen: true, matched: true, picked: true},
		}),
		analyzerRun("mephisto-success", start.Add(4*time.Hour), 30, HistoryOutcomeSuccess, "mephisto", "mephisto-route-a", true, [4]int64{10, 5, 5, 5}, "", "", []analyzerItemFixture{
			{unitID: 401, key: "unique:Harlequin Crest", name: "Harlequin Crest", action: "keep", seen: true, matched: true, picked: true, stash: true},
		}),
		analyzerRun("active", start.Add(5*time.Hour), 500, HistoryOutcomeRunning, "countess", "countess-route-a", true, [4]int64{100, 50, 0, 0}, "", "", []analyzerItemFixture{{unitID: 501, key: "base:r04:normal", action: "keep", matched: true, picked: true, stash: true}}),
		analyzerRun("incomplete", start.Add(6*time.Hour), 400, HistoryOutcomeIncomplete, "countess", "countess-route-a", true, [4]int64{100, 50, 0, 0}, "", "", nil),
		analyzerRun("diagnostic", start.Add(7*time.Hour), 10, HistoryOutcomeSuccess, "countess", "diagnostic-route", true, [4]int64{}, "", "", []analyzerItemFixture{{unitID: 601, key: "base:r05:normal", action: "keep", matched: true, picked: true, stash: true}}),
	}
	runs[len(runs)-1].Mode = HistoryModeDiagnostic
	analysis, err := AnalyzeHistory(HistorySnapshot{Runs: runs}, HistoryFilter{})
	if err != nil {
		t.Fatal(err)
	}
	summary := analysis.Summary
	if summary.Runs != 7 || summary.TerminalRuns != 5 || summary.Successful != 3 || summary.Failed != 2 || summary.Aborted != 0 || summary.Running != 1 || summary.Incomplete != 1 || summary.BossKills != 4 {
		t.Fatalf("summary=%+v", summary)
	}
	assertHistoryFloat(t, "success rate", summary.SuccessRate, 0.6)
	assertHistoryFloat(t, "summary keep/run", summary.KeepPerRun, 0.6)
	assertHistoryFloat(t, "summary keep/kill", summary.KeepPerKill, 0.75)
	assertHistoryFloat(t, "summary keep/hour", summary.KeepPerHour, 30)
	if summary.Durations.TotalMs != 360_000 || summary.Durations.AverageMs != 72_000 || summary.Durations.MedianMs != 60_000 || summary.Durations.MinimumMs != 30_000 || summary.Durations.MaximumMs != 120_000 {
		t.Fatalf("durations=%+v", summary.Durations)
	}
	if summary.Funnel.KeepReturn != 3 || summary.Funnel.Sold != 1 || summary.Funnel.PickupLost != 1 || summary.Funnel.PostPickupLost != 1 {
		t.Fatalf("funnel=%+v", summary.Funnel)
	}
	if summary.Stages != (HistoryStageDurations{TravelMs: 150_000, CombatMs: 55_000, LootMs: 55_000, ReturnTownMs: 40_000, OtherMs: 60_000}) {
		t.Fatalf("stages=%+v", summary.Stages)
	}
	if summary.TopFailure == nil || summary.TopFailure.Step != "acquire_boss" || summary.TopFailure.Reason != "boss_not_found" || summary.TopFailure.Count != 1 {
		t.Fatalf("top failure=%+v", summary.TopFailure)
	}
	if len(analysis.Comparisons) != 3 {
		t.Fatalf("comparisons=%+v", analysis.Comparisons)
	}
	comparisons := comparisonByRoute(analysis.Comparisons)
	assertHistoryFloat(t, "unstable route keep/hour", comparisons["countess-route-a"].KeepPerHour, 20)
	assertHistoryFloat(t, "route-b keep/hour", comparisons["countess-route-b"].KeepPerHour, 40)
	assertHistoryFloat(t, "mephisto keep/hour", comparisons["mephisto-route-a"].KeepPerHour, 40)
	if !comparisons["countess-route-a"].LowSample || comparisons["countess-route-a"].TerminalRuns != 2 || comparisons["countess-route-a"].BossKills != 1 {
		t.Fatalf("countess route-a=%+v", comparisons["countess-route-a"])
	}
	if len(analysis.Items) != 6 {
		t.Fatalf("items=%+v", analysis.Items)
	}
	shako := historyItemByKey(analysis.Items, "unique:Harlequin Crest")
	assertHistoryFloat(t, "shako/run", shako.YieldPerRun, 0.2)
	assertHistoryFloat(t, "shako/kill", shako.YieldPerKill, 0.25)
	assertHistoryFloat(t, "shako/hour", shako.YieldPerHour, 10)
}

func TestHistoryAnalyzerFiltersPartitionAndDoNotMutateInput(t *testing.T) {
	start := time.Date(2026, 7, 20, 10, 0, 0, 0, time.UTC)
	runs := []HistoryRun{
		analyzerRun("a", start, 10, HistoryOutcomeSuccess, "countess", "route-a", true, [4]int64{}, "", "", nil),
		analyzerRun("b", start.Add(time.Hour), 20, HistoryOutcomeFailed, "mephisto", "route-b", false, [4]int64{}, "boss", "failed", nil),
	}
	from, to := start, start.Add(30*time.Minute)
	filter := HistoryFilter{FromUTC: &from, ToUTC: &to, Runs: []string{"countess"}, Characters: []string{"MrBones"}, Difficulties: []string{"nightmare"}, Outcomes: []HistoryOutcome{HistoryOutcomeSuccess}}
	analysis, err := AnalyzeHistory(HistorySnapshot{Runs: runs}, filter)
	if err != nil || analysis.Summary.TerminalRuns != 1 || analysis.Runs[0].RunID != "a" {
		t.Fatalf("analysis=%+v err=%v", analysis, err)
	}
	filter.Runs[0] = "mutated"
	if analysis.Filter.Runs[0] != "countess" {
		t.Fatal("analysis retained caller-owned filter slice")
	}
	failed, err := AnalyzeHistory(HistorySnapshot{Runs: runs}, HistoryFilter{Outcomes: []HistoryOutcome{HistoryOutcomeFailed}})
	if err != nil || failed.Summary.TerminalRuns != 1 || analysis.Summary.TerminalRuns+failed.Summary.TerminalRuns != 2 {
		t.Fatalf("filter partitions do not sum: success=%+v failed=%+v err=%v", analysis.Summary, failed.Summary, err)
	}
	badTo := start
	if _, err := AnalyzeHistory(HistorySnapshot{}, HistoryFilter{FromUTC: &start, ToUTC: &badTo}); historyErrorCode(err) != HistoryReasonFilterInvalid {
		t.Fatalf("invalid interval err=%v", err)
	}
}

func TestHistoryAnalyzerFiltersBySessionID(t *testing.T) {
	start := time.Date(2026, 7, 20, 10, 0, 0, 0, time.UTC)
	keep := analyzerRun("keep-a", start, 60, HistoryOutcomeSuccess, "countess", "route-a", true, [4]int64{}, "", "", []analyzerItemFixture{
		{unitID: 101, key: "base:r15:normal", name: "Ko Rune", action: "keep", seen: true, matched: true, picked: true, stash: true},
		{unitID: 102, key: "base:r15:normal", name: "Ko Rune", action: "keep", seen: true, matched: true, picked: true, stash: true},
	})
	keep.SessionID = "session-keep"
	sold := analyzerRun("sold-b", start.Add(time.Hour), 30, HistoryOutcomeSuccess, "countess", "route-a", true, [4]int64{}, "", "", []analyzerItemFixture{
		{unitID: 201, key: "base:gld:normal", name: "Gold", action: "sell", seen: true, matched: true, picked: true, sold: true},
	})
	sold.SessionID = "session-other"
	analysis, err := AnalyzeHistory(HistorySnapshot{Runs: []HistoryRun{keep, sold}}, HistoryFilter{SessionIDs: []string{"session-keep"}})
	if err != nil {
		t.Fatal(err)
	}
	if analysis.Summary.TerminalRuns != 1 || analysis.Runs[0].RunID != "keep-a" || analysis.Summary.Funnel.KeepReturn != 2 || analysis.Summary.Funnel.Sold != 0 {
		t.Fatalf("session filter summary=%+v runs=%+v", analysis.Summary, analysis.Runs)
	}
	if len(analysis.Items) != 1 || analysis.Items[0].ItemKey != "base:r15:normal" || analysis.Items[0].Stashed != 2 {
		t.Fatalf("session items=%+v", analysis.Items)
	}
}

func TestHistoryAnalyzerAppliesServerComparisonSortWithStableTieBreak(t *testing.T) {
	start := time.Date(2026, 7, 20, 10, 0, 0, 0, time.UTC)
	runs := []HistoryRun{
		analyzerRun("slow", start, 120, HistoryOutcomeSuccess, "countess", "route-slow", true, [4]int64{}, "", "", nil),
		analyzerRun("fast", start.Add(time.Hour), 60, HistoryOutcomeFailed, "countess", "route-fast", true, [4]int64{}, "boss", "failed", nil),
	}
	analysis, err := AnalyzeHistory(HistorySnapshot{Runs: runs}, HistoryFilter{Sort: HistorySortAverageDuration})
	if err != nil {
		t.Fatal(err)
	}
	if len(analysis.Comparisons) != 2 || analysis.Comparisons[0].RouteID != "route-fast" {
		t.Fatalf("average-duration comparisons=%+v", analysis.Comparisons)
	}
	analysis, err = AnalyzeHistory(HistorySnapshot{Runs: runs}, HistoryFilter{Sort: HistorySortSuccessRate})
	if err != nil {
		t.Fatal(err)
	}
	if analysis.Comparisons[0].RouteID != "route-slow" {
		t.Fatalf("success-rate comparisons=%+v", analysis.Comparisons)
	}
}

func TestHistoryAnalyzerHandlesZeroDurationTiesAndSampleBoundary(t *testing.T) {
	start := time.Date(2026, 7, 20, 10, 0, 0, 0, time.UTC)
	runs := make([]HistoryRun, 0, 10)
	for index := 0; index < 10; index++ {
		runs = append(runs, analyzerRun(historyRunID(index), start, 0, HistoryOutcomeSuccess, "countess", "route-a", true, [4]int64{}, "", "", nil))
	}
	analysis, err := AnalyzeHistory(HistorySnapshot{Runs: runs}, HistoryFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(analysis.Comparisons) != 1 || analysis.Comparisons[0].LowSample || analysis.Comparisons[0].KeepPerHour != nil || analysis.Summary.Durations.MedianMs != 0 {
		t.Fatalf("zero/sample comparison=%+v summary=%+v", analysis.Comparisons, analysis.Summary)
	}
	runs = runs[:9]
	analysis, _ = AnalyzeHistory(HistorySnapshot{Runs: runs}, HistoryFilter{})
	if !analysis.Comparisons[0].LowSample {
		t.Fatal("nine boss kills were not marked as a low sample")
	}
}

func TestHistoryAnalyzerIncludesFailedAndAbortedTimeInHourlyDenominator(t *testing.T) {
	start := time.Date(2026, 7, 20, 14, 0, 0, 0, time.UTC)
	runs := []HistoryRun{
		analyzerRun("success", start, 60, HistoryOutcomeSuccess, "countess", "same-route", true, [4]int64{20, 10, 10, 10}, "", "", []analyzerItemFixture{{unitID: 1, key: "base:r01:normal", action: "keep", matched: true, picked: true, stash: true}}),
		analyzerRun("failed", start.Add(time.Hour), 60, HistoryOutcomeFailed, "countess", "same-route", false, [4]int64{40, 0, 0, 0}, "precheck", "failed", nil),
		analyzerRun("aborted", start.Add(2*time.Hour), 60, HistoryOutcomeAborted, "countess", "same-route", false, [4]int64{40, 0, 0, 0}, "precheck", "stopped", nil),
	}
	analysis, err := AnalyzeHistory(HistorySnapshot{Runs: runs}, HistoryFilter{})
	if err != nil {
		t.Fatal(err)
	}
	comparison := analysis.Comparisons[0]
	assertHistoryFloat(t, "terminal denominator success", comparison.SuccessRate, 1.0/3.0)
	assertHistoryFloat(t, "failed/aborted hourly denominator", comparison.KeepPerHour, 20)
	if comparison.Durations.TotalMs != 180_000 || comparison.Aborted != 1 || comparison.Failed != 1 {
		t.Fatalf("comparison=%+v", comparison)
	}
}

func TestHistoryDailyBucketsUseLocalCalendarDaysAndPreserveZeroDays(t *testing.T) {
	from := time.Date(2026, 7, 19, 22, 0, 0, 0, time.UTC)
	to := time.Date(2026, 7, 22, 22, 0, 0, 0, time.UTC)
	runs := []HistoryRun{
		analyzerRun("day-one", time.Date(2026, 7, 20, 21, 59, 0, 0, time.UTC), 60, HistoryOutcomeSuccess, "countess", "route-a", true, [4]int64{}, "", "", []analyzerItemFixture{{unitID: 1, key: "r01", action: "keep", matched: true, picked: true, stash: true}}),
		analyzerRun("day-three", time.Date(2026, 7, 21, 22, 0, 0, 0, time.UTC), 120, HistoryOutcomeFailed, "countess", "route-a", false, [4]int64{}, "boss", "boss_not_found", nil),
	}
	analysis, err := AnalyzeHistory(HistorySnapshot{Runs: runs}, HistoryFilter{FromUTC: &from, ToUTC: &to, Timezone: "Europe/Vienna"})
	if err != nil {
		t.Fatal(err)
	}
	if len(analysis.DailyBuckets) != 3 {
		t.Fatalf("buckets=%+v", analysis.DailyBuckets)
	}
	first, zero, last := analysis.DailyBuckets[0], analysis.DailyBuckets[1], analysis.DailyBuckets[2]
	if first.Date != "2026-07-20" || first.TerminalRuns != 1 || first.Successful != 1 || first.KeepReturn != 1 || first.ActiveDurationMs != 60_000 {
		t.Fatalf("first=%+v", first)
	}
	assertHistoryFloat(t, "daily success", first.SuccessRate, 1)
	assertHistoryFloat(t, "daily keep/hour", first.KeepPerHour, 60)
	if zero.Date != "2026-07-21" || zero.TerminalRuns != 0 || zero.SuccessRate != nil || zero.KeepPerHour != nil {
		t.Fatalf("zero=%+v", zero)
	}
	if last.Date != "2026-07-22" || last.TerminalRuns != 1 || last.Successful != 0 || last.ActiveDurationMs != 120_000 {
		t.Fatalf("last=%+v", last)
	}
}

func TestHistoryDailyBucketsExposeDSTDayBoundaries(t *testing.T) {
	tests := []struct {
		name  string
		from  time.Time
		to    time.Time
		hours time.Duration
	}{
		{name: "23 hours", from: time.Date(2026, 3, 28, 23, 0, 0, 0, time.UTC), to: time.Date(2026, 3, 29, 22, 0, 0, 0, time.UTC), hours: 23 * time.Hour},
		{name: "25 hours", from: time.Date(2026, 10, 24, 22, 0, 0, 0, time.UTC), to: time.Date(2026, 10, 25, 23, 0, 0, 0, time.UTC), hours: 25 * time.Hour},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			analysis, err := AnalyzeHistory(HistorySnapshot{}, HistoryFilter{FromUTC: &test.from, ToUTC: &test.to, Timezone: "Europe/Vienna"})
			if err != nil {
				t.Fatal(err)
			}
			if len(analysis.DailyBuckets) != 1 || analysis.DailyBuckets[0].EndUTC.Sub(analysis.DailyBuckets[0].StartUTC) != test.hours {
				t.Fatalf("buckets=%+v", analysis.DailyBuckets)
			}
		})
	}
}

func TestHistoryTimezoneValidationAndHalfOpenUTCFilter(t *testing.T) {
	start := time.Date(2026, 7, 20, 10, 0, 0, 0, time.UTC)
	to := start.Add(time.Hour)
	runs := []HistoryRun{
		analyzerRun("inside", start, 10, HistoryOutcomeSuccess, "countess", "route", true, [4]int64{}, "", "", nil),
		analyzerRun("excluded-at-to", to, 10, HistoryOutcomeSuccess, "countess", "route", true, [4]int64{}, "", "", nil),
	}
	analysis, err := AnalyzeHistory(HistorySnapshot{Runs: runs}, HistoryFilter{FromUTC: &start, ToUTC: &to, Timezone: "UTC"})
	if err != nil || analysis.Summary.TerminalRuns != 1 || analysis.Runs[0].RunID != "inside" {
		t.Fatalf("analysis=%+v err=%v", analysis, err)
	}
	if _, err := AnalyzeHistory(HistorySnapshot{}, HistoryFilter{Timezone: "Europe/Does-Not-Exist"}); historyErrorCode(err) != HistoryReasonTimezoneInvalid {
		t.Fatalf("timezone err=%v", err)
	}
}

func TestAnalyzeHistoryIsolatesTerminalRunWithOpenStep(t *testing.T) {
	start := time.Date(2026, 7, 20, 10, 0, 0, 0, time.UTC)
	good := analyzerRun("good", start, 30, HistoryOutcomeSuccess, "countess", "route-a", true, [4]int64{10, 5, 5, 5}, "", "", nil)
	poison := analyzerRun("poison", start.Add(time.Hour), 30, HistoryOutcomeAborted, "countess", "route-a", false, [4]int64{}, "", "emergency_stop_requested", nil)
	poison.RunFile = "poison.jsonl"
	poison.Events = append(poison.Events, Event{
		Timestamp: start.Add(time.Hour + 5*time.Second),
		Event:     RunStepStarted,
		Step:      "enter_town_portal",
		Stage:     HistoryStageReturnTown,
	})
	analysis, err := AnalyzeHistory(HistorySnapshot{Runs: []HistoryRun{good, poison}}, HistoryFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(analysis.Runs) != 1 || analysis.Runs[0].RunID != "good" {
		t.Fatalf("runs=%+v", analysis.Runs)
	}
	if len(analysis.Diagnostics) != 1 || analysis.Diagnostics[0].File != "poison.jsonl" || analysis.Diagnostics[0].Code != HistoryReasonStageInvalid {
		t.Fatalf("diagnostics=%+v", analysis.Diagnostics)
	}
	if analysis.Summary.Successful != 1 || analysis.Summary.TerminalRuns != 1 {
		t.Fatalf("summary=%+v", analysis.Summary)
	}
}

func TestAnalyzeHistoryStillAcceptsIncompleteWithoutEndedAt(t *testing.T) {
	start := time.Date(2026, 7, 20, 10, 0, 0, 0, time.UTC)
	incomplete := analyzerRun("incomplete-open", start, 40, HistoryOutcomeIncomplete, "countess", "route-a", false, [4]int64{10, 0, 0, 0}, "", "", nil)
	// Leave an open step by appending a started step without terminal after closed stages.
	incomplete.Events = append(incomplete.Events, Event{
		Timestamp: start.Add(15 * time.Second),
		Event:     RunStepStarted,
		Step:      "engage_boss",
		Stage:     HistoryStageCombat,
	})
	analysis, err := AnalyzeHistory(HistorySnapshot{Runs: []HistoryRun{incomplete}}, HistoryFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(analysis.Runs) != 1 || analysis.Runs[0].RunID != "incomplete-open" || analysis.Summary.Incomplete != 1 {
		t.Fatalf("analysis=%+v", analysis)
	}
	if len(analysis.Diagnostics) != 0 {
		t.Fatalf("diagnostics=%+v, want none", analysis.Diagnostics)
	}
}

func TestAnalyzeHistoryStillRejectsInvalidFilter(t *testing.T) {
	start := time.Date(2026, 7, 20, 10, 0, 0, 0, time.UTC)
	to := start
	if _, err := AnalyzeHistory(HistorySnapshot{}, HistoryFilter{FromUTC: &start, ToUTC: &to}); historyErrorCode(err) != HistoryReasonFilterInvalid {
		t.Fatalf("filter err=%v", err)
	}
	if _, err := AnalyzeHistory(HistorySnapshot{}, HistoryFilter{Timezone: "Europe/Does-Not-Exist"}); historyErrorCode(err) != HistoryReasonTimezoneInvalid {
		t.Fatalf("timezone err=%v", err)
	}
}

func analyzerRun(id string, start time.Time, durationSeconds int64, outcome HistoryOutcome, runName, route string, boss bool, stageSeconds [4]int64, failedStep, reason string, items []analyzerItemFixture) HistoryRun {
	end := start.Add(time.Duration(durationSeconds) * time.Second)
	run := HistoryRun{
		RunID: id, SessionID: "session", GameID: "game", Mode: HistoryModeProductiveFarming,
		Character: "MrBones", Difficulty: "nightmare", GameVersion: "3.2.92777", Run: runName,
		DefinitionID: runName, RouteID: route, StartedAt: start, ObservedAt: end, Outcome: outcome, Reason: reason,
	}
	if terminalHistoryOutcome(outcome) {
		run.EndedAt = &end
	}
	cursor := start
	stages := []HistoryStage{HistoryStageTravel, HistoryStageCombat, HistoryStageLoot, HistoryStageReturnTown}
	for index, seconds := range stageSeconds {
		if seconds == 0 {
			continue
		}
		step := string(stages[index])
		terminal := RunStepCompleted
		if failedStep != "" && index == lastNonZeroStage(stageSeconds) {
			step, terminal = failedStep, RunStepFailed
		}
		run.Events = append(run.Events, Event{Timestamp: cursor, Event: RunStepStarted, Step: step, Stage: stages[index]})
		cursor = cursor.Add(time.Duration(seconds) * time.Second)
		run.Events = append(run.Events, Event{Timestamp: cursor, Event: terminal, Step: step, Stage: stages[index]})
	}
	if boss {
		eventTime := start
		if durationSeconds > 0 {
			eventTime = start.Add(time.Millisecond)
		}
		run.Events = append(run.Events, Event{Timestamp: eventTime, Event: BossKillConfirmed, UnitID: 9000, BossID: runName, Stage: HistoryStageCombat})
	}
	for _, item := range items {
		eventTime := start
		if durationSeconds > 0 {
			eventTime = start.Add(2 * time.Millisecond)
		}
		base := Event{Timestamp: eventTime, UnitID: item.unitID, ItemKey: item.key, ItemName: item.name, PickitAction: item.action, PickitProfileID: runName + "-standard", PickitRuleID: "fixture", PickitProfileRevision: 2, PickitAssignmentRevision: 4}
		if item.seen {
			event := base
			event.Event, event.Stage = DropSeen, HistoryStageLoot
			run.Events = append(run.Events, event)
		}
		if item.matched {
			event := base
			event.Event, event.Stage = PickitMatch, HistoryStageLoot
			run.Events = append(run.Events, event)
		}
		if item.picked {
			event := base
			event.Event, event.Stage = PickupSuccess, HistoryStageLoot
			run.Events = append(run.Events, event)
		}
		if item.stash {
			event := base
			event.Event, event.Stage = StashSuccess, HistoryStageReturnTown
			run.Events = append(run.Events, event)
		}
		if item.sold {
			event := base
			event.Event, event.Stage = SellSuccess, HistoryStageReturnTown
			run.Events = append(run.Events, event)
		}
	}
	sort.SliceStable(run.Events, func(a, b int) bool { return run.Events[a].Timestamp.Before(run.Events[b].Timestamp) })
	return run
}

func lastNonZeroStage(stages [4]int64) int {
	for index := len(stages) - 1; index >= 0; index-- {
		if stages[index] != 0 {
			return index
		}
	}
	return -1
}

func comparisonByRoute(comparisons []HistoryComparison) map[string]HistoryComparison {
	out := make(map[string]HistoryComparison, len(comparisons))
	for _, comparison := range comparisons {
		out[comparison.RouteID] = comparison
	}
	return out
}

func historyItemByKey(items []HistoryItemAggregate, key string) HistoryItemAggregate {
	for _, item := range items {
		if item.ItemKey == key {
			return item
		}
	}
	return HistoryItemAggregate{}
}

func assertHistoryFloat(t *testing.T, name string, got *float64, want float64) {
	t.Helper()
	if got == nil || math.Abs(*got-want) > 1e-9 {
		t.Fatalf("%s=%v want=%v", name, got, want)
	}
}

func historyRunID(index int) string {
	return "sample-" + time.Unix(int64(index), 0).UTC().Format("150405")
}
