package app

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/pathing"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/tasks"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

type holdNavigator struct {
	active bool
	starts []pathing.Goal
	ticks  int
	resets int
	next   pathing.NavTickResult
}

func (n *holdNavigator) Start(goal pathing.Goal) error {
	n.active = true
	n.starts = append(n.starts, goal)
	return nil
}

func (n *holdNavigator) Tick(context.Context, world.State) pathing.NavTickResult {
	n.ticks++
	if n.next.Status != "" || n.next.MovementInputSent || n.next.Done {
		return n.next
	}
	return pathing.NavTickResult{Status: pathing.NavMoving}
}

func (n *holdNavigator) Active() bool { return n.active }
func (n *holdNavigator) Reset()       { n.active = false; n.resets++ }

func phase17AdapterRoute() pathing.Route {
	seed := uint32(17)
	return pathing.Route{
		Version: pathing.RouteVersion,
		ID:      "phase-17-adapter-route",
		Name:    "Phase 17 Adapter Route",
		Kind:    pathing.RouteKindNavigation,
		Binding: pathing.RouteBinding{
			CharacterName:  "MrBones",
			CharacterClass: "necromancer",
			Difficulty:     pathing.RouteDifficultyHell,
			MapSeed:        &seed,
			GameVersion:    "3.2.92777",
			LayoutFingerprint: pathing.RouteLayoutFingerprint{
				Version: 1, AreaID: world.ArcaneSanctuary, AnchorCount: 1, Hash: strings.Repeat("a", 64),
			},
		},
		Recording: pathing.RouteRecording{RecordedAt: time.Date(2026, 7, 29, 0, 0, 0, 0, time.UTC), SampleDistanceTiles: 4},
		Playback:  pathing.RoutePlayback{WaypointToleranceTiles: 3, MaxDriftTiles: 8, MaxLocalCorrections: 2, SegmentTimeoutMs: 30000, TransitionTimeoutMs: 10000},
		Segments: []pathing.RouteSegment{{
			ID:         "arcane-terminal",
			FromAreaID: world.ArcaneSanctuary,
			ToAreaID:   world.ArcaneSanctuary,
			Movement:   pathing.RouteMovementTeleport,
			Points:     []pathing.RoutePoint{{X: 100, Y: 100}, {X: 120, Y: 100}},
			Transition: pathing.RouteTransition{Type: "terminal"},
		}},
	}
}

func phase17AdapterState(at time.Time) world.State {
	return world.State{
		At:       at,
		Valid:    true,
		Phase:    world.GamePhaseInGame,
		Area:     world.LookupArea(world.ArcaneSanctuary),
		Player:   world.Player{Position: world.Position{X: 100, Y: 100}},
		Identity: world.GameIdentity{Valid: true, CharacterName: "MrBones", Class: world.CharacterClassNecromancer, MapSeed: 17},
	}
}

