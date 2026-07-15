package pathing

import (
	"context"
	"testing"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

func TestRoutePlayerChainsOnlyConfirmedSegments(t *testing.T) {
	route := validRoute()
	nav := &segmentNavigatorMock{next: NavTickResult{Status: NavMoving}}
	player, err := NewRoutePlayer(nav, route)
	if err != nil {
		t.Fatal(err)
	}
	state := segmentPlaybackState(world.BlackMarsh, 14858, 5068)
	if done, tickErr := player.Tick(context.Background(), state); tickErr != nil || done {
		t.Fatalf("start = %t, %v", done, tickErr)
	}
	state.Player.Position = world.Position{X: 14820, Y: 5065}
	nav.active = false
	_, _ = player.Tick(context.Background(), state)
	state.Entrances = []world.Entrance{{UnitID: 20, Kind: world.EntranceKindWildernessToTower, Position: state.Player.Position}}
	_, _ = player.Tick(context.Background(), state)
	state.Area = world.LookupArea(world.ForgottenTower)
	state.Player.Position = world.Position{X: 1000, Y: 1000}
	if done, tickErr := player.Tick(context.Background(), state); tickErr != nil || done || player.SegmentIndex() != 1 {
		t.Fatalf("boundary = %t, %v index=%d", done, tickErr, player.SegmentIndex())
	}
	_, _ = player.Tick(context.Background(), state)
	state.Entrances = []world.Entrance{{UnitID: 30, Kind: world.EntranceKindUnknown, Position: state.Player.Position}}
	_, _ = player.Tick(context.Background(), state)
	state.Area = world.LookupArea(world.TowerCellarLevel1)
	done, err := player.Tick(context.Background(), state)
	if err != nil || !done {
		t.Fatalf("complete = %t, %v", done, err)
	}
}

func TestRoutePlayerStopsOnUnexpectedBoundary(t *testing.T) {
	player, _ := NewRoutePlayer(&segmentNavigatorMock{}, validRoute())
	state := segmentPlaybackState(world.TowerCellarLevel1, 1000, 1000)
	if _, err := player.Tick(context.Background(), state); err == nil {
		t.Fatal("unexpected area was accepted")
	}
}

func TestRoutePlayerSupportsSecondSyntheticRoute(t *testing.T) {
	route := validRoute()
	route.ID = "second-navigation-route"
	route.Name = "Second Navigation Route"
	route.Tags = []string{"act1", "second-test"}

	player, err := NewRoutePlayer(&segmentNavigatorMock{}, route)
	if err != nil {
		t.Fatal(err)
	}
	state := segmentPlaybackState(world.BlackMarsh, 14858, 5068)
	if done, err := player.Tick(context.Background(), state); err != nil || done {
		t.Fatalf("second route start = %t, %v", done, err)
	}
}
