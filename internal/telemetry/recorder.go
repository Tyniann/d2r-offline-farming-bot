// Package telemetry writes fail-closed JSONL events for one farming run.
package telemetry

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// EventName is a stable machine-readable run telemetry event.
type EventName string

// Phase-5 run telemetry event names.
const (
	DropSeen      EventName = "drop_seen"
	PickitMatch   EventName = "pickit_match"
	PickupAttempt EventName = "pickup_attempt"
	PickupSuccess EventName = "pickup_success"
	PickupFailed  EventName = "pickup_failed"
	InventoryFull EventName = "inventory_full"
	StashAttempt  EventName = "stash_attempt"
	StashSuccess  EventName = "stash_success"
	StashFull     EventName = "stash_full"
	// SellSuccess bestätigt die Inventory-Transition einer gepinnten Sell-Unit.
	SellSuccess EventName = "sell_success"
	// BossKillConfirmed bestätigt den Memory-basierten Tod der gepinnten Boss-Unit.
	BossKillConfirmed EventName = "boss_kill_confirmed"
	// RoutePlaybackStarted begins one full route playback session.
	RoutePlaybackStarted EventName = "route_playback_started"
	// RoutePointStarted identifies the next recorded World point.
	RoutePointStarted EventName = "route_point_started"
	// RoutePointSkipped records a nearby point skipped after repeated movement made no target progress.
	RoutePointSkipped EventName = "route_point_skipped"
	// RouteTransitionStarted begins a strict expected Area transition.
	RouteTransitionStarted EventName = "route_transition_started"
	// RouteSegmentCompleted confirms one segment's target Area.
	RouteSegmentCompleted EventName = "route_segment_completed"
	// RoutePlaybackCompleted confirms the final route target Area.
	RoutePlaybackCompleted EventName = "route_playback_completed"
	// RoutePlaybackFailed records a fail-closed terminal error.
	RoutePlaybackFailed EventName = "route_playback_failed"
	// RoutePlaybackStopped records an explicit operator Stop.
	RoutePlaybackStopped EventName = "route_playback_stopped"
	// SessionStarted begins one finite autonomous session.
	SessionStarted EventName = "session_started"
	// GameStarted confirms one verified offline game generation.
	GameStarted EventName = "game_started"
	// GameExited confirms one supervisor-owned Save-&-Exit boundary.
	GameExited EventName = "game_exited"
	// RunStarted begins one fresh run executor.
	RunStarted EventName = "run_started"
	// RunContext binds one run generation to its resolved definition and assets.
	RunContext EventName = "run_context"
	// StuckDetected records the progress context that exhausted local recovery.
	StuckDetected EventName = "stuck_detected"
	// RunCompleted records a successful terminal run.
	RunCompleted EventName = "run_completed"
	// RunAborted records a controlled early run termination.
	RunAborted EventName = "run_aborted"
	// RunFailed records a terminal run failure.
	RunFailed EventName = "run_failed"
	// GameRestartRequested records the one recovery decision for a run result.
	GameRestartRequested EventName = "game_restart_requested"
	// SessionCompleted records planned budget completion.
	SessionCompleted EventName = "session_completed"
	// SessionStopped records an operator stop.
	SessionStopped EventName = "session_stopped"
	// SessionFailed records terminal session failure.
	SessionFailed EventName = "session_failed"
	// ProfileHookAction records one successful semantic hook input.
	ProfileHookAction EventName = "profile_hook_action"
	// ResourcePotionRequested records one successful belt input.
	ResourcePotionRequested EventName = "resource_potion_requested"
	// ResourceConsumptionConfirmed records Memory-confirmed potion consumption.
	ResourceConsumptionConfirmed EventName = "resource_consumption_confirmed"
	// ProfileActionFailed records a stable profile failure reason.
	ProfileActionFailed EventName = "profile_action_failed"
	// TownAction records one real preparation input and its decision context.
	TownAction EventName = "town_action"
	// TownStepCompleted records one verified preparation step.
	TownStepCompleted EventName = "town_step_completed"
	// RunStepStarted records entry into one shared run-pipeline step.
	RunStepStarted EventName = "run_step_started"
	// RunStepCompleted records successful completion of one shared run-pipeline step.
	RunStepCompleted EventName = "run_step_completed"
	// RunStepFailed records terminal failure of one shared run-pipeline step.
	RunStepFailed EventName = "run_step_failed"
	// RunEncounterActionStarted records one indexed pre-combat action boundary.
	RunEncounterActionStarted EventName = "run_encounter_action_started"
	// RunEncounterActionCompleted records verified completion of one indexed pre-combat action.
	RunEncounterActionCompleted EventName = "run_encounter_action_completed"
	// RouteThreatDetected records a newly selected route blocker or zone.
	RouteThreatDetected EventName = "route_threat_detected"
	// RouteClearStarted records entry into one route blockade.
	RouteClearStarted EventName = "route_clear_started"
	// RouteMonsterSnapshotSaturated records saturation entry, coverage changes, and exit.
	RouteMonsterSnapshotSaturated EventName = "route_monster_snapshot_saturated"
	// RouteClearAction records one actually sent route-clear combat or approach input.
	RouteClearAction EventName = "route_clear_action"
	// RouteClearProgress records one accepted objective watchdog reset.
	RouteClearProgress EventName = "route_clear_progress"
	// RouteClearCompleted records one aggregate stable-clear completion.
	RouteClearCompleted EventName = "route_clear_completed"
	// RouteManaHold records mana-hold start, material change, or end.
	RouteManaHold EventName = "route_mana_hold"
	// RouteRecoverySuppressed records a prevented unsafe recovery input.
	RouteRecoverySuppressed EventName = "route_recovery_suppressed"
	// TownMercenaryHealRequested records the hover-confirmed Akara click for Merc heal.
	TownMercenaryHealRequested EventName = "town_mercenary_heal_requested"
	// TownMercenaryHealConfirmed records a fresh Full-HP Merc snapshot after Akara.
	TownMercenaryHealConfirmed EventName = "town_mercenary_heal_confirmed"
	// TownMercenaryReviveRequested records the single Kashya Enter submit.
	TownMercenaryReviveRequested EventName = "town_mercenary_revive_requested"
	// TownMercenaryReviveConfirmed records a fresh Alive Merc after Kashya revive.
	TownMercenaryReviveConfirmed EventName = "town_mercenary_revive_confirmed"
	// MercenaryDied records Alive→Dead hireling transition with last known HP%.
	MercenaryDied EventName = "mercenary_died"
	// CowRecipeProgress records one Memory-confirmed semantic Cow setup or recipe boundary.
	CowRecipeProgress EventName = "cow_recipe_progress"
	// ChestOpened records a Memory-confirmed Supertruhe open. It is not a boss kill.
	ChestOpened EventName = "chest_opened"
	// ChestSkipped records a Supertruhe left unopened after the allowed click budget.
	ChestSkipped EventName = "chest_skipped"
	// RackOperated records a Memory-confirmed hut-rack open next to a Supertruhe.
	RackOperated EventName = "rack_operated"
	// RackSkipped records a hut rack left unopened after the allowed click budget.
	RackSkipped EventName = "rack_skipped"
)

