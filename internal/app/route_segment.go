package app

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/pathing"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

// RunRouteSegment replays exactly one named segment and its confirmed transition.
func (rt *Runtime) RunRouteSegment(routeID, segmentID string) error {
	registry, err := pathing.LoadRouteRegistry(rt.Config.ResolvePath(rt.Config.Routes.Directory))
	if err != nil {
		return err
	}
	route, err := registry.Get(routeID)
	if err != nil {
		return err
	}
	segmentIndex := -1
	for i := range route.Segments {
		if route.Segments[i].ID == segmentID {
			segmentIndex = i
			break
		}
	}
	if segmentIndex < 0 {
		return fmt.Errorf("route segment not found: %s/%s", routeID, segmentID)
	}
	player, err := pathing.NewRouteSegmentPlayer(rt.Pathing, route, segmentIndex)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	rt.startShutdownSignals(ctx, cancel)
	defer func() {
		if detachErr := rt.Process.Detach(); detachErr != nil {
			rt.Log.Warn("detach after segment playback", "error", detachErr)
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
	var deadline time.Time
	rt.Log.Info("route segment playback waiting for start state", "route_id", routeID, "segment_id", segmentID, "from_area_id", player.Segment().FromAreaID, "to_area_id", player.Segment().ToAreaID)
	for {
		select {
		case <-ctx.Done():
			rt.Pathing.Reset()
			return nil
		case event := <-hotkeys:
			rt.handleHotkeyEvent(event, cancel)
		case <-ticker.C:
			if err := rt.runTick(ctx, state); err != nil && !errors.Is(err, context.Canceled) {
				return err
			}
			if state.hasEverAttached && !state.attached {
				return fmt.Errorf("route segment playback: process lost")
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
				if err := pathing.ValidateRouteSegmentStart(route, segmentIndex, cur.Identity, rt.Config.Memory.GameVersion, cur); err != nil {
					return fmt.Errorf("route segment precheck: %w", err)
				}
				if segmentIndex == 0 {
					fingerprint, err := pathing.BuildLayoutFingerprint(cur)
					if err != nil {
						return fmt.Errorf("route segment layout: %w", err)
					}
					if err := pathing.ValidateRoutePrecheck(route, pathing.RoutePrecheckInput{Identity: cur.Identity, GameVersion: rt.Config.Memory.GameVersion, Layout: fingerprint, World: cur}); err != nil {
						return fmt.Errorf("route segment precheck: %w", err)
					}
				}
				started = true
				deadline = time.Now().Add(time.Duration(route.Playback.SegmentTimeoutMs) * time.Millisecond)
				rt.Log.Info("route segment playback started", "route_id", routeID, "segment_id", segmentID, "point_count", len(player.Segment().Points))
			}
			if time.Now().After(deadline) {
				rt.Pathing.Reset()
				if transitioning {
					return fmt.Errorf("%w: timeout", pathing.ErrRouteTransitionFailed)
				}
				return fmt.Errorf("route segment timeout")
			}
			done, err := player.Tick(ctx, cur)
			if err != nil {
				return err
			}
			if player.Transitioning() && !transitioning {
				transitioning = true
				deadline = time.Now().Add(time.Duration(route.Playback.TransitionTimeoutMs) * time.Millisecond)
				rt.Log.Info("route segment transition started", "route_id", routeID, "segment_id", segmentID, "target_area_id", player.Segment().ToAreaID)
			}
			if done {
				rt.Log.Info("route segment playback completed", "route_id", routeID, "segment_id", segmentID, "area_id", cur.Area.ID)
				return nil
			}
		}
	}
}