func TestRoutePlaybackAdapterHoldCreditsTenSecondsExactlyOnce(t *testing.T) {
	route := phase17AdapterRoute()
	nav := &holdNavigator{}
	player, playerErr := pathing.NewRoutePlayer(nav, route)
	if playerErr != nil {
		t.Fatal(playerErr)
	}
	base := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	now := base
	state := phase17AdapterState(base)
	adapter := &routePlaybackAdapter{
		log: slog.Default(), route: route, player: player, navigator: nav,
		deadline: base.Add(30 * time.Second), lastTickAt: state.At, lastCallAt: base,
		identity: state.Identity, clock: func() time.Time { return now },
	}
	before, ok := adapter.Progress(state)
	if !ok || before.Mode != tasks.RouteProgressMovement || before.PointIndex != 1 {
		t.Fatalf("before progress = %+v, %t", before, ok)
	}

	now = base.Add(5 * time.Second)
	if err := adapter.Hold(state); err != nil {
		t.Fatal(err)
	}
	if adapter.deadline != base.Add(30*time.Second) {
		t.Fatalf("identical snapshot changed deadline to %s", adapter.deadline)
	}
	now = base.Add(10 * time.Second)
	state.At = base.Add(10 * time.Second)
	if err := adapter.Hold(state); err != nil {
		t.Fatal(err)
	}
	if adapter.deadline != base.Add(40*time.Second) {
		t.Fatalf("deadline = %s, want %s", adapter.deadline, base.Add(40*time.Second))
	}
	if len(nav.starts) != 0 || nav.ticks != 0 {
		t.Fatalf("Hold mutated navigator: starts=%d ticks=%d resets=%d", len(nav.starts), nav.ticks, nav.resets)
	}
	// SyncReached may reset the navigator when Memory confirms a route point during hold.
	after, ok := adapter.Progress(state)
	if !ok || after != before {
		t.Fatalf("after progress = %+v, %t; before = %+v", after, ok, before)
	}

	now = base.Add(39 * time.Second)
	state.At = base.Add(11 * time.Second)
	done, err := adapter.Tick(context.Background(), state)
	if err != nil || done {
		t.Fatalf("resume done=%t err=%v", done, err)
	}
	if len(nav.starts) != 1 || nav.ticks != 1 || nav.starts[0].TargetPos != before.MovementTarget {
		t.Fatalf("resume navigation = starts=%+v ticks=%d", nav.starts, nav.ticks)
	}

	adapter.Reset()
	if _, ok := adapter.Progress(state); ok {
		t.Fatal("Progress remained available after adapter Reset")
	}
}

func TestRoutePlaybackAdapterHoldRejectsInvalidStateWithoutDeadlineCredit(t *testing.T) {
	route := phase17AdapterRoute()
	nav := &holdNavigator{}
	player, _ := pathing.NewRoutePlayer(nav, route)
	base := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	now := base.Add(time.Second)
	state := phase17AdapterState(base)
	adapter := &routePlaybackAdapter{
		log: slog.Default(), route: route, player: player, navigator: nav,
		deadline: base.Add(30 * time.Second), lastTickAt: state.At, lastCallAt: base,
		identity: state.Identity, clock: func() time.Time { return now },
	}
	originalDeadline := adapter.deadline

	stale := state
	stale.At = base.Add(-time.Millisecond)
	if err := adapter.Hold(stale); err == nil {
		t.Fatal("stale snapshot was accepted")
	}
	wrongArea := state
	wrongArea.At = base.Add(time.Millisecond)
	wrongArea.Area = world.LookupArea(world.PalaceCellarLevel3)
	if err := adapter.Hold(wrongArea); !errors.Is(err, pathing.ErrRouteUnexpectedArea) {
		t.Fatalf("area error = %v", err)
	}
	wrongIdentity := state
	wrongIdentity.At = base.Add(time.Millisecond)
	wrongIdentity.Identity.CharacterName = "Other"
	if err := adapter.Hold(wrongIdentity); err == nil {
		t.Fatal("identity change was accepted")
	}
	if adapter.deadline != originalDeadline || len(nav.starts) != 0 || nav.ticks != 0 {
		t.Fatalf("invalid Hold changed state: deadline=%s starts=%d ticks=%d", adapter.deadline, len(nav.starts), nav.ticks)
	}
}

