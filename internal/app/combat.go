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
	skills              *SkillSelector
	selector            *RightSkillSelector
	pendingTargetUnitID uint32
	hoverProbeAttempt   int
	attackHeld          bool
}

type verifiedCombatInput interface {
	SelectSkill(input.BindingSource, uint16) error
	Click(input.MouseButton) error
}

type modifierHoldInput interface {
	HoldAt(clientX, clientY int, button input.MouseButton) error
	ReleaseModifierHold() error
	ModifierHoldActive() bool
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
	adapter.log.Info("combat attack interval", "attack_interval_ms", interval.Milliseconds())
	if combatInput, ok := in.(verifiedCombatInput); ok {
		adapter.skills = NewSkillSelector(bindings, combatInput)
		adapter.selector = &RightSkillSelector{selector: adapter.skills, timeout: rightSkillSelectionTimeout}
	}
	return adapter
}

func (c *combatAdapter) CastAttackAtWorld(now time.Time, skillID uint16, player world.Player, targetPos world.Position) (bool, error) {
	cast, err := c.bindings.Resolve(skillID)
	if err != nil {
		return false, fmt.Errorf("combat resolve %s(%d): %w", memory.SkillName(skillID), skillID, err)
	}
	if cast.CastButton == input.MouseLeft {
		result, err := c.HoldStandardAttack(now, skillID, player, world.Monster{Position: targetPos, IsHovered: true})
		return result.Sent, err
	}
	return c.castRightAttackAtWorld(now, skillID, player, targetPos)
}

// HoldStandardAttack aims at the living target's visible body, then presses
// LMB and leaves it down until [combatAdapter.StopAttack]. Hover ID may
// belong to an overlaying monster; the cursor uses the supplied world
// position. A second call while the hold is active is a no-op.
func (c *combatAdapter) HoldStandardAttack(now time.Time, skillID uint16, player world.Player, target world.Monster) (profile.MonsterCastResult, error) {
	return c.holdLeftAttackAtMonster(now, skillID, player, target)
}

