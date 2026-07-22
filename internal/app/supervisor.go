package app

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/telemetry"
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
	// SupervisorReasonPausedGameLost rejects resume after the verified open game changed or disappeared.
	SupervisorReasonPausedGameLost SupervisorReason = "paused_game_lost"
	// SupervisorReasonGameStartFailed reports the supervisor-owned game-start boundary.
	SupervisorReasonGameStartFailed SupervisorReason = "game_start_failed"
	// SupervisorReasonGameExitFailed reports the supervisor-owned Save-&-Exit boundary.
	SupervisorReasonGameExitFailed SupervisorReason = "game_exit_failed"
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
	DefinitionID string
	ExecutionID  string
	SessionID    string
	QueueIndex   int
	Cycle        int
	Retry        int
	GameID       string
}

// SupervisorRunResult is the terminal result of one complete worker unit.
type SupervisorRunResult struct {
	Disposition QueueRunDisposition
	Reason      string
	// SafeToExit confirms that the run reached the verified Town boundary where
	// the supervisor may execute an orderly Save & Exit.
	SafeToExit bool
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
	RunInstanceID string
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
	GameID              string
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
	gameOpen            bool
	gameSequence        int
	gameID              string
	pendingWrap         bool
	revalidateNext      bool
	startedAt           time.Time
	now                 func() time.Time
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

// StartQueue starts a validated runtime-only queue and owns its immutable plan until termination.
func (s *SessionSupervisor) StartQueue(meta SupervisorCommandMeta, plan FarmQueuePlan) (SupervisorSnapshot, error) {
	return s.startQueue(meta, plan, farmQueuePayload(plan))
}

func (s *SessionSupervisor) startQueue(meta SupervisorCommandMeta, plan FarmQueuePlan, payload string) (SupervisorSnapshot, error) {
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
	queue, err := validateUniqueQueueRunIDs(plan.RunIDs)
	if err != nil {
		return s.snapshotLocked(), err
	}
	plan.RunIDs = queue
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
	s.gameOpen = false
	s.gameSequence = 0
	s.gameID = ""
	s.pendingWrap = false
	s.revalidateNext = false
	s.startedAt = s.now()
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
	// An orderly stop is stronger than a pause. Once requested it cannot be
	// downgraded by a later hotkey or duplicate browser action.
	if s.intent == SupervisorIntentStopAfterRun && intent == SupervisorIntentPauseAfterRun {
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
	s.revalidateNext = true
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
	if _, sameGame := s.runner.(FarmQueueLifecycleRunner); sameGame && !s.gameOpen {
		s.state = SupervisorStateStartingGame
	} else {
		s.state = SupervisorStateStartingRun
	}
	s.generation++
	s.startedRuns++
	generation := s.generation
	runID := s.plan.RunIDs[s.queueIndex]
	s.request = SupervisorRunRequest{DefinitionID: runID, ExecutionID: telemetry.NewRunID(runID), QueueIndex: s.queueIndex, Cycle: s.cycle, Retry: s.retry, GameID: s.gameID}
	request := s.request
	go s.runWorker(ctx, generation, request)
}

func (s *SessionSupervisor) runWorker(ctx context.Context, generation uint64, request SupervisorRunRequest) {
	lifecycle, sameGame := s.runner.(FarmQueueLifecycleRunner)
	if sameGame {
		s.runLifecycleWorker(ctx, request, lifecycle)
		return
	}
	s.runRunnerWithoutLifecycle(ctx, generation, request)
}

func (s *SessionSupervisor) runLifecycleWorker(ctx context.Context, request SupervisorRunRequest, lifecycle FarmQueueLifecycleRunner) {
	s.mu.Lock()
	revalidate := s.revalidateNext
	pendingWrap := s.pendingWrap
	s.revalidateNext = false
	s.mu.Unlock()

	if revalidate {
		if err := lifecycle.RevalidateGame(ctx, request); err != nil {
			s.finishLifecycleFailure(lifecycle, SupervisorReasonPausedGameLost)
			return
		}
		if pendingWrap {
			if !s.exitLifecycleGame(ctx, lifecycle, request, "queue_wrap_after_pause", lifecycleExitResumeWrap) {
				return
			}
			s.mu.Lock()
			request.Cycle = s.cycle
			s.mu.Unlock()
		}
	}

	s.mu.Lock()
	needsGame := !s.gameOpen
	if needsGame {
		s.state = SupervisorStateStartingGame
		s.gameSequence++
		s.gameID = fmt.Sprintf("game-%03d", s.gameSequence)
		request.GameID = s.gameID
		s.request = request
		s.generation++
	}
	s.mu.Unlock()
	if needsGame {
		if err := lifecycle.StartGame(ctx, request); err != nil {
			s.finishLifecycleFailure(lifecycle, SupervisorReasonGameStartFailed)
			return
		}
		s.mu.Lock()
		if s.state == SupervisorStateCancelling || ctx.Err() != nil || s.shutdown {
			s.finishQueueLocked(SupervisorStateIdle, emergencyStopResult(), true)
			s.mu.Unlock()
			return
		}
		s.gameOpen = true
		s.request = request
		s.state = SupervisorStateStartingRun
		s.generation++
		s.mu.Unlock()
	} else {
		s.mu.Lock()
		request.GameID = s.gameID
		s.request = request
		s.mu.Unlock()
	}

	s.mu.Lock()
	if s.state == SupervisorStateCancelling || ctx.Err() != nil || s.shutdown {
		s.finishQueueLocked(SupervisorStateIdle, emergencyStopResult(), true)
		s.mu.Unlock()
		return
	}
	s.state = SupervisorStateRunningRun
	s.generation++
	s.mu.Unlock()

	result := runSupervisorUnit(ctx, s.runner, request)
	s.completeLifecycleRun(ctx, lifecycle, request, result)
}

type lifecycleExitAction uint8

const (
	lifecycleExitFinishIdle lifecycleExitAction = iota
	lifecycleExitFinishError
	lifecycleExitWrap
	lifecycleExitRetry
	lifecycleExitResumeWrap
)

func (s *SessionSupervisor) completeLifecycleRun(ctx context.Context, lifecycle FarmQueueLifecycleRunner, request SupervisorRunRequest, result SupervisorRunResult) {
	s.mu.Lock()
	s.result = result
	if s.state == SupervisorStateCancelling || ctx.Err() != nil || s.shutdown || result.Reason == string(SupervisorReasonEmergencyStopRequested) {
		s.finishQueueLocked(SupervisorStateIdle, emergencyStopResult(), true)
		s.mu.Unlock()
		return
	}
	switch result.Disposition {
	case QueueRunAdvance:
		if !result.SafeToExit {
			s.finishQueueLocked(SupervisorStateStoppedError, SupervisorRunResult{Disposition: QueueRunStop, Reason: "run_town_handoff_unconfirmed"}, false)
			s.mu.Unlock()
			return
		}
		s.consecutiveFailures = 0
		s.retry = 0
		nextIndex := s.queueIndex + 1
		wrap := nextIndex == len(s.plan.RunIDs)
		if wrap {
			nextIndex = 0
		}
		s.queueIndex = nextIndex
		budgetReason := s.exhaustedBudgetLocked()
		intent := s.intent
		if intent == SupervisorIntentStopAfterRun || budgetReason != "" {
			reason := "stop_after_run"
			terminal := SupervisorRunResult{Disposition: QueueRunStop, Reason: reason, SafeToExit: true}
			if budgetReason != "" {
				reason = budgetReason
				terminal = SupervisorRunResult{Disposition: QueueRunStop, Reason: budgetReason, SafeToExit: true}
			}
			s.result = terminal
			s.mu.Unlock()
			s.exitLifecycleGame(ctx, lifecycle, request, reason, lifecycleExitFinishIdle)
			return
		}
		if intent == SupervisorIntentPauseAfterRun {
			s.pendingWrap = wrap
			s.state = SupervisorStatePausedBetweenRuns
			s.intent = SupervisorIntentNone
			s.cancel = nil
			s.request = SupervisorRunRequest{}
			s.generation++
			s.mu.Unlock()
			return
		}
		if wrap {
			s.mu.Unlock()
			s.exitLifecycleGame(ctx, lifecycle, request, "queue_wrap", lifecycleExitWrap)
			return
		}
		s.cancel = nil
		s.startWorkerLocked()
		s.mu.Unlock()
	case QueueRunRetryCurrent:
		s.consecutiveFailures++
		budgetReason := s.exhaustedBudgetLocked()
		exhaustedRetry := s.consecutiveFailures > s.plan.Budgets.MaxConsecutiveFailures || s.totalRestarts >= s.plan.Budgets.MaxTotalRestarts
		if !result.SafeToExit {
			s.finishQueueLocked(SupervisorStateStoppedError, result, false)
			s.mu.Unlock()
			return
		}
		if budgetReason != "" {
			s.result = SupervisorRunResult{Disposition: QueueRunStop, Reason: budgetReason, SafeToExit: true}
			s.mu.Unlock()
			s.exitLifecycleGame(ctx, lifecycle, request, budgetReason, lifecycleExitFinishIdle)
			return
		}
		if exhaustedRetry {
			s.mu.Unlock()
			s.exitLifecycleGame(ctx, lifecycle, request, result.Reason, lifecycleExitFinishError)
			return
		}
		s.totalRestarts++
		s.retry++
		s.mu.Unlock()
		s.exitLifecycleGame(ctx, lifecycle, request, "retry_current", lifecycleExitRetry)
	case QueueRunStop:
		if result.SafeToExit {
			s.mu.Unlock()
			s.exitLifecycleGame(ctx, lifecycle, request, result.Reason, lifecycleExitFinishError)
			return
		}
		s.finishQueueLocked(SupervisorStateStoppedError, result, false)
		s.mu.Unlock()
	default:
		s.finishQueueLocked(SupervisorStateStoppedError, SupervisorRunResult{Disposition: QueueRunStop, Reason: "invalid_queue_disposition"}, false)
		s.mu.Unlock()
	}
}

func (s *SessionSupervisor) exitLifecycleGame(ctx context.Context, lifecycle FarmQueueLifecycleRunner, request SupervisorRunRequest, reason string, action lifecycleExitAction) bool {
	s.mu.Lock()
	s.state = SupervisorStateExitingGame
	s.generation++
	s.mu.Unlock()
	err := lifecycle.ExitGame(ctx, request, reason)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state == SupervisorStateCancelling || ctx.Err() != nil || s.shutdown {
		s.finishQueueLocked(SupervisorStateIdle, emergencyStopResult(), true)
		return false
	}
	if err != nil {
		s.finishQueueLocked(SupervisorStateStoppedError, SupervisorRunResult{Disposition: QueueRunStop, Reason: string(SupervisorReasonGameExitFailed)}, false)
		return false
	}
	s.gameOpen = false
	s.gameID = ""
	if action != lifecycleExitResumeWrap {
		s.cancel = nil
	}
	s.pendingWrap = false
	s.revalidateNext = false
	switch action {
	case lifecycleExitFinishIdle:
		s.finishQueueLocked(SupervisorStateIdle, s.result, false)
	case lifecycleExitFinishError:
		s.finishQueueLocked(SupervisorStateStoppedError, s.result, false)
	case lifecycleExitWrap, lifecycleExitResumeWrap:
		s.cycle++
		if action == lifecycleExitResumeWrap {
			return true
		}
		s.startWorkerLocked()
	case lifecycleExitRetry:
		s.startWorkerLocked()
	}
	return true
}

func (s *SessionSupervisor) finishLifecycleFailure(_ FarmQueueLifecycleRunner, reason SupervisorReason) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state == SupervisorStateCancelling || s.shutdown {
		s.finishQueueLocked(SupervisorStateIdle, emergencyStopResult(), true)
		return
	}
	s.finishQueueLocked(SupervisorStateStoppedError, SupervisorRunResult{Disposition: QueueRunStop, Reason: string(reason)}, false)
}

// runRunnerWithoutLifecycle keeps the supervisor scheduler independently testable
// without granting this adapter ownership of game start or Save & Exit. Production
// CLI and dashboard wiring always provide a [FarmQueueLifecycleRunner].
func (s *SessionSupervisor) runRunnerWithoutLifecycle(ctx context.Context, generation uint64, request SupervisorRunRequest) {
	s.mu.Lock()
	if s.generation == generation && s.state == SupervisorStateStartingRun {
		s.state = SupervisorStateRunningRun
		s.generation++
	}
	s.mu.Unlock()
	result := runSupervisorUnit(ctx, s.runner, request)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.result = result
	s.cancel = nil
	if s.state == SupervisorStateCancelling || ctx.Err() != nil || s.shutdown {
		s.finishQueueLocked(SupervisorStateIdle, emergencyStopResult(), true)
		return
	}
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
			if reason := s.exhaustedBudgetLocked(); reason != "" {
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

func runSupervisorUnit(ctx context.Context, runner SupervisorRunner, request SupervisorRunRequest) (result SupervisorRunResult) {
	defer func() {
		if recovered := recover(); recovered != nil {
			result = SupervisorRunResult{Disposition: QueueRunStop, Reason: string(SupervisorReasonWorkerPanic)}
		}
	}()
	return runner.Run(ctx, request)
}

func emergencyStopResult() SupervisorRunResult {
	return SupervisorRunResult{Disposition: QueueRunStop, Reason: string(SupervisorReasonEmergencyStopRequested)}
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
		Generation: s.generation, State: s.state, PendingIntent: s.intent, ActiveRunID: s.request.DefinitionID, RunInstanceID: s.request.ExecutionID,
		QueueKnown: true, Queue: append([]string(nil), s.plan.RunIDs...), QueueIndex: s.queueIndex, Cycle: s.cycle, Retry: s.retry,
		StartedRuns: s.startedRuns, ConsecutiveFailures: s.consecutiveFailures, TotalRestarts: s.totalRestarts,
		Budgets: s.plan.Budgets, LastResult: s.result, GameID: s.gameID,
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
	if lifecycle, ok := s.runner.(FarmQueueLifecycleRunner); ok {
		if finisher, ok := s.runner.(farmQueueLifecycleFinisher); ok {
			if err := finisher.FinishQueue(result, state); err != nil {
				state = SupervisorStateStoppedError
				result = SupervisorRunResult{Disposition: QueueRunStop, Reason: "telemetry_failed"}
			}
		}
		lifecycle.CloseQueue()
	}
	s.state = state
	s.result = result
	s.intent = SupervisorIntentNone
	s.cancel = nil
	s.request = SupervisorRunRequest{}
	s.gameOpen = false
	s.gameID = ""
	s.pendingWrap = false
	s.revalidateNext = false
	if clear {
		s.plan = FarmQueuePlan{}
		s.queueIndex = 0
		s.cycle = 0
		s.retry = 0
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
