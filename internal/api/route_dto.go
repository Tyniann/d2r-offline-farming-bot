package api

// RouteLibraryDTO is the path-free Farming library projection.
type RouteLibraryDTO struct {
	SchemaVersion int             `json:"schema_version"`
	Revision      uint64          `json:"revision"`
	Character     string          `json:"character"`
	Routes        []RouteEntryDTO `json:"routes"`
}

// RouteEntryDTO describes one Farming route without exposing local files.
type RouteEntryDTO struct {
	RouteID          string `json:"route_id"`
	DisplayName      string `json:"display_name"`
	RunID            string `json:"run_id"`
	Character        string `json:"character"`
	Difficulty       string `json:"difficulty"`
	LifecycleStatus  string `json:"lifecycle_status"`
	ManagementStatus string `json:"management_status"`
	Assigned         bool   `json:"assigned"`
	Reason           string `json:"reason,omitempty"`
}

// RecordingOptionDTO describes one registered run's guided recording contract.
type RecordingOptionDTO struct {
	RunID                    string   `json:"run_id"`
	DisplayName              string   `json:"display_name"`
	InstructionsDE           string   `json:"instructions_de"`
	StartWaypoint            string   `json:"start_waypoint"`
	AllowedStartAreaID       uint32   `json:"allowed_start_area_id"`
	AllowedRouteAreaIDs      []uint32 `json:"allowed_route_area_ids"`
	TerminalAreaID           uint32   `json:"terminal_area_id"`
	TerminalMaxDistanceTiles float64  `json:"terminal_max_distance_tiles"`
	Available                bool     `json:"available"`
	Reason                   string   `json:"reason,omitempty"`
}

// RouteWorkflowDTO projects the exclusive recording/test workflow.
type RouteWorkflowDTO struct {
	WorkflowID string  `json:"workflow_id"`
	Generation uint64  `json:"generation"`
	State      string  `json:"state"`
	RunID      string  `json:"run_id"`
	Character  string  `json:"character"`
	Act        string  `json:"act,omitempty"`
	AreaID     uint32  `json:"area_id,omitempty"`
	Segment    int     `json:"segment,omitempty"`
	Progress   float64 `json:"progress,omitempty"`
	Reason     string  `json:"reason,omitempty"`
}

// RouteCandidateDTO projects immutable candidate identity and test evidence.
type RouteCandidateDTO struct {
	CandidateID          string  `json:"candidate_id"`
	RunID                string  `json:"run_id"`
	Character            string  `json:"character"`
	Difficulty           string  `json:"difficulty"`
	State                string  `json:"state"`
	MeasuredBossDistance float64 `json:"measured_boss_distance"`
	RouteSHA256          string  `json:"route_sha256"`
	Reason               string  `json:"reason,omitempty"`
}

// RouteMutationPreviewDTO binds a management confirmation to immutable revisions.
type RouteMutationPreviewDTO struct {
	Operation          string `json:"operation"`
	RouteID            string `json:"route_id"`
	CandidateID        string `json:"candidate_id,omitempty"`
	ReplacedRouteID    string `json:"replaced_route_id,omitempty"`
	CatalogRevision    uint64 `json:"catalog_revision"`
	LifecycleRevision  uint64 `json:"lifecycle_revision"`
	AssignmentRevision uint64 `json:"assignment_revision"`
	ConfirmationToken  string `json:"confirmation_token"`
}

// SystemRouteStatusDTO exposes readiness only, never system route files.
type SystemRouteStatusDTO struct {
	Act    string `json:"act"`
	Ready  bool   `json:"ready"`
	Reason string `json:"reason,omitempty"`
}

// RouteEventDTO is the stable SSE payload for recording, testing and mutation events.
type RouteEventDTO struct {
	SchemaVersion int     `json:"schema_version"`
	Sequence      uint64  `json:"sequence"`
	WorkflowID    string  `json:"workflow_id,omitempty"`
	State         string  `json:"state"`
	RunID         string  `json:"run_id,omitempty"`
	Act           string  `json:"act,omitempty"`
	AreaID        uint32  `json:"area_id,omitempty"`
	Segment       int     `json:"segment,omitempty"`
	Progress      float64 `json:"progress,omitempty"`
	Reason        string  `json:"reason,omitempty"`
}

// HotkeyHelpDTO exposes effective Core hotkeys and their operator semantics.
type HotkeyHelpDTO struct {
	RecordingFinish string `json:"recording_finish"`
	StopAfterRun    string `json:"stop_after_run"`
	EmergencyStop   string `json:"emergency_stop"`
	Pause           string `json:"pause"`
}

// RouteMutationPreviewRequest requests a side-effect-free management preview.
type RouteMutationPreviewRequest struct {
	Operation   string `json:"operation"`
	RouteID     string `json:"route_id,omitempty"`
	CandidateID string `json:"candidate_id,omitempty"`
}

// RouteMutationConfirmRequest consumes one revision-bound preview capability.
type RouteMutationConfirmRequest struct {
	ConfirmationToken string `json:"confirmation_token"`
	ConfirmRouteID    string `json:"confirm_route_id,omitempty"`
}

// RouteWorkflowRequest starts one Core-owned recording or candidate-test workflow.
type RouteWorkflowRequest struct {
	ExpectedGeneration uint64 `json:"expected_generation"`
	Operation          string `json:"operation"`
	RunID              string `json:"run_id,omitempty"`
	CandidateID        string `json:"candidate_id,omitempty"`
	Act                string `json:"act,omitempty"`
}

// RouteRecordingStartRequest starts one registered guided recording.
type RouteRecordingStartRequest struct {
	ExpectedGeneration uint64 `json:"expected_generation"`
	RunID              string `json:"run_id"`
}

// RouteWorkflowFinishRequest submits an idempotent finish intent for one recording.
type RouteWorkflowFinishRequest struct {
	ExpectedGeneration uint64 `json:"expected_generation"`
}
