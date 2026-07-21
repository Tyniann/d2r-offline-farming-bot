package api

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/telemetry"
)

const (
	controlTokenHeader = "X-D2RBot-Control-Token"
	bootstrapHeader    = "X-D2RBot-Bootstrap"
	maxRequestBody     = 64 << 10
)

var commandPaths = map[string]string{
	"/api/v1/selection/apply":         "apply_selection",
	"/api/v1/session/start":           "start_queue",
	"/api/v1/session/pause-after-run": "pause_after_run",
	"/api/v1/session/resume":          "resume",
	"/api/v1/session/stop-after-run":  "stop_after_run",
	"/api/v1/session/emergency-stop":  "emergency_stop",
}

// Config contains transport dependencies for one loopback server.
type Config struct {
	Backend Backend
	Assets  fs.FS
	Logger  *slog.Logger
	Events  *telemetry.LivePublisher
}

type routeBackend interface {
	RouteLibrary(string, bool) (RouteLibraryDTO, error)
	RecordingOptions() []RecordingOptionDTO
	RouteCandidates() ([]RouteCandidateDTO, error)
	SystemRouteStatuses() []SystemRouteStatusDTO
	HotkeyHelp() HotkeyHelpDTO
	RouteWorkflow() RouteWorkflowDTO
	PreviewRouteMutation(RouteMutationPreviewRequest) (RouteMutationPreviewDTO, error)
	ConfirmRouteMutation(RouteMutationConfirmRequest) error
	StartRouteWorkflow(RouteWorkflowRequest) (RouteWorkflowDTO, error)
	FinishRouteWorkflow(string, RouteWorkflowFinishRequest) (RouteWorkflowDTO, error)
}

// Server owns one random loopback listener and its process-local control token.
type Server struct {
	backend    Backend
	logger     *slog.Logger
	assets     fs.FS
	events     *telemetry.LivePublisher
	listener   net.Listener
	httpServer *http.Server
	token      string
	baseURL    string
}

// New constructs a server without binding a socket.
func New(config Config) (*Server, error) {
	if config.Backend == nil {
		return nil, fmt.Errorf("API backend is required")
	}
	if config.Assets == nil {
		return nil, fmt.Errorf("API UI assets are required")
	}
	if config.Logger == nil {
		config.Logger = slog.Default()
	}
	if config.Events == nil {
		config.Events = telemetry.NewLivePublisher(256, 64)
	}
	token, err := randomToken(32)
	if err != nil {
		return nil, err
	}
	return &Server{backend: config.Backend, logger: config.Logger, assets: config.Assets, events: config.Events, token: token}, nil
}

// Start binds exclusively to an ephemeral IPv4 loopback port and serves until shutdown.
func (s *Server) Start() error {
	if s == nil {
		return fmt.Errorf("API server is nil")
	}
	if s.listener != nil {
		return fmt.Errorf("API server already started")
	}
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("listen on IPv4 loopback: %w", err)
	}
	s.listener = listener
	s.baseURL = "http://" + listener.Addr().String()
	s.httpServer = &http.Server{Handler: s.routes(), ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 30 * time.Second}
	go func() {
		if serveErr := s.httpServer.Serve(listener); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			s.logger.Error("local API server failed", "error", serveErr)
		}
	}()
	return nil
}

// URL returns the safe loopback URL without the control token.
func (s *Server) URL() string {
	if s == nil {
		return ""
	}
	return s.baseURL
}

// BootstrapURL returns the one-time browser URL with the token in its fragment.
// Callers must never log or persist this value.
func (s *Server) BootstrapURL() string {
	if s == nil || s.baseURL == "" {
		return ""
	}
	return s.baseURL + "/#control_token=" + s.token
}

// Shutdown gracefully closes the local HTTP server.
func (s *Server) Shutdown(ctx context.Context) error {
	if s == nil || s.httpServer == nil {
		return nil
	}
	if err := s.httpServer.Shutdown(ctx); err != nil {
		return fmt.Errorf("shutdown local API server: %w", err)
	}
	return nil
}

