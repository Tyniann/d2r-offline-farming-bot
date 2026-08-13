package api

import (
	"context"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/app"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/telemetry"
)

// StartHistoryMaintenance starts the single hourly wake-up; the service itself enforces at most one daily run.
func (b *LiveBackend) StartHistoryMaintenance(ctx context.Context) {
	if b == nil || b.historyMaintenance == nil {
		return
	}
	go func() {
		b.runAutomaticHistoryMaintenance()
		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				b.runAutomaticHistoryMaintenance()
			}
		}
	}()
}

func (b *LiveBackend) runAutomaticHistoryMaintenance() {
	b.mu.RLock()
	store := b.operatorSettings
	idle := historyMaintenanceIdle(b.status.State, b.routeWorkflow.State)
	b.mu.RUnlock()
	if store == nil {
		return
	}
	settings, err := store.Snapshot()
	if err != nil || !settings.History.RetentionEnabled {
		return
	}
	result, err := b.historyMaintenance.Automatic(settings.History.RetentionDays, idle, nil)
	if err != nil {
		b.publisher.Publish(telemetry.LiveEvent{Event: "history_maintenance", Reason: string(telemetry.HistoryErrorCode(err))})
		return
	}
	if result.Ran {
		_, _ = b.refreshHistory("")
		b.publisher.Publish(telemetry.LiveEvent{Event: "history_maintenance", Details: map[string]any{"deleted_files": result.DeletedFiles, "diagnostics": len(result.Diagnostics)}})
	}
}

// PreviewHistoryDeleteAll creates the first delete-all confirmation only while every workflow is idle.
func (b *LiveBackend) PreviewHistoryDeleteAll(request HistoryDeletePreviewRequest) (HistoryDeletePreviewDTO, error) {
	active, err := b.historyMaintenanceContext(request.ExpectedGeneration)
	if err != nil {
		return HistoryDeletePreviewDTO{}, err
	}
	preview, err := b.historyMaintenance.PreviewDeleteAll(active)
	if err != nil {
		return HistoryDeletePreviewDTO{}, err
	}
	return HistoryDeletePreviewDTO{
		ConfirmationToken: preview.Token, IndexGeneration: preview.Generation,
		CandidateFiles: preview.CandidateFiles, CandidateBytes: preview.CandidateBytes,
		ProtectedFiles: preview.ProtectedFiles, Categories: preview.Categories,
	}, nil
}

// ConfirmHistoryDeleteAll rechecks supervisor generation, active writers and preview metadata before removal.
func (b *LiveBackend) ConfirmHistoryDeleteAll(request HistoryDeleteConfirmRequest) (HistoryDeleteResultDTO, error) {
	active, err := b.historyMaintenanceContext(request.ExpectedGeneration)
	if err != nil {
		return HistoryDeleteResultDTO{}, err
	}
	result, err := b.historyMaintenance.ConfirmDeleteAll(telemetry.HistoryDeleteConfirmation{
		Token: request.ConfirmationToken, Generation: request.IndexGeneration,
		CandidateFiles: request.CandidateFiles, CandidateBytes: request.CandidateBytes,
	}, active)
	dto := HistoryDeleteResultDTO{
		DeletedFiles: result.DeletedFiles, DeletedBytes: result.DeletedBytes,
		ProtectedFiles: result.ProtectedFiles, Diagnostics: historyMaintenanceDiagnosticsDTO(result.Diagnostics),
	}
	if result.DeletedFiles > 0 || len(result.Diagnostics) > 0 {
		_, _ = b.refreshHistory("")
	}
	return dto, err
}

func (b *LiveBackend) historyMaintenanceContext(expectedGeneration uint64) ([]string, error) {
	b.mu.RLock()
	generation := b.status.Generation
	state := b.status.State
	workflow := b.routeWorkflow.State
	activeRunID := b.status.RunInstanceID
	b.mu.RUnlock()
	if expectedGeneration != generation || !historyMaintenanceIdle(state, workflow) {
		return nil, &commandError{code: string(telemetry.HistoryReasonRetentionBlocked), message: "Die Historie kann nur ohne laufende Session oder laufenden Routenvorgang gelöscht werden."}
	}
	active := make([]string, 0, 2)
	if activeRunID != "" {
		active = append(active, activeRunID+".jsonl")
	}
	snapshot, err := b.refreshHistory(activeRunID)
	if err != nil {
		return nil, err
	}
	for _, run := range snapshot.Runs {
		if run.RunID == activeRunID {
			active = append(active, run.RunFile, run.SessionFile)
		}
	}
	return active, nil
}

func historyMaintenanceIdle(state, workflow string) bool {
	idle := state == string(app.SupervisorStateIdle) || state == string(app.SupervisorStateIdleInGame) || state == string(app.SupervisorStateStoppedError)
	// Terminale Routenworkflow-Zustände besitzen keinen aktiven Writer und werden
	// im gesamten Backend bereits als nicht beschäftigt behandelt. Die Wartung
	// muss denselben Vertrag verwenden, da die letzte Erfolgsmeldung sonst jede
	// spätere Historienlöschung bis zum nächsten Core-Neustart blockiert.
	return idle && !routeWorkflowBusy(workflow)
}

func historyMaintenanceDiagnosticsDTO(values []telemetry.HistoryMaintenanceDiagnostic) []HistoryMaintenanceDiagnosticDTO {
	out := make([]HistoryMaintenanceDiagnosticDTO, len(values))
	for index, value := range values {
		out[index] = HistoryMaintenanceDiagnosticDTO{FileID: value.FileID, Code: string(value.Code), Message: value.Message}
	}
	return out
}
