package app

import (
	"context"
	"fmt"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/pathing"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/tasks"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/town"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

type freshGameEgress interface {
	StartFrom(town.OriginAct, town.Anchor, world.State) error
	Tick(context.Context, world.State) (bool, error)
	Reset()
}

type freshGameNormalizer struct {
	egress     freshGameEgress
	waypoint   tasks.WaypointActions
	originAct  town.OriginAct
	originArea world.AreaID
	stage      int
	started    bool
}

func newFreshGameNormalizer(egress freshGameEgress, waypoint tasks.WaypointActions) *freshGameNormalizer {
	return &freshGameNormalizer{egress: egress, waypoint: waypoint}
}

func (n *freshGameNormalizer) Start(state world.State) (bool, error) {
	if n == nil || n.egress == nil || n.waypoint == nil {
		return false, fmt.Errorf("fresh game normalization is not wired")
	}
	if err := validateFreshGameTownState(state); err != nil {
		return false, err
	}
	act, normalize, err := freshGameOriginAct(state.Area.ID)
	if err != nil {
		return false, err
	}
	if !normalize {
		return true, nil
	}
	n.egress.Reset()
	n.waypoint.Reset()
	if err := n.egress.StartFrom(act, town.AnchorSpawn, state); err != nil {
		return false, fmt.Errorf("start fresh game spawn egress for %s: %w", act, err)
	}
	n.originAct = act
	n.originArea = state.Area.ID
	n.stage = 0
	n.started = true
	return false, nil
}

func (n *freshGameNormalizer) Tick(ctx context.Context, state world.State) (bool, error) {
	if n == nil || !n.started {
		return false, fmt.Errorf("fresh game normalization not started")
	}
	if n.stage < 3 {
		if !state.Valid || state.Phase != world.GamePhaseInGame {
			return false, nil
		}
		if state.Area.ID != n.originArea {
			return false, fmt.Errorf("fresh game normalization left %s before waypoint transfer", world.LookupArea(n.originArea).Name)
		}
		if queueGameUIBlocked(state) {
			return false, fmt.Errorf("fresh game normalization blocked by open UI")
		}
	}
	switch n.stage {
	case 0:
		done, err := n.egress.Tick(ctx, state)
		if err != nil {
			return false, fmt.Errorf("walk fresh game spawn egress for %s: %w", n.originAct, err)
		}
		if done {
			n.stage = 1
		}
	case 1:
		result := n.waypoint.TickTownWaypoint(ctx, state)
		if result.Done && result.Status != pathing.WaypointActionClicked {
			return false, fmt.Errorf("open %s waypoint: status=%s reason=%s", n.originAct, result.Status, result.Reason)
		}
		if result.Status == pathing.WaypointActionClicked {
			n.stage = 2
		}
	case 2:
		result := n.waypoint.SelectWaypointTarget(ctx, state, pathing.WaypointTargetRogueEncampment, time.Now())
		if result.Done && result.Status != pathing.WaypointActionClicked {
			return false, fmt.Errorf("select Rogue Encampment from %s: status=%s reason=%s", n.originAct, result.Status, result.Reason)
		}
		if result.Status == pathing.WaypointActionClicked {
			n.stage = 3
		}
	case 3:
		if !state.Valid || state.Phase != world.GamePhaseInGame {
			return false, nil
		}
		// D2R can keep the phase at `in_game` while briefly clearing the area
		// during a waypoint load. The outer normalization timeout bounds this
		// wait; only a concrete, unexpected destination is terminal here.
		if state.Area.ID == 0 {
			return false, nil
		}
		if state.Area.ID == world.RogueEncampment {
			return true, nil
		}
		if state.Area.ID != n.originArea {
			return false, fmt.Errorf("fresh game waypoint transfer reached unexpected area %s", state.Area.Name)
		}
	}
	return false, nil
}

func validateFreshGameTownState(state world.State) error {
	if !state.Valid || state.Phase != world.GamePhaseInGame || !state.Identity.Valid {
		return fmt.Errorf("fresh game town identity is unavailable")
	}
	if queueGameUIBlocked(state) {
		return fmt.Errorf("fresh game normalization blocked by open UI")
	}
	_, _, err := freshGameOriginAct(state.Area.ID)
	return err
}

func freshGameOriginAct(area world.AreaID) (town.OriginAct, bool, error) {
	switch area {
	case world.RogueEncampment:
		return town.OriginActUnknown, false, nil
	case world.LutGholein:
		return town.OriginAct2, true, nil
	case world.KurastDocks:
		return town.OriginAct3, true, nil
	case world.ThePandemoniumFortress:
		return town.OriginAct4, true, nil
	case world.Harrogath:
		return town.OriginAct5, true, nil
	default:
		return town.OriginActUnknown, false, fmt.Errorf("fresh game start area %s is not a supported town", stateAreaName(area))
	}
}

func stateAreaName(area world.AreaID) string {
	name := world.LookupArea(area).Name
	if name == "" {
		return fmt.Sprintf("%d", area)
	}
	return name
}
