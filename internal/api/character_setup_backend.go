package api

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/app"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/tasks"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/telemetry"
)

type characterSetupCommandRecord struct {
	name     string
	payload  string
	response CharacterSetupPreviewDTO
}

// SetCharacterCaptureHandler bindet den exklusiven vorhandenen Runtime-Capturepfad.
func (b *LiveBackend) SetCharacterCaptureHandler(handler app.CharacterSetupCaptureFunc) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.characterCapture = handler
	if b.characterSetup != nil {
		b.characterSetup.SetCapture(handler)
	}
}

// ReloadCharacters liest den begrenzten Katalog synchron neu.
func (b *LiveBackend) ReloadCharacters() (CharacterReloadDTO, error) {
	previousRevision := b.characterCatalog.Snapshot().Revision
	catalog, err := b.characterCatalog.Reload()
	if err != nil {
		return CharacterReloadDTO{}, err
	}
	b.publishCharacterCatalog(catalog, catalog.Revision != previousRevision)
	return CharacterReloadDTO{SchemaVersion: schemaVersion, Catalog: b.Catalog()}, nil
}

// PreviewCharacterSetup liefert ausschließlich frisch gelesenen Corezustand.
func (b *LiveBackend) PreviewCharacterSetup(request CharacterSetupPreviewRequest) (CharacterSetupPreviewDTO, error) {
	b.mu.RLock()
	service := b.characterSetup
	b.mu.RUnlock()
	if service == nil {
		return CharacterSetupPreviewDTO{}, fmt.Errorf("character setup service is unavailable")
	}
	value, err := service.Preview(request.Character)
	if err != nil {
		return CharacterSetupPreviewDTO{}, characterSetupCommandError(err)
	}
	return characterSetupPreviewDTO(value), nil
}

// ConfirmCharacterSetup schreibt unter dem serialisierten Idle-Gate zuerst OperatorSettings und danach Pickit.
func (b *LiveBackend) ConfirmCharacterSetup(request CharacterSetupConfirmRequest) (CharacterSetupPreviewDTO, error) {
	b.commandMu.Lock()
	defer b.commandMu.Unlock()
	payload := fmt.Sprintf("%s\x00%s\x00%d\x00%d\x00%d\x00%d", request.Character, request.ProfileID, request.ExpectedCatalogRevision, request.ExpectedOperatorSettingsRevision, request.ExpectedPickitAssignmentRevision, request.ExpectedGeneration)
	if response, ok, err := b.replayCharacterCommand(request.CommandID, "confirm", payload); ok || err != nil {
		return response, err
	}
	service, err := b.characterSetupMutationContext(request.CommandID, request.ExpectedGeneration)
	if err != nil {
		return CharacterSetupPreviewDTO{}, err
	}
	value, err := service.Confirm(app.CharacterSetupConfirmRequest{
		Character: request.Character, ProfileID: request.ProfileID,
		ExpectedCatalogRevision: request.ExpectedCatalogRevision, ExpectedSettingsRevision: request.ExpectedOperatorSettingsRevision,
		ExpectedPickitRevision: request.ExpectedPickitAssignmentRevision,
	})
	if err != nil {
		return CharacterSetupPreviewDTO{}, characterSetupCommandError(err)
	}
	response := characterSetupPreviewDTO(value)
	changed := response.CatalogRevision != request.ExpectedCatalogRevision ||
		response.OperatorSettingsRevision != request.ExpectedOperatorSettingsRevision ||
		response.PickitAssignmentRevision != request.ExpectedPickitAssignmentRevision
	b.publishCharacterCatalog(b.characterCatalog.Snapshot(), changed)
	b.characterCommands[request.CommandID] = characterSetupCommandRecord{name: "confirm", payload: payload, response: response}
	return response, nil
}

