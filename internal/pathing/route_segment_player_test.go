package pathing

import (
	"context"
	"errors"
	"testing"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

type segmentNavigatorMock struct {
	active bool
	goals  []Goal
	next   NavTickResult
	resets int
}

func (m *segmentNavigatorMock) Start(goal Goal) error {
	m.active = true
	m.goals = append(m.goals, goal)
	return nil
}
func (m *segmentNavigatorMock) Tick(context.Context, world.State) NavTickResult {
	result := m.next
	if result.Done {
		m.active = false
	}
	return result
}
func (m *segmentNavigatorMock) Active() bool { return m.active }
func (m *segmentNavigatorMock) Reset()       { m.active = false; m.resets++ }

func segmentPlaybackState(area world.AreaID, x, y uint32) world.State {
	return world.State{Valid: true, Phase: world.GamePhaseInGame, Area: world.LookupArea(area), Player: world.Player{Position: world.Position{X: x, Y: y}}, Identity: world.GameIdentity{Valid: true, CharacterName: "MrBones", Class: world.CharacterClassNecromancer}}
}

func TestRouteSegmentPlayerReplaysPointsAndStrictTransition(t *testing.T) {
	route := validRoute()
	nav := &segmentNavigatorMock{next: NavTickResult{Status: NavMoving}}
	player, err := NewRouteSegmentPlayer(nav, route, 0)
	if err != nil {
		t.Fatal(err)
	}
	state := segmentPlaybackState(world.BlackMarsh, 14858, 5068)
	if _, err := player.Tick(context.Background(), state); err != nil {
		t.Fatal(err)
	}
	if len(nav.goals) != 1 || nav.goals[0].Kind != GoalKindMoveToPosition {
		t.Fatalf("goals = %+v", nav.goals)
	}
	state.Player.Position = world.Position{X: 14820, Y: 5065}
	nav.active = false
	if _, err := player.Tick(context.Background(), state); err != nil {
		t.Fatal(err)
	}
	state.Entrances = []world.Entrance{{UnitID: 20, Kind: world.EntranceKindWildernessToTower, Position: state.Player.Position}}
	if _, err := player.Tick(context.Background(), state); err != nil {
		t.Fatal(err)
	}
	if len(nav.goals) != 2 || nav.goals[1].Kind != GoalKindMoveToArea || !nav.goals[1].StrictEntrance {
		t.Fatalf("transition goal = %+v", nav.goals)
	}
	state.Area = world.LookupArea(world.ForgottenTower)
	done, err := player.Tick(context.Background(), state)
	if err != nil || !done {
		t.Fatalf("done=%t err=%v", done, err)
	}
}

func TestRouteSegmentPlayerRejectsDriftAndArea(t *testing.T) {
	route := validRoute()
	route.Playback.MaxLocalCorrections = 0
	player, _ := NewRouteSegmentPlayer(&segmentNavigatorMock{}, route, 0)
	if _, err := player.Tick(context.Background(), segmentPlaybackState(world.BlackMarsh, 14950, 5200)); !errors.Is(err, ErrRouteDriftExceeded) {
		t.Fatalf("drift error = %v", err)
	}
	player, _ = NewRouteSegmentPlayer(&segmentNavigatorMock{}, route, 0)
	if _, err := player.Tick(context.Background(), segmentPlaybackState(world.ForgottenTower, 14858, 5068)); !errors.Is(err, ErrRouteUnexpectedArea) {
		t.Fatalf("area error = %v", err)
	}
}

func TestRouteSegmentPlayerLimitsCorrections(t *testing.T) {
	route := validRoute()
	route.Playback.MaxLocalCorrections = 1
	nav := &segmentNavigatorMock{next: NavTickResult{Done: true, Status: NavStuck, Reason: ReasonStuck}}
	player, _ := NewRouteSegmentPlayer(nav, route, 0)
	state := segmentPlaybackState(world.BlackMarsh, 14858, 5068)
	if _, err := player.Tick(context.Background(), state); err != nil {
		t.Fatalf("first failure = %v", err)
	}
	if _, err := player.Tick(context.Background(), state); !errors.Is(err, ErrRouteSegmentFailed) {
		t.Fatalf("second failure = %v", err)
	}
}

func TestRouteSegmentPlayerDriftRecoversToPreviousPoint(t *testing.T) {
	route := validRoute()
	nav := &segmentNavigatorMock{next: NavTickResult{Status: NavMoving}}
	player, _ := NewRouteSegmentPlayer(nav, route, 0)
	state := segmentPlaybackState(world.BlackMarsh, 14858, 5068)
	_, _ = player.Tick(context.Background(), state)
	nav.active = false
	state.Player.Position = world.Position{X: 14900, Y: 5100}
	if _, err := player.Tick(context.Background(), state); err != nil {
		t.Fatalf("recovery error = %v", err)
	}
	last := nav.goals[len(nav.goals)-1]
	if last.TargetPos != (world.Position{X: 14858, Y: 5068}) {
		t.Fatalf("recovery target = %+v", last.TargetPos)
	}
}

func TestRouteSegmentPlayerStopResetsWithoutFurtherGoals(t *testing.T) {
	route := validRoute()
	nav := &segmentNavigatorMock{}
	player, _ := NewRouteSegmentPlayer(nav, route, 0)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := player.Tick(ctx, segmentPlaybackState(world.BlackMarsh, 14858, 5068)); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel error = %v", err)
	}
	if len(nav.goals) != 0 || nav.resets != 1 {
		t.Fatalf("goals=%d resets=%d", len(nav.goals), nav.resets)
	}
}