func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/status", s.handleStatus)
	mux.HandleFunc("/api/v1/catalog", s.handleCatalog)
	mux.HandleFunc("/api/v1/pickit/catalog", s.handlePickitCatalog)
	mux.HandleFunc("/api/v1/pickit/profiles", s.handlePickitProfiles)
	mux.HandleFunc("/api/v1/pickit/profiles/validate", s.handlePickitValidation)
	mux.HandleFunc("/api/v1/pickit/preview", s.handlePickitPreview)
	mux.HandleFunc("/api/v1/pickit/assignments", s.handlePickitAssignments)
	mux.HandleFunc("/api/v1/pickit/import", s.handlePickitImport)
	mux.HandleFunc("/api/v1/pickit/profiles/{profileID}/duplicate", s.handlePickitDuplicate)
	mux.HandleFunc("/api/v1/pickit/profiles/{profileID}/export", s.handlePickitExport)
	mux.HandleFunc("/api/v1/pickit/profiles/{profileID}", s.handlePickitProfile)
	mux.HandleFunc("/api/v1/events", s.handleEvents)
	mux.HandleFunc("/api/v1/control/bootstrap", s.handleControlBootstrap)
	for path, command := range commandPaths {
		commandName := command
		mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) { s.handleCommand(w, r, commandName) })
	}
	mux.HandleFunc("/api/v1/selection/preview", s.handleSelectionPreview)
	mux.HandleFunc("/api/v1/queue/validate", s.handleQueueValidation)
	mux.HandleFunc("/api/v1/routes", s.handleRouteLibrary)
	mux.HandleFunc("/api/v1/routes/recording-options", s.handleRecordingOptions)
	mux.HandleFunc("/api/v1/route-recording/options", s.handleRecordingOptions)
	mux.HandleFunc("/api/v1/route-recordings", s.handleRouteRecordingStart)
	mux.HandleFunc("/api/v1/route-recordings/{workflowID}/finish", s.handleRouteRecordingFinish)
	mux.HandleFunc("/api/v1/route-candidates/{candidateID}/test", s.handleRouteCandidateTest)
	mux.HandleFunc("/api/v1/route-candidates/{candidateID}/publish/{stage}", s.handleCandidatePublish)
	mux.HandleFunc("/api/v1/routes/{routeID}/{operation}/{stage}", s.handleNamedRouteMutation)
	mux.HandleFunc("/api/v1/routes/candidates", s.handleRouteCandidates)
	mux.HandleFunc("/api/v1/routes/system-status", s.handleSystemRouteStatus)
	mux.HandleFunc("/api/v1/system-routes/status", s.handleSystemRouteStatus)
	mux.HandleFunc("/api/v1/routes/hotkeys", s.handleHotkeyHelp)
	mux.HandleFunc("/api/v1/routes/workflow", s.handleRouteWorkflow)
	mux.HandleFunc("/api/v1/routes/workflow/start", s.handleRouteWorkflowStart)
	mux.HandleFunc("/api/v1/routes/preview", s.handleRouteMutationPreview)
	mux.HandleFunc("/api/v1/routes/confirm", s.handleRouteMutationConfirm)
	mux.HandleFunc("/api/", s.handleUnsupportedAPI)
	mux.Handle("/", spaHandler(s.assets))
	return s.security(mux)
}

func (s *Server) pickitBackend(w http.ResponseWriter, r *http.Request) (pickitBackend, bool) {
	backend, ok := s.backend.(pickitBackend)
	if !ok {
		s.writeError(w, http.StatusNotImplemented, "feature_unavailable", "Pickit ist nicht verfügbar.", requestIDFrom(r), nil)
	}
	return backend, ok
}

