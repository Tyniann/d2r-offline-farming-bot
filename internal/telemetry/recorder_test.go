package telemetry

import (
	"bufio"
	"encoding/json"
	"os"
	"testing"
)

func TestRecorderWritesOneJSONLObjectPerLineAndDeduplicatesObservedUnits(t *testing.T) {
	r, err := New(t.TempDir(), "countess", "loot-and-return")
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
		if emitErr := r.Emit(event); emitErr != nil {
			t.Fatal(emitErr)
		}
	}
	if closeErr := r.Close(); closeErr != nil {
		t.Fatal(closeErr)
	}

	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	var events []Event
	var records []map[string]json.RawMessage
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var record map[string]json.RawMessage
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			t.Fatal(err)
		}
		var event Event
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			t.Fatal(err)
		}
		records = append(records, record)
		events = append(events, event)
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if len(events) != 4 {
		t.Fatalf("event count=%d, want 4", len(events))
	}
	if first := events[0]; first.SchemaVersion != HistorySchemaVersion || first.Stream != HistoryStreamRun || first.Mode != HistoryModeDiagnostic || first.RunID != r.RunID() || first.Run != "countess" || first.Phase != "loot-and-return" || first.Timestamp.IsZero() {
		t.Fatalf("incomplete context event: %+v", first)
	}
	for index, event := range events[1:] {
		if event.SchemaVersion != 0 || event.Stream != "" || event.Mode != "" || event.RunID != "" || event.Run != "" || event.Phase != "" || event.Timestamp.IsZero() {
			t.Fatalf("non-compact event: %+v", event)
		}
		for _, key := range []string{"schema_version", "stream", "mode", "run_id", "run", "phase"} {
			if _, exists := records[index+1][key]; exists {
				t.Fatalf("compact record %d repeats %q: %s", index+1, key, records[index+1][key])
			}
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

func TestRecorderPersistsResolvedRunContext(t *testing.T) {
	r, err := New(t.TempDir(), "mephisto", "")
	if err != nil {
		t.Fatal(err)
	}
	path := r.Path()
	want := Event{
		Event: RunContext, DefinitionID: "mephisto", RouteID: "durance-route",
		RouteLayoutFingerprint: "fingerprint", WaypointTarget: "durance_of_hate_level_2",
		TownOrigin: "act3",
	}
	if emitErr := r.Emit(want); emitErr != nil {
		t.Fatal(emitErr)
	}
	if closeErr := r.Close(); closeErr != nil {
		t.Fatal(closeErr)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var got Event
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got.Event != RunContext || got.DefinitionID != want.DefinitionID || got.RouteID != want.RouteID || got.RouteLayoutFingerprint != want.RouteLayoutFingerprint || got.WaypointTarget != want.WaypointTarget || got.TownOrigin != want.TownOrigin {
		t.Fatalf("run context=%+v", got)
	}
}

func TestRecorderPersistsRouteThreatFieldsIncludingFalseTransitions(t *testing.T) {
	r, err := New(t.TempDir(), "summoner", "full")
	if err != nil {
		t.Fatal(err)
	}
	path := r.Path()
	falseValue := false
	if emitErr := r.Emit(Event{
		Event: RouteMonsterSnapshotSaturated, RouteID: "arcane", SegmentID: "arm",
		Zone: "landing", NPCID: 40, PlayerX: 10, PlayerY: 11, TargetX: 20, TargetY: 21,
		MonstersTruncated: &falseValue, CoverageComplete: &falseValue,
		EligibleMonsterCount: 513, RetainedMonsterCount: 512,
		RequiredRadiusTiles: 42, CoverageRadiusTiles: 40,
	}); emitErr != nil {
		t.Fatal(emitErr)
	}
	if closeErr := r.Close(); closeErr != nil {
		t.Fatal(closeErr)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"monsters_truncated", "coverage_complete", "eligible_monster_count", "retained_monster_count", "required_radius_tiles", "coverage_radius_tiles"} {
		if _, ok := raw[field]; !ok {
			t.Fatalf("route telemetry field %q missing in %s", field, data)
		}
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
