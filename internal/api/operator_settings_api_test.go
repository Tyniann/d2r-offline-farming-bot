package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"path/filepath"
	"testing"
	"testing/fstest"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/app"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/telemetry"
)

func TestOperatorSettingsBackendEnforcesGenerationIdleLockAndControlledRestart(t *testing.T) {
	backend := newSelectionTestBackend(t)
	root := t.TempDir()
	store, err := app.NewOperatorSettingsStore(root, backend.cfg, []string{"MrBones", "MrHammer"})
	if err != nil {
		t.Fatal(err)
	}
	backend.SetOperatorSettingsStore(store)
	current, err := backend.OperatorSettings()
	if err != nil {
		t.Fatal(err)
	}
	draft := cloneOperatorSettingsDTO(current)
	draft.Input.Enabled = !draft.Input.Enabled
	preview, err := backend.PreviewOperatorSettings(OperatorSettingsMutationRequest{ExpectedRevision: current.Revision, ExpectedGeneration: 0, Settings: draft})
	if err != nil || !preview.RestartRequired || preview.ReasonCode != string(app.Phase15ReasonConfigRestartRequired) {
		t.Fatalf("preview=%+v err=%v", preview, err)
	}
	beforeEnabled := backend.cfg.Input.Enabled
	updated, err := backend.UpdateOperatorSettings(OperatorSettingsMutationRequest{ExpectedRevision: current.Revision, ExpectedGeneration: 0, Settings: draft})
	if err != nil || !updated.RestartRequired || backend.cfg.Input.Enabled != beforeEnabled {
		t.Fatalf("updated=%+v err=%v runtime_input=%t", updated, err, backend.cfg.Input.Enabled)
	}

	backend.UpdateSupervisor(app.SupervisorSnapshot{State: app.SupervisorStateRunningRun, Generation: 5, QueueKnown: true, Queue: []string{"countess"}})
	activeDraft := cloneOperatorSettingsDTO(updated.Settings)
	activeDraft.Budgets.MaxRuns++
	if _, err := backend.UpdateOperatorSettings(OperatorSettingsMutationRequest{ExpectedRevision: updated.Settings.Revision, ExpectedGeneration: 5, Settings: activeDraft}); err == nil {
		t.Fatal("active session accepted settings mutation")
	}
	if _, err := backend.UpdateOperatorSettings(OperatorSettingsMutationRequest{ExpectedRevision: updated.Settings.Revision, ExpectedGeneration: 4, Settings: activeDraft}); err == nil {
		t.Fatal("stale generation was accepted")
	}
}

func TestOperatorSettingsPreviewReturnsActionablePartialCTAError(t *testing.T) {
	backend := newSelectionTestBackend(t)
	store, err := app.NewOperatorSettingsStore(t.TempDir(), backend.cfg, []string{"MrHammer"})
	if err != nil {
		t.Fatal(err)
	}
	initial, err := store.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	assigned, err := store.AssignCharacterProfile("MrHammer", "paladin", "paladin_hammerdin", initial.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if err = backend.SetOperatorSettingsStore(store); err != nil {
		t.Fatal(err)
	}
	draft := operatorSettingsDTO(assigned.Settings)
	character := draft.Characters["mrhammer"]
	character.ProfileBindings = map[string]OperatorProfileBindingsDTO{"paladin_hammerdin": {
		Skills: map[string]string{"battle_command": "f6"},
	}}
	draft.Characters["mrhammer"] = character
	_, err = backend.PreviewOperatorSettings(OperatorSettingsMutationRequest{
		ExpectedRevision: assigned.Settings.Revision, ExpectedGeneration: 0, Settings: draft,
	})
	var commandErr *commandError
	if !errors.As(err, &commandErr) || commandErr.code != "config_invalid" || commandErr.message != "Für Call to Arms müssen Battle Command und Battle Orders beide belegt sein." {
		t.Fatalf("partial CTA API error=%v", err)
	}
}

func TestOperatorSettingsHTTPReadPreviewUpdateResetAndTokenGate(t *testing.T) {
	backend := &operatorSettingsTransportBackend{settings: sampleOperatorSettingsDTO()}
	assets := fstest.MapFS{"index.html": {Data: []byte("ok")}}
	server, err := New(Config{Backend: backend, Assets: assets, Events: telemetry.NewLivePublisher(8, 2)})
	if err != nil {
		t.Fatal(err)
	}
	if startErr := server.Start(); startErr != nil {
		t.Fatal(startErr)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = server.Shutdown(ctx)
	})

	response, err := http.Get(server.URL() + "/api/v1/settings/operator")
	if err != nil || response.StatusCode != http.StatusOK {
		t.Fatalf("GET status=%v err=%v", response.StatusCode, err)
	}
	var read OperatorSettingsDTO
	if err := json.NewDecoder(response.Body).Decode(&read); err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if got := read.Characters["mrbones"]; read.SchemaVersion != 3 || got.CharacterClass != "necromancer" || got.CombatProfile != "necro_bone_spear" {
		t.Fatalf("read=%+v", read)
	}

	mutation := OperatorSettingsMutationRequest{ExpectedRevision: 1, ExpectedGeneration: 3, Settings: backend.settings}
	body, _ := json.Marshal(mutation)
	preview, _ := http.Post(server.URL()+"/api/v1/settings/operator/preview", "application/json", bytes.NewReader(body))
	if preview.StatusCode != http.StatusOK {
		t.Fatalf("preview status=%d", preview.StatusCode)
	}
	_ = preview.Body.Close()

	unauthorized, _ := http.NewRequest(http.MethodPut, server.URL()+"/api/v1/settings/operator", bytes.NewReader(body))
	unauthorized.Header.Set("Content-Type", "application/json")
	unauthorizedResponse, _ := http.DefaultClient.Do(unauthorized)
	assertAPIError(t, unauthorizedResponse, http.StatusUnauthorized, "request_unauthorized")
	_ = unauthorizedResponse.Body.Close()

	update, _ := http.NewRequest(http.MethodPut, server.URL()+"/api/v1/settings/operator", bytes.NewReader(body))
	update.Header.Set("Content-Type", "application/json")
	update.Header.Set(controlTokenHeader, server.token)
	updateResponse, _ := http.DefaultClient.Do(update)
	if updateResponse.StatusCode != http.StatusOK || backend.updates != 1 {
		t.Fatalf("update status=%d updates=%d", updateResponse.StatusCode, backend.updates)
	}
	_ = updateResponse.Body.Close()

	resetBody, _ := json.Marshal(OperatorSettingsResetRequest{ExpectedRevision: 2, ExpectedGeneration: 3})
	reset, _ := http.NewRequest(http.MethodPost, server.URL()+"/api/v1/settings/operator/reset", bytes.NewReader(resetBody))
	reset.Header.Set("Content-Type", "application/json")
	reset.Header.Set(controlTokenHeader, server.token)
	resetResponse, _ := http.DefaultClient.Do(reset)
	if resetResponse.StatusCode != http.StatusOK || backend.resets != 1 {
		t.Fatalf("reset status=%d resets=%d", resetResponse.StatusCode, backend.resets)
	}
	_ = resetResponse.Body.Close()
}