func (s *Server) handlePickitCatalog(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet, s) {
		return
	}
	backend, ok := s.pickitBackend(w, r)
	if ok {
		s.writeJSON(w, http.StatusOK, backend.PickitCatalog())
	}
}
func (s *Server) handlePickitProfiles(w http.ResponseWriter, r *http.Request) {
	backend, ok := s.pickitBackend(w, r)
	if !ok {
		return
	}
	switch r.Method {
	case http.MethodGet:
		value, err := backend.PickitProfiles()
		if err != nil {
			s.writePickitError(w, r, err)
			return
		}
		s.writeJSON(w, http.StatusOK, value)
	case http.MethodPost:
		if !requireJSONMutation(w, r, s, http.MethodPost) {
			return
		}
		var request PickitCreateRequest
		if !s.decodeBody(w, r, &request) {
			return
		}
		value, err := backend.CreatePickit(request)
		if err != nil {
			s.writePickitError(w, r, err)
			return
		}
		s.writeJSON(w, http.StatusCreated, value)
	default:
		requireMethod(w, r, http.MethodGet, s)
	}
}
func (s *Server) handlePickitValidation(w http.ResponseWriter, r *http.Request) {
	if !requireJSONPost(w, r, s, false) {
		return
	}
	backend, ok := s.pickitBackend(w, r)
	if !ok {
		return
	}
	var request PickitValidationRequest
	if !s.decodeBody(w, r, &request) {
		return
	}
	value, err := backend.ValidatePickit(request)
	if err != nil {
		s.writePickitError(w, r, err)
		return
	}
	s.writeJSON(w, http.StatusOK, value)
}
func (s *Server) handlePickitPreview(w http.ResponseWriter, r *http.Request) {
	if !requireJSONPost(w, r, s, false) {
		return
	}
	backend, ok := s.pickitBackend(w, r)
	if !ok {
		return
	}
	var request PickitPreviewRequest
	if !s.decodeBody(w, r, &request) {
		return
	}
	value, err := backend.PreviewPickit(request)
	if err != nil {
		s.writePickitError(w, r, err)
		return
	}
	s.writeJSON(w, http.StatusOK, value)
}
func (s *Server) handlePickitProfile(w http.ResponseWriter, r *http.Request) {
	backend, ok := s.pickitBackend(w, r)
	if !ok {
		return
	}
	id := r.PathValue("profileID")
	switch r.Method {
	case http.MethodPut:
		if !requireJSONMutation(w, r, s, http.MethodPut) {
			return
		}
		var request PickitUpdateRequest
		if !s.decodeBody(w, r, &request) {
			return
		}
		value, err := backend.UpdatePickit(id, request)
		if err != nil {
			s.writePickitError(w, r, err)
			return
		}
		s.writeJSON(w, http.StatusOK, value)
	case http.MethodDelete:
		if !requireJSONMutation(w, r, s, http.MethodDelete) {
			return
		}
		var request PickitDeleteRequest
		if !s.decodeBody(w, r, &request) {
			return
		}
		if err := backend.DeletePickit(id, request); err != nil {
			s.writePickitError(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		requireMethod(w, r, http.MethodPut, s)
	}
}
func (s *Server) handlePickitDuplicate(w http.ResponseWriter, r *http.Request) {
	if !requireJSONMutation(w, r, s, http.MethodPost) {
		return
	}
	backend, ok := s.pickitBackend(w, r)
	if !ok {
		return
	}
	var request PickitDuplicateRequest
	if !s.decodeBody(w, r, &request) {
		return
	}
	value, err := backend.DuplicatePickit(r.PathValue("profileID"), request)
	if err != nil {
		s.writePickitError(w, r, err)
		return
	}
	s.writeJSON(w, http.StatusCreated, value)
}
func (s *Server) handlePickitAssignments(w http.ResponseWriter, r *http.Request) {
	backend, ok := s.pickitBackend(w, r)
	if !ok {
		return
	}
	if r.Method == http.MethodGet {
		value, err := backend.PickitAssignments()
		if err != nil {
			s.writePickitError(w, r, err)
			return
		}
		s.writeJSON(w, http.StatusOK, value)
		return
	}
	if !requireJSONMutation(w, r, s, http.MethodPut) {
		return
	}
	var request PickitAssignmentUpdateRequest
	if !s.decodeBody(w, r, &request) {
		return
	}
	value, err := backend.UpdatePickitAssignment(request)
	if err != nil {
		s.writePickitError(w, r, err)
		return
	}
	s.writeJSON(w, http.StatusOK, value)
}
func (s *Server) handlePickitImport(w http.ResponseWriter, r *http.Request) {
	if !requireJSONPost(w, r, s, false) {
		return
	}
	backend, ok := s.pickitBackend(w, r)
	if !ok {
		return
	}
	var request PickitImportRequest
	if !s.decodeBody(w, r, &request) {
		return
	}
	value, err := backend.ImportPickit(request)
	if err != nil {
		s.writePickitError(w, r, err)
		return
	}
	s.writeJSON(w, http.StatusOK, value)
}
func (s *Server) handlePickitExport(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet, s) {
		return
	}
	backend, ok := s.pickitBackend(w, r)
	if !ok {
		return
	}
	value, err := backend.ExportPickit(r.PathValue("profileID"))
	if err != nil {
		s.writePickitError(w, r, err)
		return
	}
	s.writeJSON(w, http.StatusOK, value)
}

func (s *Server) writePickitError(w http.ResponseWriter, r *http.Request, err error) {
	var commandErr *commandError
	if errors.As(err, &commandErr) {
		s.writeError(w, http.StatusConflict, commandErr.code, commandErr.message, requestIDFrom(r), commandErr.details)
		return
	}
	code, status := "pickit_invalid", http.StatusBadRequest
	switch {
	case strings.Contains(err.Error(), "revision_conflict"):
		code, status = "revision_conflict", http.StatusConflict
	case strings.Contains(err.Error(), "id_conflict"):
		code, status = "id_conflict", http.StatusConflict
	case strings.Contains(err.Error(), "assigned"):
		code, status = "profile_assigned", http.StatusConflict
	case errors.Is(err, fs.ErrNotExist), strings.Contains(err.Error(), "file does not exist"):
		code, status = "pickit_not_found", http.StatusNotFound
	}
	details := map[string]any{"path": "pickit", "error": err.Error()}
	s.writeError(w, status, code, err.Error(), requestIDFrom(r), details)
}

func (s *Server) handleRouteRecordingStart(w http.ResponseWriter, r *http.Request) {
	if !requireJSONPost(w, r, s, true) {
		return
	}
	backend, ok := s.routesBackend(w, r)
	if !ok {
		return
	}
	var request RouteRecordingStartRequest
	if !s.decodeBody(w, r, &request) {
		return
	}
	workflowRequest := RouteWorkflowRequest{ExpectedGeneration: request.ExpectedGeneration, Operation: "record", RunID: request.RunID}
	value, err := backend.StartRouteWorkflow(workflowRequest)
	if err != nil {
		s.writeError(w, http.StatusConflict, "route_workflow_conflict", err.Error(), requestIDFrom(r), nil)
		return
	}
	s.writeJSON(w, http.StatusAccepted, value)
}

func (s *Server) handleRouteRecordingFinish(w http.ResponseWriter, r *http.Request) {
	if !requireJSONPost(w, r, s, true) {
		return
	}
	backend, ok := s.routesBackend(w, r)
	if !ok {
		return
	}
	var request RouteWorkflowFinishRequest
	if !s.decodeBody(w, r, &request) {
		return
	}
	value, err := backend.FinishRouteWorkflow(r.PathValue("workflowID"), request)
	if err != nil {
		s.writeError(w, http.StatusConflict, "route_workflow_changed", err.Error(), requestIDFrom(r), nil)
		return
	}
	s.writeJSON(w, http.StatusAccepted, value)
}

func (s *Server) handleRouteCandidateTest(w http.ResponseWriter, r *http.Request) {
	if !requireJSONPost(w, r, s, true) {
		return
	}
	backend, ok := s.routesBackend(w, r)
	if !ok {
		return
	}
	var request RouteWorkflowFinishRequest
	if !s.decodeBody(w, r, &request) {
		return
	}
	value, err := backend.StartRouteWorkflow(RouteWorkflowRequest{ExpectedGeneration: request.ExpectedGeneration, Operation: "test", CandidateID: r.PathValue("candidateID")})
	if err != nil {
		s.writeError(w, http.StatusConflict, "route_workflow_conflict", err.Error(), requestIDFrom(r), nil)
		return
	}
	s.writeJSON(w, http.StatusAccepted, value)
}

func (s *Server) handleCandidatePublish(w http.ResponseWriter, r *http.Request) {
	if r.PathValue("stage") == "preview" {
		s.handleRouteMutationPreviewValue(w, r, RouteMutationPreviewRequest{Operation: "publish", CandidateID: r.PathValue("candidateID")})
		return
	}
	if r.PathValue("stage") == "confirm" {
		s.handleRouteMutationConfirm(w, r)
		return
	}
	s.handleUnsupportedAPI(w, r)
}

func (s *Server) handleNamedRouteMutation(w http.ResponseWriter, r *http.Request) {
	operation := r.PathValue("operation")
	if operation != "archive" && operation != "restore" && operation != "delete" {
		s.handleUnsupportedAPI(w, r)
		return
	}
	if r.PathValue("stage") == "preview" {
		s.handleRouteMutationPreviewValue(w, r, RouteMutationPreviewRequest{Operation: operation, RouteID: r.PathValue("routeID")})
		return
	}
	if r.PathValue("stage") == "confirm" {
		s.handleRouteMutationConfirm(w, r)
		return
	}
	s.handleUnsupportedAPI(w, r)
}

func (s *Server) handleRouteMutationPreviewValue(w http.ResponseWriter, r *http.Request, request RouteMutationPreviewRequest) {
	if !requireJSONPost(w, r, s, false) {
		return
	}
	backend, ok := s.routesBackend(w, r)
	if !ok {
		return
	}
	// Named preview endpoints deliberately accept an empty JSON object only.
	var body struct{}
	if !s.decodeBody(w, r, &body) {
		return
	}
	value, err := backend.PreviewRouteMutation(request)
	if err != nil {
		s.writeError(w, http.StatusConflict, "route_preview_stale", err.Error(), requestIDFrom(r), nil)
		return
	}
	s.writeJSON(w, http.StatusOK, value)
}

func (s *Server) routesBackend(w http.ResponseWriter, r *http.Request) (routeBackend, bool) {
	backend, ok := s.backend.(routeBackend)
	if !ok {
		s.writeError(w, http.StatusServiceUnavailable, "route_feature_unavailable", "Die Routenverwaltung ist noch nicht verfügbar.", requestIDFrom(r), nil)
	}
	return backend, ok
}

func (s *Server) handleRouteLibrary(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet, s) {
		return
	}
	backend, ok := s.routesBackend(w, r)
	if !ok {
		return
	}
	includeArchived := r.URL.Query().Get("view") == "archive"
	if raw := strings.TrimSpace(r.URL.Query().Get("include_archived")); raw != "" {
		parsed, err := strconv.ParseBool(raw)
		if err != nil {
			s.writeError(w, http.StatusBadRequest, "request_invalid", "include_archived ist ungültig.", requestIDFrom(r), nil)
			return
		}
		includeArchived = parsed
	}
	value, err := backend.RouteLibrary(r.URL.Query().Get("character"), includeArchived)
	if err != nil {
		s.writeError(w, http.StatusConflict, "route_catalog_unavailable", err.Error(), requestIDFrom(r), nil)
		return
	}
	s.writeJSON(w, http.StatusOK, value)
}
func (s *Server) handleRecordingOptions(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet, s) {
		return
	}
	backend, ok := s.routesBackend(w, r)
	if ok {
		s.writeJSON(w, http.StatusOK, backend.RecordingOptions())
	}
}
func (s *Server) handleRouteCandidates(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet, s) {
		return
	}
	backend, ok := s.routesBackend(w, r)
	if !ok {
		return
	}
	value, err := backend.RouteCandidates()
	if err != nil {
		s.writeError(w, http.StatusConflict, "route_candidates_unavailable", err.Error(), requestIDFrom(r), nil)
		return
	}
	s.writeJSON(w, http.StatusOK, value)
}
func (s *Server) handleSystemRouteStatus(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet, s) {
		return
	}
	backend, ok := s.routesBackend(w, r)
	if ok {
		s.writeJSON(w, http.StatusOK, backend.SystemRouteStatuses())
	}
}
func (s *Server) handleHotkeyHelp(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet, s) {
		return
	}
	backend, ok := s.routesBackend(w, r)
	if ok {
		s.writeJSON(w, http.StatusOK, backend.HotkeyHelp())
	}
}
func (s *Server) handleRouteWorkflow(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet, s) {
		return
	}
	backend, ok := s.routesBackend(w, r)
	if ok {
		s.writeJSON(w, http.StatusOK, backend.RouteWorkflow())
	}
}
func (s *Server) handleRouteMutationPreview(w http.ResponseWriter, r *http.Request) {
	if !requireJSONPost(w, r, s, false) {
		return
	}
	backend, ok := s.routesBackend(w, r)
	if !ok {
		return
	}
	var request RouteMutationPreviewRequest
	if !s.decodeBody(w, r, &request) {
		return
	}
	value, err := backend.PreviewRouteMutation(request)
	if err != nil {
		s.writeError(w, http.StatusConflict, "route_preview_stale", err.Error(), requestIDFrom(r), nil)
		return
	}
	s.writeJSON(w, http.StatusOK, value)
}
func (s *Server) handleRouteMutationConfirm(w http.ResponseWriter, r *http.Request) {
	if !requireJSONPost(w, r, s, true) {
		return
	}
	backend, ok := s.routesBackend(w, r)
	if !ok {
		return
	}
	var request RouteMutationConfirmRequest
	if !s.decodeBody(w, r, &request) {
		return
	}
	if err := backend.ConfirmRouteMutation(request); err != nil {
		s.writeError(w, http.StatusConflict, "route_confirmation_stale", err.Error(), requestIDFrom(r), nil)
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]string{"status": "completed"})
}
func (s *Server) handleRouteWorkflowStart(w http.ResponseWriter, r *http.Request) {
	if !requireJSONPost(w, r, s, true) {
		return
	}
	backend, ok := s.routesBackend(w, r)
	if !ok {
		return
	}
	var request RouteWorkflowRequest
	if !s.decodeBody(w, r, &request) {
		return
	}
	value, err := backend.StartRouteWorkflow(request)
	if err != nil {
		s.writeError(w, http.StatusConflict, "route_workflow_conflict", err.Error(), requestIDFrom(r), nil)
		return
	}
	s.writeJSON(w, http.StatusAccepted, value)
}

