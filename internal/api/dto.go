// Package api provides the versioned loopback HTTP transport. It owns DTOs,
// request validation and security, but no session, route or input decisions.
package api

import "encoding/json"

const schemaVersion = 1

// StatusDTO is the transport projection returned by GET /api/v1/status.
type StatusDTO struct {
	SchemaVersion int                `json:"schema_version"`
	CoreVersion   string             `json:"core_version"`
	State         string             `json:"state"`
	Generation    uint64             `json:"generation"`
	PendingIntent string             `json:"pending_intent,omitempty"`
	ActiveRunID   string             `json:"active_run_id,omitempty"`
	Step          string             `json:"step,omitempty"`
	D2R           D2RDTO             `json:"d2r"`
	Input         InputDTO           `json:"input"`
	World         WorldDTO           `json:"world"`
	Selection     SelectionStatusDTO `json:"selection"`
	Queue         QueueStatusDTO     `json:"queue"`
	LastResult    *SessionResultDTO  `json:"last_result,omitempty"`
	LastError     *ErrorDTO          `json:"last_error,omitempty"`
}

// SessionResultDTO projects the last terminal worker disposition and reason.
type SessionResultDTO struct {
	Disposition string `json:"disposition"`
	Reason      string `json:"reason,omitempty"`
}

// QueueStatusDTO projects the immutable active runtime queue and hard budgets.
type QueueStatusDTO struct {
	Entries             []string        `json:"entries"`
	DefaultEntries      []string        `json:"default_entries"`
	Index               int             `json:"index"`
	Cycle               int             `json:"cycle"`
	Retry               int             `json:"retry"`
	StartedRuns         int             `json:"started_runs"`
	ConsecutiveFailures int             `json:"consecutive_failures"`
	TotalRestarts       int             `json:"total_restarts"`
	Budgets             QueueBudgetsDTO `json:"budgets"`
}

// QueueBudgetsDTO exposes the YAML-authoritative queue limits.
type QueueBudgetsDTO struct {
	MaxRuns                int   `json:"max_runs"`
	MaxDurationMs          int64 `json:"max_duration_ms"`
	MaxConsecutiveFailures int   `json:"max_consecutive_failures"`
	MaxTotalRestarts       int   `json:"max_total_restarts"`
}

// SelectionStatusDTO projects only the Memory-confirmed active context.
type SelectionStatusDTO struct {
	Character  string `json:"character,omitempty"`
	Difficulty string `json:"difficulty,omitempty"`
}

// D2RDTO projects process and bound-window state without exposing handles.
type D2RDTO struct {
	State        string `json:"state"`
	PID          uint32 `json:"pid,omitempty"`
	WindowBound  bool   `json:"window_bound"`
	ClientWidth  int    `json:"client_width,omitempty"`
	ClientHeight int    `json:"client_height,omitempty"`
}

// InputDTO projects the three independent input safety gates.
type InputDTO struct {
	Enabled bool `json:"enabled"`
	Paused  bool `json:"paused"`
	Stopped bool `json:"stopped"`
}

// WorldDTO projects the latest read-only semantic game state.
type WorldDTO struct {
	Valid    bool   `json:"valid"`
	Phase    string `json:"phase"`
	AreaID   uint32 `json:"area_id,omitempty"`
	AreaName string `json:"area_name,omitempty"`
}

// CatalogDTO is the immutable catalog projection returned by GET /api/v1/catalog.
type CatalogDTO struct {
	SchemaVersion     int                      `json:"schema_version"`
	Revision          uint64                   `json:"revision"`
	DefaultDifficulty string                   `json:"default_difficulty"`
	Characters        []CharacterCatalogEntry  `json:"characters"`
	Difficulties      []DifficultyCatalogEntry `json:"difficulties"`
	Profiles          []ProfileCatalogEntry    `json:"profiles"`
	Runs              []RunCatalogEntry        `json:"runs"`
}

// CharacterCatalogEntry exposes a save filename identity and its fail-closed availability.
type CharacterCatalogEntry struct {
	Name          string   `json:"name"`
	Slug          string   `json:"slug"`
	ExpectedClass string   `json:"expected_class,omitempty"`
	Selectable    bool     `json:"selectable"`
	Reasons       []string `json:"reasons,omitempty"`
}

