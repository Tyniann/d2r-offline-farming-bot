package app

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/tasks"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/telemetry"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

// PickitPolicySnapshot bindet die exakten Assignment- und Profilrevisionen einer Run-Generation.
type PickitPolicySnapshot struct {
	Character          string
	RunID              tasks.RunID
	Profiles           []string
	ProfileRevisions   map[string]uint64
	AssignmentRevision uint64
}

func (rt *Runtime) prepareSessionRun(request SupervisorRunRequest) (string, error) {
	if err := rt.reloadPickitPolicy(); err != nil {
		return "", err
	}
	contextEvent, err := rt.sessionRunContextEvent()
	if err != nil {
		return "", err
	}
	profiles := make([]telemetry.PickitProfileContext, 0, len(rt.ActivePickit.Profiles))
	for _, profileID := range rt.ActivePickit.Profiles {
		profiles = append(profiles, telemetry.PickitProfileContext{ID: profileID, Revision: rt.ActivePickit.ProfileRevisions[profileID]})
	}
	trace, err := telemetry.NewRunRecorder(rt.Config.Telemetry.Directory, telemetry.RunRecorderContext{
		RunID: request.ExecutionID, SessionID: request.SessionID, GameID: request.GameID,
		Mode: telemetry.HistoryModeProductiveFarming, Character: rt.Config.Session.Character,
		Difficulty: rt.Config.Session.Difficulty, GameVersion: rt.Config.Memory.GameVersion,
		Run: rt.Config.Session.Run, DefinitionID: contextEvent.DefinitionID,
		RouteID: contextEvent.RouteID, RouteLayoutFingerprint: contextEvent.RouteLayoutFingerprint,
		SetupRouteID: contextEvent.SetupRouteID, SetupRouteLayoutFingerprint: contextEvent.SetupRouteLayoutFingerprint,
		QueueIndex: request.QueueIndex, QueueCycle: request.Cycle, StartedAt: time.Now().UTC(),
		PickitProfiles: profiles, PickitAssignmentRevision: rt.ActivePickit.AssignmentRevision,
	})
	if err != nil {
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

func (rt *Runtime) reloadPickitPolicy() error {
	if rt.PickitAssignments == nil {
		return nil
	}
	effective, err := rt.PickitAssignments.Resolve(rt.Config.Session.Character, tasks.RunID(rt.Config.Session.Run))
	if err != nil {
		return fmt.Errorf("reload pickit policy at run boundary: %w", err)
	}
	// Erst die vollständig validierte, neu kompilierte Policy wird als Ganzes
	// aktiviert. Ein Reload-Fehler lässt den bisherigen Snapshot unverändert.
	rt.Loot.SetPickit(effective.All)
	if rt.townPreparation != nil {
		rt.townPreparation.setItemPolicies(rt.Loot, rt.Config.Loot.Stash)
	}
	rt.ActivePickit = PickitPolicySnapshot{Character: rt.Config.Session.Character, RunID: tasks.RunID(rt.Config.Session.Run), Profiles: append([]string(nil), effective.Profiles...), ProfileRevisions: cloneRevisionMap(effective.ProfileRevisions), AssignmentRevision: effective.AssignmentRevision}
	return nil
}

func cloneRevisionMap(source map[string]uint64) map[string]uint64 {
	clone := make(map[string]uint64, len(source))
	for key, value := range source {
		clone[key] = value
	}
	return clone
}

func (rt *Runtime) sessionRunContextEvent() (telemetry.Event, error) {
	// The run boundary must use the same immutable loadout as Runtime
	// construction. Re-resolving without it would silently fall back to the
	// Necromancer profile and reject valid Paladin routes.
	plan, err := ResolveSessionPlan(rt.Config, Options{SessionInspect: true, Loadout: rt.Options.Loadout})
	if err != nil {
		return telemetry.Event{}, fmt.Errorf("resolve session run context: %w", err)
	}
	definition, ok := tasks.DefaultRunRegistry().Definition(tasks.RunID(rt.Config.Session.Run))
	if !ok {
		return telemetry.Event{}, fmt.Errorf("resolve session run context: %s: %q", tasks.RunReasonUnknown, rt.Config.Session.Run)
	}
	routeID := plan.RouteID
	if routeID == "" {
		routeID = rt.runConfig.RouteID
	}
	return telemetry.Event{
		Event:                       telemetry.RunContext,
		DefinitionID:                string(definition.ID),
		RouteID:                     routeID,
		RouteLayoutFingerprint:      plan.RouteLayoutFingerprint,
		SetupRouteID:                plan.SetupRouteID,
		SetupRouteLayoutFingerprint: plan.SetupRouteLayoutFingerprint,
		WaypointTarget:              string(definition.WaypointTarget),
		TownOrigin:                  string(definition.ReturnOrigin),
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

func (rt *Runtime) finishSessionRunTelemetry(result SupervisorRunResult) error {
	if rt.Telemetry == nil {
		return fmt.Errorf("session run telemetry is not active")
	}
	terminal := telemetry.Event{Event: queueRunTerminalEvent(result), Reason: result.Reason}
	emitErr := rt.Telemetry.Emit(terminal)
	closeErr := rt.closeSessionRunTelemetry()
	if emitErr != nil {
		return fmt.Errorf("emit session run terminal: %w", emitErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close session run telemetry: %w", closeErr)
	}
	return nil
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
			if abortErr := rt.Tasks.AbortOpenStep(string(SupervisorReasonEmergencyStopRequested)); abortErr != nil {
				rt.Log.Warn("abort open run step failed", "error", abortErr)
			}
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

func (rt *Runtime) runRetryReturnToTown(parent context.Context) error {
	rt.Tasks = tasks.NewRunner(rt.Log, tasks.RunSelection{
		Run:   rt.Config.Session.Run,
		Phase: tasks.RunPhaseRetryReturn,
	}, rt.runConfig, rt.taskDeps)
	result, err := rt.runTaskToTerminal(parent)
	if err != nil {
		return fmt.Errorf("retry return execution: %w", err)
	}
	if result.Outcome != tasks.RunOutcomeSuccess {
		return fmt.Errorf("retry return failed: %s", result.Reason)
	}
	state := rt.World.Current()
	if !state.Valid || state.Phase != world.GamePhaseInGame || state.Area.ID != world.RogueEncampment {
		return fmt.Errorf("retry return Act-1 handoff unconfirmed")
	}
	return nil
}
