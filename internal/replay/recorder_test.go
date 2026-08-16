package replay

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

func TestRecorderRoundTripUsesVersionedSafeSchema(t *testing.T) {
	directory := t.TempDir()
	now := time.Date(2026, time.August, 14, 9, 30, 0, 0, time.UTC)
	recorder := newTestRecorder(t, directory, now, Config{
		Enabled:        true,
		Label:          "focus-loss",
		SaveSuccessful: true,
	}, ContractSnapshot{
		RunID:     "mephisto",
		ProfileID: "sorceress",
		Definition: map[string]any{
			"terminal_step": "quit_game",
			"save_path":     `C:\Users\Mario\Saved Games\Diablo II Resurrected Offline`,
		},
		Loadout: map[string]any{"token": "trace-secret"},
	})

	state := frozenWorldState(now)
	recorder.BeginTick(now.Add(time.Second), NormalizeWorld(state), state.Generation, RuntimeGates{InputEnabled: true, WindowBound: true}, TickState{Step: "traverse", Outcome: "running", Active: true})
	recorder.RecordDependency("pathing.tick", map[string]any{"target_x": 42}, map[string]any{"status": "moving"}, errors.New(`password=hunter2 at C:\Users\Mario\route.yaml`))
	recorder.RecordIntent("teleport", map[string]any{"x": 42, "y": 24}, "sent")
	recorder.EndTick(TickState{Step: "traverse", Outcome: "failure", Reason: "focus_lost"})

	result, err := recorder.Finalize(Terminal{Step: "traverse", Outcome: "failure", Reason: "focus_lost"})
	if err != nil {
		t.Fatalf("Finalize() error = %v", err)
	}
	if !result.Saved || !strings.HasSuffix(result.Filename, BundleExtension) {
		t.Fatalf("Finalize() result = %+v, want saved %s bundle", result, BundleExtension)
	}
	bundle, err := ReadBundle(filepath.Join(directory, result.Filename), 1<<20)
	if err != nil {
		t.Fatalf("ReadBundle() error = %v", err)
	}
	if bundle.SchemaVersion != SchemaVersion || bundle.Contract.RunID != "mephisto" {
		t.Fatalf("bundle identity = version %d run %q", bundle.SchemaVersion, bundle.Contract.RunID)
	}
	if got := bundle.Frames[0].World.Player.SkillsKnown; len(got) != 2 || got[0] != 36 || got[1] != 54 {
		t.Fatalf("normalized skills = %v, want [36 54]", got)
	}
	if got := bundle.Frames[0].World.Player; !got.WeaponSetAvailable || got.ActiveWeaponSet != "secondary" {
		t.Fatalf("normalized weapon set = %+v, want available secondary", got)
	}
	if got := bundle.Frames[0].Dependencies[0].Error; strings.Contains(got, "hunter2") || strings.Contains(strings.ToLower(got), `c:\users`) {
		t.Fatalf("dependency error was not redacted: %q", got)
	}

	encoded, err := json.Marshal(bundle)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	for _, forbidden := range []string{"trace-secret", "hunter2", `C:\\Users`, "identity_raw_id", "raw_location", `\"stats\"`, "map_seed"} {
		if strings.Contains(strings.ToLower(string(encoded)), strings.ToLower(forbidden)) {
			t.Fatalf("bundle contains prohibited value or field %q", forbidden)
		}
	}
}

func TestRecorderIsStrictlyOptIn(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "must-not-exist")
	recorder, err := NewRecorder(Config{Directory: directory, Label: "disabled"}, Metadata{}, ContractSnapshot{})
	if err != nil {
		t.Fatalf("NewRecorder() error = %v", err)
	}
	recorder.BeginTick(time.Now(), WorldFrame{}, 1, RuntimeGates{}, TickState{})
	result, err := recorder.Finalize(Terminal{Outcome: "failure"})
	if err != nil || result.Saved {
		t.Fatalf("Finalize() = %+v, %v; want disabled no-op", result, err)
	}
	if _, err := os.Stat(directory); !os.IsNotExist(err) {
		t.Fatalf("disabled recorder created directory: %v", err)
	}
}

