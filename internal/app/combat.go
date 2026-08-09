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

const combatMonsterMaxHoverAttempts = 5

type combatAdapter struct {
	log                 *slog.Logger
	input               inputController
	bindings            configBindingSource
	projector           pathing.RelativeProjector
	hoverProbe          pathing.ClickConfig
	forceMoveKey        string
	interval            time.Duration
	lastAction          time.Time
	selector            *RightSkillSelector
	pendingTargetUnitID uint32
	hoverProbeAttempt   int
}

type verifiedCombatInput interface {
	SelectSkill(input.BindingSource, uint16) error
	Click(input.MouseButton) error
}

func newCombatAdapter(log *slog.Logger, in inputController, bindings configBindingSource, cfg pathing.Config, interval time.Duration) *combatAdapter {
	hoverProbe := cfg.Click
	if hoverProbe.MaxHoverAttempts > combatMonsterMaxHoverAttempts {
		// Moving monsters invalidate projected sprite positions much faster than
		// static UI entities. Keep the shared click configuration as an upper
		// bound, but never spend the full UI search budget on one combat target.
		hoverProbe.MaxHoverAttempts = combatMonsterMaxHoverAttempts
	}
	adapter := &combatAdapter{
		log:          log.With("component", "combat"),
		input:        in,
		bindings:     bindings,
		projector:    cfg.Projector(),
		hoverProbe:   hoverProbe,
		forceMoveKey: cfg.TownWalk.ForceMoveKey,
		interval:     interval,
	}
	if combatInput, ok := in.(verifiedCombatInput); ok {
		adapter.selector = NewRightSkillSelector(bindings, combatInput)
	}
	return adapter
}

