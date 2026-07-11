package app

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/pathing"
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
	route       pathing.Route
	player      *pathing.RoutePlayer
	deadline    time.Time
	lastTickAt  time.Time
	transition  bool
}

func newRoutePlaybackAdapter(log *slog.Logger, directory, gameVersion string, navigator pathing.SegmentNavigator, trace *telemetry.Recorder) *routePlaybackAdapter {
	return &routePlaybackAdapter{log: log.With("component", "route_adapter"), directory: directory, gameVersion: gameVersion, navigator: navigator, telemetry: trace}
}

func (a *routePlaybackAdapter) Start(routeID string, state world.State) error {
	a.Reset()
	registry, err := pathing.LoadRouteRegistry(a.directory)
	if err != nil {
		return err
	}
	route, err := registry.Get(routeID)
	if err != nil {
		return err
	}
	fingerprint, err := pathing.BuildLayoutFingerprint(state)
	if err != nil {
		return fmt.Errorf("countess route layout: %w", err)
	}
	if err := pathing.ValidateRoutePrecheck(route, pathing.RoutePrecheckInput{Identity: state.Identity, GameVersion: a.gameVersion, Layout: fingerprint, World: state}); err != nil {
		return fmt.Errorf("countess route precheck: %w", err)
	}
	player, err := pathing.NewRoutePlayer(a.navigator, route)
	if err != nil {
		return err
	}
	a.route, a.player = route, player
	a.deadline = time.Now().Add(time.Duration(route.Playback.SegmentTimeoutMs) * time.Millisecond)
	a.lastTickAt = state.At
	if err := a.emit(telemetry.Event{Event: telemetry.RoutePlaybackStarted, RouteID: route.ID, SegmentID: player.Segment().ID, AreaID: uint32(state.Area.ID)}); err != nil {
		a.Reset()
		return err
	}
	a.log.Info("Countess route playback started", "route_id", route.ID, "character", state.Identity.CharacterName, "layout_fingerprint", fingerprint.Hash)
	return nil
}

func (a *routePlaybackAdapter) Tick(ctx context.Context, state world.State) (bool, error) {
	if a.player == nil {
		return false, fmt.Errorf("Countess route playback not started")
	}
	if !a.lastTickAt.IsZero() && !state.At.IsZero() && state.At.Sub(a.lastTickAt) > 2*time.Second {
		a.deadline = a.deadline.Add(state.At.Sub(a.lastTickAt))
	}
	if !state.At.IsZero() {
		a.lastTickAt = state.At
	}
	if time.Now().After(a.deadline) {
		if a.transition {
			return false, fmt.Errorf("%w: timeout", pathing.ErrRouteTransitionFailed)
		}
		return false, fmt.Errorf("route segment timeout: %s", a.player.Segment().ID)
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
		a.deadline = time.Now().Add(time.Duration(a.route.Playback.SegmentTimeoutMs) * time.Millisecond)
		a.log.Info("Countess route segment completed", "route_id", a.route.ID, "segment_id", completed.ID, "area_id", state.Area.ID)
	}
	if done {
		idx := len(a.route.Segments) - 1
		if err := a.emit(telemetry.Event{Event: telemetry.RouteSegmentCompleted, RouteID: a.route.ID, SegmentID: a.route.Segments[idx].ID, SegmentIndex: &idx, AreaID: uint32(state.Area.ID)}); err != nil {
			return false, err
		}
		if err := a.emit(telemetry.Event{Event: telemetry.RoutePlaybackCompleted, RouteID: a.route.ID, SegmentID: a.route.Segments[idx].ID, AreaID: uint32(state.Area.ID)}); err != nil {
			return false, err
		}
		a.log.Info("Countess route playback completed", "route_id", a.route.ID, "area_id", state.Area.ID)
		return true, nil
	}
	if a.player.Transitioning() && !a.transition {
		a.transition = true
		a.deadline = time.Now().Add(time.Duration(a.route.Playback.TransitionTimeoutMs) * time.Millisecond)
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
	a.transition = false
}

func (a *routePlaybackAdapter) emit(event telemetry.Event) error {
	if a.telemetry == nil {
		return nil
	}
	if err := a.telemetry.Emit(event); err != nil {
		return fmt.Errorf("Countess route telemetry: %w", err)
	}
	return nil
}
