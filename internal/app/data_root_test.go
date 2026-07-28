package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/telemetry"
)

func TestDataRootFreshDefaultsAndRerunAreAtomic(t *testing.T) {
	bundle := buildTestDefaultBundle(t)
	for _, relative := range []string{"configs/ui/character-play.png", "configs/ui/difficulty-dialog.png"} {
		if _, err := os.Stat(filepath.Join(bundle, filepath.FromSlash(relative))); err != nil {
			t.Fatalf("missing global UI evidence %s: %v", relative, err)
		}
	}
	if _, err := os.Stat(filepath.Join(bundle, "configs", "ui", "characters", "mrbones-selected.png")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("fresh default bundle contains character-specific evidence: %v", err)
	}
	target := filepath.Join(t.TempDir(), Phase15DataRootDirectoryName)
	manager, err := NewDataRootManager(target)
	if err != nil {
		t.Fatal(err)
	}
	result, err := manager.InitializeDefaults(context.Background(), bundle)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != DataRootPublished || result.Root != target || len(result.Diagnostics) != 0 {
		t.Fatalf("result=%+v", result)
	}
	if _, loadErr := config.LoadFromDataRoot(target); loadErr != nil {
		t.Fatalf("load published root: %v", loadErr)
	}
	for _, relative := range []string{"configs/config.yaml", "configs/operator-settings.local.yaml", "configs/pickit-assignments.local.yaml", "configs/route-assignments.local.yaml", "configs/route-lifecycle.local.yaml", "logs/telemetry", "backups", "diagnostics"} {
		if _, statErr := os.Stat(filepath.Join(target, filepath.FromSlash(relative))); statErr != nil {
			t.Fatalf("missing %s: %v", relative, statErr)
		}
	}
	digest := treeDigest(t, target)
	runAgain, err := manager.InitializeDefaults(context.Background(), bundle)
	if err != nil || runAgain.Status != DataRootExisting {
		t.Fatalf("rerun=%+v err=%v", runAgain, err)
	}
	if got := treeDigest(t, target); got != digest {
		t.Fatalf("rerun changed root: %s != %s", got, digest)
	}
	assertNoDataRootStaging(t, target)
}

func TestDataRootImportKeepsSourceAndExcludesRuntimeArtifacts(t *testing.T) {
	source := initializeTestRoot(t)
	if err := os.WriteFile(filepath.Join(source, "logs", "d2rbot-old.log"), []byte("old log"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "diagnostics", "old.zip"), []byte("zip"), 0o600); err != nil {
		t.Fatal(err)
	}
	before := treeDigest(t, source)
	target := filepath.Join(t.TempDir(), Phase15DataRootDirectoryName)
	manager, _ := NewDataRootManager(target)
	result, err := manager.Import(context.Background(), source)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != DataRootPublished || len(result.Diagnostics) != 0 {
		t.Fatalf("result=%+v", result)
	}
	if _, err := os.Stat(filepath.Join(target, "logs", "d2rbot-old.log")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("runtime log was imported: %v", err)
	}
	if _, err := os.Stat(filepath.Join(target, "diagnostics", "old.zip")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("diagnostic ZIP was imported: %v", err)
	}
	if got := treeDigest(t, source); got != before {
		t.Fatalf("source changed: %s != %s", got, before)
	}
}

