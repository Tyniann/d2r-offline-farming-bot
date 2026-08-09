package app

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
)

func TestOperatorSettingsInitializesDefaultsAndPersistsTwoCharacterQueues(t *testing.T) {
	store, root := newOperatorSettingsTestStore(t)
	initial, err := store.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if initial.Revision != 1 || !initial.History.RetentionEnabled || initial.History.RetentionDays != 60 || initial.Input.Enabled {
		t.Fatalf("initial=%+v", initial)
	}
	if _, statErr := os.Stat(filepath.Join(root, "configs", operatorSettingsFilename)); statErr != nil {
		t.Fatal(statErr)
	}
	replacement := cloneOperatorSettings(initial)
	replacement.Characters["mrbones"] = OperatorCharacterSettings{LastDifficulty: "nightmare", Queue: []string{"countess", "mephisto"}}
	replacement.Characters["mrhammer"] = OperatorCharacterSettings{LastDifficulty: "hell", Queue: []string{"mephisto", "countess"}}
	change, err := store.Update(1, replacement)
	if err != nil {
		t.Fatal(err)
	}
	if change.Settings.Revision != 2 || change.RestartRequired || len(change.ChangedFields) != 1 || change.ChangedFields[0] != "characters" {
		t.Fatalf("change=%+v", change)
	}
	reloaded, err := store.Snapshot()
	if err != nil || reloaded.Characters["mrhammer"].LastDifficulty != "hell" || reloaded.Characters["mrbones"].Queue[1] != "mephisto" {
		t.Fatalf("reloaded=%+v err=%v", reloaded, err)
	}
}