func (s *Server) handleControlBootstrap(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet, s) {
		return
	}
	// A foreign browser cannot send this non-simple header without a CORS
	// preflight, which the loopback server rejects before exposing the token.
	if r.Header.Get(bootstrapHeader) != "1" {
		s.writeError(w, http.StatusUnauthorized, "request_unauthorized", "Der lokale Control-Bootstrap wurde nicht bestätigt.", requestIDFrom(r), nil)
		return
	}
	s.writeJSON(w, http.StatusOK, struct {
		Token string `json:"control_token"`
	}{Token: s.token})
}

func (s *Server) security(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := newRequestID()
		w.Header().Set("X-Request-ID", requestID)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; connect-src 'self'; img-src 'self'; style-src 'self'; script-src 'self'")
		if r.Host != strings.TrimPrefix(s.baseURL, "http://") {
			s.writeError(w, http.StatusBadRequest, "request_invalid", "Ungültiger Host.", requestID, nil)
			return
		}
		if origin := r.Header.Get("Origin"); origin != "" && origin != s.baseURL {
			s.writeError(w, http.StatusForbidden, "origin_rejected", "Die Anfrage stammt nicht aus der lokalen Oberfläche.", requestID, nil)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), requestIDKey{}, requestID)))
	})
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet, s) {
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		s.writeError(w, http.StatusInternalServerError, "stream_unavailable", "Der Live-Stream ist nicht verfügbar.", requestIDFrom(r), nil)
		return
	}
	after := s.events.Sequence()
	if header := strings.TrimSpace(r.Header.Get("Last-Event-ID")); header != "" {
		parsed, err := strconv.ParseUint(header, 10, 64)
		if err != nil {
			s.writeError(w, http.StatusBadRequest, "request_invalid", "Last-Event-ID ist ungültig.", requestIDFrom(r), nil)
			return
		}
		if parsed > s.events.Sequence() {
			s.writeError(w, http.StatusBadRequest, "request_invalid", "Last-Event-ID liegt vor dem aktuellen Stream.", requestIDFrom(r), nil)
			return
		}
		after = parsed
	}
	replay, subscription := s.events.Subscribe(after)
	defer subscription.Close()

	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Connection", "keep-alive")
	status := s.backend.Status()
	status.SchemaVersion = schemaVersion
	if !writeSSE(w, "snapshot", after, status) {
		return
	}
	for _, event := range replay {
		if !writeSSE(w, event.Event, event.Sequence, event) {
			return
		}
	}
	flusher.Flush()

	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case event, open := <-subscription.Events:
			if !open || !writeSSE(w, event.Event, event.Sequence, event) {
				return
			}
			flusher.Flush()
		case <-heartbeat.C:
			if _, err := io.WriteString(w, ": heartbeat\n\n"); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func writeSSE(w io.Writer, event string, sequence uint64, value any) bool {
	payload, err := json.Marshal(value)
	if err != nil {
		return false
	}
	_, err = fmt.Fprintf(w, "id: %d\nevent: %s\ndata: %s\n\n", sequence, event, payload)
	return err == nil
}

