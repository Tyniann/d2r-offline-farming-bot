package pathing

import (
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

// GoalKind selects the navigation objective type.
type GoalKind int

// GoalKind values. The zero value is GoalKindNone (no active goal).
const (
	GoalKindNone GoalKind = iota
	// GoalKindMoveToArea navigates until world.State.Area.ID equals TargetArea.
	GoalKindMoveToArea
	// GoalKindMoveToPosition navigates until the player is within arrival
	// distance of TargetPos inside the current area.
	GoalKindMoveToPosition
)

// String returns a stable label for structured logging.
func (k GoalKind) String() string {
	switch k {
	case GoalKindMoveToArea:
		return "move_to_area"
	case GoalKindMoveToPosition:
		return "move_to_position"
	default:
		return "none"
	}
}

// Goal describes a navigation objective for [Navigator.Start].
type Goal struct {
	Kind       GoalKind
	TargetArea world.AreaID   // Required for GoalKindMoveToArea.
	TargetPos  world.Position // Required for GoalKindMoveToPosition.
	// ArrivalDistance overrides the general navigator tolerance when positive.
	ArrivalDistance float64
	ViaEntrance     world.EntranceKind // Optional: entrance kind that leads to TargetArea.
	// ViaEntranceUnitID pins a strict transition to one runtime entrance unit.
	ViaEntranceUnitID uint32
	// StrictEntrance blocks bearing exploration when the expected entrance is unavailable.
	StrictEntrance bool
	ViaObject      world.ObjectKind // Optional object portal that leads to TargetArea.
	// ViaObjectUnitID pins a strict object transition to one runtime object unit.
	ViaObjectUnitID uint32
	// StrictObject blocks exploration and accepts only the pinned object.
	StrictObject bool
}

// NavStatus describes the current navigator state-machine phase.
type NavStatus string

// NavStatus values. Terminal states are arrived, stuck, and failed.
const (
	NavIdle      NavStatus = "idle"
	NavMoving    NavStatus = "moving"
	NavExploring NavStatus = "exploring"
	NavClicking  NavStatus = "clicking"
	NavArrived   NavStatus = "arrived"
	NavStuck     NavStatus = "stuck"
	NavFailed    NavStatus = "failed"
)

// NavResult reasons for terminal states.
const (
	ReasonStuck               = "stuck"
	ReasonHoverNotFound       = "hover_not_found"
	ReasonCancelled           = "cancelled"
	ReasonInvalidGoal         = "invalid_goal"
	ReasonProjectionFailed    = "projection_failed"
	ReasonEntranceUnavailable = "entrance_unavailable"
	ReasonObjectUnavailable   = "object_unavailable"
)

// NavResult is the final outcome of a navigation goal.
// Reason is empty on success (arrived) and set for stuck/failed/cancelled outcomes.
type NavResult struct {
	Status NavStatus
	Reason string
	Goal   Goal
}

// NavTickResult reports the navigator state after a single Tick.
// Done is true when the navigator reached a terminal state this tick or earlier.
type NavTickResult struct {
	Status NavStatus
	Reason string
	Done   bool
	// MovementInputSent reports a teleport cast sent by this exact tick.
	MovementInputSent bool
	// NextMovementInputAt is the earliest time the movement throttle permits another cast.
	NextMovementInputAt time.Time
	// MovementOutcomeAt is the earliest time a still unchanged position proves
	// that the sent teleport did not produce movement.
	MovementOutcomeAt time.Time
	// MovementProgressTiles is the minimum position delta that confirms cast progress.
	MovementProgressTiles float64
}
