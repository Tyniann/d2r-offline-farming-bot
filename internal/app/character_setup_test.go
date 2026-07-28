package app

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/tasks"
)

func TestCharacterSetupPreviewConfirmAndIdempotentRetry(t *testing.T) {
	service, store, assignments, _ := newCharacterSetupTestService(t, nil)
	preview, err := service.Preview("mrbones")
	if err != nil {
		t.Fatal(err)
	}
	if preview.SetupState != CharacterSetupNeedsSetup || !preview.Supported || preview.DefaultProfileID != "necro_bone_spear" ||
		len(preview.Profiles) != 1 || preview.Profiles[0].DisplayName != "Knochen-Speer" {
		t.Fatalf("preview=%+v", preview)
	}
	confirmed, err := service.Confirm(CharacterSetupConfirmRequest{
		Character: "MrBones", ExpectedCatalogRevision: preview.CatalogRevision,
		ExpectedSettingsRevision: preview.OperatorSettingsRevision, ExpectedPickitRevision: preview.PickitAssignmentRevision,
	})
	if err != nil {
		t.Fatal(err)
	}
	if confirmed.SetupState != CharacterSetupNeedsAnchor || confirmed.SelectedProfileID != "necro_bone_spear" ||
		confirmed.OperatorSettingsRevision != 2 || confirmed.PickitAssignmentRevision != 2 {
		t.Fatalf("confirmed=%+v", confirmed)
	}
	manifest, _ := assignments.Snapshot()
	if !reflect.DeepEqual(manifest.Assignments["MrBones"][tasks.RunIDCountess], []string{"gems", "keys", "countess-standard"}) {
		t.Fatalf("manifest=%+v", manifest)
	}
	retry, err := service.Confirm(CharacterSetupConfirmRequest{
		Character: "MrBones", ProfileID: "necro_bone_spear", ExpectedCatalogRevision: confirmed.CatalogRevision,
		ExpectedSettingsRevision: confirmed.OperatorSettingsRevision, ExpectedPickitRevision: confirmed.PickitAssignmentRevision,
	})
	if err != nil || retry.OperatorSettingsRevision != 2 || retry.PickitAssignmentRevision != 2 {
		t.Fatalf("retry=%+v err=%v", retry, err)
	}
	settings, _ := store.Snapshot()
	if settings.Characters["mrbones"].CharacterClass != "necromancer" {
		t.Fatalf("settings=%+v", settings)
	}
}

func TestCharacterSetupStaleAndInvalidProfileWriteNothing(t *testing.T) {
	service, store, assignments, _ := newCharacterSetupTestService(t, nil)
	preview, _ := service.Preview("MrBones")
	tests := []CharacterSetupConfirmRequest{
		{Character: "MrBones", ExpectedCatalogRevision: 99, ExpectedSettingsRevision: 1, ExpectedPickitRevision: 1},
		{Character: "MrBones", ProfileID: "missing", ExpectedCatalogRevision: preview.CatalogRevision, ExpectedSettingsRevision: 1, ExpectedPickitRevision: 1},
	}
	for _, request := range tests {
		if _, err := service.Confirm(request); err == nil {
			t.Fatalf("request accepted: %+v", request)
		}
		settings, _ := store.Snapshot()
		manifest, _ := assignments.Snapshot()
		if settings.Revision != 1 || manifest.Revision != 1 {
			t.Fatalf("stale/invalid request mutated stores: settings=%d pickit=%d", settings.Revision, manifest.Revision)
		}
	}
}

