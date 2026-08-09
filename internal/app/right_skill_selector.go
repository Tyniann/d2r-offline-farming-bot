package app

import (
	"fmt"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/input"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/memory"
)

const rightSkillSelectionTimeout = 1500 * time.Millisecond

// RightSkillSelector confirms RightSkillID before any productive RMB cast.
// It never clicks in the same tick as an F-key selection.
type RightSkillSelector struct {
	bindings       configBindingSource
	input          verifiedCombatInput
	pending        uint16
	rightAtRequest uint16
	requestedAt    time.Time
	timeout        time.Duration
}

// NewRightSkillSelector wires shared RMB select-confirm-cast behavior.
func NewRightSkillSelector(bindings configBindingSource, in verifiedCombatInput) *RightSkillSelector {
	return &RightSkillSelector{bindings: bindings, input: in, timeout: rightSkillSelectionTimeout}
}

// Reset clears any in-flight skill selection, e.g. after game change or an
// independently authorized cast from another owner.
func (s *RightSkillSelector) Reset() {
	if s == nil {
		return
	}
	s.pending = 0
	s.rightAtRequest = 0
	s.requestedAt = time.Time{}
}

// EnsureAndCast selects skillID onto RMB when needed and only then runs cast.
// now is the authoritative tick/snapshot time used for confirmation freshness.
// cast must perform Move/Click only; it must not press another skill key.
func (s *RightSkillSelector) EnsureAndCast(skillID uint16, rightSkillID uint16, now time.Time, cast func() error) (sent bool, err error) {
	if s == nil {
		return false, fmt.Errorf("right skill selector not wired")
	}
	if skillID == 0 {
		return false, fmt.Errorf("right skill selector requires skill id")
	}
	if now.IsZero() {
		now = time.Now()
	}
	if rightSkillMatches(skillID, rightSkillID) {
		s.Reset()
		if cast == nil {
			return false, fmt.Errorf("right skill cast callback missing")
		}
		if castErr := cast(); castErr != nil {
			return false, castErr
		}
		return true, nil
	}
	if s.pending == skillID {
		unchanged := rightSkillID == s.rightAtRequest
		timedOut := !s.requestedAt.IsZero() && now.Sub(s.requestedAt) >= s.timeout
		if unchanged && !timedOut {
			return false, nil
		}
		current := memory.SkillName(rightSkillID)
		wanted := memory.SkillName(skillID)
		s.Reset()
		return false, fmt.Errorf("right_skill_selection_unconfirmed: want %s(%d) have %s(%d)", wanted, skillID, current, rightSkillID)
	}
	castBinding, err := s.bindings.Resolve(skillID)
	if err != nil {
		return false, fmt.Errorf("resolve %s(%d): %w", memory.SkillName(skillID), skillID, err)
	}
	if castBinding.CastButton != input.MouseRight {
		return false, fmt.Errorf("%s(%d) must use right mouse, configured=%s", memory.SkillName(skillID), skillID, castBinding.CastButton)
	}
	if err := s.input.SelectSkill(s.bindings, skillID); err != nil {
		s.Reset()
		return false, fmt.Errorf("select %s(%d): %w", memory.SkillName(skillID), skillID, err)
	}
	s.pending = skillID
	s.rightAtRequest = rightSkillID
	s.requestedAt = now
	return false, nil
}