func (s *Server) handleUnsupportedAPI(w http.ResponseWriter, r *http.Request) {
	s.writeError(w, http.StatusNotFound, "api_version_unsupported", "API-Version oder Endpunkt wird nicht unterstützt.", requestIDFrom(r), nil)
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet, s) {
		return
	}
	status := s.backend.Status()
	status.SchemaVersion = schemaVersion
	s.writeJSON(w, http.StatusOK, status)
}

func (s *Server) handleCatalog(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet, s) {
		return
	}
	catalog := s.backend.Catalog()
	catalog.SchemaVersion = schemaVersion
	s.writeJSON(w, http.StatusOK, catalog)
}

func (s *Server) handleSelectionPreview(w http.ResponseWriter, r *http.Request) {
	if !requireJSONPost(w, r, s, false) {
		return
	}
	var request SelectionPreviewRequest
	if !s.decodeBody(w, r, &request) {
		return
	}
	preview, err := s.backend.PreviewSelection(request)
	if err != nil {
		var commandErr *commandError
		if errors.As(err, &commandErr) {
			s.writeError(w, http.StatusConflict, commandErr.code, commandErr.message, requestIDFrom(r), commandErr.details)
			return
		}
		s.writeError(w, http.StatusConflict, "state_changed", "Der Auswahlkontext hat sich geändert.", requestIDFrom(r), nil)
		return
	}
	preview.SchemaVersion = schemaVersion
	s.writeJSON(w, http.StatusOK, preview)
}

