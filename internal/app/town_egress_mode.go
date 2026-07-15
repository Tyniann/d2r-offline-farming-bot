package app

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/pathing"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/town"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

func (rt *Runtime) act3EgressConfig() (town.EgressConfig, error) {
	egress, reason := rt.Config.Town.EgressFor(town.OriginAct3)
	if reason != "" {
		return town.EgressConfig{}, fmt.Errorf("Act-3 egress: %s", reason)
	}
	return egress, nil
}

// RunTownEgressInspect reports the live, read-only Kurast binding and waypoint anchor.
func (rt *Runtime) RunTownEgressInspect() error {
	egress, err := rt.act3EgressConfig()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	rt.startShutdownSignals(ctx, cancel)
	defer func() {
		if detachErr := rt.Process.Detach(); detachErr != nil {
			rt.Log.Warn("process detach failed", "error", detachErr)
		}
	}()
	defer rt.Input.Unbind()
	ticker := time.NewTicker(time.Duration(rt.Config.Runtime.PollIntervalMs) * time.Millisecond)
	defer ticker.Stop()
	state := &runState{}
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("Act-3 egress inspect timeout waiting for Kurast Docks identity and waypoint")
		case <-ticker.C:
			if err := rt.runTick(ctx, state); err != nil && !errors.Is(err, context.Canceled) {
				return err
			}
			current := rt.World.Current()
			if !current.Valid || !current.Identity.Valid || current.Area.ID != world.KurastDocks {
				continue
			}
			waypoint, ok := current.NearestObject(world.ObjectKindWaypoint)
			if !ok {
				continue
			}
			fingerprint, err := pathing.BuildLayoutFingerprint(current)
			if err != nil {
				continue
			}
			rt.Log.Info("Act-3 egress live binding confirmed", "route_id", egress.RouteID, "character", current.Identity.CharacterName, "character_class", current.Identity.Class, "map_seed", current.Identity.MapSeed, "game_version", rt.Config.Memory.GameVersion, "area_id", current.Area.ID, "player_x", current.Player.Position.X, "player_y", current.Player.Position.Y, "waypoint_unit_id", waypoint.UnitID, "waypoint_x", waypoint.Position.X, "waypoint_y", waypoint.Position.Y, "layout_fingerprint", fingerprint.Hash)
			return nil
		}
	}
}

// RunTownEgressRecord records the configured Act-3 portal-arrival-to-waypoint walk route.
func (rt *Runtime) RunTownEgressRecord(name, difficulty string) error {
	egress, err := rt.act3EgressConfig()
	if err != nil {
		return err
	}
	return rt.runRouteRecord(egress.RouteID, name, difficulty, pathing.RouteMovementWalk, rt.Config.ResolvePath(egress.RoutesDirectory), world.KurastDocks)
}

// RunTownEgressValidate structurally validates the configured Act-3 egress asset.
func (rt *Runtime) RunTownEgressValidate() error {
	egress, err := rt.act3EgressConfig()
	if err != nil {
		return err
	}
	registry, err := pathing.LoadRouteRegistry(rt.Config.ResolvePath(egress.RoutesDirectory))
	if err != nil {
		return err
	}
	route, err := registry.Get(egress.RouteID)
	if err != nil {
		return err
	}
	if err := validateAct3EgressRoute(route); err != nil {
		return err
	}
	rt.Log.Info("Act-3 egress route valid", "route_id", route.ID, "movement", route.Segments[0].Movement, "point_count", len(route.Segments[0].Points), "layout_fingerprint", route.Binding.LayoutFingerprint.Hash)
	return nil
}

// RunTownEgressPlay performs the isolated Kurast walk and registered transfer to Rogue Encampment.
func (rt *Runtime) RunTownEgressPlay() error {
	if !rt.Input.Status().Enabled {
		return fmt.Errorf("Act-3 egress playback requires input.enabled=true")
	}
	ctx, cancel := context.WithCancel(context.Background())
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
		return err
	}
	defer rt.stopHotkeys(cancel)
	ticker := time.NewTicker(time.Duration(rt.Config.Runtime.PollIntervalMs) * time.Millisecond)
	defer ticker.Stop()
	state := &runState{}
	deadline := time.Now().Add(rt.attachTimeoutOrDefault(2 * time.Minute))
	if err := rt.waitPathingTestReady(ctx, state, hotkeys, ticker, deadline, cancel, true); err != nil {
		return err
	}
	stage := 0
	rt.townEgress.Reset()
	rt.taskDeps.Waypoint.Reset()
	for time.Now().Before(deadline) {
		current, stop, err := rt.pathingTestTick(ctx, state, hotkeys, ticker, cancel)
		if err != nil {
			return err
		}
		if stop {
			return nil
		}
		switch stage {
		case 0:
			if !current.Valid || current.Area.ID != world.KurastDocks {
				continue
			}
			if err := rt.townEgress.Start(town.OriginAct3, current); err != nil {
				// The isolated test may become input-ready one or two ticks before
				// the read-only identity probe reaches its consistency threshold.
				if errors.Is(err, pathing.ErrGameIdentityUnavailable) {
					continue
				}
				return err
			}
			stage = 1
		case 1:
			done, err := rt.townEgress.Tick(ctx, current)
			if err != nil {
				return err
			}
			if done {
				stage = 2
			}
		case 2:
			result := rt.taskDeps.Waypoint.TickTownWaypoint(ctx, current)
			if result.Done && result.Status != pathing.WaypointActionClicked {
				return fmt.Errorf("Act-3 waypoint open failed: status=%s reason=%s", result.Status, result.Reason)
			}
			if result.Status == pathing.WaypointActionClicked {
				stage = 3
			}
		case 3:
			if !current.UI.WaypointOpen {
				continue
			}
			result := rt.taskDeps.Waypoint.SelectWaypointTarget(ctx, current, pathing.WaypointTargetRogueEncampment, time.Now())
			if result.Done && result.Status != pathing.WaypointActionClicked {
				return fmt.Errorf("rogue encampment selection failed: status=%s reason=%s", result.Status, result.Reason)
			}
			if result.Status == pathing.WaypointActionClicked {
				stage = 4
			}
		case 4:
			if current.Area.ID == world.RogueEncampment {
				rt.Log.Info("Act-3 egress acceptance completed", "target_area_id", current.Area.ID, "outcome", "success")
				return nil
			}
		}
	}
	return fmt.Errorf("Act-3 egress playback timeout at stage %d", stage)
}
