package api

import (
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/telemetry"
)

// HistoryFilterDTO echoes the canonical server-side filter.
type HistoryFilterDTO struct {
	FromUTC        *time.Time                 `json:"from_utc,omitempty"`
	ToUTC          *time.Time                 `json:"to_utc,omitempty"`
	Timezone       string                     `json:"timezone"`
	Runs           []string                   `json:"runs"`
	Characters     []string                   `json:"characters"`
	Difficulties   []string                   `json:"difficulties"`
	Outcomes       []telemetry.HistoryOutcome `json:"outcomes"`
	Reasons        []string                   `json:"reasons"`
	PickitProfiles []string                   `json:"pickit_profiles"`
	Sort           telemetry.HistorySort      `json:"sort,omitempty"`
}

// HistoryDailyBucketDTO is a display-ready local calendar-day projection.
type HistoryDailyBucketDTO struct {
	Date             string    `json:"date"`
	StartUTC         time.Time `json:"start_utc"`
	EndUTC           time.Time `json:"end_utc"`
	TerminalRuns     int       `json:"terminal_runs"`
	Successful       int       `json:"successful"`
	SuccessRate      *float64  `json:"success_rate,omitempty"`
	ActiveDurationMs int64     `json:"active_duration_ms"`
	ActiveHours      float64   `json:"active_hours"`
	KeepReturn       int       `json:"keep_return"`
	KeepPerHour      *float64  `json:"keep_per_hour,omitempty"`
}

// HistoryDiagnosticDTO exposes only a basename and stable error explanation.
type HistoryDiagnosticDTO struct {
	File string `json:"file"`
	Code string `json:"code"`
}

// HistoryMetaDTO binds every response to one index generation and filter.
type HistoryMetaDTO struct {
	SchemaVersion   int                    `json:"schema_version"`
	GeneratedAt     time.Time              `json:"generated_at"`
	Timezone        string                 `json:"timezone"`
	IndexGeneration uint64                 `json:"index_generation"`
	Filter          HistoryFilterDTO       `json:"filter"`
	Diagnostics     []HistoryDiagnosticDTO `json:"diagnostics"`
	IgnoredFiles    int                    `json:"ignored_files"`
}

// HistoryDurationDTO is the transport form of terminal duration statistics.
type HistoryDurationDTO struct {
	Count     int     `json:"count"`
	TotalMs   int64   `json:"total_ms"`
	AverageMs float64 `json:"average_ms"`
	MedianMs  float64 `json:"median_ms"`
	MinimumMs int64   `json:"minimum_ms"`
	MaximumMs int64   `json:"maximum_ms"`
}

// HistoryStagesDTO is the transport form of disjoint stage time.
type HistoryStagesDTO struct {
	TravelMs     int64 `json:"travel_ms"`
	CombatMs     int64 `json:"combat_ms"`
	LootMs       int64 `json:"loot_ms"`
	ReturnTownMs int64 `json:"return_town_ms"`
	OtherMs      int64 `json:"other_ms"`
}

// HistoryFunnelDTO is the transport form of distinct item-unit outcomes.
type HistoryFunnelDTO struct {
	Seen           int `json:"seen"`
	Matched        int `json:"matched"`
	PickedUp       int `json:"picked_up"`
	Stashed        int `json:"stashed"`
	Sold           int `json:"sold"`
	KeepReturn     int `json:"keep_return"`
	PickupLost     int `json:"pickup_lost"`
	PostPickupLost int `json:"post_pickup_lost"`
}

// HistoryFailureDTO is one stable step/reason failure group.
type HistoryFailureDTO struct {
	Step           string `json:"step"`
	Reason         string `json:"reason"`
	Count          int    `json:"count"`
	LostDurationMs int64  `json:"lost_duration_ms"`
}

// HistorySummaryDTO contains the filtered high-level farming answer.
type HistorySummaryDTO struct {
	Runs         int                `json:"runs"`
	TerminalRuns int                `json:"terminal_runs"`
	Successful   int                `json:"successful"`
	Failed       int                `json:"failed"`
	Aborted      int                `json:"aborted"`
	Incomplete   int                `json:"incomplete"`
	Running      int                `json:"running"`
	SuccessRate  *float64           `json:"success_rate,omitempty"`
	BossKills    int                `json:"boss_kills"`
	Durations    HistoryDurationDTO `json:"durations"`
	Stages       HistoryStagesDTO   `json:"stages"`
	Funnel       HistoryFunnelDTO   `json:"funnel"`
	KeepPerRun   *float64           `json:"keep_per_run,omitempty"`
	KeepPerKill  *float64           `json:"keep_per_kill,omitempty"`
	KeepPerHour  *float64           `json:"keep_per_hour,omitempty"`
	TopFailure   *HistoryFailureDTO `json:"top_failure,omitempty"`
}