func (s *Server) handleQueueValidation(w http.ResponseWriter, r *http.Request) {
	if !requireJSONPost(w, r, s, false) {
		return
	}
	var request QueueValidationRequest
	if !s.decodeBody(w, r, &request) {
		return
	}
	validation, err := s.backend.ValidateQueue(request)
	if err != nil {
		var commandErr *commandError
		if errors.As(err, &commandErr) {
			s.writeError(w, http.StatusConflict, commandErr.code, commandErr.message, requestIDFrom(r), commandErr.details)
			return
		}
		s.writeError(w, http.StatusConflict, "queue_entry_unavailable", "Die Farm-Queue konnte nicht sicher geprüft werden.", requestIDFrom(r), nil)
		return
	}
	validation.SchemaVersion = schemaVersion
	s.writeJSON(w, http.StatusOK, validation)
}

func (s *Server) handleCommand(w http.ResponseWriter, r *http.Request, command string) {
	if !requireJSONPost(w, r, s, true) {
		return
	}
	var request CommandRequest
	if !s.decodeBody(w, r, &request) {
		return
	}
	if strings.TrimSpace(request.CommandID) == "" {
		s.writeError(w, http.StatusBadRequest, "request_invalid", "command_id ist erforderlich.", requestIDFrom(r), nil)
		return
	}
	response, err := s.backend.Command(command, request)
	if err != nil {
		var commandErr *commandError
		if errors.As(err, &commandErr) {
			s.logger.Warn("API command rejected", "command", command, "code", commandErr.code, "request_id", requestIDFrom(r))
			s.writeError(w, http.StatusConflict, commandErr.code, commandErr.message, requestIDFrom(r), commandErr.details)
			return
		}
		s.writeError(w, http.StatusConflict, "state_changed", "Der Core-Zustand hat sich geändert.", requestIDFrom(r), nil)
		return
	}
	response.SchemaVersion = schemaVersion
	response.CommandID = request.CommandID
	s.writeJSON(w, http.StatusOK, response)
}

