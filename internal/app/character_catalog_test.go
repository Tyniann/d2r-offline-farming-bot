package app

import (
	"errors"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
)

func TestCharacterCatalogUsesOnlyRegularSaveNamesAndAvailability(t *testing.T) {
	root := t.TempDir()
	saves := filepath.Join(root, "saves")
	if err := os.MkdirAll(saves, 0o755); err != nil {
		t.Fatal(err)
	}
	writeCharacterSaveTestFile(t, filepath.Join(saves, "MrBones.d2s"), "MrBones", 105, 2)
	writeCharacterSaveTestFile(t, filepath.Join(saves, "Other.d2s"), "Other", 105, 3)
	for _, name := range []string{"bad name.d2s", "notes.txt"} {
		if err := os.WriteFile(filepath.Join(saves, name), []byte("ignored"), 0o644); err != nil {
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
	if catalog.Revision != 1 || len(catalog.Characters) != 2 {
		t.Fatalf("characters = %+v", catalog.Characters)
	}
	if got := catalog.Characters[0]; got.Name != "MrBones" || got.Slug != "mrbones" || got.Selectable || got.ExpectedClass != "necromancer" ||
		!equalStrings(got.Reasons, []string{CharacterReasonProfileMissing}) {
		t.Fatalf("configured character = %+v", got)
	}
	if !filepath.IsAbs(catalog.Characters[0].AnchorPath) {
		t.Fatalf("configured character anchor is relative: %q", catalog.Characters[0].AnchorPath)
	}
	if got := catalog.Characters[1]; got.Name != "Other" || got.Selectable || got.ExpectedClass != "paladin" ||
		!equalStrings(got.Reasons, []string{CharacterReasonClassUnsupported}) {
		t.Fatalf("unsupported character = %+v", got)
	}
}

func TestCharacterCatalogAllowsAnchoredSaveWithEmptySessionDefault(t *testing.T) {
	root := t.TempDir()
	saves := filepath.Join(root, "saves")
	if err := os.MkdirAll(saves, 0o755); err != nil {
		t.Fatal(err)
	}
	writeCharacterSaveTestFile(t, filepath.Join(saves, "MrBones.d2s"), "MrBones", 105, 2)
	writeCatalogPNG(t, filepath.Join(root, "ui", "character-play.png"), 203, 47)
	writeCatalogPNG(t, filepath.Join(root, "ui", "difficulty-dialog.png"), 180, 175)
	writeCatalogPNG(t, filepath.Join(root, "ui", "characters", "mrbones-selected.png"), 210, 60)
	cfg := catalogTestConfig(filepath.Join(root, "config.yaml"))
	cfg.Session.Character = ""

	catalog, err := resolveCharacterCatalogAt(cfg, saves)
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.Characters) != 1 || catalog.Characters[0].Selectable || catalog.Characters[0].ExpectedClass != "necromancer" ||
		!equalStrings(catalog.Characters[0].Reasons, []string{CharacterReasonProfileMissing}) {
		t.Fatalf("fresh-root character = %+v", catalog.Characters)
	}
}

func TestCharacterCatalogNeverDerivesClassFromRunProfile(t *testing.T) {
	root := t.TempDir()
	saves := filepath.Join(root, "saves")
	if err := os.MkdirAll(saves, 0o755); err != nil {
		t.Fatal(err)
	}
	writeCharacterSaveTestFile(t, filepath.Join(saves, "MrBones.d2s"), "MrBones", 105, 3)
	writeCatalogPNG(t, filepath.Join(root, "ui", "character-play.png"), 203, 47)
	writeCatalogPNG(t, filepath.Join(root, "ui", "difficulty-dialog.png"), 180, 175)
	writeCatalogPNG(t, filepath.Join(root, "ui", "characters", "mrbones-selected.png"), 210, 60)
	cfg := catalogTestConfig(filepath.Join(root, "config.yaml"))
	cfg.Profiles = nil

	catalog, err := resolveCharacterCatalogAt(cfg, saves)
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.Characters) != 1 || catalog.Characters[0].ExpectedClass != "paladin" ||
		!equalStrings(catalog.Characters[0].Reasons, []string{CharacterReasonClassUnsupported}) {
		t.Fatalf("header class projection = %+v", catalog.Characters)
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
	if entry.Name != "MrBones" || entry.Selectable || !equalStrings(entry.Reasons, []string{CharacterReasonSaveMissing}) {
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
	saves, err := readCharacterSaves(directory)
	if err != nil {
		t.Fatal(err)
	}
	if len(saves) != 0 {
		t.Fatalf("symlink was listed: %+v", saves)
	}
}

func TestCharacterCatalogIsolatesBrokenSaveBesideValidSave(t *testing.T) {
	root := t.TempDir()
	saves := filepath.Join(root, "saves")
	if err := os.MkdirAll(saves, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(saves, "Broken.d2s"), []byte("short"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeCharacterSaveTestFile(t, filepath.Join(saves, "MrBook.d2s"), "MrBook", 105, 7)
	cfg := catalogTestConfig(filepath.Join(root, "config.yaml"))
	cfg.Session.Character = ""

	catalog, err := resolveCharacterCatalogAt(cfg, saves)
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.Characters) != 2 {
		t.Fatalf("catalog=%+v", catalog)
	}
	if got := catalog.Characters[0]; got.Name != "Broken" || !equalStrings(got.Reasons, []string{CharacterReasonSaveHeaderInvalid}) {
		t.Fatalf("broken=%+v", got)
	}
	if got := catalog.Characters[1]; got.Name != "MrBook" || got.ExpectedClass != "warlock" ||
		!equalStrings(got.Reasons, []string{CharacterReasonClassUnsupported}) {
		t.Fatalf("warlock=%+v", got)
	}
}

func TestReadCharacterSavesKeepsDisappearedEntryVisible(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "Gone.d2s")
	writeCharacterSaveTestFile(t, path, "Gone", 105, 2)
	items, err := readCharacterSavesWith(
		directory,
		os.ReadDir,
		func(string) (os.FileInfo, error) { return nil, os.ErrNotExist },
		func(string, string) (characterSaveHeader, error) {
			t.Fatal("header reader must not run after lstat failure")
			return characterSaveHeader{}, nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].name != "Gone" || items[0].reason != Phase16ReasonCharacterSaveUnreadable {
		t.Fatalf("items=%+v", items)
	}
}

func TestCharacterSaveNamesRejectCaseInsensitiveConflict(t *testing.T) {
	seen := make(map[string]struct{})
	if duplicateCharacterSave(seen, "MrBones") {
		t.Fatal("first name reported as duplicate")
	}
	if !duplicateCharacterSave(seen, "mrbones") {
		t.Fatal("case-insensitive duplicate was accepted")
	}
}

func TestReadCharacterSavesRejectsCaseInsensitiveConflictWithoutPublishingPartialData(t *testing.T) {
	firstRoot := t.TempDir()
	secondRoot := t.TempDir()
	writeCharacterSaveTestFile(t, filepath.Join(firstRoot, "MrBones.d2s"), "MrBones", 105, 2)
	writeCharacterSaveTestFile(t, filepath.Join(secondRoot, "mrbones.d2s"), "mrbones", 105, 2)
	firstEntries, err := os.ReadDir(firstRoot)
	if err != nil {
		t.Fatal(err)
	}
	secondEntries, err := os.ReadDir(secondRoot)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(filepath.Join(firstRoot, "MrBones.d2s"))
	if err != nil {
		t.Fatal(err)
	}
	items, err := readCharacterSavesWith(
		"unused",
		func(string) ([]os.DirEntry, error) { return []os.DirEntry{firstEntries[0], secondEntries[0]}, nil },
		func(string) (os.FileInfo, error) { return info, nil },
		func(_ string, name string) (characterSaveHeader, error) {
			return characterSaveHeader{Name: name, SaveVersion: 105}, nil
		},
	)
	if items != nil {
		t.Fatalf("partial items=%+v", items)
	}
	var catalogErr *CharacterCatalogError
	if !errors.As(err, &catalogErr) || catalogErr.Reason != Phase16ReasonCharacterSaveNameConflict {
		t.Fatalf("err=%v", err)
	}
}

func TestCharacterCatalogStoreRevisesOnlyChangedSuccessfulProjection(t *testing.T) {
	calls := 0
	store, err := newCharacterCatalogStore(func() (CharacterCatalog, error) {
		calls++
		switch calls {
		case 1, 2:
			return CharacterCatalog{Characters: []CharacterCatalogEntry{{Name: "MrBones", Reasons: []string{CharacterReasonProfileMissing}}}}, nil
		case 3:
			return CharacterCatalog{Characters: []CharacterCatalogEntry{{Name: "MrBones"}, {Name: "MrBook"}}}, nil
		default:
			return CharacterCatalog{}, errors.New("reload failed")
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if snapshot := store.Snapshot(); snapshot.Revision != 1 {
		t.Fatalf("initial=%+v", snapshot)
	}
	same, err := store.Reload()
	if err != nil || same.Revision != 1 {
		t.Fatalf("same=%+v err=%v", same, err)
	}
	changed, err := store.Reload()
	if err != nil || changed.Revision != 2 || len(changed.Characters) != 2 {
		t.Fatalf("changed=%+v err=%v", changed, err)
	}
	failed, err := store.Reload()
	if err == nil || failed.Revision != 2 || len(failed.Characters) != 2 {
		t.Fatalf("failed=%+v err=%v", failed, err)
	}
	failed.Characters[0].Name = "mutated"
	if store.Snapshot().Characters[0].Name != "MrBones" {
		t.Fatal("snapshot mutation reached published catalog")
	}
}

func TestCharacterCatalogStoreProjectsOnlyMatchingPersistedProfileAsSelectable(t *testing.T) {
	cfg, err := config.Load("../../configs/config.example.yaml")
	if err != nil {
		t.Fatal(err)
	}
	settings, err := NewOperatorSettingsStore(t.TempDir(), cfg, []string{"MrBones"})
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := settings.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	store, err := newCharacterCatalogStore(func() (CharacterCatalog, error) {
		return CharacterCatalog{Characters: []CharacterCatalogEntry{{
			Name: "MrBones", Slug: "mrbones", ExpectedClass: "necromancer",
			Reasons: []string{CharacterReasonProfileMissing},
		}}}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	store.cfg = cfg
	missing, err := store.BindOperatorSettings(settings)
	if err != nil {
		t.Fatal(err)
	}
	if got := missing.Characters[0]; got.Selectable || got.CombatProfile != "" || !equalStrings(got.Reasons, []string{CharacterReasonProfileMissing}) {
		t.Fatalf("missing setup projection = %+v", got)
	}
	if _, err = settings.AssignCharacterProfile("MrBones", "necromancer", "necro_bone_spear", snapshot.Revision); err != nil {
		t.Fatal(err)
	}
	ready, err := store.Reload()
	if err != nil {
		t.Fatal(err)
	}
	if got := ready.Characters[0]; !got.Selectable || got.CombatProfile != "necro_bone_spear" || len(got.Reasons) != 0 {
		t.Fatalf("ready setup projection = %+v", got)
	}
	unchanged, err := store.Reload()
	if err != nil {
		t.Fatal(err)
	}
	if unchanged.Revision != ready.Revision {
		t.Fatalf("unchanged ready reload revision = %d, want %d", unchanged.Revision, ready.Revision)
	}
	delete(cfg.Profiles, "necro_bone_spear")
	incompatible, err := store.Reload()
	if err != nil {
		t.Fatal(err)
	}
	if got := incompatible.Characters[0]; got.Selectable || got.CombatProfile != "" || !equalStrings(got.Reasons, []string{CharacterReasonProfileIncompatible}) {
		t.Fatalf("incompatible setup projection = %+v", got)
	}
}

func TestCharacterCatalogConflictErrorCarriesStableReason(t *testing.T) {
	err := &CharacterCatalogError{Reason: Phase16ReasonCharacterSaveNameConflict, Err: errors.New("conflict")}
	if !strings.Contains(err.Error(), CharacterReasonSaveNameConflict) || !errors.Is(err, err.Err) {
		t.Fatalf("err=%v", err)
	}
}

func TestPhase16LocalAuthenticCatalog(t *testing.T) {
	if os.Getenv("D2RBOT_TEST_LOCAL_PHASE16_SAVES") != "1" {
		t.Skip("set D2RBOT_TEST_LOCAL_PHASE16_SAVES=1 only for the approved read-only local gate")
	}
	cfg, err := config.Load(filepath.Join("..", "..", "configs", "config.example.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := ResolveCharacterCatalog(cfg)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"MrBones": "necromancer", "MrHammer": "paladin", "MrBook": "warlock"}
	for name, class := range want {
		found := false
		for _, entry := range catalog.Characters {
			if strings.EqualFold(entry.Name, name) {
				found = true
				if entry.ExpectedClass != class {
					t.Fatalf("%s class=%q want=%q", name, entry.ExpectedClass, class)
				}
				break
			}
		}
		if !found {
			t.Fatalf("%s is missing from the local catalog", name)
		}
	}
}

func catalogTestConfig(loadedFrom string) *config.Config {
	return &config.Config{
		LoadedFrom: loadedFrom,
		Session:    config.SessionConfig{Run: "countess", Character: "MrBones"},
		Runs: config.RunsConfig{Definitions: map[string]config.RunConfig{
			"countess": {Combat: config.CombatConfig{}},
		}},
		Profiles: config.ProfilesConfig{"necro": {
			CharacterClass: "necromancer", DisplayName: "Testprofil",
			Setup: config.ProfileSetupConfig{Enabled: true, Default: true},
		}},
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
