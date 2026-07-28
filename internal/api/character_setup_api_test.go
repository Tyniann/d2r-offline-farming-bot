package api

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"testing"
	"testing/fstest"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/app"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/telemetry"
)

func TestCharacterSetupHTTPContractAndTokenGates(t *testing.T) {
	backend := &characterSetupTransportBackend{}
	server, err := New(Config{
		Backend: backend, Assets: fstest.MapFS{"index.html": {Data: []byte("ok")}},
		Events: telemetry.NewLivePublisher(8, 2),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err = server.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = server.Shutdown(ctx)
	})

	reload, _ := http.Post(server.URL()+"/api/v1/characters/reload", "application/json", bytes.NewReader([]byte("{}")))
	if reload.StatusCode != http.StatusOK {
		t.Fatalf("reload=%d", reload.StatusCode)
	}
	_ = reload.Body.Close()

	preview, _ := http.Post(server.URL()+"/api/v1/characters/setup/preview", "application/json", bytes.NewReader([]byte(`{"character":"MrBones"}`)))
	if preview.StatusCode != http.StatusOK {
		t.Fatalf("preview=%d", preview.StatusCode)
	}
	_ = preview.Body.Close()

	confirmBody := []byte(`{"command_id":"setup-1","character":"MrBones","expected_catalog_revision":1,"expected_operator_settings_revision":1,"expected_pickit_assignment_revision":1,"expected_generation":0}`)
	unauthorized, _ := http.Post(server.URL()+"/api/v1/characters/setup/confirm", "application/json", bytes.NewReader(confirmBody))
	if unauthorized.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthorized=%d", unauthorized.StatusCode)
	}
	_ = unauthorized.Body.Close()
	confirm, _ := http.NewRequest(http.MethodPost, server.URL()+"/api/v1/characters/setup/confirm", bytes.NewReader(confirmBody))
	confirm.Header.Set("Content-Type", "application/json")
	confirm.Header.Set(controlTokenHeader, server.token)
	confirmed, _ := http.DefaultClient.Do(confirm)
	if confirmed.StatusCode != http.StatusOK || backend.confirms != 1 {
		t.Fatalf("confirmed=%d calls=%d", confirmed.StatusCode, backend.confirms)
	}
	_ = confirmed.Body.Close()

	captureBody := []byte(`{"command_id":"capture-1","character":"MrBones","expected_catalog_revision":1,"expected_generation":0}`)
	capture, _ := http.NewRequest(http.MethodPost, server.URL()+"/api/v1/characters/selection/capture", bytes.NewReader(captureBody))
	capture.Header.Set("Content-Type", "application/json")
	capture.Header.Set(controlTokenHeader, server.token)
	captured, _ := http.DefaultClient.Do(capture)
	if captured.StatusCode != http.StatusOK || backend.captures != 1 {
		t.Fatalf("captured=%d calls=%d", captured.StatusCode, backend.captures)
	}
	_ = captured.Body.Close()

	badMethod, _ := http.Get(server.URL() + "/api/v1/characters/setup/preview")
	if badMethod.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("method=%d", badMethod.StatusCode)
	}
	_ = badMethod.Body.Close()
	badContent, _ := http.Post(server.URL()+"/api/v1/characters/setup/preview", "text/plain", bytes.NewReader([]byte("{}")))
	if badContent.StatusCode != http.StatusUnsupportedMediaType {
		t.Fatalf("content=%d", badContent.StatusCode)
	}
	_ = badContent.Body.Close()
	unknown, _ := http.Post(server.URL()+"/api/v1/characters/setup/preview", "application/json", bytes.NewReader([]byte(`{"character":"MrBones","unknown":true}`)))
	if unknown.StatusCode != http.StatusBadRequest {
		t.Fatalf("unknown=%d", unknown.StatusCode)
	}
	_ = unknown.Body.Close()
	invalidName, _ := http.Post(server.URL()+"/api/v1/characters/setup/preview", "application/json", bytes.NewReader([]byte(`{"character":"../MrBones"}`)))
	if invalidName.StatusCode != http.StatusBadRequest {
		t.Fatalf("invalid name=%d", invalidName.StatusCode)
	}
	_ = invalidName.Body.Close()
	reloadContent, _ := http.Post(server.URL()+"/api/v1/characters/reload", "text/plain", bytes.NewReader([]byte("{}")))
	if reloadContent.StatusCode != http.StatusUnsupportedMediaType {
		t.Fatalf("reload content=%d", reloadContent.StatusCode)
	}
	_ = reloadContent.Body.Close()
	reloadUnknown, _ := http.Post(server.URL()+"/api/v1/characters/reload", "application/json", bytes.NewReader([]byte(`{"unknown":true}`)))
	if reloadUnknown.StatusCode != http.StatusBadRequest {
		t.Fatalf("reload unknown=%d", reloadUnknown.StatusCode)
	}
	_ = reloadUnknown.Body.Close()
	oversizedBody := append([]byte(`{"padding":"`), bytes.Repeat([]byte("x"), maxRequestBody+1)...)
	oversizedBody = append(oversizedBody, []byte(`"}`)...)
	oversized, _ := http.Post(server.URL()+"/api/v1/characters/reload", "application/json", bytes.NewReader(oversizedBody))
	if oversized.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("reload oversized=%d", oversized.StatusCode)
	}
	_ = oversized.Body.Close()
}

