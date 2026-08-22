package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
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
	VerifyProfileSkills(context.Context) SupervisorRunResult
	RunToTown(context.Context, SupervisorRunRequest, bool) SupervisorRunResult
	ExitGame(context.Context) error
	Close()
}

type queueRunUnitFactory func(string) (queueRunUnit, error)

// CanAdoptQueueGame reports whether a queue may verify and reuse the passive
// monitor's current game instead of entering the character-screen start flow.
// `idle_in_game` after Apply is not enough: Memory must currently show an
// attached, bound Rogue Encampment. Otherwise StartGame uses the offline
// selector. [RuntimeQueueRunner.StartGame] still proves character and town
// through Memory before any run input.
func CanAdoptQueueGame(state SupervisorState, runtime UIStatusSnapshot) bool {
	if state != SupervisorStateIdle && state != SupervisorStateIdleInGame {
		return false
	}
	return runtime.ProcessState == "attached" &&
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

type farmQueueLifecycleFinisher interface {
	FinishQueue(SupervisorRunResult, SupervisorState) error
}

// RuntimeQueueRunner owns the production game boundary while creating fresh
// run-specific state for every queue entry. Closing one Go runtime never ends
// the D2R game; only [RuntimeQueueRunner.ExitGame] may send Save & Exit.
type RuntimeQueueRunner struct {
	mu              sync.Mutex
	controlMu       sync.RWMutex
	config          *config.Config
	publish         func(UIStatusSnapshot)
	initialInGame   bool
	gameOpen        bool
	unitRunID       string
	unit            queueRunUnit
	newUnit         queueRunUnitFactory
	sessionTrace    *telemetry.SessionRecorder
	persistEvents   bool
	pauseAfterRun   func() error
	stopAfterRun    func() error
	runsInGame      int
	skillsVerified  bool
	loadoutResolver *CharacterLoadoutResolver
	frozenLoadout   *CharacterLoadoutSnapshot
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

// SetLoadoutResolver binds the character loadout authority used when a queue session starts.
func (r *RuntimeQueueRunner) SetLoadoutResolver(resolver *CharacterLoadoutResolver) {
	if r == nil {
		return
	}
	r.controlMu.Lock()
	r.loadoutResolver = resolver
	r.controlMu.Unlock()
}

// BeginQueue resets the per-queue game boundary and freezes the character loadout for the session.
// Only a Memory-confirmed `idle_in_game` start may consume an already open game.
func (r *RuntimeQueueRunner) BeginQueue(initialInGame bool) error {
	if r == nil {
		return fmt.Errorf("queue runtime missing")
	}
	r.controlMu.RLock()
	resolver := r.loadoutResolver
	r.controlMu.RUnlock()
	if r.config != nil && strings.TrimSpace(r.config.Session.Character) != "" && resolver == nil {
		return fmt.Errorf("freeze queue loadout: character loadout resolver is unavailable")
	}
	var frozen *CharacterLoadoutSnapshot
	if resolver != nil {
		if r.config == nil {
			return fmt.Errorf("freeze queue loadout: runtime config is unavailable")
		}
		snapshot, err := resolver.Resolve(r.config.Session.Character)
		if err != nil {
			return fmt.Errorf("freeze queue loadout: %w", err)
		}
		clone := CloneCharacterLoadoutSnapshot(snapshot)
		frozen = &clone
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.closeUnitLocked()
	r.initialInGame = initialInGame
	r.gameOpen = false
	r.runsInGame = 0
	r.skillsVerified = false
	r.frozenLoadout = frozen
	return nil
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
	unit, err := r.ensureUnitLocked(request.DefinitionID)
	if err != nil {
		return err
	}
	if r.persistEvents && r.sessionTrace == nil {
		r.sessionTrace, err = telemetry.NewSessionRecorderWithContext(r.config.Telemetry.Directory, telemetry.SessionRecorderContext{
			Mode: telemetry.HistoryModeProductiveFarming, Character: r.config.Session.Character,
			Difficulty: r.config.Session.Difficulty, GameVersion: r.config.Memory.GameVersion,
		})
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
		if err := r.sessionTrace.Emit(queueTelemetryEvent(telemetry.GameStarted, request)); err != nil {
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
	unit, err := r.ensureUnitLocked(request.DefinitionID)
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
	unit, err := r.ensureUnitLocked(request.DefinitionID)
	if err != nil {
		return queueRuntimeTerminal(fmt.Errorf("initialize queue run %q: %w", request.DefinitionID, err))
	}
	if err := unit.VerifySameGame(ctx); err != nil {
		return queueRuntimeTerminal(fmt.Errorf("verify queue game: %w", err))
	}
	if !r.skillsVerified {
		gate := unit.VerifyProfileSkills(ctx)
		if gate.Disposition != "" {
			return gate
		}
		r.skillsVerified = true
	}
	if r.persistEvents && r.sessionTrace == nil {
		return SupervisorRunResult{Disposition: QueueRunStop, Reason: "telemetry_failed"}
	}
	if r.sessionTrace != nil {
		request.SessionID = r.sessionTrace.SessionID()
		if err := r.sessionTrace.Emit(queueTelemetryEvent(telemetry.RunStarted, request)); err != nil {
			return SupervisorRunResult{Disposition: QueueRunStop, Reason: "telemetry_failed"}
		}
	}
	result := unit.RunToTown(ctx, request, r.runsInGame > 0)
	r.runsInGame++
	if r.sessionTrace != nil {
		terminal := queueTelemetryEvent(queueRunTerminalEvent(result), request)
		terminal.Reason = result.Reason
		if result.Detail != "" {
			terminal.Name = result.Detail
		}
		if err := r.sessionTrace.Emit(terminal); err != nil {
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
	unit, err := r.ensureUnitLocked(request.DefinitionID)
	if err != nil {
		return err
	}
	if err := unit.ExitGame(ctx); err != nil {
		return fmt.Errorf("exit queue game (%s): %w", reason, err)
	}
	if r.sessionTrace != nil {
		exited := queueTelemetryEvent(telemetry.GameExited, request)
		exited.Reason = reason
		if err := r.sessionTrace.Emit(exited); err != nil {
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
	r.skillsVerified = false
	r.frozenLoadout = nil
}

// FinishQueue schreibt genau ein terminales Schema-3-Sessionereignis vor dem Close.
func (r *RuntimeQueueRunner) FinishQueue(result SupervisorRunResult, state SupervisorState) error {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.sessionTrace == nil {
		return nil
	}
	event := telemetry.SessionCompleted
	if state == SupervisorStateStoppedError {
		event = telemetry.SessionFailed
	} else if result.Reason == string(SupervisorReasonEmergencyStopRequested) || result.Reason == "stop_after_run" {
		event = telemetry.SessionStopped
	}
	if err := r.sessionTrace.Emit(telemetry.Event{Event: event, Reason: result.Reason}); err != nil {
		return fmt.Errorf("emit queue session terminal: %w", err)
	}
	return nil
}

func queueTelemetryEvent(name telemetry.EventName, request SupervisorRunRequest) telemetry.Event {
	event := telemetry.Event{Event: name, GameID: request.GameID}
	if name != telemetry.RunStarted && name != telemetry.RunCompleted && name != telemetry.RunAborted && name != telemetry.RunFailed {
		// Game-Grenzen gehören zur Session und dürfen nicht zufällig den gerade
		// verfügbaren Run-Kontext übernehmen. Nur Run-Lifecycle-Events tragen die
		// Korrelations-ID und Queueposition des konkreten Versuchs.
		return event
	}
	queueIndex, queueCycle := request.QueueIndex, request.Cycle
	event.RunID, event.Run = request.ExecutionID, request.DefinitionID
	event.QueueIndex, event.QueueCycle = &queueIndex, &queueCycle
	return event
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
	opts := Options{}
	if r.frozenLoadout != nil {
		clone := CloneCharacterLoadoutSnapshot(*r.frozenLoadout)
		opts.Loadout = &clone
	}
	runtime, err := New(&cfg, opts)
	if err != nil {
		return nil, err
	}
	runtime.runReadinessPending = true
	runtime.SetUIStatusPublisher(r.publish)
	runtime.setPauseHotkeyHandler(r.requestPauseAfterRun)
	runtime.setStopAfterRunHotkeyHandler(r.requestStopAfterRun)
	return &runtimeQueueUnit{runtime: runtime}, nil
}

type runtimeQueueUnit struct {
	runtime *Runtime
}

const queueReasonRetryReturnFailed = "retry_return_failed"

func (u *runtimeQueueUnit) StartOrVerifyGame(ctx context.Context, alreadyActive bool) error {
	u.runtime.Log.Info("queue game lifecycle start", "adopt_existing_game", alreadyActive)
	if alreadyActive {
		if err := u.runtime.verifyActiveQueueGame(ctx); err == nil {
			return u.finishVerifiedQueueGame(ctx, true)
		} else if !isMissingActiveQueueGame(err) {
			u.runtime.Log.Error("queue game lifecycle start failed", "stage", "active_game_verification", "error", err)
			return err
		} else {
			u.runtime.Log.Info("queue game adopt unavailable, starting from character screen", "error", err)
		}
	}
	catalog, err := ResolveCharacterCatalog(u.runtime.Config)
	if err != nil {
		return fmt.Errorf("resolve queue character selection: %w", err)
	}
	selection, err := configuredQueueCharacterSelection(u.runtime.Config, catalog)
	if err != nil {
		return err
	}
	// Queue starts must not depend on whichever offline save D2R happened to
	// leave selected. Reuse the bounded Home/Down selector that onboarding
	// already validated, then keep its visual and post-entry Memory gates.
	if err := u.runtime.ApplyCharacterSelection(ctx, selection); err != nil {
		u.runtime.Log.Error("queue game lifecycle start failed", "stage", "character_selection", "error", err)
		return err
	}
	if err := u.runtime.verifyActiveQueueGame(ctx); err != nil {
		u.runtime.Log.Error("queue game lifecycle start failed", "stage", "active_game_verification", "error", err)
		return err
	}
	return u.finishVerifiedQueueGame(ctx, false)
}

func (u *runtimeQueueUnit) finishVerifiedQueueGame(ctx context.Context, alreadyActive bool) error {
	if delay := offlinePlayersFadeDelay(alreadyActive); delay > 0 {
		u.runtime.Log.Info("offline players waiting for game fade", "settle_ms", delay.Milliseconds())
		timer := time.NewTimer(delay)
		defer timer.Stop()
		select {
		case <-timer.C:
		case <-ctx.Done():
			return fmt.Errorf("wait for offline players game fade: %w", ctx.Err())
		}
	}
	if err := u.runtime.applyOfflinePlayersCommand(); err != nil {
		u.runtime.Log.Error("queue game lifecycle start failed", "stage", "offline_players", "error", err)
		return err
	}
	u.rearmRunReadinessForNewGame(alreadyActive)
	return nil
}

func isMissingActiveQueueGame(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return errors.Is(err, context.DeadlineExceeded)
	}
	msg := err.Error()
	return strings.Contains(msg, "expected in_game") || strings.Contains(msg, "context deadline exceeded")
}

func (u *runtimeQueueUnit) rearmRunReadinessForNewGame(alreadyActive bool) {
	if u == nil || u.runtime == nil || alreadyActive {
		return
	}
	// A retry may reuse the same runtime unit after Save & Exit. Rearm only
	// at the verified new-game boundary so dead-Merc recovery and Cow start
	// reserve run again without repeating preparation between same-game runs.
	u.runtime.runReadinessPending = true
}

func configuredQueueCharacterSelection(cfg *config.Config, catalog CharacterCatalog) (CharacterSelectionRequest, error) {
	if cfg == nil {
		return CharacterSelectionRequest{}, fmt.Errorf("queue character selection requires config")
	}
	for _, entry := range catalog.Characters {
		if !strings.EqualFold(entry.Name, cfg.Session.Character) {
			continue
		}
		return CharacterSelectionRequest{
			Character: entry.Name, Difficulty: cfg.Session.Difficulty,
			CatalogRevision: catalog.Revision, CharacterCount: len(catalog.Characters),
			AnchorPath: entry.AnchorPath, ExpectedClass: entry.ExpectedClass,
		}, nil
	}
	return CharacterSelectionRequest{}, fmt.Errorf("queue character %q is missing from the offline save catalog", cfg.Session.Character)
}

func (u *runtimeQueueUnit) VerifySameGame(ctx context.Context) error {
	return u.runtime.verifyActiveQueueGame(ctx)
}

func (u *runtimeQueueUnit) VerifyProfileSkills(ctx context.Context) SupervisorRunResult {
	profileID, err := u.runtime.resolvedCombatProfileID()
	if err != nil {
		u.runtime.Log.Error("profile skills gate resolve failed", "error", err)
		return SupervisorRunResult{Disposition: QueueRunStop, Reason: reasonProfileSkillsReadUnavailable, ExitRequired: true}
	}
	profileCfg, ok := u.runtime.Config.Profiles[profileID]
	if !ok {
		u.runtime.Log.Error("profile skills gate missing profile", "profile", profileID)
		return SupervisorRunResult{Disposition: QueueRunStop, Reason: reasonProfileSkillsReadUnavailable, ExitRequired: true}
	}
	return u.runtime.verifyProfileSkillsOnce(ctx, profileID, profileCfg.RequiredSkills)
}

func (u *runtimeQueueUnit) RunToTown(ctx context.Context, request SupervisorRunRequest, sameGameContinuation bool) SupervisorRunResult {
	u.runtime.Tasks.Reset("queue_run_start")
	if sameGameContinuation {
		// The prior run's verified Town handoff replaces the new-game settle
		// delay. The hook action itself remains run-scoped and still executes.
		u.runtime.Profile.SkipInitialDelay(profile.HookTownReady)
	}
	if _, err := u.runtime.prepareSessionRun(request); err != nil {
		return queueRuntimeTerminal(fmt.Errorf("prepare queue run: %w", err))
	}
	u.runtime.productiveRunActive = true
	taskResult, runErr := u.runtime.runTaskToTerminal(ctx)
	u.runtime.productiveRunActive = false
	var result SupervisorRunResult
	if runErr != nil {
		var readinessErr *runReadinessError
		if errors.As(runErr, &readinessErr) {
			result = SupervisorRunResult{Disposition: QueueRunStop, Reason: readinessErr.reason}
		} else {
			result = queueRuntimeTerminal(fmt.Errorf("execute queue run: %w", runErr))
		}
	} else if taskResult.Outcome == tasks.RunOutcomeSuccess {
		result = SupervisorRunResult{Disposition: QueueRunAdvance, SafeToExit: true}
	} else if isTerminalMercenaryFailure(taskResult.Reason) {
		result = SupervisorRunResult{Disposition: QueueRunStop, Reason: taskResult.Reason}
	} else if request.DefinitionID == string(tasks.RunIDCows) && taskResult.Reason == "cow_return_portal_failed" {
		// The Cow setup already exhausted its bounded portal return. Bypass
		// configurable retry classes and delegate one Save & Exit to the supervisor.
		result = SupervisorRunResult{Disposition: QueueRunStop, Reason: taskResult.Reason, ExitRequired: true}
	} else if isMandatoryControlledExit(taskResult.Reason) || isRestartableSessionFailure(taskResult.Reason, u.runtime.Config.Session.RetryClasses) {
		var recoveryErr error
		result, recoveryErr = controlledRetryResult(ctx, taskResult.Reason, u.runtime.runRetryReturnToTown)
		if recoveryErr != nil {
			u.runtime.Log.Error("controlled retry return failed", "run", request.DefinitionID, "reason", taskResult.Reason, "error", recoveryErr)
		}
	} else {
		result = SupervisorRunResult{Disposition: QueueRunStop, Reason: taskResult.Reason}
	}
	if err := u.runtime.finishSessionRunTelemetry(result); err != nil {
		return SupervisorRunResult{Disposition: QueueRunStop, Reason: "telemetry_failed", SafeToExit: result.SafeToExit, ExitRequired: result.ExitRequired}
	}
	return result
}

func isMandatoryControlledExit(reason string) bool {
	return reason == reasonMercenaryDiedDuringRun || reason == "combat_resource_exhausted" || reason == string(tasks.RouteThreatReasonManaRecoveryFailed)
}

func controlledRetryResult(ctx context.Context, reason string, recoverToTown func(context.Context) error) (SupervisorRunResult, error) {
	if recoverToTown == nil {
		return SupervisorRunResult{Disposition: QueueRunStop, Reason: queueReasonRetryReturnFailed}, errors.New("controlled retry return is not wired")
	}
	if err := recoverToTown(ctx); err != nil {
		return SupervisorRunResult{Disposition: QueueRunStop, Reason: queueReasonRetryReturnFailed}, fmt.Errorf("controlled retry return: %w", err)
	}
	return SupervisorRunResult{Disposition: QueueRunRetryCurrent, Reason: reason, SafeToExit: true}, nil
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

func (rt *Runtime) verifyActiveQueueGame(parent context.Context) (err error) {
	ctx, cancel := context.WithTimeout(parent, time.Duration(rt.Config.Session.StateTimeoutMs)*time.Millisecond)
	defer cancel()
	rt.startShutdownSignals(ctx, cancel)
	defer func() {
		if err != nil {
			rt.Input.Unbind()
			_ = rt.Process.Detach()
		}
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
			if pollErr := rt.pollQueueSnapshot(ctx, state); pollErr != nil && !errors.Is(pollErr, context.Canceled) {
				return pollErr
			}
			rt.ensureVisibleInputWindow()
			if _, confirmed, observeErr := verifier.Observe(rt.World.Current(), rt.Config.Memory.GameVersion); observeErr != nil {
				return observeErr
			} else if confirmed {
				if focusErr := focusVerifiedQueueGame(rt.Input); focusErr != nil {
					return focusErr
				}
				return nil
			}
		}
	}
}

func queueGameStartDetail(err error) string {
	if err == nil {
		return "Das Spiel konnte nicht sicher gestartet werden."
	}
	msg := err.Error()
	if detail, ok := queueUnsupportedResolutionDetail(msg); ok {
		return detail
	}
	switch {
	case strings.Contains(msg, "expected in_game"):
		return "Kein laufendes Spiel im Rogue Encampment. D2R muss im Lager stehen oder auf dem Offline-Charakterbildschirm, damit der Bot das Spiel öffnet."
	case strings.Contains(msg, "character mismatch"):
		return "Im Spiel ist ein anderer Charakter aktiv als die bestätigte Auswahl."
	case strings.Contains(msg, "start area mismatch"):
		return "Das Spiel muss im Rogue Encampment stehen, bevor die Queue startet."
	case strings.Contains(msg, "blocked by open UI"):
		return "Schließe Inventar, Händler und andere Fenster, bevor die Queue startet."
	case strings.Contains(msg, "offline players"):
		return "Die Spieleranzahl konnte nach dem Spielstart nicht gesetzt werden."
	case strings.Contains(msg, "no usable client area"):
		return "Das D2R-Fenster hat keine nutzbare Fläche. Stelle Fenster-Modus 1280 × 720 ein und lass das Fenster sichtbar, nicht minimiert."
	case strings.Contains(msg, "character selection timeout"):
		return "Der Charakterbildschirm wurde nicht sicher erkannt. D2R muss auf dem Offline-Charakterbildschirm bei 1280 × 720 stehen."
	case strings.Contains(msg, "hotkey"):
		return "Die Tastatursteuerung konnte nicht sicher gestartet werden."
	case errors.Is(err, context.DeadlineExceeded) || strings.Contains(msg, "context deadline exceeded"):
		return "Das Spiel im Rogue Encampment wurde nicht rechtzeitig bestätigt."
	default:
		return "Das Spiel konnte nicht sicher gestartet werden."
	}
}

func queueUnsupportedResolutionDetail(msg string) (string, bool) {
	required := fmt.Sprintf("requires %dx%d", offlineExitClientWidth, offlineExitClientHeight)
	if !strings.Contains(msg, required) {
		return "", false
	}
	if width, height, ok := parseGotClientSize(msg); ok {
		return fmt.Sprintf("D2R läuft in %d × %d. Stelle Fenster-Modus %d × %d ein. Der Bot arbeitet nur in dieser Auflösung.", width, height, offlineExitClientWidth, offlineExitClientHeight), true
	}
	return fmt.Sprintf("D2R läuft nicht in %d × %d. Stelle Fenster-Modus %d × %d ein. Der Bot arbeitet nur in dieser Auflösung.", offlineExitClientWidth, offlineExitClientHeight, offlineExitClientWidth, offlineExitClientHeight), true
}

func parseGotClientSize(msg string) (int, int, bool) {
	idx := strings.LastIndex(msg, "got ")
	if idx < 0 {
		return 0, 0, false
	}
	var width, height int
	if _, err := fmt.Sscanf(msg[idx:], "got %dx%d", &width, &height); err != nil {
		return 0, 0, false
	}
	return width, height, true
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
	return SupervisorRunResult{Disposition: QueueRunStop, Reason: "queue_runtime_failed"}
}

func queueRunTerminalEvent(result SupervisorRunResult) telemetry.EventName {
	switch result.Disposition {
	case QueueRunAdvance:
		return telemetry.RunCompleted
	case QueueRunRetryCurrent:
		return telemetry.RunAborted
	default:
		if result.Reason == string(SupervisorReasonEmergencyStopRequested) {
			return telemetry.RunAborted
		}
		return telemetry.RunFailed
	}
}
