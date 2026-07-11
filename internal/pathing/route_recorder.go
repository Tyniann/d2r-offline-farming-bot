package pathing

import (
	"errors"
	"fmt"
	"strings"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

var (
	// ErrRouteRecordingNotStarted indicates that no valid start snapshot was accepted.
	ErrRouteRecordingNotStarted = errors.New("route recording not started")
	// ErrRouteRecordingIdentityChanged blocks a recording after a character change.
	ErrRouteRecordingIdentityChanged = errors.New("route recording identity changed")
	// ErrRouteRecordingIncomplete indicates that no complete area transition was captured.
	ErrRouteRecordingIncomplete = errors.New("route recording incomplete")
)

// RouteRecorderConfig controls read-only world-coordinate sampling.
type RouteRecorderConfig struct {
	SampleDistanceTiles float64
	Movement            RouteMovement
}

// RouteRecorderEvent describes one accepted sample or completed transition.
type RouteRecorderEvent struct {
	SampleAccepted  bool
	SegmentComplete bool
	AreaID          world.AreaID
	Position        world.Position
	Segment         RouteSegment
}

// RouteRecorder observes immutable World States and builds completed area segments.
// It never owns or invokes an input controller.
type RouteRecorder struct {
	cfg          RouteRecorderConfig
	identity     world.GameIdentity
	started      bool
	currentArea  world.AreaID
	currentID    string
	points       []RoutePoint
	segments     []RouteSegment
	previous     world.State
	segmentNames map[world.AreaID]int
}

// NewRouteRecorder creates a recorder with strict positive sampling distance.
func NewRouteRecorder(cfg RouteRecorderConfig) (*RouteRecorder, error) {
	if !finitePositive(cfg.SampleDistanceTiles) || cfg.SampleDistanceTiles > 20 {
		return nil, fmt.Errorf("route recorder sample distance must be > 0 and <= 20")
	}
	if cfg.Movement != RouteMovementTeleport && cfg.Movement != RouteMovementWalk {
		return nil, fmt.Errorf("route recorder movement unsupported: %q", cfg.Movement)
	}
	return &RouteRecorder{cfg: cfg, segmentNames: make(map[world.AreaID]int)}, nil
}

// Observe accepts valid In-Game snapshots and ignores loading or inconsistent reads.
func (r *RouteRecorder) Observe(state world.State) (RouteRecorderEvent, error) {
	if !state.Valid || state.Phase != world.GamePhaseInGame {
		return RouteRecorderEvent{}, nil
	}
	if !state.Identity.Valid {
		if r.started {
			return RouteRecorderEvent{}, ErrGameIdentityUnavailable
		}
		return RouteRecorderEvent{}, nil
	}
	if !r.started {
		r.started = true
		r.identity = state.Identity
		r.beginArea(state.Area.ID, state.Player.Position)
		r.previous = state
		return RouteRecorderEvent{SampleAccepted: true, AreaID: state.Area.ID, Position: state.Player.Position}, nil
	}
	if state.Identity.CharacterName != r.identity.CharacterName || state.Identity.Class != r.identity.Class {
		return RouteRecorderEvent{}, ErrRouteRecordingIdentityChanged
	}
	if state.Area.ID != r.currentArea {
		segment := r.completeArea(state.Area.ID)
		r.segments = append(r.segments, segment)
		r.beginArea(state.Area.ID, state.Player.Position)
		r.previous = state
		return RouteRecorderEvent{SampleAccepted: true, SegmentComplete: true, AreaID: state.Area.ID, Position: state.Player.Position, Segment: segment}, nil
	}
	position := state.Player.Position
	accepted := false
	if len(r.points) == 0 || world.Distance(routePointPosition(r.points[len(r.points)-1]), position) >= r.cfg.SampleDistanceTiles {
		r.points = append(r.points, RoutePoint{X: position.X, Y: position.Y})
		accepted = true
	}
	r.previous = state
	return RouteRecorderEvent{SampleAccepted: accepted, AreaID: state.Area.ID, Position: position}, nil
}

// Finish returns completed segments and discards the unterminated current-area tail.
func (r *RouteRecorder) Finish() ([]RouteSegment, error) {
	if !r.started {
		return nil, ErrRouteRecordingNotStarted
	}
	if len(r.segments) == 0 {
		return nil, ErrRouteRecordingIncomplete
	}
	segments := make([]RouteSegment, len(r.segments))
	copy(segments, r.segments)
	for i := range segments {
		segments[i].Points = append([]RoutePoint(nil), segments[i].Points...)
	}
	return segments, nil
}

// Identity returns the character confirmed at recording start.
func (r *RouteRecorder) Identity() world.GameIdentity { return r.identity }

func (r *RouteRecorder) beginArea(areaID world.AreaID, position world.Position) {
	r.currentArea = areaID
	r.currentID = r.nextSegmentID(areaID)
	r.points = []RoutePoint{{X: position.X, Y: position.Y}}
}

func (r *RouteRecorder) completeArea(toArea world.AreaID) RouteSegment {
	lastPosition := r.previous.Player.Position
	if len(r.points) == 0 || routePointPosition(r.points[len(r.points)-1]) != lastPosition {
		r.points = append(r.points, RoutePoint{X: lastPosition.X, Y: lastPosition.Y})
	}
	transitionKind := "unknown"
	if entrance, ok := nearestRecordingEntrance(r.previous); ok {
		transitionKind = entrance.Kind.String()
	}
	return RouteSegment{ID: r.currentID, FromAreaID: r.currentArea, ToAreaID: toArea, Movement: r.cfg.Movement, Points: append([]RoutePoint(nil), r.points...), Transition: RouteTransition{Type: "entrance", EntranceKind: transitionKind}}
}

func (r *RouteRecorder) nextSegmentID(areaID world.AreaID) string {
	base := strings.ToLower(world.LookupArea(areaID).Name)
	var builder strings.Builder
	lastDash := false
	for _, c := range base {
		if c >= 'a' && c <= 'z' || c >= '0' && c <= '9' {
			builder.WriteRune(c)
			lastDash = false
		} else if !lastDash && builder.Len() > 0 {
			builder.WriteByte('-')
			lastDash = true
		}
	}
	id := strings.Trim(builder.String(), "-")
	if len(id) < 3 {
		id = fmt.Sprintf("area-%d", areaID)
	}
	r.segmentNames[areaID]++
	if r.segmentNames[areaID] > 1 {
		id = fmt.Sprintf("%s-%d", id, r.segmentNames[areaID])
	}
	return id
}

func nearestRecordingEntrance(state world.State) (world.Entrance, bool) {
	var best world.Entrance
	bestDistance := 0.0
	found := false
	for _, entrance := range state.Entrances {
		distance := world.Distance(state.Player.Position, entrance.Position)
		if !found || distance < bestDistance {
			best, bestDistance, found = entrance, distance, true
		}
	}
	return best, found
}

func routePointPosition(point RoutePoint) world.Position {
	return world.Position{X: point.X, Y: point.Y}
}
