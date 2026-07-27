package telemetry

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

const phase15PerformanceRunCount = 10_000

var phase15PerformanceStart = time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)

func TestPhase15HistoryPerformanceFixtureIsDeterministic(t *testing.T) {
	snapshot := phase15PerformanceSnapshot()
	if len(snapshot.Runs) != phase15PerformanceRunCount {
		t.Fatalf("runs=%d want=%d", len(snapshot.Runs), phase15PerformanceRunCount)
	}
	end := phase15PerformanceStart.Add(100 * 24 * time.Hour)
	tests := []struct {
		name       string
		days       int
		terminal   int
		successful int
		keep       int
	}{
		{name: "30_days", days: 30, terminal: 3_000, successful: 2_400, keep: 2_400},
		{name: "60_days", days: 60, terminal: 6_000, successful: 4_800, keep: 4_800},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			from := end.Add(-time.Duration(test.days) * 24 * time.Hour)
			analysis, err := AnalyzeHistory(snapshot, HistoryFilter{FromUTC: &from, ToUTC: &end})
			if err != nil {
				t.Fatal(err)
			}
			if analysis.Summary.TerminalRuns != test.terminal || analysis.Summary.Successful != test.successful || analysis.Summary.Funnel.KeepReturn != test.keep {
				t.Fatalf("summary=%+v", analysis.Summary)
			}
		})
	}
}

func BenchmarkPhase15HistoryTenThousandRuns(b *testing.B) {
	directory := b.TempDir()
	if err := writePhase15PerformanceFiles(directory); err != nil {
		b.Fatal(err)
	}
	b.Run("startup", func(b *testing.B) {
		for iteration := 0; iteration < b.N; iteration++ {
			index, err := NewHistoryIndex(directory)
			if err != nil {
				b.Fatal(err)
			}
			if err := index.Refresh(); err != nil {
				b.Fatal(err)
			}
			if runs := len(index.Snapshot("").Runs); runs != phase15PerformanceRunCount {
				b.Fatalf("startup runs=%d", runs)
			}
		}
	})
	index, err := NewHistoryIndex(directory)
	if err != nil {
		b.Fatal(err)
	}
	if err := index.Refresh(); err != nil {
		b.Fatal(err)
	}
	b.Run("rebuild", func(b *testing.B) {
		for iteration := 0; iteration < b.N; iteration++ {
			if err := index.Rebuild(); err != nil {
				b.Fatal(err)
			}
			if runs := len(index.Snapshot("").Runs); runs != phase15PerformanceRunCount {
				b.Fatalf("rebuilt runs=%d", runs)
			}
		}
	})
	snapshot := index.Snapshot("")
	end := phase15PerformanceStart.Add(100 * 24 * time.Hour)
	for _, days := range []int{30, 60} {
		from := end.Add(-time.Duration(days) * 24 * time.Hour)
		filter := HistoryFilter{FromUTC: &from, ToUTC: &end}
		b.Run(fmt.Sprintf("query_%d_days", days), func(b *testing.B) {
			for iteration := 0; iteration < b.N; iteration++ {
				analysis, err := AnalyzeHistory(snapshot, filter)
				if err != nil {
					b.Fatal(err)
				}
				if analysis.Summary.TerminalRuns != days*100 {
					b.Fatalf("terminal runs=%d", analysis.Summary.TerminalRuns)
				}
			}
		})
	}
}