func TestDataRootUnknownSchemaBrokenRouteAndCollisionFailClosed(t *testing.T) {
	t.Run("unknown default schema", func(t *testing.T) {
		bundle := buildTestDefaultBundle(t)
		path := filepath.Join(bundle, dataRootBundleManifestName)
		data, _ := os.ReadFile(path)
		var manifest DefaultBundleManifest
		if err := json.Unmarshal(data, &manifest); err != nil {
			t.Fatal(err)
		}
		manifest.SchemaVersion = 99
		data, _ = json.Marshal(manifest)
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
		target := filepath.Join(t.TempDir(), Phase15DataRootDirectoryName)
		manager, _ := NewDataRootManager(target)
		_, err := manager.InitializeDefaults(context.Background(), bundle)
		assertDataRootError(t, err, Phase15ReasonConfigSchemaUnsupported)
		assertTargetAbsent(t, target)
	})

	t.Run("broken farming route", func(t *testing.T) {
		source := initializeTestRoot(t)
		path := filepath.Join(source, "configs", "routes", "farming", "broken", "nightmare", "broken.yaml")
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("version: ["), 0o600); err != nil {
			t.Fatal(err)
		}
		before := treeDigest(t, source)
		target := filepath.Join(t.TempDir(), Phase15DataRootDirectoryName)
		manager, _ := NewDataRootManager(target)
		_, err := manager.Import(context.Background(), source)
		assertDataRootError(t, err, Phase15ReasonDataImportInvalid)
		assertTargetAbsent(t, target)
		if got := treeDigest(t, source); got != before {
			t.Fatalf("source changed: %s != %s", got, before)
		}
		assertNoDataRootStaging(t, target)
	})

	t.Run("target collision", func(t *testing.T) {
		source := initializeTestRoot(t)
		target := filepath.Join(t.TempDir(), Phase15DataRootDirectoryName)
		if err := os.MkdirAll(target, 0o755); err != nil {
			t.Fatal(err)
		}
		sentinel := filepath.Join(target, "sentinel.txt")
		if err := os.WriteFile(sentinel, []byte("unchanged"), 0o600); err != nil {
			t.Fatal(err)
		}
		manager, _ := NewDataRootManager(target)
		_, err := manager.Import(context.Background(), source)
		assertDataRootError(t, err, Phase15ReasonDataImportConflict)
		body, _ := os.ReadFile(sentinel)
		if string(body) != "unchanged" {
			t.Fatalf("collision target changed: %q", body)
		}
	})
}

