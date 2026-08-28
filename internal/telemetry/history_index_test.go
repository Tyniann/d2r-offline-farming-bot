package telemetry

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestHistoryIndexCorrelatesTerminalIncompleteAndActiveRuns(t *testing.T) {
	directory := t.TempDir()
	started := time.Date(2026, 7, 22, 8, 0, 0, 0, time.UTC)
	writeHistoryRun(t, directory, "countess-a", started, RunCompleted)
	writeHistoryRun(t, directory, "mephisto-active", started.Add(time.Hour), "")
	if err := os.WriteFile(filepath.Join(directory, "legacy.jsonl"), []byte("{\"schema_version\":2}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "corrupt.jsonl"), []byte("{not-json}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	index, err := NewHistoryIndex(directory)
	if err != nil {
		t.Fatal(err)
	}
	if err := index.Refresh(); err != nil {
		t.Fatal(err)
	}
	snapshot := index.Snapshot("mephisto-active")
	if len(snapshot.Runs) != 2 || snapshot.Runs[0].Outcome != HistoryOutcomeSuccess || snapshot.Runs[1].Outcome != HistoryOutcomeRunning || snapshot.IgnoredFiles != 1 {
		t.Fatalf("snapshot=%+v", snapshot)
	}
	if len(snapshot.Runs[0].Events) < 3 || snapshot.Runs[0].Events[0].Event != RunStarted {
		t.Fatalf("correlated events=%+v", snapshot.Runs[0].Events)
	}
	if len(snapshot.Diagnostics) != 1 || snapshot.Diagnostics[0].File != "corrupt.jsonl" || snapshot.Diagnostics[0].Code != HistoryReasonFileInvalid {
		t.Fatalf("diagnostics=%+v", snapshot.Diagnostics)
	}
	if got := index.Snapshot("").Runs[1].Outcome; got != HistoryOutcomeIncomplete {
		t.Fatalf("inactive crash outcome=%q", got)
	}
	// Returned event slices must never mutate the index authority.
	snapshot.Runs[0].Events[0].RunID = "mutated"
	if got := index.Snapshot("").Runs[0].Events[0].RunID; got == "mutated" {
		t.Fatal("snapshot mutated cached history")
	}
}

func TestHistoryIndexIncrementalContentProofMatchesCleanRebuild(t *testing.T) {
	directory := t.TempDir()
	writeHistoryRun(t, directory, "countess-a", time.Date(2026, 7, 22, 9, 0, 0, 0, time.UTC), RunFailed)
	index, _ := NewHistoryIndex(directory)
	if err := index.Refresh(); err != nil {
		t.Fatal(err)
	}
	first := index.Snapshot("")
	if err := index.Refresh(); err != nil {
		t.Fatal(err)
	}
	if got := index.Snapshot("").Generation; got != first.Generation {
		t.Fatalf("unchanged refresh generation=%d want=%d", got, first.Generation)
	}
	runPath := filepath.Join(directory, "countess-a.jsonl")
	info, _ := os.Stat(runPath)
	data, _ := os.ReadFile(runPath)
	changed := strings.ReplaceAll(string(data), "route-a", "route-b")
	if len(changed) != len(data) {
		t.Fatal("same-size mutation setup failed")
	}
	if err := os.WriteFile(runPath, []byte(changed), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(runPath, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
	if err := index.Refresh(); err != nil {
		t.Fatal(err)
	}
	incremental := index.Snapshot("")
	if len(incremental.Runs) != 1 || incremental.Runs[0].RouteID != "route-b" || incremental.Generation == first.Generation {
		t.Fatalf("incremental=%+v", incremental)
	}
	clean, _ := NewHistoryIndex(directory)
	if err := clean.Rebuild(); err != nil {
		t.Fatal(err)
	}
	rebuilt := clean.Snapshot("")
	if historyRunDigest(incremental.Runs) != historyRunDigest(rebuilt.Runs) || fmt.Sprint(incremental.Diagnostics) != fmt.Sprint(rebuilt.Diagnostics) {
		t.Fatalf("incremental and rebuild differ:\ninc=%+v\nrebuild=%+v", incremental, rebuilt)
	}
}

func TestHistoryIndexRetainsLastStableVersionDuringPartialWrite(t *testing.T) {
	directory := t.TempDir()
	writeHistoryRun(t, directory, "partial-a", time.Date(2026, 7, 22, 9, 30, 0, 0, time.UTC), RunCompleted)
	index, _ := NewHistoryIndex(directory)
	if err := index.Refresh(); err != nil {
		t.Fatal(err)
	}
	stable := index.Snapshot("")
	path := filepath.Join(directory, "partial-a.jsonl")
	data, _ := os.ReadFile(path)
	changed := strings.ReplaceAll(string(data), "route-a", "route-b")
	if err := os.WriteFile(path, []byte(strings.TrimSuffix(changed, "\n")), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := index.Refresh(); err != nil {
		t.Fatal(err)
	}
	duringWrite := index.Snapshot("")
	if len(duringWrite.Runs) != 1 || duringWrite.Runs[0].RouteID != "route-a" || duringWrite.Generation != stable.Generation || len(duringWrite.Diagnostics) != 0 {
		t.Fatalf("partial write replaced stable version: %+v", duringWrite)
	}
	if err := os.WriteFile(path, []byte(changed), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := index.Refresh(); err != nil {
		t.Fatal(err)
	}
	completed := index.Snapshot("")
	if len(completed.Runs) != 1 || completed.Runs[0].RouteID != "route-b" || completed.Generation == stable.Generation {
		t.Fatalf("completed write not indexed: %+v", completed)
	}
}

func TestHistoryIndexIsolatesOversizedAndChangedFiles(t *testing.T) {
	directory := t.TempDir()
	writeHistoryRun(t, directory, "valid-a", time.Date(2026, 7, 22, 10, 0, 0, 0, time.UTC), RunCompleted)
	oversized := append([]byte(`{"schema_version":3,"stream":"run","timestamp":"2026-07-22T10:00:00Z","event":"run_context","run_id":"huge"}`), make([]byte, HistoryMaximumLineBytes)...)
	oversized = append(oversized, '\n')
	if err := os.WriteFile(filepath.Join(directory, "huge.jsonl"), oversized, 0o644); err != nil {
		t.Fatal(err)
	}
	index, _ := NewHistoryIndex(directory)
	if err := index.Refresh(); err != nil {
		t.Fatal(err)
	}
	snapshot := index.Snapshot("")
	if len(snapshot.Runs) != 1 || !hasHistoryDiagnostic(snapshot.Diagnostics, "huge.jsonl", HistoryReasonLineTooLarge) {
		t.Fatalf("snapshot=%+v", snapshot)
	}
	if err := os.WriteFile(filepath.Join(directory, "valid-a.jsonl"), []byte("{broken}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := index.Refresh(); err != nil {
		t.Fatal(err)
	}
	snapshot = index.Snapshot("")
	if len(snapshot.Runs) != 0 || !hasHistoryDiagnostic(snapshot.Diagnostics, "valid-a.jsonl", HistoryReasonFileInvalid) {
		t.Fatalf("changed file was partially retained: %+v", snapshot)
	}
}

func TestHistoryIndexConcurrentRefreshAndSnapshotsWithLiveWriter(t *testing.T) {
	directory := t.TempDir()
	started := time.Date(2026, 7, 22, 11, 0, 0, 0, time.UTC)
	session, run := openHistoryRun(t, directory, "live-a", started)
	defer session.Close()
	defer run.Close()
	index, _ := NewHistoryIndex(directory)
	if err := index.Refresh(); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for n := 0; n < 100; n++ {
			name := RunStepStarted
			if n%2 == 1 {
				name = RunStepCompleted
			}
			_ = run.Emit(Event{Timestamp: started.Add(time.Duration(n+2) * time.Millisecond), Event: name, Step: "precheck", Stage: HistoryStageTravel})
		}
	}()
	go func() {
		defer wg.Done()
		for n := 0; n < 100; n++ {
			_ = index.Refresh()
			_ = index.Snapshot("live-a")
		}
	}()
	wg.Wait()
	terminalAt := started.Add(time.Second)
	if err := session.Emit(Event{Timestamp: terminalAt, Event: RunCompleted, RunID: "live-a", GameID: "game-live-a", Run: "countess"}); err != nil {
		t.Fatal(err)
	}
	if err := index.Refresh(); err != nil {
		t.Fatal(err)
	}
	snapshot := index.Snapshot("")
	if len(snapshot.Runs) != 1 || snapshot.Runs[0].Outcome != HistoryOutcomeSuccess || len(snapshot.Diagnostics) != 0 {
		t.Fatalf("live snapshot=%+v", snapshot)
	}
}

func TestHistoryIndexKeepsSessionWithRecoveryRunIDsAndSessionKeepReturn(t *testing.T) {
	directory := t.TempDir()
	started := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	session, run := openHistoryRun(t, directory, "countess-recovery", started)
	events := []Event{
		{Timestamp: started.Add(time.Second), Event: PickitMatch, Stage: HistoryStageLoot, UnitID: 347, ItemKey: "base:r04:normal", PickitAction: "keep"},
		{Timestamp: started.Add(2 * time.Second), Event: PickupSuccess, Stage: HistoryStageLoot, UnitID: 347, ItemKey: "base:r04:normal", PickitAction: "keep"},
		{Timestamp: started.Add(3 * time.Second), Event: StashSuccess, Stage: HistoryStageReturnTown, UnitID: 347, ItemKey: "base:r04:normal", PickitAction: "keep"},
	}
	for _, event := range events {
		if err := run.Emit(event); err != nil {
			t.Fatal(err)
		}
	}
	recovery := Event{Timestamp: started.Add(5 * time.Second), Event: DirectExitStarted, RunID: "countess-recovery", GameID: "game-countess-recovery", Run: "countess", OriginalReason: "route_transition_failed", RecoveryReason: "town_portal_enter_failed"}
	if err := session.Emit(recovery); err != nil {
		t.Fatal(err)
	}
	recovery.Timestamp = started.Add(6 * time.Second)
	recovery.Event = DirectExitCompleted
	recovery.Confirmed = true
	if err := session.Emit(recovery); err != nil {
		t.Fatal(err)
	}
	if err := session.Emit(Event{Timestamp: started.Add(10 * time.Second), Event: RunAborted, RunID: "countess-recovery", GameID: "game-countess-recovery", Run: "countess", Reason: "retry_current"}); err != nil {
		t.Fatal(err)
	}
	if err := run.Close(); err != nil {
		t.Fatal(err)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
	index, err := NewHistoryIndex(directory)
	if err != nil {
		t.Fatal(err)
	}
	if err = index.Refresh(); err != nil {
		t.Fatal(err)
	}
	snapshot := index.Snapshot("")
	if len(snapshot.Runs) != 1 || snapshot.Runs[0].Outcome != HistoryOutcomeAborted || len(snapshot.Diagnostics) != 0 {
		t.Fatalf("recovery session was not correlated: runs=%+v diagnostics=%+v", snapshot.Runs, snapshot.Diagnostics)
	}
	analysis, err := AnalyzeHistory(snapshot, HistoryFilter{SessionIDs: []string{"session-countess-recovery"}})
	if err != nil {
		t.Fatal(err)
	}
	if analysis.Summary.TerminalRuns != 1 || analysis.Summary.Funnel.KeepReturn != 1 || analysis.Summary.Funnel.PickedUp != 1 {
		t.Fatalf("session summary lost keep return after recovery telemetry: %+v", analysis.Summary)
	}
}

func TestHistoryIndexRejectsGameLifecycleThatLeaksRunID(t *testing.T) {
	directory := t.TempDir()
	started := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	session, run := openHistoryRun(t, directory, "countess-leak", started)
	if err := session.Emit(Event{Timestamp: started.Add(time.Second), Event: GameExited, RunID: "countess-leak", GameID: "game-countess-leak", Run: "countess"}); err != nil {
		t.Fatal(err)
	}
	if err := session.Emit(Event{Timestamp: started.Add(10 * time.Second), Event: RunCompleted, RunID: "countess-leak", GameID: "game-countess-leak", Run: "countess"}); err != nil {
		t.Fatal(err)
	}
	_ = run.Close()
	_ = session.Close()
	index, err := NewHistoryIndex(directory)
	if err != nil {
		t.Fatal(err)
	}
	if err = index.Refresh(); err != nil {
		t.Fatal(err)
	}
	snapshot := index.Snapshot("")
	if len(snapshot.Runs) != 0 || !hasHistoryDiagnostic(snapshot.Diagnostics, "session-countess-leak.jsonl", HistoryReasonEventInvalid) {
		t.Fatalf("game lifecycle run_id leak was not rejected: %+v", snapshot)
	}
}

func TestHistoryIndexKeepsIncompleteItemFunnelsButRejectsIdentityDrift(t *testing.T) {
	directory := t.TempDir()
	started := time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC)
	session, run := openHistoryRun(t, directory, "losses-a", started)
	events := []Event{
		{Timestamp: started.Add(time.Second), Event: DropSeen, Stage: HistoryStageLoot, UnitID: 101, ItemKey: "base:r01:normal"},
		{Timestamp: started.Add(2 * time.Second), Event: PickitMatch, Stage: HistoryStageLoot, UnitID: 101, ItemKey: "base:r01:normal", PickitAction: "keep"},
		{Timestamp: started.Add(3 * time.Second), Event: PickitMatch, Stage: HistoryStageLoot, UnitID: 102, ItemKey: "base:r02:normal", PickitAction: "keep"},
		{Timestamp: started.Add(4 * time.Second), Event: PickupSuccess, Stage: HistoryStageLoot, UnitID: 102, ItemKey: "base:r02:normal", PickitAction: "keep"},
	}
	for _, event := range events {
		if err := run.Emit(event); err != nil {
			t.Fatal(err)
		}
	}
	if err := session.Emit(Event{Timestamp: started.Add(10 * time.Second), Event: RunCompleted, RunID: "losses-a", GameID: "game-losses-a", Run: "countess"}); err != nil {
		t.Fatal(err)
	}
	_ = run.Close()
	_ = session.Close()
	index, _ := NewHistoryIndex(directory)
	if err := index.Refresh(); err != nil {
		t.Fatal(err)
	}
	if snapshot := index.Snapshot(""); len(snapshot.Runs) != 1 || len(snapshot.Diagnostics) != 0 {
		t.Fatalf("missing pickup/stash must remain analyzable: %+v", snapshot)
	}

	driftSession, driftRun := openHistoryRun(t, directory, "drift-a", started.Add(time.Hour))
	_ = driftRun.Emit(Event{Timestamp: started.Add(time.Hour + time.Second), Event: PickitMatch, Stage: HistoryStageLoot, UnitID: 201, ItemKey: "unique:Harlequin Crest", PickitAction: "keep"})
	_ = driftRun.Emit(Event{Timestamp: started.Add(time.Hour + 2*time.Second), Event: PickupSuccess, Stage: HistoryStageLoot, UnitID: 201, ItemKey: "unique:Wrong Item", PickitAction: "keep"})
	_ = driftSession.Emit(Event{Timestamp: started.Add(time.Hour + 10*time.Second), Event: RunFailed, RunID: "drift-a", GameID: "game-drift-a", Run: "countess"})
	_ = driftRun.Close()
	_ = driftSession.Close()
	if err := index.Refresh(); err != nil {
		t.Fatal(err)
	}
	snapshot := index.Snapshot("")
	if len(snapshot.Runs) != 1 || !hasHistoryDiagnostic(snapshot.Diagnostics, "drift-a.jsonl", HistoryReasonItemChainInvalid) {
		t.Fatalf("identity drift was not isolated: %+v", snapshot)
	}
}

func TestHistoryIndexRequiresProductiveCrossStreamAndDeduplicatesBoss(t *testing.T) {
	directory := t.TempDir()
	started := time.Date(2026, 7, 22, 13, 0, 0, 0, time.UTC)
	orphanSession, orphan := openHistoryRun(t, directory, "orphan-a", started)
	// Removing the closed session file simulates a missing lifecycle stream.
	_ = orphanSession.Close()
	if err := os.Remove(filepath.Join(directory, "session-orphan-a.jsonl")); err != nil {
		t.Fatal(err)
	}
	_ = orphan.Close()

	session, run := openHistoryRun(t, directory, "boss-a", started.Add(time.Hour))
	for n := 0; n < 2; n++ {
		if err := run.Emit(Event{Timestamp: started.Add(time.Hour + time.Duration(n+1)*time.Second), Event: BossKillConfirmed, Stage: HistoryStageCombat, UnitID: 9001, BossID: "countess", BossName: "Die Gräfin"}); err != nil {
			t.Fatal(err)
		}
	}
	_ = session.Emit(Event{Timestamp: started.Add(time.Hour + 10*time.Second), Event: RunCompleted, RunID: "boss-a", GameID: "game-boss-a", Run: "countess"})
	_ = run.Close()
	_ = session.Close()
	index, _ := NewHistoryIndex(directory)
	if err := index.Refresh(); err != nil {
		t.Fatal(err)
	}
	snapshot := index.Snapshot("")
	if len(snapshot.Runs) != 0 || !hasHistoryDiagnostic(snapshot.Diagnostics, "orphan-a.jsonl", HistoryReasonStreamMissing) || !hasHistoryDiagnostic(snapshot.Diagnostics, "boss-a.jsonl", HistoryReasonBossDuplicate) {
		t.Fatalf("cross-stream/boss validation=%+v", snapshot)
	}
}

func writeHistoryRun(t *testing.T, directory, runID string, started time.Time, terminal EventName) {
	t.Helper()
	session, run := openHistoryRun(t, directory, runID, started)
	if err := run.Emit(Event{Timestamp: started.Add(time.Second), Event: RunStepStarted, Step: "precheck", Stage: HistoryStageTravel}); err != nil {
		t.Fatal(err)
	}
	if err := run.Emit(Event{Timestamp: started.Add(2 * time.Second), Event: RunStepCompleted, Step: "precheck", Stage: HistoryStageTravel}); err != nil {
		t.Fatal(err)
	}
	if terminal != "" {
		if err := session.Emit(Event{Timestamp: started.Add(10 * time.Second), Event: terminal, RunID: runID, GameID: "game-" + runID, Run: historyRunName(runID), Reason: "fixture"}); err != nil {
			t.Fatal(err)
		}
	}
	if err := run.Close(); err != nil {
		t.Fatal(err)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
}

func openHistoryRun(t *testing.T, directory, runID string, started time.Time) (*SessionRecorder, *Recorder) {
	t.Helper()
	sessionID := "session-" + runID
	runName := historyRunName(runID)
	session, err := NewSessionRecorderWithContext(directory, SessionRecorderContext{SessionID: sessionID, Mode: HistoryModeProductiveFarming, Character: "MrBones", Difficulty: "nightmare", GameVersion: "3.2.92777"})
	if err != nil {
		t.Fatal(err)
	}
	if emitErr := session.Emit(Event{Timestamp: started.Add(-2 * time.Second), Event: SessionStarted}); emitErr != nil {
		t.Fatal(emitErr)
	}
	queueIndex, queueCycle := 0, 0
	if emitErr := session.Emit(Event{Timestamp: started, Event: RunStarted, RunID: runID, GameID: "game-" + runID, Run: runName, QueueIndex: &queueIndex, QueueCycle: &queueCycle}); emitErr != nil {
		t.Fatal(emitErr)
	}
	run, err := NewRunRecorder(directory, RunRecorderContext{
		RunID: runID, SessionID: sessionID, GameID: "game-" + runID, Mode: HistoryModeProductiveFarming,
		Character: "MrBones", Difficulty: "nightmare", GameVersion: "3.2.92777", Run: runName,
		DefinitionID: runName, RouteID: "route-a", QueueIndex: queueIndex, QueueCycle: queueCycle, StartedAt: started,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := run.Emit(Event{Timestamp: started.Add(time.Millisecond), Event: RunContext}); err != nil {
		t.Fatal(err)
	}
	return session, run
}

func historyRunName(runID string) string {
	if strings.HasPrefix(runID, "mephisto") {
		return "mephisto"
	}
	return "countess"
}

func hasHistoryDiagnostic(diagnostics []HistoryFileDiagnostic, file string, code HistoryReasonCode) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.File == file && diagnostic.Code == code {
			return true
		}
	}
	return false
}

func historyRunDigest(runs []HistoryRun) string {
	var builder strings.Builder
	for _, run := range runs {
		fmt.Fprintf(&builder, "%s|%s|%s|%s|%d;", run.RunID, run.RouteID, run.Outcome, run.Reason, len(run.Events))
	}
	return builder.String()
}
