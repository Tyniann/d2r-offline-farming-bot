package pathing

import (
	"context"
	"log/slog"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

const (
	personalStashClientWidth  = 1280
	personalStashClientHeight = 720
	personalStashOpenTimeout  = 2500 * time.Millisecond
)

var personalStashApproachOffsets = []world.Position{
	{X: 10, Y: 18},
	{X: 4, Y: 14},
}

// PersonalStashStatus is a stable per-tick outcome of walking to and opening the personal stash.
type PersonalStashStatus string

// Personal stash action statuses.
const (
	PersonalStashPending               PersonalStashStatus = "pending"
	PersonalStashOpened                PersonalStashStatus = "opened"
	PersonalStashNotFound              PersonalStashStatus = "stash_not_found"
	PersonalStashApproachFailed        PersonalStashStatus = "stash_approach_failed"
	PersonalStashOpenFailed            PersonalStashStatus = "stash_open_failed"
	PersonalStashUnsupportedResolution PersonalStashStatus = "unsupported_resolution"
	PersonalStashWrongArea             PersonalStashStatus = "wrong_area"
	PersonalStashInputError            PersonalStashStatus = "input_error"
)

// PersonalStashResult reports one transfer-free personal-stash action tick.
type PersonalStashResult struct {
	Status PersonalStashStatus
	Reason string
	Done   bool
}

// PersonalStashActions walks to the memory-discovered Act-1 stash and opens it after hover confirmation.
// It is intentionally independent from the Town service graph because post-run
// loot transfer begins at the portal and must establish the graph's Stash origin.
type PersonalStashActions struct {
	log              *slog.Logger
	input            InputDriver
	projector        Projector
	clicker          *EntityClicker
	townWalk         TownWalkConfig
	maxClickDistance float64

	lastMoveAt     time.Time
	lastProgressAt time.Time
	lastPos        world.Position
	clickedAt      time.Time
	routeIndex     int
}

// NewPersonalStashActions wires transfer-free personal-stash navigation to pathing input.
func NewPersonalStashActions(log *slog.Logger, in InputDriver, cfg Config) *PersonalStashActions {
	return &PersonalStashActions{
		log:              log.With("component", "pathing.personal_stash"),
		input:            in,
		projector:        cfg.Projector(),
		clicker:          NewEntityClicker(log, in, cfg.Projector(), cfg.Click),
		townWalk:         cfg.TownWalk,
		maxClickDistance: cfg.Waypoint.MaxClickDistance,
	}
}

// Reset clears in-flight walking, hover, and open-confirmation state.
func (a *PersonalStashActions) Reset() {
	if a == nil {
		return
	}
	a.lastMoveAt = time.Time{}
	a.lastProgressAt = time.Time{}
	a.lastPos = world.Position{}
	a.clickedAt = time.Time{}
	a.routeIndex = 0
	if a.clicker != nil {
		a.clicker.Reset()
	}
}

// Tick advances direct town walking, hover-confirmed clicking, and Memory UI confirmation.
func (a *PersonalStashActions) Tick(ctx context.Context, state world.State) PersonalStashResult {
	if ctx.Err() != nil {
		return PersonalStashResult{Status: PersonalStashInputError, Reason: ctx.Err().Error(), Done: true}
	}
	if a == nil || a.input == nil || a.clicker == nil {
		return PersonalStashResult{Status: PersonalStashInputError, Reason: "personal stash actions not wired", Done: true}
	}
	win, ok := a.input.Window()
	if !ok || win.ClientWidth != personalStashClientWidth || win.ClientHeight != personalStashClientHeight {
		a.Reset()
		return PersonalStashResult{Status: PersonalStashUnsupportedResolution, Reason: string(PersonalStashUnsupportedResolution), Done: true}
	}
	if state.UI.StashOpen {
		a.Reset()
		return PersonalStashResult{Status: PersonalStashOpened, Done: true}
	}
	if state.UI.InventoryOpen {
		a.Reset()
		return PersonalStashResult{Status: PersonalStashOpenFailed, Reason: "inventory_open_without_stash", Done: true}
	}
	if !state.Valid || state.Phase != world.GamePhaseInGame {
		return PersonalStashResult{Status: PersonalStashPending}
	}
	if state.Area.ID != world.RogueEncampment {
		a.Reset()
		return PersonalStashResult{Status: PersonalStashWrongArea, Reason: "not_act1_town", Done: true}
	}
	stash, ok := state.NearestObject(world.ObjectKindPersonalStash)
	if !ok {
		a.Reset()
		return PersonalStashResult{Status: PersonalStashNotFound, Reason: string(PersonalStashNotFound), Done: true}
	}

	now := state.At
	if now.IsZero() {
		now = time.Now()
	}
	if !a.clickedAt.IsZero() {
		if now.Sub(a.clickedAt) >= personalStashOpenTimeout {
			a.Reset()
			return PersonalStashResult{Status: PersonalStashOpenFailed, Reason: string(PersonalStashOpenFailed), Done: true}
		}
		return PersonalStashResult{Status: PersonalStashPending}
	}

	if world.Distance(state.Player.Position, stash.Position) > a.maxClickDistance {
		return a.tickApproach(now, state, stash)
	}
	if !a.approachSettled(now, state.Player.Position) {
		return PersonalStashResult{Status: PersonalStashPending}
	}
	a.lastMoveAt = time.Time{}

	res, err := a.clicker.Tick(state, ClickTarget{
		UnitID: stash.UnitID, UnitType: world.HoverUnitTypeObject,
		Position: stash.Position, Name: stash.Name,
	}, a.maxClickDistance)
	if err != nil {
		a.Reset()
		return PersonalStashResult{Status: PersonalStashInputError, Reason: err.Error(), Done: true}
	}
	switch res.Status {
	case ClickPending:
		return PersonalStashResult{Status: PersonalStashPending}
	case ClickHit:
		a.clickedAt = now
		a.log.Info("personal stash clicked", "unit_id", stash.UnitID, "hover_attempts", res.Attempt)
		return PersonalStashResult{Status: PersonalStashPending}
	default:
		a.Reset()
		return PersonalStashResult{Status: PersonalStashOpenFailed, Reason: string(res.Status), Done: true}
	}
}

// approachSettled confirms through consecutive Memory positions that the
// force-move approach has stopped before the hover-click loop starts.
func (a *PersonalStashActions) approachSettled(now time.Time, position world.Position) bool {
	// Live validation showed that D2R can expose the correct Stash hover while
	// ignoring a click sent during residual Force-Move movement.
	if a.lastProgressAt.IsZero() {
		a.lastProgressAt = now
		a.lastPos = position
		a.clicker.Reset()
		return false
	}
	if world.Distance(a.lastPos, position) >= 1 {
		a.lastProgressAt = now
		a.lastPos = position
		a.clicker.Reset()
		return false
	}
	return now.Sub(a.lastProgressAt) >= a.townWalk.SettleTimeout
}

func (a *PersonalStashActions) tickApproach(now time.Time, state world.State, stash world.Object) PersonalStashResult {
	a.clicker.Reset()
	target := stash.Position
	for a.routeIndex < len(personalStashApproachOffsets) {
		offset := personalStashApproachOffsets[a.routeIndex]
		routeTarget := world.Position{X: stash.Position.X + offset.X, Y: stash.Position.Y + offset.Y}
		if world.Distance(state.Player.Position, routeTarget) > 3 {
			target = routeTarget
			break
		}
		a.routeIndex++
		a.lastProgressAt = now
		a.lastPos = state.Player.Position
	}
	if a.lastProgressAt.IsZero() || world.Distance(a.lastPos, state.Player.Position) >= 1 {
		a.lastProgressAt = now
		a.lastPos = state.Player.Position
	}
	if now.Sub(a.lastProgressAt) >= a.townWalk.StuckTimeout {
		a.Reset()
		return PersonalStashResult{Status: PersonalStashApproachFailed, Reason: "stuck", Done: true}
	}
	if !a.lastMoveAt.IsZero() && now.Sub(a.lastMoveAt) < a.townWalk.MoveInterval {
		return PersonalStashResult{Status: PersonalStashPending}
	}
	win, _ := a.input.Window()
	clientX, clientY, ok := a.projector.Project(state.Player.Position, target, win)
	if !ok {
		a.Reset()
		return PersonalStashResult{Status: PersonalStashApproachFailed, Reason: "projection_failed", Done: true}
	}
	clientX = clampInt(clientX, 0, win.ClientWidth-1)
	clientY = clampInt(clientY, 0, int(float64(win.ClientHeight)*maxTeleportClientYFraction))
	if err := a.input.MoveTo(clientX, clientY); err != nil {
		a.Reset()
		return PersonalStashResult{Status: PersonalStashInputError, Reason: err.Error(), Done: true}
	}
	if err := a.input.PressKey(a.townWalk.ForceMoveKey); err != nil {
		a.Reset()
		return PersonalStashResult{Status: PersonalStashInputError, Reason: err.Error(), Done: true}
	}
	a.lastMoveAt = now
	a.log.Debug("personal stash approach",
		"unit_id", stash.UnitID,
		"player_x", state.Player.Position.X,
		"player_y", state.Player.Position.Y,
		"stash_x", stash.Position.X,
		"stash_y", stash.Position.Y,
		"distance", world.Distance(state.Player.Position, stash.Position),
		"route_index", a.routeIndex,
		"target_x", target.X,
		"target_y", target.Y,
		"client_x", clientX,
		"client_y", clientY,
	)
	return PersonalStashResult{Status: PersonalStashPending}
}
