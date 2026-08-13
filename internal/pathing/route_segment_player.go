package pathing

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

const (
	blockedPointMaxInputs        = 2
	blockedPointMinProgressTiles = 1.0
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
	edgeAnchor        world.Position
	started           bool
	transition        bool
	done              bool
	corrections       int
	recovering        bool
	recoveryInput     routeRecoveryInput
	pointWatchdog     routePointWatchdog
	lastConfirmed     int
	lastSkipped       int
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

type routePointWatchdog struct {
	point              int
	target             world.Position
	bestDistance       float64
	inputsWithoutGain  int
	latestInputOutcome time.Time
	active             bool
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
	return &RouteSegmentPlayer{
		navigator: navigator, route: route, segment: route.Segments[segmentIndex], segmentIndex: segmentIndex,
		lastConfirmed: -1, lastSkipped: -1,
	}, nil
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
		if world.Distance(state.Player.Position, p.edgeAnchor) <= p.route.Playback.WaypointToleranceTiles {
			p.recovering = false
			p.recoveryInput = routeRecoveryInput{}
			p.navigator.Reset()
		} else {
			if !p.navigator.Active() {
				if err := p.navigator.Start(Goal{Kind: GoalKindMoveToPosition, TargetPos: p.edgeAnchor, ArrivalDistance: p.route.Playback.WaypointToleranceTiles}); err != nil {
					return false, fmt.Errorf("start route recovery for point %d: %w", p.point, err)
				}
			}
			return p.tickNavigator(ctx, state)
		}
	}
	if distanceToEdge(state.Player.Position, p.edgeAnchor, target) > p.route.Playback.MaxDriftTiles {
		p.resetPointWatchdog()
		p.navigator.Reset()
		if p.corrections >= p.route.Playback.MaxLocalCorrections {
			return false, fmt.Errorf("%w at point %d after %d corrections", ErrRouteDriftExceeded, p.point, p.corrections)
		}
		p.corrections++
		p.recovering = true
		if err := p.navigator.Start(Goal{Kind: GoalKindMoveToPosition, TargetPos: p.edgeAnchor, ArrivalDistance: p.route.Playback.WaypointToleranceTiles}); err != nil {
			return false, fmt.Errorf("start route recovery for point %d: %w", p.point, err)
		}
		return p.tickNavigator(ctx, state)
	}
	if p.tickPointWatchdog(state, target) {
		return false, nil
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
	edgeAnchor := p.edgeAnchor
	if !p.started {
		edgeAnchor = previous
	}
	drift := distanceToEdge(state.Player.Position, edgeAnchor, target)
	if (p.recovering && world.Distance(state.Player.Position, edgeAnchor) > p.route.Playback.WaypointToleranceTiles) ||
		drift > p.route.Playback.MaxDriftTiles {
		return p.projectProgress(point, previous, RouteProgressRecovery, edgeAnchor, true, drift), true
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

// ReconcileForward rebases playback onto the first later recorded edge that
// contains the player inside the configured drift corridor. It is reserved for
// caller-authorized external movement such as a combat or loot Hold; ordinary
// playback remains strictly sequential and never invokes this method.
func (p *RouteSegmentPlayer) ReconcileForward(state world.State) (bool, error) {
	if p.done {
		return false, nil
	}
	if !state.Valid || state.Phase != world.GamePhaseInGame {
		return false, fmt.Errorf("route forward reconciliation requires valid in-game state")
	}
	expectedClass, _ := parseCharacterClass(p.route.Binding.CharacterClass)
	if !state.Identity.Valid || state.Identity.CharacterName != p.route.Binding.CharacterName || state.Identity.Class != expectedClass {
		return false, ErrGameIdentityUnavailable
	}
	if p.transition {
		return false, nil
	}
	if state.Area.ID != p.segment.FromAreaID {
		return false, fmt.Errorf("%w: got %d want %d", ErrRouteUnexpectedArea, state.Area.ID, p.segment.FromAreaID)
	}

	p.syncReachedPoints(state)
	if p.point >= len(p.segment.Points) {
		return false, nil
	}
	currentTarget := routePointPosition(p.segment.Points[p.point])
	if distanceToEdge(state.Player.Position, p.edgeAnchor, currentTarget) <= p.route.Playback.MaxDriftTiles {
		return false, nil
	}

	// Select the first forward edge that contains the externally moved player.
	// Using the earliest match avoids skipping an entire loop when a later route
	// section crosses the same world coordinates.
	for point := p.point + 1; point < len(p.segment.Points); point++ {
		previous := routePointPosition(p.segment.Points[point-1])
		target := routePointPosition(p.segment.Points[point])
		if distanceToEdge(state.Player.Position, previous, target) > p.route.Playback.MaxDriftTiles {
			continue
		}
		p.previous = previous
		p.edgeAnchor = previous
		p.point = point
		p.corrections = 0
		p.recovering = false
		p.recoveryInput = routeRecoveryInput{}
		p.navigator.Reset()
		p.syncReachedPoints(state)
		return true, nil
	}
	return false, nil
}

func (p *RouteSegmentPlayer) syncReachedPoints(state world.State) {
	if !p.started {
		p.previous = routePointPosition(p.segment.Points[0])
		p.edgeAnchor = p.previous
		p.started = true
	}
	advanced := false
	for p.point < len(p.segment.Points) && world.Distance(state.Player.Position, routePointPosition(p.segment.Points[p.point])) <= p.route.Playback.WaypointToleranceTiles {
		p.previous = routePointPosition(p.segment.Points[p.point])
		p.edgeAnchor = p.previous
		p.lastConfirmed = p.point
		p.point++
		p.corrections = 0
		p.recovering = false
		p.recoveryInput = routeRecoveryInput{}
		p.resetPointWatchdog()
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
func (p *RouteSegmentPlayer) LastConfirmedPointIndex() int { return p.lastConfirmed }

// LastSkippedPoint returns the most recent blocked point accepted by the
// point-progress watchdog. A skipped point is never reported as Memory-confirmed.
func (p *RouteSegmentPlayer) LastSkippedPoint() (RoutePoint, int, bool) {
	if p.lastSkipped < 0 || p.lastSkipped >= len(p.segment.Points) {
		return RoutePoint{}, -1, false
	}
	return p.segment.Points[p.lastSkipped], p.lastSkipped, true
}

// LocalRecoveryAttempts returns corrections consumed in the active segment.
func (p *RouteSegmentPlayer) LocalRecoveryAttempts() int { return p.corrections }

// DriftTiles returns the current distance from the active recorded path edge.
func (p *RouteSegmentPlayer) DriftTiles(position world.Position) float64 {
	if p.transition || p.point >= len(p.segment.Points) {
		return 0
	}
	return distanceToEdge(position, p.edgeAnchor, routePointPosition(p.segment.Points[p.point]))
}

func (p *RouteSegmentPlayer) tickNavigator(ctx context.Context, state world.State) (bool, error) {
	result := p.navigator.Tick(ctx, state)
	if !p.recovering && result.MovementInputSent {
		p.notePointMovementInput(result)
	}
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

// tickPointWatchdog recognizes repeated movement that makes no meaningful
// progress toward one concrete point. It waits for the latest cast outcome and
// only accepts a blocked point while the player remains inside the existing
// route corridor. The final point of a terminal segment remains authoritative.
func (p *RouteSegmentPlayer) tickPointWatchdog(state world.State, target world.Position) bool {
	distance := world.Distance(state.Player.Position, target)
	if !p.pointWatchdog.active || p.pointWatchdog.point != p.point || p.pointWatchdog.target != target {
		p.pointWatchdog = routePointWatchdog{point: p.point, target: target, bestDistance: distance, active: true}
		return false
	}
	if distance <= p.pointWatchdog.bestDistance-blockedPointMinProgressTiles {
		p.pointWatchdog.bestDistance = distance
		p.pointWatchdog.inputsWithoutGain = 0
		p.pointWatchdog.latestInputOutcome = time.Time{}
		return false
	}
	if p.pointWatchdog.inputsWithoutGain < blockedPointMaxInputs {
		return false
	}
	now := state.At
	if now.IsZero() {
		now = time.Now()
	}
	if !p.pointWatchdog.latestInputOutcome.IsZero() && now.Before(p.pointWatchdog.latestInputOutcome) {
		return true
	}
	if distance > p.route.Playback.MaxDriftTiles ||
		distanceToEdge(state.Player.Position, p.edgeAnchor, target) > p.route.Playback.MaxDriftTiles ||
		(p.point == len(p.segment.Points)-1 && p.segment.Transition.Type == "terminal") {
		p.resetPointWatchdog()
		return false
	}

	p.navigator.Reset()
	// The live position becomes the next edge's temporary anchor. Returning to
	// the unreachable recorded point during a later drift recovery would
	// recreate the loop this watchdog is intended to break.
	p.edgeAnchor = state.Player.Position
	p.lastSkipped = p.point
	p.point++
	p.corrections = 0
	p.recovering = false
	p.recoveryInput = routeRecoveryInput{}
	p.resetPointWatchdog()
	return true
}

func (p *RouteSegmentPlayer) notePointMovementInput(result NavTickResult) {
	if !p.pointWatchdog.active {
		return
	}
	p.pointWatchdog.inputsWithoutGain++
	if result.MovementOutcomeAt.After(p.pointWatchdog.latestInputOutcome) {
		p.pointWatchdog.latestInputOutcome = result.MovementOutcomeAt
	}
}

func (p *RouteSegmentPlayer) resetPointWatchdog() {
	p.pointWatchdog = routePointWatchdog{}
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
