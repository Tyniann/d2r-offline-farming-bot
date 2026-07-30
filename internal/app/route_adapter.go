package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/pathing"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/tasks"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/telemetry"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

// routePlaybackAdapter keeps run-specific selection outside the generic player.
type routePlaybackAdapter struct {
	log         *slog.Logger
	directory   string
	gameVersion string
	navigator   pathing.SegmentNavigator
	telemetry   *telemetry.Recorder
	lifecycle   *RouteLifecycleStore
	route       pathing.Route
	player      *pathing.RoutePlayer
	deadline    time.Time
	lastTickAt  time.Time
	lastCallAt  time.Time
	identity    world.GameIdentity
	transition  bool
	clock       func() time.Time
}

func newRoutePlaybackAdapter(log *slog.Logger, directory, gameVersion string, navigator pathing.SegmentNavigator, trace *telemetry.Recorder, lifecycle ...*RouteLifecycleStore) *routePlaybackAdapter {
	adapter := &routePlaybackAdapter{log: log.With("component", "route_adapter"), directory: directory, gameVersion: gameVersion, navigator: navigator, telemetry: trace, clock: time.Now}
	if len(lifecycle) > 0 {
		adapter.lifecycle = lifecycle[0]
	}
	return adapter
}

func (a *routePlaybackAdapter) setTelemetry(trace *telemetry.Recorder) { a.telemetry = trace }

func (a *routePlaybackAdapter) Start(routeID string, state world.State) error {
	a.Reset()
	var route pathing.Route
	if a.lifecycle != nil {
		_, catalog, err := a.lifecycle.Snapshot()
		if err != nil {
			return err
		}
		found := false
		for _, entry := range catalog.Entries {
			if entry.ID != routeID {
				continue
			}
			if entry.Status == RouteLifecycleStale || entry.Status == RouteLifecycleUnavailable {
				return fmt.Errorf("route %q lifecycle status %s: %s", routeID, entry.Status, entry.Reason)
			}
			route, found = entry.Route, true
			break
		}
		if !found {
			return fmt.Errorf("%w: %q", pathing.ErrRouteNotFound, routeID)
		}
	} else {
		registry, err := pathing.LoadRouteRegistry(a.directory)
		if err != nil {
			return err
		}
		candidate, getErr := registry.Get(routeID)
		if getErr != nil {
			return getErr
		}
		route = candidate
	}
	fingerprint, err := pathing.BuildLayoutFingerprint(state)
	if err != nil {
		return fmt.Errorf("run route layout: %w", err)
	}
	if validationErr := pathing.ValidateRoutePrecheck(route, pathing.RoutePrecheckInput{Identity: state.Identity, GameVersion: a.gameVersion, Layout: fingerprint, World: state}); validationErr != nil {
		if errors.Is(validationErr, pathing.ErrRouteLayoutMismatch) && a.lifecycle != nil {
			manifest, _, snapshotErr := a.lifecycle.Snapshot()
			if snapshotErr != nil {
				return fmt.Errorf("run route lifecycle before layout invalidation: %w", snapshotErr)
			}
			if _, invalidationErr := a.lifecycle.InvalidateLayout(route.Binding.CharacterName, manifest.Revision, time.Now().UTC()); invalidationErr != nil {
				return fmt.Errorf("run route lifecycle layout invalidation: %w", invalidationErr)
			}
		}
		return fmt.Errorf("run route precheck: %w", validationErr)
	}
	player, err := pathing.NewRoutePlayer(a.navigator, route)
	if err != nil {
		return err
	}
	a.route, a.player = route, player
	now := a.now()
	a.deadline = now.Add(time.Duration(route.Playback.SegmentTimeoutMs) * time.Millisecond)
	a.lastTickAt = state.At
	a.lastCallAt = now
	a.identity = state.Identity
	if err := a.emit(telemetry.Event{Event: telemetry.RoutePlaybackStarted, RouteID: route.ID, SegmentID: player.Segment().ID, AreaID: uint32(state.Area.ID)}); err != nil {
		a.Reset()
		return err
	}
	a.log.Info("run route playback started", "route_id", route.ID, "character", state.Identity.CharacterName, "layout_fingerprint", fingerprint.Hash)
	return nil
}