// Event is one JSONL record. Zero-valued optional fields are omitted.
type Event struct {
	SchemaVersion                int                    `json:"schema_version,omitempty"`
	Stream                       HistoryStream          `json:"stream,omitempty"`
	Timestamp                    time.Time              `json:"timestamp"`
	Event                        EventName              `json:"event"`
	RunID                        string                 `json:"run_id,omitempty"`
	SessionID                    string                 `json:"session_id,omitempty"`
	GameID                       string                 `json:"game_id,omitempty"`
	Mode                         HistoryMode            `json:"mode,omitempty"`
	Character                    string                 `json:"character,omitempty"`
	Difficulty                   string                 `json:"difficulty,omitempty"`
	GameVersion                  string                 `json:"game_version,omitempty"`
	Run                          string                 `json:"run,omitempty"`
	DefinitionID                 string                 `json:"definition_id,omitempty"`
	Phase                        string                 `json:"phase,omitempty"`
	Step                         string                 `json:"step,omitempty"`
	Stage                        HistoryStage           `json:"stage,omitempty"`
	ActionIndex                  *int                   `json:"action_index,omitempty"`
	AreaID                       uint32                 `json:"area_id,omitempty"`
	UnitID                       uint32                 `json:"unit_id,omitempty"`
	TxtFileNo                    uint32                 `json:"txt_file_no,omitempty"`
	Code                         string                 `json:"code,omitempty"`
	Name                         string                 `json:"name,omitempty"`
	BossID                       string                 `json:"boss_id,omitempty"`
	BossName                     string                 `json:"boss_name,omitempty"`
	ItemKey                      string                 `json:"item_key,omitempty"`
	ItemName                     string                 `json:"item_name,omitempty"`
	BaseCode                     string                 `json:"base_code,omitempty"`
	Quality                      string                 `json:"quality,omitempty"`
	ItemIdentityKind             string                 `json:"item_identity_kind,omitempty"`
	ItemIdentityKey              string                 `json:"item_identity_key,omitempty"`
	Reason                       string                 `json:"reason,omitempty"`
	Attempt                      int                    `json:"attempt,omitempty"`
	HoverAttempt                 int                    `json:"hover_attempt,omitempty"`
	GridX                        *int                   `json:"grid_x,omitempty"`
	GridY                        *int                   `json:"grid_y,omitempty"`
	CandidateCount               int                    `json:"candidate_count,omitempty"`
	RouteID                      string                 `json:"route_id,omitempty"`
	RouteLayoutFingerprint       string                 `json:"route_layout_fingerprint,omitempty"`
	SetupRouteID                 string                 `json:"setup_route_id,omitempty"`
	SetupRouteLayoutFingerprint  string                 `json:"setup_route_layout_fingerprint,omitempty"`
	RouteRole                    string                 `json:"route_role,omitempty"`
	WaypointTarget               string                 `json:"waypoint_target,omitempty"`
	TownOrigin                   string                 `json:"town_origin,omitempty"`
	SegmentID                    string                 `json:"segment_id,omitempty"`
	SegmentIndex                 *int                   `json:"segment_index,omitempty"`
	PointIndex                   *int                   `json:"point_index,omitempty"`
	TargetX                      uint32                 `json:"target_x,omitempty"`
	TargetY                      uint32                 `json:"target_y,omitempty"`
	TargetAreaID                 uint32                 `json:"target_area_id,omitempty"`
	RunOrdinal                   int                    `json:"run_ordinal,omitempty"`
	QueueIndex                   *int                   `json:"queue_index,omitempty"`
	QueueCycle                   *int                   `json:"queue_cycle,omitempty"`
	RunStartedAt                 *time.Time             `json:"run_started_at,omitempty"`
	Outcome                      string                 `json:"outcome,omitempty"`
	Decision                     string                 `json:"decision,omitempty"`
	LastStep                     string                 `json:"last_step,omitempty"`
	ElapsedMs                    int64                  `json:"elapsed_ms,omitempty"`
	ConsecutiveFailures          int                    `json:"consecutive_failures,omitempty"`
	TotalRestarts                int                    `json:"total_restarts,omitempty"`
	RemainingRestarts            int                    `json:"remaining_restarts,omitempty"`
	LastConfirmedPoint           *int                   `json:"last_confirmed_point,omitempty"`
	DriftTiles                   float64                `json:"drift_tiles,omitempty"`
	LocalRecoveryAttempts        int                    `json:"local_recovery_attempts,omitempty"`
	RunsStarted                  int                    `json:"runs_started,omitempty"`
	RunsSuccessful               int                    `json:"runs_successful,omitempty"`
	RunsAborted                  int                    `json:"runs_aborted,omitempty"`
	RunsFailed                   int                    `json:"runs_failed,omitempty"`
	MaxRuns                      int                    `json:"max_runs,omitempty"`
	MaxDurationMs                int64                  `json:"max_duration_ms,omitempty"`
	Profile                      string                 `json:"profile,omitempty"`
	Hook                         string                 `json:"hook,omitempty"`
	Skill                        string                 `json:"skill,omitempty"`
	SkillID                      uint16                 `json:"skill_id,omitempty"`
	Target                       string                 `json:"target,omitempty"`
	Resource                     string                 `json:"resource,omitempty"`
	Recipient                    string                 `json:"recipient,omitempty"`
	ThresholdPercent             uint8                  `json:"threshold_percent,omitempty"`
	BeltSlot                     int                    `json:"belt_slot,omitempty"`
	Confirmed                    bool                   `json:"confirmed,omitempty"`
	MercUnitID                   uint32                 `json:"merc_unit_id,omitempty"`
	HPBefore                     int                    `json:"hp_before,omitempty"`
	HPAfter                      int                    `json:"hp_after,omitempty"`
	TownStep                     *int                   `json:"town_step,omitempty"`
	TownKind                     string                 `json:"town_kind,omitempty"`
	TownService                  string                 `json:"town_service,omitempty"`
	CurrentCount                 *int                   `json:"current_count,omitempty"`
	TriggerThreshold             *int                   `json:"trigger_threshold,omitempty"`
	BeltSlots                    []int                  `json:"belt_slots,omitempty"`
	PurchaseMode                 string                 `json:"purchase_mode,omitempty"`
	Vendor                       string                 `json:"vendor,omitempty"`
	Cost                         *int                   `json:"cost,omitempty"`
	VerifiedFinalCount           *int                   `json:"verified_final_count,omitempty"`
	PickitProfileID              string                 `json:"pickit_profile_id,omitempty"`
	PickitRuleID                 string                 `json:"pickit_rule_id,omitempty"`
	PickitAction                 string                 `json:"pickit_action,omitempty"`
	PickitProfileRevision        uint64                 `json:"pickit_profile_revision,omitempty"`
	PickitAssignmentRevision     uint64                 `json:"pickit_assignment_revision,omitempty"`
	PickitProfiles               []PickitProfileContext `json:"pickit_profiles,omitempty"`
	Zone                         string                 `json:"zone,omitempty"`
	ModeName                     string                 `json:"mode_name,omitempty"`
	Strategy                     string                 `json:"strategy,omitempty"`
	ActionKind                   string                 `json:"action_kind,omitempty"`
	TargetingMode                string                 `json:"targeting_mode,omitempty"`
	ProgressKind                 string                 `json:"progress_kind,omitempty"`
	NPCID                        uint32                 `json:"npc_id,omitempty"`
	PlayerX                      uint32                 `json:"player_x,omitempty"`
	PlayerY                      uint32                 `json:"player_y,omitempty"`
	DistanceTiles                float64                `json:"distance_tiles,omitempty"`
	RequiredRadiusTiles          float64                `json:"required_radius_tiles,omitempty"`
	CoverageRadiusTiles          float64                `json:"coverage_radius_tiles,omitempty"`
	CoverageComplete             *bool                  `json:"coverage_complete,omitempty"`
	MonstersTruncated            *bool                  `json:"monsters_truncated,omitempty"`
	EligibleMonsterCount         int                    `json:"eligible_monster_count,omitempty"`
	RetainedMonsterCount         int                    `json:"retained_monster_count,omitempty"`
	RelevantThreatCount          int                    `json:"relevant_threat_count,omitempty"`
	CowGroupAnchorUnitID         uint32                 `json:"cow_group_anchor_unit_id,omitempty"`
	CowGroupLivingCount          int                    `json:"cow_group_living_count,omitempty"`
	CowCorpseAnchorDistanceTiles float64                `json:"cow_corpse_anchor_distance_tiles,omitempty"`
	CowCorpseCoverageCount       int                    `json:"cow_corpse_coverage_count,omitempty"`
	PreviousEligibleCount        int                    `json:"previous_eligible_count,omitempty"`
	PreviousRelevantCount        int                    `json:"previous_relevant_count,omitempty"`
	HPPercent                    uint8                  `json:"hp_percent,omitempty"`
	ManaPercent                  uint8                  `json:"mana_percent,omitempty"`
	NoProgressTimeoutMs          int64                  `json:"no_progress_timeout_ms,omitempty"`
	CombatActionsSent            int                    `json:"combat_actions_sent,omitempty"`
	TargetsSeen                  int                    `json:"targets_seen,omitempty"`
	DensityReliefActions         int                    `json:"density_relief_actions,omitempty"`
	HoldMs                       int64                  `json:"hold_ms,omitempty"`
	HoverConfirmed               *bool                  `json:"hover_confirmed,omitempty"`
	ManaDemand                   string                 `json:"mana_demand,omitempty"`
	Threatened                   *bool                  `json:"threatened,omitempty"`
	PositionProgressTiles        float64                `json:"position_progress_tiles,omitempty"`
}