func TestRecorderRetentionOnlyRemovesOldTraceBundles(t *testing.T) {
	directory := t.TempDir()
	keepPath := filepath.Join(directory, "operator-note.txt")
	if err := os.WriteFile(keepPath, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	oldPath := filepath.Join(directory, "old"+BundleExtension)
	if err := os.WriteFile(oldPath, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	oldTime := time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC)
	if err := os.Chtimes(oldPath, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}

	recorder := newTestRecorder(t, directory, oldTime.Add(24*time.Hour), Config{Enabled: true, Label: "retention", SaveSuccessful: true, MaximumBundles: 1}, ContractSnapshot{RunID: "mephisto", Definition: map[string]any{}})
	appendTestFrame(recorder, oldTime.Add(24*time.Hour))
	if _, err := recorder.Finalize(Terminal{Step: "done", Outcome: "success"}); err != nil {
		t.Fatalf("Finalize() error = %v", err)
	}
	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Fatalf("old trace still exists: %v", err)
	}
	if _, err := os.Stat(keepPath); err != nil {
		t.Fatalf("unrelated file was touched: %v", err)
	}
}

func TestRecorderAtomicPublishFailureLeavesNoBundle(t *testing.T) {
	directory := t.TempDir()
	now := time.Date(2026, time.August, 14, 10, 0, 0, 0, time.UTC)
	recorder := newTestRecorder(t, directory, now, Config{Enabled: true, Label: "atomic", SaveSuccessful: true}, ContractSnapshot{RunID: "mephisto", Definition: map[string]any{}})
	appendTestFrame(recorder, now)
	recorder.rename = func(_, _ string) error { return errors.New("injected rename failure") }
	if _, err := recorder.Finalize(Terminal{Step: "done", Outcome: "failure"}); err == nil || !strings.Contains(err.Error(), "publish runtime trace atomically") {
		t.Fatalf("Finalize() error = %v, want atomic publish failure", err)
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("atomic failure left files: %v", entries)
	}
}

func TestRecorderIntentObservationCannotExecuteInput(t *testing.T) {
	directory := t.TempDir()
	now := time.Date(2026, time.August, 14, 10, 30, 0, 0, time.UTC)
	recorder := newTestRecorder(t, directory, now, Config{Enabled: true, Label: "observer", SaveSuccessful: true}, ContractSnapshot{RunID: "mephisto", Definition: map[string]any{}})
	inputCalled := false
	recorder.BeginTick(now, WorldFrame{Phase: "in_game", Valid: true}, 1, RuntimeGates{}, TickState{Outcome: "running"})
	recorder.RecordIntent("teleport", map[string]any{"untrusted_callback": func() { inputCalled = true }}, "requested")
	recorder.EndTick(TickState{Outcome: "failure"})
	if _, err := recorder.Finalize(Terminal{Outcome: "failure"}); err != nil {
		t.Fatalf("Finalize() error = %v", err)
	}
	if inputCalled {
		t.Fatal("trace observation executed an input callback")
	}
}

func TestRecorderRejectsMultipleProductiveIntentsInOneTick(t *testing.T) {
	directory := t.TempDir()
	now := time.Date(2026, time.August, 14, 10, 45, 0, 0, time.UTC)
	recorder := newTestRecorder(t, directory, now, Config{Enabled: true, Label: "intent-invariant", SaveSuccessful: true}, ContractSnapshot{RunID: "mephisto", Definition: map[string]any{}})
	recorder.BeginTick(now, WorldFrame{Phase: "in_game", Valid: true}, 1, RuntimeGates{}, TickState{Outcome: "running"})
	recorder.RecordIntent("teleport", map[string]any{"x": 10}, "sent")
	recorder.RecordIntent("click", map[string]any{"button": "right"}, "sent")
	recorder.EndTick(TickState{Outcome: "failed"})
	if _, err := recorder.Finalize(Terminal{Outcome: "failed"}); err == nil || !strings.Contains(err.Error(), "more than one input intent") {
		t.Fatalf("Finalize() error = %v, want per-tick intent invariant", err)
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("invalid multi-intent trace published files: %v", entries)
	}
}

