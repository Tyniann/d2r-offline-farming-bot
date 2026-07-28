package app

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/tasks"
	"gopkg.in/yaml.v3"
)

func TestPhase16CharacterizationCatalogUsesSaveClassWithoutRunFallback(t *testing.T) {
	root := t.TempDir()
	saves := filepath.Join(root, "saves")
	if err := os.MkdirAll(saves, 0o755); err != nil {
		t.Fatal(err)
	}
	writeCharacterSaveTestFile(t, filepath.Join(saves, "MrBones.d2s"), "MrBones", 105, 2)
	writeCharacterSaveTestFile(t, filepath.Join(saves, "MrHammer.d2s"), "MrHammer", 105, 3)
	writeCatalogPNG(t, filepath.Join(root, "ui", "character-play.png"), 203, 47)
	writeCatalogPNG(t, filepath.Join(root, "ui", "difficulty-dialog.png"), 180, 175)
	writeCatalogPNG(t, filepath.Join(root, "ui", "characters", "mrbones-selected.png"), 210, 60)

	cfg := catalogTestConfig(filepath.Join(root, "config.yaml"))
	catalog, err := resolveCharacterCatalogAt(cfg, saves)
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.Characters) != 2 {
		t.Fatalf("characters=%+v", catalog.Characters)
	}
	if got := catalog.Characters[0]; got.Name != "MrBones" || got.ExpectedClass != "necromancer" || got.Selectable ||
		!reflect.DeepEqual(got.Reasons, []string{CharacterReasonProfileMissing}) {
		t.Fatalf("MrBones=%+v", got)
	}
	if got := catalog.Characters[1]; got.Name != "MrHammer" || got.ExpectedClass != "paladin" || got.Selectable ||
		!reflect.DeepEqual(got.Reasons, []string{CharacterReasonClassUnsupported}) {
		t.Fatalf("MrHammer=%+v", got)
	}
}

func TestPhase16CharacterizationOperatorSettingsStartAsSchema2WithEmptySetupFields(t *testing.T) {
	store, _ := newOperatorSettingsTestStore(t)
	settings, err := store.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if settings.SchemaVersion != 2 || OperatorSettingsSchemaVersion != 2 {
		t.Fatalf("operator schema=%d constant=%d", settings.SchemaVersion, OperatorSettingsSchemaVersion)
	}
	data, err := yaml.Marshal(settings)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range [][]byte{[]byte("character_class:"), []byte("combat_profile:")} {
		if bytes.Contains(data, forbidden) {
			t.Fatalf("empty schema-2 setup unexpectedly contains %q:\n%s", forbidden, data)
		}
	}
}

func TestPhase16CharacterizationPickitAssignmentsRemainRunSpecific(t *testing.T) {
	profiles, err := NewPickitProfileService(filepath.Join("..", "..", "configs", "pickit", "profiles"))
	if err != nil {
		t.Fatal(err)
	}
	assignments, err := NewPickitAssignmentStore(filepath.Join("..", "..", "configs", "pickit-assignments.example.yaml"), profiles)
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := assignments.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if manifest.SchemaVersion != 1 || manifest.Revision != 1 {
		t.Fatalf("assignment contract=%+v", manifest)
	}
	if got := findPickitAssignment(manifest, "mrbones", tasks.RunIDCountess); !reflect.DeepEqual(got, []string{"gems", "keys", "countess-standard"}) {
		t.Fatalf("Countess assignment=%v", got)
	}
	if got := findPickitAssignment(manifest, "MRBONES", tasks.RunIDMephisto); !reflect.DeepEqual(got, []string{"gems", "mephisto-standard"}) {
		t.Fatalf("Mephisto assignment=%v", got)
	}
	if got := findPickitAssignment(manifest, "MrBones", tasks.RunIDSummoner); !reflect.DeepEqual(got, []string{"gems", "keys"}) {
		t.Fatalf("Summoner assignment=%v", got)
	}
	if got := findPickitAssignment(manifest, "MrBones", tasks.RunIDNihlathak); !reflect.DeepEqual(got, []string{"gems", "keys"}) {
		t.Fatalf("Nihlathak assignment=%v", got)
	}
}

func TestPhase16CharacterizationExplicitRunsRetainDirectProfileClass(t *testing.T) {
	cfg, err := config.Load(filepath.Join("..", "..", "configs", "config.example.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, runID := range []string{"countess", "mephisto", "summoner", "nihlathak"} {
		cfg.Session.Run = runID
		run, ok := cfg.Runs.Run(runID)
		if !ok || run.Combat.Profile != "necro_bone_spear" {
			t.Fatalf("%s run=%+v found=%t", runID, run, ok)
		}
		if got := cfg.Profiles[run.Combat.Profile].CharacterClass; got != "necromancer" {
			t.Fatalf("%s direct profile class=%q", runID, got)
		}
	}
}
