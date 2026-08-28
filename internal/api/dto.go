// Package api provides the versioned loopback HTTP transport. It owns DTOs,
// request validation and security, but no session, route or input decisions.
package api

import "encoding/json"

const schemaVersion = 1

// StatusDTO is the transport projection returned by GET /api/v1/status.
type StatusDTO struct {
	SchemaVersion  int                `json:"schema_version"`
	CoreVersion    string             `json:"core_version"`
	AppVersion     string             `json:"app_version"`
	State          string             `json:"state"`
	Generation     uint64             `json:"generation"`
	PendingIntent  string             `json:"pending_intent,omitempty"`
	ActiveRunID    string             `json:"active_run_id,omitempty"`
	RunInstanceID  string             `json:"run_id,omitempty"`
	GameID         string             `json:"game_id,omitempty"`
	LifecyclePhase string             `json:"lifecycle_phase"`
	RecoveryStep   string             `json:"recovery_step,omitempty"`
	RunProgress    *RunProgressDTO    `json:"run_progress,omitempty"`
	D2R            D2RDTO             `json:"d2r"`
	Compatibility  CompatibilityDTO   `json:"compatibility"`
	Input          InputDTO           `json:"input"`
	World          WorldDTO           `json:"world"`
	Selection      SelectionStatusDTO `json:"selection"`
	Queue          QueueStatusDTO     `json:"queue"`
	LastResult     *SessionResultDTO  `json:"last_result,omitempty"`
	LastError      *ProblemDTO        `json:"last_error,omitempty"`
}

// RunProgressDTO transports one stable, user-facing stage of an active run.
type RunProgressDTO struct {
	StageCode string         `json:"stage_code"`
	Params    map[string]any `json:"params,omitempty"`
	Current   int            `json:"current"`
	Total     int            `json:"total"`
}

// CompatibilityDTO projects the path-free authoritative D2R version gate.
type CompatibilityDTO struct {
	State             string `json:"state"`
	Reason            string `json:"reason,omitempty"`
	SupportedVersion  string `json:"supported_version"`
	ExpectedVersion   string `json:"expected_version"`
	OffsetVersion     string `json:"offset_version"`
	ActualVersion     string `json:"actual_version,omitempty"`
	PrivilegeMismatch bool   `json:"privilege_mismatch"`
}

// SessionResultDTO projects the last terminal worker disposition and reason.
type SessionResultDTO struct {
	Disposition    string `json:"disposition"`
	Reason         string `json:"reason,omitempty"`
	OriginalReason string `json:"original_reason,omitempty"`
	RecoveryReason string `json:"recovery_reason,omitempty"`
	SessionID      string `json:"session_id,omitempty"`
	DurationMs     int64  `json:"duration_ms,omitempty"`
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

// RunAvailabilitiesDTO is the character-bound run catalog from GET /api/v1/runs.
type RunAvailabilitiesDTO struct {
	SchemaVersion int               `json:"schema_version"`
	Character     string            `json:"character"`
	Difficulty    string            `json:"difficulty"`
	Runs          []RunCatalogEntry `json:"runs"`
}

// CharacterCatalogEntry exposes a save filename identity and its fail-closed availability.
type CharacterCatalogEntry struct {
	Name             string   `json:"name"`
	Slug             string   `json:"slug"`
	ExpectedClass    string   `json:"expected_class,omitempty"`
	Selectable       bool     `json:"selectable"`
	Reasons          []string `json:"reasons,omitempty"`
	FarmReady        bool     `json:"farm_ready"`
	FarmReadyReasons []string `json:"farm_ready_reasons,omitempty"`
}

// DifficultyCatalogEntry describes one supported offline difficulty.
type DifficultyCatalogEntry struct {
	ID string `json:"id"`
}

// ProfileCatalogEntry describes one configured combat profile without YAML internals.
type ProfileCatalogEntry struct {
	ID             string `json:"id"`
	CharacterClass string `json:"character_class"`
}

// RunCatalogEntry describes one stable run and its current availability.
type RunCatalogEntry struct {
	RunID       string               `json:"run_id"`
	Status      string               `json:"status"`
	Reasons     []string             `json:"reasons,omitempty"`
	RouteCombat RouteCombatConfigDTO `json:"route_combat"`
}

// RouteCombatConfigDTO projects the effective, Core-validated route threat settings read-only.
type RouteCombatConfigDTO struct {
	Enabled                    bool    `json:"enabled"`
	ImmediateRadiusTiles       float64 `json:"immediate_radius_tiles"`
	CorridorWidthTiles         float64 `json:"corridor_width_tiles"`
	LandingRadiusTiles         float64 `json:"landing_radius_tiles"`
	AttackDistanceTiles        float64 `json:"attack_distance_tiles"`
	NoProgressTimeoutMs        int     `json:"no_progress_timeout_ms"`
	TeleportManaReservePercent int     `json:"teleport_mana_reserve_percent"`
	ResumeManaPercent          int     `json:"resume_mana_percent"`
	EmergencyManaPercent       int     `json:"emergency_mana_percent"`
	ManaRecoveryTimeoutMs      int     `json:"mana_recovery_timeout_ms"`
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
	Warnings        []string        `json:"warnings,omitempty"`
}

// SessionStartPayload binds start to the exact queue preflight context.
type SessionStartPayload struct {
	Entries         []string `json:"entries"`
	Character       string   `json:"character"`
	Difficulty      string   `json:"difficulty"`
	CatalogRevision uint64   `json:"catalog_revision"`
}

// ProblemDTO identifies a user-visible problem without carrying localized prose.
type ProblemDTO struct {
	Code   string         `json:"code"`
	Params map[string]any `json:"params,omitempty"`
}

// ErrorDTO is the stable HTTP error envelope with request correlation.
type ErrorDTO struct {
	Code      string         `json:"code"`
	Params    map[string]any `json:"params,omitempty"`
	RequestID string         `json:"request_id"`
}

// Backend supplies transport-neutral queries and commands to the HTTP layer.
type Backend interface {
	Status() StatusDTO
	Catalog() CatalogDTO
	RunAvailabilities(character, difficulty string) (RunAvailabilitiesDTO, error)
	PreviewSelection(SelectionPreviewRequest) (SelectionPreviewDTO, error)
	ValidateQueue(QueueValidationRequest) (QueueValidationDTO, error)
	Command(name string, request CommandRequest) (CommandResponse, error)
}

type commandError struct {
	code   string
	params map[string]any
	cause  error
}

func (e *commandError) Error() string { return e.code }

func (e *commandError) Unwrap() error { return e.cause }
