package api

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/app"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/telemetry"
)

func TestHistoryDeleteAllAPIRequiresTokenAndProjectsPreviewAndResult(t *testing.T) {
	server, backend := startAPITestServer(t)
	backend.historyDeletePreview = HistoryDeletePreviewDTO{
		ConfirmationToken: "one-use", IndexGeneration: 7, CandidateFiles: 4,
		CandidateBytes: 1234, ProtectedFiles: 1, Categories: map[string]int{"legacy": 2, "schema3_run": 2},
	}
	backend.historyDeleteResult = HistoryDeleteResultDTO{
		DeletedFiles: 4, DeletedBytes: 1234, ProtectedFiles: 1,
		Diagnostics: []HistoryMaintenanceDiagnosticDTO{{FileID: "history-a1b2c3d4", Code: "history_delete_active_protected", Message: "geschützt"}},
	}
	unguarded, _ := http.NewRequest(http.MethodPost, server.URL()+"/api/v1/history/delete-all/preview", strings.NewReader(`{"expected_generation":3}`))
	unguarded.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(unguarded)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unguarded status=%d", response.StatusCode)
	}

	previewRequest := newCommandRequest(t, server, "/api/v1/history/delete-all/preview", `{"expected_generation":3}`)
	response, err = http.DefaultClient.Do(previewRequest)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var preview HistoryDeletePreviewDTO
	if decodeErr := json.NewDecoder(response.Body).Decode(&preview); decodeErr != nil {
		t.Fatal(decodeErr)
	}
	if response.StatusCode != http.StatusOK || preview.ConfirmationToken != "one-use" || preview.CandidateFiles != 4 || preview.Categories["legacy"] != 2 {
		t.Fatalf("status=%d preview=%+v", response.StatusCode, preview)
	}

	confirmRequest := newCommandRequest(t, server, "/api/v1/history/delete-all/confirm", `{"expected_generation":3,"confirmation_token":"one-use","index_generation":7,"candidate_files":4,"candidate_bytes":1234}`)
	response, err = http.DefaultClient.Do(confirmRequest)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var result HistoryDeleteResultDTO
	if decodeErr := json.NewDecoder(response.Body).Decode(&result); decodeErr != nil {
		t.Fatal(decodeErr)
	}
	if response.StatusCode != http.StatusOK || result.DeletedFiles != 4 || len(result.Diagnostics) != 1 || result.Diagnostics[0].FileID == "" {
		t.Fatalf("status=%d result=%+v", response.StatusCode, result)
	}
}

func TestHistoryDeleteAllAllowsTerminalRouteWorkflowAndRepeatedEmptyPreview(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "legacy.jsonl"), []byte("{\"schema_version\":2}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	index, err := telemetry.NewHistoryIndex(root)
	if err != nil {
		t.Fatal(err)
	}
	if refreshErr := index.Refresh(); refreshErr != nil {
		t.Fatal(refreshErr)
	}
	maintenance, err := telemetry.NewHistoryMaintenanceService(root, index)
	if err != nil {
		t.Fatal(err)
	}
	backend := &LiveBackend{
		status:             StatusDTO{State: string(app.SupervisorStateIdle), Generation: 4},
		routeWorkflow:      RouteWorkflowDTO{State: string(app.RouteWorkflowCompleted)},
		history:            index,
		historyMaintenance: maintenance,
		publisher:          telemetry.NewLivePublisher(8, 2),
	}

	preview, err := backend.PreviewHistoryDeleteAll(HistoryDeletePreviewRequest{ExpectedGeneration: 4})
	if err != nil || preview.CandidateFiles != 1 {
		t.Fatalf("preview=%+v err=%v", preview, err)
	}
	result, err := backend.ConfirmHistoryDeleteAll(HistoryDeleteConfirmRequest{
		ExpectedGeneration: 4, ConfirmationToken: preview.ConfirmationToken,
		IndexGeneration: preview.IndexGeneration, CandidateFiles: preview.CandidateFiles, CandidateBytes: preview.CandidateBytes,
	})
	if err != nil || result.DeletedFiles != 1 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if _, statErr := os.Stat(filepath.Join(root, "legacy.jsonl")); !os.IsNotExist(statErr) {
		t.Fatalf("deleted history still exists: %v", statErr)
	}

	empty, err := backend.PreviewHistoryDeleteAll(HistoryDeletePreviewRequest{ExpectedGeneration: 4})
	if err != nil || empty.CandidateFiles != 0 || empty.CandidateBytes != 0 || empty.ProtectedFiles != 0 {
		t.Fatalf("empty preview=%+v err=%v", empty, err)
	}
	backend.mu.Lock()
	backend.routeWorkflow.State = string(app.RouteWorkflowRecording)
	backend.mu.Unlock()
	if _, err := backend.PreviewHistoryDeleteAll(HistoryDeletePreviewRequest{ExpectedGeneration: 4}); err == nil {
		t.Fatal("active route workflow allowed history maintenance")
	}
}
