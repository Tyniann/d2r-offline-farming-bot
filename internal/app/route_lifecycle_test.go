package app

import (
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/pathing"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/tasks"
)

func TestRouteLifecycleBootstrapExactContextDoesNotInvalidate(t *testing.T) {
	cfg, root := lifecycleTestConfig(t, "MrBones", "nightmare")
	path := saveLifecycleTestRoute(t, root, "MrBones", "nightmare", "countess-bootstrap", time.Now().Add(-time.Hour))
	store, err := NewRouteLifecycleStore(cfg)
	if err != nil {
		t.Fatal(err)
	}
	manifest, catalog, err := store.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if manifest.BootstrapExpected == nil || catalog.Revision != 1 || len(catalog.Entries) != 1 {
		t.Fatalf("bootstrap manifest=%+v catalog=%+v", manifest, catalog)
	}
	preview, err := store.Preview("MrBones", "nightmare")
	if err != nil {
		t.Fatal(err)
	}
	if preview.Reason != "" || len(preview.AffectedRoutes) != 0 {
		t.Fatalf("exact bootstrap preview = %+v", preview)
	}
	confirmed, err := store.Confirm(preview, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if confirmed.BootstrapExpected != nil || confirmed.Characters["mrbones"].LastConfirmedDifficulty != "nightmare" {
		t.Fatalf("confirmed manifest = %+v", confirmed)
	}
	_, catalog, err = store.Snapshot()
	if err != nil || catalog.Entries[0].Status != RouteLifecycleRuntimeValidationRequired {
		t.Fatalf("post-confirm catalog=%+v err=%v", catalog, err)
	}
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = file.WriteString("\n")
	_ = file.Close()
	_, catalog, err = store.Snapshot()
	if err != nil || catalog.Entries[0].Status != RouteLifecycleUnavailable || catalog.Entries[0].Reason != "route_file_changed" {
		t.Fatalf("changed route catalog=%+v err=%v", catalog, err)
	}
}

func TestRouteLifecycleDifficultyChangeInvalidatesOnlyCharacterFarmingRoutes(t *testing.T) {
	cfg, root := lifecycleTestConfig(t, "MrBones", "nightmare")
	saveLifecycleTestRoute(t, root, "MrBones", "nightmare", "countess-old", time.Now().Add(-2*time.Hour))
	saveLifecycleTestRoute(t, root, "MrBones", "hell", "mephisto-old", time.Now().Add(-2*time.Hour))
	saveLifecycleTestRoute(t, root, "MrHammer", "normal", "hammer-route", time.Now().Add(-2*time.Hour))
	store, _ := NewRouteLifecycleStore(cfg)
	preview, err := store.Preview("MrBones", "hell")
	if err != nil {
		t.Fatal(err)
	}
	if preview.Reason != "difficulty_changed" || !sameStrings(preview.AffectedRoutes, []string{"countess-old", "mephisto-old"}) {
		t.Fatalf("difficulty preview = %+v", preview)
	}
	invalidatedAt := time.Now().UTC()
	if _, confirmErr := store.Confirm(preview, invalidatedAt); confirmErr != nil {
		t.Fatal(confirmErr)
	}
	_, catalog, err := store.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	statuses := lifecycleStatuses(catalog)
	if statuses["countess-old"] != RouteLifecycleStale || statuses["mephisto-old"] != RouteLifecycleStale || statuses["hammer-route"] != RouteLifecycleRuntimeValidationRequired {
		t.Fatalf("statuses after difficulty change = %+v", statuses)
	}
	newPath := saveLifecycleTestRoute(t, root, "MrBones", "hell", "mephisto-new", invalidatedAt.Add(time.Second))
	if _, err := store.RecordRoute(newPath); err != nil {
		t.Fatal(err)
	}
	_, catalog, _ = store.Snapshot()
	statuses = lifecycleStatuses(catalog)
	if statuses["mephisto-new"] != RouteLifecycleRuntimeValidationRequired || statuses["mephisto-old"] != RouteLifecycleStale {
		t.Fatalf("new recording rehabilitated siblings: %+v", statuses)
	}
}

func TestRouteLifecycleCharacterSwitchAndSameDifficultyDoNotInvalidate(t *testing.T) {
	cfg, root := lifecycleTestConfig(t, "MrBones", "nightmare")
	saveLifecycleTestRoute(t, root, "MrBones", "nightmare", "mrbones-route", time.Now().Add(-time.Hour))
	saveLifecycleTestRoute(t, root, "MrHammer", "normal", "mrhammer-route", time.Now().Add(-time.Hour))
	store, _ := NewRouteLifecycleStore(cfg)
	exact, _ := store.Preview("MrBones", "nightmare")
	if _, err := store.Confirm(exact, time.Now()); err != nil {
		t.Fatal(err)
	}
	switchPreview, err := store.Preview("MrHammer", "normal")
	if err != nil {
		t.Fatal(err)
	}
	if switchPreview.Reason != "" || len(switchPreview.AffectedRoutes) != 0 {
		t.Fatalf("character switch preview = %+v", switchPreview)
	}
	if _, err := store.Confirm(switchPreview, time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	_, catalog, _ := store.Snapshot()
	for id, status := range lifecycleStatuses(catalog) {
		if status != RouteLifecycleRuntimeValidationRequired {
			t.Fatalf("route %s unexpectedly invalidated: %s", id, status)
		}
	}
}

func TestRouteLifecycleSameContextConfirmationIsRevisionIdempotent(t *testing.T) {
	cfg, root := lifecycleTestConfig(t, "MrBones", "nightmare")
	saveLifecycleTestRoute(t, root, "MrBones", "nightmare", "mrbones-route", time.Now().Add(-time.Hour))
	store, _ := NewRouteLifecycleStore(cfg)
	firstPreview, _ := store.Preview("MrBones", "nightmare")
	first, err := store.Confirm(firstPreview, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	secondPreview, _ := store.Preview("MrBones", "nightmare")
	second, err := store.Confirm(secondPreview, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if second.Revision != first.Revision || !second.Characters["mrbones"].ConfirmedAt.Equal(*first.Characters["mrbones"].ConfirmedAt) {
		t.Fatalf("same-context confirmation mutated lifecycle: first=%+v second=%+v", first, second)
	}
}

func TestRouteLifecycleRejectsDuplicateCorruptAndStaleRevision(t *testing.T) {
	cfg, root := lifecycleTestConfig(t, "MrBones", "nightmare")
	saveLifecycleTestRoute(t, root, "MrBones", "nightmare", "duplicate-route", time.Now())
	saveLifecycleTestRoute(t, root, "MrBones", "hell", "duplicate-route", time.Now())
	saveLifecycleTestRoute(t, root, "MrBones", "normal", "unique-route", time.Now())
	store, _ := NewRouteLifecycleStore(cfg)
	_, catalog, err := store.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range catalog.Entries {
		if entry.ID == "unique-route" {
			continue
		}
		if entry.Status != RouteLifecycleUnavailable || !strings.HasPrefix(entry.Reason, "route_duplicate_id:") {
			t.Fatalf("duplicate entry = %+v", entry)
		}
	}
	preview, _ := store.Preview("MrBones", "hell")
	manifest, _, _ := store.Snapshot()
	if _, err := store.InvalidateLayout("MrBones", manifest.Revision, time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Confirm(preview, time.Now()); err == nil || !strings.Contains(err.Error(), "revision changed") {
		t.Fatalf("stale preview error = %v", err)
	}
	if err := os.WriteFile(cfg.ResolvePath(cfg.Routes.LifecycleFile), []byte("schema_version: ["), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.Snapshot(); err == nil || !strings.Contains(err.Error(), "decode route lifecycle") {
		t.Fatalf("corrupt manifest error = %v", err)
	}
}

func TestRouteLifecycleLayoutMismatchInvalidatesOnlyOneCharacter(t *testing.T) {
	cfg, root := lifecycleTestConfig(t, "MrBones", "nightmare")
	saveLifecycleTestRoute(t, root, "MrBones", "nightmare", "mrbones-layout", time.Now().Add(-time.Hour))
	saveLifecycleTestRoute(t, root, "MrHammer", "normal", "mrhammer-layout", time.Now().Add(-time.Hour))
	store, _ := NewRouteLifecycleStore(cfg)
	manifest, _, err := store.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.InvalidateLayout("MrBones", manifest.Revision, time.Now()); err != nil {
		t.Fatal(err)
	}
	_, catalog, _ := store.Snapshot()
	statuses := lifecycleStatuses(catalog)
	if statuses["mrbones-layout"] != RouteLifecycleStale || statuses["mrhammer-layout"] != RouteLifecycleRuntimeValidationRequired {
		t.Fatalf("layout statuses = %+v", statuses)
	}
}

func TestRoutePlaybackLayoutMismatchInvalidatesBeforePlayerCreation(t *testing.T) {
	cfg, root := lifecycleTestConfig(t, "MrBones", "nightmare")
	saveLifecycleTestRoute(t, root, "MrBones", "nightmare", "layout-preinput", time.Now().Add(-time.Hour))
	store, _ := NewRouteLifecycleStore(cfg)
	if _, _, err := store.Snapshot(); err != nil {
		t.Fatal(err)
	}
	state, _ := egressTestState(t)
	adapter := newRoutePlaybackAdapter(slog.Default(), root, "3.2.92777", nil, nil, store)
	err := adapter.Start("layout-preinput", state)
	if !errors.Is(err, pathing.ErrRouteLayoutMismatch) {
		t.Fatalf("Start() error = %v, want layout mismatch", err)
	}
	if adapter.player != nil {
		t.Fatal("route player was created before lifecycle invalidation")
	}
	_, catalog, snapshotErr := store.Snapshot()
	if snapshotErr != nil || lifecycleStatuses(catalog)["layout-preinput"] != RouteLifecycleStale {
		t.Fatalf("catalog after mismatch=%+v err=%v", catalog, snapshotErr)
	}
}

func TestRouteLifecycleWriteFailurePublishesNoManifest(t *testing.T) {
	cfg, root := lifecycleTestConfig(t, "MrBones", "nightmare")
	saveLifecycleTestRoute(t, root, "MrBones", "nightmare", "write-failure", time.Now())
	cfg.Routes.LifecycleFile = "blocked"
	if err := os.Mkdir(cfg.ResolvePath(cfg.Routes.LifecycleFile), 0o755); err != nil {
		t.Fatal(err)
	}
	store, _ := NewRouteLifecycleStore(cfg)
	if _, _, err := store.Snapshot(); err == nil {
		t.Fatalf("write failure error = %v", err)
	}
}

func TestRouteLifecycleManagementIsOrthogonalToInvalidation(t *testing.T) {
	cfg, root := lifecycleTestConfig(t, "MrBones", "nightmare")
	saveLifecycleTestRoute(t, root, "MrBones", "nightmare", "managed-route", time.Now().Add(-time.Hour))
	store, _ := NewRouteLifecycleStore(cfg)
	manifest, _, err := store.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	manifest, err = store.SetManagement("managed-route", RouteManagementArchived, tasks.RunIDCountess, manifest.Revision)
	if err != nil {
		t.Fatal(err)
	}
	manifest, err = store.InvalidateLayout("MrBones", manifest.Revision, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	route := manifest.Characters["mrbones"].Routes["managed-route"]
	if route.ManagementStatus != RouteManagementArchived || route.InvalidatedAt == nil || route.InvalidationReason != "layout_mismatch_detected" {
		t.Fatalf("orthogonal route=%+v", route)
	}
}

func lifecycleTestConfig(t *testing.T, character, difficulty string) (*config.Config, string) {
	t.Helper()
	directory := t.TempDir()
	cfg := &config.Config{
		LoadedFrom: filepath.Join(directory, "config.yaml"),
		Routes:     config.RoutesConfig{FarmingRoot: "routes/farming", LifecycleFile: "route-lifecycle.local.yaml"},
		Session:    config.SessionConfig{Character: character, Difficulty: difficulty},
	}
	return cfg, cfg.ResolvePath(cfg.Routes.FarmingRoot)
}

func saveLifecycleTestRoute(t *testing.T, root, character, difficulty, id string, recordedAt time.Time) string {
	t.Helper()
	route, err := pathing.LoadRoute(filepath.Join("..", "..", "configs", "routes", "farming", "mrbones", "nightmare", "black-marsh-cellar5-nightmare-mrbones.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	route.ID, route.Name = id, id
	route.Binding.CharacterName = character
	if strings.EqualFold(character, "MrHammer") {
		route.Binding.CharacterClass = "paladin"
	}
	route.Binding.Difficulty = pathing.RouteDifficulty(difficulty)
	route.Recording.RecordedAt = recordedAt.UTC()
	path := filepath.Join(root, strings.ToLower(character), difficulty, id+".yaml")
	if err := pathing.SaveRoute(path, route); err != nil {
		t.Fatal(err)
	}
	return path
}

func lifecycleStatuses(catalog FarmingRouteCatalog) map[string]RouteLifecycleStatus {
	statuses := make(map[string]RouteLifecycleStatus, len(catalog.Entries))
	for _, entry := range catalog.Entries {
		statuses[entry.ID] = entry.Status
	}
	return statuses
}
