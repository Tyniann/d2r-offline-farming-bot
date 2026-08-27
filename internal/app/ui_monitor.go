package app

import (
	"context"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/tasks"
)

// UIStatusSnapshot is the immutable read-only runtime projection consumed by
// the local API. It contains no Memory addresses or mutable World slices.
type UIStatusSnapshot struct {
	At            time.Time
	ProcessState  string
	PID           uint32
	WindowBound   bool
	ClientWidth   int
	ClientHeight  int
	InputEnabled  bool
	InputPaused   bool
	InputStopped  bool
	WorldValid    bool
	WorldPhase    string
	AreaID        uint32
	AreaName      string
	RunID         string
	Step          string
	RecoveryStep  string
	RunProgress   *tasks.RunProgress
	LastError     string
	Compatibility D2RCompatibilitySnapshot
}

// CurrentUIStatus returns a consistent projection from component snapshots.
func (rt *Runtime) CurrentUIStatus(lastError string) UIStatusSnapshot {
	processStatus := rt.Process.Status()
	inputStatus := rt.Input.Status()
	worldState := rt.World.Current()
	compatibility := rt.CompatibilitySnapshot()
	window, windowBound := rt.Input.Window()
	snapshot := UIStatusSnapshot{
		At: time.Now().UTC(), ProcessState: string(processStatus.State), PID: processStatus.PID,
		WindowBound: windowBound, InputEnabled: inputStatus.Enabled && compatibility.State == D2RCompatibilityCompatible, InputPaused: inputStatus.Paused,
		InputStopped: inputStatus.Stopped, WorldValid: worldState.Valid, WorldPhase: worldState.Phase.String(), LastError: lastError,
		Compatibility: compatibility,
	}
	if windowBound {
		snapshot.ClientWidth, snapshot.ClientHeight = window.ClientWidth, window.ClientHeight
	}
	if worldState.Valid {
		snapshot.AreaID, snapshot.AreaName = uint32(worldState.Area.ID), worldState.Area.Name
	}
	if rt.Tasks != nil {
		snapshot.RunID = rt.Tasks.ConfiguredRun()
		result := rt.Tasks.Result()
		snapshot.Step = result.Step
		if progress, ok := rt.Tasks.Progress(worldState.Area.ID); ok {
			snapshot.RunProgress = &progress
		}
		snapshot.RecoveryStep = rt.Tasks.RecoveryStep()
	}
	if step := rt.currentRecoveryStep(); step != "" {
		snapshot.RecoveryStep = step
	}
	return snapshot
}

func (rt *Runtime) setRecoveryStep(step string) {
	if rt == nil || step == "" {
		if rt != nil {
			rt.recoveryStep.Store(nil)
		}
		return
	}
	value := step
	rt.recoveryStep.Store(&value)
}

func (rt *Runtime) currentRecoveryStep() string {
	if rt == nil {
		return ""
	}
	value := rt.recoveryStep.Load()
	if value == nil {
		return ""
	}
	return *value
}

// SetUIStatusPublisher installs an optional non-blocking observer used by the
// local dashboard while a productive queue worker owns the runtime.
func (rt *Runtime) SetUIStatusPublisher(publish func(UIStatusSnapshot)) {
	rt.uiStatusPublisher = publish
}

// RunUIMonitor continuously performs only process attach, window binding and
// read-only snapshot updates. Errors remain visible and retryable; this monitor
// never starts tasks, registers session commands or sends gameplay input.
func (rt *Runtime) RunUIMonitor(ctx context.Context, publish func(UIStatusSnapshot)) error {
	if rt == nil {
		return context.Canceled
	}
	if publish == nil {
		publish = func(UIStatusSnapshot) {}
	}
	ticker := time.NewTicker(time.Duration(rt.Config.Runtime.PollIntervalMs) * time.Millisecond)
	defer ticker.Stop()
	defer func() {
		rt.Input.Unbind()
		_ = rt.Process.Detach()
	}()
	state := &runState{}
	publish(rt.CurrentUIStatus(""))
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			lastError := ""
			if err := rt.runTick(ctx, state); err != nil && ctx.Err() == nil {
				lastError = err.Error()
				state.attachWaitStarted = time.Now()
				rt.Log.Warn("UI read-only monitor tick failed", "error", err)
			}
			publish(rt.CurrentUIStatus(lastError))
		}
	}
}
