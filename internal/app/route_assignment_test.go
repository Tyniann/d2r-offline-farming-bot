package app

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/tasks"
)

func TestRouteAssignmentMigrationIsIdempotentAndRemovesLegacyFields(t *testing.T) {
	source, err := os.ReadFile(filepath.Join("..", "..", "configs", "config.example.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	body := strings.Replace(string(source), "    countess:\n      combat:", "    countess:\n      route_id: black-marsh-cellar5-nightmare-mrbones\n      combat:", 1)
	body = strings.Replace(body, "    mephisto:\n      #", "    mephisto:\n      route_id: durance-2-mephisto-nightmare-mrbones\n      #", 1)
	directory := t.TempDir()
	configPath := filepath.Join(directory, "config.yaml")
	if writeErr := os.WriteFile(configPath, []byte(body), 0o600); writeErr != nil {
		t.Fatal(writeErr)
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Session.Character = "MrBones"
	cfg.Routes.AssignmentsFile = "route-assignments.local.yaml"
	store, _ := NewRouteAssignmentStore(cfg)
	first, err := store.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if first.Revision != second.Revision || first.Assignments["mrbones"][tasks.RunIDCountess] == "" || first.Assignments["mrbones"][tasks.RunIDMephisto] == "" {
		t.Fatalf("migration=%+v second=%+v", first, second)
	}
	migrated, _ := os.ReadFile(configPath)
	definitions := string(migrated[:strings.Index(string(migrated), "town:")])
	if strings.Contains(definitions, "route_id:") {
		t.Fatalf("legacy field remained:\n%s", definitions)
	}
	if strings.Contains(string(migrated), "route_id: act3-egress") {
		t.Fatal("obsolete system Egress route_id remained")
	}
}

func TestRouteAssignmentCommitSupportsCharactersRunsAndRejectsStaleParallelConfirm(t *testing.T) {
	cfg := &config.Config{LoadedFrom: filepath.Join(t.TempDir(), "config.yaml"), Routes: config.RoutesConfig{AssignmentsFile: "assignments.yaml"}, Session: config.SessionConfig{Character: "MrBones"}}
	writeTestRouteAssignments(t, cfg, map[tasks.RunID]string{tasks.RunIDCountess: "countess-a"})
	store, _ := NewRouteAssignmentStore(cfg)
	snapshot, _ := store.Snapshot()
	next := cloneRouteAssignmentManifest(snapshot).Assignments
	next["mrbones"][tasks.RunIDMephisto] = "mephisto-a"
	next["sorc"] = map[tasks.RunID]string{tasks.RunIDCountess: "countess-sorc"}
	var successes int
	var mu sync.Mutex
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := store.Commit(snapshot.Revision, next); err == nil {
				mu.Lock()
				successes++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if successes != 1 {
		t.Fatalf("parallel successes=%d", successes)
	}
	final, _ := store.Snapshot()
	if final.Assignments["mrbones"][tasks.RunIDMephisto] != "mephisto-a" || final.Assignments["sorc"][tasks.RunIDCountess] != "countess-sorc" {
		t.Fatalf("final=%+v", final)
	}
}

func TestRouteAssignmentCorruptionAndAtomicWriteFailureAreFailClosed(t *testing.T) {
	cfg := &config.Config{LoadedFrom: filepath.Join(t.TempDir(), "config.yaml"), Routes: config.RoutesConfig{AssignmentsFile: "assignments.yaml"}, Session: config.SessionConfig{Character: "MrBones"}}
	path := cfg.ResolvePath(cfg.Routes.AssignmentsFile)
	if err := os.WriteFile(path, []byte("schema_version: ["), 0o600); err != nil {
		t.Fatal(err)
	}
	store, _ := NewRouteAssignmentStore(cfg)
	if _, err := store.Snapshot(); err == nil {
		t.Fatal("corrupt manifest accepted")
	}
	_ = os.Remove(path)
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatal(err)
	}
	store, _ = NewRouteAssignmentStore(cfg)
	if _, err := store.Snapshot(); err == nil {
		t.Fatal("assignment write failure accepted")
	}
}
