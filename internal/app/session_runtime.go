package app

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/tasks"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/telemetry"
)

// RunSession executes one finite autonomous offline session through the same
// long-lived command boundary used by future UI clients.
func (rt *Runtime) RunSession() error {
	supervisor, err := NewSessionSupervisor(runtimeSessionRunner{runtime: rt})
	if err != nil {
		return err
	}
	start, err := supervisor.Start(SupervisorCommandMeta{CommandID: "cli-session-start", ExpectedGeneration: 0}, SupervisorRunRequest{RunID: rt.Config.Session.Run})
	if err != nil {
		return err
	}
	if err := supervisor.Wait(context.Background()); err != nil {
		return err
	}
	result := supervisor.Snapshot().LastResult
	if result.Disposition != QueueRunAdvance {
		return fmt.Errorf("autonomous session stopped: %s", result.Reason)
	}
	rt.Log.Debug("CLI session supervisor completed", "generation", start.Generation, "run", rt.Config.Session.Run)
	return nil
}

type runtimeSessionRunner struct {
	runtime *Runtime
}

func (r runtimeSessionRunner) Run(ctx context.Context, _ SupervisorRunRequest) SupervisorRunResult {
	if err := r.runtime.runSession(ctx); err != nil {
		if errors.Is(err, context.Canceled) {
			return SupervisorRunResult{Disposition: QueueRunStop, Reason: string(SupervisorReasonEmergencyStopRequested)}
		}
		return SupervisorRunResult{Disposition: QueueRunStop, Reason: err.Error()}
	}
	return SupervisorRunResult{Disposition: QueueRunAdvance}
}

func (rt *Runtime) runSession(ctx context.Context) error {
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
		if err := rt.runOfflineDifficultyTest(ctx, rt.Config.Session.Difficulty); err != nil {
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
		if emitErr := sessionTrace.Emit(telemetry.Event{Event: telemetry.GameStarted, GameID: gameID, RunID: runID, Run: rt.Config.Session.Run, RunOrdinal: ordinal}); emitErr != nil {
			return emitErr
		}
		if emitErr := sessionTrace.Emit(telemetry.Event{Event: telemetry.RunStarted, GameID: gameID, RunID: runID, Run: rt.Config.Session.Run, RunOrdinal: ordinal}); emitErr != nil {
			return emitErr
		}
		runStarted := time.Now()
		result, err := rt.runTaskToTerminal(ctx)
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
		if err := rt.runOfflineExitTest(ctx); err != nil {
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
		if err := waitSessionCooldown(ctx, time.Duration(rt.Config.Session.CooldownMs)*time.Millisecond); err != nil {
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
	contextEvent, err := rt.sessionRunContextEvent()
	if err != nil {
		_ = trace.Close()
		return "", err
	}
	if err := trace.Emit(contextEvent); err != nil {
		_ = trace.Close()
		return "", fmt.Errorf("emit session run context: %w", err)
	}
	rt.Telemetry = trace
	rt.routePlayback.setTelemetry(trace)
	if rt.townEgress != nil {
		rt.townEgress.setTelemetry(trace)
	}
	rt.lootActions.setTelemetry(trace)
	// The runner owns a copy of taskDeps, so bind the new generation recorder
	// before constructing it. Updating only the feature adapters leaves shared
	// step and encounter telemetry disconnected and must fail closed.
	rt.taskDeps.Telemetry = trace
	if rt.townTelemetry != nil {
		rt.townTelemetry.setTelemetry(trace)
	}
	if rt.profileTelemetry != nil {
		rt.profileTelemetry.setTelemetry(trace)
	}
	rt.Tasks = tasks.NewRunner(rt.Log, rt.sessionSelection, rt.runConfig, rt.taskDeps)
	return trace.RunID(), nil
}

func (rt *Runtime) sessionRunContextEvent() (telemetry.Event, error) {
	plan, err := ResolveSessionPlan(rt.Config, Options{SessionInspect: true})
	if err != nil {
		return telemetry.Event{}, fmt.Errorf("resolve session run context: %w", err)
	}
	definition, ok := tasks.DefaultRunRegistry().Definition(tasks.RunID(rt.Config.Session.Run))
	if !ok {
		return telemetry.Event{}, fmt.Errorf("resolve session run context: %s: %q", tasks.RunReasonUnknown, rt.Config.Session.Run)
	}
	return telemetry.Event{
		Event:                  telemetry.RunContext,
		DefinitionID:           string(definition.ID),
		RouteID:                plan.RouteID,
		RouteLayoutFingerprint: plan.RouteLayoutFingerprint,
		WaypointTarget:         string(definition.WaypointTarget),
		LootPickupPolicy:       rt.runConfig.Loot.PickupFile,
		LootSellPolicy:         rt.runConfig.Loot.SellFile,
		TownOrigin:             string(definition.ReturnOrigin),
	}, nil
}

func (rt *Runtime) closeSessionRunTelemetry() error {
	if rt.Telemetry == nil {
		return nil
	}
	err := rt.Telemetry.Close()
	rt.Telemetry = nil
	rt.routePlayback.setTelemetry(nil)
	if rt.townEgress != nil {
		rt.townEgress.setTelemetry(nil)
	}
	rt.lootActions.setTelemetry(nil)
	rt.taskDeps.Telemetry = nil
	if rt.townTelemetry != nil {
		rt.townTelemetry.setTelemetry(nil)
	}
	if rt.profileTelemetry != nil {
		rt.profileTelemetry.setTelemetry(nil)
	}
	return err
}

func (rt *Runtime) runTaskToTerminal(parent context.Context) (tasks.TickResult, error) {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	rt.startShutdownSignals(ctx, cancel)
	defer func() {
		if detachErr := rt.Process.Detach(); detachErr != nil {
			rt.Log.Warn("process detach failed", "error", detachErr)
		}
	}()
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
