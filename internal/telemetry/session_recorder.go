package telemetry

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// SessionRecorder owns one synchronously flushed JSONL file for correlated
// session, game, and run lifecycle events.
type SessionRecorder struct {
	mu              sync.Mutex
	file            *os.File
	writer          flushWriter
	path            string
	sessionID       string
	context         SessionRecorderContext
	terminals       map[string]EventName
	runs            map[string]sessionEventContext
	sessionTerminal EventName
	closed          bool
}

type sessionEventContext struct {
	gameID string
	run    string
}

// SessionRecorderContext bindet den unveränderlichen Kontext einer Schema-3-Session.
type SessionRecorderContext struct {
	SessionID   string
	Mode        HistoryMode
	Character   string
	Difficulty  string
	GameVersion string
}

// NewSessionRecorder creates a unique lifecycle file before session input is possible.
func NewSessionRecorder(directory string) (*SessionRecorder, error) {
	return NewSessionRecorderWithContext(directory, SessionRecorderContext{Mode: HistoryModeDiagnostic})
}

// NewSessionRecorderWithContext erstellt den Schema-3-Lifecycle-Stream vor Session-Input.
func NewSessionRecorderWithContext(directory string, context SessionRecorderContext) (*SessionRecorder, error) {
	if directory == "" {
		return nil, fmt.Errorf("session telemetry directory is required")
	}
	if context.Mode != HistoryModeProductiveFarming && context.Mode != HistoryModeDiagnostic {
		return nil, fmt.Errorf("%s: unsupported session mode %q", HistoryReasonContextMissing, context.Mode)
	}
	if context.Mode == HistoryModeProductiveFarming && (context.Character == "" || context.Difficulty == "" || context.GameVersion == "") {
		return nil, fmt.Errorf("%s: productive session context is incomplete", HistoryReasonContextMissing)
	}
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return nil, fmt.Errorf("create session telemetry directory: %w", err)
	}
	now := time.Now().UTC()
	id := context.SessionID
	if id == "" {
		id = fmt.Sprintf("session-%s-%s", now.Format("20060102t150405999999999z"), randomSuffix())
	}
	if safePart(id) != id {
		return nil, fmt.Errorf("%s: invalid session_id %q", HistoryReasonContextMissing, id)
	}
	context.SessionID = id
	path := filepath.Join(directory, id+".jsonl")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("create session telemetry %q: %w", path, err)
	}
	return &SessionRecorder{file: file, writer: bufio.NewWriter(file), path: path, sessionID: id, context: context, terminals: make(map[string]EventName), runs: make(map[string]sessionEventContext)}, nil
}

// Emit appends and flushes one lifecycle event with the recorder's session ID.
func (r *SessionRecorder) Emit(event Event) error {
	if r == nil {
		return fmt.Errorf("session telemetry recorder is nil")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed || r.writer == nil {
		return fmt.Errorf("session telemetry recorder is closed")
	}
	if event.SessionID != "" && event.SessionID != r.sessionID {
		return fmt.Errorf("%s: event session_id %q does not match recorder %q", HistoryReasonContextMissing, event.SessionID, r.sessionID)
	}
	if event.Mode != "" && event.Mode != r.context.Mode {
		return fmt.Errorf("%s: event mode %q does not match recorder %q", HistoryReasonContextMissing, event.Mode, r.context.Mode)
	}
	if event.Event == RunStarted || isRunTerminal(event.Event) {
		if event.RunID == "" || event.GameID == "" || event.Run == "" {
			return fmt.Errorf("%s: run lifecycle context is incomplete", HistoryReasonContextMissing)
		}
	}
	if event.RunID != "" {
		context := sessionEventContext{gameID: event.GameID, run: event.Run}
		if previous, ok := r.runs[event.RunID]; ok && previous != context {
			return fmt.Errorf("%s: run %q changed context", HistoryReasonRunIDMismatch, event.RunID)
		}
		r.runs[event.RunID] = context
	}
	if isSessionTerminal(event.Event) && r.sessionTerminal != "" {
		return fmt.Errorf("%s: session already ended with %q", HistoryReasonTerminalDuplicate, r.sessionTerminal)
	}
	if isRunTerminal(event.Event) {
		if previous, duplicate := r.terminals[event.RunID]; duplicate {
			return fmt.Errorf("%s: run %q already ended with %q", HistoryReasonTerminalDuplicate, event.RunID, previous)
		}
	}
	event.SchemaVersion = HistorySchemaVersion
	event.Stream = HistoryStreamSession
	event.Mode = r.context.Mode
	event.Character = r.context.Character
	event.Difficulty = r.context.Difficulty
	event.GameVersion = r.context.GameVersion
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	} else {
		event.Timestamp = event.Timestamp.UTC()
	}
	event.SessionID = r.sessionID
	line, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal session telemetry event %q: %w", event.Event, err)
	}
	line = append(line, '\n')
	if _, err := r.writer.Write(line); err != nil {
		return fmt.Errorf("write session telemetry event %q: %w", event.Event, err)
	}
	if err := r.writer.Flush(); err != nil {
		return fmt.Errorf("flush session telemetry event %q: %w", event.Event, err)
	}
	if isRunTerminal(event.Event) {
		r.terminals[event.RunID] = event.Event
	}
	if isSessionTerminal(event.Event) {
		r.sessionTerminal = event.Event
	}
	return nil
}

func isRunTerminal(event EventName) bool {
	return event == RunCompleted || event == RunFailed || event == RunAborted
}

func isSessionTerminal(event EventName) bool {
	return event == SessionCompleted || event == SessionStopped || event == SessionFailed
}

// SessionID returns the stable ID written into every lifecycle event.
func (r *SessionRecorder) SessionID() string {
	if r == nil {
		return ""
	}
	return r.sessionID
}

// Path returns the lifecycle JSONL file path.
func (r *SessionRecorder) Path() string {
	if r == nil {
		return ""
	}
	return r.path
}

// Close flushes and closes the lifecycle file. It is idempotent.
func (r *SessionRecorder) Close() error {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return nil
	}
	r.closed = true
	if err := r.writer.Flush(); err != nil {
		return fmt.Errorf("flush session telemetry: %w", err)
	}
	if err := r.file.Close(); err != nil {
		return fmt.Errorf("close session telemetry: %w", err)
	}
	return nil
}
