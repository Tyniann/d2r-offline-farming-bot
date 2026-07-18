package app

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/profile"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/tasks"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/telemetry"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

type queueRunUnit interface {
	StartOrVerifyGame(context.Context, bool) error
	VerifySameGame(context.Context) error
	RunToTown(context.Context, SupervisorRunRequest, bool) SupervisorRunResult
	ExitGame(context.Context) error
	Close()
}

type queueRunUnitFactory func(string) (queueRunUnit, error)

// CanAdoptQueueGame reports whether a queue may verify and reuse the passive
// monitor's current game instead of entering the character-screen start flow.
// This is only a branch-selection hint: [RuntimeQueueRunner.StartGame] still
// proves character and Rogue Encampment through Memory before any run input.
func CanAdoptQueueGame(state SupervisorState, runtime UIStatusSnapshot) bool {
	if state == SupervisorStateIdleInGame {
		return true
	}
	return state == SupervisorStateIdle &&
		runtime.ProcessState == "attached" &&
		runtime.WindowBound &&
		runtime.WorldValid &&
		runtime.WorldPhase == world.GamePhaseInGame.String() &&
		runtime.AreaID == uint32(world.RogueEncampment)
}

// FarmQueueLifecycleRunner extends one-run execution with supervisor-owned
// game boundaries. Save & Exit must remain outside [SupervisorRunner.Run].
type FarmQueueLifecycleRunner interface {
	SupervisorRunner
	StartGame(context.Context, SupervisorRunRequest) error
	RevalidateGame(context.Context, SupervisorRunRequest) error
	ExitGame(context.Context, SupervisorRunRequest, string) error
	CloseQueue()
}

// RuntimeQueueRunner owns the production game boundary while creating fresh
// run-specific state for every queue entry. Closing one Go runtime never ends
// the D2R game; only [RuntimeQueueRunner.ExitGame] may send Save & Exit.
type RuntimeQueueRunner struct {
	mu            sync.Mutex
	controlMu     sync.RWMutex
	config        *config.Config
	publish       func(UIStatusSnapshot)
	initialInGame bool
	gameOpen      bool
	unitRunID     string
	unit          queueRunUnit
	newUnit       queueRunUnitFactory
	sessionTrace  *telemetry.SessionRecorder
	persistEvents bool
	pauseAfterRun func() error
	stopAfterRun  func() error
	runsInGame    int
}

// SetStopAfterRunHandler routes the configured orderly-stop hotkey to the
// supervisor without cancelling the active run or releasing input ownership.
func (r *RuntimeQueueRunner) SetStopAfterRunHandler(handler func() error) {
	if r == nil {
		return
	}
	r.controlMu.Lock()
	r.stopAfterRun = handler
	r.controlMu.Unlock()
}

// SetPauseAfterRunHandler routes the configured Pause hotkey to the long-lived
// supervisor intent while a queue worker is active. It deliberately consumes
// the key so route playback is never suspended mid-input.
func (r *RuntimeQueueRunner) SetPauseAfterRunHandler(handler func() error) {
	if r == nil {
		return
	}
	r.controlMu.Lock()
	r.pauseAfterRun = handler
	r.controlMu.Unlock()
}

func (r *RuntimeQueueRunner) requestPauseAfterRun() error {
	r.controlMu.RLock()
	handler := r.pauseAfterRun
	r.controlMu.RUnlock()
	if handler == nil {
		return fmt.Errorf("pause-after-run handler is not configured")
	}
	return handler()
}

func (r *RuntimeQueueRunner) requestStopAfterRun() error {
	r.controlMu.RLock()
	handler := r.stopAfterRun
	r.controlMu.RUnlock()
	if handler == nil {
		return fmt.Errorf("stop-after-run handler is not configured")
	}
	return handler()
}

// NewRuntimeQueueRunner creates the sole production owner for queue game and
// run boundaries. The factory never crosses the app package boundary.
func NewRuntimeQueueRunner(cfg *config.Config, publish func(UIStatusSnapshot)) (*RuntimeQueueRunner, error) {
	if cfg == nil {
		return nil, fmt.Errorf("runtime queue runner requires config")
	}
	runner := &RuntimeQueueRunner{config: cfg, publish: publish, initialInGame: true, persistEvents: true}
	runner.newUnit = runner.newRuntimeUnit
	return runner, nil
}

