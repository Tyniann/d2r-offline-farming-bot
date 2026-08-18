package app

import (
	"fmt"
	"strings"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/input"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/memory"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

const (
	hammerdinPrebuffCTA     = "hammerdin-prebuff:cta"
	hammerdinPrebuffNoCTA   = "hammerdin-prebuff:no-cta"
	weaponSetSwitchKey      = "w"
	weaponSetTimeout        = 1500 * time.Millisecond
	prebuffCastSettle       = 300 * time.Millisecond
	prebuffWeaponSwapSettle = 1000 * time.Millisecond
	// prebuffRecastSwapSettle waits after a combat CTA recast before W.
	// D2R ignores W during the Paladin cast animation; 126 ms after releasing
	// Blessed Hammer was not enough, but a full 1000 ms leaves a Hell pack
	// unattended.
	prebuffRecastSwapSettle = 250 * time.Millisecond
	ctaRecastAfter          = 150 * time.Second
)

const (
	reasonCTABindingsIncomplete     = "hammerdin_cta_bindings_incomplete"
	reasonWeaponSetUnavailable      = "weapon_set_read_unavailable"
	reasonWeaponSetUnconfirmed      = "weapon_set_switch_unconfirmed"
	reasonCTASkillUnconfirmed       = "hammerdin_cta_skill_unconfirmed"
	reasonHolyShieldUnconfirmed     = "hammerdin_holy_shield_unconfirmed"
	reasonPrimaryRestoreFailed      = "hammerdin_primary_set_restore_failed"
	reasonBlessedHammerUnconfirmed  = "hammerdin_blessed_hammer_unconfirmed"
	reasonConcentrationUnconfirmed  = "hammerdin_concentration_unconfirmed"
	reasonPrebuffRequiresField      = "hammerdin_prebuff_requires_field"
	hammerdinCTAPairRequiredMessage = "Für Call to Arms müssen Battle Command und Battle Orders beide belegt sein."
)

type weaponSetInput interface {
	PressKey(string) error
}

// weaponSetSelector sends at most one swap key for a target set. A newer
// snapshot must confirm the target; unavailable reads never imply Primary.
type weaponSetSelector struct {
	input               weaponSetInput
	pending             bool
	target              world.WeaponSet
	requestedGeneration uint64
	requestedAt         time.Time
	timeout             time.Duration
}

func newWeaponSetSelector(in weaponSetInput) *weaponSetSelector {
	return &weaponSetSelector{input: in, timeout: weaponSetTimeout}
}

func (s *weaponSetSelector) reset() {
	s.pending = false
	s.target = world.WeaponSetPrimary
	s.requestedGeneration = 0
	s.requestedAt = time.Time{}
}

func (s *weaponSetSelector) ensure(target world.WeaponSet, current world.WeaponSetState, generation uint64, now time.Time) (confirmed, action bool, err error) {
	if s == nil || s.input == nil {
		return false, false, fmt.Errorf("%s: selector not wired", reasonWeaponSetUnavailable)
	}
	if !current.Available {
		s.reset()
		return false, false, fmt.Errorf("%s", reasonWeaponSetUnavailable)
	}
	if s.pending {
		if target != s.target {
			return false, false, fmt.Errorf("%s: pending target changed", reasonWeaponSetUnconfirmed)
		}
		if generation <= s.requestedGeneration {
			return false, false, nil
		}
		if current.Set == target {
			s.reset()
			return true, false, nil
		}
		if now.Sub(s.requestedAt) < s.timeout {
			return false, false, nil
		}
		s.reset()
		return false, false, fmt.Errorf("%s: target=%s current=%s", reasonWeaponSetUnconfirmed, target, current.Set)
	}
	if current.Set == target {
		return true, false, nil
	}
	if err := s.input.PressKey(weaponSetSwitchKey); err != nil {
		return false, false, fmt.Errorf("%s: send W: %w", reasonWeaponSetUnconfirmed, err)
	}
	s.pending = true
	s.target = target
	s.requestedGeneration = generation
	s.requestedAt = now
	return false, true, nil
}

// timedCastAnchor becomes due only after a successfully authorized cast and
// is reset at game or generation boundaries by its owner.
type timedCastAnchor struct {
	last time.Time
}

func (a *timedCastAnchor) mark(at time.Time) { a.last = at }
func (a *timedCastAnchor) reset()            { a.last = time.Time{} }
func (a timedCastAnchor) due(now time.Time) bool {
	return a.last.IsZero() || now.Sub(a.last) >= ctaRecastAfter
}

type hammerdinPrebuffInput interface {
	verifiedCombatInput
	weaponSetInput
	Focus() error
	MoveTo(int, int) error
	Window() (input.WindowInfo, bool)
}

type hammerdinPrebuffState uint8

const (
	prebuffConfirmPrimary hammerdinPrebuffState = iota
	prebuffSwitchSecondary
	prebuffBattleCommandFirst
	prebuffBattleOrders
	prebuffBattleCommandSecond
	prebuffHolyShield
	prebuffRestorePrimary
	prebuffRestoreHammer
	prebuffRestoreConcentration
	prebuffComplete
)

type hammerdinPrebuffResult struct {
	Done   bool
	Action string
}

// hammerdinPrebuff owns the complete BC -> BO -> BC -> Holy Shield sequence.
// Each Tick consumes one fresh World snapshot and sends at most one semantic
// input action. Productive consumers may call reset at every game boundary.
type hammerdinPrebuff struct {
	input       hammerdinPrebuffInput
	bindings    configBindingSource
	selector    *SkillSelector
	weapon      *weaponSetSelector
	cta         bool
	state       hammerdinPrebuffState
	settleUntil time.Time
	ctaAnchor   timedCastAnchor
}

func newHammerdinPrebuff(bindings configBindingSource, in hammerdinPrebuffInput, expectCTA bool) (*hammerdinPrebuff, error) {
	prebuff, err := newInferredHammerdinPrebuff(bindings, in)
	if err != nil {
		return nil, err
	}
	if prebuff.cta != expectCTA {
		return nil, fmt.Errorf("town test %s requires CTA bindings=%t", map[bool]string{true: "cta", false: "no-cta"}[expectCTA], expectCTA)
	}
	return prebuff, nil
}

func newInferredHammerdinPrebuff(bindings configBindingSource, in hammerdinPrebuffInput) (*hammerdinPrebuff, error) {
	if in == nil {
		return nil, fmt.Errorf("hammerdin prebuff input not wired")
	}
	cta, err := hammerdinCTAEnabled(bindings)
	if err != nil {
		return nil, err
	}
	return &hammerdinPrebuff{
		input: in, bindings: bindings, selector: NewSkillSelector(bindings, in),
		weapon: newWeaponSetSelector(in), cta: cta,
	}, nil
}

func hammerdinCTAEnabled(bindings configBindingSource) (bool, error) {
	_, bcErr := bindings.Resolve(memory.MustSkillID("battle_command"))
	_, boErr := bindings.Resolve(memory.MustSkillID("battle_orders"))
	hasBC, hasBO := bcErr == nil, boErr == nil
	if hasBC != hasBO {
		return false, fmt.Errorf("%s: %s", reasonCTABindingsIncomplete, hammerdinCTAPairRequiredMessage)
	}
	return hasBC && hasBO, nil
}

func (p *hammerdinPrebuff) reset() {
	if p == nil {
		return
	}
	p.restartSequence()
	p.ctaAnchor.reset()
}

func (p *hammerdinPrebuff) restartSequence() {
	if p == nil {
		return
	}
	p.selector.Reset()
	p.weapon.reset()
	p.state = prebuffConfirmPrimary
	p.settleUntil = time.Time{}
}

func (p *hammerdinPrebuff) tick(state world.State, now time.Time) (hammerdinPrebuffResult, error) {
	if p == nil {
		return hammerdinPrebuffResult{}, fmt.Errorf("hammerdin prebuff not wired")
	}
	if state.Phase == world.GamePhaseMenu || state.Phase == world.GamePhaseLoading {
		p.reset()
		return hammerdinPrebuffResult{}, nil
	}
	if state.Valid && state.Phase == world.GamePhaseInGame && state.Area.IsTown() {
		return hammerdinPrebuffResult{}, fmt.Errorf("%s", reasonPrebuffRequiresField)
	}
	if !state.Valid || state.Phase != world.GamePhaseInGame {
		return hammerdinPrebuffResult{}, fmt.Errorf("hammerdin prebuff requires a fresh combat-area snapshot")
	}
	if now.IsZero() {
		now = state.At
	}
	if now.IsZero() {
		now = time.Now()
	}
	if !p.settleUntil.IsZero() && now.Before(p.settleUntil) {
		return hammerdinPrebuffResult{}, nil
	}
	p.settleUntil = time.Time{}
	if p.state == prebuffComplete {
		if !p.cta || !p.ctaAnchor.due(now) {
			return hammerdinPrebuffResult{Done: true}, nil
		}
		p.restartSequence()
		p.settleUntil = now.Add(prebuffRecastSwapSettle)
		return hammerdinPrebuffResult{}, nil
	}

	switch p.state {
	case prebuffConfirmPrimary:
		if !state.Player.ActiveWeaponSet.Available {
			return hammerdinPrebuffResult{}, fmt.Errorf("%s", reasonWeaponSetUnavailable)
		}
		if state.Player.ActiveWeaponSet.Set != world.WeaponSetPrimary {
			return hammerdinPrebuffResult{}, fmt.Errorf("%s: prebuff must start on primary", reasonWeaponSetUnconfirmed)
		}
		if p.cta {
			p.state = prebuffSwitchSecondary
		} else {
			p.state = prebuffHolyShield
		}
		return hammerdinPrebuffResult{}, nil
	case prebuffSwitchSecondary:
		confirmed, action, err := p.weapon.ensure(world.WeaponSetSecondary, state.Player.ActiveWeaponSet, state.Generation, now)
		if err != nil {
			return hammerdinPrebuffResult{}, err
		}
		if confirmed {
			p.state = prebuffBattleCommandFirst
		}
		return hammerdinPrebuffResult{Action: actionName(action, "weapon_set_secondary")}, nil
	case prebuffBattleCommandFirst:
		return p.castRight(state, now, memory.MustSkillID("battle_command"), prebuffBattleOrders, reasonCTASkillUnconfirmed, false)
	case prebuffBattleOrders:
		return p.castRight(state, now, memory.MustSkillID("battle_orders"), prebuffBattleCommandSecond, reasonCTASkillUnconfirmed, false)
	case prebuffBattleCommandSecond:
		return p.castRight(state, now, memory.MustSkillID("battle_command"), prebuffHolyShield, reasonCTASkillUnconfirmed, true)
	case prebuffHolyShield:
		expected := world.WeaponSetPrimary
		if p.cta {
			expected = world.WeaponSetSecondary
		}
		if err := requireWeaponSet(state.Player.ActiveWeaponSet, expected, reasonHolyShieldUnconfirmed); err != nil {
			return hammerdinPrebuffResult{}, err
		}
		next := prebuffRestoreHammer
		if p.cta {
			next = prebuffRestorePrimary
		}
		return p.castRight(state, now, memory.MustSkillID("holy_shield"), next, reasonHolyShieldUnconfirmed, false)
	case prebuffRestorePrimary:
		confirmed, action, err := p.weapon.ensure(world.WeaponSetPrimary, state.Player.ActiveWeaponSet, state.Generation, now)
		if err != nil {
			return hammerdinPrebuffResult{}, fmt.Errorf("%s: %v", reasonPrimaryRestoreFailed, err)
		}
		if confirmed {
			p.state = prebuffRestoreHammer
		}
		return hammerdinPrebuffResult{Action: actionName(action, "weapon_set_primary")}, nil
	case prebuffRestoreHammer:
		confirmed, err := p.selector.EnsureSelected(SkillSlotLeft, memory.MustSkillID("blessed_hammer"), state.Player.LeftSkillID, state.Player.RightSkillID, now)
		if err != nil {
			return hammerdinPrebuffResult{}, fmt.Errorf("%s: %w", reasonBlessedHammerUnconfirmed, err)
		}
		if confirmed {
			p.state = prebuffRestoreConcentration
		}
		return hammerdinPrebuffResult{Action: selectionAction(p.selector, SkillSlotLeft)}, nil
	case prebuffRestoreConcentration:
		confirmed, err := p.selector.EnsureSelected(SkillSlotRight, memory.MustSkillID("concentration"), state.Player.LeftSkillID, state.Player.RightSkillID, now)
		if err != nil {
			return hammerdinPrebuffResult{}, fmt.Errorf("%s: %w", reasonConcentrationUnconfirmed, err)
		}
		if confirmed {
			p.state = prebuffComplete
			return hammerdinPrebuffResult{Done: true}, nil
		}
		return hammerdinPrebuffResult{Action: selectionAction(p.selector, SkillSlotRight)}, nil
	case prebuffComplete:
		return hammerdinPrebuffResult{Done: true}, nil
	default:
		return hammerdinPrebuffResult{}, fmt.Errorf("hammerdin prebuff invalid state")
	}
}

func (p *hammerdinPrebuff) castRight(state world.State, now time.Time, skillID uint16, next hammerdinPrebuffState, reason string, anchor bool) (hammerdinPrebuffResult, error) {
	expected := world.WeaponSetPrimary
	if p.cta {
		expected = world.WeaponSetSecondary
	}
	if err := requireWeaponSet(state.Player.ActiveWeaponSet, expected, reason); err != nil {
		return hammerdinPrebuffResult{}, err
	}
	pendingBefore := p.selector.pending[SkillSlotRight]
	sent, err := p.selector.EnsureAndCast(SkillSlotRight, skillID, state.Player.LeftSkillID, state.Player.RightSkillID, now, p.selfCast)
	if err != nil {
		return hammerdinPrebuffResult{}, fmt.Errorf("%s: %w", reason, err)
	}
	if sent {
		p.state = next
		p.settleUntil = now.Add(castSettleFor(next))
		if anchor {
			p.ctaAnchor.mark(now)
		}
		return hammerdinPrebuffResult{Action: "cast_" + memory.SkillName(skillID)}, nil
	}
	if pendingBefore == 0 && p.selector.pending[SkillSlotRight] == skillID {
		return hammerdinPrebuffResult{Action: "select_" + memory.SkillName(skillID)}, nil
	}
	return hammerdinPrebuffResult{}, nil
}

func (p *hammerdinPrebuff) selfCast() error {
	if err := p.input.Focus(); err != nil {
		return fmt.Errorf("self cast focus: %w", err)
	}
	window, ok := p.input.Window()
	if !ok {
		return fmt.Errorf("self cast: window not bound")
	}
	if err := p.input.MoveTo(window.ClientWidth/2, window.ClientHeight/2); err != nil {
		return fmt.Errorf("self cast aim: %w", err)
	}
	if err := p.input.Click(input.MouseRight); err != nil {
		return fmt.Errorf("self cast click: %w", err)
	}
	return nil
}

func castSettleFor(next hammerdinPrebuffState) time.Duration {
	if next == prebuffRestorePrimary {
		// D2R ignores W during the Paladin cast animation. Live CTA restore
		// needs 1000 ms after Holy Shield before the one-shot swap.
		return prebuffWeaponSwapSettle
	}
	return prebuffCastSettle
}

func requireWeaponSet(current world.WeaponSetState, expected world.WeaponSet, reason string) error {
	if !current.Available {
		return fmt.Errorf("%s: %s", reasonWeaponSetUnavailable, reason)
	}
	if current.Set != expected {
		return fmt.Errorf("%s: expected=%s current=%s", reason, expected, current.Set)
	}
	return nil
}

func actionName(action bool, name string) string {
	if action {
		return name
	}
	return ""
}

func selectionAction(selector *SkillSelector, slot SkillSlot) string {
	if selector != nil && selector.pending[slot] != 0 {
		return "select_" + strings.ReplaceAll(memory.SkillName(selector.pending[slot]), " ", "_")
	}
	return ""
}

func isHammerdinPrebuffTownTest(spec string) bool {
	switch strings.ToLower(strings.TrimSpace(spec)) {
	case hammerdinPrebuffCTA, hammerdinPrebuffNoCTA:
		return true
	default:
		return false
	}
}
