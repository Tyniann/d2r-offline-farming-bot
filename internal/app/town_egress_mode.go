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

func (rt *Runtime) systemEgressConfig(act town.OriginAct) (town.EgressConfig, world.AreaID, error) {
	egress, reason := rt.Config.Town.EgressFor(act)
	if reason != "" {
		return town.EgressConfig{}, world.None, fmt.Errorf("%s egress: %s", act, reason)
	}
	area, ok := town.TownAreaForAct(act)
	if !ok {
		return town.EgressConfig{}, world.None, fmt.Errorf("unsupported system egress act %q", act)
	}
	return egress, area, nil
}

// RunTownEgressInspect reports the live, read-only Town binding and waypoint anchor.
func (rt *Runtime) RunTownEgressInspect(act town.OriginAct) error {
	_, area, err := rt.systemEgressConfig(act)
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
			return fmt.Errorf("%s egress inspect timeout waiting for town identity and waypoint", act)
		case <-ticker.C:
			if err := rt.runTick(ctx, state); err != nil && !errors.Is(err, context.Canceled) {
				return err
			}
			current := rt.World.Current()
			if !current.Valid || current.Area.ID != area {
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
			rt.Log.Info("system egress live binding confirmed", "act", act, "game_version", rt.Config.Memory.GameVersion, "area_id", current.Area.ID, "player_x", current.Player.Position.X, "player_y", current.Player.Position.Y, "waypoint_unit_id", waypoint.UnitID, "waypoint_x", waypoint.Position.X, "waypoint_y", waypoint.Position.Y, "layout_fingerprint", fingerprint.Hash)
			return nil
		}
	}
}

// RunTownEgressRecord records one global portal-arrival-to-waypoint walk route.
func (rt *Runtime) RunTownEgressRecord(act town.OriginAct, name string) error {
	return rt.RunTownEgressRecordFrom(act, town.AnchorPortalArrival, name)
}

// RunTownEgressRecordFrom records a global Town walk from the explicit start
// anchor to the local waypoint.
func (rt *Runtime) RunTownEgressRecordFrom(act town.OriginAct, startAnchor town.Anchor, name string) error {
	egress, area, err := rt.systemEgressConfig(act)
	if err != nil {
		return err
	}
	return rt.runSystemEgressRecord(act, startAnchor, name, rt.Config.ResolvePath(egress.RoutesDirectory), area, nil, nil)
}

// RunTownEgressValidate structurally validates one configured global Egress asset.
func (rt *Runtime) RunTownEgressValidate(act town.OriginAct) error {
	return rt.RunTownEgressValidateFrom(act, town.AnchorPortalArrival)
}

// RunTownEgressValidateFrom structurally validates the configured global
// Egress asset for the explicit portal-arrival or spawn start anchor.
func (rt *Runtime) RunTownEgressValidateFrom(act town.OriginAct, startAnchor town.Anchor) error {
	egress, area, err := rt.systemEgressConfig(act)
	if err != nil {
		return err
	}
	filename, err := town.SystemEgressFilenameForAnchor(startAnchor)
	if err != nil {
		return err
	}
	route, err := town.LoadSystemEgressRoute(rt.Config.ResolvePath(egress.RoutesDirectory + "/" + filename))
	if err != nil {
		return err
	}
	if route.Contract.Act != act || route.Contract.TownArea != area || route.Contract.From != startAnchor {
		return fmt.Errorf("system egress contract does not match %s", act)
	}
	rt.Log.Info("system egress route valid", "act", act, "start_anchor", startAnchor, "movement", route.Contract.Movement, "point_count", len(route.Points), "layout_fingerprint", route.Contract.LayoutFingerprint.Hash)
	return nil
}

// RunTownEgressPlay performs the isolated Town walk and transfer to Rogue Encampment.
func (rt *Runtime) RunTownEgressPlay(act town.OriginAct) error {
	return rt.RunTownEgressPlayFrom(act, town.AnchorPortalArrival)
}

// RunTownEgressPlayFrom performs the isolated Town walk from the explicit
// start anchor and transfers to Rogue Encampment.
func (rt *Runtime) RunTownEgressPlayFrom(act town.OriginAct, startAnchor town.Anchor) error {
	if !rt.Input.Status().Enabled {
		return fmt.Errorf("%s egress playback requires input.enabled=true", act)
	}
	_, area, err := rt.systemEgressConfig(act)
	if err != nil {
		return err
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
	// Dashboard starts leave the browser in front. Reuse the guarded Core focus
	// path before the first playback input instead of relying on operator timing.
	if err := rt.Input.Focus(); err != nil {
		return fmt.Errorf("focus D2R for %s egress playback: %w", act, err)
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
			if !current.Valid || current.Area.ID != area {
				continue
			}
			if err := rt.townEgress.StartFrom(act, startAnchor, current); err != nil {
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
				return fmt.Errorf("%s waypoint open failed: status=%s reason=%s", act, result.Status, result.Reason)
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
				rt.Log.Info("system egress acceptance completed", "act", act, "target_area_id", current.Area.ID, "outcome", "success")
				return nil
			}
		}
	}
	return fmt.Errorf("%s egress playback timeout at stage %d", act, stage)
}
