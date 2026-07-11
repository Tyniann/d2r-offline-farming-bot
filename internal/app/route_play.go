package app

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/pathing"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/telemetry"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

// RunRoutePlay replays every segment in one continuously verified session.
func (rt *Runtime) RunRoutePlay(routeID string) (retErr error) {
	registry, err := pathing.LoadRouteRegistry(rt.Config.ResolvePath(rt.Config.Routes.Directory))
	if err != nil {
		return err
	}
	route, err := registry.Get(routeID)
	if err != nil {
		return err
	}
	player, err := pathing.NewRoutePlayer(rt.Pathing, route)
	if err != nil {
		return err
	}
	trace, err := telemetry.New(rt.Config.Telemetry.Directory, "route-"+routeID, "play")
	if err != nil {
		return fmt.Errorf("create route telemetry: %w", err)
	}
	defer func() {
		if retErr != nil {
			_ = trace.Emit(telemetry.Event{Event: telemetry.RoutePlaybackFailed, RouteID: routeID, SegmentID: player.Segment().ID, Reason: retErr.Error()})
		}
		if err := trace.Close(); err != nil && retErr == nil {
			retErr = err
		}
	}()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	rt.startShutdownSignals(ctx, cancel)
	defer func() {
		player.Reset()
		if err := rt.Process.Detach(); err != nil {
			rt.Log.Warn("detach after route playback", "error", err)
		}
	}()
	defer rt.Input.Unbind()
	hotkeys, err := rt.startHotkeys(ctx)
	if err != nil {
		return err
	}
	ticker := time.NewTicker(time.Duration(rt.Config.Runtime.PollIntervalMs) * time.Millisecond)
	defer ticker.Stop()
	state := &runState{}
	started := false
	transitioning := false
	lastSegment, lastPoint := -1, -1
	var deadline time.Time
	rt.Log.Info("route playback waiting for verified start", "route_id", routeID, "segments", len(route.Segments), "telemetry_path", trace.Path())
	for {
		select {
		case <-ctx.Done():
			if err := trace.Emit(telemetry.Event{Event: telemetry.RoutePlaybackStopped, RouteID: routeID, SegmentID: player.Segment().ID, Reason: "operator_stop"}); err != nil {
				return err
			}
			return nil
		case event := <-hotkeys:
			rt.handleHotkeyEvent(event, cancel)
		case <-ticker.C:
			if err := rt.runTick(ctx, state); err != nil && !errors.Is(err, context.Canceled) {
				return err
			}
			if state.hasEverAttached && !state.attached {
				return fmt.Errorf("route playback: process lost")
			}
			if rt.Input.Status().Paused {
				if started {
					deadline = deadline.Add(time.Duration(rt.Config.Runtime.PollIntervalMs) * time.Millisecond)
				}
				continue
			}
			cur := rt.World.Current()
			if !started {
				if !cur.Valid || cur.Phase != world.GamePhaseInGame || !cur.Identity.Valid {
					continue
				}
				fingerprint, err := pathing.BuildLayoutFingerprint(cur)
				if err != nil {
					return fmt.Errorf("route layout: %w", err)
				}
				if err := pathing.ValidateRoutePrecheck(route, pathing.RoutePrecheckInput{Identity: cur.Identity, GameVersion: rt.Config.Memory.GameVersion, Layout: fingerprint, World: cur}); err != nil {
					return fmt.Errorf("route precheck: %w", err)
				}
				started = true
				deadline = time.Now().Add(time.Duration(route.Playback.SegmentTimeoutMs) * time.Millisecond)
				if err := trace.Emit(telemetry.Event{Event: telemetry.RoutePlaybackStarted, RouteID: routeID, SegmentID: player.Segment().ID, AreaID: uint32(cur.Area.ID)}); err != nil {
					return err
				}
				rt.Log.Info("route playback started", "route_id", routeID, "character", cur.Identity.CharacterName, "layout_fingerprint", fingerprint.Hash)
			}
			if time.Now().After(deadline) {
				if transitioning {
					return fmt.Errorf("%w: timeout", pathing.ErrRouteTransitionFailed)
				}
				return fmt.Errorf("route segment timeout: %s", player.Segment().ID)
			}
			beforeSegment := player.SegmentIndex()
			done, err := player.Tick(ctx, cur)
			if err != nil {
				return err
			}
			if !done && player.SegmentIndex() != beforeSegment {
				completed := route.Segments[beforeSegment]
				idx := beforeSegment
				if err := trace.Emit(telemetry.Event{Event: telemetry.RouteSegmentCompleted, RouteID: routeID, SegmentID: completed.ID, SegmentIndex: &idx, AreaID: uint32(cur.Area.ID)}); err != nil {
					return err
				}
				transitioning = false
				lastPoint = -1
				deadline = time.Now().Add(time.Duration(route.Playback.SegmentTimeoutMs) * time.Millisecond)
				rt.Log.Info("route playback segment completed", "route_id", routeID, "segment_id", completed.ID, "area_id", cur.Area.ID)
			}
			if done {
				idx := len(route.Segments) - 1
				if err := trace.Emit(telemetry.Event{Event: telemetry.RouteSegmentCompleted, RouteID: routeID, SegmentID: route.Segments[idx].ID, SegmentIndex: &idx, AreaID: uint32(cur.Area.ID)}); err != nil {
					return err
				}
				if err := trace.Emit(telemetry.Event{Event: telemetry.RoutePlaybackCompleted, RouteID: routeID, SegmentID: route.Segments[idx].ID, AreaID: uint32(cur.Area.ID)}); err != nil {
					return err
				}
				rt.Log.Info("route playback completed", "route_id", routeID, "segments", len(route.Segments), "area_id", cur.Area.ID)
				return nil
			}
			if player.SegmentIndex() != lastSegment {
				lastSegment = player.SegmentIndex()
				lastPoint = -1
			}
			if player.Transitioning() && !transitioning {
				transitioning = true
				deadline = time.Now().Add(time.Duration(route.Playback.TransitionTimeoutMs) * time.Millisecond)
				idx := player.SegmentIndex()
				if err := trace.Emit(telemetry.Event{Event: telemetry.RouteTransitionStarted, RouteID: routeID, SegmentID: player.Segment().ID, SegmentIndex: &idx, TargetAreaID: uint32(player.Segment().ToAreaID)}); err != nil {
					return err
				}
				rt.Log.Info("route playback transition started", "route_id", routeID, "segment_id", player.Segment().ID, "target_area_id", player.Segment().ToAreaID)
			}
			if point := player.PointIndex(); !player.Transitioning() && point != lastPoint {
				lastPoint = point
				if target, ok := player.CurrentTarget(); ok {
					idx, pointIdx := player.SegmentIndex(), point
					if err := trace.Emit(telemetry.Event{Event: telemetry.RoutePointStarted, RouteID: routeID, SegmentID: player.Segment().ID, SegmentIndex: &idx, PointIndex: &pointIdx, TargetX: target.X, TargetY: target.Y, AreaID: uint32(cur.Area.ID)}); err != nil {
						return err
					}
				}
			}
		}
	}
}
