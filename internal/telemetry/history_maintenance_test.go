package telemetry

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestHistoryAutomaticRetentionUsesStrictCompleteBundleBoundary(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 7, 26, 12, 0, 0, 0, time.UTC)
	writeMaintenanceBundle(t, root, "session-59", []string{"run-59"}, now.AddDate(0, 0, -59), nil)
	writeMaintenanceBundle(t, root, "session-60", []string{"run-60"}, now.AddDate(0, 0, -60), nil)
	writeMaintenanceBundle(t, root, "session-61", []string{"run-61"}, now.AddDate(0, 0, -61), nil)
	index, err := NewHistoryIndex(root)
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewHistoryMaintenanceService(root, index)
	if err != nil {
		t.Fatal(err)
	}
	service.now = func() time.Time { return now }
	result, err := service.Automatic(60, true, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Ran || result.DeletedFiles != 2 {
		t.Fatalf("result=%+v", result)
	}
	assertMaintenanceExists(t, root, "session-59.jsonl", true)
	assertMaintenanceExists(t, root, "session-60.jsonl", true)
	assertMaintenanceExists(t, root, "session-61.jsonl", false)
	assertMaintenanceExists(t, root, "run-61.jsonl", false)
	second, err := service.Automatic(60, true, nil)
	if err != nil || second.Ran {
		t.Fatalf("second=%+v err=%v", second, err)
	}
}