// CaptureCharacterSelection erfasst unter demselben Idle-Gate ohne Navigationsinput.
func (b *LiveBackend) CaptureCharacterSelection(ctx context.Context, request CharacterSelectionCaptureRequest) (CharacterSetupPreviewDTO, error) {
	b.commandMu.Lock()
	defer b.commandMu.Unlock()
	payload := fmt.Sprintf("%s\x00%d\x00%d", request.Character, request.ExpectedCatalogRevision, request.ExpectedGeneration)
	if response, ok, err := b.replayCharacterCommand(request.CommandID, "capture", payload); ok || err != nil {
		return response, err
	}
	service, err := b.characterSetupMutationContext(request.CommandID, request.ExpectedGeneration)
	if err != nil {
		return CharacterSetupPreviewDTO{}, err
	}
	value, err := service.Capture(ctx, app.CharacterSetupCaptureRequest{
		Character: request.Character, ExpectedCatalogRevision: request.ExpectedCatalogRevision,
	})
	if err != nil {
		return CharacterSetupPreviewDTO{}, characterSetupCommandError(err)
	}
	response := characterSetupPreviewDTO(value)
	b.publishCharacterCatalog(b.characterCatalog.Snapshot(), response.CatalogRevision != request.ExpectedCatalogRevision)
	b.characterCommands[request.CommandID] = characterSetupCommandRecord{name: "capture", payload: payload, response: response}
	return response, nil
}

func (b *LiveBackend) replayCharacterCommand(commandID, name, payload string) (CharacterSetupPreviewDTO, bool, error) {
	if strings.TrimSpace(commandID) == "" {
		return CharacterSetupPreviewDTO{}, false, nil
	}
	record, ok := b.characterCommands[commandID]
	if !ok {
		return CharacterSetupPreviewDTO{}, false, nil
	}
	if record.name != name || record.payload != payload {
		return CharacterSetupPreviewDTO{}, false, &commandError{code: "command_id_conflict", message: "Die Command-ID wurde bereits anders verwendet."}
	}
	return record.response, true, nil
}

func (b *LiveBackend) characterSetupMutationContext(commandID string, expectedGeneration uint64) (*app.CharacterSetupService, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if strings.TrimSpace(commandID) == "" {
		return nil, &commandError{code: "command_invalid", message: "Eine Command-ID ist erforderlich."}
	}
	if b.characterSetup == nil {
		return nil, fmt.Errorf("character setup service is unavailable")
	}
	if expectedGeneration != b.status.Generation {
		return nil, &commandError{code: "state_changed", message: "Der Core-Zustand hat sich geändert."}
	}
	if b.status.State != string(app.SupervisorStateIdle) && b.status.State != string(app.SupervisorStateStoppedError) {
		return nil, &commandError{code: "command_conflict", message: "Charakter-Setup ist nur bei inaktiver Session möglich."}
	}
	if routeWorkflowBusy(b.routeWorkflow.State) {
		return nil, &commandError{code: "command_conflict", message: "Charakter-Setup ist während eines Routen-Workflows gesperrt."}
	}
	return b.characterSetup, nil
}

func (b *LiveBackend) publishCharacterCatalog(catalog app.CharacterCatalog, publishEvent bool) {
	b.mu.Lock()
	characters := make([]CharacterCatalogEntry, 0, len(catalog.Characters))
	index := make(map[string]app.CharacterCatalogEntry, len(catalog.Characters))
	for _, entry := range catalog.Characters {
		characters = append(characters, CharacterCatalogEntry{
			Name: entry.Name, Slug: entry.Slug, ExpectedClass: entry.ExpectedClass,
			Selectable: entry.Selectable, Reasons: append([]string(nil), entry.Reasons...),
		})
		index[entry.Slug] = entry
	}
	b.catalog.Revision = catalog.Revision
	b.catalog.Characters = characters
	b.bootstrap.characters = index
	b.mu.Unlock()
	if publishEvent {
		b.publisher.Publish(telemetry.LiveEvent{Event: "catalog_changed", Details: map[string]any{"revision": catalog.Revision}})
	}
}

func (b *LiveBackend) reloadCharacterCatalog() (app.CharacterCatalog, error) {
	b.mu.RLock()
	previousRevision := b.catalog.Revision
	reload := b.characterCatalogReload
	b.mu.RUnlock()
	if reload == nil {
		return app.CharacterCatalog{}, &commandError{code: "character_catalog_unavailable", message: "Die lokalen Spielstände konnten nicht sicher neu gelesen werden."}
	}
	catalog, err := reload()
	if err != nil {
		return app.CharacterCatalog{}, &commandError{code: "character_catalog_unavailable", message: "Die lokalen Spielstände konnten nicht sicher neu gelesen werden."}
	}
	b.publishCharacterCatalog(catalog, catalog.Revision != previousRevision)
	return catalog, nil
}