func TestDataRootCorruptTelemetryIsImportedAsIsolatedDiagnostic(t *testing.T) {
	source := initializeTestRoot(t)
	corrupt := filepath.Join(source, "logs", "telemetry", "corrupt.jsonl")
	if err := os.WriteFile(corrupt, []byte("{not-json}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), Phase15DataRootDirectoryName)
	manager, _ := NewDataRootManager(target)
	result, err := manager.Import(context.Background(), source)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Diagnostics) != 1 || result.Diagnostics[0].File != "corrupt.jsonl" || result.Diagnostics[0].Code != telemetry.HistoryReasonFileInvalid {
		t.Fatalf("diagnostics=%+v", result.Diagnostics)
	}
	if body, err := os.ReadFile(filepath.Join(target, "logs", "telemetry", "corrupt.jsonl")); err != nil || string(body) != "{not-json}\n" {
		t.Fatalf("corrupt telemetry was changed: %q err=%v", body, err)
	}
}

func TestDataRootWriteFailureReparseAndCrashBeforePublishLeaveNoTarget(t *testing.T) {
	source := initializeTestRoot(t)
	tests := []struct {
		name   string
		mutate func(*DataRootManager)
		code   Phase15ReasonCode
	}{
		{name: "write protected publish", code: Phase15ReasonDataImportFailed, mutate: func(manager *DataRootManager) {
			manager.rename = func(string, string) error { return fs.ErrPermission }
		}},
		{name: "reparse source", code: Phase15ReasonDataImportInvalid, mutate: func(manager *DataRootManager) {
			actual := manager.reparse
			manager.reparse = func(path string) (bool, error) {
				if samePath(path, filepath.Join(source, "configs")) {
					return true, nil
				}
				return actual(path)
			}
		}},
		{name: "crash before publish", code: Phase15ReasonDataImportFailed, mutate: func(manager *DataRootManager) {
			manager.beforePublish = func(string) error { return errors.New("injected crash") }
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			target := filepath.Join(t.TempDir(), Phase15DataRootDirectoryName)
			manager, _ := NewDataRootManager(target)
			test.mutate(manager)
			_, err := manager.Import(context.Background(), source)
			assertDataRootError(t, err, test.code)
			assertTargetAbsent(t, target)
			assertNoDataRootStaging(t, target)
		})
	}
}

func TestDataRootFailedPublishedReReadRollsBackTarget(t *testing.T) {
	bundle := buildTestDefaultBundle(t)
	target := filepath.Join(t.TempDir(), Phase15DataRootDirectoryName)
	manager, _ := NewDataRootManager(target)
	validations := 0
	manager.validate = func(root string) ([]telemetry.HistoryFileDiagnostic, error) {
		validations++
		if validations == 2 {
			return nil, errors.New("injected post-publish read error")
		}
		return ValidateInstalledDataRoot(root)
	}
	_, err := manager.InitializeDefaults(context.Background(), bundle)
	assertDataRootError(t, err, Phase15ReasonDataImportFailed)
	assertTargetAbsent(t, target)
}

func TestDataRootRejectsFilesystemRoot(t *testing.T) {
	root := filepath.VolumeName(t.TempDir()) + string(filepath.Separator)
	_, err := NewDataRootManager(root)
	assertDataRootError(t, err, Phase15ReasonDataRootUnavailable)
}

func TestDefaultBundleRejectsHashDriftAndUnknownFields(t *testing.T) {
	bundle := buildTestDefaultBundle(t)
	t.Run("hash drift", func(t *testing.T) {
		copy := filepath.Join(t.TempDir(), "bundle")
		if err := copyRegularTree(bundle, copy, isReparsePoint); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(copy, "configs", "config.yaml"), []byte("changed"), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := readDefaultBundle(copy, isReparsePoint); err == nil {
			t.Fatal("hash drift was accepted")
		}
	})
	t.Run("unknown field", func(t *testing.T) {
		copy := filepath.Join(t.TempDir(), "bundle")
		if err := copyRegularTree(bundle, copy, isReparsePoint); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(copy, dataRootBundleManifestName)
		data, _ := os.ReadFile(path)
		data = []byte(strings.Replace(string(data), "\"schema_version\": 1", "\"schema_version\": 1, \"unknown\": true", 1))
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := readDefaultBundle(copy, isReparsePoint); err == nil {
			t.Fatal("unknown manifest field was accepted")
		}
	})
}

func buildTestDefaultBundle(t *testing.T) string {
	t.Helper()
	bundle := filepath.Join(t.TempDir(), "bundle")
	if err := BuildDefaultBundle(filepath.Join("..", "..", "configs"), bundle); err != nil {
		t.Fatal(err)
	}
	return bundle
}

func initializeTestRoot(t *testing.T) string {
	t.Helper()
	bundle := buildTestDefaultBundle(t)
	root := filepath.Join(t.TempDir(), Phase15DataRootDirectoryName)
	manager, _ := NewDataRootManager(root)
	if _, err := manager.InitializeDefaults(context.Background(), bundle); err != nil {
		t.Fatal(err)
	}
	return root
}

func assertDataRootError(t *testing.T, err error, code Phase15ReasonCode) {
	t.Helper()
	var rootErr *DataRootError
	if !errors.As(err, &rootErr) || rootErr.Code != code {
		t.Fatalf("err=%v code=%v want=%s", err, rootErr, code)
	}
}

func assertTargetAbsent(t *testing.T, target string) {
	t.Helper()
	if _, err := os.Lstat(target); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("target exists after failure: %v", err)
	}
}

func assertNoDataRootStaging(t *testing.T, target string) {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(filepath.Dir(target), "."+filepath.Base(target)+".staging-*"))
	if err != nil || len(matches) != 0 {
		t.Fatalf("staging remains: %v err=%v", matches, err)
	}
}

func treeDigest(t *testing.T, root string) string {
	t.Helper()
	rows := make([]string, 0)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		relative, _ := filepath.Rel(root, path)
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		hash := sha256.Sum256(data)
		rows = append(rows, filepath.ToSlash(relative)+":"+hex.EncodeToString(hash[:]))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(rows)
	hash := sha256.Sum256([]byte(strings.Join(rows, "\n")))
	return hex.EncodeToString(hash[:])
}
