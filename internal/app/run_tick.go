package app

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/input"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/memory"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/process"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/replay"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

const (
	inputBindRetryInterval = time.Second
	inputBindLogHeartbeat  = 5 * time.Second
)

// processController is the subset of process.Service used by the run loop.
type processController interface {
	Attach(ctx context.Context) error
	Poll() process.Status
	Status() process.Status
	Detach() error
	Ready() bool
}

// snapshotReader reads Phase-1 probe snapshots from attached process memory.
type snapshotReader interface {
	Snapshot() memory.Snapshot
}

// inputController is the subset of input.Controller used by the run loop.
type inputController interface {
	Bind(pid uint32) error
	Unbind()
	Bound() bool
	Ready() bool
	Status() input.Status
	CastBelt(src input.BeltBindingSource, slot int) error
	CastBeltWithModifier(src input.BeltBindingSource, modifier string, slot int) error
	CastSkillAt(src input.BindingSource, skillID uint16, clientX, clientY int) error
	SelectSkill(src input.BindingSource, skillID uint16) error
	MoveTo(clientX, clientY int) error
	Click(button input.MouseButton) error
	ClickWithModifier(modifier string, button input.MouseButton) error
	ClickAtWithModifier(clientX, clientY int, modifier string, button input.MouseButton) error
	PressKey(key string) error
	Focus() error
	Window() (input.WindowInfo, bool)
	TogglePause(reason string) bool
	Stop(reason string)
	ListenHotkeys(ctx context.Context, events chan<- input.HotkeyEvent, ready chan<- error)
}

// runState holds mutable loop state for a single Runtime run.
type runState struct {
	attached             bool
	hasEverAttached      bool
	attachWaitStarted    time.Time
	waitingLogged        bool
	lastFatalErr         string
	lastLoggedState      process.State
	world                worldLoopState
	input                inputLoopState
	bindingsPrecheckDone bool
}

type inputLoopState struct {
	lastBindAttempt time.Time
	lastBindLog     time.Time
	lastBindErr     string
}

type worldLoopState struct {
	forceLog   bool
	lastLogged world.State
	lastLog    time.Time
}

// runTick executes one poll-loop iteration: attach, poll, snapshot read, and world update.
func (rt *Runtime) runTick(ctx context.Context, state *runState) (runErr error) {
	return rt.runTickWithMode(ctx, state, runTickModeFull)
}

type runTickMode int

const (
	runTickModeFull runTickMode = iota
	runTickModeSnapshotOnly
)

// pollQueueSnapshot updates process/input/world state without bindings precheck,
// run readiness, or task execution. Queue game verification and the session
// skill gate use this path so Missing-Skill stops never race with productive input.
func (rt *Runtime) pollQueueSnapshot(ctx context.Context, state *runState) error {
	return rt.runTickWithMode(ctx, state, runTickModeSnapshotOnly)
}

func (rt *Runtime) runTickWithMode(ctx context.Context, state *runState, mode runTickMode) (runErr error) {
	defer func() {
		if rt.uiStatusPublisher != nil {
			lastError := ""
			if runErr != nil && !errors.Is(runErr, context.Canceled) {
				lastError = runErr.Error()
			}
			rt.uiStatusPublisher(rt.CurrentUIStatus(lastError))
		}
	}()
	if !state.attached && rt.processPreAttached.Swap(false) {
		state.attached = true
		state.hasEverAttached = true
		state.waitingLogged = false
	}
	if !state.attached {
		if !state.hasEverAttached {
			if timeout := rt.Config.Process.AttachTimeoutMs; timeout > 0 {
				if state.attachWaitStarted.IsZero() {
					state.attachWaitStarted = time.Now()
				}
				limit := time.Duration(timeout) * time.Millisecond
				if time.Since(state.attachWaitStarted) >= limit {
					return fmt.Errorf(
						"attach timeout after %dms waiting for %s",
						timeout,
						rt.Config.Process.ProcessName,
					)
				}
			}
		}

		if err := rt.Process.Attach(ctx); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if process.IsFatal(err) {
				errMsg := err.Error()
				if errMsg != state.lastFatalErr {
					rt.Log.Error("process attach failed", "error", err)
					state.lastFatalErr = errMsg
				}
			} else if !state.waitingLogged {
				rt.Log.Info("waiting for target process",
					"process", rt.Config.Process.ProcessName,
				)
				state.waitingLogged = true
			}
			return nil
		}
		state.attached = true
		state.hasEverAttached = true
		state.waitingLogged = false
		state.lastFatalErr = ""
		if rt.Options.Probe {
			state.world.forceLog = true
		}
		rt.logProcessStateChange(state.lastLoggedState, process.StateAttached)
		state.lastLoggedState = process.StateAttached
		if rt.CompatibilitySnapshot().State != D2RCompatibilityCompatible {
			rt.Input.Unbind()
			return nil
		}
		if err := rt.tryBindInput(state); err != nil {
			return err
		}
		return nil
	}

	st := rt.Process.Poll()
	if st.State == process.StateLost {
		if rt.RuntimeTrace != nil && rt.RuntimeTrace.Enabled() {
			result := rt.Tasks.Result()
			if err := rt.finalizeRuntimeTrace(replay.Terminal{Step: result.Step, Outcome: "failed", Reason: "process_lost"}); err != nil {
				rt.Log.Error("runtime trace finalization failed", "error", err)
			}
		}
		rt.Input.Unbind()
		if mode == runTickModeFull {
			rt.Tasks.Reset("process_lost")
		}
		state.input = inputLoopState{}
		state.bindingsPrecheckDone = false
		prev := rt.World.Current()
		cur := rt.World.Reset(time.Now(), worldResetReasonProcessLost)
		if rt.Options.Probe && worldShouldLog(prev, cur, state.world.lastLog, worldHeartbeat, state.world.forceLog, rt.Options.Verbose) {
			rt.logWorldState(prev, cur, worldLogIsHeartbeat(state.world.lastLog, worldHeartbeat), rt.Options.Verbose)
		}
		state.world = worldLoopState{}
		state.attached = false
		rt.logProcessStateChange(state.lastLoggedState, process.StateLost)
		state.lastLoggedState = process.StateLost
		return nil
	}
	if rt.CompatibilitySnapshot().State != D2RCompatibilityCompatible {
		rt.Input.Unbind()
		return nil
	}

	if err := rt.tryBindInput(state); err != nil {
		return err
	}

	snap := rt.Probe.Snapshot()
	rt.lastSnapshot = snap
	prevWorld := rt.World.Current()
	cur := rt.World.Update(snap)
	if mode == runTickModeFull {
		rt.observeMercenaryDeath(prevWorld, cur)
		if err := rt.abortRunOnMercenaryDeath(prevWorld, cur); err != nil {
			return err
		}
		if rt.shouldRunBindingsPrecheck(snap, state) {
			state.bindingsPrecheckDone = true
			if err := BindingsPrecheck(rt.Log, rt.Bindings, snap, true); err != nil {
				return fmt.Errorf("bindings precheck: %w", err)
			}
		}
		ready, err := rt.consumeRunReadiness(ctx, cur)
		if err != nil {
			return err
		}
		if ready && rt.shouldTickTasks(cur) {
			now := time.Now()
			if rt.RuntimeTrace != nil && rt.RuntimeTrace.Enabled() {
				status := rt.Input.Status()
				rt.RuntimeTrace.BeginTick(now, replay.NormalizeWorld(cur), cur.Generation, replay.RuntimeGates{InputEnabled: status.Enabled, Paused: status.Paused, Stopped: status.Stopped, WindowBound: rt.Input.Bound()}, traceTickState(rt.Tasks.Result()))
			}
			result := rt.Tasks.Tick(ctx, cur, now)
			if rt.RuntimeTrace != nil && rt.RuntimeTrace.Enabled() {
				rt.RuntimeTrace.EndTick(traceTickState(result))
			}
		}
	}
	prev := state.world.lastLogged
	if rt.Options.Probe && worldShouldLog(prev, cur, state.world.lastLog, worldHeartbeat, state.world.forceLog, rt.Options.Verbose) {
		rt.logWorldState(prev, cur, worldLogIsHeartbeat(state.world.lastLog, worldHeartbeat), rt.Options.Verbose)
		state.world.lastLogged = cur
		state.world.lastLog = time.Now()
		state.world.forceLog = false
	}

	return nil
}