// Progress returns the task-facing projection of the player's effective next target.
func (a *routePlaybackAdapter) Progress(state world.State) (tasks.RouteProgress, bool) {
	if a.player == nil {
		return tasks.RouteProgress{}, false
	}
	progress, ok := a.player.Progress(state)
	if !ok {
		return tasks.RouteProgress{}, false
	}
	mode := tasks.RouteProgressMode(progress.Mode)
	return tasks.RouteProgress{
		RouteID:               progress.RouteID,
		SegmentID:             progress.SegmentID,
		SegmentIndex:          progress.SegmentIndex,
		PointIndex:            progress.PointIndex,
		PreviousConfirmed:     progress.PreviousConfirmed,
		MovementTarget:        progress.MovementTarget,
		TargetAvailable:       progress.TargetAvailable,
		Mode:                  mode,
		DriftTiles:            progress.DriftTiles,
		LocalRecoveryAttempts: progress.LocalRecoveryAttempts,
		RecoveryInputSent:     progress.RecoveryInputSent,
		RecoveryInputAt:       progress.RecoveryInputAt,
		RecoveryInputOrigin:   progress.RecoveryInputOrigin,
		RecoveryNextInputAt:   progress.RecoveryNextInputAt,
		RecoveryOutcomeAt:     progress.RecoveryOutcomeAt,
		RecoveryProgressTiles: progress.RecoveryProgressTiles,
	}, true
}

// Hold freezes route playback for one validated snapshot and extends only the
// adapter-owned deadline by the real elapsed hold duration. It commits points
// already reached in Memory so external combat movement cannot revive a passed
// point, but it sends no route input and ticks no navigator or transition.
func (a *routePlaybackAdapter) Hold(state world.State) error {
	if a.player == nil {
		return fmt.Errorf("run route playback not started")
	}
	if err := a.validateHoldState(state); err != nil {
		return err
	}
	if err := a.player.SyncReached(state); err != nil {
		return fmt.Errorf("run route hold sync reached points: %w", err)
	}
	// An unchanged snapshot is not a new hold observation. Keeping lastCallAt
	// unchanged ensures repeated polling cannot credit the same interval twice.
	if state.At.Equal(a.lastTickAt) {
		return nil
	}
	now := a.now()
	if now.Before(a.lastCallAt) {
		return fmt.Errorf("run route hold clock moved backwards")
	}
	a.deadline = a.deadline.Add(now.Sub(a.lastCallAt))
	a.lastCallAt = now
	a.lastTickAt = state.At
	return nil
}