// DifficultyCatalogEntry describes one supported offline difficulty.
type DifficultyCatalogEntry struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
}

// ProfileCatalogEntry describes one configured combat profile without YAML internals.
type ProfileCatalogEntry struct {
	ID             string `json:"id"`
	CharacterClass string `json:"character_class"`
}

// RunCatalogEntry describes one stable run and its current availability.
type RunCatalogEntry struct {
	RunID       string   `json:"run_id"`
	DisplayName string   `json:"display_name"`
	Status      string   `json:"status"`
	Reasons     []string `json:"reasons,omitempty"`
}

// CommandRequest carries idempotency and optimistic concurrency metadata.
type CommandRequest struct {
	CommandID          string          `json:"command_id"`
	ExpectedGeneration uint64          `json:"expected_generation"`
	Payload            json.RawMessage `json:"payload,omitempty"`
}

// CommandResponse acknowledges one command using the resulting generation.
type CommandResponse struct {
	SchemaVersion int    `json:"schema_version"`
	CommandID     string `json:"command_id"`
	Generation    uint64 `json:"generation"`
	State         string `json:"state"`
}

// SelectionPreviewRequest requests a side-effect-free lifecycle comparison.
type SelectionPreviewRequest struct {
	Character       string `json:"character"`
	Difficulty      string `json:"difficulty"`
	CatalogRevision uint64 `json:"catalog_revision"`
}

// SelectionPreviewDTO binds confirmation to exact catalog and manifest revisions.
type SelectionPreviewDTO struct {
	SchemaVersion        int      `json:"schema_version"`
	Character            string   `json:"character"`
	OldDifficulty        string   `json:"old_difficulty,omitempty"`
	NewDifficulty        string   `json:"new_difficulty"`
	AffectedRoutes       []string `json:"affected_routes"`
	InvalidationReason   string   `json:"invalidation_reason,omitempty"`
	RequiresConfirmation bool     `json:"requires_confirmation"`
	ConfirmationToken    string   `json:"confirmation_token"`
	CatalogRevision      uint64   `json:"catalog_revision"`
	LifecycleRevision    uint64   `json:"lifecycle_revision"`
}

// QueueValidationRequest requests a side-effect-free full-queue preflight.
type QueueValidationRequest struct {
	Entries         []string `json:"entries"`
	Character       string   `json:"character"`
	Difficulty      string   `json:"difficulty"`
	CatalogRevision uint64   `json:"catalog_revision"`
}

// QueueValidationDTO is the immutable plan that a later start command may consume.
type QueueValidationDTO struct {
	SchemaVersion   int             `json:"schema_version"`
	Entries         []string        `json:"entries"`
	Character       string          `json:"character"`
	Difficulty      string          `json:"difficulty"`
	CatalogRevision uint64          `json:"catalog_revision"`
	Budgets         QueueBudgetsDTO `json:"budgets"`
}

// SessionStartPayload binds start to the exact queue preflight context.
type SessionStartPayload struct {
	Entries         []string `json:"entries"`
	Character       string   `json:"character"`
	Difficulty      string   `json:"difficulty"`
	CatalogRevision uint64   `json:"catalog_revision"`
}

// ErrorDTO is the stable API error envelope with a German operator message.
type ErrorDTO struct {
	Code      string         `json:"code"`
	Message   string         `json:"message"`
	Details   map[string]any `json:"details,omitempty"`
	RequestID string         `json:"request_id"`
}

// Backend supplies transport-neutral queries and commands to the HTTP layer.
type Backend interface {
	Status() StatusDTO
	Catalog() CatalogDTO
	PreviewSelection(SelectionPreviewRequest) (SelectionPreviewDTO, error)
	ValidateQueue(QueueValidationRequest) (QueueValidationDTO, error)
	Command(name string, request CommandRequest) (CommandResponse, error)
}

type commandError struct {
	code    string
	message string
}

func (e *commandError) Error() string { return e.code }
