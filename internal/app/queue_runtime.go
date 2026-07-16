package app

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/tasks"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

type queueUnitExecutor func(context.Context, SupervisorRunRequest, bool) (SupervisorRunResult, bool)

// RuntimeQueueRunner creates one run-specific Runtime for every queue entry.
// The first worker consumes the game confirmed by selection; every later
// worker starts a fresh game after the preceding worker completed Save & Exit.
type RuntimeQueueRunner struct {
	mu            sync.Mutex
	controlMu     sync.RWMutex
	config        *config.Config
	publish       func(UIStatusSnapshot)
	initialInGame bool
	execute       queueUnitExecutor
	pauseAfterRun func() error
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

// NewRuntimeQueueRunner creates the production adapter from FarmQueue workers
// to exactly one existing Phase-10 run, Town and Save-&-Exit cycle.
func NewRuntimeQueueRunner(cfg *config.Config, publish func(UIStatusSnapshot)) (*RuntimeQueueRunner, error) {
	if cfg == nil {
		return nil, fmt.Errorf("runtime queue runner requires config")
	}
	runner := &RuntimeQueueRunner{config: cfg, publish: publish, initialInGame: true}
	runner.execute = runner.executeRuntimeUnit
	return runner, nil
}

// BeginQueue resets the per-queue game boundary. Only a Memory-confirmed
// `idle_in_game` start may consume an already open game.
func (r *RuntimeQueueRunner) BeginQueue(initialInGame bool) {
	if r == nil {
		return
	}
	r.mu.Lock()
	r.initialInGame = initialInGame
	r.mu.Unlock()
}

// Run executes one immutable queue entry. Runtime configuration is copied so
// selecting Countess or Mephisto never mutates YAML-backed process defaults.
func (r *RuntimeQueueRunner) Run(ctx context.Context, request SupervisorRunRequest) SupervisorRunResult {
	if r == nil {
		return SupervisorRunResult{Disposition: QueueRunStop, Reason: "queue_runtime_missing"}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	result, exited := r.execute(ctx, request, r.initialInGame)
	if exited {
		r.initialInGame = false
	}
	return result
}

func (r *RuntimeQueueRunner) executeRuntimeUnit(ctx context.Context, request SupervisorRunRequest, initialInGame bool) (result SupervisorRunResult, exited bool) {
	cfg := *r.config
	cfg.Session = r.config.Session
	cfg.Session.Enabled = true
	cfg.Session.Run = request.RunID
	runtime, err := New(&cfg, Options{})
	if err != nil {
		return queueRuntimeTerminal(fmt.Errorf("initialize queue run %q: %w", request.RunID, err)), false
	}
	defer func() {
		if closeErr := runtime.CloseLog(); closeErr != nil {
			runtime.Log.Warn("queue runtime log close failed", "error", closeErr)
		}
	}()
	defer func() {
		switch result.Disposition {
		case QueueRunAdvance:
			runtime.Log.Info("queue worker completed", "run", request.RunID, "disposition", result.Disposition)
		case QueueRunRetryCurrent:
			runtime.Log.Warn("queue worker requested retry", "run", request.RunID, "disposition", result.Disposition, "reason", result.Reason)
		case QueueRunStop:
			runtime.Log.Error("queue worker stopped", "run", request.RunID, "disposition", result.Disposition, "reason", result.Reason)
		}
	}()
	runtime.SetUIStatusPublisher(r.publish)
	runtime.setPauseHotkeyHandler(r.requestPauseAfterRun)
	runtime.Tasks.Reset("queue_game_verification")

	if !initialInGame {
		runtime.Options.OfflineDifficulty = cfg.Session.Difficulty
		runtime.Options.OfflineCharacter = cfg.Session.Character
		if err := runtime.runOfflineDifficultyTest(ctx, cfg.Session.Difficulty); err != nil {
			return queueRuntimeTerminal(fmt.Errorf("start queue game: %w", err)), false
		}
		runtime.Options.OfflineDifficulty, runtime.Options.OfflineCharacter = "", ""
	}
	if err := runtime.verifyActiveQueueGame(ctx); err != nil {
		return queueRuntimeTerminal(fmt.Errorf("verify queue game: %w", err)), false
	}
	if _, err := runtime.prepareSessionRun(); err != nil {
		return queueRuntimeTerminal(fmt.Errorf("prepare queue run: %w", err)), false
	}
	taskResult, runErr := runtime.runTaskToTerminal(ctx)
	if closeErr := runtime.closeSessionRunTelemetry(); runErr == nil && closeErr != nil {
		runErr = closeErr
	}
	if runErr != nil {
		return queueRuntimeTerminal(fmt.Errorf("execute queue run: %w", runErr)), false
	}
	runResult := sessionRunResult{Outcome: sessionRunSuccess, Step: taskResult.Step, Reason: taskResult.Reason}
	if taskResult.Outcome != tasks.RunOutcomeSuccess {
		runResult.Outcome = sessionRunFailed
		if classifySessionFailure(result.Reason) == sessionFailureRestartable {
			runResult.Outcome = sessionRunAborted
		}
	}
	if err := runtime.runOfflineExitTest(ctx); err != nil {
		return queueRuntimeTerminal(fmt.Errorf("exit queue game: %w", err)), false
	}
	if runResult.Outcome == sessionRunSuccess {
		return SupervisorRunResult{Disposition: QueueRunAdvance}, true
	}
	if runResult.Outcome == sessionRunAborted {
		return SupervisorRunResult{Disposition: QueueRunRetryCurrent, Reason: runResult.Reason}, true
	}
	return SupervisorRunResult{Disposition: QueueRunStop, Reason: runResult.Reason}, true
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
				return nil
			}
		}
	}
}

func queueRuntimeTerminal(err error) SupervisorRunResult {
	if errors.Is(err, context.Canceled) {
		return SupervisorRunResult{Disposition: QueueRunStop, Reason: string(SupervisorReasonEmergencyStopRequested)}
	}
	return SupervisorRunResult{Disposition: QueueRunStop, Reason: err.Error()}
}
