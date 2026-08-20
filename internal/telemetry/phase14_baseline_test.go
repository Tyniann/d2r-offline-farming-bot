package telemetry

import (
	"bufio"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"
)

type phase14FixtureEvent struct {
	SchemaVersion int       `json:"schema_version"`
	Timestamp     time.Time `json:"timestamp"`
	Stream        string    `json:"stream"`
	Event         string    `json:"event"`
	RunID         string    `json:"run_id"`
	Mode          string    `json:"mode"`
	Stage         string    `json:"stage"`
	UnitID        uint32    `json:"unit_id"`
	PickitAction  string    `json:"pickit_action"`
}

type phase14FixtureRun struct {
	start, terminal time.Time
	terminalEvent   string
	bossKilled      bool
	items           map[uint32]*phase14FixtureItem
}

type phase14FixtureItem struct {
	action  string
	matched bool
	picked  bool
	stashed bool
	sold    bool
}

func TestPhase14HandCalculatedFixtureMatrix(t *testing.T) {
	events := readPhase14Fixture(t, "matrix.jsonl")
	runs := make(map[string]*phase14FixtureRun)
	stages := make(map[string]bool)
	for _, event := range events {
		if event.SchemaVersion != HistorySchemaVersion || event.Mode != string(HistoryModeProductiveFarming) {
			continue
		}
		run := runs[event.RunID]
		if run == nil {
			run = &phase14FixtureRun{items: make(map[uint32]*phase14FixtureItem)}
			runs[event.RunID] = run
		}
		if event.Stage != "" {
			stages[event.Stage] = true
		}
		switch event.Event {
		case "run_started":
			run.start = event.Timestamp
		case "run_completed", "run_failed", "run_aborted":
			run.terminal, run.terminalEvent = event.Timestamp, event.Event
		case "boss_kill_confirmed":
			run.bossKilled = true
		case "pickit_match", "pickup_success", "stash_success", "sell_success":
			item := run.items[event.UnitID]
			if item == nil {
				item = &phase14FixtureItem{}
				run.items[event.UnitID] = item
			}
			if event.PickitAction != "" {
				item.action = event.PickitAction
			}
			item.matched = item.matched || event.Event == "pickit_match"
			item.picked = item.picked || event.Event == "pickup_success"
			item.stashed = item.stashed || event.Event == "stash_success"
			item.sold = item.sold || event.Event == "sell_success"
		}
	}

	var durations []float64
	terminal, successful, failed, bossKills := 0, 0, 0, 0
	keep, sold, pickupLost, postPickupLost, incomplete := 0, 0, 0, 0, 0
	for _, run := range runs {
		if run.terminal.IsZero() {
			incomplete++
			continue
		}
		terminal++
		durations = append(durations, run.terminal.Sub(run.start).Seconds())
		switch run.terminalEvent {
		case "run_completed":
			successful++
		case "run_failed":
			failed++
		}
		if run.bossKilled {
			bossKills++
		}
		for _, item := range run.items {
			if item.action == "keep" && item.matched && item.picked && item.stashed {
				keep++
			}
			if item.action == "sell" && item.matched && item.picked && item.sold {
				sold++
			}
			if item.matched && !item.picked {
				pickupLost++
			}
			if item.action == "keep" && item.picked && !item.stashed {
				postPickupLost++
			}
		}
	}
	sort.Float64s(durations)
	activeSeconds := 0.0
	for _, duration := range durations {
		activeSeconds += duration
	}
	assertPhase14Number(t, "terminal runs", float64(terminal), 5)
	assertPhase14Number(t, "successful runs", float64(successful), 3)
	assertPhase14Number(t, "failed runs", float64(failed), 2)
	assertPhase14Number(t, "incomplete runs", float64(incomplete), 1)
	assertPhase14Number(t, "boss kills", float64(bossKills), 4)
	assertPhase14Number(t, "keep return", float64(keep), 3)
	assertPhase14Number(t, "sell confirmed", float64(sold), 1)
	assertPhase14Number(t, "pickup loss", float64(pickupLost), 1)
	assertPhase14Number(t, "post-pickup loss", float64(postPickupLost), 1)
	assertPhase14Number(t, "active seconds", activeSeconds, 360)
	assertPhase14Number(t, "success rate", float64(successful)/float64(terminal), 0.6)
	assertPhase14Number(t, "duration average", activeSeconds/float64(terminal), 72)
	assertPhase14Number(t, "duration median", durations[len(durations)/2], 60)
	assertPhase14Number(t, "duration minimum", durations[0], 30)
	assertPhase14Number(t, "duration maximum", durations[len(durations)-1], 120)
	assertPhase14Number(t, "keep per run", float64(keep)/float64(terminal), 0.6)
	assertPhase14Number(t, "keep per kill", float64(keep)/float64(bossKills), 0.75)
	assertPhase14Number(t, "keep per hour", float64(keep)/(activeSeconds/3600), 30)
	for _, stage := range []HistoryStage{HistoryStageTravel, HistoryStageCombat, HistoryStageLoot, HistoryStageReturnTown} {
		if !stages[string(stage)] {
			t.Fatalf("fixture does not cover stage %q", stage)
		}
	}
}

