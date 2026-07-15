package telemetry

import (
	"bufio"
	"encoding/json"
	"os"
	"testing"
)

func TestSessionRecorderCorrelatesAndFlushesLifecycleEvents(t *testing.T) {
	recorder, err := NewSessionRecorder(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	path := recorder.Path()
	for _, event := range []Event{
		{Event: SessionStarted},
		{Event: GameStarted, GameID: "game-1"},
		{Event: RunStarted, GameID: "game-1", RunID: "run-1", Run: "countess"},
	} {
		if emitErr := recorder.Emit(event); emitErr != nil {
			t.Fatal(emitErr)
		}
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	count := 0
	for scanner.Scan() {
		var event Event
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			t.Fatal(err)
		}
		if event.SchemaVersion != 2 || event.SessionID != recorder.SessionID() || event.Timestamp.IsZero() {
			t.Fatalf("event = %+v", event)
		}
		count++
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Fatalf("event count = %d", count)
	}
	if err := recorder.Close(); err != nil {
		t.Fatal(err)
	}
	if err := recorder.Emit(Event{Event: SessionFailed}); err == nil {
		t.Fatal("expected emit-after-close error")
	}
}