// HistoryComparisonDTO is one server-sorted character/difficulty/definition/route row.
type HistoryComparisonDTO struct {
	ID           string             `json:"id"`
	Character    string             `json:"character"`
	Difficulty   string             `json:"difficulty"`
	DefinitionID string             `json:"definition_id"`
	Run          string             `json:"run"`
	RouteID      string             `json:"route_id"`
	TerminalRuns int                `json:"terminal_runs"`
	Successful   int                `json:"successful"`
	Failed       int                `json:"failed"`
	Aborted      int                `json:"aborted"`
	SuccessRate  *float64           `json:"success_rate,omitempty"`
	BossKills    int                `json:"boss_kills"`
	LowSample    bool               `json:"low_sample"`
	Durations    HistoryDurationDTO `json:"durations"`
	Stages       HistoryStagesDTO   `json:"stages"`
	Funnel       HistoryFunnelDTO   `json:"funnel"`
	KeepPerRun   *float64           `json:"keep_per_run,omitempty"`
	KeepPerKill  *float64           `json:"keep_per_kill,omitempty"`
	KeepPerHour  *float64           `json:"keep_per_hour,omitempty"`
	TopFailure   *HistoryFailureDTO `json:"top_failure,omitempty"`
}

// HistoryItemDTO is one stable item-key aggregate.
type HistoryItemDTO struct {
	ItemKey        string   `json:"item_key"`
	ItemName       string   `json:"item_name"`
	BaseCode       string   `json:"base_code,omitempty"`
	Quality        string   `json:"quality,omitempty"`
	Seen           int      `json:"seen"`
	Matched        int      `json:"matched"`
	PickedUp       int      `json:"picked_up"`
	Stashed        int      `json:"stashed"`
	Sold           int      `json:"sold"`
	PickupLost     int      `json:"pickup_lost"`
	PostPickupLost int      `json:"post_pickup_lost"`
	YieldPerRun    *float64 `json:"yield_per_run,omitempty"`
	YieldPerKill   *float64 `json:"yield_per_kill,omitempty"`
	YieldPerHour   *float64 `json:"yield_per_hour,omitempty"`
}

// HistoryRunDTO is one paginated run-list row.
type HistoryRunDTO struct {
	RunID        string                   `json:"run_id"`
	StartedAt    time.Time                `json:"started_at"`
	ObservedAt   time.Time                `json:"observed_at"`
	Character    string                   `json:"character"`
	Difficulty   string                   `json:"difficulty"`
	Run          string                   `json:"run"`
	DefinitionID string                   `json:"definition_id"`
	RouteID      string                   `json:"route_id"`
	Outcome      telemetry.HistoryOutcome `json:"outcome"`
	Reason       string                   `json:"reason,omitempty"`
	LastStep     string                   `json:"last_step,omitempty"`
	DurationMs   int64                    `json:"duration_ms"`
	BossKills    int                      `json:"boss_kills"`
	Funnel       HistoryFunnelDTO         `json:"funnel"`
}

// HistoryRunItemDTO is one item chain inside a run detail.
type HistoryRunItemDTO struct {
	UnitID                   uint32 `json:"unit_id"`
	ItemKey                  string `json:"item_key,omitempty"`
	ItemName                 string `json:"item_name,omitempty"`
	BaseCode                 string `json:"base_code,omitempty"`
	Quality                  string `json:"quality,omitempty"`
	IdentityKind             string `json:"identity_kind,omitempty"`
	IdentityKey              string `json:"identity_key,omitempty"`
	PickitProfileID          string `json:"pickit_profile_id,omitempty"`
	PickitRuleID             string `json:"pickit_rule_id,omitempty"`
	PickitAction             string `json:"pickit_action,omitempty"`
	PickitProfileRevision    uint64 `json:"pickit_profile_revision,omitempty"`
	PickitAssignmentRevision uint64 `json:"pickit_assignment_revision,omitempty"`
	Seen                     bool   `json:"seen"`
	Matched                  bool   `json:"matched"`
	PickedUp                 bool   `json:"picked_up"`
	Stashed                  bool   `json:"stashed"`
	Sold                     bool   `json:"sold"`
	PickupLost               bool   `json:"pickup_lost"`
	PostPickupLost           bool   `json:"post_pickup_lost"`
}