// shouldRunBindingsPrecheck reports whether this full tick must fail closed on
// Teleport. Isolated probes skip it. Idle desktop and character confirmation
// poll with empty dummy bindings and must not require a loadout; frozen
// Schema-3 loadouts still precheck before recording or candidate playback.
func (rt *Runtime) shouldRunBindingsPrecheck(snap memory.Snapshot, state *runState) bool {
	if !rt.Config.Input.Enabled || state.bindingsPrecheckDone || !snap.Valid || snap.Phase != memory.GamePhaseInGame {
		return false
	}
	if rt.Options.InputTest != "" || rt.Options.OfflineDifficulty != "" || rt.Options.OfflineExitTest || rt.Options.UIStateProbe != "" || rt.Options.ScreenAnchorCapture != "" || rt.Options.MercenaryProbe != "" || rt.Options.CowProbe != "" || rt.Options.WeaponSetProbe != "" || rt.Options.ObjectInspect != "" || rt.Options.TownInspect || rt.Options.TownTest != "" || rt.pathingTestIsReadOnly() || rt.routeCommandIsReadOnly() {
		return false
	}
	if rt.Options.Loadout == nil && !CharacterLoadoutRequired(rt.Options) {
		return false
	}
	return true
}

func (rt *Runtime) tryBindInput(state *runState) error {
	if err := rt.requireCompatible(); err != nil {
		return err
	}
	if rt.Input.Bound() {
		return nil
	}

	now := time.Now()
	if !state.input.lastBindAttempt.IsZero() && now.Sub(state.input.lastBindAttempt) < inputBindRetryInterval {
		return nil
	}
	state.input.lastBindAttempt = now

	pid := rt.Process.Status().PID
	err := rt.Input.Bind(pid)
	if err == nil {
		state.input.lastBindErr = ""
		rt.warnPathingResolution()
		return nil
	}
	if !input.IsBindRetryable(err) {
		return fmt.Errorf("input bind pid=%d: %w", pid, err)
	}

	errMsg := err.Error()
	shouldLog := state.input.lastBindLog.IsZero() ||
		errMsg != state.input.lastBindErr ||
		now.Sub(state.input.lastBindLog) >= inputBindLogHeartbeat
	if shouldLog {
		rt.Log.Info("waiting for input window", "pid", pid, "error", err)
		state.input.lastBindLog = now
		state.input.lastBindErr = errMsg
	}
	return nil
}

type visibleWindowBinder interface {
	BindVisible(pid uint32) error
}

// ensureVisibleInputWindow restores a 0×0/minimized D2R HWND during selection
// and offline start. Idle [tryBindInput] stays restore-free so the dashboard
// does not yank the game every second.
func (rt *Runtime) ensureVisibleInputWindow() {
	if rt.Input.Bound() {
		return
	}
	pid := rt.Process.Status().PID
	if pid == 0 {
		return
	}
	binder, ok := rt.Input.(visibleWindowBinder)
	if !ok {
		return
	}
	if err := binder.BindVisible(pid); err != nil {
		rt.Log.Debug("visible input window not ready", "pid", pid, "error", err)
	}
}
