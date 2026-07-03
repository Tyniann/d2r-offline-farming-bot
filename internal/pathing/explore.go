package pathing

import (
	"math"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

// ExploreMode selects how the planner drives the next move.
type ExploreMode string

// ExploreMode values: bearing teleports along compass directions, entity hands
// a nearby entrance to the click loop, and entity_approach moves toward a
// visible entrance that is still outside click range.
const (
	ExploreBearing        ExploreMode = "bearing"
	ExploreEntity         ExploreMode = "entity"
	ExploreEntityApproach ExploreMode = "entity_approach"
)

const maxEntranceConsiderDistance = 160.0

// ExplorePlan is one planned step: either a teleport target or an entrance to
// click.
type ExplorePlan struct {
	Mode       ExploreMode
	Target     world.Position // Teleport target when Mode is Bearing or EntityApproach.
	Entrance   world.Entrance // Entity target when Mode is ExploreEntity or EntityApproach.
	ForceClick bool           // Disable distance gating after a blocked entity approach.
}

// ExplorePlanner walks unknown layouts bearing-first: it teleports along
// rotating compass directions and only switches to entity mode when a matching
// entrance is close enough for the hover click loop to see it on screen.
// An area change resets the bearing rotation.
type ExplorePlanner struct {
	cfg              ExploreConfig
	bearingIndex     int
	lastArea         world.AreaID
	hasArea          bool
	forceClickUnitID uint32
}

// NewExplorePlanner builds a planner with the given explore tuning.
func NewExplorePlanner(cfg ExploreConfig) *ExplorePlanner {
	return &ExplorePlanner{cfg: cfg}
}

// Reset clears bearing rotation and area tracking, e.g. when a new goal starts.
func (p *ExplorePlanner) Reset() {
	p.bearingIndex = 0
	p.hasArea = false
	p.forceClickUnitID = 0
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

// ForceClickEntrance makes the next plan for unitID enter the hover-click loop
// even when the player is still outside the normal click distance. It is used
// after a direct approach teleport to a visible entrance is blocked by room
// geometry.
func (p *ExplorePlanner) ForceClickEntrance(unitID uint32) {
	p.forceClickUnitID = unitID
}

// Plan returns the next explore step for state. Matching visible entrances are
// prioritized over bearing exploration: nearby entrances are clicked, and far
// entrances become an approach target.
func (p *ExplorePlanner) Plan(state world.State, goal Goal) ExplorePlan {
	if !p.hasArea || state.Area.ID != p.lastArea {
		p.bearingIndex = 0
		p.forceClickUnitID = 0
		p.lastArea = state.Area.ID
		p.hasArea = true
	}

	if goal.ViaEntrance != world.EntranceKindUnknown {
		if e, ok := p.nearestEntrance(state, goal.ViaEntrance); ok {
			return p.planEntrance(state, e)
		}
	}
	if e, ok := forgottenTowerCellar1Entrance(state, goal); ok {
		return p.planEntrance(state, e)
	}

	return ExplorePlan{
		Mode:   ExploreBearing,
		Target: bearingTarget(state.Player.Position, p.bearingIndex, p.cfg),
	}
}

func forgottenTowerCellar1Entrance(state world.State, goal Goal) (world.Entrance, bool) {
	if state.Area.ID != world.ForgottenTower || goal.TargetArea != world.TowerCellarLevel1 {
		return world.Entrance{}, false
	}
	var best world.Entrance
	bestDist := 0.0
	found := false
	for _, e := range state.Entrances {
		if e.Kind != world.EntranceKindUnknown {
			continue
		}
		d := world.Distance(state.Player.Position, e.Position)
		if !found || d < bestDist {
			best = e
			bestDist = d
			found = true
		}
	}
	return best, found
}

func (p *ExplorePlanner) nearestEntrance(state world.State, kind world.EntranceKind) (world.Entrance, bool) {
	if !state.Valid {
		return world.Entrance{}, false
	}
	var best world.Entrance
	bestDist := 0.0
	found := false
	for _, e := range state.Entrances {
		if e.Kind != kind {
			continue
		}
		d := world.Distance(state.Player.Position, e.Position)
		if d > maxEntranceConsiderDistance {
			continue
		}
		if !found || d < bestDist {
			best = e
			bestDist = d
			found = true
		}
	}
	return best, found
}

func (p *ExplorePlanner) planEntrance(state world.State, e world.Entrance) ExplorePlan {
	dist := world.Distance(state.Player.Position, e.Position)
	forceClick := e.UnitID != 0 && e.UnitID == p.forceClickUnitID
	if forceClick || dist <= p.cfg.MaxEntranceClickDistance {
		return ExplorePlan{Mode: ExploreEntity, Entrance: e, ForceClick: forceClick}
	}
	return ExplorePlan{
		Mode:     ExploreEntityApproach,
		Target:   stepToward(state.Player.Position, e.Position, p.entityApproachStep()),
		Entrance: e,
	}
}

func (p *ExplorePlanner) entityApproachStep() float64 {
	step := p.cfg.StepDistanceTiles
	if p.cfg.MaxEntranceClickDistance > 0 {
		step = math.Min(step, p.cfg.MaxEntranceClickDistance/2)
	}
	return math.Max(1, step)
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
