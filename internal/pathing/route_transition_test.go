package pathing

import (
	"context"
	"errors"
	"testing"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

func TestRouteTransitionHandlerPinsMatchingEntrance(t *testing.T) {
	nav := &segmentNavigatorMock{next: NavTickResult{Status: NavClicking}}
	segment := validRoute().Segments[0]
	handler := NewRouteTransitionHandler(nav, segment, 2)
	state := segmentPlaybackState(world.BlackMarsh, 14820, 5065)
	state.Entrances = []world.Entrance{
		{UnitID: 10, Kind: world.EntranceKindTowerToWilderness, Position: world.Position{X: 14819, Y: 5065}},
		{UnitID: 20, Kind: world.EntranceKindWildernessToTower, Position: world.Position{X: 14821, Y: 5065}},
	}
	if done, err := handler.Tick(context.Background(), state); err != nil || done {
		t.Fatalf("Tick() = %t, %v", done, err)
	}
	if len(nav.goals) != 1 || nav.goals[0].ViaEntranceUnitID != 20 || !nav.goals[0].StrictEntrance {
		t.Fatalf("goal = %+v", nav.goals)
	}
	state.Area = world.LookupArea(world.ForgottenTower)
	if done, err := handler.Tick(context.Background(), state); err != nil || !done {
		t.Fatalf("complete = %t, %v", done, err)
	}
}

func TestRouteTransitionHandlerPinsDuranceDownEntrance(t *testing.T) {
	nav := &segmentNavigatorMock{next: NavTickResult{Status: NavClicking}}
	segment := RouteSegment{
		FromAreaID: world.DuranceOfHateLevel2,
		ToAreaID:   world.DuranceOfHateLevel3,
		Transition: RouteTransition{Type: "entrance", EntranceKind: "durance_down"},
	}
	handler := NewRouteTransitionHandler(nav, segment, 2)
	state := segmentPlaybackState(world.DuranceOfHateLevel2, 17705, 6513)
	state.Entrances = []world.Entrance{
		{UnitID: 1, Kind: world.EntranceKindDuranceDown, Position: world.Position{X: 17710, Y: 6511}},
		{UnitID: 2, Kind: world.EntranceKindDuranceUp, Position: world.Position{X: 17685, Y: 8021}},
	}
	if done, err := handler.Tick(context.Background(), state); err != nil || done {
		t.Fatalf("Tick() = %t, %v", done, err)
	}
	if len(nav.goals) != 1 || nav.goals[0].ViaEntrance != world.EntranceKindDuranceDown || nav.goals[0].ViaEntranceUnitID != 1 || !nav.goals[0].StrictEntrance {
		t.Fatalf("Durance goal = %+v", nav.goals)
	}
}

func TestRouteTransitionHandlerRejectsWrongEntrance(t *testing.T) {
	nav := &segmentNavigatorMock{}
	handler := NewRouteTransitionHandler(nav, validRoute().Segments[0], 2)
	state := segmentPlaybackState(world.BlackMarsh, 14820, 5065)
	state.Entrances = []world.Entrance{{UnitID: 10, Kind: world.EntranceKindTowerToWilderness, Position: state.Player.Position}}
	if done, err := handler.Tick(context.Background(), state); err != nil || done {
		t.Fatalf("Tick() = %t, %v", done, err)
	}
	if len(nav.goals) != 0 {
		t.Fatalf("wrong entrance started goals: %+v", nav.goals)
	}
}

func TestRouteTransitionHandlerPinsPermanentPortalUnit(t *testing.T) {
	nav := &segmentNavigatorMock{next: NavTickResult{Status: NavClicking}}
	segment := RouteSegment{FromAreaID: world.StonyField, ToAreaID: world.Tristram, Transition: RouteTransition{Type: "object_portal", ObjectKind: world.ObjectKindPermanentPortal, ExpectedToArea: world.Tristram}}
	handler := NewRouteTransitionHandler(nav, segment, 2)
	state := segmentPlaybackState(world.StonyField, 100, 100)
	state.Objects = []world.Object{{Kind: world.ObjectKindPermanentPortal, UnitID: 81, Position: world.Position{X: 102, Y: 100}}}
	if done, err := handler.Tick(context.Background(), state); err != nil || done {
		t.Fatalf("Tick() = %t, %v", done, err)
	}
	if len(nav.goals) != 1 || nav.goals[0].ViaObject != world.ObjectKindPermanentPortal || nav.goals[0].ViaObjectUnitID != 81 || !nav.goals[0].StrictObject || nav.goals[0].TargetArea != world.Tristram {
		t.Fatalf("portal goal = %+v", nav.goals)
	}
}

func TestRouteTransitionHandlerBoundsNavigatorRecovery(t *testing.T) {
	nav := &segmentNavigatorMock{next: NavTickResult{Done: true, Status: NavFailed, Reason: ReasonHoverNotFound}}
	segment := validRoute().Segments[0]
	handler := NewRouteTransitionHandler(nav, segment, 1)
	state := segmentPlaybackState(world.BlackMarsh, 14820, 5065)
	state.Entrances = []world.Entrance{{UnitID: 20, Kind: world.EntranceKindWildernessToTower, Position: state.Player.Position}}
	if _, err := handler.Tick(context.Background(), state); err != nil {
		t.Fatalf("first failure = %v", err)
	}
	if _, err := handler.Tick(context.Background(), state); !errors.Is(err, ErrRouteTransitionFailed) {
		t.Fatalf("second failure = %v", err)
	}
}
