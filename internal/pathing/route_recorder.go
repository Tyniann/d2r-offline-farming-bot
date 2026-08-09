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

// Finish returns completed segments and preserves the current-area tail as a
// terminal segment when it contains an actual movement path.
func (r *RouteRecorder) Finish() ([]RouteSegment, error) {
	if !r.started {
		return nil, ErrRouteRecordingNotStarted
	}
	if len(r.points) >= 2 {
		last := r.previous.Player.Position
		if routePointPosition(r.points[len(r.points)-1]) != last {
			r.points = append(r.points, RoutePoint{X: last.X, Y: last.Y})
		}
		r.segments = append(r.segments, RouteSegment{ID: r.currentID, FromAreaID: r.currentArea, ToAreaID: r.currentArea, Movement: r.cfg.Movement, Points: append([]RoutePoint(nil), r.points...), Transition: RouteTransition{Type: "terminal"}})
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
	transition := recordingTransition(r.currentArea, toArea, r.previous)
	return RouteSegment{ID: r.currentID, FromAreaID: r.currentArea, ToAreaID: toArea, Movement: r.cfg.Movement, Points: append([]RoutePoint(nil), r.points...), Transition: transition}
}

func recordingTransition(fromArea, toArea world.AreaID, state world.State) RouteTransition {
	if portal, ok := nearestRecordingObjectOfKind(state, world.ObjectKindPermanentPortal); ok && portal.UnitID != 0 {
		return RouteTransition{Type: "object_portal", ObjectKind: portal.Kind, ExpectedToArea: toArea}
	}
	return RouteTransition{Type: "entrance", EntranceKind: recordingTransitionKind(fromArea, toArea, state)}
}

func nearestRecordingObjectOfKind(state world.State, kind world.ObjectKind) (world.Object, bool) {
	var best world.Object
	bestDistance := 0.0
	found := false
	for _, object := range state.Objects {
		if object.Kind != kind {
			continue
		}
		distance := world.Distance(state.Player.Position, object.Position)
		if !found || distance < bestDistance {
			best, bestDistance, found = object, distance, true
		}
	}
	return best, found
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
	return nearestRecordingEntranceOfKind(state, world.EntranceKindUnknown)
}

func nearestRecordingEntranceOfKind(state world.State, expected world.EntranceKind) (world.Entrance, bool) {
	var best world.Entrance
	bestDistance := 0.0
	found := false
	for _, entrance := range state.Entrances {
		if expected != world.EntranceKindUnknown && entrance.Kind != expected {
			continue
		}
		distance := world.Distance(state.Player.Position, entrance.Position)
		if !found || distance < bestDistance {
			best, bestDistance, found = entrance, distance, true
		}
	}
	return best, found
}

func recordingTransitionKind(fromArea, toArea world.AreaID, state world.State) string {
	if expected, constrained := expectedHallsTransitionKind(fromArea, toArea); constrained {
		// Halls levels expose both up/down entrances. Never substitute the
		// nearest opposite direction because playback matches this kind strictly.
		if entrance, ok := nearestRecordingEntranceOfKind(state, expected); ok {
			return entrance.Kind.String()
		}
		return world.EntranceKindUnknown.String()
	}
	if entrance, ok := nearestRecordingEntrance(state); ok {
		return entrance.Kind.String()
	}
	return world.EntranceKindUnknown.String()
}

func expectedHallsTransitionKind(fromArea, toArea world.AreaID) (world.EntranceKind, bool) {
	switch {
	case fromArea == world.NihlathaksTemple && toArea == world.HallsOfAnguish:
		return world.EntranceKindHallsEntrance, true
	case fromArea == world.HallsOfAnguish && toArea == world.NihlathaksTemple:
		return world.EntranceKindHallsUp, true
	case fromArea == world.HallsOfAnguish && toArea == world.HallsOfPain:
		return world.EntranceKindHallsDown, true
	case fromArea == world.HallsOfPain && toArea == world.HallsOfAnguish:
		return world.EntranceKindHallsUp, true
	case fromArea == world.HallsOfPain && toArea == world.HallsOfVaught:
		return world.EntranceKindHallsDown, true
	case fromArea == world.HallsOfVaught && toArea == world.HallsOfPain:
		return world.EntranceKindHallsUp, true
	default:
		return world.EntranceKindUnknown, false
	}
}

func routePointPosition(point RoutePoint) world.Position {
	return world.Position{X: point.X, Y: point.Y}
}