func (a *routePlaybackAdapter) Tick(ctx context.Context, state world.State) (bool, error) {
	if a.player == nil {
		return false, fmt.Errorf("run route playback not started")
	}
	if !a.lastTickAt.IsZero() && !state.At.IsZero() && state.At.Sub(a.lastTickAt) > 2*time.Second {
		a.deadline = a.deadline.Add(state.At.Sub(a.lastTickAt))
	}
	if !state.At.IsZero() {
		a.lastTickAt = state.At
	}
	now := a.now()
	a.lastCallAt = now
	if now.After(a.deadline) {
		if a.transition {
			return false, fmt.Errorf("%w: timeout", pathing.ErrRouteTransitionFailed)
		}
		return false, fmt.Errorf("%w: %s", pathing.ErrRouteSegmentTimeout, a.player.Segment().ID)
	}
	before := a.player.SegmentIndex()
	done, err := a.player.Tick(ctx, state)
	if err != nil {
		_ = a.emit(telemetry.Event{Event: telemetry.RoutePlaybackFailed, RouteID: a.route.ID, SegmentID: a.player.Segment().ID, Reason: err.Error()})
		return false, err
	}
	if !done && a.player.SegmentIndex() != before {
		completed := a.route.Segments[before]
		idx := before
		if err := a.emit(telemetry.Event{Event: telemetry.RouteSegmentCompleted, RouteID: a.route.ID, SegmentID: completed.ID, SegmentIndex: &idx, AreaID: uint32(state.Area.ID)}); err != nil {
			return false, err
		}
		a.transition = false
		a.deadline = now.Add(time.Duration(a.route.Playback.SegmentTimeoutMs) * time.Millisecond)
		a.log.Info("run route segment completed", "route_id", a.route.ID, "segment_id", completed.ID, "area_id", state.Area.ID)
	}
	if done {
		idx := len(a.route.Segments) - 1
		if err := a.emit(telemetry.Event{Event: telemetry.RouteSegmentCompleted, RouteID: a.route.ID, SegmentID: a.route.Segments[idx].ID, SegmentIndex: &idx, AreaID: uint32(state.Area.ID)}); err != nil {
			return false, err
		}
		if err := a.emit(telemetry.Event{Event: telemetry.RoutePlaybackCompleted, RouteID: a.route.ID, SegmentID: a.route.Segments[idx].ID, AreaID: uint32(state.Area.ID)}); err != nil {
			return false, err
		}
		a.log.Info("run route playback completed", "route_id", a.route.ID, "area_id", state.Area.ID)
		return true, nil
	}
	if a.player.Transitioning() && !a.transition {
		a.transition = true
		a.deadline = now.Add(time.Duration(a.route.Playback.TransitionTimeoutMs) * time.Millisecond)
		idx := a.player.SegmentIndex()
		if err := a.emit(telemetry.Event{Event: telemetry.RouteTransitionStarted, RouteID: a.route.ID, SegmentID: a.player.Segment().ID, SegmentIndex: &idx, TargetAreaID: uint32(a.player.Segment().ToAreaID)}); err != nil {
			return false, err
		}
	}
	return false, nil
}

func (a *routePlaybackAdapter) Reset() {
	if a.player != nil {
		a.player.Reset()
	} else if a.navigator != nil {
		a.navigator.Reset()
	}
	a.route = pathing.Route{}
	a.player = nil
	a.deadline = time.Time{}
	a.lastTickAt = time.Time{}
	a.lastCallAt = time.Time{}
	a.identity = world.GameIdentity{}
	a.transition = false
}

func (a *routePlaybackAdapter) validateHoldState(state world.State) error {
	if !state.Valid || state.Phase != world.GamePhaseInGame {
		return fmt.Errorf("run route hold requires valid in-game state")
	}
	if !state.Identity.Valid || state.Identity != a.identity {
		return fmt.Errorf("run route hold identity changed")
	}
	if state.At.IsZero() || (!a.lastTickAt.IsZero() && state.At.Before(a.lastTickAt)) {
		return fmt.Errorf("run route hold snapshot is not monotonic")
	}
	segment := a.player.Segment()
	if state.Area.ID != segment.FromAreaID && (!a.player.Transitioning() || state.Area.ID != segment.ToAreaID) {
		return fmt.Errorf("%w: got %d want %d", pathing.ErrRouteUnexpectedArea, state.Area.ID, segment.FromAreaID)
	}
	if _, ok := a.player.Progress(state); !ok {
		return fmt.Errorf("run route hold has no active progress")
	}
	return nil
}

func (a *routePlaybackAdapter) now() time.Time {
	if a.clock != nil {
		return a.clock()
	}
	return time.Now()
}

func (a *routePlaybackAdapter) emit(event telemetry.Event) error {
	if a.telemetry == nil {
		return nil
	}
	if err := a.telemetry.Emit(event); err != nil {
		return fmt.Errorf("run route telemetry: %w", err)
	}
	return nil
}
