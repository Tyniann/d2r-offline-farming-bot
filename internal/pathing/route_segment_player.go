package pathing

import (
	"context"
	"errors"
	"fmt"
	"math"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

var (
	// ErrRouteUnexpectedArea indicates that playback left its segment area unexpectedly.
	ErrRouteUnexpectedArea = errors.New("route unexpected area")
	// ErrRouteDriftExceeded indicates that the player moved too far from the active path edge.
	ErrRouteDriftExceeded = errors.New("route drift exceeded")
	// ErrRouteSegmentFailed indicates that the delegated navigator failed.
	ErrRouteSegmentFailed = errors.New("route segment failed")
	// ErrRouteHardStuck indicates exhausted route-local recovery after a
	// navigator reported no progress.
	ErrRouteHardStuck = errors.New("route hard stuck")
	// ErrRouteSegmentTimeout indicates that a route segment exceeded its finite budget.
	ErrRouteSegmentTimeout = errors.New("route segment timeout")
)

// SegmentNavigator is the movement surface required by [RouteSegmentPlayer].
type SegmentNavigator interface {
	Start(Goal) error
	Tick(context.Context, world.State) NavTickResult
	Active() bool
	Reset()
}

// RouteSegmentPlayer replays exactly one validated route segment.
// States: points → transition → complete; any invalid state terminates fail-closed.
type RouteSegmentPlayer struct {
	navigator         SegmentNavigator
	route             Route
	segment           RouteSegment
	point             int
	previous          world.Position
	started           bool
	transition        bool
	done              bool
	corrections       int
	recovering        bool
	transitionHandler *RouteTransitionHandler
}

// NewRouteSegmentPlayer builds an isolated player for one segment index.
func NewRouteSegmentPlayer(navigator SegmentNavigator, route Route, segmentIndex int) (*RouteSegmentPlayer, error) {
	if navigator == nil {
		return nil, fmt.Errorf("segment player navigator required")
	}
	if err := route.Validate(); err != nil {
		return nil, fmt.Errorf("segment player route: %w", err)
	}
	if segmentIndex < 0 || segmentIndex >= len(route.Segments) {
		return nil, fmt.Errorf("segment index %d out of range", segmentIndex)
	}
	return &RouteSegmentPlayer{navigator: navigator, route: route, segment: route.Segments[segmentIndex]}, nil
}

// Tick advances point movement or the strict entrance transition.
func (p *RouteSegmentPlayer) Tick(ctx context.Context, state world.State) (bool, error) {
	if p.done {
		return true, nil
	}
	if ctx.Err() != nil {
		p.navigator.Reset()
		return false, ctx.Err()
	}
	if !state.Valid || state.Phase != world.GamePhaseInGame {
		return false, nil
	}
	expectedClass, _ := parseCharacterClass(p.route.Binding.CharacterClass)
	if !state.Identity.Valid || state.Identity.CharacterName != p.route.Binding.CharacterName || state.Identity.Class != expectedClass {
		p.navigator.Reset()
		return false, ErrGameIdentityUnavailable
	}
	if p.transition {
		done, err := p.transitionHandler.Tick(ctx, state)
		if done {
			p.done = true
		}
		return done, err
	}
	if state.Area.ID != p.segment.FromAreaID {
		p.navigator.Reset()
		return false, fmt.Errorf("%w: got %d want %d", ErrRouteUnexpectedArea, state.Area.ID, p.segment.FromAreaID)
	}
	if !p.started {
		p.previous = routePointPosition(p.segment.Points[0])
		p.started = true
	}
	for p.point < len(p.segment.Points) && world.Distance(state.Player.Position, routePointPosition(p.segment.Points[p.point])) <= p.route.Playback.WaypointToleranceTiles {
		p.previous = routePointPosition(p.segment.Points[p.point])
		p.point++
		p.corrections = 0
		p.navigator.Reset()
	}
	if p.point >= len(p.segment.Points) {
		if p.segment.Transition.Type == "terminal" {
			p.done = true
			return true, nil
		}
		p.transition = true
		p.transitionHandler = NewRouteTransitionHandler(p.navigator, p.segment, p.route.Playback.MaxLocalCorrections)
		return false, nil
	}
	target := routePointPosition(p.segment.Points[p.point])
	if p.recovering {
		if world.Distance(state.Player.Position, p.previous) <= p.route.Playback.WaypointToleranceTiles {
			p.recovering = false
			p.navigator.Reset()
		} else {
			if !p.navigator.Active() {
				if err := p.navigator.Start(Goal{Kind: GoalKindMoveToPosition, TargetPos: p.previous, ArrivalDistance: p.route.Playback.WaypointToleranceTiles}); err != nil {
					return false, fmt.Errorf("start route recovery for point %d: %w", p.point, err)
				}
			}
			return p.tickNavigator(ctx, state)
		}
	}
	if distanceToEdge(state.Player.Position, p.previous, target) > p.route.Playback.MaxDriftTiles {
		p.navigator.Reset()
		if p.corrections >= p.route.Playback.MaxLocalCorrections {
			return false, fmt.Errorf("%w at point %d after %d corrections", ErrRouteDriftExceeded, p.point, p.corrections)
		}
		p.corrections++
		p.recovering = true
		if err := p.navigator.Start(Goal{Kind: GoalKindMoveToPosition, TargetPos: p.previous, ArrivalDistance: p.route.Playback.WaypointToleranceTiles}); err != nil {
			return false, fmt.Errorf("start route recovery for point %d: %w", p.point, err)
		}
		return p.tickNavigator(ctx, state)
	}
	if !p.navigator.Active() {
		if err := p.navigator.Start(Goal{Kind: GoalKindMoveToPosition, TargetPos: target, ArrivalDistance: p.route.Playback.WaypointToleranceTiles}); err != nil {
			return false, fmt.Errorf("start route point %d: %w", p.point, err)
		}
	}
	return p.tickNavigator(ctx, state)
}

// Segment returns the immutable segment selected for playback.
func (p *RouteSegmentPlayer) Segment() RouteSegment { return p.segment }

// Transitioning reports whether all recorded points are complete and the area change is pending.
func (p *RouteSegmentPlayer) Transitioning() bool { return p.transition }

// PointIndex returns the next point index, or len(points) during transition.
func (p *RouteSegmentPlayer) PointIndex() int { return p.point }

// CurrentTarget returns the next recorded point while point playback is active.
func (p *RouteSegmentPlayer) CurrentTarget() (RoutePoint, bool) {
	if p.transition || p.point >= len(p.segment.Points) {
		return RoutePoint{}, false
	}
	return p.segment.Points[p.point], true
}

// LastConfirmedPointIndex returns the most recent confirmed point index, or -1
// while playback is still approaching the first point.
func (p *RouteSegmentPlayer) LastConfirmedPointIndex() int { return p.point - 1 }

// LocalRecoveryAttempts returns corrections consumed in the active segment.
func (p *RouteSegmentPlayer) LocalRecoveryAttempts() int { return p.corrections }

// DriftTiles returns the current distance from the active recorded path edge.
func (p *RouteSegmentPlayer) DriftTiles(position world.Position) float64 {
	if p.transition || p.point >= len(p.segment.Points) {
		return 0
	}
	return distanceToEdge(position, p.previous, routePointPosition(p.segment.Points[p.point]))
}

func (p *RouteSegmentPlayer) tickNavigator(ctx context.Context, state world.State) (bool, error) {
	result := p.navigator.Tick(ctx, state)
	if result.Done && result.Status != NavArrived {
		if p.corrections < p.route.Playback.MaxLocalCorrections {
			p.corrections++
			p.navigator.Reset()
			return false, nil
		}
		if result.Reason == ReasonStuck {
			return false, fmt.Errorf("%w: %w: status=%s", ErrRouteSegmentFailed, ErrRouteHardStuck, result.Status)
		}
		return false, fmt.Errorf("%w: status=%s reason=%s", ErrRouteSegmentFailed, result.Status, result.Reason)
	}
	if result.Done && result.Status == NavArrived {
		p.corrections = 0
	}
	return false, nil
}

func distanceToEdge(point, start, end world.Position) float64 {
	dx, dy := float64(end.X)-float64(start.X), float64(end.Y)-float64(start.Y)
	if dx == 0 && dy == 0 {
		return world.Distance(point, start)
	}
	t := ((float64(point.X)-float64(start.X))*dx + (float64(point.Y)-float64(start.Y))*dy) / (dx*dx + dy*dy)
	t = math.Max(0, math.Min(1, t))
	return math.Hypot(float64(point.X)-(float64(start.X)+t*dx), float64(point.Y)-(float64(start.Y)+t*dy))
}

func parseEntranceKind(value string) world.EntranceKind {
	for kind := world.EntranceKindUnknown; kind <= world.EntranceKindCatacombsDown; kind++ {
		if kind.String() == value {
			return kind
		}
	}
	return world.EntranceKindUnknown
}
