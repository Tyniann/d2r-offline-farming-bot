package app

import (
	"context"
	"fmt"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/memory"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/process"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
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

// runState holds mutable loop state for a single Runtime run.
type runState struct {
	attached          bool
	hasEverAttached   bool
	attachWaitStarted time.Time
	waitingLogged     bool
	lastFatalErr      string
	lastLoggedState   process.State
	world             worldLoopState
}

type worldLoopState struct {
	forceLog   bool
	lastLogged world.State
	lastLog    time.Time
}

// runTick executes one poll-loop iteration: attach, poll, snapshot read, and world update.
func (rt *Runtime) runTick(ctx context.Context, state *runState) error {
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
		return nil
	}

	st := rt.Process.Poll()
	if st.State == process.StateLost {
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

	snap := rt.Probe.Snapshot()
	cur := rt.World.Update(snap)
	prev := state.world.lastLogged
	if rt.Options.Probe && worldShouldLog(prev, cur, state.world.lastLog, worldHeartbeat, state.world.forceLog, rt.Options.Verbose) {
		rt.logWorldState(prev, cur, worldLogIsHeartbeat(state.world.lastLog, worldHeartbeat), rt.Options.Verbose)
		state.world.lastLogged = cur
		state.world.lastLog = time.Now()
		state.world.forceLog = false
	}

	return nil
}