// PickitProfileContext bindet ein Profil und seine Revision an eine Run-Generation.
type PickitProfileContext struct {
	ID       string `json:"id"`
	Revision uint64 `json:"revision"`
}

// RunRecorderContext ist der unveränderliche Kontext einer Schema-4-Run-Datei.
type RunRecorderContext struct {
	RunID                       string
	SessionID                   string
	GameID                      string
	Mode                        HistoryMode
	Character                   string
	Difficulty                  string
	GameVersion                 string
	Run                         string
	DefinitionID                string
	Phase                       string
	RouteID                     string
	RouteLayoutFingerprint      string
	SetupRouteID                string
	SetupRouteLayoutFingerprint string
	QueueIndex                  int
	QueueCycle                  int
	StartedAt                   time.Time
	PickitProfiles              []PickitProfileContext
	PickitAssignmentRevision    uint64
}

type flushWriter interface {
	Write([]byte) (int, error)
	Flush() error
}

// Recorder owns one JSONL file and flushes every event before returning.
type Recorder struct {
	mu             sync.Mutex
	file           *os.File
	writer         flushWriter
	path           string
	runID          string
	run            string
	phase          string
	context        RunRecorderContext
	seen           map[string]struct{}
	contextWritten bool
	closed         bool
}

// New creates one telemetry file for the selected run.
func New(directory, run, phase string) (*Recorder, error) {
	return NewRunRecorder(directory, RunRecorderContext{RunID: NewRunID(run), Mode: HistoryModeDiagnostic, Run: run, Phase: phase, StartedAt: time.Now().UTC()})
}