func TestCharacterSetupPartialFailureIsSafeAndRetryCompletes(t *testing.T) {
	service, store, assignments, _ := newCharacterSetupTestService(t, nil)
	preview, _ := service.Preview("MrBones")
	originalWrite := assignments.write
	assignments.write = func(string, []byte, string) error { return errors.New("injected pickit write") }
	_, err := service.Confirm(CharacterSetupConfirmRequest{
		Character: "MrBones", ExpectedCatalogRevision: preview.CatalogRevision,
		ExpectedSettingsRevision: 1, ExpectedPickitRevision: 1,
	})
	var setupErr *CharacterSetupError
	if !errors.As(err, &setupErr) || !setupErr.Partial {
		t.Fatalf("partial error=%v", err)
	}
	settings, _ := store.Snapshot()
	manifest, _ := assignments.Snapshot()
	if settings.Revision != 2 || settings.Characters["mrbones"].CombatProfile == "" || manifest.Revision != 1 {
		t.Fatalf("unsafe partial state settings=%+v manifest=%+v", settings, manifest)
	}
	assignments.write = originalWrite
	current, _ := service.Preview("MrBones")
	completed, err := service.Confirm(CharacterSetupConfirmRequest{
		Character: "MrBones", ProfileID: "necro_bone_spear", ExpectedCatalogRevision: current.CatalogRevision,
		ExpectedSettingsRevision: current.OperatorSettingsRevision, ExpectedPickitRevision: current.PickitAssignmentRevision,
	})
	if err != nil || completed.PickitAssignmentRevision != 2 {
		t.Fatalf("retry=%+v err=%v", completed, err)
	}
}

func TestCharacterSetupCapturePublishesOnceAndRejectsExistingAnchor(t *testing.T) {
	var captures int
	service, _, _, anchorPath := newCharacterSetupTestService(t, func(_ context.Context, path string) error {
		captures++
		writeCatalogPNG(t, path, 210, 60)
		return nil
	})
	preview, _ := service.Preview("MrBones")
	confirmed, err := service.Confirm(CharacterSetupConfirmRequest{
		Character: "MrBones", ExpectedCatalogRevision: preview.CatalogRevision,
		ExpectedSettingsRevision: 1, ExpectedPickitRevision: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	captured, err := service.Capture(context.Background(), CharacterSetupCaptureRequest{Character: "MrBones", ExpectedCatalogRevision: confirmed.CatalogRevision})
	if err != nil || captured.SetupState != CharacterSetupReady || captured.AnchorState != CharacterAnchorReady || captures != 1 {
		t.Fatalf("captured=%+v captures=%d err=%v", captured, captures, err)
	}
	if _, err = service.Capture(context.Background(), CharacterSetupCaptureRequest{Character: "MrBones", ExpectedCatalogRevision: captured.CatalogRevision}); err == nil {
		t.Fatal("existing valid anchor was overwritten")
	}
	if captures != 1 || !validPNGSize(anchorPath, phase16CharacterAnchorSize) {
		t.Fatalf("captures=%d anchor_valid=%t", captures, validPNGSize(anchorPath, phase16CharacterAnchorSize))
	}
}

func newCharacterSetupTestService(t *testing.T, capture CharacterSetupCaptureFunc) (*CharacterSetupService, *OperatorSettingsStore, *PickitAssignmentStore, string) {
	t.Helper()
	root := t.TempDir()
	cfg, err := config.Load(filepath.Join("..", "..", "configs", "config.example.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	cfg.LoadedFrom = filepath.Join(root, "config.yaml")
	anchorPath := filepath.Join(root, "ui", "characters", "mrbones-selected.png")
	catalog, err := newCharacterCatalogStore(func() (CharacterCatalog, error) {
		reasons := []string{CharacterReasonProfileMissing}
		if !validPNGSize(anchorPath, phase16CharacterAnchorSize) {
			reasons = append(reasons, CharacterReasonAnchorMissing)
		}
		return CharacterCatalog{Characters: []CharacterCatalogEntry{{
			Name: "MrBones", Slug: "mrbones", ExpectedClass: "necromancer", Reasons: reasons, AnchorPath: anchorPath,
		}}}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	settings, err := NewOperatorSettingsStore(root, cfg, []string{"MrBones"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = settings.Snapshot(); err != nil {
		t.Fatal(err)
	}
	profiles, err := NewPickitProfileService(filepath.Join("..", "..", "configs", "pickit", "profiles"))
	if err != nil {
		t.Fatal(err)
	}
	assignments, err := NewPickitAssignmentStore(filepath.Join(root, "pickit.yaml"), profiles)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = assignments.Initialize(map[string]map[tasks.RunID][]string{"MrBones": {}}); err != nil {
		t.Fatal(err)
	}
	service, err := NewCharacterSetupService(CharacterSetupDependencies{
		Config: cfg, Catalog: catalog, Settings: settings, PickitAssignments: assignments, PickitProfiles: profiles, Capture: capture,
	})
	if err != nil {
		t.Fatal(err)
	}
	return service, settings, assignments, anchorPath
}
