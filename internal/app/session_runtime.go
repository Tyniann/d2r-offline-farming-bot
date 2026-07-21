package app

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/tasks"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/telemetry"
)

// PickitPolicySnapshot bindet die exakten Assignment- und Profilrevisionen einer Run-Generation.
type PickitPolicySnapshot struct {
	Character          string
	RunID              tasks.RunID
	Profiles           []string
	ProfileRevisions   map[string]uint64
	AssignmentRevision uint64
}

func (rt *Runtime) prepareSessionRun() (string, error) {
	if err := rt.reloadPickitPolicy(); err != nil {
		return "", err
	}
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
