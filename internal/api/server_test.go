package api

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io/fs"
	"log/slog"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"testing/fstest"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/telemetry"
)

type apiTestBackend struct {
	commands atomic.Int32
	previews atomic.Int32
	queueErr error
}

func (b *apiTestBackend) Status() StatusDTO {
	return StatusDTO{CoreVersion: "test", State: "idle", D2R: D2RDTO{State: "detached"}, World: WorldDTO{Phase: "unknown"}}
}

func (b *apiTestBackend) Catalog() CatalogDTO {
	return CatalogDTO{Revision: 1, Runs: []RunCatalogEntry{{RunID: "countess", DisplayName: "Countess", Status: "runtime_validation_required"}}}
}

func (b *apiTestBackend) PreviewSelection(request SelectionPreviewRequest) (SelectionPreviewDTO, error) {
	b.previews.Add(1)
	return SelectionPreviewDTO{Character: request.Character, NewDifficulty: request.Difficulty, CatalogRevision: request.CatalogRevision, ConfirmationToken: "preview"}, nil
}

func (b *apiTestBackend) ValidateQueue(request QueueValidationRequest) (QueueValidationDTO, error) {
	if b.queueErr != nil {
		return QueueValidationDTO{}, b.queueErr
	}
	return QueueValidationDTO{Entries: append([]string(nil), request.Entries...), Character: request.Character, Difficulty: request.Difficulty, CatalogRevision: request.CatalogRevision}, nil
}

func TestSelectionPreviewIsTokenFreeStrictAndOriginProtected(t *testing.T) {
	server, backend := startAPITestServer(t)
	request, err := http.NewRequest(http.MethodPost, server.URL()+"/api/v1/selection/preview", strings.NewReader(`{"character":"MrBones","difficulty":"nightmare","catalog_revision":1}`))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK || backend.previews.Load() != 1 {
		t.Fatalf("preview status=%d calls=%d", response.StatusCode, backend.previews.Load())
	}

	request, err = http.NewRequest(http.MethodPost, server.URL()+"/api/v1/selection/preview", strings.NewReader(`{"character":"MrBones","difficulty":"nightmare","catalog_revision":1,"unknown":true}`))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	response, err = http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	assertAPIError(t, response, http.StatusBadRequest, "request_invalid")
	_ = response.Body.Close()

	request, err = http.NewRequest(http.MethodPost, server.URL()+"/api/v1/selection/preview", strings.NewReader(`{"character":"MrBones","difficulty":"nightmare","catalog_revision":1}`))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", "https://evil.example")
	response, err = http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	assertAPIError(t, response, http.StatusForbidden, "origin_rejected")
	_ = response.Body.Close()
	if backend.previews.Load() != 1 {
		t.Fatalf("rejected previews reached backend: %d", backend.previews.Load())
	}
}

func TestQueueValidationIsTokenFreeAndStrict(t *testing.T) {
	server, _ := startAPITestServer(t)
	request, err := http.NewRequest(http.MethodPost, server.URL()+"/api/v1/queue/validate", strings.NewReader(`{"entries":["countess","mephisto"],"character":"MrBones","difficulty":"nightmare","catalog_revision":1}`))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var validation QueueValidationDTO
	if decodeErr := json.NewDecoder(response.Body).Decode(&validation); decodeErr != nil {
		t.Fatal(decodeErr)
	}
	if response.StatusCode != http.StatusOK || validation.SchemaVersion != 1 || len(validation.Entries) != 2 {
		t.Fatalf("queue validation status=%d body=%+v", response.StatusCode, validation)
	}

	request, err = http.NewRequest(http.MethodPost, server.URL()+"/api/v1/queue/validate", strings.NewReader(`{"entries":[],"character":"MrBones","difficulty":"nightmare","catalog_revision":1,"unknown":true}`))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	response, err = http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	assertAPIError(t, response, http.StatusBadRequest, "request_invalid")
}

func TestQueueValidationReturnsDuplicateIndices(t *testing.T) {
	server, backend := startAPITestServer(t)
	backend.queueErr = &commandError{code: "queue_duplicate_run", message: "Die Farm-Queue enthält einen Run mehrfach.", details: map[string]any{"run_id": "countess", "first_index": 0, "duplicate_index": 2}}
	request, err := http.NewRequest(http.MethodPost, server.URL()+"/api/v1/queue/validate", strings.NewReader(`{"entries":["countess","mephisto","countess"],"character":"MrBones","difficulty":"nightmare","catalog_revision":1}`))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var apiErr ErrorDTO
	if err := json.NewDecoder(response.Body).Decode(&apiErr); err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusConflict || apiErr.Code != "queue_duplicate_run" || apiErr.Details["run_id"] != "countess" || apiErr.Details["first_index"] != float64(0) || apiErr.Details["duplicate_index"] != float64(2) {
		t.Fatalf("duplicate response status=%d body=%+v", response.StatusCode, apiErr)
	}
}