type operatorSettingsTransportBackend struct {
	apiTestBackend
	settings OperatorSettingsDTO
	updates  int
	resets   int
}

func (b *operatorSettingsTransportBackend) OperatorSettings() (OperatorSettingsDTO, error) {
	return b.settings, nil
}

func (b *operatorSettingsTransportBackend) PreviewOperatorSettings(request OperatorSettingsMutationRequest) (OperatorSettingsChangeDTO, error) {
	return OperatorSettingsChangeDTO{SchemaVersion: 1, Generation: request.ExpectedGeneration, Settings: request.Settings, ChangedFields: []string{"budgets"}}, nil
}

func (b *operatorSettingsTransportBackend) UpdateOperatorSettings(request OperatorSettingsMutationRequest) (OperatorSettingsChangeDTO, error) {
	b.updates++
	request.Settings.Revision++
	b.settings = request.Settings
	return OperatorSettingsChangeDTO{SchemaVersion: 1, Generation: request.ExpectedGeneration, Settings: request.Settings, ChangedFields: []string{"budgets"}}, nil
}

func (b *operatorSettingsTransportBackend) PreviewResetOperatorSettings(request OperatorSettingsResetRequest) (OperatorSettingsChangeDTO, error) {
	return OperatorSettingsChangeDTO{SchemaVersion: 1, Generation: request.ExpectedGeneration, Settings: b.settings, ChangedFields: []string{"input"}, RestartRequired: true}, nil
}

func (b *operatorSettingsTransportBackend) ResetOperatorSettings(request OperatorSettingsResetRequest) (OperatorSettingsChangeDTO, error) {
	b.resets++
	return OperatorSettingsChangeDTO{SchemaVersion: 1, Generation: request.ExpectedGeneration, Settings: b.settings}, nil
}

func sampleOperatorSettingsDTO() OperatorSettingsDTO {
	return OperatorSettingsDTO{
		SchemaVersion: 3, Revision: 1,
		Characters: map[string]OperatorCharacterSettingsDTO{"mrbones": {
			CharacterClass: "necromancer", CombatProfile: "necro_bone_spear",
			LastDifficulty: "nightmare", Queue: []string{"countess", "mephisto"},
		}},
		Budgets: OperatorBudgetSettingsDTO{MaxRuns: 3, MaxDurationMs: 7200000, MaxConsecutiveFailures: 2, MaxTotalRestarts: 3},
		Input:   OperatorInputSettingsDTO{PauseHotkey: "pause", StopAfterRunHotkey: "f10", RecordingFinishHotkey: "f9", EmergencyStopHotkey: "f11"},
		History: OperatorHistorySettingsDTO{RetentionEnabled: true, RetentionDays: 60},
	}
}

func cloneOperatorSettingsDTO(settings OperatorSettingsDTO) OperatorSettingsDTO {
	clone := settings
	clone.Characters = make(map[string]OperatorCharacterSettingsDTO, len(settings.Characters))
	for character, value := range settings.Characters {
		value.Queue = append([]string(nil), value.Queue...)
		clone.Characters[character] = value
	}
	return clone
}

func TestInstalledOperatorSettingsAreAppliedBeforeRuntimeConstruction(t *testing.T) {
	cfg, err := config.Load(filepath.Join("..", "..", "configs", "config.example.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	cfg.Session.Character = "MrBones"
	store, err := app.NewOperatorSettingsStore(t.TempDir(), cfg, []string{"MrBones"})
	if err != nil {
		t.Fatal(err)
	}
	settings, _ := store.Snapshot()
	settings.Characters["mrbones"] = app.OperatorCharacterSettings{LastDifficulty: "hell", Queue: []string{"mephisto", "countess"}}
	settings.Input.Enabled = true
	updated, err := store.Update(settings.Revision, settings)
	if err != nil {
		t.Fatal(err)
	}
	app.ApplyOperatorSettingsToConfig(cfg, updated.Settings)
	if cfg.Session.Difficulty != "hell" || cfg.Session.Run != "mephisto" || !cfg.Input.Enabled {
		t.Fatalf("cfg session=%+v input=%+v", cfg.Session, cfg.Input)
	}
}
