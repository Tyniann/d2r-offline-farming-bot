package telemetry

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestPhase14RunIDsAreUniqueAcrossParallelAndSequentialGeneration(t *testing.T) {
	const count = 1000
	ids := make(chan string, count)
	var workers sync.WaitGroup
	for index := 0; index < count; index++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			ids <- NewRunID("countess")
		}()
	}
	workers.Wait()
	close(ids)
	seen := make(map[string]struct{}, count)
	for id := range ids {
		if id == "" || safePart(id) != id {
			t.Fatalf("unsafe run ID %q", id)
		}
		if _, duplicate := seen[id]; duplicate {
			t.Fatalf("duplicate run ID %q", id)
		}
		seen[id] = struct{}{}
	}
}

func TestPhase14ProductiveStreamsShareExactRunContext(t *testing.T) {
	directory := t.TempDir()
	session, err := NewSessionRecorderWithContext(directory, SessionRecorderContext{
		Mode: HistoryModeProductiveFarming, Character: "MrBones", Difficulty: "nightmare", GameVersion: "3.2.92777",
	})
	if err != nil {
		t.Fatal(err)
	}
	runID := NewRunID("countess")
	queueIndex, queueCycle := 0, 2
	if emitErr := session.Emit(Event{Event: RunStarted, GameID: "game-003", RunID: runID, Run: "countess", QueueIndex: &queueIndex, QueueCycle: &queueCycle}); emitErr != nil {
		t.Fatal(emitErr)
	}
	startedAt := time.Date(2026, 7, 22, 8, 0, 0, 0, time.UTC)
	run, err := NewRunRecorder(directory, RunRecorderContext{
		RunID: runID, SessionID: session.SessionID(), GameID: "game-003", Mode: HistoryModeProductiveFarming,
		Character: "MrBones", Difficulty: "nightmare", GameVersion: "3.2.92777", Run: "countess",
		DefinitionID: "countess", RouteID: "countess-route", RouteLayoutFingerprint: "layout",
		QueueIndex: queueIndex, QueueCycle: queueCycle, StartedAt: startedAt,
		PickitProfiles: []PickitProfileContext{{ID: "countess-standard", Revision: 2}}, PickitAssignmentRevision: 4,
	})
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(run.Path()) != runID+".jsonl" {
		t.Fatalf("run path %q does not use exact run ID", run.Path())
	}
	if err := run.Emit(Event{Event: RunContext}); err != nil {
		t.Fatal(err)
	}
	if err := session.Emit(Event{Event: RunCompleted, GameID: "game-003", RunID: runID, Run: "countess"}); err != nil {
		t.Fatal(err)
	}
	if err := run.Close(); err != nil {
		t.Fatal(err)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}

	runEvent := readSingleSchema3Event(t, run.Path())
	if runEvent.SchemaVersion != HistorySchemaVersion || runEvent.Stream != HistoryStreamRun || runEvent.Mode != HistoryModeProductiveFarming || runEvent.RunID != runID || runEvent.SessionID != session.SessionID() || runEvent.GameID != "game-003" || runEvent.Character != "MrBones" || runEvent.Difficulty != "nightmare" || runEvent.GameVersion != "3.2.92777" || runEvent.RouteID != "countess-route" || runEvent.QueueIndex == nil || *runEvent.QueueIndex != queueIndex || runEvent.QueueCycle == nil || *runEvent.QueueCycle != queueCycle || runEvent.RunStartedAt == nil || !runEvent.RunStartedAt.Equal(startedAt) || runEvent.PickitAssignmentRevision != 4 || len(runEvent.PickitProfiles) != 1 {
		t.Fatalf("incomplete immutable run event: %+v", runEvent)
	}
	sessionEvents := readSchema3Events(t, session.Path())
	if len(sessionEvents) != 2 || sessionEvents[0].RunID != runID || sessionEvents[1].RunID != runID || sessionEvents[0].SessionID != runEvent.SessionID || sessionEvents[0].Stream != HistoryStreamSession {
		t.Fatalf("cross-stream correlation failed: session=%+v run=%+v", sessionEvents, runEvent)
	}
}

func TestPhase14Schema3RejectsContextDriftAndDuplicateTerminal(t *testing.T) {
	context := RunRecorderContext{
		RunID: "countess-test", SessionID: "session-test", GameID: "game-test", Mode: HistoryModeProductiveFarming,
		Character: "MrBones", Difficulty: "nightmare", GameVersion: "3.2.92777", Run: "countess",
		DefinitionID: "countess", RouteID: "route-a", StartedAt: time.Now().UTC(),
	}
	run, err := NewRunRecorder(t.TempDir(), context)
	if err != nil {
		t.Fatal(err)
	}
	defer run.Close()
	if emitErr := run.Emit(Event{Event: RunContext, RunID: "other"}); emitErr == nil {
		t.Fatal("run-ID drift was accepted")
	}
	if emitErr := run.Emit(Event{Event: RunContext, Mode: HistoryModeDiagnostic}); emitErr == nil {
		t.Fatal("mode drift was accepted")
	}
	if emitErr := run.Emit(Event{Event: RunContext, RouteID: "route-b"}); emitErr == nil {
		t.Fatal("route drift was accepted")
	}

	session, err := NewSessionRecorderWithContext(t.TempDir(), SessionRecorderContext{Mode: HistoryModeProductiveFarming, Character: "MrBones", Difficulty: "nightmare", GameVersion: "3.2.92777"})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	terminal := Event{Event: RunCompleted, GameID: "game-test", RunID: "countess-test", Run: "countess"}
	if err := session.Emit(terminal); err != nil {
		t.Fatal(err)
	}
	terminal.Event = RunFailed
	if err := session.Emit(terminal); err == nil {
		t.Fatal("duplicate terminal was accepted")
	}
	if _, err := NewRunRecorder(t.TempDir(), RunRecorderContext{RunID: "incomplete", Mode: HistoryModeProductiveFarming, Run: "countess", StartedAt: time.Now()}); err == nil {
		t.Fatal("missing productive context was accepted")
	}
}

func readSingleSchema3Event(t *testing.T, path string) Event {
	t.Helper()
	events := readSchema3Events(t, path)
	if len(events) != 1 {
		t.Fatalf("event count=%d, want 1", len(events))
	}
	return events[0]
}

func readSchema3Events(t *testing.T, path string) []Event {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	var events []Event
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var event Event
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			t.Fatal(err)
		}
		events = append(events, event)
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	return events
}