func (c *combatAdapter) CastAttackAtWorld(now time.Time, skillID uint16, player world.Player, targetPos world.Position) (bool, error) {
	combatInput, ok := c.input.(verifiedCombatInput)
	if !ok || c.selector == nil {
		return false, fmt.Errorf("combat verified input not wired")
	}
	if player.RightSkillID == skillID {
		if !c.ready(now) {
			return false, nil
		}
	} else if !c.ready(now) && c.selector.pending != skillID {
		return false, nil
	}
	sent, err := c.selector.EnsureAndCast(skillID, player.RightSkillID, now, func() error {
		clientX, clientY, projectErr := c.project(player.Position, targetPos)
		if projectErr != nil {
			return projectErr
		}
		if moveErr := c.input.MoveTo(clientX, clientY); moveErr != nil {
			return fmt.Errorf("combat aim %s(%d): %w", memory.SkillName(skillID), skillID, moveErr)
		}
		if clickErr := combatInput.Click(input.MouseRight); clickErr != nil {
			return fmt.Errorf("combat right-click %s(%d): %w", memory.SkillName(skillID), skillID, clickErr)
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
	})
	if err != nil {
		return false, err
	}
	if !sent {
		if c.selector.pending == skillID && c.selector.requestedAt.Equal(now) {
			c.lastAction = now
			c.log.Info("combat right-mouse skill selection requested", "skill", memory.SkillName(skillID), "skill_id", skillID, "current_right_skill_id", player.RightSkillID)
		}
		return false, nil
	}
	return true, nil
}

func (c *combatAdapter) CastAttackAtMonster(now time.Time, skillID uint16, player world.Player, target world.Monster) (profile.MonsterCastResult, error) {
	combatInput, ok := c.input.(verifiedCombatInput)
	if !ok || c.selector == nil {
		return profile.MonsterCastResult{}, fmt.Errorf("combat verified input not wired")
	}
	if player.RightSkillID != skillID {
		c.pendingTargetUnitID = 0
		if !c.ready(now) && c.selector.pending != skillID {
			return profile.MonsterCastResult{}, nil
		}
		sent, selectErr := c.selector.EnsureAndCast(skillID, player.RightSkillID, now, func() error {
			return nil
		})
		if selectErr != nil {
			return profile.MonsterCastResult{}, fmt.Errorf("combat select %s(%d): %w", memory.SkillName(skillID), skillID, selectErr)
		}
		if !sent {
			c.lastAction = now
			c.log.Info("combat right-mouse skill selection requested", "skill", memory.SkillName(skillID), "skill_id", skillID, "current_right_skill_id", player.RightSkillID)
		}
		return profile.MonsterCastResult{}, nil
	}
	c.selector.Reset()
	if !target.IsHovered {
		if c.pendingTargetUnitID != target.UnitID {
			c.hoverProbeAttempt = 0
		}
		if c.hoverProbe.MaxHoverAttempts > 0 && c.hoverProbeAttempt >= c.hoverProbe.MaxHoverAttempts {
			attempts := c.hoverProbeAttempt
			win, windowOK := c.input.Window()
			if !windowOK {
				return profile.MonsterCastResult{}, fmt.Errorf("combat projection: window not bound")
			}
			clientX, clientY, projected := pathing.ProjectHoverProbe(c.projector, player.Position, target.Position, win, c.hoverProbe, 0)
			if !projected {
				c.pendingTargetUnitID = 0
				c.hoverProbeAttempt = 0
				return profile.MonsterCastResult{}, fmt.Errorf("%w: unit %d", profile.ErrRouteClearTargetUnprojectable, target.UnitID)
			}
			if !c.ready(now) {
				return profile.MonsterCastResult{}, nil
			}
			if moveErr := c.input.MoveTo(clientX, clientY); moveErr != nil {
				return profile.MonsterCastResult{}, fmt.Errorf("combat project monster %d: %w", target.UnitID, moveErr)
			}
			if clickErr := combatInput.Click(input.MouseRight); clickErr != nil {
				return profile.MonsterCastResult{}, fmt.Errorf("combat projected right-click %s(%d) at monster %d: %w", memory.SkillName(skillID), skillID, target.UnitID, clickErr)
			}
			c.lastAction = now
			c.pendingTargetUnitID = 0
			c.hoverProbeAttempt = 0
			c.log.Info("combat skill cast at projected living monster",
				"unit_id", target.UnitID,
				"npc_id", target.NPCID,
				"skill", memory.SkillName(skillID),
				"skill_id", skillID,
				"hover_attempts", attempts,
				"target_x", target.Position.X,
				"target_y", target.Position.Y,
				"client_x", clientX,
				"client_y", clientY,
			)
			return profile.MonsterCastResult{Sent: true, TargetingMode: profile.MonsterTargetingWorldProjected}, nil
		}
		win, windowOK := c.input.Window()
		if !windowOK {
			return profile.MonsterCastResult{}, fmt.Errorf("combat projection: window not bound")
		}
		attempt := c.hoverProbeAttempt
		clientX, clientY, projected := pathing.ProjectHoverProbe(c.projector, player.Position, target.Position, win, c.hoverProbe, attempt)
		if !projected {
			return profile.MonsterCastResult{}, fmt.Errorf("%w: unit %d", profile.ErrRouteClearTargetUnprojectable, target.UnitID)
		}
		if moveErr := c.input.MoveTo(clientX, clientY); moveErr != nil {
			return profile.MonsterCastResult{}, fmt.Errorf("combat aim monster %d: %w", target.UnitID, moveErr)
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
		return profile.MonsterCastResult{}, nil
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
		return profile.MonsterCastResult{}, nil
	}
	if err := combatInput.Click(input.MouseRight); err != nil {
		return profile.MonsterCastResult{}, fmt.Errorf("combat right-click %s(%d) at monster %d: %w", memory.SkillName(skillID), skillID, target.UnitID, err)
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
	return profile.MonsterCastResult{Sent: true, TargetingMode: profile.MonsterTargetingHoverConfirmed}, nil
}

func (c *combatAdapter) StopAttack() error {
	if c.selector != nil {
		c.selector.Reset()
	}
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

// FarthestProjectableMonsterApproach finds the smallest necessary approach
// along the existing player-to-target line. It deliberately does not search
// arbitrary landing points or infer safety from geometry.
func (c *combatAdapter) FarthestProjectableMonsterApproach(playerPos, targetPos world.Position) (world.Position, float64, bool) {
	distance := world.Distance(playerPos, targetPos)
	for desiredDistance := math.Floor(distance) - 1; desiredDistance > 0; desiredDistance-- {
		candidate := combatStepTowardTarget(playerPos, targetPos, desiredDistance)
		if candidate == playerPos || !c.MonsterAimProjectable(candidate, targetPos) {
			continue
		}
		return candidate, world.Distance(candidate, targetPos), true
	}
	return world.Position{}, 0, false
}

func (c *combatAdapter) TeleportToward(now time.Time, player world.Player, targetPos world.Position, desiredDistanceTiles float64) (bool, error) {
	combatInput, ok := c.input.(verifiedCombatInput)
	if !ok || c.selector == nil {
		return false, fmt.Errorf("combat verified input not wired")
	}
	c.pendingTargetUnitID = 0
	c.hoverProbeAttempt = 0
	if c.selector.pending != 0 && c.selector.pending != memory.SkillTeleport {
		c.selector.Reset()
	}
	if player.RightSkillID == memory.SkillTeleport {
		if !c.ready(now) {
			return false, nil
		}
	} else if !c.ready(now) && c.selector.pending != memory.SkillTeleport {
		return false, nil
	}
	teleportTarget := combatStepTowardTarget(player.Position, targetPos, desiredDistanceTiles)
	clientX, clientY, err := c.project(player.Position, teleportTarget)
	if err != nil {
		return false, err
	}
	sent, err := c.selector.EnsureAndCast(memory.SkillTeleport, player.RightSkillID, now, func() error {
		if moveErr := c.input.MoveTo(clientX, clientY); moveErr != nil {
			return fmt.Errorf("combat teleport aim: %w", moveErr)
		}
		if clickErr := combatInput.Click(input.MouseRight); clickErr != nil {
			return fmt.Errorf("combat teleport click: %w", clickErr)
		}
		c.lastAction = now
		c.log.Debug("combat teleport toward",
			"target_x", targetPos.X,
			"target_y", targetPos.Y,
			"teleport_x", teleportTarget.X,
			"teleport_y", teleportTarget.Y,
			"desired_distance_tiles", desiredDistanceTiles,
			"client_x", clientX,
			"client_y", clientY,
		)
		return nil
	})
	if err != nil {
		return false, err
	}
	if !sent {
		c.lastAction = now
		c.log.Info("combat teleport skill selection requested", "current_right_skill_id", player.RightSkillID)
	}
	return sent, nil
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