// NewRunID erzeugt eine pro Prozess und Neustart global eindeutige Run-ID.
func NewRunID(run string) string {
	now := time.Now().UTC()
	return fmt.Sprintf("%s-%s-%s", safePart(run), now.Format("20060102t150405999999999z"), randomSuffix())
}

// NewRunRecorder erstellt eine kompakte Schema-4-Datei mit unveränderlichem Run-Kontext.
func NewRunRecorder(directory string, context RunRecorderContext) (*Recorder, error) {
	if err := validateRunRecorderContext(context); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return nil, fmt.Errorf("create telemetry directory: %w", err)
	}
	path := filepath.Join(directory, context.RunID+".jsonl")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("create telemetry file %q: %w", path, err)
	}
	return &Recorder{
		file: file, writer: bufio.NewWriter(file), path: path,
		runID: context.RunID, run: context.Run, phase: context.Phase, context: cloneRunRecorderContext(context), seen: make(map[string]struct{}),
	}, nil
}

// Emit appends and flushes one event. Drop/pickit events are deduplicated per UnitID and run.
func (r *Recorder) Emit(event Event) error {
	if r == nil {
		return fmt.Errorf("telemetry recorder is nil")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed || r.writer == nil {
		return fmt.Errorf("telemetry recorder is closed")
	}
	key := dedupeKey(event)
	if key != "" {
		if _, ok := r.seen[key]; ok {
			return nil
		}
	}
	if err := r.applyContext(&event); err != nil {
		return err
	}
	event.SchemaVersion = HistorySchemaVersion
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	} else {
		event.Timestamp = event.Timestamp.UTC()
	}
	diskEvent := event
	if r.contextWritten {
		diskEvent = CompactRunEvent(event)
	}
	line, err := json.Marshal(diskEvent)
	if err != nil {
		return fmt.Errorf("marshal telemetry event %q: %w", event.Event, err)
	}
	line = append(line, '\n')
	if _, err := r.writer.Write(line); err != nil {
		return fmt.Errorf("write telemetry event %q: %w", event.Event, err)
	}
	if err := r.writer.Flush(); err != nil {
		return fmt.Errorf("flush telemetry event %q: %w", event.Event, err)
	}
	if key != "" {
		r.seen[key] = struct{}{}
	}
	r.contextWritten = true
	return nil
}