func TestServerEventsSendSnapshotAndReplay(t *testing.T) {
	publisher := telemetry.NewLivePublisher(8, 4)
	publisher.Publish(telemetry.LiveEvent{Event: "area_changed", AreaID: 1, Area: "Rogue Encampment"})
	server, _ := startAPITestServerWithEvents(t, publisher)
	request, err := http.NewRequest(http.MethodGet, server.URL()+"/api/v1/events", nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Last-Event-ID", "0")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	reader := bufio.NewReader(response.Body)
	stream := ""
	for strings.Count(stream, "\n\n") < 2 {
		line, readErr := reader.ReadString('\n')
		if readErr != nil {
			t.Fatal(readErr)
		}
		stream += line
	}
	if !strings.Contains(stream, "event: snapshot") || !strings.Contains(stream, "event: area_changed") || !strings.Contains(stream, "id: 1") {
		t.Fatalf("unexpected event stream: %q", stream)
	}
}

func TestServerEventsRejectInvalidLastEventID(t *testing.T) {
	server, _ := startAPITestServer(t)
	request, err := http.NewRequest(http.MethodGet, server.URL()+"/api/v1/events", nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Last-Event-ID", "not-a-sequence")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	assertAPIError(t, response, http.StatusBadRequest, "request_invalid")
}

func (b *apiTestBackend) Command(_ string, request CommandRequest) (CommandResponse, error) {
	b.commands.Add(1)
	return CommandResponse{CommandID: request.CommandID, Generation: request.ExpectedGeneration + 1, State: "starting_run"}, nil
}

func TestServerBindsLoopbackAndServesVersionedQueries(t *testing.T) {
	server, backend := startAPITestServer(t)
	if !strings.HasPrefix(server.URL(), "http://127.0.0.1:") {
		t.Fatalf("server URL = %q", server.URL())
	}
	response, err := http.Get(server.URL() + "/api/v1/status")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var status StatusDTO
	if err := json.NewDecoder(response.Body).Decode(&status); err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK || status.SchemaVersion != 1 || status.State != "idle" {
		t.Fatalf("status response=%d body=%+v", response.StatusCode, status)
	}
	if backend.commands.Load() != 0 {
		t.Fatal("read-only status invoked a command")
	}
}

func TestServerRejectsForeignOriginBeforeCommand(t *testing.T) {
	server, backend := startAPITestServer(t)
	request := newCommandRequest(t, server, "/api/v1/session/start", `{"command_id":"start","expected_generation":0}`)
	request.Header.Set("Origin", "https://evil.example")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	assertAPIError(t, response, http.StatusForbidden, "origin_rejected")
	if backend.commands.Load() != 0 {
		t.Fatal("foreign origin reached command backend")
	}
}

func TestServerRejectsWrongHostAndUnsupportedAPIVersion(t *testing.T) {
	server, _ := startAPITestServer(t)
	request, err := http.NewRequest(http.MethodGet, server.URL()+"/api/v1/status", nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Host = "evil.example"
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	assertAPIError(t, response, http.StatusBadRequest, "request_invalid")
	_ = response.Body.Close()

	response, err = http.Get(server.URL() + "/api/v2/status")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	assertAPIError(t, response, http.StatusNotFound, "api_version_unsupported")
}

func TestServerSecurityEnvelope(t *testing.T) {
	server, backend := startAPITestServer(t)
	tests := []struct {
		name        string
		method      string
		contentType string
		token       string
		body        string
		wantStatus  int
		wantCode    string
	}{
		{name: "method", method: http.MethodGet, wantStatus: http.StatusMethodNotAllowed, wantCode: "request_invalid"},
		{name: "content type", method: http.MethodPost, token: server.token, body: `{}`, wantStatus: http.StatusUnsupportedMediaType, wantCode: "request_invalid"},
		{name: "token", method: http.MethodPost, contentType: "application/json", body: `{}`, wantStatus: http.StatusUnauthorized, wantCode: "request_unauthorized"},
		{name: "malformed", method: http.MethodPost, contentType: "application/json", token: server.token, body: `{`, wantStatus: http.StatusBadRequest, wantCode: "request_invalid"},
		{name: "command id", method: http.MethodPost, contentType: "application/json", token: server.token, body: `{"expected_generation":0}`, wantStatus: http.StatusBadRequest, wantCode: "request_invalid"},
		{name: "unknown field", method: http.MethodPost, contentType: "application/json", token: server.token, body: `{"command_id":"x","expected_generation":0,"unknown":true}`, wantStatus: http.StatusBadRequest, wantCode: "request_invalid"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request, err := http.NewRequest(test.method, server.URL()+"/api/v1/session/start", strings.NewReader(test.body))
			if err != nil {
				t.Fatal(err)
			}
			request.Header.Set("Content-Type", test.contentType)
			request.Header.Set(controlTokenHeader, test.token)
			response, err := http.DefaultClient.Do(request)
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			assertAPIError(t, response, test.wantStatus, test.wantCode)
		})
	}
	if backend.commands.Load() != 0 {
		t.Fatalf("rejected requests invoked %d commands", backend.commands.Load())
	}
}

func TestServerRejectsOversizedPayload(t *testing.T) {
	server, backend := startAPITestServer(t)
	body := `{"command_id":"x","expected_generation":0,"payload":"` + strings.Repeat("a", maxRequestBody) + `"}`
	request := newCommandRequest(t, server, "/api/v1/session/start", body)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	assertAPIError(t, response, http.StatusRequestEntityTooLarge, "payload_too_large")
	if backend.commands.Load() != 0 {
		t.Fatal("oversized request reached command backend")
	}
}

func TestServerAcceptsAuthenticatedCommandAndReturnsRequestID(t *testing.T) {
	server, backend := startAPITestServer(t)
	request := newCommandRequest(t, server, "/api/v1/session/start", `{"command_id":"start-1","expected_generation":7}`)
	request.Header.Set("Origin", server.URL())
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK || response.Header.Get("X-Request-ID") == "" {
		t.Fatalf("response status=%d request_id=%q", response.StatusCode, response.Header.Get("X-Request-ID"))
	}
	var command CommandResponse
	if err := json.NewDecoder(response.Body).Decode(&command); err != nil {
		t.Fatal(err)
	}
	if command.SchemaVersion != 1 || command.CommandID != "start-1" || command.Generation != 8 {
		t.Fatalf("command response = %+v", command)
	}
	if backend.commands.Load() != 1 {
		t.Fatalf("command calls = %d", backend.commands.Load())
	}
}

func TestControlTokenIsNotLoggedOrIncludedInSafeURL(t *testing.T) {
	var logs bytes.Buffer
	backend := &apiTestBackend{}
	assets := fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("ok"), Mode: fs.FileMode(0o644)}}
	server, err := New(Config{Backend: backend, Assets: assets, Logger: slog.New(slog.NewTextHandler(&logs, nil))})
	if err != nil {
		t.Fatal(err)
	}
	if err := server.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = server.Shutdown(context.Background()) })
	if strings.Contains(server.URL(), server.token) || strings.Contains(logs.String(), server.token) {
		t.Fatal("control token leaked into safe URL or logs")
	}
	if !strings.Contains(server.BootstrapURL(), "#control_token="+server.token) {
		t.Fatalf("bootstrap URL does not carry fragment token: %q", server.BootstrapURL())
	}
}

