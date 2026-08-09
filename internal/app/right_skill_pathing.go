package app

import (
	"fmt"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/input"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/pathing"
)

// rightSkillPathingClicker adapts [RightSkillSelector] for [pathing.TeleportMover].
type rightSkillPathingClicker struct {
	selector *RightSkillSelector
	mover    interface {
		MoveTo(clientX, clientY int) error
	}
	clicker verifiedCombatInput
}

func (a rightSkillPathingClicker) CastRightSkillAt(skillID uint16, rightSkillID uint16, now time.Time, clientX, clientY int) (bool, error) {
	if a.selector == nil || a.mover == nil || a.clicker == nil {
		return false, fmt.Errorf("pathing right skill clicker not wired")
	}
	return a.selector.EnsureAndCast(skillID, rightSkillID, now, func() error {
		if err := a.mover.MoveTo(clientX, clientY); err != nil {
			return err
		}
		return a.clicker.Click(input.MouseRight)
	})
}

func wireNavigatorRightSkill(nav *pathing.Navigator, combat *combatAdapter, in inputController) {
	if nav == nil || combat == nil || combat.selector == nil {
		return
	}
	combatInput, ok := in.(verifiedCombatInput)
	if !ok {
		return
	}
	nav.SetRightSkillClicker(rightSkillPathingClicker{selector: combat.selector, mover: in, clicker: combatInput})
}

func wireTeleportMover(mover *pathing.TeleportMover, combat *combatAdapter, in inputController) {
	if mover == nil || combat == nil || combat.selector == nil {
		return
	}
	combatInput, ok := in.(verifiedCombatInput)
	if !ok {
		return
	}
	mover.SetRightSkillClicker(rightSkillPathingClicker{selector: combat.selector, mover: in, clicker: combatInput})
}