// CompactRunEvent removes immutable file context from one hydrated run event.
// The first JSONL record remains the sole authoritative context source.
func CompactRunEvent(event Event) Event {
	event.SchemaVersion = 0
	event.Stream = ""
	event.RunID = ""
	event.SessionID = ""
	event.GameID = ""
	event.Mode = ""
	event.Character = ""
	event.Difficulty = ""
	event.GameVersion = ""
	event.Run = ""
	event.DefinitionID = ""
	event.Phase = ""
	event.RouteID = ""
	event.RouteLayoutFingerprint = ""
	event.SetupRouteID = ""
	event.SetupRouteLayoutFingerprint = ""
	event.QueueIndex = nil
	event.QueueCycle = nil
	event.RunStartedAt = nil
	event.PickitProfiles = nil
	event.PickitAssignmentRevision = 0
	return event
}

func (r *Recorder) applyContext(event *Event) error {
	if event.RunID != "" && event.RunID != r.context.RunID {
		return fmt.Errorf("%s: event run_id %q does not match recorder %q", HistoryReasonRunIDMismatch, event.RunID, r.context.RunID)
	}
	if event.Mode != "" && event.Mode != r.context.Mode {
		return fmt.Errorf("%s: event mode %q does not match recorder %q", HistoryReasonContextMissing, event.Mode, r.context.Mode)
	}
	for _, field := range []struct {
		name   string
		target *string
		value  string
	}{
		{name: "session_id", target: &event.SessionID, value: r.context.SessionID},
		{name: "game_id", target: &event.GameID, value: r.context.GameID},
		{name: "character", target: &event.Character, value: r.context.Character},
		{name: "difficulty", target: &event.Difficulty, value: r.context.Difficulty},
		{name: "game_version", target: &event.GameVersion, value: r.context.GameVersion},
		{name: "run", target: &event.Run, value: r.context.Run},
	} {
		if err := applyImmutableString(field.name, field.target, field.value); err != nil {
			return err
		}
	}
	event.Stream = HistoryStreamRun
	event.RunID = r.context.RunID
	event.Mode = r.context.Mode
	if err := applyImmutableString("definition_id", &event.DefinitionID, r.context.DefinitionID); err != nil {
		return err
	}
	event.Phase = r.context.Phase
	if err := applyImmutableString("route_id", &event.RouteID, r.context.RouteID); err != nil {
		return err
	}
	if err := applyImmutableString("route_layout_fingerprint", &event.RouteLayoutFingerprint, r.context.RouteLayoutFingerprint); err != nil {
		return err
	}
	if err := applyImmutableString("setup_route_id", &event.SetupRouteID, r.context.SetupRouteID); err != nil {
		return err
	}
	if err := applyImmutableString("setup_route_layout_fingerprint", &event.SetupRouteLayoutFingerprint, r.context.SetupRouteLayoutFingerprint); err != nil {
		return err
	}
	if r.context.Mode == HistoryModeProductiveFarming {
		queueIndex, queueCycle, startedAt := r.context.QueueIndex, r.context.QueueCycle, r.context.StartedAt.UTC()
		event.QueueIndex, event.QueueCycle, event.RunStartedAt = &queueIndex, &queueCycle, &startedAt
	}
	event.PickitProfiles = append([]PickitProfileContext(nil), r.context.PickitProfiles...)
	event.PickitAssignmentRevision = r.context.PickitAssignmentRevision
	return nil
}

