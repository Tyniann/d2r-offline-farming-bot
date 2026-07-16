package app

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

// SupervisorReason is a stable machine-readable command or terminal reason.
type SupervisorReason string

const (
	// SupervisorReasonCommandConflict rejects a command while another generation is active.
	SupervisorReasonCommandConflict SupervisorReason = "command_conflict"
	// SupervisorReasonStateChanged rejects a command against a stale generation.
	SupervisorReasonStateChanged SupervisorReason = "state_changed"
	// SupervisorReasonSessionNotRunning rejects a running-only command.
	SupervisorReasonSessionNotRunning SupervisorReason = "session_not_running"
	// SupervisorReasonSessionNotPaused rejects resume outside a between-runs pause.
	SupervisorReasonSessionNotPaused SupervisorReason = "session_not_paused"
	// SupervisorReasonEmergencyStopRequested correlates API and F11 cancellation.
	SupervisorReasonEmergencyStopRequested SupervisorReason = "emergency_stop_requested"
	// SupervisorReasonWorkerPanic reports a recovered worker panic.
	SupervisorReasonWorkerPanic SupervisorReason = "worker_panic"
)

// SupervisorCommandError reports a rejected command without changing state.
type SupervisorCommandError struct {
	Code SupervisorReason
}

// Error implements error.
func (e *SupervisorCommandError) Error() string {
	return string(e.Code)
}

// SupervisorCommandMeta supplies idempotency and optimistic concurrency data.
type SupervisorCommandMeta struct {
	CommandID          string
	ExpectedGeneration uint64
}

// SupervisorRunRequest is the immutable input owned by one worker generation.
type SupervisorRunRequest struct {
	RunID      string
	QueueIndex int
	Cycle      int
	Retry      int
}

// SupervisorRunResult is the terminal result of one complete worker unit.
type SupervisorRunResult struct {
	Disposition QueueRunDisposition
	Reason      string
}

// SupervisorRunner executes one complete session unit and must return after
// its context is cancelled. It must not start an independent session pipeline.
type SupervisorRunner interface {
	Run(context.Context, SupervisorRunRequest) SupervisorRunResult
}

// FarmQueueGuard rechecks the immutable queue and current entry before each fresh worker starts.
type FarmQueueGuard func(FarmQueuePlan, int) error

// SupervisorSnapshot is an immutable, race-safe view of the supervisor.
type SupervisorSnapshot struct {
	Generation    uint64
	State         SupervisorState
	PendingIntent SupervisorIntent
	ActiveRunID   string
	// QueueKnown distinguishes an authoritative empty queue from passive runtime updates.
	QueueKnown          bool
	Queue               []string
	QueueIndex          int
	Cycle               int
	Retry               int
	StartedRuns         int
	ConsecutiveFailures int
	TotalRestarts       int
	Budgets             FarmQueueBudgets
	LastResult          SupervisorRunResult
}

type supervisorCommandRecord struct {
	command SupervisorCommand
	payload string
	result  SupervisorSnapshot
}

// SessionSupervisor serializes commands and owns exactly one cancellable
// worker generation. Snapshot never exposes mutable internal state.
type SessionSupervisor struct {
	mu                  sync.Mutex
	runner              SupervisorRunner
	queueGuard          FarmQueueGuard
	state               SupervisorState
	intent              SupervisorIntent
	request             SupervisorRunRequest
	plan                FarmQueuePlan
	queueIndex          int
	cycle               int
	retry               int
	startedRuns         int
	consecutiveFailures int
	totalRestarts       int
	startedAt           time.Time
	now                 func() time.Time
	legacySingle        bool
	result              SupervisorRunResult
	generation          uint64
	cancel              context.CancelFunc
	done                chan struct{}
	commands            map[string]supervisorCommandRecord
	shutdown            bool
}

