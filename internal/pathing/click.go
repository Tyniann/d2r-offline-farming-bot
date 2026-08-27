package pathing

import (
	"log/slog"
	"math"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/input"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

// ClickStatus is the per-tick outcome of the entity click loop.
type ClickStatus string

// ClickStatus values. Terminal outcomes are hit, hover_not_found, too_far,
// and projection_failed; pending means the mouse moved and the next tick
// checks the hover buffer.
const (
	ClickPending          ClickStatus = "pending"
	ClickHit              ClickStatus = "hit"
	ClickTooFar           ClickStatus = "too_far"
	ClickHoverNotFound    ClickStatus = "hover_not_found"
	ClickProjectionFailed ClickStatus = "projection_failed"
)

// ClickTarget identifies the entity to click via hover confirmation.
type ClickTarget struct {
	UnitID   uint32
	UnitType world.HoverUnitType
	Position world.Position
	Name     string
}

// ClickTickResult reports the click-loop state after one Tick.
type ClickTickResult struct {
	Status  ClickStatus
	Attempt int // Mouse positions tried so far for the current target.
	Done    bool
	// TargetUnitID identifies the entity pinned for this bounded click search.
	TargetUnitID uint32
	// BlockerUnitID is the last monster Memory confirmed under a hover probe.
	BlockerUnitID uint32
}

// EntityClicker moves the mouse to a projected entity position and clicks
// only after memory hover data confirms the target unit is under the cursor.
// It never clicks blindly: without a hover match the loop ends in
// hover_not_found after Config.Click.MaxHoverAttempts positions.
//
// The loop is tick-based so stop/pause hotkeys stay responsive: each Tick
// first checks the hover state produced by the previous mouse move, then
// either clicks (hit) or moves to the next spiral offset.
type EntityClicker struct {
	log       *slog.Logger
	input     InputDriver
	projector Projector
	cfg       ClickConfig

	target  ClickTarget
	active  bool
	attempt int
	// lastBlockerUnitID is evidence only. It never authorizes a click and is
	// cleared whenever the click target changes or the bounded search ends.
	lastBlockerUnitID uint32
}

// NewEntityClicker wires the click loop to input, a projector, and click tuning.
func NewEntityClicker(log *slog.Logger, in InputDriver, projector Projector, cfg ClickConfig) *EntityClicker {
	return &EntityClicker{
		log:       log.With("component", "pathing.clicker"),
		input:     in,
		projector: projector,
		cfg:       cfg,
	}
}

// Reset clears the attempt state, e.g. when the navigator switches targets.
func (c *EntityClicker) Reset() {
	c.active = false
	c.attempt = 0
	c.target = ClickTarget{}
	c.lastBlockerUnitID = 0
}

// Tick advances the hover-feedback loop by one step. maxDistance gates how far
// the target may be from the player (0 disables the gate). The returned error
// covers input failures (disabled/paused/stopped); loop outcomes are reported
// via ClickTickResult.
func (c *EntityClicker) Tick(state world.State, target ClickTarget, maxDistance float64) (ClickTickResult, error) {
	if c.active && c.target.UnitID != target.UnitID {
		c.Reset()
	}

	if maxDistance > 0 {
		if d := world.Distance(state.Player.Position, target.Position); d > maxDistance {
			c.Reset()
			return ClickTickResult{Status: ClickTooFar, Done: true, TargetUnitID: target.UnitID}, nil
		}
	}
	if c.active && state.Hover.IsHovered && state.Hover.UnitType == world.HoverUnitTypeMonster {
		c.lastBlockerUnitID = state.Hover.UnitID
	}

	// Hover confirmed from the previous mouse move: click and finish.
	if c.active && state.Hover.Matches(target.UnitType, target.UnitID) {
		if err := c.input.Click(input.MouseLeft); err != nil {
			return ClickTickResult{Status: ClickPending, Attempt: c.attempt, TargetUnitID: target.UnitID, BlockerUnitID: c.lastBlockerUnitID}, err
		}
		attempts := c.attempt
		blockerUnitID := c.lastBlockerUnitID
		c.log.Info("entity click confirmed",
			"target", target.Name,
			"unit_id", target.UnitID,
			"unit_type", target.UnitType.String(),
			"hover_attempt", attempts,
		)
		c.Reset()
		return ClickTickResult{Status: ClickHit, Attempt: attempts, Done: true, TargetUnitID: target.UnitID, BlockerUnitID: blockerUnitID}, nil
	}

	if c.active && c.attempt >= c.cfg.MaxHoverAttempts {
		attempts := c.attempt
		blockerUnitID := c.lastBlockerUnitID
		c.log.Warn("entity click hover not found",
			"target", target.Name,
			"unit_id", target.UnitID,
			"unit_type", target.UnitType.String(),
			"attempts", attempts,
			"hovered", state.Hover.IsHovered,
			"hover_unit_type", state.Hover.UnitType.String(),
			"hover_unit_id", state.Hover.UnitID,
		)
		c.Reset()
		return ClickTickResult{Status: ClickHoverNotFound, Attempt: attempts, Done: true, TargetUnitID: target.UnitID, BlockerUnitID: blockerUnitID}, nil
	}

	win, ok := c.input.Window()
	if !ok {
		c.Reset()
		return ClickTickResult{Status: ClickProjectionFailed, Done: true, TargetUnitID: target.UnitID}, nil
	}

	clientX, clientY, ok := ProjectHoverProbe(c.projector, state.Player.Position, target.Position, win, c.cfg, c.attempt)
	if !ok {
		c.Reset()
		return ClickTickResult{Status: ClickProjectionFailed, Done: true, TargetUnitID: target.UnitID}, nil
	}

	if err := c.input.MoveTo(clientX, clientY); err != nil {
		return ClickTickResult{Status: ClickPending, Attempt: c.attempt, TargetUnitID: target.UnitID, BlockerUnitID: c.lastBlockerUnitID}, err
	}

	c.target = target
	c.active = true
	c.attempt++
	c.log.Debug("entity click probe",
		"target", target.Name,
		"unit_id", target.UnitID,
		"hover_attempt", c.attempt,
		"client_x", clientX,
		"client_y", clientY,
	)
	return ClickTickResult{Status: ClickPending, Attempt: c.attempt, TargetUnitID: target.UnitID, BlockerUnitID: c.lastBlockerUnitID}, nil
}

// ProjectHoverProbe projects an entity's visible-body anchor and applies one
// deterministic spiral probe. It is shared by click and combat hover loops so
// callers never need a blind per-monster pixel offset.
func ProjectHoverProbe(projector Projector, player, target world.Position, win input.WindowInfo, cfg ClickConfig, attempt int) (clientX, clientY int, ok bool) {
	if projector == nil || attempt < 0 {
		return 0, 0, false
	}
	anchor := anchoredPosition(target, cfg.AnchorOffsetTiles)
	baseX, baseY, ok := projector.Project(player, anchor, win)
	if !ok {
		return 0, 0, false
	}
	dx, dy := spiralOffset(attempt, cfg.SpiralStepDegrees)
	clientX, clientY = baseX+dx, baseY+dy
	return clientX, clientY, isPlayableClientPoint(clientX, clientY, win)
}

// anchoredPosition shifts the ground tile toward the visible entity body.
// Entrances and objects render "above" their tile in screen space, which in
// world coordinates means smaller X and Y (Koolo subtracts ~2 tiles).
func anchoredPosition(pos world.Position, offsetTiles float64) world.Position {
	off := uint32(math.Round(offsetTiles))
	out := pos
	if out.X > off {
		out.X -= off
	}
	if out.Y > off {
		out.Y -= off
	}
	return out
}

// spiralOffset returns deterministic pixel offsets on an archimedean spiral
// (Koolo helper.Spiral): attempt 0 starts near the projected point and later
// attempts sweep outward around it.
func spiralOffset(attempt int, stepDegrees float64) (dx, dy int) {
	const (
		spiralA = 4.0
		spiralB = -2.0
	)
	t := float64(attempt) * stepDegrees * math.Pi / 180.0
	x := (spiralA + spiralB*t) * math.Cos(t)
	y := (spiralA + spiralB*t) * math.Sin(t)
	return int(x), int(y)
}