func TestCharacterSetupMutationContextRequiresGenerationIdleAndNoWorkflow(t *testing.T) {
	backend := &LiveBackend{
		characterSetup: &app.CharacterSetupService{}, characterCommands: map[string]characterSetupCommandRecord{},
		status:        StatusDTO{Generation: 4, State: string(app.SupervisorStateIdle)},
		routeWorkflow: RouteWorkflowDTO{State: string(app.RouteWorkflowIdle)},
	}
	if _, err := backend.characterSetupMutationContext("setup", 4); err != nil {
		t.Fatal(err)
	}
	if _, err := backend.characterSetupMutationContext("setup", 3); err == nil {
		t.Fatal("stale generation accepted")
	}
	backend.status.State = string(app.SupervisorStateRunningRun)
	if _, err := backend.characterSetupMutationContext("setup", 4); err == nil {
		t.Fatal("active session accepted")
	}
	backend.status.State = string(app.SupervisorStateIdle)
	backend.routeWorkflow.State = string(app.RouteWorkflowRecording)
	if _, err := backend.characterSetupMutationContext("setup", 4); err == nil {
		t.Fatal("active route workflow accepted")
	}
}

func TestCharacterSetupHTTPMapsConflictAndUnavailable(t *testing.T) {
	backend := &characterSetupTransportBackend{previewErr: &commandError{code: "config_revision_conflict", message: "stale"}}
	server := newUnstartedCharacterSetupServer(t, backend)
	response, _ := http.Post(server.URL()+"/api/v1/characters/setup/preview", "application/json", bytes.NewReader([]byte(`{"character":"MrBones"}`)))
	if response.StatusCode != http.StatusConflict {
		t.Fatalf("conflict=%d", response.StatusCode)
	}
	_ = response.Body.Close()
	backend.previewErr = errors.New("broken store")
	response, _ = http.Post(server.URL()+"/api/v1/characters/setup/preview", "application/json", bytes.NewReader([]byte(`{"character":"MrBones"}`)))
	if response.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("unavailable=%d", response.StatusCode)
	}
	_ = response.Body.Close()
}

func TestCharacterCatalogInvalidationPublishesExactlyOnceForAChangedProjection(t *testing.T) {
	publisher := telemetry.NewLivePublisher(8, 2)
	backend := &LiveBackend{
		bootstrap: &BootstrapBackend{},
		publisher: publisher,
		catalog:   CatalogDTO{Revision: 1},
	}
	catalog := app.CharacterCatalog{Revision: 2, Characters: []app.CharacterCatalogEntry{{Name: "MrBones", Slug: "mrbones"}}}
	backend.publishCharacterCatalog(catalog, false)
	if publisher.Sequence() != 0 {
		t.Fatalf("unchanged publish sequence = %d", publisher.Sequence())
	}
	backend.publishCharacterCatalog(catalog, true)
	if publisher.Sequence() != 1 {
		t.Fatalf("changed publish sequence = %d", publisher.Sequence())
	}
	replay, subscription := publisher.Subscribe(0)
	subscription.Close()
	if len(replay) != 1 || replay[0].Event != "catalog_changed" || replay[0].Details["revision"] != uint64(2) {
		t.Fatalf("catalog events = %+v", replay)
	}
}

type characterSetupTransportBackend struct {
	apiTestBackend
	confirms   int
	captures   int
	previewErr error
}

func (b *characterSetupTransportBackend) ReloadCharacters() (CharacterReloadDTO, error) {
	return CharacterReloadDTO{SchemaVersion: 1, Catalog: b.Catalog()}, nil
}
func (b *characterSetupTransportBackend) PreviewCharacterSetup(CharacterSetupPreviewRequest) (CharacterSetupPreviewDTO, error) {
	return sampleCharacterSetupPreviewDTO(), b.previewErr
}
func (b *characterSetupTransportBackend) ConfirmCharacterSetup(CharacterSetupConfirmRequest) (CharacterSetupPreviewDTO, error) {
	b.confirms++
	return sampleCharacterSetupPreviewDTO(), nil
}
func (b *characterSetupTransportBackend) CaptureCharacterSelection(context.Context, CharacterSelectionCaptureRequest) (CharacterSetupPreviewDTO, error) {
	b.captures++
	value := sampleCharacterSetupPreviewDTO()
	value.AnchorState, value.SetupState = "ready", "ready"
	return value, nil
}

func sampleCharacterSetupPreviewDTO() CharacterSetupPreviewDTO {
	return CharacterSetupPreviewDTO{
		SchemaVersion: 1, CatalogRevision: 1, OperatorSettingsRevision: 1, PickitAssignmentRevision: 1,
		Character: CharacterSetupCharacterDTO{Name: "MrBones", Slug: "mrbones", CharacterClass: "necromancer", ClassDisplayName: "Totenbeschwörer"},
		Supported: true, Profiles: []CharacterSetupProfileDTO{{ID: "necro_bone_spear", DisplayName: "Knochen-Speer", IsDefault: true}},
		PickitDefaults: []CharacterSetupPickitDefaultDTO{}, AnchorState: "missing", SetupState: "needs_setup", Reasons: []string{"character_profile_missing"},
	}
}

func newUnstartedCharacterSetupServer(t *testing.T, backend Backend) *Server {
	t.Helper()
	server, err := New(Config{Backend: backend, Assets: fstest.MapFS{"index.html": {Data: []byte("ok")}}, Events: telemetry.NewLivePublisher(8, 2)})
	if err != nil {
		t.Fatal(err)
	}
	if err = server.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = server.Shutdown(ctx)
	})
	return server
}
