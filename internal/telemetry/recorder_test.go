package telemetry

import (
	"bufio"
	"encoding/json"
	"os"
	"testing"
)

func TestRecorderWritesOneJSONLObjectPerLineAndDeduplicatesObservedUnits(t *testing.T) {
	r, err := New(t.TempDir(), "countess", "loot-countess")
	if err != nil {
		t.Fatal(err)
	}
	path := r.Path()
	for _, event := range []Event{
		{Event: DropSeen, UnitID: 7, Code: "r03"},
		{Event: DropSeen, UnitID: 7, Code: "r03"},
		{Event: PickitMatch, UnitID: 7, Code: "r03"},
		{Event: PickupAttempt, UnitID: 7, Attempt: 1},
		{Event: PickupAttempt, UnitID: 7, Attempt: 2},
	} {
		if err := r.Emit(event); err != nil {
			t.Fatal(err)
		}
	}
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}

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
	if len(events) != 4 {
		t.Fatalf("event count=%d, want 4", len(events))
	}
	for _, event := range events {
		if event.SchemaVersion != 1 || event.RunID != r.RunID() || event.Run != "countess" || event.Phase != "loot-countess" || event.Timestamp.IsZero() {
			t.Fatalf("incomplete event: %+v", event)
		}
	}
}

func TestRecorderRejectsEmitAfterClose(t *testing.T) {
	r, err := New(t.TempDir(), "countess", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	if err := r.Emit(Event{Event: DropSeen, UnitID: 1}); err == nil {
		t.Fatal("Emit() after Close error=nil")
	}
}

func TestRecorderFailsWhenDirectoryCannotBeCreated(t *testing.T) {
	path := t.TempDir() + string(os.PathSeparator) + "file"
	if err := os.WriteFile(path, []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := New(path, "countess", ""); err == nil {
		t.Fatal("New() error=nil for file used as directory")
	}
}