func requireMethod(w http.ResponseWriter, r *http.Request, method string, s *Server) bool {
	if r.Method == method {
		return true
	}
	w.Header().Set("Allow", method)
	s.writeError(w, http.StatusMethodNotAllowed, "request_invalid", "HTTP-Methode nicht erlaubt.", requestIDFrom(r), nil)
	return false
}

func requireJSONPost(w http.ResponseWriter, r *http.Request, s *Server, tokenRequired bool) bool {
	if !requireMethod(w, r, http.MethodPost, s) {
		return false
	}
	mediaType := strings.ToLower(strings.TrimSpace(strings.Split(r.Header.Get("Content-Type"), ";")[0]))
	if mediaType != "application/json" {
		s.writeError(w, http.StatusUnsupportedMediaType, "request_invalid", "Content-Type application/json ist erforderlich.", requestIDFrom(r), nil)
		return false
	}
	if tokenRequired && r.Header.Get(controlTokenHeader) != s.token {
		s.writeError(w, http.StatusUnauthorized, "request_unauthorized", "Control-Token fehlt oder ist ungültig.", requestIDFrom(r), nil)
		return false
	}
	return true
}

func requireJSONMutation(w http.ResponseWriter, r *http.Request, s *Server, method string) bool {
	if !requireMethod(w, r, method, s) {
		return false
	}
	mediaType := strings.ToLower(strings.TrimSpace(strings.Split(r.Header.Get("Content-Type"), ";")[0]))
	if mediaType != "application/json" {
		s.writeError(w, http.StatusUnsupportedMediaType, "request_invalid", "Content-Type application/json ist erforderlich.", requestIDFrom(r), nil)
		return false
	}
	if r.Header.Get(controlTokenHeader) != s.token {
		s.writeError(w, http.StatusUnauthorized, "request_unauthorized", "Control-Token fehlt oder ist ungültig.", requestIDFrom(r), nil)
		return false
	}
	return true
}

