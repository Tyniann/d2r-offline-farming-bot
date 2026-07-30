package pathing

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

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

// RouteProgressMode identifies the source of the effective next route target.
type RouteProgressMode string

const (
	// RouteProgressMovement identifies the next regular recorded point.
	RouteProgressMovement RouteProgressMode = "movement"
	// RouteProgressRecovery identifies the previous point used for local recovery.
	RouteProgressRecovery RouteProgressMode = "recovery"
	// RouteProgressTransition identifies an area transition without a positional target.
	RouteProgressTransition RouteProgressMode = "transition"
)

// RouteProgress is an immutable projection of the effective next route action.
// Progress inspection never starts or ticks navigation.
type RouteProgress struct {
	RouteID               string
	SegmentID             string
	SegmentIndex          int
	PointIndex            int
	PreviousConfirmed     world.Position
	MovementTarget        world.Position
	TargetAvailable       bool
	Mode                  RouteProgressMode
	DriftTiles            float64
	LocalRecoveryAttempts int
	RecoveryInputSent     bool
	RecoveryInputAt       time.Time
	RecoveryInputOrigin   world.Position
	RecoveryNextInputAt   time.Time
	RecoveryOutcomeAt     time.Time
	RecoveryProgressTiles float64
}

// RouteSegmentPlayer replays exactly one validated route segment.
// States: points → transition → complete; any invalid state terminates fail-closed.
type RouteSegmentPlayer struct {
	navigator         SegmentNavigator
	route             Route
	segment           RouteSegment
	segmentIndex      int
	point             int
	previous          world.Position
	started           bool
	transition        bool
	done              bool
	corrections       int
	recovering        bool
	recoveryInput     routeRecoveryInput
	transitionHandler *RouteTransitionHandler
}

type routeRecoveryInput struct {
	point         int
	target        world.Position
	origin        world.Position
	at            time.Time
	nextInputAt   time.Time
	outcomeAt     time.Time
	progressTiles float64
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
	return &RouteSegmentPlayer{navigator: navigator, route: route, segment: route.Segments[segmentIndex], segmentIndex: segmentIndex}, nil
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
	p.syncReachedPoints(state)
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
			p.recoveryInput = routeRecoveryInput{}
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

// Progress returns the effective next movement, recovery, or transition target
// for state without mutating player or navigator state.
func (p *RouteSegmentPlayer) Progress(state world.State) (RouteProgress, bool) {
	if p.done || !state.Valid || state.Phase != world.GamePhaseInGame {
		return RouteProgress{}, false
	}
	expectedClass, _ := parseCharacterClass(p.route.Binding.CharacterClass)
	if !state.Identity.Valid || state.Identity.CharacterName != p.route.Binding.CharacterName || state.Identity.Class != expectedClass {
		return RouteProgress{}, false
	}
	if p.transition {
		if state.Area.ID != p.segment.FromAreaID && state.Area.ID != p.segment.ToAreaID {
			return RouteProgress{}, false
		}
		return p.projectProgress(p.point, p.previous, RouteProgressTransition, world.Position{}, false, 0), true
	}
	if state.Area.ID != p.segment.FromAreaID {
		return RouteProgress{}, false
	}

	point := p.point
	previous := p.previous
	if !p.started {
		previous = routePointPosition(p.segment.Points[0])
	}
	for point < len(p.segment.Points) && world.Distance(state.Player.Position, routePointPosition(p.segment.Points[point])) <= p.route.Playback.WaypointToleranceTiles {
		previous = routePointPosition(p.segment.Points[point])
		point++
	}
	if point >= len(p.segment.Points) {
		return p.projectProgress(point, previous, RouteProgressTransition, world.Position{}, false, 0), true
	}

	target := routePointPosition(p.segment.Points[point])
	drift := distanceToEdge(state.Player.Position, previous, target)
	if (p.recovering && world.Distance(state.Player.Position, previous) > p.route.Playback.WaypointToleranceTiles) ||
		drift > p.route.Playback.MaxDriftTiles {
		return p.projectProgress(point, previous, RouteProgressRecovery, previous, true, drift), true
	}
	return p.projectProgress(point, previous, RouteProgressMovement, target, true, drift), true
}

// SyncReached commits only recorded points already confirmed inside the
// configured waypoint tolerance. It sends no movement input and does not enter
// a terminal or transition state; the next [RouteSegmentPlayer.Tick] owns that
// state change.
func (p *RouteSegmentPlayer) SyncReached(state world.State) error {
	if p.done {
		return nil
	}
	if !state.Valid || state.Phase != world.GamePhaseInGame {
		return fmt.Errorf("route point sync requires valid in-game state")
	}
	expectedClass, _ := parseCharacterClass(p.route.Binding.CharacterClass)
	if !state.Identity.Valid || state.Identity.CharacterName != p.route.Binding.CharacterName || state.Identity.Class != expectedClass {
		return ErrGameIdentityUnavailable
	}
	if p.transition {
		return nil
	}
	if state.Area.ID != p.segment.FromAreaID {
		return fmt.Errorf("%w: got %d want %d", ErrRouteUnexpectedArea, state.Area.ID, p.segment.FromAreaID)
	}
	p.syncReachedPoints(state)
	return nil
}

func (p *RouteSegmentPlayer) syncReachedPoints(state world.State) {
	if !p.started {
		p.previous = routePointPosition(p.segment.Points[0])
		p.started = true
	}
	advanced := false
	for p.point < len(p.segment.Points) && world.Distance(state.Player.Position, routePointPosition(p.segment.Points[p.point])) <= p.route.Playback.WaypointToleranceTiles {
		p.previous = routePointPosition(p.segment.Points[p.point])
		p.point++
		p.corrections = 0
		p.recovering = false
		p.recoveryInput = routeRecoveryInput{}
		advanced = true
	}
	if advanced {
		p.navigator.Reset()
	}
}

func (p *RouteSegmentPlayer) projectProgress(point int, previous world.Position, mode RouteProgressMode, target world.Position, available bool, drift float64) RouteProgress {
	progress := RouteProgress{
		RouteID:               p.route.ID,
		SegmentID:             p.segment.ID,
		SegmentIndex:          p.segmentIndex,
		PointIndex:            point,
		PreviousConfirmed:     previous,
		MovementTarget:        target,
		TargetAvailable:       available,
		Mode:                  mode,
		DriftTiles:            drift,
		LocalRecoveryAttempts: p.corrections,
	}
	if mode == RouteProgressRecovery &&
		p.recoveryInput.at != (time.Time{}) &&
		p.recoveryInput.point == point &&
		p.recoveryInput.target == target {
		progress.RecoveryInputSent = true
		progress.RecoveryInputAt = p.recoveryInput.at
		progress.RecoveryInputOrigin = p.recoveryInput.origin
		progress.RecoveryNextInputAt = p.recoveryInput.nextInputAt
		progress.RecoveryOutcomeAt = p.recoveryInput.outcomeAt
		progress.RecoveryProgressTiles = p.recoveryInput.progressTiles
	}
	return progress
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
	if p.recovering && result.MovementInputSent {
		p.recoveryInput = routeRecoveryInput{
			point: p.point, target: p.previous, origin: state.Player.Position, at: state.At,
			nextInputAt: result.NextMovementInputAt, outcomeAt: result.MovementOutcomeAt,
			progressTiles: result.MovementProgressTiles,
		}
	}
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
	if kind, ok := world.ParseEntranceKind(value); ok {
		return kind
	}
	return world.EntranceKindUnknown
}
