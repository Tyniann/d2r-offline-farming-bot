package api

import (
	"testing"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/telemetry"
)

func TestHistoryRefreshPublishesOnlyBoundedChangeSignal(t *testing.T) {
	directory := t.TempDir()
	index, err := telemetry.NewHistoryIndex(directory)
	if err != nil {
		t.Fatal(err)
	}
	if err := index.Refresh(); err != nil {
		t.Fatal(err)
	}
	publisher := telemetry.NewLivePublisher(8, 8)
	backend := &LiveBackend{history: index, publisher: publisher, historyTerminalHash: terminalHistoryHash(index.Snapshot("").Runs)}

	writeIncompleteHistoryRun(t, directory)
	if _, err := backend.refreshHistory(""); err != nil {
		t.Fatal(err)
	}
	if publisher.Sequence() != 0 {
		t.Fatalf("incomplete writer change published %d event(s), want none", publisher.Sequence())
	}

	writeTerminalHistoryRun(t, directory)
	if _, err := backend.refreshHistory(""); err != nil {
		t.Fatal(err)
	}
	replay, subscription := publisher.Subscribe(0)
	defer subscription.Close()
	if len(replay) != 1 || replay[0].Event != "history_changed" {
		t.Fatalf("events=%+v, want one history_changed signal", replay)
	}
	if len(replay[0].Details) != 1 || replay[0].Details["generation"] == nil {
		t.Fatalf("details=%+v, want bounded generation only", replay[0].Details)
	}
	if replay[0].RunID != "" || replay[0].Reason != "" || replay[0].Step != "" {
		t.Fatalf("signal leaks run telemetry: %+v", replay[0])
	}

	if _, err := backend.refreshHistory(""); err != nil {
		t.Fatal(err)
	}
	if publisher.Sequence() != 1 {
		t.Fatalf("unchanged refresh sequence=%d, want 1", publisher.Sequence())
	}
}

func writeIncompleteHistoryRun(t *testing.T, directory string) {
	t.Helper()
	started := time.Date(2026, 7, 22, 9, 0, 0, 0, time.UTC)
	context := telemetry.SessionRecorderContext{SessionID: "session-api-sse-incomplete", Mode: telemetry.HistoryModeProductiveFarming, Character: "MrBones", Difficulty: "nightmare", GameVersion: "3.2.92777"}
	session, err := telemetry.NewSessionRecorderWithContext(directory, context)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	queueIndex, queueCycle := 0, 0
	if emitErr := session.Emit(telemetry.Event{Timestamp: started.Add(-time.Second), Event: telemetry.SessionStarted}); emitErr != nil {
		t.Fatal(emitErr)
	}
	if emitErr := session.Emit(telemetry.Event{Timestamp: started, Event: telemetry.RunStarted, RunID: "run-api-sse-incomplete", GameID: "game-api-sse-incomplete", Run: "countess", QueueIndex: &queueIndex, QueueCycle: &queueCycle}); emitErr != nil {
		t.Fatal(emitErr)
	}
	run, err := telemetry.NewRunRecorder(directory, telemetry.RunRecorderContext{
		RunID: "run-api-sse-incomplete", SessionID: context.SessionID, GameID: "game-api-sse-incomplete", Mode: context.Mode,
		Character: context.Character, Difficulty: context.Difficulty, GameVersion: context.GameVersion,
		Run: "countess", DefinitionID: "countess", RouteID: "route-a", QueueIndex: queueIndex, QueueCycle: queueCycle, StartedAt: started,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer run.Close()
	if emitErr := run.Emit(telemetry.Event{Timestamp: started.Add(time.Millisecond), Event: telemetry.RunContext}); emitErr != nil {
		t.Fatal(emitErr)
	}
}

func writeTerminalHistoryRun(t *testing.T, directory string) {
	t.Helper()
	started := time.Date(2026, 7, 22, 10, 0, 0, 0, time.UTC)
	context := telemetry.SessionRecorderContext{SessionID: "session-api-sse", Mode: telemetry.HistoryModeProductiveFarming, Character: "MrBones", Difficulty: "nightmare", GameVersion: "3.2.92777"}
	session, err := telemetry.NewSessionRecorderWithContext(directory, context)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	queueIndex, queueCycle := 0, 0
	if emitErr := session.Emit(telemetry.Event{Timestamp: started.Add(-time.Second), Event: telemetry.SessionStarted}); emitErr != nil {
		t.Fatal(emitErr)
	}
	if emitErr := session.Emit(telemetry.Event{Timestamp: started, Event: telemetry.RunStarted, RunID: "run-api-sse", GameID: "game-api-sse", Run: "countess", QueueIndex: &queueIndex, QueueCycle: &queueCycle}); emitErr != nil {
		t.Fatal(emitErr)
	}
	run, err := telemetry.NewRunRecorder(directory, telemetry.RunRecorderContext{
		RunID: "run-api-sse", SessionID: context.SessionID, GameID: "game-api-sse", Mode: context.Mode,
		Character: context.Character, Difficulty: context.Difficulty, GameVersion: context.GameVersion,
		Run: "countess", DefinitionID: "countess", RouteID: "route-a", QueueIndex: queueIndex, QueueCycle: queueCycle, StartedAt: started,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer run.Close()
	if emitErr := run.Emit(telemetry.Event{Timestamp: started.Add(time.Millisecond), Event: telemetry.RunContext}); emitErr != nil {
		t.Fatal(emitErr)
	}
	ended := started.Add(time.Minute)
	if emitErr := run.Emit(telemetry.Event{Timestamp: ended, Event: telemetry.RunCompleted}); emitErr != nil {
		t.Fatal(emitErr)
	}
	if emitErr := session.Emit(telemetry.Event{Timestamp: ended, Event: telemetry.RunCompleted, RunID: "run-api-sse", GameID: "game-api-sse", Run: "countess"}); emitErr != nil {
		t.Fatal(emitErr)
	}
}
