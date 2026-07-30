package app

import (
	"fmt"
	"log/slog"
	"math"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/input"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/memory"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/pathing"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/profile"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

type combatAdapter struct {
	log                 *slog.Logger
	input               inputController
	bindings            configBindingSource
	projector           pathing.RelativeProjector
	hoverProbe          pathing.ClickConfig
	forceMoveKey        string
	interval            time.Duration
	lastAction          time.Time
	pendingSkill        uint16
	pendingTargetUnitID uint32
	hoverProbeAttempt   int
}

type verifiedCombatInput interface {
	SelectSkill(input.BindingSource, uint16) error
	Click(input.MouseButton) error
}

func newCombatAdapter(log *slog.Logger, in inputController, bindings configBindingSource, cfg pathing.Config, interval time.Duration) *combatAdapter {
	return &combatAdapter{
		log:          log.With("component", "combat"),
		input:        in,
		bindings:     bindings,
		projector:    cfg.Projector(),
		hoverProbe:   cfg.Click,
		forceMoveKey: cfg.TownWalk.ForceMoveKey,
		interval:     interval,
	}
}

func (c *combatAdapter) CastAttackAtWorld(now time.Time, skillID uint16, player world.Player, targetPos world.Position) (bool, error) {
	combatInput, ok := c.input.(verifiedCombatInput)
	if !ok {
		return false, fmt.Errorf("combat verified input not wired")
	}
	cast, err := c.bindings.Resolve(skillID)
	if err != nil {
		return false, fmt.Errorf("combat resolve %s(%d): %w", memory.SkillName(skillID), skillID, err)
	}
	if cast.CastButton != input.MouseRight {
		return false, fmt.Errorf("combat attack %s(%d) must use right mouse, configured=%s", memory.SkillName(skillID), skillID, cast.CastButton)
	}
	if player.RightSkillID != skillID {
		if c.pendingSkill == skillID {
			if !c.ready(now) {
				return false, nil
			}
			return false, fmt.Errorf("combat select %s(%d): right mouse selection not confirmed, current=%s(%d)", memory.SkillName(skillID), skillID, memory.SkillName(player.RightSkillID), player.RightSkillID)
		}
		if !c.ready(now) {
			return false, nil
		}
		if selectErr := combatInput.SelectSkill(c.bindings, skillID); selectErr != nil {
			return false, fmt.Errorf("combat select %s(%d): %w", memory.SkillName(skillID), skillID, selectErr)
		}
		c.pendingSkill = skillID
		c.lastAction = now
		c.log.Info("combat right-mouse skill selection requested", "skill", memory.SkillName(skillID), "skill_id", skillID, "current_right_skill_id", player.RightSkillID)
		return false, nil
	}
	c.pendingSkill = 0
	if !c.ready(now) {
		return false, nil
	}
	clientX, clientY, err := c.project(player.Position, targetPos)
	if err != nil {
		return false, err
	}
	if err := c.input.MoveTo(clientX, clientY); err != nil {
		return false, fmt.Errorf("combat aim %s(%d): %w", memory.SkillName(skillID), skillID, err)
	}
	if err := combatInput.Click(input.MouseRight); err != nil {
		return false, fmt.Errorf("combat right-click %s(%d): %w", memory.SkillName(skillID), skillID, err)
	}
	c.lastAction = now
	c.log.Debug("combat skill cast",
		"skill", memory.SkillName(skillID),
		"skill_id", skillID,
		"target_x", targetPos.X,
		"target_y", targetPos.Y,
		"client_x", clientX,
		"client_y", clientY,
	)
	return true, nil
}

func (c *combatAdapter) CastAttackAtMonster(now time.Time, skillID uint16, player world.Player, target world.Monster) (bool, error) {
	combatInput, ok := c.input.(verifiedCombatInput)
	if !ok {
		return false, fmt.Errorf("combat verified input not wired")
	}
	cast, err := c.bindings.Resolve(skillID)
	if err != nil {
		return false, fmt.Errorf("combat resolve %s(%d): %w", memory.SkillName(skillID), skillID, err)
	}
	if cast.CastButton != input.MouseRight {
		return false, fmt.Errorf("combat attack %s(%d) must use right mouse, configured=%s", memory.SkillName(skillID), skillID, cast.CastButton)
	}
	if player.RightSkillID != skillID {
		c.pendingTargetUnitID = 0
		if c.pendingSkill == skillID {
			if !c.ready(now) {
				return false, nil
			}
			return false, fmt.Errorf("combat select %s(%d): right mouse selection not confirmed, current=%s(%d)", memory.SkillName(skillID), skillID, memory.SkillName(player.RightSkillID), player.RightSkillID)
		}
		if !c.ready(now) {
			return false, nil
		}
		if selectErr := combatInput.SelectSkill(c.bindings, skillID); selectErr != nil {
			return false, fmt.Errorf("combat select %s(%d): %w", memory.SkillName(skillID), skillID, selectErr)
		}
		c.pendingSkill = skillID
		c.lastAction = now
		c.log.Info("combat right-mouse skill selection requested", "skill", memory.SkillName(skillID), "skill_id", skillID, "current_right_skill_id", player.RightSkillID)
		return false, nil
	}
	c.pendingSkill = 0
	if !target.IsHovered {
		if c.pendingTargetUnitID != target.UnitID {
			c.hoverProbeAttempt = 0
		}
		win, windowOK := c.input.Window()
		if !windowOK {
			return false, fmt.Errorf("combat projection: window not bound")
		}
		attempt := c.hoverProbeAttempt
		if c.hoverProbe.MaxHoverAttempts > 0 {
			attempt %= c.hoverProbe.MaxHoverAttempts
		}
		clientX, clientY, projected := pathing.ProjectHoverProbe(c.projector, player.Position, target.Position, win, c.hoverProbe, attempt)
		if !projected {
			return false, fmt.Errorf("%w: unit %d", profile.ErrRouteClearTargetUnprojectable, target.UnitID)
		}
		if moveErr := c.input.MoveTo(clientX, clientY); moveErr != nil {
			return false, fmt.Errorf("combat aim monster %d: %w", target.UnitID, moveErr)
		}
		c.pendingTargetUnitID = target.UnitID
		c.hoverProbeAttempt++
		c.log.Debug("combat monster aim requested",
			"unit_id", target.UnitID,
			"npc_id", target.NPCID,
			"hover_probe_attempt", attempt+1,
			"target_x", target.Position.X,
			"target_y", target.Position.Y,
			"client_x", clientX,
			"client_y", clientY,
		)
		return false, nil
	}
	if c.pendingTargetUnitID != target.UnitID {
		// Memory already proved that this fresh target is the living monster
		// under the cursor. Attack it immediately instead of restarting aim for
		// the originally selected blocker hidden behind the same sprite.
		c.log.Debug("combat accepted hovered living monster",
			"previous_unit_id", c.pendingTargetUnitID,
			"unit_id", target.UnitID,
			"npc_id", target.NPCID,
		)
		c.pendingTargetUnitID = target.UnitID
		c.hoverProbeAttempt = 0
	}
	if !c.ready(now) {
		return false, nil
	}
	if err := combatInput.Click(input.MouseRight); err != nil {
		return false, fmt.Errorf("combat right-click %s(%d) at monster %d: %w", memory.SkillName(skillID), skillID, target.UnitID, err)
	}
	c.lastAction = now
	c.hoverProbeAttempt = 0
	c.log.Debug("combat skill cast at confirmed living monster",
		"skill", memory.SkillName(skillID),
		"skill_id", skillID,
		"unit_id", target.UnitID,
		"npc_id", target.NPCID,
		"target_x", target.Position.X,
		"target_y", target.Position.Y,
	)
	return true, nil
}

func (c *combatAdapter) StopAttack() error {
	c.pendingSkill = 0
	c.pendingTargetUnitID = 0
	c.hoverProbeAttempt = 0
	return nil
}

// MonsterAimProjectable reports whether the same visible-body anchor used by
// [combatAdapter.CastAttackAtMonster] can begin its hover search from playerPos.
func (c *combatAdapter) MonsterAimProjectable(playerPos, targetPos world.Position) bool {
	win, ok := c.input.Window()
	if !ok {
		return false
	}
	_, _, ok = pathing.ProjectHoverProbe(c.projector, playerPos, targetPos, win, c.hoverProbe, 0)
	return ok
}

// FarthestProjectableMonsterDistance finds the smallest necessary approach
// along the existing player-to-target line. It deliberately does not search
// arbitrary landing points or infer safety from geometry.
func (c *combatAdapter) FarthestProjectableMonsterDistance(playerPos, targetPos world.Position) (float64, bool) {
	distance := world.Distance(playerPos, targetPos)
	for desiredDistance := math.Floor(distance) - 1; desiredDistance > 0; desiredDistance-- {
		candidate := combatStepTowardTarget(playerPos, targetPos, desiredDistance)
		if candidate == playerPos || !c.MonsterAimProjectable(candidate, targetPos) {
			continue
		}
		return world.Distance(candidate, targetPos), true
	}
	return 0, false
}

func (c *combatAdapter) TeleportToward(now time.Time, playerPos, targetPos world.Position, desiredDistanceTiles float64) (bool, error) {
	if err := c.StopAttack(); err != nil {
		return false, err
	}
	if !c.ready(now) {
		return false, nil
	}
	teleportTarget := combatStepTowardTarget(playerPos, targetPos, desiredDistanceTiles)
	clientX, clientY, err := c.project(playerPos, teleportTarget)
	if err != nil {
		return false, err
	}
	if err := c.input.CastSkillAt(c.bindings, memory.SkillTeleport, clientX, clientY); err != nil {
		return false, fmt.Errorf("combat teleport: %w", err)
	}
	c.lastAction = now
	c.log.Debug("combat teleport toward",
		"target_x", targetPos.X,
		"target_y", targetPos.Y,
		"teleport_x", teleportTarget.X,
		"teleport_y", teleportTarget.Y,
		"desired_distance_tiles", desiredDistanceTiles,
	)
	return true, nil
}

// ForceMoveToward reuses the configured Town-Walk Force-Move binding while
// leaving route-point ownership with the task pipeline.
func (c *combatAdapter) ForceMoveToward(now time.Time, playerPos, targetPos world.Position) (bool, error) {
	if err := c.StopAttack(); err != nil {
		return false, err
	}
	if !c.ready(now) {
		return false, nil
	}
	clientX, clientY, err := c.project(playerPos, targetPos)
	if err != nil {
		return false, err
	}
	if err := c.input.MoveTo(clientX, clientY); err != nil {
		return false, fmt.Errorf("combat force move aim: %w", err)
	}
	if err := c.input.PressKey(c.forceMoveKey); err != nil {
		return false, fmt.Errorf("combat force move: %w", err)
	}
	c.lastAction = now
	c.log.Info("combat force move toward route point",
		"target_x", targetPos.X,
		"target_y", targetPos.Y,
		"client_x", clientX,
		"client_y", clientY,
		"key", c.forceMoveKey,
	)
	return true, nil
}

func (c *combatAdapter) Reset() {
	if err := c.StopAttack(); err != nil {
		c.log.Warn("combat reset could not release attack input", "error", err)
	}
	c.lastAction = time.Time{}
	c.pendingSkill = 0
	c.pendingTargetUnitID = 0
}

func (c *combatAdapter) ready(now time.Time) bool {
	if now.IsZero() {
		now = time.Now()
	}
	return c.lastAction.IsZero() || now.Sub(c.lastAction) >= c.interval
}

func (c *combatAdapter) project(playerPos, targetPos world.Position) (int, int, error) {
	win, ok := c.input.Window()
	if !ok {
		return 0, 0, fmt.Errorf("combat projection: window not bound")
	}
	clientX, clientY, ok := c.projector.Project(playerPos, targetPos, win)
	if !ok {
		return 0, 0, fmt.Errorf("combat projection failed")
	}
	return clientX, clientY, nil
}

func combatStepTowardTarget(playerPos, targetPos world.Position, desiredDistanceTiles float64) world.Position {
	dist := world.Distance(playerPos, targetPos)
	if dist <= desiredDistanceTiles || dist == 0 {
		return playerPos
	}
	move := dist - desiredDistanceTiles
	scale := move / dist
	x := float64(playerPos.X) + (float64(targetPos.X)-float64(playerPos.X))*scale
	y := float64(playerPos.Y) + (float64(targetPos.Y)-float64(playerPos.Y))*scale
	return world.Position{
		X: uint32(math.Round(x)),
		Y: uint32(math.Round(y)),
	}
}

var _ input.BindingSource = configBindingSource{}
