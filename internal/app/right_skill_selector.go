package app

import (
	"fmt"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/input"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/memory"
)

const skillSelectionTimeout = 1500 * time.Millisecond

// rightSkillSelectionTimeout preserves the existing test and behavior contract.
const rightSkillSelectionTimeout = skillSelectionTimeout

// SkillSlot identifies the mouse slot on which a selected skill must appear.
type SkillSlot uint8

const (
	// SkillSlotLeft requires confirmation through `LeftSkillID`.
	SkillSlotLeft SkillSlot = iota
	// SkillSlotRight requires confirmation through `RightSkillID`.
	SkillSlotRight
)

func (s SkillSlot) label() string {
	if s == SkillSlotLeft {
		return "left"
	}
	return "right"
}

func (s SkillSlot) button() input.MouseButton {
	if s == SkillSlotLeft {
		return input.MouseLeft
	}
	return input.MouseRight
}

// SkillSelector owns independent select-confirm state for LMB and RMB. It
// never invokes the productive cast in the same tick that sends a select key.
type SkillSelector struct {
	bindings       configBindingSource
	input          verifiedCombatInput
	pending        [2]uint16
	skillAtRequest [2]uint16
	requestedAt    [2]time.Time
	timeout        time.Duration
}

// NewSkillSelector wires shared slot-aware select-confirm-cast behavior.
func NewSkillSelector(bindings configBindingSource, in verifiedCombatInput) *SkillSelector {
	return &SkillSelector{bindings: bindings, input: in, timeout: skillSelectionTimeout}
}

// Reset clears both in-flight slot selections, for example after game change.
func (s *SkillSelector) Reset() {
	if s == nil {
		return
	}
	s.ResetSlot(SkillSlotLeft)
	s.ResetSlot(SkillSlotRight)
}

// ResetSlot clears the in-flight selection for one mouse slot.
func (s *SkillSelector) ResetSlot(slot SkillSlot) {
	if s == nil || slot > SkillSlotRight {
		return
	}
	s.pending[slot] = 0
	s.skillAtRequest[slot] = 0
	s.requestedAt[slot] = time.Time{}
}

// EnsureAndCast selects skillID on slot when needed and invokes cast only
// after the expected skill is already selected or a newer tick confirms it.
// A pending selection can be replaced only after its timeout.
func (s *SkillSelector) EnsureAndCast(slot SkillSlot, skillID, leftSkillID, rightSkillID uint16, now time.Time, cast func() error) (sent bool, err error) {
	if s == nil {
		return false, fmt.Errorf("skill selector not wired")
	}
	if slot > SkillSlotRight {
		return false, fmt.Errorf("skill selector requires valid slot")
	}
	if skillID == 0 {
		return false, fmt.Errorf("%s skill selector requires skill id", slot.label())
	}
	if now.IsZero() {
		now = time.Now()
	}
	selected := leftSkillID
	if slot == SkillSlotRight {
		selected = rightSkillID
	}
	pending := s.pending[slot]
	if pending != 0 && pending != skillID {
		timedOut := !s.requestedAt[slot].IsZero() && now.Sub(s.requestedAt[slot]) >= s.timeout
		if !timedOut {
			return false, nil
		}
		s.ResetSlot(slot)
		pending = 0
	}
	if skillMatchesSlot(slot, skillID, selected) {
		// A select key never authorizes a cast from the same tick. Confirmation
		// must be based on a strictly newer snapshot/tick timestamp.
		if pending == skillID && !now.After(s.requestedAt[slot]) {
			return false, nil
		}
		s.ResetSlot(slot)
		if cast == nil {
			return false, fmt.Errorf("%s skill cast callback missing", slot.label())
		}
		if castErr := cast(); castErr != nil {
			return false, castErr
		}
		return true, nil
	}
	if pending == skillID {
		unchanged := selected == s.skillAtRequest[slot]
		timedOut := !s.requestedAt[slot].IsZero() && now.Sub(s.requestedAt[slot]) >= s.timeout
		if unchanged && !timedOut {
			return false, nil
		}
		current := memory.SkillName(selected)
		wanted := memory.SkillName(skillID)
		s.ResetSlot(slot)
		return false, fmt.Errorf("%s_skill_selection_unconfirmed: want %s(%d) have %s(%d)", slot.label(), wanted, skillID, current, selected)
	}
	castBinding, err := s.bindings.Resolve(skillID)
	if err != nil {
		return false, fmt.Errorf("resolve %s(%d): %w", memory.SkillName(skillID), skillID, err)
	}
	wantButton := slot.button()
	if castBinding.CastButton != wantButton {
		return false, fmt.Errorf("%s(%d) must use %s mouse, configured=%s", memory.SkillName(skillID), skillID, slot.label(), castBinding.CastButton)
	}
	if err := s.input.SelectSkill(s.bindings, skillID); err != nil {
		s.ResetSlot(slot)
		return false, fmt.Errorf("select %s(%d): %w", memory.SkillName(skillID), skillID, err)
	}
	s.pending[slot] = skillID
	s.skillAtRequest[slot] = selected
	s.requestedAt[slot] = now
	return false, nil
}

// EnsureSelected selects skillID on slot and returns true only after the
// expected mouse-slot skill is confirmed by a fresh tick. It sends no cast.
func (s *SkillSelector) EnsureSelected(slot SkillSlot, skillID, leftSkillID, rightSkillID uint16, now time.Time) (confirmed bool, err error) {
	return s.EnsureAndCast(slot, skillID, leftSkillID, rightSkillID, now, func() error { return nil })
}

func skillMatchesSlot(slot SkillSlot, wanted, selected uint16) bool {
	if slot == SkillSlotRight {
		return rightSkillMatches(wanted, selected)
	}
	return wanted == selected
}

// RightSkillSelector preserves the existing RMB API while delegating to the
// shared slot-aware selector.
type RightSkillSelector struct {
	selector    *SkillSelector
	pending     uint16
	requestedAt time.Time
	timeout     time.Duration
}

// NewRightSkillSelector wires the existing RMB select-confirm-cast behavior.
func NewRightSkillSelector(bindings configBindingSource, in verifiedCombatInput) *RightSkillSelector {
	return &RightSkillSelector{selector: NewSkillSelector(bindings, in), timeout: rightSkillSelectionTimeout}
}

// Reset clears the in-flight RMB selection.
func (s *RightSkillSelector) Reset() {
	if s == nil || s.selector == nil {
		return
	}
	s.selector.ResetSlot(SkillSlotRight)
	s.pending = 0
	s.requestedAt = time.Time{}
}

// EnsureAndCast preserves the existing RMB contract through [SkillSelector].
func (s *RightSkillSelector) EnsureAndCast(skillID uint16, rightSkillID uint16, now time.Time, cast func() error) (sent bool, err error) {
	if s == nil || s.selector == nil {
		return false, fmt.Errorf("right skill selector not wired")
	}
	s.selector.timeout = s.timeout
	sent, err = s.selector.EnsureAndCast(SkillSlotRight, skillID, 0, rightSkillID, now, cast)
	s.pending = s.selector.pending[SkillSlotRight]
	s.requestedAt = s.selector.requestedAt[SkillSlotRight]
	return sent, err
}
