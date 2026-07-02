package pathing

import (
	"math"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

// ExploreMode selects how the planner drives the next move.
type ExploreMode string

// ExploreMode values: bearing teleports along compass directions; entity hands
// the confirmed nearby entrance to the click loop.
const (
	ExploreBearing ExploreMode = "bearing"
	ExploreEntity  ExploreMode = "entity"
)

// ExplorePlan is one planned step: either a bearing teleport target or a
// nearby entrance to click.
type ExplorePlan struct {
	Mode     ExploreMode
	Target   world.Position // Teleport target when Mode is ExploreBearing.
	Entrance world.Entrance // Click target when Mode is ExploreEntity.
}

// ExplorePlanner walks unknown layouts bearing-first: it teleports along
// rotating compass directions and only switches to entity mode when a matching
// entrance is close enough for the hover click loop to see it on screen.
// An area change resets the bearing rotation.
type ExplorePlanner struct {
	cfg          ExploreConfig
	bearingIndex int
	lastArea     world.AreaID
	hasArea      bool
}

// NewExplorePlanner builds a planner with the given explore tuning.
func NewExplorePlanner(cfg ExploreConfig) *ExplorePlanner {
	return &ExplorePlanner{cfg: cfg}
}

// Reset clears bearing rotation and area tracking, e.g. when a new goal starts.
func (p *ExplorePlanner) Reset() {
	p.bearingIndex = 0
	p.hasArea = false
}

// BearingIndex exposes the current compass index for logging.
func (p *ExplorePlanner) BearingIndex() int {
	return p.bearingIndex
}

// Rotate advances to the next compass direction (wraps at bearing_count).
func (p *ExplorePlanner) Rotate() {
	if p.cfg.BearingCount <= 0 {
		return
	}
	p.bearingIndex = (p.bearingIndex + 1) % p.cfg.BearingCount
}

// Plan returns the next explore step for state. Entity mode is only chosen
// when goal.ViaEntrance matches a visible entrance within
// max_entrance_click_distance; otherwise the current bearing target is used.
func (p *ExplorePlanner) Plan(state world.State, goal Goal) ExplorePlan {
	if !p.hasArea || state.Area.ID != p.lastArea {
		p.bearingIndex = 0
		p.lastArea = state.Area.ID
		p.hasArea = true
	}

	if goal.ViaEntrance != world.EntranceKindUnknown {
		if e, ok := state.NearestEntrance(goal.ViaEntrance); ok {
			if world.Distance(state.Player.Position, e.Position) <= p.cfg.MaxEntranceClickDistance {
				return ExplorePlan{Mode: ExploreEntity, Entrance: e}
			}
		}
	}

	return ExplorePlan{
		Mode:   ExploreBearing,
		Target: bearingTarget(state.Player.Position, p.bearingIndex, p.cfg),
	}
}

// bearingTarget offsets pos by step_distance_tiles along compass direction index.
func bearingTarget(pos world.Position, index int, cfg ExploreConfig) world.Position {
	count := cfg.BearingCount
	if count <= 0 {
		count = 8
	}
	angle := 2 * math.Pi * float64(index) / float64(count)
	dx := math.Cos(angle) * cfg.StepDistanceTiles
	dy := math.Sin(angle) * cfg.StepDistanceTiles
	return offsetPosition(pos, dx, dy)
}

// offsetPosition applies signed tile deltas to a position, clamping at zero.
func offsetPosition(pos world.Position, dx, dy float64) world.Position {
	x := float64(pos.X) + dx
	y := float64(pos.Y) + dy
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	return world.Position{X: uint32(math.Round(x)), Y: uint32(math.Round(y))}
}
