package app

import (
	"fmt"
	"log/slog"
	"math"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/input"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/memory"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/pathing"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

type combatAdapter struct {
	log          *slog.Logger
	input        inputController
	bindings     configBindingSource
	projector    pathing.RelativeProjector
	interval     time.Duration
	lastAction   time.Time
	pendingSkill uint16
}

type verifiedCombatInput interface {
	SelectSkill(input.BindingSource, uint16) error
	Click(input.MouseButton) error
}

func newCombatAdapter(log *slog.Logger, in inputController, bindings configBindingSource, cfg pathing.Config, interval time.Duration) *combatAdapter {
	return &combatAdapter{
		log:       log.With("component", "combat"),
		input:     in,
		bindings:  bindings,
		projector: cfg.Projector(),
		interval:  interval,
	}
}

func (c *combatAdapter) CastAttackAtWorld(now time.Time, skillID uint16, player world.Player, targetPos world.Position) error {
	combatInput, ok := c.input.(verifiedCombatInput)
	if !ok {
		return fmt.Errorf("combat verified input not wired")
	}
	cast, err := c.bindings.Resolve(skillID)
	if err != nil {
		return fmt.Errorf("combat resolve %s(%d): %w", memory.SkillName(skillID), skillID, err)
	}
	if cast.CastButton != input.MouseRight {
		return fmt.Errorf("combat attack %s(%d) must use right mouse, configured=%s", memory.SkillName(skillID), skillID, cast.CastButton)
	}
	if player.RightSkillID != skillID {
		if c.pendingSkill == skillID {
			if !c.ready(now) {
				return nil
			}
			return fmt.Errorf("combat select %s(%d): right mouse selection not confirmed, current=%s(%d)", memory.SkillName(skillID), skillID, memory.SkillName(player.RightSkillID), player.RightSkillID)
		}
		if !c.ready(now) {
			return nil
		}
		if selectErr := combatInput.SelectSkill(c.bindings, skillID); selectErr != nil {
			return fmt.Errorf("combat select %s(%d): %w", memory.SkillName(skillID), skillID, selectErr)
		}
		c.pendingSkill = skillID
		c.lastAction = now
		c.log.Info("combat right-mouse skill selection requested", "skill", memory.SkillName(skillID), "skill_id", skillID, "current_right_skill_id", player.RightSkillID)
		return nil
	}
	c.pendingSkill = 0
	if !c.ready(now) {
		return nil
	}
	clientX, clientY, err := c.project(player.Position, targetPos)
	if err != nil {
		return err
	}
	if err := c.input.MoveTo(clientX, clientY); err != nil {
		return fmt.Errorf("combat aim %s(%d): %w", memory.SkillName(skillID), skillID, err)
	}
	if err := combatInput.Click(input.MouseRight); err != nil {
		return fmt.Errorf("combat right-click %s(%d): %w", memory.SkillName(skillID), skillID, err)
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
	return nil
}

func (c *combatAdapter) StopAttack() error {
	c.pendingSkill = 0
	return nil
}

func (c *combatAdapter) TeleportToward(now time.Time, playerPos, targetPos world.Position, desiredDistanceTiles float64) error {
	if err := c.StopAttack(); err != nil {
		return err
	}
	if !c.ready(now) {
		return nil
	}
	teleportTarget := combatStepTowardTarget(playerPos, targetPos, desiredDistanceTiles)
	clientX, clientY, err := c.project(playerPos, teleportTarget)
	if err != nil {
		return err
	}
	if err := c.input.CastSkillAt(c.bindings, memory.SkillTeleport, clientX, clientY); err != nil {
		return fmt.Errorf("combat teleport: %w", err)
	}
	c.lastAction = now
	c.log.Debug("combat teleport toward",
		"target_x", targetPos.X,
		"target_y", targetPos.Y,
		"teleport_x", teleportTarget.X,
		"teleport_y", teleportTarget.Y,
		"desired_distance_tiles", desiredDistanceTiles,
	)
	return nil
}

func (c *combatAdapter) Reset() {
	if err := c.StopAttack(); err != nil {
		c.log.Warn("combat reset could not release attack input", "error", err)
	}
	c.lastAction = time.Time{}
	c.pendingSkill = 0
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