func applyImmutableString(field string, target *string, value string) error {
	if value == "" {
		return nil
	}
	if *target != "" && *target != value {
		return fmt.Errorf("%s: event %s %q does not match recorder %q", HistoryReasonContextMissing, field, *target, value)
	}
	*target = value
	return nil
}

func validateRunRecorderContext(context RunRecorderContext) error {
	if strings.TrimSpace(context.RunID) == "" || safePart(context.RunID) != context.RunID || strings.TrimSpace(context.Run) == "" {
		return fmt.Errorf("%s: telemetry run_id and run are required", HistoryReasonContextMissing)
	}
	if context.Mode != HistoryModeProductiveFarming && context.Mode != HistoryModeDiagnostic {
		return fmt.Errorf("%s: unsupported telemetry mode %q", HistoryReasonContextMissing, context.Mode)
	}
	if context.StartedAt.IsZero() {
		return fmt.Errorf("%s: run start time is required", HistoryReasonContextMissing)
	}
	if context.Mode == HistoryModeProductiveFarming && (context.SessionID == "" || context.GameID == "" || context.Character == "" || context.Difficulty == "" || context.GameVersion == "" || context.DefinitionID == "" || context.RouteID == "") {
		return fmt.Errorf("%s: productive farming context is incomplete", HistoryReasonContextMissing)
	}
	if (context.SetupRouteID == "") != (context.SetupRouteLayoutFingerprint == "") {
		return fmt.Errorf("%s: setup route context is incomplete", HistoryReasonContextMissing)
	}
	return nil
}

