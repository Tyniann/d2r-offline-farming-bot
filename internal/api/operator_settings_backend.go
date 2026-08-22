package api

import (
	"errors"
	"fmt"
	"strings"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/app"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/telemetry"
)

// SetOperatorSettingsStore bindet die einzige Core-eigene GUI-Konfigurationsautorität.
func (b *LiveBackend) SetOperatorSettingsStore(store *app.OperatorSettingsStore) error {
	if store == nil {
		return fmt.Errorf("operator settings store is unavailable")
	}
	b.mu.RLock()
	selection := b.status.Selection
	b.mu.RUnlock()
	catalog, err := b.characterCatalog.BindOperatorSettings(store)
	if err != nil {
		return fmt.Errorf("bind character catalog settings: %w", err)
	}
	selectionConfigured := false
	var configuredEntry app.CharacterCatalogEntry
	for _, entry := range catalog.Characters {
		if strings.EqualFold(entry.Name, selection.Character) && entry.Selectable && entry.CombatProfile != "" {
			selectionConfigured = true
			configuredEntry = entry
			break
		}
	}
	if selection.Character != "" && selection.Difficulty != "" && selectionConfigured {
		if _, confirmErr := store.ConfirmSelection(selection.Character, selection.Difficulty); confirmErr != nil {
			return fmt.Errorf("persist confirmed selection: %w", confirmErr)
		}
	} else if selection.Character != "" {
		b.mu.Lock()
		b.status.Selection = SelectionStatusDTO{}
		b.mu.Unlock()
	}
	service, err := app.NewCharacterSetupService(app.CharacterSetupDependencies{
		Config: b.cfg, Catalog: b.characterCatalog, Settings: store,
		PickitAssignments: b.pickitAssignments, PickitProfiles: b.pickitProfiles, Capture: b.characterCapture,
	})
	if err != nil {
		return fmt.Errorf("character setup service: %w", err)
	}
	b.mu.Lock()
	b.operatorSettings = store
	b.characterSetup = service
	b.mu.Unlock()
	b.publishCharacterCatalog(catalog, false)
	if selectionConfigured {
		runs, resolveErr := b.resolveRunsForEntry(configuredEntry, selection.Difficulty)
		if resolveErr != nil {
			return fmt.Errorf("resolve configured character runs: %w", resolveErr)
		}
		b.mu.Lock()
		b.catalog.Runs = runs
		b.mu.Unlock()
	}
	return nil
}

// OperatorSettings liefert den aktuellen persistenten Core-Stand.
func (b *LiveBackend) OperatorSettings() (OperatorSettingsDTO, error) {
	b.mu.RLock()
	store := b.operatorSettings
	b.mu.RUnlock()
	if store == nil {
		return OperatorSettingsDTO{}, fmt.Errorf("operator settings store is unavailable")
	}
	settings, err := store.Snapshot()
	if err != nil {
		return OperatorSettingsDTO{}, err
	}
	return operatorSettingsDTO(settings), nil
}

// PreviewOperatorSettings validiert die komplette Ersetzung ohne Mutation.
func (b *LiveBackend) PreviewOperatorSettings(request OperatorSettingsMutationRequest) (OperatorSettingsChangeDTO, error) {
	b.mu.RLock()
	store, generation := b.operatorSettings, b.status.Generation
	b.mu.RUnlock()
	if store == nil {
		return OperatorSettingsChangeDTO{}, fmt.Errorf("operator settings store is unavailable")
	}
	if request.ExpectedGeneration != generation {
		return OperatorSettingsChangeDTO{}, &commandError{code: "state_changed"}
	}
	change, err := store.Preview(request.ExpectedRevision, operatorSettingsFromDTO(request.Settings))
	if err != nil {
		return OperatorSettingsChangeDTO{}, operatorSettingsCommandError(err)
	}
	return operatorSettingsChangeDTO(change, generation), nil
}

// PreviewResetOperatorSettings projiziert die sicheren Defaults ohne Mutation.
func (b *LiveBackend) PreviewResetOperatorSettings(request OperatorSettingsResetRequest) (OperatorSettingsChangeDTO, error) {
	b.mu.RLock()
	store, generation := b.operatorSettings, b.status.Generation
	b.mu.RUnlock()
	if store == nil {
		return OperatorSettingsChangeDTO{}, fmt.Errorf("operator settings store is unavailable")
	}
	if request.ExpectedGeneration != generation {
		return OperatorSettingsChangeDTO{}, &commandError{code: "state_changed"}
	}
	change, err := store.PreviewReset(request.ExpectedRevision)
	if err != nil {
		return OperatorSettingsChangeDTO{}, operatorSettingsCommandError(err)
	}
	return operatorSettingsChangeDTO(change, generation), nil
}

