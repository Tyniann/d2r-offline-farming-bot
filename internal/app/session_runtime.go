package app

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/tasks"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/telemetry"
)

// RunSession executes one finite autonomous offline session using the validated
// Phase-7 start, run, exit, recovery, and telemetry contracts.
func (rt *Runtime) RunSession() error {
	sessionTrace, err := telemetry.NewSessionRecorder(rt.Config.Telemetry.Directory)
	if err != nil {
		return err
	}
	defer sessionTrace.Close()
	policy := newSessionRecoveryPolicy(rt.Config.Session.RetryClasses, rt.Config.Session.MaxConsecutiveFailures, rt.Config.Session.MaxTotalRestarts)
	recovery := &sessionRecoveryCoordinator{policy: policy, emitter: sessionTrace}
	startedAt := time.Now()
	if err := sessionTrace.Emit(telemetry.Event{Event: telemetry.SessionStarted, Run: rt.Config.Session.Run, MaxRuns: rt.Config.Session.MaxRuns, MaxDurationMs: int64(rt.Config.Session.MaxDurationMs)}); err != nil {
		return err
	}
	rt.Log.Info("autonomous session started", "session_id", sessionTrace.SessionID(), "max_runs", rt.Config.Session.MaxRuns, "difficulty", rt.Config.Session.Difficulty, "character", rt.Config.Session.Character)

	for ordinal := 1; ordinal <= rt.Config.Session.MaxRuns; ordinal++ {
		if time.Since(startedAt) >= time.Duration(rt.Config.Session.MaxDurationMs)*time.Millisecond {
			return recovery.emitTerminal(telemetry.SessionCompleted, "max_duration", time.Since(startedAt).Milliseconds())
		}
		if err := rt.sessionReset.ResetForNextGame(time.Now(), "session_cycle_start"); err != nil {
			_ = recovery.emitTerminal(telemetry.SessionFailed, "cycle_reset_failed", time.Since(startedAt).Milliseconds())
			return err
		}
		rt.Options.OfflineDifficulty = rt.Config.Session.Difficulty
		rt.Options.OfflineCharacter = rt.Config.Session.Character
		if err := rt.RunOfflineDifficultyTest(rt.Config.Session.Difficulty); err != nil {
			_ = recovery.emitTerminal(telemetry.SessionFailed, "start_game_failed", time.Since(startedAt).Milliseconds())
			return err
		}
		rt.Options.OfflineDifficulty, rt.Options.OfflineCharacter = "", ""
		gameID := fmt.Sprintf("%s-game-%03d", sessionTrace.SessionID(), ordinal)
		runID, err := rt.prepareSessionRun()
		if err != nil {
			_ = recovery.emitTerminal(telemetry.SessionFailed, "run_creation_failed", time.Since(startedAt).Milliseconds())
			return err
		}
		if err := sessionTrace.Emit(telemetry.Event{Event: telemetry.GameStarted, GameID: gameID, RunID: runID, Run: rt.Config.Session.Run, RunOrdinal: ordinal}); err != nil {
			return err
		}
		if err := sessionTrace.Emit(telemetry.Event{Event: telemetry.RunStarted, GameID: gameID, RunID: runID, Run: rt.Config.Session.Run, RunOrdinal: ordinal}); err != nil {
			return err
		}
		runStarted := time.Now()
		result, err := rt.runTaskToTerminal()
		if closeErr := rt.closeSessionRunTelemetry(); err == nil && closeErr != nil {
			err = closeErr
		}
		if err != nil {
			_ = recovery.emitTerminal(telemetry.SessionFailed, "run_runtime_failed", time.Since(startedAt).Milliseconds())
			return err
		}
		runResult := sessionRunResult{Outcome: sessionRunSuccess, Step: result.Step, Reason: result.Reason}
		if result.Outcome != tasks.RunOutcomeSuccess {
			runResult.Outcome = sessionRunFailed
			if classifySessionFailure(result.Reason) == sessionFailureRestartable {
				runResult.Outcome = sessionRunAborted
			}
		}
		stuck, _ := rt.routePlayback.failureContext()
		decision, err := recovery.handle(runResult, sessionRunContext{GameID: gameID, RunID: runID, Run: rt.Config.Session.Run, Ordinal: ordinal, ElapsedMs: time.Since(runStarted).Milliseconds(), Stuck: stuck})
		if err != nil {
			return err
		}
		if err := rt.RunOfflineExitTest(); err != nil {
			_ = recovery.emitTerminal(telemetry.SessionFailed, "exit_game_failed", time.Since(startedAt).Milliseconds())
			return err
		}
		if decision == sessionRecoveryTerminal {
			return recovery.emitTerminal(telemetry.SessionFailed, runResult.Reason, time.Since(startedAt).Milliseconds())
		}
		if ordinal == rt.Config.Session.MaxRuns {
			if err := recovery.emitTerminal(telemetry.SessionCompleted, "max_runs", time.Since(startedAt).Milliseconds()); err != nil {
				return err
			}
			rt.Log.Info("autonomous session completed", "session_id", sessionTrace.SessionID(), "runs", ordinal)
			return nil
		}
		if err := waitSessionCooldown(context.Background(), time.Duration(rt.Config.Session.CooldownMs)*time.Millisecond); err != nil {
			return err
		}
	}
	return nil
}

func (rt *Runtime) prepareSessionRun() (string, error) {
	trace, err := telemetry.New(rt.Config.Telemetry.Directory, rt.Config.Session.Run, "")
	if err != nil {
		return "", err
	}
	rt.Telemetry = trace
	rt.routePlayback.setTelemetry(trace)
	rt.lootActions.setTelemetry(trace)
	rt.Tasks = tasks.NewRunner(rt.Log, rt.sessionSelection, rt.runConfig, rt.taskDeps)
	return trace.RunID(), nil
}

func (rt *Runtime) closeSessionRunTelemetry() error {
	if rt.Telemetry == nil {
		return nil
	}
	err := rt.Telemetry.Close()
	rt.Telemetry = nil
	rt.routePlayback.setTelemetry(nil)
	rt.lootActions.setTelemetry(nil)
	return err
}

func (rt *Runtime) runTaskToTerminal() (tasks.TickResult, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	rt.startShutdownSignals(ctx, cancel)
	defer rt.Process.Detach()
	defer rt.Input.Unbind()
	hotkeys, err := rt.startHotkeys(ctx)
	if err != nil {
		return tasks.TickResult{}, err
	}
	defer rt.stopHotkeys(cancel)
	ticker := time.NewTicker(time.Duration(rt.Config.Runtime.PollIntervalMs) * time.Millisecond)
	defer ticker.Stop()
	state := &runState{}
	for {
		select {
		case <-ctx.Done():
			return tasks.TickResult{}, ctx.Err()
		case event := <-hotkeys:
			rt.handleHotkeyEvent(event, cancel)
		case <-ticker.C:
			if err := rt.runTick(ctx, state); err != nil && !errors.Is(err, context.Canceled) {
				return tasks.TickResult{}, err
			}
			if rt.Tasks.Terminal() {
				return rt.Tasks.Result(), nil
			}
		}
	}
}
