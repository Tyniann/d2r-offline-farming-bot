package pathing

import (
	"context"
	"errors"
	"fmt"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

var (
	// ErrRouteTransitionFailed indicates exhausted local transition recovery.
	ErrRouteTransitionFailed = errors.New("route transition failed")
	// ErrRouteEntranceUnavailable indicates that no matching runtime transition entity is visible.
	ErrRouteEntranceUnavailable = errors.New("route transition entity unavailable")
)

// RouteTransitionHandler binds a semantic transition to one runtime entrance or
// object portal at a time.
type RouteTransitionHandler struct {
	navigator      SegmentNavigator
	segment        RouteSegment
	maxCorrections int
	corrections    int
	activeUnitID   uint32
	done           bool
}

// NewRouteTransitionHandler creates strict local transition recovery for one segment.
func NewRouteTransitionHandler(navigator SegmentNavigator, segment RouteSegment, maxCorrections int) *RouteTransitionHandler {
	return &RouteTransitionHandler{navigator: navigator, segment: segment, maxCorrections: maxCorrections}
}

// Tick selects a matching entity, drives hover-confirmed interaction, and verifies target Area.
func (h *RouteTransitionHandler) Tick(ctx context.Context, state world.State) (bool, error) {
	if h.done {
		return true, nil
	}
	if ctx.Err() != nil {
		h.navigator.Reset()
		return false, ctx.Err()
	}
	if !state.Valid || state.Phase != world.GamePhaseInGame {
		return false, nil
	}
	if state.Area.ID == h.segment.ToAreaID {
		h.done = true
		h.navigator.Reset()
		return true, nil
	}
	if state.Area.ID != h.segment.FromAreaID {
		h.navigator.Reset()
		return false, fmt.Errorf("%w: got area %d", ErrRouteUnexpectedArea, state.Area.ID)
	}
	if h.activeUnitID != 0 && !h.transitionEntityVisible(state) {
		h.navigator.Reset()
		h.activeUnitID = 0
		if err := h.consumeCorrection(ErrRouteEntranceUnavailable); err != nil {
			return false, err
		}
	}
	if !h.navigator.Active() {
		if h.segment.Transition.Type == "object_portal" {
			portal, ok := selectRouteObject(state, h.segment)
			if !ok {
				return false, nil
			}
			h.activeUnitID = portal.UnitID
			goal := Goal{Kind: GoalKindMoveToArea, TargetArea: h.segment.ToAreaID, ViaObject: portal.Kind, ViaObjectUnitID: portal.UnitID, StrictObject: true}
			if err := h.navigator.Start(goal); err != nil {
				return false, fmt.Errorf("start strict object portal transition: %w", err)
			}
		} else {
			entrance, ok := selectRouteEntrance(state, h.segment)
			if !ok {
				return false, nil
			}
			h.activeUnitID = entrance.UnitID
			goal := Goal{Kind: GoalKindMoveToArea, TargetArea: h.segment.ToAreaID, ViaEntrance: entrance.Kind, ViaEntranceUnitID: entrance.UnitID, StrictEntrance: true}
			if err := h.navigator.Start(goal); err != nil {
				return false, fmt.Errorf("start strict route transition: %w", err)
			}
		}
	}
	result := h.navigator.Tick(ctx, state)
	if result.Done && result.Status != NavArrived {
		h.navigator.Reset()
		h.activeUnitID = 0
		if err := h.consumeCorrection(fmt.Errorf("navigator status=%s reason=%s", result.Status, result.Reason)); err != nil {
			return false, err
		}
	}
	return false, nil
}

func (h *RouteTransitionHandler) transitionEntityVisible(state world.State) bool {
	if h.segment.Transition.Type == "object_portal" {
		for _, object := range state.Objects {
			if object.UnitID == h.activeUnitID && object.Kind == h.segment.Transition.ObjectKind {
				return true
			}
		}
		return false
	}
	return transitionEntranceVisible(state, h.activeUnitID)
}

func selectRouteObject(state world.State, segment RouteSegment) (world.Object, bool) {
	var best world.Object
	bestDistance := 0.0
	found := false
	for _, object := range state.Objects {
		if object.Kind != segment.Transition.ObjectKind {
			continue
		}
		distance := world.Distance(state.Player.Position, object.Position)
		if !found || distance < bestDistance {
			best, bestDistance, found = object, distance, true
		}
	}
	return best, found
}

func (h *RouteTransitionHandler) consumeCorrection(cause error) error {
	if h.corrections >= h.maxCorrections {
		return fmt.Errorf("%w: %v", ErrRouteTransitionFailed, cause)
	}
	h.corrections++
	return nil
}

func selectRouteEntrance(state world.State, segment RouteSegment) (world.Entrance, bool) {
	expected := parseEntranceKind(segment.Transition.EntranceKind)
	var best world.Entrance
	bestDistance := 0.0
	found := false
	for _, entrance := range state.Entrances {
		if expected != world.EntranceKindUnknown && entrance.Kind != expected {
			continue
		}
		if expected == world.EntranceKindUnknown && entrance.Kind != world.EntranceKindUnknown {
			continue
		}
		distance := world.Distance(state.Player.Position, entrance.Position)
		if !found || distance < bestDistance {
			best, bestDistance, found = entrance, distance, true
		}
	}
	return best, found
}

func transitionEntranceVisible(state world.State, unitID uint32) bool {
	for _, entrance := range state.Entrances {
		if entrance.UnitID == unitID {
			return true
		}
	}
	return false
}
