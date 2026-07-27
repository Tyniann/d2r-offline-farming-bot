package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// DesktopHandshake ist der einmalige tokenhaltige Core-Bootstrap über die private Parent-/Child-Pipe.
type DesktopHandshake struct {
	SchemaVersion int    `json:"schema_version"`
	CorePID       int    `json:"core_pid"`
	Generation    uint64 `json:"generation"`
	BaseURL       string `json:"base_url"`
	BootstrapURL  string `json:"bootstrap_url"`
}

// WriteDesktopHandshake schreibt exakt einen JSON-Vertrag und schließt die Pipe danach.
func WriteDesktopHandshake(ctx context.Context, pipeName string, handshake DesktopHandshake) error {
	if ctx == nil || strings.TrimSpace(pipeName) == "" {
		return fmt.Errorf("desktop handshake context and pipe are required")
	}
	if handshake.SchemaVersion != 1 || handshake.CorePID <= 0 || handshake.Generation == 0 || handshake.BaseURL == "" || handshake.BootstrapURL == "" {
		return fmt.Errorf("desktop handshake is incomplete")
	}
	pipe, err := openDesktopHandshakePipe(ctx, pipeName)
	if err != nil {
		return err
	}
	defer pipe.Close()
	encoder := json.NewEncoder(pipe)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(handshake); err != nil {
		return fmt.Errorf("write desktop handshake: %w", err)
	}
	if syncer, ok := pipe.(interface{ Sync() error }); ok {
		if err := syncer.Sync(); err != nil {
			return fmt.Errorf("flush desktop handshake: %w", err)
		}
	}
	return nil
}

type desktopHandshakePipe interface {
	io.Writer
	io.Closer
}