// HistoryRunDetailDTO contains semantic detail and optionally raw events.
type HistoryRunDetailDTO struct {
	HistoryRunDTO
	EndedAt                *time.Time          `json:"ended_at,omitempty"`
	RouteLayoutFingerprint string              `json:"route_layout_fingerprint,omitempty"`
	Stages                 HistoryStagesDTO    `json:"stages"`
	Items                  []HistoryRunItemDTO `json:"items"`
	RawContext             map[string]any      `json:"raw_context,omitempty"`
	RawEvents              []telemetry.Event   `json:"raw_events,omitempty"`
}

// HistorySummaryResponse is the summary endpoint envelope.
type HistorySummaryResponse struct {
	Meta         HistoryMetaDTO          `json:"meta"`
	Summary      HistorySummaryDTO       `json:"summary"`
	DailyBuckets []HistoryDailyBucketDTO `json:"daily_buckets"`
}

// HistoryComparisonsResponse is the comparison endpoint envelope.
type HistoryComparisonsResponse struct {
	Meta        HistoryMetaDTO         `json:"meta"`
	Comparisons []HistoryComparisonDTO `json:"comparisons"`
}

// HistoryItemsResponse is a cursor-paginated item envelope.
type HistoryItemsResponse struct {
	Meta       HistoryMetaDTO   `json:"meta"`
	Items      []HistoryItemDTO `json:"items"`
	NextCursor string           `json:"next_cursor,omitempty"`
}

// HistoryRunsResponse is a cursor-paginated run envelope.
type HistoryRunsResponse struct {
	Meta       HistoryMetaDTO  `json:"meta"`
	Runs       []HistoryRunDTO `json:"runs"`
	NextCursor string          `json:"next_cursor,omitempty"`
}

// HistoryRunDetailResponse is one run-detail envelope.
type HistoryRunDetailResponse struct {
	Meta HistoryMetaDTO      `json:"meta"`
	Run  HistoryRunDetailDTO `json:"run"`
}

// HistoryReportDTO is the complete filtered JSON export from the same analysis.
type HistoryReportDTO struct {
	Meta         HistoryMetaDTO          `json:"meta"`
	Summary      HistorySummaryDTO       `json:"summary"`
	DailyBuckets []HistoryDailyBucketDTO `json:"daily_buckets"`
	Comparisons  []HistoryComparisonDTO  `json:"comparisons"`
	Items        []HistoryItemDTO        `json:"items"`
	Runs         []HistoryRunDTO         `json:"runs"`
}

// HistoryMaintenanceDiagnosticDTO is one path-free maintenance diagnostic.
type HistoryMaintenanceDiagnosticDTO struct {
	FileID string `json:"file_id,omitempty"`
	Code   string `json:"code"`
}

// HistoryDeletePreviewRequest binds preview creation to the current supervisor generation.
type HistoryDeletePreviewRequest struct {
	ExpectedGeneration uint64 `json:"expected_generation"`
}

// HistoryDeletePreviewDTO is the first non-destructive delete-all confirmation stage.
type HistoryDeletePreviewDTO struct {
	ConfirmationToken string         `json:"confirmation_token"`
	IndexGeneration   uint64         `json:"index_generation"`
	CandidateFiles    int            `json:"candidate_files"`
	CandidateBytes    int64          `json:"candidate_bytes"`
	ProtectedFiles    int            `json:"protected_files"`
	Categories        map[string]int `json:"categories"`
}

// HistoryDeleteConfirmRequest is the exact second confirmation payload.
type HistoryDeleteConfirmRequest struct {
	ExpectedGeneration uint64 `json:"expected_generation"`
	ConfirmationToken  string `json:"confirmation_token"`
	IndexGeneration    uint64 `json:"index_generation"`
	CandidateFiles     int    `json:"candidate_files"`
	CandidateBytes     int64  `json:"candidate_bytes"`
}

// HistoryDeleteResultDTO reports complete or partial delete-all results without paths.
type HistoryDeleteResultDTO struct {
	DeletedFiles   int                               `json:"deleted_files"`
	DeletedBytes   int64                             `json:"deleted_bytes"`
	ProtectedFiles int                               `json:"protected_files"`
	Diagnostics    []HistoryMaintenanceDiagnosticDTO `json:"diagnostics"`
}