// UpdateOperatorSettings persistiert nur bei passender Revision, Generation und inaktivem Core.
func (b *LiveBackend) UpdateOperatorSettings(request OperatorSettingsMutationRequest) (OperatorSettingsChangeDTO, error) {
	b.commandMu.Lock()
	defer b.commandMu.Unlock()
	store, generation, err := b.operatorSettingsMutationContext(request.ExpectedGeneration)
	if err != nil {
		return OperatorSettingsChangeDTO{}, err
	}
	change, err := store.Update(request.ExpectedRevision, operatorSettingsFromDTO(request.Settings))
	if err != nil {
		return OperatorSettingsChangeDTO{}, operatorSettingsCommandError(err)
	}
	b.applyOperatorSettingsChange(change)
	return operatorSettingsChangeDTO(change, generation), nil
}

// ResetOperatorSettings stellt sichere Defaults nur am selben Mutationsgate wieder her.
func (b *LiveBackend) ResetOperatorSettings(request OperatorSettingsResetRequest) (OperatorSettingsChangeDTO, error) {
	b.commandMu.Lock()
	defer b.commandMu.Unlock()
	store, generation, err := b.operatorSettingsMutationContext(request.ExpectedGeneration)
	if err != nil {
		return OperatorSettingsChangeDTO{}, err
	}
	change, err := store.Reset(request.ExpectedRevision)
	if err != nil {
		return OperatorSettingsChangeDTO{}, operatorSettingsCommandError(err)
	}
	b.applyOperatorSettingsChange(change)
	return operatorSettingsChangeDTO(change, generation), nil
}

func (b *LiveBackend) operatorSettingsMutationContext(expectedGeneration uint64) (*app.OperatorSettingsStore, uint64, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.operatorSettings == nil {
		return nil, 0, fmt.Errorf("operator settings store is unavailable")
	}
	if expectedGeneration != b.status.Generation {
		return nil, 0, &commandError{code: "state_changed"}
	}
	if b.status.State != string(app.SupervisorStateIdle) && b.status.State != string(app.SupervisorStateStoppedError) {
		return nil, 0, &commandError{code: "command_conflict", params: map[string]any{"operation": "operator_settings"}}
	}
	if routeWorkflowBusy(b.routeWorkflow.State) {
		return nil, 0, &commandError{code: "command_conflict", params: map[string]any{"operation": "operator_settings"}}
	}
	return b.operatorSettings, b.status.Generation, nil
}

func (b *LiveBackend) applyOperatorSettingsChange(change app.OperatorSettingsChange) {
	if len(change.ChangedFields) == 0 {
		return
	}
	b.mu.Lock()
	// Input und Hotkeys bleiben bis zum kontrollierten Core-Neustart an den
	// bereits aufgebauten Controllern unverändert. Alle übrigen Werte dürfen
	// in idle für die nächste Queuegeneration übernommen werden.
	b.cfg.Session.MaxRuns = change.Settings.Budgets.MaxRuns
	b.cfg.Session.MaxDurationMs = change.Settings.Budgets.MaxDurationMs
	b.cfg.Session.MaxConsecutiveFailures = change.Settings.Budgets.MaxConsecutiveFailures
	b.cfg.Session.MaxTotalRestarts = change.Settings.Budgets.MaxTotalRestarts
	b.status.Queue.Budgets = QueueBudgetsDTO{
		MaxRuns: change.Settings.Budgets.MaxRuns, MaxDurationMs: int64(change.Settings.Budgets.MaxDurationMs),
		MaxConsecutiveFailures: change.Settings.Budgets.MaxConsecutiveFailures, MaxTotalRestarts: change.Settings.Budgets.MaxTotalRestarts,
	}
	character := strings.ToLower(strings.TrimSpace(b.status.Selection.Character))
	if value, ok := change.Settings.Characters[character]; ok {
		b.cfg.Session.Queue = append([]string(nil), value.Queue...)
		if len(value.Queue) > 0 {
			b.cfg.Session.Run = value.Queue[0]
		}
		b.status.Queue.Entries = append([]string(nil), value.Queue...)
		b.status.Queue.DefaultEntries = append([]string(nil), value.Queue...)
	}
	b.mu.Unlock()
	// Ohne ein Event behält der Renderer nach dem Speichern seine alte
	// Statusprojektion und würde diese veraltete Queue wieder an den Core senden.
	b.publisher.Publish(telemetry.LiveEvent{
		Event:   "operator_settings_changed",
		Details: map[string]any{"revision": change.Settings.Revision},
	})
}

func operatorSettingsCommandError(err error) error {
	var validationErr *app.OperatorSettingsValidationError
	if errors.As(err, &validationErr) {
		return &commandError{code: "config_invalid", cause: fmt.Errorf("validate operator settings: %w", validationErr)}
	}
	var settingsErr *app.OperatorSettingsError
	if errors.As(err, &settingsErr) {
		switch settingsErr.Code {
		case app.Phase15ReasonConfigRevisionConflict:
			return &commandError{code: string(settingsErr.Code)}
		case app.Phase15ReasonConfigSchemaUnsupported:
			return &commandError{code: string(settingsErr.Code)}
		}
	}
	return &commandError{code: "config_invalid", cause: fmt.Errorf("apply operator settings: %w", err)}
}