func TestOperatorSettingsSchema3SetupAssignmentProtectionResetAndNewCharacter(t *testing.T) {
	store, _ := newOperatorSettingsTestStore(t)
	initial, err := store.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if initial.SchemaVersion != 3 || initial.Characters["mrbones"].CharacterClass != "" || initial.Characters["mrbones"].CombatProfile != "" {
		t.Fatalf("initial=%+v", initial)
	}

	assigned, err := store.AssignCharacterProfile("MrBones", "necromancer", "necro_bone_spear", initial.Revision)
	if err != nil {
		t.Fatal(err)
	}
	value := assigned.Settings.Characters["mrbones"]
	if assigned.Settings.Revision != 2 || value.CharacterClass != "necromancer" || value.CombatProfile != "necro_bone_spear" {
		t.Fatalf("assigned=%+v", assigned)
	}

	mutated := cloneOperatorSettings(assigned.Settings)
	mutated.Characters["mrbones"] = OperatorCharacterSettings{
		CharacterClass: "paladin", CombatProfile: "necro_bone_spear",
		LastDifficulty: value.LastDifficulty, Queue: append([]string(nil), value.Queue...),
	}
	if _, updateErr := store.Update(assigned.Settings.Revision, mutated); updateErr == nil || updateErr.Error() != "operator_settings_setup_read_only" {
		t.Fatalf("general setup mutation error=%v", updateErr)
	}

	preview, err := store.PreviewReset(assigned.Settings.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if got := preview.Settings.Characters["mrbones"]; got.CharacterClass != value.CharacterClass || got.CombatProfile != value.CombatProfile {
		t.Fatalf("preview reset lost setup=%+v", got)
	}
	reset, err := store.Reset(assigned.Settings.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if got := reset.Settings.Characters["mrbones"]; got.CharacterClass != value.CharacterClass || got.CombatProfile != value.CombatProfile {
		t.Fatalf("reset lost setup=%+v", got)
	}

	added, err := store.AssignCharacterProfile("FreshHero", "necromancer", "necro_bone_spear", reset.Settings.Revision)
	if err != nil {
		t.Fatal(err)
	}
	fresh := added.Settings.Characters["freshhero"]
	if fresh.CharacterClass != "necromancer" || fresh.CombatProfile != "necro_bone_spear" ||
		fresh.LastDifficulty != store.characterDefaults.LastDifficulty || !reflect.DeepEqual(fresh.Queue, store.characterDefaults.Queue) {
		t.Fatalf("fresh=%+v defaults=%+v", fresh, store.characterDefaults)
	}
	reloaded, err := store.Snapshot()
	if err != nil || !reflect.DeepEqual(reloaded, added.Settings) {
		t.Fatalf("schema-3 round-trip=%+v err=%v", reloaded, err)
	}
	idempotent, err := store.AssignCharacterProfile("FreshHero", "necromancer", "necro_bone_spear", reloaded.Revision)
	if err != nil || idempotent.Settings.Revision != reloaded.Revision || len(idempotent.ChangedFields) != 0 {
		t.Fatalf("idempotent=%+v err=%v", idempotent, err)
	}
}

func TestOperatorSettingsSetupPairValidationAndSchema1Rejection(t *testing.T) {
	store, _ := newOperatorSettingsTestStore(t)
	initial, err := store.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	half := cloneOperatorSettings(initial)
	value := half.Characters["mrbones"]
	value.CharacterClass = "necromancer"
	half.Characters["mrbones"] = value
	if validationErr := validateOperatorSettings(half, store.profiles); validationErr == nil {
		t.Fatal("half setup pair was accepted")
	}
	value.CombatProfile = "necro_bone_spear"
	half.Characters["mrbones"] = value
	if validationErr := validateOperatorSettings(half, store.profiles); validationErr != nil {
		t.Fatalf("complete setup pair rejected: %v", validationErr)
	}

	schema1 := cloneOperatorSettings(initial)
	schema1.SchemaVersion = 1
	if mkdirErr := os.MkdirAll(filepath.Dir(store.path), 0o755); mkdirErr != nil {
		t.Fatal(mkdirErr)
	}
	if writeErr := os.WriteFile(store.path, mustMarshalOperatorSettings(schema1), 0o600); writeErr != nil {
		t.Fatal(writeErr)
	}
	_, err = store.Snapshot()
	var settingsErr *OperatorSettingsError
	if !errors.As(err, &settingsErr) || settingsErr.Code != Phase15ReasonConfigSchemaUnsupported {
		t.Fatalf("schema-1 error=%v", err)
	}
}

func TestOperatorSettingsPersistsConfirmedSelectionWithoutReplacingOtherValues(t *testing.T) {
	store, _ := newOperatorSettingsTestStore(t)
	initial, err := store.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	initial.LastCharacter = ""
	initial.Characters["mrbones"] = OperatorCharacterSettings{LastDifficulty: "normal", Queue: []string{"mephisto", "countess"}}
	cleared, err := store.Update(initial.Revision, initial)
	if err != nil {
		t.Fatal(err)
	}

	change, err := store.ConfirmSelection("MrBones", "hell")
	if err != nil {
		t.Fatal(err)
	}
	got := change.Settings
	if got.LastCharacter != "MrBones" || got.Characters["mrbones"].LastDifficulty != "hell" {
		t.Fatalf("selection=%+v", got)
	}
	if want := []string{"mephisto", "countess"}; !reflect.DeepEqual(got.Characters["mrbones"].Queue, want) {
		t.Fatalf("queue=%v want=%v", got.Characters["mrbones"].Queue, want)
	}
	if got.Budgets != cleared.Settings.Budgets || got.Input != cleared.Settings.Input || got.History != cleared.Settings.History {
		t.Fatal("selection persistence replaced unrelated operator settings")
	}
}

func TestOperatorSettingsRejectsDuplicateUnknownAndStaleWithoutMutation(t *testing.T) {
	store, _ := newOperatorSettingsTestStore(t)
	initial, _ := store.Snapshot()
	tests := []struct {
		name   string
		mutate func(*OperatorSettings)
	}{
		{name: "duplicate", mutate: func(value *OperatorSettings) {
			value.Characters["mrbones"] = OperatorCharacterSettings{LastDifficulty: "normal", Queue: []string{"countess", "countess"}}
		}},
		{name: "unknown run", mutate: func(value *OperatorSettings) {
			value.Characters["mrbones"] = OperatorCharacterSettings{LastDifficulty: "normal", Queue: []string{"andariel"}}
		}},
		{name: "invalid character key", mutate: func(value *OperatorSettings) {
			value.Characters["invalid name"] = OperatorCharacterSettings{LastDifficulty: "normal", Queue: []string{"countess"}}
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			replacement := cloneOperatorSettings(initial)
			test.mutate(&replacement)
			if _, err := store.Update(initial.Revision, replacement); err == nil {
				t.Fatal("invalid settings were accepted")
			}
			current, _ := store.Snapshot()
			if current.Revision != initial.Revision {
				t.Fatalf("revision changed to %d", current.Revision)
			}
		})
	}
	stale := cloneOperatorSettings(initial)
	stale.Budgets.MaxRuns++
	_, err := store.Update(99, stale)
	var settingsErr *OperatorSettingsError
	if !errors.As(err, &settingsErr) || settingsErr.Code != Phase15ReasonConfigRevisionConflict {
		t.Fatalf("err=%v", err)
	}
}

func TestOperatorSettingsWriteAndReReadFailuresKeepOldEffectiveState(t *testing.T) {
	t.Run("write", func(t *testing.T) {
		store, _ := newOperatorSettingsTestStore(t)
		initial, _ := store.Snapshot()
		original, _ := os.ReadFile(store.path)
		store.write = func(string, []byte, string) error { return fs.ErrPermission }
		replacement := cloneOperatorSettings(initial)
		replacement.Budgets.MaxRuns++
		if _, err := store.Update(initial.Revision, replacement); err == nil {
			t.Fatal("write error was ignored")
		}
		body, _ := os.ReadFile(store.path)
		if string(body) != string(original) || store.effective.Revision != initial.Revision {
			t.Fatal("old effective state changed")
		}
	})

	t.Run("re-read", func(t *testing.T) {
		store, _ := newOperatorSettingsTestStore(t)
		initial, _ := store.Snapshot()
		original, _ := os.ReadFile(store.path)
		reads := 0
		store.read = func(path string) ([]byte, error) {
			reads++
			if reads == 2 {
				return nil, errors.New("injected re-read error")
			}
			return os.ReadFile(path)
		}
		replacement := cloneOperatorSettings(initial)
		replacement.Budgets.MaxRuns++
		if _, err := store.Update(initial.Revision, replacement); err == nil {
			t.Fatal("re-read error was ignored")
		}
		body, _ := os.ReadFile(store.path)
		if string(body) != string(original) || store.effective.Revision != initial.Revision {
			t.Fatal("old effective state or file changed")
		}
	})
}

func TestOperatorSettingsResetRestartAndBackupLimit(t *testing.T) {
	store, root := newOperatorSettingsTestStore(t)
	current, _ := store.Snapshot()
	inputChange := cloneOperatorSettings(current)
	inputChange.Input.Enabled = true
	changed, err := store.Update(current.Revision, inputChange)
	if err != nil || !changed.RestartRequired || changed.Settings.Revision != 2 {
		t.Fatalf("changed=%+v err=%v", changed, err)
	}
	resetPreview, err := store.PreviewReset(changed.Settings.Revision)
	if err != nil || !resetPreview.RestartRequired || resetPreview.Settings.Input.Enabled {
		t.Fatalf("reset preview=%+v err=%v", resetPreview, err)
	}
	afterPreview, _ := store.Snapshot()
	if afterPreview.Revision != changed.Settings.Revision || !afterPreview.Input.Enabled {
		t.Fatal("reset preview mutated the store")
	}
	reset, err := store.Reset(changed.Settings.Revision)
	if err != nil || !reset.RestartRequired || reset.Settings.Input.Enabled || reset.Settings.Revision != 3 {
		t.Fatalf("reset=%+v err=%v", reset, err)
	}
	current = reset.Settings
	for iteration := 0; iteration < 11; iteration++ {
		replacement := cloneOperatorSettings(current)
		replacement.Budgets.MaxRuns = 10 + iteration
		change, updateErr := store.Update(current.Revision, replacement)
		if updateErr != nil {
			t.Fatal(updateErr)
		}
		current = change.Settings
	}
	entries, err := os.ReadDir(filepath.Join(root, "backups"))
	if err != nil || len(entries) != operatorSettingsBackupLimit {
		t.Fatalf("backups=%d err=%v", len(entries), err)
	}
}

func TestOperatorSettingsStrictSchemaAndDifficulty(t *testing.T) {
	store, _ := newOperatorSettingsTestStore(t)
	if err := os.MkdirAll(filepath.Dir(store.path), 0o755); err != nil {
		t.Fatal(err)
	}
	unknown := "schema_version: 1\nrevision: 1\ncharacters: {}\nbudgets: {max_runs: 3, max_duration_ms: 1000, max_consecutive_failures: 1, max_total_restarts: 1}\ninput: {enabled: false, pause_hotkey: pause, stop_after_run_hotkey: f10, recording_finish_hotkey: f9, emergency_stop_hotkey: f11}\nhistory: {retention_enabled: true, retention_days: 60}\nunknown: true\n"
	if err := os.WriteFile(store.path, []byte(unknown), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Snapshot(); err == nil {
		t.Fatal("unknown field was accepted")
	}

	validStore, _ := newOperatorSettingsTestStore(t)
	current, _ := validStore.Snapshot()
	replacement := cloneOperatorSettings(current)
	replacement.Characters["mrbones"] = OperatorCharacterSettings{LastDifficulty: "inferno", Queue: []string{"countess"}}
	if _, err := validStore.Preview(current.Revision, replacement); err == nil {
		t.Fatal("invalid difficulty was accepted")
	}
}

func newOperatorSettingsTestStore(t *testing.T) (*OperatorSettingsStore, string) {
	t.Helper()
	root := t.TempDir()
	cfg, err := config.Load(filepath.Join("..", "..", "configs", "config.example.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	cfg.Session.Character = "MrBones"
	store, err := NewOperatorSettingsStore(root, cfg, []string{"MrBones", "MrHammer"})
	if err != nil {
		t.Fatal(err)
	}
	return store, root
}