func (s *Server) decodeBody(w http.ResponseWriter, r *http.Request, target any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		code, status, message := "request_invalid", http.StatusBadRequest, "Ungültiger JSON-Request."
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			code, status, message = "payload_too_large", http.StatusRequestEntityTooLarge, "Der Request ist zu groß."
		}
		s.writeError(w, status, code, message, requestIDFrom(r), nil)
		return false
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		s.writeError(w, http.StatusBadRequest, "request_invalid", "Der Request enthält mehrere JSON-Werte.", requestIDFrom(r), nil)
		return false
	}
	return true
}

func (s *Server) writeError(w http.ResponseWriter, status int, code, message, requestID string, details map[string]any) {
	s.writeJSON(w, status, ErrorDTO{Code: code, Message: message, Details: details, RequestID: requestID})
}

func (s *Server) writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func randomToken(size int) (string, error) {
	buffer := make([]byte, size)
	if _, err := rand.Read(buffer); err != nil {
		return "", fmt.Errorf("generate API control token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}

func newRequestID() string {
	id, err := randomToken(12)
	if err != nil {
		return "request-unavailable"
	}
	return id
}

type requestIDKey struct{}

func requestIDFrom(r *http.Request) string {
	requestID, _ := r.Context().Value(requestIDKey{}).(string)
	return requestID
}

func spaHandler(assets fs.FS) http.Handler {
	files := http.FileServer(http.FS(assets))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}
		if _, err := fs.Stat(assets, path); err != nil {
			r.URL.Path = "/"
		}
		files.ServeHTTP(w, r)
	})
}
