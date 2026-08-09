package pathing

import (
	"context"
	"errors"
	"testing"
	"time"

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
	if _, tickErr := player.Tick(context.Background(), state); tickErr != nil {
		t.Fatal(tickErr)
	}
	if len(nav.goals) != 1 || nav.goals[0].Kind != GoalKindMoveToPosition {
		t.Fatalf("goals = %+v", nav.goals)
	}
	state.Player.Position = world.Position{X: 14820, Y: 5065}
	nav.active = false
	if _, tickErr := player.Tick(context.Background(), state); tickErr != nil {
		t.Fatal(tickErr)
	}
	state.Entrances = []world.Entrance{{UnitID: 20, Kind: world.EntranceKindWildernessToTower, Position: state.Player.Position}}
	if _, tickErr := player.Tick(context.Background(), state); tickErr != nil {
		t.Fatal(tickErr)
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

func TestRouteSegmentPlayerProgressIsReadOnlyAndTracksEffectiveTarget(t *testing.T) {
	route := validRoute()
	nav := &segmentNavigatorMock{next: NavTickResult{Status: NavMoving}}
	player, _ := NewRouteSegmentPlayer(nav, route, 0)
	state := segmentPlaybackState(world.BlackMarsh, 14858, 5068)

	first, ok := player.Progress(state)
	if !ok || first.Mode != RouteProgressMovement || first.PointIndex != 1 ||
		first.PreviousConfirmed != (world.Position{X: 14858, Y: 5068}) ||
		first.MovementTarget != (world.Position{X: 14820, Y: 5065}) {
		t.Fatalf("first progress = %+v, %t", first, ok)
	}
	second, ok := player.Progress(state)
	if !ok || second != first || len(nav.goals) != 0 || nav.resets != 0 {
		t.Fatalf("repeated progress = %+v, %t goals=%d resets=%d", second, ok, len(nav.goals), nav.resets)
	}

	if _, err := player.Tick(context.Background(), state); err != nil {
		t.Fatal(err)
	}
	nav.active = false
	state.Player.Position = world.Position{X: 14900, Y: 5100}
	recovery, ok := player.Progress(state)
	if !ok || recovery.Mode != RouteProgressRecovery ||
		recovery.MovementTarget != recovery.PreviousConfirmed ||
		recovery.PointIndex != first.PointIndex {
		t.Fatalf("recovery progress = %+v, %t", recovery, ok)
	}
	if len(nav.goals) != 1 {
		t.Fatalf("Progress sent navigation input: goals=%d", len(nav.goals))
	}
}

func TestRouteSegmentPlayerProgressProjectsTransitionWithoutTickingIt(t *testing.T) {
	route := validRoute()
	nav := &segmentNavigatorMock{next: NavTickResult{Status: NavMoving}}
	player, _ := NewRouteSegmentPlayer(nav, route, 0)
	state := segmentPlaybackState(world.BlackMarsh, 14858, 5068)
	_, _ = player.Tick(context.Background(), state)
	state.Player.Position = world.Position{X: 14820, Y: 5065}
	nav.active = false
	_, _ = player.Tick(context.Background(), state)

	beforeGoals, beforeResets := len(nav.goals), nav.resets
	progress, ok := player.Progress(state)
	if !ok || progress.Mode != RouteProgressTransition || progress.TargetAvailable ||
		progress.PointIndex != len(route.Segments[0].Points) {
		t.Fatalf("transition progress = %+v, %t", progress, ok)
	}
	if len(nav.goals) != beforeGoals || nav.resets != beforeResets {
		t.Fatalf("transition Progress mutated navigator: goals=%d resets=%d", len(nav.goals), nav.resets)
	}
}

func TestRouteSegmentPlayerProjectsConfirmedRecoveryInput(t *testing.T) {
	route := validRoute()
	base := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	nav := &segmentNavigatorMock{next: NavTickResult{Status: NavMoving}}
	player, _ := NewRouteSegmentPlayer(nav, route, 0)
	state := segmentPlaybackState(world.BlackMarsh, 14858, 5068)
	state.At = base
	_, _ = player.Tick(context.Background(), state)

	nav.active = false
	state.At = base.Add(time.Second)
	state.Player.Position = world.Position{X: 14900, Y: 5100}
	nav.next = NavTickResult{
		Status: NavMoving, MovementInputSent: true,
		NextMovementInputAt: state.At.Add(250 * time.Millisecond), MovementProgressTiles: 3,
	}
	if _, err := player.Tick(context.Background(), state); err != nil {
		t.Fatal(err)
	}
	progress, ok := player.Progress(state)
	if !ok || progress.Mode != RouteProgressRecovery || !progress.RecoveryInputSent ||
		progress.RecoveryInputAt != state.At ||
		progress.RecoveryInputOrigin != state.Player.Position ||
		progress.RecoveryNextInputAt != state.At.Add(250*time.Millisecond) ||
		progress.RecoveryProgressTiles != 3 {
		t.Fatalf("recovery input progress = %+v, %t", progress, ok)
	}
}

func TestRouteSegmentPlayerReconcilesAuthorizedForwardMovement(t *testing.T) {
	route := validRoute()
	route.Segments[0].Points = []RoutePoint{
		{X: 100, Y: 100},
		{X: 120, Y: 100},
		{X: 140, Y: 100},
		{X: 160, Y: 100},
	}
	nav := &segmentNavigatorMock{next: NavTickResult{Status: NavMoving}}
	player, err := NewRouteSegmentPlayer(nav, route, 0)
	if err != nil {
		t.Fatal(err)
	}
	state := segmentPlaybackState(world.BlackMarsh, 100, 100)
	if syncErr := player.SyncReached(state); syncErr != nil {
		t.Fatal(syncErr)
	}

	state.Player.Position = world.Position{X: 155, Y: 103}
	reconciled, err := player.ReconcileForward(state)
	if err != nil || !reconciled {
		t.Fatalf("reconciled=%t err=%v", reconciled, err)
	}
	progress, ok := player.Progress(state)
	if !ok || progress.Mode != RouteProgressMovement || progress.PointIndex != 3 ||
		progress.PreviousConfirmed != (world.Position{X: 140, Y: 100}) ||
		progress.MovementTarget != (world.Position{X: 160, Y: 100}) ||
		progress.LocalRecoveryAttempts != 0 {
		t.Fatalf("reconciled progress = %+v, %t", progress, ok)
	}
}
