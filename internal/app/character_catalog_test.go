package app

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
)

func TestCharacterCatalogUsesOnlyRegularSaveNamesAndAvailability(t *testing.T) {
	root := t.TempDir()
	saves := filepath.Join(root, "saves")
	if err := os.MkdirAll(saves, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"MrBones.d2s", "Other.d2s", "bad name.d2s", "notes.txt"} {
		if err := os.WriteFile(filepath.Join(saves, name), []byte("content must never be read"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	configPath := filepath.Join(root, "config.yaml")
	writeCatalogPNG(t, filepath.Join(root, "ui", "character-play.png"), 203, 47)
	writeCatalogPNG(t, filepath.Join(root, "ui", "difficulty-dialog.png"), 180, 175)
	writeCatalogPNG(t, filepath.Join(root, "ui", "characters", "mrbones-selected.png"), 210, 60)
	cfg := catalogTestConfig(configPath)

	catalog, err := resolveCharacterCatalogAt(cfg, saves)
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.Characters) != 2 {
		t.Fatalf("characters = %+v", catalog.Characters)
	}
	if got := catalog.Characters[0]; got.Name != "MrBones" || got.Slug != "mrbones" || !got.Selectable || got.ExpectedClass != "necromancer" {
		t.Fatalf("configured character = %+v", got)
	}
	if got := catalog.Characters[1]; got.Name != "Other" || got.Selectable || len(got.Reasons) != 2 || got.Reasons[0] != CharacterReasonAnchorMissing || got.Reasons[1] != CharacterReasonUnconfigured {
		t.Fatalf("unconfigured character = %+v", got)
	}
}

func TestCharacterCatalogMarksMissingSaveAndAnchor(t *testing.T) {
	root := t.TempDir()
	saves := filepath.Join(root, "saves")
	if err := os.MkdirAll(saves, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := catalogTestConfig(filepath.Join(root, "config.yaml"))
	catalog, err := resolveCharacterCatalogAt(cfg, saves)
	if err != nil {
		t.Fatal(err)
	}
	entry := catalog.Characters[0]
	if entry.Name != "MrBones" || entry.Selectable || !containsFold(entry.Reasons, CharacterReasonSaveMissing) || !containsFold(entry.Reasons, CharacterReasonAnchorMissing) {
		t.Fatalf("missing configured character = %+v", entry)
	}
}

func TestReadCharacterSaveNamesRejectsSymlinkWhenSupported(t *testing.T) {
	directory := t.TempDir()
	target := filepath.Join(directory, "target.bin")
	if err := os.WriteFile(target, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(directory, "Linked.d2s")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	names, err := readCharacterSaveNames(directory)
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 0 {
		t.Fatalf("symlink was listed: %v", names)
	}
}

func catalogTestConfig(loadedFrom string) *config.Config {
	return &config.Config{
		LoadedFrom: loadedFrom,
		Session:    config.SessionConfig{Run: "countess", Character: "MrBones"},
		Runs: config.RunsConfig{Definitions: map[string]config.RunConfig{
			"countess": {Combat: config.CombatConfig{Profile: "necro"}},
		}},
		Profiles: config.ProfilesConfig{"necro": {CharacterClass: "necromancer"}},
	}
}

func writeCatalogPNG(t *testing.T, path string, width, height int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(file, image.NewRGBA(image.Rect(0, 0, width, height))); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}
