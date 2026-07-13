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
	// RoutePlaybackStarted begins one full route playback session.
	RoutePlaybackStarted EventName = "route_playback_started"
	// RoutePointStarted identifies the next recorded World point.
	RoutePointStarted EventName = "route_point_started"
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
	// RunStarted begins one fresh run executor.
	RunStarted EventName = "run_started"
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
)

// Event is one JSONL record. Zero-valued optional fields are omitted.
type Event struct {
	SchemaVersion         int       `json:"schema_version"`
	Timestamp             time.Time `json:"timestamp"`
	Event                 EventName `json:"event"`
	RunID                 string    `json:"run_id"`
	SessionID             string    `json:"session_id,omitempty"`
	GameID                string    `json:"game_id,omitempty"`
	Run                   string    `json:"run"`
	Phase                 string    `json:"phase,omitempty"`
	AreaID                uint32    `json:"area_id,omitempty"`
	UnitID                uint32    `json:"unit_id,omitempty"`
	TxtFileNo             uint32    `json:"txt_file_no,omitempty"`
	Code                  string    `json:"code,omitempty"`
	Name                  string    `json:"name,omitempty"`
	Reason                string    `json:"reason,omitempty"`
	Attempt               int       `json:"attempt,omitempty"`
	HoverAttempt          int       `json:"hover_attempt,omitempty"`
	GridX                 *int      `json:"grid_x,omitempty"`
	GridY                 *int      `json:"grid_y,omitempty"`
	CandidateCount        int       `json:"candidate_count,omitempty"`
	RouteID               string    `json:"route_id,omitempty"`
	SegmentID             string    `json:"segment_id,omitempty"`
	SegmentIndex          *int      `json:"segment_index,omitempty"`
	PointIndex            *int      `json:"point_index,omitempty"`
	TargetX               uint32    `json:"target_x,omitempty"`
	TargetY               uint32    `json:"target_y,omitempty"`
	TargetAreaID          uint32    `json:"target_area_id,omitempty"`
	RunOrdinal            int       `json:"run_ordinal,omitempty"`
	Outcome               string    `json:"outcome,omitempty"`
	Decision              string    `json:"decision,omitempty"`
	LastStep              string    `json:"last_step,omitempty"`
	ElapsedMs             int64     `json:"elapsed_ms,omitempty"`
	ConsecutiveFailures   int       `json:"consecutive_failures,omitempty"`
	TotalRestarts         int       `json:"total_restarts,omitempty"`
	RemainingRestarts     int       `json:"remaining_restarts,omitempty"`
	LastConfirmedPoint    *int      `json:"last_confirmed_point,omitempty"`
	DriftTiles            float64   `json:"drift_tiles,omitempty"`
	LocalRecoveryAttempts int       `json:"local_recovery_attempts,omitempty"`
	RunsStarted           int       `json:"runs_started,omitempty"`
	RunsSuccessful        int       `json:"runs_successful,omitempty"`
	RunsAborted           int       `json:"runs_aborted,omitempty"`
	RunsFailed            int       `json:"runs_failed,omitempty"`
	MaxRuns               int       `json:"max_runs,omitempty"`
	MaxDurationMs         int64     `json:"max_duration_ms,omitempty"`
	Profile               string    `json:"profile,omitempty"`
	Hook                  string    `json:"hook,omitempty"`
	Skill                 string    `json:"skill,omitempty"`
	SkillID               uint16    `json:"skill_id,omitempty"`
	Target                string    `json:"target,omitempty"`
	Resource              string    `json:"resource,omitempty"`
	ThresholdPercent      uint8     `json:"threshold_percent,omitempty"`
	BeltSlot              int       `json:"belt_slot,omitempty"`
	Confirmed             bool      `json:"confirmed,omitempty"`
	TownStep              *int      `json:"town_step,omitempty"`
	TownKind              string    `json:"town_kind,omitempty"`
	TownService           string    `json:"town_service,omitempty"`
	CurrentCount          *int      `json:"current_count,omitempty"`
	TriggerThreshold      *int      `json:"trigger_threshold,omitempty"`
	BeltSlots             []int     `json:"belt_slots,omitempty"`
	PurchaseMode          string    `json:"purchase_mode,omitempty"`
	Vendor                string    `json:"vendor,omitempty"`
	Cost                  *int      `json:"cost,omitempty"`
	VerifiedFinalCount    *int      `json:"verified_final_count,omitempty"`
}

type flushWriter interface {
	Write([]byte) (int, error)
	Flush() error
}

// Recorder owns one JSONL file and flushes every event before returning.
type Recorder struct {
	mu     sync.Mutex
	file   *os.File
	writer flushWriter
	path   string
	runID  string
	run    string
	phase  string
	seen   map[string]struct{}
	closed bool
}

// New creates one telemetry file for the selected run.
func New(directory, run, phase string) (*Recorder, error) {
	if strings.TrimSpace(directory) == "" || strings.TrimSpace(run) == "" {
		return nil, fmt.Errorf("telemetry directory and run are required")
	}
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return nil, fmt.Errorf("create telemetry directory: %w", err)
	}
	now := time.Now().UTC()
	runID := fmt.Sprintf("%s-%s-%s", safePart(run), now.Format("20060102T150405.000000000Z"), randomSuffix())
	path := filepath.Join(directory, runID+".jsonl")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("create telemetry file %q: %w", path, err)
	}
	return &Recorder{
		file: file, writer: bufio.NewWriter(file), path: path,
		runID: runID, run: run, phase: phase, seen: make(map[string]struct{}),
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
	event.SchemaVersion = 1
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	} else {
		event.Timestamp = event.Timestamp.UTC()
	}
	event.RunID = r.runID
	event.Run = r.run
	event.Phase = r.phase
	line, err := json.Marshal(event)
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
	return nil
}

// Path returns the JSONL file path.
func (r *Recorder) Path() string {
	if r == nil {
		return ""
	}
	return r.path
}

// RunID returns the stable identifier embedded in every event.
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
