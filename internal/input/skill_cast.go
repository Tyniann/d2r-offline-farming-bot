package input

import (
	"fmt"
	"time"
)

// SkillCast describes a resolved in-game skill action from configured bindings.
type SkillCast struct {
	SkillID    uint16
	SelectKey  string
	CastButton MouseButton
}

// BindingSource supplies configured skill bindings.
type BindingSource interface {
	Resolve(skillID uint16) (SkillCast, error)
}

// BeltBindingSource supplies configured belt hotkeys.
type BeltBindingSource interface {
	BeltKeyName(slot int) (string, error)
}

// CastSkill selects a skill and optionally moves and clicks to cast it.
// Pass clientX and clientY >= 0 to move and click; negative coordinates select only (no click).
func (c *Controller) CastSkill(src BindingSource, skillID uint16, clientX, clientY int) error {
	if clientX < 0 || clientY < 0 {
		return c.SelectSkill(src, skillID)
	}
	return c.CastSkillAt(src, skillID, clientX, clientY)
}

// NoClientCoord is used with CastSkill to select a skill without moving or clicking.
const NoClientCoord = -1

// SelectSkill presses the in-game hotkey that places skillID on the mouse cursor.
func (c *Controller) SelectSkill(src BindingSource, skillID uint16) error {
	cast, err := src.Resolve(skillID)
	if err != nil {
		return err
	}
	k, err := NormalizeKey(cast.SelectKey)
	if err != nil {
		return err
	}
	return c.pressKey(k, "skill_select")
}

// CastSkillAt selects a skill, moves to client coordinates, and clicks the bound mouse button.
func (c *Controller) CastSkillAt(src BindingSource, skillID uint16, clientX, clientY int) error {
	cast, err := src.Resolve(skillID)
	if err != nil {
		return err
	}
	if guardErr := c.actionGuard("skill", "cast", "skill_cast",
		"skill_id", skillID,
		"select_key", cast.SelectKey,
		"cast_button", cast.CastButton,
		"client_x", clientX,
		"client_y", clientY,
	); guardErr != nil {
		return guardErr
	}

	k, err := NormalizeKey(cast.SelectKey)
	if err != nil {
		return err
	}
	if err := c.pressKey(k, "skill_select"); err != nil {
		return err
	}
	if hold := c.keyboard.ComboHoldMs; hold > 0 {
		c.timings.sleep(time.Duration(hold) * time.Millisecond)
	}

	c.mu.Lock()
	if !c.bound {
		c.mu.Unlock()
		return fmt.Errorf("cast skill: %w", ErrWindowNotBound)
	}
	win := c.window
	c.mu.Unlock()

	if err := c.moveTo(win, clientX, clientY, "skill_cast_move"); err != nil {
		return err
	}
	if err := c.click(cast.CastButton, "skill_cast_click"); err != nil {
		return err
	}

	c.logAllowedAction("skill", "cast", "skill_cast",
		"skill_id", skillID,
		"select_key", cast.SelectKey,
		"cast_button", cast.CastButton,
		"client_x", clientX,
		"client_y", clientY,
	)
	return nil
}

// CastBelt presses the in-game belt hotkey for the given slot (1-based).
func (c *Controller) CastBelt(src BeltBindingSource, slot int) error {
	if slot < 1 || slot > 4 {
		return fmt.Errorf("cast belt slot %d: %w", slot, ErrInvalidSlot)
	}
	key, err := src.BeltKeyName(slot)
	if err != nil {
		return err
	}
	if key == "" {
		return fmt.Errorf("cast belt slot %d: %w", slot, ErrUnconfiguredSlot)
	}
	k, err := NormalizeKey(key)
	if err != nil {
		return err
	}
	return c.pressKey(k, "belt_slot")
}