func TestControlBootstrapRequiresCustomHeaderAndRejectsForeignOrigin(t *testing.T) {
	server, _ := startAPITestServer(t)
	response, err := http.Get(server.URL() + "/api/v1/control/bootstrap")
	if err != nil {
		t.Fatal(err)
	}
	assertAPIError(t, response, http.StatusUnauthorized, "request_unauthorized")
	_ = response.Body.Close()

	request, err := http.NewRequest(http.MethodGet, server.URL()+"/api/v1/control/bootstrap", nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set(bootstrapHeader, "1")
	request.Header.Set("Origin", "https://evil.example")
	response, err = http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	assertAPIError(t, response, http.StatusForbidden, "origin_rejected")
	_ = response.Body.Close()
}

func TestControlBootstrapRestoresProcessTokenWithoutCaching(t *testing.T) {
	server, _ := startAPITestServer(t)
	request, err := http.NewRequest(http.MethodGet, server.URL()+"/api/v1/control/bootstrap", nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set(bootstrapHeader, "1")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var body struct {
		Token string `json:"control_token"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK || body.Token != server.token || response.Header.Get("Cache-Control") != "no-store" {
		t.Fatalf("bootstrap status=%d token_match=%t cache=%q", response.StatusCode, body.Token == server.token, response.Header.Get("Cache-Control"))
	}
}

func startAPITestServer(t *testing.T) (*Server, *apiTestBackend) {
	return startAPITestServerWithEvents(t, nil)
}

func startAPITestServerWithEvents(t *testing.T, events *telemetry.LivePublisher) (*Server, *apiTestBackend) {
	t.Helper()
	backend := &apiTestBackend{}
	assets := fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("ok"), Mode: fs.FileMode(0o644)}}
	server, err := New(Config{Backend: backend, Assets: assets, Events: events})
	if err != nil {
		t.Fatal(err)
	}
	if err := server.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			t.Errorf("shutdown API server: %v", err)
		}
	})
	return server, backend
}

func newCommandRequest(t *testing.T, server *Server, path, body string) *http.Request {
	t.Helper()
	request, err := http.NewRequest(http.MethodPost, server.URL()+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set(controlTokenHeader, server.token)
	return request
}

func assertAPIError(t *testing.T, response *http.Response, status int, code string) {
	t.Helper()
	if response.StatusCode != status {
		t.Fatalf("status = %d, want %d", response.StatusCode, status)
	}
	var apiError ErrorDTO
	if err := json.NewDecoder(response.Body).Decode(&apiError); err != nil {
		t.Fatal(err)
	}
	if apiError.Code != code || apiError.Message == "" || apiError.RequestID == "" {
		t.Fatalf("API error = %+v", apiError)
	}
}