func TestHistoryAutomaticRetentionSkipsIncompleteAmbiguousActiveAndMixedBundles(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 7, 26, 12, 0, 0, 0, time.UTC)
	old := now.AddDate(0, 0, -61)
	writeMaintenanceBundle(t, root, "session-active", []string{"run-active"}, old, nil)
	writeMaintenanceBundle(t, root, "session-missing", []string{"run-missing"}, old, nil)
	if err := os.Remove(filepath.Join(root, "run-missing.jsonl")); err != nil {
		t.Fatal(err)
	}
	recent := now.AddDate(0, 0, -1)
	writeMaintenanceBundle(t, root, "session-mixed", []string{"run-old", "run-recent"}, old, map[string]time.Time{"run-recent": recent})
	writeMaintenanceBundle(t, root, "session-extra", []string{"run-main"}, old, nil)
	writeMaintenanceRun(t, root, "session-extra", "run-extra", old)
	if err := os.WriteFile(filepath.Join(root, "legacy.jsonl"), []byte("{\"schema_version\":2}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "corrupt.jsonl"), []byte("{broken}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	service, err := NewHistoryMaintenanceService(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	service.now = func() time.Time { return now }
	result, err := service.Automatic(60, true, []string{"session-active.jsonl", "run-active.jsonl"})
	if err != nil {
		t.Fatal(err)
	}
	if result.DeletedFiles != 0 || len(result.Diagnostics) < 4 {
		t.Fatalf("result=%+v", result)
	}
	for _, name := range []string{"session-active.jsonl", "run-active.jsonl", "session-missing.jsonl", "session-mixed.jsonl", "run-old.jsonl", "run-recent.jsonl", "session-extra.jsonl", "run-main.jsonl", "run-extra.jsonl", "legacy.jsonl", "corrupt.jsonl"} {
		assertMaintenanceExists(t, root, name, true)
	}
	blocked, err := service.Automatic(60, false, nil)
	if err != nil || blocked.Ran || len(blocked.Diagnostics) != 1 || blocked.Diagnostics[0].Code != HistoryReasonRetentionBlocked {
		t.Fatalf("blocked=%+v err=%v", blocked, err)
	}
}

func TestHistoryDeleteAllBindsPreviewMetadataAndProtectsNewActiveSet(t *testing.T) {
	root := t.TempDir()
	writeMaintenanceBundle(t, root, "session-delete", []string{"run-delete"}, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), nil)
	if err := os.WriteFile(filepath.Join(root, "legacy.jsonl"), []byte("{\"schema_version\":2}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "corrupt.jsonl"), []byte("{broken}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "nested", "hidden.jsonl"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	index, _ := NewHistoryIndex(root)
	if err := index.Refresh(); err != nil {
		t.Fatal(err)
	}
	service, err := NewHistoryMaintenanceService(root, index)
	if err != nil {
		t.Fatal(err)
	}
	preview, err := service.PreviewDeleteAll(nil)
	if err != nil {
		t.Fatal(err)
	}
	if preview.Token == "" || preview.CandidateFiles != 4 || preview.Categories["schema3_session"] != 1 || preview.Categories["schema3_run"] != 1 || preview.Categories["legacy"] != 1 || preview.Categories["corrupt"] != 1 {
		t.Fatalf("preview=%+v", preview)
	}
	result, err := service.ConfirmDeleteAll(HistoryDeleteConfirmation{
		Token: preview.Token, Generation: preview.Generation, CandidateFiles: preview.CandidateFiles, CandidateBytes: preview.CandidateBytes,
	}, []string{"run-delete.jsonl"})
	if err != nil {
		t.Fatal(err)
	}
	if result.DeletedFiles != 3 || result.ProtectedFiles != 1 {
		t.Fatalf("result=%+v", result)
	}
	assertMaintenanceExists(t, root, "run-delete.jsonl", true)
	assertMaintenanceExists(t, root, filepath.Join("nested", "hidden.jsonl"), true)
}

func TestHistoryDeleteAllRejectsStalePreviewAndReportsPartialFailure(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.jsonl"), []byte("{\"schema_version\":2}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "b.jsonl"), []byte("{\"schema_version\":2}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	service, err := NewHistoryMaintenanceService(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	preview, _ := service.PreviewDeleteAll(nil)
	if writeErr := os.WriteFile(filepath.Join(root, "a.jsonl"), []byte("{\"schema_version\":2}\n{\"changed\":true}\n"), 0o644); writeErr != nil {
		t.Fatal(writeErr)
	}
	if _, confirmErr := service.ConfirmDeleteAll(HistoryDeleteConfirmation{Token: preview.Token, Generation: preview.Generation, CandidateFiles: preview.CandidateFiles, CandidateBytes: preview.CandidateBytes}, nil); HistoryErrorCode(confirmErr) != HistoryReasonDeletePreviewStale {
		t.Fatalf("stale err=%v", confirmErr)
	}
	preview, _ = service.PreviewDeleteAll(nil)
	service.remove = func(path string) error {
		if filepath.Base(path) == "b.jsonl" {
			return errors.New("locked")
		}
		return os.Remove(path)
	}
	result, err := service.ConfirmDeleteAll(HistoryDeleteConfirmation{Token: preview.Token, Generation: preview.Generation, CandidateFiles: preview.CandidateFiles, CandidateBytes: preview.CandidateBytes}, nil)
	if HistoryErrorCode(err) != HistoryReasonDeleteFailed || result.DeletedFiles != 1 || len(result.Diagnostics) != 1 || result.Diagnostics[0].FileID == "" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	assertMaintenanceExists(t, root, "b.jsonl", true)
}

func TestHistoryMaintenanceExcludesSubdirectoriesAndReparseFiles(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "target.txt")
	if err := os.WriteFile(target, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "linked.jsonl")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	service, err := NewHistoryMaintenanceService(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	preview, err := service.PreviewDeleteAll(nil)
	if err != nil {
		t.Fatal(err)
	}
	if preview.CandidateFiles != 0 {
		t.Fatalf("preview=%+v", preview)
	}
	assertMaintenanceExists(t, root, "linked.jsonl", true)
}

func writeMaintenanceBundle(t *testing.T, root, sessionID string, runIDs []string, started time.Time, overrides map[string]time.Time) {
	t.Helper()
	session, err := NewSessionRecorderWithContext(root, SessionRecorderContext{SessionID: sessionID, Mode: HistoryModeProductiveFarming, Character: "MrBones", Difficulty: "nightmare", GameVersion: "3.2.92777"})
	if err != nil {
		t.Fatal(err)
	}
	if err := session.Emit(Event{Timestamp: started.Add(-time.Minute), Event: SessionStarted}); err != nil {
		t.Fatal(err)
	}
	latest := started
	for index, runID := range runIDs {
		runStart := started.Add(time.Duration(index) * time.Minute)
		if override, ok := overrides[runID]; ok {
			runStart = override
		}
		gameID := "game-" + runID
		if err := session.Emit(Event{Timestamp: runStart, Event: RunStarted, RunID: runID, GameID: gameID, Run: "countess"}); err != nil {
			t.Fatal(err)
		}
		writeMaintenanceRun(t, root, sessionID, runID, runStart)
		terminal := runStart.Add(time.Second)
		if err := session.Emit(Event{Timestamp: terminal, Event: RunCompleted, RunID: runID, GameID: gameID, Run: "countess"}); err != nil {
			t.Fatal(err)
		}
		if terminal.After(latest) {
			latest = terminal
		}
	}
	if err := session.Emit(Event{Timestamp: latest.Add(time.Second), Event: SessionCompleted}); err != nil {
		t.Fatal(err)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
}

func writeMaintenanceRun(t *testing.T, root, sessionID, runID string, started time.Time) {
	t.Helper()
	recorder, err := NewRunRecorder(root, RunRecorderContext{
		RunID: runID, SessionID: sessionID, GameID: "game-" + runID, Mode: HistoryModeProductiveFarming,
		Character: "MrBones", Difficulty: "nightmare", GameVersion: "3.2.92777", Run: "countess",
		DefinitionID: "countess", RouteID: "route", RouteLayoutFingerprint: "layout", StartedAt: started,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := recorder.Emit(Event{Timestamp: started, Event: RunContext}); err != nil {
		t.Fatal(err)
	}
	if err := recorder.Emit(Event{Timestamp: started.Add(time.Second), Event: RunCompleted}); err != nil {
		t.Fatal(err)
	}
	if err := recorder.Close(); err != nil {
		t.Fatal(err)
	}
}

func assertMaintenanceExists(t *testing.T, root, name string, want bool) {
	t.Helper()
	_, err := os.Lstat(filepath.Join(root, name))
	if (err == nil) != want {
		t.Fatalf("%s exists=%t want=%t err=%v", name, err == nil, want, err)
	}
}