func TestPhase14FixtureNonGoals(t *testing.T) {
	for _, name := range []string{"legacy-schema-1.jsonl", "legacy-schema-2.jsonl"} {
		events := readPhase14Fixture(t, name)
		for _, event := range events {
			if event.SchemaVersion == HistorySchemaVersion {
				t.Fatalf("%s unexpectedly entered the Phase-14 epoch", name)
			}
		}
	}
	productive := 0
	for _, event := range readPhase14Fixture(t, "matrix.jsonl") {
		if event.SchemaVersion == HistorySchemaVersion && event.Mode == string(HistoryModeProductiveFarming) && event.Event == "run_started" {
			productive++
		}
	}
	if productive != 6 {
		t.Fatalf("productive run starts=%d, want 6; diagnostic modes must stay excluded", productive)
	}
	if _, err := readPhase14FixtureFile(filepath.Join("testdata", "phase14", "corrupt.jsonl")); err == nil {
		t.Fatal("corrupt fixture was partially accepted")
	}
}

func TestPhase14CoreContractIsStable(t *testing.T) {
	if HistorySchemaVersion != 4 || HistoryDefaultPageLimit != 50 || HistoryMaximumPageLimit != 200 || HistoryLowSampleBossKills != 10 {
		t.Fatal("Phase-14 numeric contract changed")
	}
	if HistorySortKeepPerHour != "keep_per_hour" {
		t.Fatalf("default comparison sort=%q", HistorySortKeepPerHour)
	}
	if len(HistoryRunCSVColumns()) != 14 || len(HistoryItemCSVColumns()) != 12 {
		t.Fatalf("unexpected export columns: runs=%v items=%v", HistoryRunCSVColumns(), HistoryItemCSVColumns())
	}
	for _, code := range []HistoryReasonCode{HistoryReasonFileInvalid, HistoryReasonContextMissing, HistoryReasonBossDuplicate, HistoryReasonFilterInvalid, HistoryReasonUnavailable} {
		if message, ok := HistoryReasonMessage(code); !ok || message == "" {
			t.Fatalf("missing German message for %q", code)
		}
	}
	for _, code := range []HistoryReasonCode{
		"route_clear_no_progress",
		"route_threat_out_of_range",
		"boss_combat_unprojectable",
		"retry_return_failed",
		"route_mana_recovery_failed",
		"route_recovery_unsafe",
		"route_threat_state_invalid",
		"combat_resource_exhausted",
		"mercenary_died_during_run",
		"cow_rejuvenation_reserve_missing",
		"chest_sweep_empty",
	} {
		if message, ok := HistoryReasonMessage(code); !ok || message == "" {
			t.Fatalf("missing German route-threat message for %q", code)
		}
	}
}

func readPhase14Fixture(t *testing.T, name string) []phase14FixtureEvent {
	t.Helper()
	events, err := readPhase14FixtureFile(filepath.Join("testdata", "phase14", name))
	if err != nil {
		t.Fatal(err)
	}
	return events
}

func readPhase14FixtureFile(path string) ([]phase14FixtureEvent, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	var events []phase14FixtureEvent
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var event phase14FixtureEvent
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, scanner.Err()
}

func assertPhase14Number(t *testing.T, name string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("%s=%v, want %v", name, got, want)
	}
}
