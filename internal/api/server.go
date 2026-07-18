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
	mux.HandleFunc("/api/v1/events", s.handleEvents)
	mux.HandleFunc("/api/v1/control/bootstrap", s.handleControlBootstrap)
	for path, command := range commandPaths {
		commandName := command
		mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) { s.handleCommand(w, r, commandName) })
	}
	mux.HandleFunc("/api/v1/selection/preview", s.handleSelectionPreview)
	mux.HandleFunc("/api/v1/queue/validate", s.handleQueueValidation)
	mux.HandleFunc("/api/", s.handleUnsupportedAPI)
	mux.Handle("/", spaHandler(s.assets))
	return s.security(mux)
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