func cloneRunRecorderContext(context RunRecorderContext) RunRecorderContext {
	context.PickitProfiles = append([]PickitProfileContext(nil), context.PickitProfiles...)
	context.StartedAt = context.StartedAt.UTC()
	return context
}

// Path returns the JSONL file path.
func (r *Recorder) Path() string {
	if r == nil {
		return ""
	}
	return r.path
}

// RunID returns the stable identifier owned by this run recorder.
func (r *Recorder) RunID() string {
	if r == nil {
		return ""
	}
	return r.runID
}

// Close flushes and closes the run file. It is idempotent.
func (r *Recorder) Close() error {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return nil
	}
	r.closed = true
	var flushErr error
	if r.writer != nil {
		flushErr = r.writer.Flush()
	}
	var closeErr error
	if r.file != nil {
		closeErr = r.file.Close()
	}
	if flushErr != nil {
		return fmt.Errorf("flush telemetry: %w", flushErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close telemetry: %w", closeErr)
	}
	return nil
}

func dedupeKey(event Event) string {
	if event.Event != DropSeen && event.Event != PickitMatch {
		return ""
	}
	return fmt.Sprintf("%s:%d", event.Event, event.UnitID)
}

func safePart(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteByte('-')
		}
	}
	return strings.Trim(b.String(), "-")
}

func randomSuffix() string {
	buf := make([]byte, 4)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("%08x", time.Now().UnixNano())
	}
	return hex.EncodeToString(buf)
}