func phase15PerformanceSnapshot() HistorySnapshot {
	runs := make([]HistoryRun, 0, phase15PerformanceRunCount)
	for index := 0; index < phase15PerformanceRunCount; index++ {
		started := phase15PerformanceStart.Add(time.Duration(index) * 864 * time.Second)
		runName, route := "countess", fmt.Sprintf("countess-route-%d", index%2)
		if index%2 == 1 {
			runName, route = "mephisto", fmt.Sprintf("mephisto-route-%d", index%2)
		}
		outcome, boss, failedStep, reason := HistoryOutcomeSuccess, true, "", ""
		items := []analyzerItemFixture{{unitID: uint32(index + 1), key: "base:r01:normal", name: "El Rune", action: "keep", seen: true, matched: true, picked: true, stash: true}}
		if index%5 == 0 {
			outcome, boss, failedStep, reason, items = HistoryOutcomeFailed, false, "acquire_boss", "boss_not_found", nil
		}
		runs = append(runs, analyzerRun(fmt.Sprintf("phase15-run-%05d", index), started, 60, outcome, runName, route, boss, [4]int64{20, 10, 10, 15}, failedStep, reason, items))
	}
	return HistorySnapshot{Generation: 1, Runs: runs}
}

func writePhase15PerformanceFiles(directory string) (returnErr error) {
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return err
	}
	sessionID := "session-phase15-performance"
	sessionPath := filepath.Join(directory, sessionID+".jsonl")
	sessionFile, err := os.OpenFile(sessionPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	session := bufio.NewWriterSize(sessionFile, 1<<20)
	defer func() {
		if flushErr := session.Flush(); returnErr == nil && flushErr != nil {
			returnErr = flushErr
		}
		if closeErr := sessionFile.Close(); returnErr == nil && closeErr != nil {
			returnErr = closeErr
		}
	}()
	writeSession := func(event Event) error {
		event.SchemaVersion = HistorySchemaVersion
		event.Stream = HistoryStreamSession
		event.Mode = HistoryModeProductiveFarming
		event.SessionID = sessionID
		event.Character = "MrBones"
		event.Difficulty = "nightmare"
		event.GameVersion = "3.2.92777"
		line, err := json.Marshal(event)
		if err != nil {
			return err
		}
		line = append(line, '\n')
		_, err = session.Write(line)
		return err
	}
	if err := writeSession(Event{Timestamp: phase15PerformanceStart.Add(-time.Second), Event: SessionStarted}); err != nil {
		return err
	}
	queueIndex, queueCycle := 0, 0
	for index := 0; index < phase15PerformanceRunCount; index++ {
		started := phase15PerformanceStart.Add(time.Duration(index) * 864 * time.Second)
		runID := fmt.Sprintf("phase15-run-%05d", index)
		runName := "countess"
		if index%2 == 1 {
			runName = "mephisto"
		}
		gameID := fmt.Sprintf("phase15-game-%05d", index/2)
		if err := writeSession(Event{Timestamp: started, Event: RunStarted, RunID: runID, GameID: gameID, Run: runName, QueueIndex: &queueIndex, QueueCycle: &queueCycle}); err != nil {
			return err
		}
		runStartedAt := started
		runEvent := Event{
			SchemaVersion: HistorySchemaVersion, Stream: HistoryStreamRun, Timestamp: started.Add(time.Millisecond), Event: RunContext,
			RunID: runID, SessionID: sessionID, GameID: gameID, Mode: HistoryModeProductiveFarming,
			Character: "MrBones", Difficulty: "nightmare", GameVersion: "3.2.92777", Run: runName, DefinitionID: runName,
			RouteID: runName + "-route", QueueIndex: &queueIndex, QueueCycle: &queueCycle, RunStartedAt: &runStartedAt,
		}
		line, err := json.Marshal(runEvent)
		if err != nil {
			return err
		}
		line = append(line, '\n')
		if err := os.WriteFile(filepath.Join(directory, runID+".jsonl"), line, 0o644); err != nil {
			return err
		}
		terminal, reason := RunCompleted, ""
		if index%5 == 0 {
			terminal, reason = RunFailed, "boss_not_found"
		}
		if err := writeSession(Event{Timestamp: started.Add(time.Minute), Event: terminal, RunID: runID, GameID: gameID, Run: runName, Reason: reason}); err != nil {
			return err
		}
	}
	return nil
}
