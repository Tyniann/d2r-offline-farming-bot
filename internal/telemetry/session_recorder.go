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
	mu        sync.Mutex
	file      *os.File
	writer    flushWriter
	path      string
	sessionID string
	closed    bool
}

// NewSessionRecorder creates a unique lifecycle file before session input is possible.
func NewSessionRecorder(directory string) (*SessionRecorder, error) {
	if directory == "" {
		return nil, fmt.Errorf("session telemetry directory is required")
	}
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return nil, fmt.Errorf("create session telemetry directory: %w", err)
	}
	now := time.Now().UTC()
	id := fmt.Sprintf("session-%s-%s", now.Format("20060102T150405.000000000Z"), randomSuffix())
	path := filepath.Join(directory, id+".jsonl")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("create session telemetry %q: %w", path, err)
	}
	return &SessionRecorder{file: file, writer: bufio.NewWriter(file), path: path, sessionID: id}, nil
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
	event.SchemaVersion = 2
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
	return nil
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