func TestRecorderAllowsBoundedCompressibleWorldHistory(t *testing.T) {
	directory := t.TempDir()
	now := time.Date(2026, time.August, 14, 11, 0, 0, 0, time.UTC)
	recorder := newTestRecorder(t, directory, now, Config{Enabled: true, Label: "compressible", SaveSuccessful: true, MaximumBundleBytes: 1 << 20}, ContractSnapshot{RunID: "mephisto", Definition: map[string]any{}})
	worldFrame := WorldFrame{Phase: "in_game", Valid: true, Monsters: make([]MonsterFrame, 512)}
	for index := range worldFrame.Monsters {
		worldFrame.Monsters[index] = MonsterFrame{NPCID: 242, UnitID: uint32(index + 1), X: 17590, Y: 8069, TypeFlag: 10}
	}
	for tick := 0; tick < 350; tick++ {
		at := now.Add(time.Duration(tick) * 100 * time.Millisecond)
		recorder.BeginTick(at, worldFrame, uint64(tick+1), RuntimeGates{InputEnabled: true, WindowBound: true}, TickState{Step: "play_bound_route", Outcome: "running", Active: true})
		recorder.EndTick(TickState{Step: "play_bound_route", Outcome: "running", Active: true})
	}
	result, err := recorder.Finalize(Terminal{Step: "play_bound_route", Outcome: "failed", Reason: "operator_stop"})
	if err != nil {
		t.Fatalf("Finalize() error = %v", err)
	}
	if !result.Saved || result.Bytes > 1<<20 {
		t.Fatalf("Finalize() = %+v, want compressed bundle within limit", result)
	}
	if _, err := ReadBundle(filepath.Join(directory, result.Filename), 1<<20); err != nil {
		t.Fatalf("ReadBundle() error = %v", err)
	}
}

func newTestRecorder(t *testing.T, directory string, now time.Time, config Config, contract ContractSnapshot) *Recorder {
	t.Helper()
	config.Directory = directory
	config.Now = func() time.Time { return now }
	recorder, err := NewRecorder(config, Metadata{BotVersion: "test", Commit: "fixture"}, contract)
	if err != nil {
		t.Fatalf("NewRecorder() error = %v", err)
	}
	return recorder
}

func appendTestFrame(recorder *Recorder, at time.Time) {
	recorder.BeginTick(at, WorldFrame{Phase: "in_game", Valid: true}, 1, RuntimeGates{}, TickState{Step: "start", Outcome: "running", Active: true})
	recorder.EndTick(TickState{Step: "done", Outcome: "success"})
}

func frozenWorldState(at time.Time) world.State {
	return world.State{
		At:         at,
		Generation: 7,
		Phase:      world.GamePhaseInGame,
		Valid:      true,
		Area:       world.LookupArea(world.AreaID(102)),
		Player: world.Player{
			Position:        world.Position{X: 10, Y: 20},
			HP:              800,
			MaxHP:           1000,
			Mana:            450,
			MaxMana:         500,
			ActiveWeaponSet: world.WeaponSetState{Set: world.WeaponSetSecondary, Available: true},
			SkillsKnown:     map[uint16]bool{54: true, 36: true, 99: false},
			SkillsComplete:  true,
		},
		Identity: world.GameIdentity{Valid: true, CharacterName: "TraceSorceress", Class: world.CharacterClassSorceress, MapSeed: 0xdecafbad},
		Items: []world.Item{{
			TxtFileNo: 610, UnitID: 101, Code: "r33", Quality: world.ItemQualityNormal,
			Location: world.ItemLocationGround, Position: world.Position{X: 12, Y: 22},
			RawLocation: 5, Flags: 0xdeadbeef, Stats: []world.ItemStat{{ID: 194, Value: 6}},
		}},
	}
}