func (c *combatAdapter) castRightAttackAtWorld(now time.Time, skillID uint16, player world.Player, targetPos world.Position) (bool, error) {
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

func (c *combatAdapter) holdLeftAttackAtMonster(now time.Time, skillID uint16, player world.Player, target world.Monster) (profile.MonsterCastResult, error) {
	if c.skills == nil {
		return profile.MonsterCastResult{}, fmt.Errorf("combat verified input not wired")
	}
	holder, ok := c.input.(modifierHoldInput)
	if !ok {
		return profile.MonsterCastResult{}, fmt.Errorf("combat modified hold not wired")
	}
	if c.standardAttackHeld(holder) {
		if c.pendingTargetUnitID == 0 || c.pendingTargetUnitID == target.UnitID {
			return profile.MonsterCastResult{}, nil
		}
		if err := holder.ReleaseModifierHold(); err != nil {
			return profile.MonsterCastResult{}, fmt.Errorf("combat release standard attack hold: %w", err)
		}
		c.attackHeld = false
		c.log.Info("combat left-mouse skill hold released", "reason", "target_changed", "previous_unit_id", c.pendingTargetUnitID, "unit_id", target.UnitID)
	}
	if concentrationID, bound := c.boundConcentrationID(); bound && player.RightSkillID != concentrationID {
		if !c.ready(now) && c.skills.pending[SkillSlotRight] != concentrationID {
			return profile.MonsterCastResult{}, nil
		}
		confirmed, err := c.skills.EnsureSelected(SkillSlotRight, concentrationID, player.LeftSkillID, player.RightSkillID, now)
		c.syncRightSelector()
		if err != nil {
			return profile.MonsterCastResult{}, err
		}
		if !confirmed && c.skills.pending[SkillSlotRight] == concentrationID && c.skills.requestedAt[SkillSlotRight].Equal(now) {
			c.lastAction = now
			c.log.Info("combat right-mouse skill selection requested", "skill", memory.SkillName(concentrationID), "skill_id", concentrationID, "current_right_skill_id", player.RightSkillID)
			if err := c.primeMonsterHover(player, target); err != nil {
				return profile.MonsterCastResult{}, err
			}
		}
		return profile.MonsterCastResult{}, nil
	}
	if player.LeftSkillID == skillID {
		if !c.ready(now) {
			return profile.MonsterCastResult{}, nil
		}
	} else if !c.ready(now) && c.skills.pending[SkillSlotLeft] != skillID {
		return profile.MonsterCastResult{}, nil
	}
	clientX, clientY, mode, aimOnly, err := c.monsterHoldCursor(player, target)
	if err != nil {
		return profile.MonsterCastResult{}, err
	}
	if aimOnly {
		c.lastAction = now
		return profile.MonsterCastResult{AimRequested: true}, nil
	}
	sent, err := c.skills.EnsureAndCast(SkillSlotLeft, skillID, player.LeftSkillID, player.RightSkillID, now, func() error {
		if holdErr := holder.HoldAt(clientX, clientY, input.MouseLeft); holdErr != nil {
			return fmt.Errorf("combat left hold %s(%d): %w", memory.SkillName(skillID), skillID, holdErr)
		}
		c.attackHeld = true
		c.lastAction = now
		c.log.Info("combat left-mouse skill hold started",
			"skill", memory.SkillName(skillID),
			"skill_id", skillID,
			"left_skill_id", player.LeftSkillID,
			"unit_id", target.UnitID,
			"npc_id", target.NPCID,
			"hovered", target.IsHovered,
			"targeting_mode", string(mode),
			"mana", player.Mana,
			"max_mana", player.MaxMana,
			"target_x", target.Position.X,
			"target_y", target.Position.Y,
			"client_x", clientX,
			"client_y", clientY,
		)
		return nil
	})
	if err != nil {
		return profile.MonsterCastResult{}, err
	}
	if !sent {
		if c.skills.pending[SkillSlotLeft] == skillID && c.skills.requestedAt[SkillSlotLeft].Equal(now) {
			c.lastAction = now
			c.log.Info("combat left-mouse skill selection requested", "skill", memory.SkillName(skillID), "skill_id", skillID, "current_left_skill_id", player.LeftSkillID)
		}
		return profile.MonsterCastResult{}, nil
	}
	return profile.MonsterCastResult{Sent: true, TargetingMode: mode}, nil
}

// primeMonsterHover overlaps the cursor move with Concentration selection.
// The next snapshot still has to confirm the living unit under the cursor
// before LMB may be held.
func (c *combatAdapter) primeMonsterHover(player world.Player, target world.Monster) error {
	if target.IsHovered {
		return nil
	}
	win, ok := c.input.Window()
	if !ok {
		return fmt.Errorf("combat projection: window not bound")
	}
	clientX, clientY, projected := pathing.ProjectHoverProbe(c.projector, player.Position, target.Position, win, c.hoverProbe, 0)
	if !projected {
		return nil
	}
	if err := c.input.MoveTo(clientX, clientY); err != nil {
		return fmt.Errorf("combat prime monster %d: %w", target.UnitID, err)
	}
	c.pendingTargetUnitID = target.UnitID
	c.hoverProbeAttempt = 1
	c.log.Info("combat monster aim requested",
		"unit_id", target.UnitID,
		"npc_id", target.NPCID,
		"hover_probe_attempt", 1,
		"target_x", target.Position.X,
		"target_y", target.Position.Y,
		"client_x", clientX,
		"client_y", clientY,
		"overlapped_with", "concentration_select",
	)
	return nil
}

func (c *combatAdapter) monsterHoldCursor(player world.Player, target world.Monster) (clientX, clientY int, mode profile.MonsterTargetingMode, aimOnly bool, err error) {
	win, ok := c.input.Window()
	if !ok {
		return 0, 0, "", false, fmt.Errorf("combat projection: window not bound")
	}
	if target.IsHovered {
		var projected bool
		clientX, clientY, projected = pathing.ProjectHoverProbe(c.projector, player.Position, target.Position, win, c.hoverProbe, 0)
		if !projected {
			return 0, 0, "", false, fmt.Errorf("%w: unit %d", profile.ErrRouteClearTargetUnprojectable, target.UnitID)
		}
		c.pendingTargetUnitID = target.UnitID
		c.hoverProbeAttempt = 0
		return clientX, clientY, profile.MonsterTargetingHoverConfirmed, false, nil
	}
	if c.pendingTargetUnitID != target.UnitID {
		c.hoverProbeAttempt = 0
	}
	if c.hoverProbe.MaxHoverAttempts > 0 && c.hoverProbeAttempt >= c.hoverProbe.MaxHoverAttempts {
		var projected bool
		clientX, clientY, projected = pathing.ProjectHoverProbe(c.projector, player.Position, target.Position, win, c.hoverProbe, 0)
		if !projected {
			c.pendingTargetUnitID = 0
			c.hoverProbeAttempt = 0
			return 0, 0, "", false, fmt.Errorf("%w: unit %d", profile.ErrRouteClearTargetUnprojectable, target.UnitID)
		}
		c.pendingTargetUnitID = 0
		c.hoverProbeAttempt = 0
		return clientX, clientY, profile.MonsterTargetingWorldProjected, false, nil
	}
	attempt := c.hoverProbeAttempt
	var projected bool
	clientX, clientY, projected = pathing.ProjectHoverProbe(c.projector, player.Position, target.Position, win, c.hoverProbe, attempt)
	if !projected {
		return 0, 0, "", false, fmt.Errorf("%w: unit %d", profile.ErrRouteClearTargetUnprojectable, target.UnitID)
	}
	if moveErr := c.input.MoveTo(clientX, clientY); moveErr != nil {
		return 0, 0, "", false, fmt.Errorf("combat aim monster %d: %w", target.UnitID, moveErr)
	}
	c.pendingTargetUnitID = target.UnitID
	c.hoverProbeAttempt++
	c.log.Info("combat monster aim requested",
		"unit_id", target.UnitID,
		"npc_id", target.NPCID,
		"hover_probe_attempt", attempt+1,
		"target_x", target.Position.X,
		"target_y", target.Position.Y,
		"client_x", clientX,
		"client_y", clientY,
	)
	return clientX, clientY, "", true, nil
}

func (c *combatAdapter) CastAttackAtMonster(now time.Time, skillID uint16, player world.Player, target world.Monster) (profile.MonsterCastResult, error) {
	cast, err := c.bindings.Resolve(skillID)
	if err != nil {
		return profile.MonsterCastResult{}, fmt.Errorf("combat resolve %s(%d): %w", memory.SkillName(skillID), skillID, err)
	}
	if cast.CastButton == input.MouseLeft {
		return c.HoldStandardAttack(now, skillID, player, target)
	}
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
		return profile.MonsterCastResult{AimRequested: true}, nil
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
	if holder, ok := c.input.(modifierHoldInput); ok && (c.attackHeld || holder.ModifierHoldActive()) {
		if err := holder.ReleaseModifierHold(); err != nil {
			return fmt.Errorf("combat release standard attack hold: %w", err)
		}
		c.log.Info("combat left-mouse skill hold released")
	}
	c.attackHeld = false
	if c.skills != nil {
		c.skills.Reset()
	}
	c.syncRightSelector()
	c.pendingTargetUnitID = 0
	c.hoverProbeAttempt = 0
	return nil
}

func (c *combatAdapter) standardAttackHeld(holder modifierHoldInput) bool {
	if c.attackHeld && holder.ModifierHoldActive() {
		return true
	}
	c.attackHeld = false
	return false
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
	if holder, ok := c.input.(modifierHoldInput); ok && (c.attackHeld || holder.ModifierHoldActive()) {
		if err := c.StopAttack(); err != nil {
			return false, err
		}
	}
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
	if win, ok := c.input.Window(); ok {
		// Route teleports already exclude the bottom HUD; combat teleports
		// must use the same playable Y so RMB cannot open the belt.
		clientX, clientY = pathing.ClampTeleportClientPoint(clientX, clientY, win)
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

func (c *combatAdapter) boundConcentrationID() (uint16, bool) {
	id := memory.MustSkillID("concentration")
	if _, err := c.bindings.Resolve(id); err != nil {
		return 0, false
	}
	return id, true
}

func (c *combatAdapter) syncRightSelector() {
	if c.selector == nil {
		return
	}
	if c.skills == nil {
		c.selector.pending = 0
		c.selector.requestedAt = time.Time{}
		return
	}
	c.selector.pending = c.skills.pending[SkillSlotRight]
	c.selector.requestedAt = c.skills.requestedAt[SkillSlotRight]
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