func TestRoutePlaybackAdapterConfirmedPointProgressRefreshesTimeout(t *testing.T) {
	route := phase17AdapterRoute()
	route.Segments[0].Points = append(route.Segments[0].Points, pathing.RoutePoint{X: 140, Y: 100})
	nav := &holdNavigator{}
	player, err := pathing.NewRoutePlayer(nav, route)
	if err != nil {
		t.Fatal(err)
	}
	base := time.Date(2026, 8, 9, 4, 0, 0, 0, time.UTC)
	now := base
	state := phase17AdapterState(base)
	adapter := &routePlaybackAdapter{
		log: slog.Default(), route: route, player: player, navigator: nav,
		deadline: base.Add(30 * time.Second), lastTickAt: state.At, lastCallAt: base,
		identity: state.Identity, clock: func() time.Time { return now },
	}

	if done, tickErr := adapter.Tick(context.Background(), state); tickErr != nil || done {
		t.Fatalf("initial tick done=%t err=%v", done, tickErr)
	}
	now = base.Add(29 * time.Second)
	state.At, state.Player.Position = now, world.Position{X: 120, Y: 100}
	if done, tickErr := adapter.Tick(context.Background(), state); tickErr != nil || done {
		t.Fatalf("progress tick done=%t err=%v", done, tickErr)
	}
	if want := base.Add(59 * time.Second); adapter.deadline != want {
		t.Fatalf("progress deadline=%s, want %s", adapter.deadline, want)
	}

	now = base.Add(58 * time.Second)
	state.At, state.Player.Position = now, world.Position{X: 140, Y: 100}
	if done, tickErr := adapter.Tick(context.Background(), state); tickErr != nil || !done {
		t.Fatalf("terminal progress done=%t err=%v", done, tickErr)
	}
}

func TestRoutePlaybackAdapterWithoutPointProgressStillTimesOut(t *testing.T) {
	route := phase17AdapterRoute()
	nav := &holdNavigator{}
	player, err := pathing.NewRoutePlayer(nav, route)
	if err != nil {
		t.Fatal(err)
	}
	base := time.Date(2026, 8, 9, 4, 0, 0, 0, time.UTC)
	now := base
	state := phase17AdapterState(base)
	adapter := &routePlaybackAdapter{
		log: slog.Default(), route: route, player: player, navigator: nav,
		deadline: base.Add(30 * time.Second), lastTickAt: state.At, lastCallAt: base,
		identity: state.Identity, clock: func() time.Time { return now },
	}
	if _, tickErr := adapter.Tick(context.Background(), state); tickErr != nil {
		t.Fatal(tickErr)
	}

	now = base.Add(31 * time.Second)
	// Keep the Memory generation fresh without simulating a load-screen-sized
	// snapshot gap, which intentionally credits that gap to the deadline.
	state.At = base.Add(time.Second)
	if _, tickErr := adapter.Tick(context.Background(), state); !errors.Is(tickErr, pathing.ErrRouteSegmentTimeout) {
		t.Fatalf("timeout error=%v, want route segment timeout", tickErr)
	}
}

func TestRoutePlaybackAdapterMapsConfirmedRecoveryInput(t *testing.T) {
	route := phase17AdapterRoute()
	base := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	nav := &holdNavigator{}
	player, _ := pathing.NewRoutePlayer(nav, route)
	state := phase17AdapterState(base)
	adapter := &routePlaybackAdapter{
		log: slog.Default(), route: route, player: player, navigator: nav,
		deadline: base.Add(30 * time.Second), lastTickAt: state.At, lastCallAt: base,
		identity: state.Identity, clock: func() time.Time { return state.At },
	}
	if _, err := adapter.Tick(context.Background(), state); err != nil {
		t.Fatal(err)
	}

	nav.active = false
	state.At = base.Add(time.Second)
	state.Player.Position = world.Position{X: 140, Y: 120}
	nav.next = pathing.NavTickResult{
		Status: pathing.NavMoving, MovementInputSent: true,
		NextMovementInputAt: state.At.Add(250 * time.Millisecond), MovementProgressTiles: 3,
	}
	if _, err := adapter.Tick(context.Background(), state); err != nil {
		t.Fatal(err)
	}
	progress, ok := adapter.Progress(state)
	if !ok || progress.Mode != tasks.RouteProgressRecovery || !progress.RecoveryInputSent ||
		progress.RecoveryInputAt != state.At ||
		progress.RecoveryInputOrigin != state.Player.Position ||
		progress.RecoveryNextInputAt != state.At.Add(250*time.Millisecond) ||
		progress.RecoveryProgressTiles != 3 {
		t.Fatalf("mapped recovery progress = %+v, %t", progress, ok)
	}
}