func (b *LiveBackend) validateDesktopCharacterContract(character string, runIDs []string) (app.CharacterCatalogEntry, uint64, error) {
	b.mu.RLock()
	settingsStore := b.operatorSettings
	assignments := b.pickitAssignments
	b.mu.RUnlock()
	if settingsStore == nil {
		return app.CharacterCatalogEntry{}, 0, &commandError{code: "character_setup_unavailable", message: "Die Charaktereinrichtung ist noch nicht verfügbar."}
	}
	catalog, err := b.reloadCharacterCatalog()
	if err != nil {
		return app.CharacterCatalogEntry{}, 0, err
	}
	var entry app.CharacterCatalogEntry
	found := false
	for _, candidate := range catalog.Characters {
		if strings.EqualFold(candidate.Name, strings.TrimSpace(character)) {
			entry, found = candidate, true
			break
		}
	}
	if !found {
		return app.CharacterCatalogEntry{}, catalog.Revision, &commandError{code: app.CharacterReasonSaveMissing, message: "Der lokale Offline-Spielstand wurde nicht gefunden."}
	}
	if !entry.Selectable || entry.ExpectedClass == "" || entry.CombatProfile == "" {
		code := app.CharacterReasonProfileMissing
		if len(entry.Reasons) > 0 {
			code = entry.Reasons[0]
		}
		return app.CharacterCatalogEntry{}, catalog.Revision, &commandError{code: code, message: "Der Charakter ist noch nicht vollständig eingerichtet."}
	}
	settings, snapshotErr := settingsStore.Snapshot()
	if snapshotErr != nil {
		return app.CharacterCatalogEntry{}, catalog.Revision, &commandError{code: "character_setup_unavailable", message: "Die Charaktereinrichtung konnte nicht sicher gelesen werden."}
	}
	stored := settings.Characters[entry.Slug]
	profile, profileExists := b.cfg.Profiles[stored.CombatProfile]
	if stored.CharacterClass != entry.ExpectedClass || stored.CombatProfile != entry.CombatProfile ||
		!profileExists || !profile.Setup.Enabled || profile.CharacterClass != entry.ExpectedClass {
		return app.CharacterCatalogEntry{}, catalog.Revision, &commandError{code: app.CharacterReasonProfileIncompatible, message: "Das gespeicherte Kampfprofil passt nicht zur Charakterklasse."}
	}
	for _, runID := range runIDs {
		if _, ok := b.cfg.Runs.Run(strings.TrimSpace(runID)); !ok {
			return app.CharacterCatalogEntry{}, catalog.Revision, &commandError{
				code:    string(tasks.RunReasonConfigMissing),
				message: "Dieser Run ist in der Konfiguration nicht vorhanden.",
				details: map[string]any{"run_id": runID},
			}
		}
		if _, ok := app.DefaultCombatStrategyRegistry().Resolve(entry.CombatProfile, strings.TrimSpace(runID)); !ok {
			return app.CharacterCatalogEntry{}, catalog.Revision, &commandError{
				code:    string(tasks.RunReasonProfileRunStrategyUnavailable),
				message: "Das Kampfprofil unterstützt diesen Run noch nicht.",
				details: map[string]any{"run_id": runID},
			}
		}
		if assignments == nil {
			return app.CharacterCatalogEntry{}, catalog.Revision, &commandError{code: "pickit_assignment_invalid", message: "Die Lootprofil-Zuordnung ist nicht verfügbar."}
		}
		if _, resolveErr := assignments.Resolve(entry.Name, tasks.RunID(runID)); resolveErr != nil {
			return app.CharacterCatalogEntry{}, catalog.Revision, &commandError{
				code:    "pickit_assignment_invalid",
				message: "Für diesen Charakter und Run fehlt eine gültige Lootprofil-Zuordnung.",
				details: map[string]any{"run_id": runID},
			}
		}
	}
	return entry, catalog.Revision, nil
}

func characterSetupCommandError(err error) error {
	var setupErr *app.CharacterSetupError
	if errors.As(err, &setupErr) {
		if setupErr.Code == "character_setup_unavailable" {
			return setupErr
		}
		return &commandError{code: setupErr.Code, message: "Der Charakter-Setup-Stand hat sich geändert oder ist unvollständig.", details: map[string]any{"partial": setupErr.Partial}}
	}
	return err
}
