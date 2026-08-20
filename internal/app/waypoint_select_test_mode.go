package app

import (
	"context"
	"fmt"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/pathing"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

const (
	waypointLowerKurastTownTest    = "waypoint:lower_kurast"
	waypointSelectTownTestTimeout  = 2 * time.Minute
	waypointSelectTownTestOpen     = 0
	waypointSelectTownTestSelect   = 1
	waypointSelectTownTestWaitArea = 2
)

// runLowerKurastWaypointTownTest opens the town waypoint, selects
// [pathing.WaypointTargetLowerKurast] with the Durance executor, and waits for Area 79.
func (rt *Runtime) runLowerKurastWaypointTownTest() error {
	if !rt.Input.Status().Enabled {
		return fmt.Errorf("waypoint test requires input.enabled=true")
	}
	if rt.taskDeps.Waypoint == nil {
		return fmt.Errorf("waypoint test: waypoint actions not wired")
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
	if readyErr := rt.waitPathingTestReady(ctx, state, hotkeys, ticker, time.Now().Add(rt.attachTimeoutOrDefault(60*time.Second)), cancel, true); readyErr != nil {
		return readyErr
	}

	rt.taskDeps.Waypoint.Reset()
	stage := waypointSelectTownTestOpen
	deadline := time.Now().Add(waypointSelectTownTestTimeout)
	rt.Log.Info("waypoint select acceptance started", "target", pathing.WaypointTargetLowerKurast, "expected_area_id", world.LowerKurast)
	for time.Now().Before(deadline) {
		current, stop, tickErr := rt.pathingTestTick(ctx, state, hotkeys, ticker, cancel)
		if tickErr != nil {
			return tickErr
		}
		if stop {
			return nil
		}
		if !current.Valid || current.Phase != world.GamePhaseInGame {
			continue
		}
		switch stage {
		case waypointSelectTownTestOpen:
			if !current.Area.IsTown() {
				return fmt.Errorf("waypoint test must start in town, area=%s id=%d", current.Area.Name, current.Area.ID)
			}
			result := rt.taskDeps.Waypoint.TickTownWaypoint(ctx, current)
			if result.Done && result.Status != pathing.WaypointActionClicked {
				return fmt.Errorf("waypoint open failed: status=%s reason=%s", result.Status, result.Reason)
			}
			if result.Status == pathing.WaypointActionClicked {
				stage = waypointSelectTownTestSelect
			}
		case waypointSelectTownTestSelect:
			if !current.UI.WaypointOpen {
				continue
			}
			result := rt.taskDeps.Waypoint.SelectWaypointTarget(ctx, current, pathing.WaypointTargetLowerKurast, time.Now())
			if result.Done && result.Status != pathing.WaypointActionClicked {
				return fmt.Errorf("waypoint selection failed: status=%s reason=%s", result.Status, result.Reason)
			}
			if result.Status == pathing.WaypointActionClicked {
				stage = waypointSelectTownTestWaitArea
			}
		case waypointSelectTownTestWaitArea:
			if current.Area.ID == world.LowerKurast {
				rt.Log.Info("waypoint select acceptance completed", "target", pathing.WaypointTargetLowerKurast, "area_id", current.Area.ID, "outcome", "success")
				return nil
			}
			if current.Area.IsTown() {
				continue
			}
			return fmt.Errorf("waypoint test arrived in unexpected area %s id=%d, want Lower Kurast %d", current.Area.Name, current.Area.ID, world.LowerKurast)
		}
	}
	return fmt.Errorf("waypoint test timeout after %s at stage %d", waypointSelectTownTestTimeout, stage)
}