// SetQueueGuard installs the between-runs availability guard while no queue is active.
func (s *SessionSupervisor) SetQueueGuard(guard FarmQueueGuard) error {
	if s == nil {
		return fmt.Errorf("session supervisor is nil")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state != SupervisorStateIdle && s.state != SupervisorStateIdleInGame && s.state != SupervisorStateStoppedError {
		return &QueueValidationError{Code: QueueReasonLocked, EntryIndex: -1}
	}
	s.queueGuard = guard
	return nil
}

// NewSessionSupervisor creates an idle long-lived supervisor.
func NewSessionSupervisor(runner SupervisorRunner) (*SessionSupervisor, error) {
	if runner == nil {
		return nil, fmt.Errorf("session supervisor runner is required")
	}
	return &SessionSupervisor{runner: runner, state: SupervisorStateIdle, intent: SupervisorIntentNone, commands: make(map[string]supervisorCommandRecord), now: time.Now}, nil
}

// Snapshot returns the current immutable supervisor projection.
func (s *SessionSupervisor) Snapshot() SupervisorSnapshot {
	if s == nil {
		return SupervisorSnapshot{}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.snapshotLocked()
}

// Start starts one fresh worker generation.
func (s *SessionSupervisor) Start(meta SupervisorCommandMeta, request SupervisorRunRequest) (SupervisorSnapshot, error) {
	plan := FarmQueuePlan{
		RunIDs:  []string{request.RunID},
		Budgets: FarmQueueBudgets{MaxRuns: int(^uint(0) >> 1), MaxDuration: 365 * 24 * time.Hour},
	}
	return s.startQueue(meta, plan, "single:"+request.RunID, true)
}

// StartQueue starts a validated runtime-only queue and owns its immutable plan until termination.
func (s *SessionSupervisor) StartQueue(meta SupervisorCommandMeta, plan FarmQueuePlan) (SupervisorSnapshot, error) {
	return s.startQueue(meta, plan, farmQueuePayload(plan), false)
}

func (s *SessionSupervisor) startQueue(meta SupervisorCommandMeta, plan FarmQueuePlan, payload string, legacySingle bool) (SupervisorSnapshot, error) {
	if s == nil {
		return SupervisorSnapshot{}, fmt.Errorf("session supervisor is nil")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if snapshot, ok, err := s.replayLocked(meta, SupervisorCommandStartQueue, payload); ok || err != nil {
		return snapshot, err
	}
	if s.shutdown || !SupervisorCommandAllowed(s.state, SupervisorCommandStartQueue) {
		return s.snapshotLocked(), &SupervisorCommandError{Code: SupervisorReasonCommandConflict}
	}
	if err := s.validateMetaLocked(meta); err != nil {
		return s.snapshotLocked(), err
	}
	if len(plan.RunIDs) == 0 {
		return s.snapshotLocked(), &QueueValidationError{Code: QueueReasonEmpty, EntryIndex: -1}
	}
	if err := validateFarmQueueBudgets(plan.Budgets); err != nil {
		return s.snapshotLocked(), err
	}
	for i, runID := range plan.RunIDs {
		if strings.TrimSpace(runID) == "" {
			return s.snapshotLocked(), fmt.Errorf("farm queue run ID at index %d is required", i)
		}
	}
	s.plan = cloneFarmQueuePlan(plan)
	s.queueIndex = 0
	s.cycle = 0
	s.retry = 0
	s.startedRuns = 0
	s.consecutiveFailures = 0
	s.totalRestarts = 0
	s.startedAt = s.now()
	s.legacySingle = legacySingle
	s.done = make(chan struct{})
	s.intent = SupervisorIntentNone
	s.result = SupervisorRunResult{}
	s.startWorkerLocked()
	snapshot := s.snapshotLocked()
	s.rememberLocked(meta, SupervisorCommandStartQueue, payload, snapshot)
	return snapshot, nil
}

// PauseAfterRun idempotently requests a pause after the active complete run.
func (s *SessionSupervisor) PauseAfterRun(meta SupervisorCommandMeta) (SupervisorSnapshot, error) {
	return s.setIntent(meta, SupervisorCommandPauseAfterRun, SupervisorIntentPauseAfterRun)
}

// StopAfterRun idempotently requests an orderly stop after the active complete run.
func (s *SessionSupervisor) StopAfterRun(meta SupervisorCommandMeta) (SupervisorSnapshot, error) {
	return s.setIntent(meta, SupervisorCommandStopAfterRun, SupervisorIntentStopAfterRun)
}

func (s *SessionSupervisor) setIntent(meta SupervisorCommandMeta, command SupervisorCommand, intent SupervisorIntent) (SupervisorSnapshot, error) {
	if s == nil {
		return SupervisorSnapshot{}, fmt.Errorf("session supervisor is nil")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if snapshot, ok, err := s.replayLocked(meta, command, ""); ok || err != nil {
		return snapshot, err
	}
	if err := s.validateMetaLocked(meta); err != nil {
		return s.snapshotLocked(), err
	}
	if !SupervisorCommandAllowed(s.state, command) {
		return s.snapshotLocked(), &SupervisorCommandError{Code: SupervisorReasonSessionNotRunning}
	}
	if s.intent == intent {
		snapshot := s.snapshotLocked()
		s.rememberLocked(meta, command, "", snapshot)
		return snapshot, nil
	}
	s.intent = intent
	s.generation++
	snapshot := s.snapshotLocked()
	s.rememberLocked(meta, command, "", snapshot)
	return snapshot, nil
}

// Resume starts a fresh worker after a between-runs pause.
func (s *SessionSupervisor) Resume(meta SupervisorCommandMeta) (SupervisorSnapshot, error) {
	if s == nil {
		return SupervisorSnapshot{}, fmt.Errorf("session supervisor is nil")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if snapshot, ok, err := s.replayLocked(meta, SupervisorCommandResume, ""); ok || err != nil {
		return snapshot, err
	}
	if err := s.validateMetaLocked(meta); err != nil {
		return s.snapshotLocked(), err
	}
	if !SupervisorCommandAllowed(s.state, SupervisorCommandResume) {
		return s.snapshotLocked(), &SupervisorCommandError{Code: SupervisorReasonSessionNotPaused}
	}
	s.intent = SupervisorIntentNone
	s.startWorkerLocked()
	snapshot := s.snapshotLocked()
	s.rememberLocked(meta, SupervisorCommandResume, "", snapshot)
	return snapshot, nil
}

// EmergencyStop immediately cancels the active generation using the F11 reason.
func (s *SessionSupervisor) EmergencyStop(meta SupervisorCommandMeta) (SupervisorSnapshot, error) {
	if s == nil {
		return SupervisorSnapshot{}, fmt.Errorf("session supervisor is nil")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if snapshot, ok, err := s.replayLocked(meta, SupervisorCommandEmergencyStop, ""); ok || err != nil {
		return snapshot, err
	}
	if err := s.validateMetaLocked(meta); err != nil {
		return s.snapshotLocked(), err
	}
	if !SupervisorCommandAllowed(s.state, SupervisorCommandEmergencyStop) {
		return s.snapshotLocked(), &SupervisorCommandError{Code: SupervisorReasonSessionNotRunning}
	}
	s.state = SupervisorStateCancelling
	s.intent = SupervisorIntentNone
	s.generation++
	if s.cancel != nil {
		s.cancel()
	} else {
		s.finishQueueLocked(SupervisorStateIdle, SupervisorRunResult{Disposition: QueueRunStop, Reason: string(SupervisorReasonEmergencyStopRequested)}, true)
	}
	snapshot := s.snapshotLocked()
	s.rememberLocked(meta, SupervisorCommandEmergencyStop, "", snapshot)
	return snapshot, nil
}

// Wait blocks until the current worker generation has terminated.
func (s *SessionSupervisor) Wait(ctx context.Context) error {
	if s == nil {
		return fmt.Errorf("session supervisor is nil")
	}
	s.mu.Lock()
	done := s.done
	s.mu.Unlock()
	if done == nil {
		return nil
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-done:
		return nil
	}
}

// Shutdown cancels an active worker and waits for it to stop.
func (s *SessionSupervisor) Shutdown(ctx context.Context) error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	s.shutdown = true
	if s.cancel != nil {
		s.state = SupervisorStateCancelling
		s.cancel()
	} else if s.state == SupervisorStatePausedBetweenRuns {
		s.finishQueueLocked(SupervisorStateIdle, SupervisorRunResult{Disposition: QueueRunStop, Reason: string(SupervisorReasonEmergencyStopRequested)}, true)
	}
	done := s.done
	s.mu.Unlock()
	if done == nil {
		return nil
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-done:
		return nil
	}
}

func (s *SessionSupervisor) startWorkerLocked() {
	if len(s.plan.RunIDs) == 0 || s.queueIndex < 0 || s.queueIndex >= len(s.plan.RunIDs) {
		s.finishQueueLocked(SupervisorStateStoppedError, SupervisorRunResult{Disposition: QueueRunStop, Reason: string(QueueReasonEntryUnavailable)}, false)
		return
	}
	if s.queueGuard != nil {
		if err := s.queueGuard(cloneFarmQueuePlan(s.plan), s.queueIndex); err != nil {
			s.finishQueueLocked(SupervisorStateStoppedError, SupervisorRunResult{Disposition: QueueRunStop, Reason: err.Error()}, false)
			return
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	s.state = SupervisorStateStartingRun
	s.generation++
	s.startedRuns++
	generation := s.generation
	s.request = SupervisorRunRequest{RunID: s.plan.RunIDs[s.queueIndex], QueueIndex: s.queueIndex, Cycle: s.cycle, Retry: s.retry}
	request := s.request
	go s.runWorker(ctx, generation, request)
}

func (s *SessionSupervisor) runWorker(ctx context.Context, generation uint64, request SupervisorRunRequest) {
	s.mu.Lock()
	if s.generation == generation && s.state == SupervisorStateStartingRun {
		s.state = SupervisorStateRunningRun
		s.generation++
		generation = s.generation
	}
	s.mu.Unlock()
	result := SupervisorRunResult{}
	func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				result = SupervisorRunResult{Disposition: QueueRunStop, Reason: string(SupervisorReasonWorkerPanic)}
			}
		}()
		result = s.runner.Run(ctx, request)
	}()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.result = result
	s.cancel = nil
	if s.state == SupervisorStateCancelling || ctx.Err() != nil || s.shutdown {
		s.finishQueueLocked(SupervisorStateIdle, SupervisorRunResult{Disposition: QueueRunStop, Reason: string(SupervisorReasonEmergencyStopRequested)}, true)
		return
	}
	// F11 cancels inside the worker's own hotkey context. Treat its canonical
	// reason exactly like the API emergency command, including queue disposal.
	if result.Reason == string(SupervisorReasonEmergencyStopRequested) {
		s.finishQueueLocked(SupervisorStateIdle, result, true)
		return
	}
	switch result.Disposition {
	case QueueRunAdvance:
		s.consecutiveFailures = 0
		s.retry = 0
		s.queueIndex++
		if s.queueIndex == len(s.plan.RunIDs) {
			s.queueIndex = 0
			s.cycle++
		}
		switch s.intent {
		case SupervisorIntentPauseAfterRun:
			if reason := s.exhaustedBudgetLocked(); reason != "" {
				s.finishQueueLocked(SupervisorStateIdle, SupervisorRunResult{Disposition: QueueRunStop, Reason: reason}, false)
			} else {
				s.state = SupervisorStatePausedBetweenRuns
				s.intent = SupervisorIntentNone
				s.request = SupervisorRunRequest{}
				s.generation++
			}
		case SupervisorIntentStopAfterRun:
			s.finishQueueLocked(SupervisorStateIdle, result, true)
		case SupervisorIntentNone:
			if s.legacySingle {
				s.finishQueueLocked(SupervisorStateIdle, result, true)
			} else if reason := s.exhaustedBudgetLocked(); reason != "" {
				s.finishQueueLocked(SupervisorStateIdle, SupervisorRunResult{Disposition: QueueRunStop, Reason: reason}, false)
			} else {
				s.startWorkerLocked()
			}
		}
	case QueueRunRetryCurrent:
		s.consecutiveFailures++
		if reason := s.exhaustedBudgetLocked(); reason != "" {
			s.finishQueueLocked(SupervisorStateIdle, SupervisorRunResult{Disposition: QueueRunStop, Reason: reason}, false)
			return
		}
		if s.consecutiveFailures > s.plan.Budgets.MaxConsecutiveFailures || s.totalRestarts >= s.plan.Budgets.MaxTotalRestarts {
			s.finishQueueLocked(SupervisorStateStoppedError, result, false)
			return
		}
		s.totalRestarts++
		s.retry++
		s.startWorkerLocked()
	case QueueRunStop:
		s.finishQueueLocked(SupervisorStateStoppedError, result, false)
	default:
		s.finishQueueLocked(SupervisorStateStoppedError, SupervisorRunResult{Disposition: QueueRunStop, Reason: "invalid_queue_disposition"}, false)
	}
}

func (s *SessionSupervisor) validateMetaLocked(meta SupervisorCommandMeta) error {
	if meta.CommandID == "" {
		return fmt.Errorf("supervisor command ID is required")
	}
	if meta.ExpectedGeneration != s.generation {
		return &SupervisorCommandError{Code: SupervisorReasonStateChanged}
	}
	return nil
}

func (s *SessionSupervisor) replayLocked(meta SupervisorCommandMeta, command SupervisorCommand, payload string) (SupervisorSnapshot, bool, error) {
	if meta.CommandID == "" {
		return SupervisorSnapshot{}, false, nil
	}
	record, ok := s.commands[meta.CommandID]
	if !ok {
		return SupervisorSnapshot{}, false, nil
	}
	if record.command != command || record.payload != payload {
		return s.snapshotLocked(), false, fmt.Errorf("supervisor command ID reused with different content")
	}
	return record.result, true, nil
}

func (s *SessionSupervisor) rememberLocked(meta SupervisorCommandMeta, command SupervisorCommand, payload string, result SupervisorSnapshot) {
	s.commands[meta.CommandID] = supervisorCommandRecord{command: command, payload: payload, result: result}
}

func (s *SessionSupervisor) snapshotLocked() SupervisorSnapshot {
	return SupervisorSnapshot{
		Generation: s.generation, State: s.state, PendingIntent: s.intent, ActiveRunID: s.request.RunID,
		QueueKnown: true, Queue: append([]string(nil), s.plan.RunIDs...), QueueIndex: s.queueIndex, Cycle: s.cycle, Retry: s.retry,
		StartedRuns: s.startedRuns, ConsecutiveFailures: s.consecutiveFailures, TotalRestarts: s.totalRestarts,
		Budgets: s.plan.Budgets, LastResult: s.result,
	}
}

func (s *SessionSupervisor) exhaustedBudgetLocked() string {
	if !s.startedAt.IsZero() && s.now().Sub(s.startedAt) >= s.plan.Budgets.MaxDuration {
		return string(QueueReasonDurationBudgetExhausted)
	}
	if s.startedRuns >= s.plan.Budgets.MaxRuns {
		return string(QueueReasonRunBudgetExhausted)
	}
	return ""
}

func (s *SessionSupervisor) finishQueueLocked(state SupervisorState, result SupervisorRunResult, clear bool) {
	s.state = state
	s.result = result
	s.intent = SupervisorIntentNone
	s.cancel = nil
	s.request = SupervisorRunRequest{}
	if clear {
		s.plan = FarmQueuePlan{}
		s.queueIndex = 0
		s.cycle = 0
		s.retry = 0
		s.legacySingle = false
	}
	s.generation++
	if s.done != nil {
		close(s.done)
		s.done = nil
	}
}

func cloneFarmQueuePlan(plan FarmQueuePlan) FarmQueuePlan {
	clone := plan
	clone.RunIDs = append([]string(nil), plan.RunIDs...)
	return clone
}

func farmQueuePayload(plan FarmQueuePlan) string {
	return fmt.Sprintf("%s|%s|%d|%d|%d|%d|%d|%s",
		plan.Character, plan.Difficulty, plan.CatalogRevision, plan.Budgets.MaxRuns,
		plan.Budgets.MaxDuration, plan.Budgets.MaxConsecutiveFailures, plan.Budgets.MaxTotalRestarts,
		strings.Join(plan.RunIDs, "\x00"),
	)
}