// BeginQueue resets the per-queue game boundary. Only a Memory-confirmed
// `idle_in_game` start may consume an already open game.
func (r *RuntimeQueueRunner) BeginQueue(initialInGame bool) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.closeUnitLocked()
	r.initialInGame = initialInGame
	r.gameOpen = false
	r.runsInGame = 0
}

// StartGame starts and verifies one game, or consumes exactly the confirmed
// game supplied by `idle_in_game`. It never executes a farming run.
func (r *RuntimeQueueRunner) StartGame(ctx context.Context, request SupervisorRunRequest) error {
	if r == nil {
		return fmt.Errorf("queue runtime missing")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.runsInGame = 0
	unit, err := r.ensureUnitLocked(request.RunID)
	if err != nil {
		return err
	}
	if r.persistEvents && r.sessionTrace == nil {
		r.sessionTrace, err = telemetry.NewSessionRecorder(r.config.Telemetry.Directory)
		if err != nil {
			return fmt.Errorf("start queue telemetry: %w", err)
		}
		if err := r.sessionTrace.Emit(telemetry.Event{Event: telemetry.SessionStarted, MaxRuns: r.config.Session.MaxRuns, MaxDurationMs: int64(r.config.Session.MaxDurationMs)}); err != nil {
			return fmt.Errorf("emit queue session start: %w", err)
		}
	}
	if err := unit.StartOrVerifyGame(ctx, r.initialInGame); err != nil {
		return fmt.Errorf("start queue game: %w", err)
	}
	if r.sessionTrace != nil {
		if err := r.sessionTrace.Emit(telemetry.Event{Event: telemetry.GameStarted, GameID: request.GameID, RunID: request.ExecutionID, Run: request.RunID}); err != nil {
			return fmt.Errorf("emit queue game start: %w", err)
		}
	}
	r.initialInGame = false
	r.gameOpen = true
	return nil
}

// RevalidateGame proves that a paused queue still owns the same safe open game
// before any next-run input is allowed.
func (r *RuntimeQueueRunner) RevalidateGame(ctx context.Context, request SupervisorRunRequest) error {
	if r == nil {
		return fmt.Errorf("queue runtime missing")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.gameOpen {
		return fmt.Errorf("%s", SupervisorReasonPausedGameLost)
	}
	unit, err := r.ensureUnitLocked(request.RunID)
	if err != nil {
		return err
	}
	if err := unit.VerifySameGame(ctx); err != nil {
		return fmt.Errorf("%s: %w", SupervisorReasonPausedGameLost, err)
	}
	return nil
}

// Run executes exactly one fresh run through loot and the safe Town handoff.
// Game start and Save & Exit are deliberately outside this method.
func (r *RuntimeQueueRunner) Run(ctx context.Context, request SupervisorRunRequest) SupervisorRunResult {
	if r == nil {
		return SupervisorRunResult{Disposition: QueueRunStop, Reason: "queue_runtime_missing"}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.gameOpen {
		return SupervisorRunResult{Disposition: QueueRunStop, Reason: "queue_game_not_active"}
	}
	unit, err := r.ensureUnitLocked(request.RunID)
	if err != nil {
		return queueRuntimeTerminal(fmt.Errorf("initialize queue run %q: %w", request.RunID, err))
	}
	if err := unit.VerifySameGame(ctx); err != nil {
		return queueRuntimeTerminal(fmt.Errorf("verify queue game: %w", err))
	}
	if r.persistEvents && r.sessionTrace == nil {
		return SupervisorRunResult{Disposition: QueueRunStop, Reason: "telemetry_failed"}
	}
	if r.sessionTrace != nil {
		if err := r.sessionTrace.Emit(telemetry.Event{Event: telemetry.RunStarted, GameID: request.GameID, RunID: request.ExecutionID, Run: request.RunID}); err != nil {
			return SupervisorRunResult{Disposition: QueueRunStop, Reason: "telemetry_failed"}
		}
	}
	result := unit.RunToTown(ctx, request, r.runsInGame > 0)
	r.runsInGame++
	event := telemetry.RunFailed
	switch result.Disposition {
	case QueueRunAdvance:
		event = telemetry.RunCompleted
	case QueueRunRetryCurrent:
		event = telemetry.RunAborted
	}
	if r.sessionTrace != nil {
		if err := r.sessionTrace.Emit(telemetry.Event{Event: event, GameID: request.GameID, RunID: request.ExecutionID, Run: request.RunID, Reason: result.Reason}); err != nil {
			return SupervisorRunResult{Disposition: QueueRunStop, Reason: "telemetry_failed", SafeToExit: result.SafeToExit}
		}
	}
	return result
}

// ExitGame performs the one supervisor-authorized Save-&-Exit boundary. Calls
// after a confirmed exit are idempotent and send no additional input.
func (r *RuntimeQueueRunner) ExitGame(ctx context.Context, request SupervisorRunRequest, reason string) error {
	if r == nil {
		return fmt.Errorf("queue runtime missing")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.gameOpen {
		return nil
	}
	unit, err := r.ensureUnitLocked(request.RunID)
	if err != nil {
		return err
	}
	if err := unit.ExitGame(ctx); err != nil {
		return fmt.Errorf("exit queue game (%s): %w", reason, err)
	}
	if r.sessionTrace != nil {
		if err := r.sessionTrace.Emit(telemetry.Event{Event: telemetry.GameExited, GameID: request.GameID, RunID: request.ExecutionID, Run: request.RunID, Reason: reason}); err != nil {
			return fmt.Errorf("emit queue game exit: %w", err)
		}
	}
	r.gameOpen = false
	r.runsInGame = 0
	return nil
}

// CloseQueue releases current run resources without sending D2R input.
func (r *RuntimeQueueRunner) CloseQueue() {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.closeUnitLocked()
	if r.sessionTrace != nil {
		_ = r.sessionTrace.Close()
		r.sessionTrace = nil
	}
	r.gameOpen = false
	r.runsInGame = 0
}

func (r *RuntimeQueueRunner) ensureUnitLocked(runID string) (queueRunUnit, error) {
	if r.unit != nil && r.unitRunID == runID {
		return r.unit, nil
	}
	r.closeUnitLocked()
	if r.newUnit == nil {
		return nil, fmt.Errorf("queue run unit factory is required")
	}
	unit, err := r.newUnit(runID)
	if err != nil {
		return nil, err
	}
	if unit == nil {
		return nil, fmt.Errorf("queue run unit factory returned nil")
	}
	r.unit = unit
	r.unitRunID = runID
	return unit, nil
}

func (r *RuntimeQueueRunner) closeUnitLocked() {
	if r.unit != nil {
		r.unit.Close()
	}
	r.unit = nil
	r.unitRunID = ""
}

func (r *RuntimeQueueRunner) newRuntimeUnit(runID string) (queueRunUnit, error) {
	cfg := *r.config
	cfg.Session = r.config.Session
	cfg.Session.Enabled = true
	cfg.Session.Run = runID
	runtime, err := New(&cfg, Options{})
	if err != nil {
		return nil, err
	}
	runtime.SetUIStatusPublisher(r.publish)
	runtime.setPauseHotkeyHandler(r.requestPauseAfterRun)
	runtime.setStopAfterRunHotkeyHandler(r.requestStopAfterRun)
	return &runtimeQueueUnit{runtime: runtime}, nil
}

type runtimeQueueUnit struct {
	runtime *Runtime
}

func (u *runtimeQueueUnit) StartOrVerifyGame(ctx context.Context, alreadyActive bool) error {
	u.runtime.Log.Info("queue game lifecycle start", "adopt_existing_game", alreadyActive)
	if !alreadyActive {
		u.runtime.Options.OfflineDifficulty = u.runtime.Config.Session.Difficulty
		u.runtime.Options.OfflineCharacter = u.runtime.Config.Session.Character
		if err := u.runtime.runOfflineDifficultyTest(ctx, u.runtime.Config.Session.Difficulty); err != nil {
			return err
		}
		u.runtime.Options.OfflineDifficulty, u.runtime.Options.OfflineCharacter = "", ""
	}
	return u.runtime.verifyActiveQueueGame(ctx)
}

func (u *runtimeQueueUnit) VerifySameGame(ctx context.Context) error {
	return u.runtime.verifyActiveQueueGame(ctx)
}

func (u *runtimeQueueUnit) RunToTown(ctx context.Context, _ SupervisorRunRequest, sameGameContinuation bool) SupervisorRunResult {
	u.runtime.Tasks.Reset("queue_run_start")
	if sameGameContinuation {
		// The prior run's verified Town handoff replaces the new-game settle
		// delay. The hook action itself remains run-scoped and still executes.
		u.runtime.Profile.SkipInitialDelay(profile.HookTownReady)
	}
	if _, err := u.runtime.prepareSessionRun(); err != nil {
		return queueRuntimeTerminal(fmt.Errorf("prepare queue run: %w", err))
	}
	taskResult, runErr := u.runtime.runTaskToTerminal(ctx)
	if closeErr := u.runtime.closeSessionRunTelemetry(); runErr == nil && closeErr != nil {
		runErr = closeErr
	}
	if runErr != nil {
		return queueRuntimeTerminal(fmt.Errorf("execute queue run: %w", runErr))
	}
	if taskResult.Outcome == tasks.RunOutcomeSuccess {
		return SupervisorRunResult{Disposition: QueueRunAdvance, SafeToExit: true}
	}
	if isRestartableSessionFailure(taskResult.Reason, u.runtime.Config.Session.RetryClasses) {
		return SupervisorRunResult{Disposition: QueueRunRetryCurrent, Reason: taskResult.Reason}
	}
	return SupervisorRunResult{Disposition: QueueRunStop, Reason: taskResult.Reason}
}

func (u *runtimeQueueUnit) ExitGame(ctx context.Context) error {
	return u.runtime.runOfflineExitTest(ctx)
}

func (u *runtimeQueueUnit) Close() {
	if u == nil || u.runtime == nil {
		return
	}
	_ = u.runtime.closeSessionRunTelemetry()
	if err := u.runtime.CloseLog(); err != nil {
		u.runtime.Log.Warn("queue runtime log close failed", "error", err)
	}
}

func (rt *Runtime) verifyActiveQueueGame(parent context.Context) error {
	ctx, cancel := context.WithTimeout(parent, time.Duration(rt.Config.Session.StateTimeoutMs)*time.Millisecond)
	defer cancel()
	rt.startShutdownSignals(ctx, cancel)
	defer func() {
		rt.Input.Unbind()
		_ = rt.Process.Detach()
	}()
	hotkeys, err := rt.startHotkeys(ctx)
	if err != nil {
		return err
	}
	defer rt.stopHotkeys(cancel)
	verifier := newSessionGameVerifier(sessionGameExpectation{
		Character: rt.Config.Session.Character, GameVersion: rt.Config.Memory.GameVersion, StartArea: world.RogueEncampment,
	})
	verifier.ResetForNextGame()
	state := &runState{}
	ticker := time.NewTicker(time.Duration(rt.Config.Runtime.PollIntervalMs) * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case event := <-hotkeys:
			rt.handleHotkeyEvent(event, cancel)
		case <-ticker.C:
			if err := rt.runTick(ctx, state); err != nil && !errors.Is(err, context.Canceled) {
				return err
			}
			if _, confirmed, err := verifier.Observe(rt.World.Current(), rt.Config.Memory.GameVersion); err != nil {
				return err
			} else if confirmed {
				if err := focusVerifiedQueueGame(rt.Input); err != nil {
					return err
				}
				return nil
			}
		}
	}
}

func focusVerifiedQueueGame(controller inputController) error {
	if err := controller.Focus(); err != nil {
		return fmt.Errorf("focus verified queue game: %w", err)
	}
	return nil
}

func queueRuntimeTerminal(err error) SupervisorRunResult {
	if errors.Is(err, context.Canceled) {
		return SupervisorRunResult{Disposition: QueueRunStop, Reason: string(SupervisorReasonEmergencyStopRequested)}
	}
	return SupervisorRunResult{Disposition: QueueRunStop, Reason: err.Error()}
}
