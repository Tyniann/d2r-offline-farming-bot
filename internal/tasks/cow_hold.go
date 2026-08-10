package tasks

import (
	"context"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/profile"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

const (
	cowCorpseMaxAttempts              = 2
	cowCorpseExplosionMinimumLiving   = 5
	cowCorpseExplosionMinimumCoverage = 4
	// One radius owns group membership, corpse eligibility, and CE coverage so
	// density cannot be counted under a wider contract than target selection.
	cowActiveGroupRadiusTiles      = 12.0
	cowEmergencyPreemptMarginTiles = 4.0
)

type cowHoldCombatPhase uint8

const (
	cowHoldAwaitDensity cowHoldCombatPhase = iota
	cowHoldCorpseExplosion
	cowHoldCleanup
)

// cowHoldExecutor is the Cow-only stationary strategy behind
// RouteThreatController. The shared controller still owns route Hold,
// resources, coverage, telemetry, and the no-progress watchdog.
type cowHoldExecutor struct {
	delegate CowRouteClearExecutor
	config   RouteCombatConfig
	state    world.State

	openerSent  bool
	combatPhase cowHoldCombatPhase
	cePending   uint32
	attempts    map[uint32]int
	known       map[uint32]bool

	packBoundarySet   bool
	freshPackOnly     bool
	activeGroupIDs    map[uint32]bool
	activeGroupOrigin world.Position
	anchorUnitID      uint32
	oldCorpseIDs      map[uint32]bool
	corpseSkipped     map[uint32]bool
	livingSkipped     map[uint32]bool
}

func newCowHoldExecutor(config RouteCombatConfig) cowHoldExecutor {
	return cowHoldExecutor{config: config}
}

func (e *cowHoldExecutor) bind(delegate CowRouteClearExecutor) {
	e.delegate = delegate
}

// ObserveObjectiveProgress binds the current complete snapshot and reports
// each new local corpse or confirmed CE consumption at most once per generation.
func (e *cowHoldExecutor) ObserveObjectiveProgress(state world.State) bool {
	e.state = state
	if !state.Valid || state.Phase != world.GamePhaseInGame || !state.CowCorpsesComplete || state.Generation == 0 {
		return false
	}
	current := make(map[uint32]bool)
	progressed := false
	for _, corpse := range state.CowCorpses {
		if !usableCowCorpse(state, corpse, e.config.AttackDistanceTiles) {
			continue
		}
		current[corpse.UnitID] = true
		if e.known != nil && !e.known[corpse.UnitID] {
			progressed = true
		}
	}
	for unitID := range e.known {
		corpse, found := state.FindCurrentCowCorpse(unitID)
		if !found || corpse.Consumed {
			progressed = true
		}
	}
	e.known = current
	return progressed
}

// TickRouteClear applies AD once, then prefers concrete Cow corpses for CE and
// otherwise delegates the shared hover-/projection-authorized Bone Spear.
func (e *cowHoldExecutor) TickRouteClear(ctx context.Context, request profile.RouteClearRequest, now time.Time) profile.Result {
	if e.delegate == nil {
		return profile.Result{Status: profile.StatusFailed, Reason: "cow_combat_not_wired"}
	}
	if !e.state.Valid || e.state.Phase != world.GamePhaseInGame || !e.state.CowCorpsesComplete || e.state.At != request.AssessmentAt {
		return profile.Result{Status: profile.StatusPending, Reason: profile.CorpseExplosionReasonSnapshotUnavailable}
	}
	anchor, found := e.selectActiveGroupTarget(e.livingSkipped)
	if !found {
		// Only a hold-local projection exclusion may hide otherwise valid cows.
		// Preserve the controller's bounded out-of-range recovery instead of
		// interpreting that condition as a safe terminal snapshot.
		if fallback, stillLiving := e.selectActiveGroupTarget(nil); stillLiving {
			return profile.Result{
				Status: profile.StatusPending, Reason: profile.RouteClearReasonTargetUnprojectable,
				TargetUnitID: fallback.UnitID, TargetNPCID: fallback.NPCID,
			}
		}
		return profile.Result{Status: profile.StatusPending}
	}
	activeLiving := e.activeGroupLivingCount()
	openerTarget := anchor
	if activeLiving < cowCorpseExplosionMinimumLiving {
		if standardTarget, ok := e.selectActiveGroupCowKing(e.livingSkipped); ok {
			openerTarget = standardTarget
			anchor = standardTarget
			e.anchorUnitID = standardTarget.UnitID
		}
	}
	request.Target = openerTarget
	request.Mode = profile.RouteClearThreat

	if !e.openerSent {
		result := e.delegate.TickRouteClear(ctx, request, now)
		if result.Status == profile.StatusAction && result.ActionKind == profile.RouteClearActionCurse {
			e.openerSent = true
			// Establish the old-pack boundary before the first Bone Spear can
			// create a current-pack corpse. Delaying this until CE becomes dense
			// enough can misclassify that first corpse as stale.
			e.ensurePackBoundary()
			e.advanceCombatPhase(activeLiving, false)
		}
		return withCowGroupContext(e.handleLivingProjection(result, openerTarget), anchor, activeLiving)
	}

	ceResolved := false
	if e.cePending != 0 {
		result := e.tickPendingCorpse(ctx, now)
		if result.Status != profile.StatusComplete {
			return result
		}
		ceResolved = true
	}
	e.ensurePackBoundary()
	e.advanceCombatPhase(activeLiving, ceResolved)
	if e.combatPhase != cowHoldCorpseExplosion {
		target := anchor
		if activeLiving < cowCorpseExplosionMinimumLiving {
			if king, ok := e.selectActiveGroupCowKing(e.livingSkipped); ok {
				target = king
				anchor = king
				e.anchorUnitID = king.UnitID
			}
		}
		request.Target = target
		return withCowGroupContext(e.handleLivingProjection(e.delegate.TickRouteClear(ctx, request, now), target), anchor, activeLiving)
	}
	// CE stays spatially bound to the nearest attacking group. A distant Cow
	// King or a corpse from another local group must not pull the cast away.
	request.Target = anchor
	if corpse, coverage, ok := e.selectCorpse(anchor.Position); ok && coverage >= cowCorpseExplosionMinimumCoverage {
		result := e.delegate.TickAuthorizedCorpseExplosion(ctx, e.state, corpse.UnitID, now)
		if result.Status == profile.StatusAction {
			if e.attempts == nil {
				e.attempts = make(map[uint32]int)
			}
			e.attempts[corpse.UnitID]++
			e.cePending = corpse.UnitID
		}
		if result.Status == profile.StatusComplete && result.Reason == profile.CorpseExplosionReasonTargetUnprojectable {
			e.skipCorpse(corpse.UnitID)
		}
		return withCowCorpseContext(withCowGroupContext(result, anchor, activeLiving), anchor, corpse, coverage)
	}
	// A group that shrank before producing a useful CE candidate must not
	// satisfy the earlier density latch with a one-target explosion.
	if activeLiving < cowCorpseExplosionMinimumLiving {
		e.combatPhase = cowHoldCleanup
	}
	return withCowGroupContext(e.handleLivingProjection(e.delegate.TickRouteClear(ctx, request, now), anchor), anchor, activeLiving)
}

func (e *cowHoldExecutor) selectActiveGroupTarget(skipped map[uint32]bool) (world.Monster, bool) {
	if !e.ensureActiveGroup() {
		return world.Monster{}, false
	}
	if anchor, ok := e.activeGroupMonster(e.anchorUnitID, skipped); ok {
		// Pinning prevents ordinary target churn, but a ranged character must not
		// chase a remote group member while another member is already immediate.
		if positionDistanceSquared(e.state.Player.Position, anchor.Position) > e.config.ImmediateRadiusTiles*e.config.ImmediateRadiusTiles {
			if immediate, found := e.nearestActiveGroupTarget(skipped, e.config.ImmediateRadiusTiles); found {
				e.anchorUnitID = immediate.UnitID
				return immediate, true
			}
		}
		return anchor, true
	}
	selected, found := e.nearestActiveGroupTarget(skipped, e.config.AttackDistanceTiles)
	if found {
		e.anchorUnitID = selected.UnitID
	}
	return selected, found
}

func (e *cowHoldExecutor) nearestActiveGroupTarget(skipped map[uint32]bool, radius float64) (world.Monster, bool) {
	var selected world.Monster
	var selectedDistance float64
	found := false
	for _, monster := range e.state.Monsters {
		if !e.activeGroupIDs[monster.UnitID] || skipped[monster.UnitID] || !usableCowLiving(e.state, monster, radius) {
			continue
		}
		distance := positionDistanceSquared(e.state.Player.Position, monster.Position)
		if !found || distance < selectedDistance || distance == selectedDistance && monster.UnitID < selected.UnitID {
			selected, selectedDistance, found = monster, distance, true
		}
	}
	return selected, found
}

func (e *cowHoldExecutor) ensureActiveGroup() bool {
	if e.activeGroupLivingCount() > 0 {
		e.extendActiveGroup()
		if threat, found := e.emergencyPreemptionTarget(); found {
			e.bindActiveGroup(threat)
		}
		return true
	}
	seed, found := nearestCowLivingTarget(e.state, e.config.AttackDistanceTiles, nil)
	if !found {
		e.activeGroupIDs = nil
		e.activeGroupOrigin = world.Position{}
		e.anchorUnitID = 0
		return false
	}
	e.bindActiveGroup(seed)
	return true
}

func (e *cowHoldExecutor) bindActiveGroup(seed world.Monster) {
	e.activeGroupIDs = make(map[uint32]bool)
	e.activeGroupOrigin = seed.Position
	e.extendActiveGroup()
	e.anchorUnitID = seed.UnitID
	e.livingSkipped = nil
}

func (e *cowHoldExecutor) emergencyPreemptionTarget() (world.Monster, bool) {
	current, found := e.activeGroupMonster(e.anchorUnitID, nil)
	if !found {
		return world.Monster{}, false
	}
	candidate, found := e.nearestCowOutsideActiveGroup(e.config.ImmediateRadiusTiles)
	if !found {
		return world.Monster{}, false
	}
	currentDistance := world.Distance(e.state.Player.Position, current.Position)
	candidateDistance := world.Distance(e.state.Player.Position, candidate.Position)
	// Group pinning prevents ordinary anchor churn, but it must not overrule
	// player survival. Zone precedence handles a remote anchor; the margin is
	// hysteresis when both targets are already immediate.
	if currentDistance <= e.config.ImmediateRadiusTiles &&
		candidateDistance+cowEmergencyPreemptMarginTiles > currentDistance {
		return world.Monster{}, false
	}
	return candidate, true
}

func (e *cowHoldExecutor) nearestCowOutsideActiveGroup(radius float64) (world.Monster, bool) {
	var selected world.Monster
	var selectedDistance float64
	found := false
	for _, monster := range e.state.Monsters {
		if e.activeGroupIDs[monster.UnitID] || !usableCowLiving(e.state, monster, radius) {
			continue
		}
		distance := positionDistanceSquared(e.state.Player.Position, monster.Position)
		if !found || distance < selectedDistance || distance == selectedDistance && monster.UnitID < selected.UnitID {
			selected, selectedDistance, found = monster, distance, true
		}
	}
	return selected, found
}

func (e *cowHoldExecutor) extendActiveGroup() {
	for _, monster := range e.state.Monsters {
		if usableCowLiving(e.state, monster, e.config.AttackDistanceTiles) &&
			positionDistanceSquared(e.activeGroupOrigin, monster.Position) <= cowActiveGroupRadiusTiles*cowActiveGroupRadiusTiles {
			e.activeGroupIDs[monster.UnitID] = true
		}
	}
}

func (e *cowHoldExecutor) activeGroupMonster(unitID uint32, skipped map[uint32]bool) (world.Monster, bool) {
	if unitID == 0 || !e.activeGroupIDs[unitID] || skipped[unitID] {
		return world.Monster{}, false
	}
	monster, found := e.state.FindMonsterByUnitID(unitID)
	if !found || !usableCowLiving(e.state, monster, e.config.AttackDistanceTiles) {
		return world.Monster{}, false
	}
	return monster, true
}

func (e *cowHoldExecutor) activeGroupLivingCount() int {
	count := 0
	for _, monster := range e.state.Monsters {
		if e.activeGroupIDs[monster.UnitID] && usableCowLiving(e.state, monster, e.config.AttackDistanceTiles) {
			count++
		}
	}
	return count
}

func (e *cowHoldExecutor) selectActiveGroupCowKing(skipped map[uint32]bool) (world.Monster, bool) {
	var selected world.Monster
	var selectedDistance float64
	found := false
	for _, monster := range e.state.Monsters {
		if !e.activeGroupIDs[monster.UnitID] || skipped[monster.UnitID] || !isCowKingMonster(monster) ||
			!usableCowLiving(e.state, monster, e.config.AttackDistanceTiles) {
			continue
		}
		distance := positionDistanceSquared(e.state.Player.Position, monster.Position)
		if !found || distance < selectedDistance || distance == selectedDistance && monster.UnitID < selected.UnitID {
			selected, selectedDistance, found = monster, distance, true
		}
	}
	return selected, found
}

func (e *cowHoldExecutor) advanceCombatPhase(living int, ceResolved bool) {
	switch e.combatPhase {
	case cowHoldAwaitDensity:
		if living >= cowCorpseExplosionMinimumLiving {
			e.combatPhase = cowHoldCorpseExplosion
		}
	case cowHoldCorpseExplosion:
		// Density may fall while Bone Spear is still creating the first corpse.
		// A resolved useful CE hands a shrinking pack to standard cleanup; the
		// selection path separately rejects a latched low-coverage explosion.
		if ceResolved && living < cowCorpseExplosionMinimumLiving {
			e.combatPhase = cowHoldCleanup
		}
	case cowHoldCleanup:
		// A genuinely new dense add group makes area damage worthwhile again.
		if living >= cowCorpseExplosionMinimumLiving {
			e.combatPhase = cowHoldCorpseExplosion
		}
	}
}

func (e *cowHoldExecutor) tickPendingCorpse(ctx context.Context, now time.Time) profile.Result {
	unitID := e.cePending
	result := e.delegate.TickAuthorizedCorpseExplosion(ctx, e.state, unitID, now)
	if result.Status == profile.StatusPending || result.Status == profile.StatusFailed {
		return result
	}
	if result.Status == profile.StatusAction {
		// The profile owns the pending settle and therefore cannot send another
		// CE action while this task-side binding is active.
		return profile.Result{Status: profile.StatusFailed, Reason: "cow_ce_back_to_back"}
	}
	if result.Reason == profile.CorpseExplosionReasonTargetUnprojectable {
		e.skipCorpse(unitID)
	} else if corpse, found := e.state.FindCurrentCowCorpse(unitID); !found || corpse.Consumed {
		e.skipCorpse(unitID)
	} else if e.attempts[unitID] >= cowCorpseMaxAttempts {
		e.skipCorpse(unitID)
	}
	e.cePending = 0
	return result
}

func (e *cowHoldExecutor) ensurePackBoundary() {
	if e.packBoundarySet {
		return
	}
	e.packBoundarySet = true
	_, livingDistance, livingFound := nearestCowLiving(e.state, e.config.AttackDistanceTiles)
	_, corpseDistance, corpseFound := e.nearestCorpse(false)
	if corpseFound && (!livingFound || corpseDistance <= livingDistance) {
		return
	}

	// When a living cow is nearer than every current corpse, all currently
	// visible corpses belong to the previous pack. Latch that boundary for the
	// whole Hold so later distance changes cannot revive stale CE candidates.
	e.freshPackOnly = true
	e.oldCorpseIDs = make(map[uint32]bool)
	for _, corpse := range e.state.CowCorpses {
		if usableCowCorpse(e.state, corpse, e.config.AttackDistanceTiles) {
			e.oldCorpseIDs[corpse.UnitID] = true
		}
	}
}

func (e *cowHoldExecutor) selectCorpse(anchor world.Position) (world.CowCorpse, int, bool) {
	var selected world.CowCorpse
	selectedCoverage := 0
	var selectedAnchorDistance float64
	found := false
	for _, corpse := range e.state.CowCorpses {
		if e.corpseSkipped[corpse.UnitID] || e.attempts[corpse.UnitID] >= cowCorpseMaxAttempts ||
			e.freshPackOnly && e.oldCorpseIDs[corpse.UnitID] ||
			!usableCowCorpse(e.state, corpse, e.config.AttackDistanceTiles) {
			continue
		}
		anchorDistance := positionDistanceSquared(anchor, corpse.Position)
		if anchorDistance > cowActiveGroupRadiusTiles*cowActiveGroupRadiusTiles {
			continue
		}
		coverage := e.countCowCorpseCoverage(corpse.Position)
		if !found || coverage > selectedCoverage || coverage == selectedCoverage &&
			(anchorDistance < selectedAnchorDistance || anchorDistance == selectedAnchorDistance && corpse.UnitID < selected.UnitID) {
			selected, selectedCoverage, selectedAnchorDistance, found = corpse, coverage, anchorDistance, true
		}
	}
	return selected, selectedCoverage, found
}

func withCowGroupContext(result profile.Result, anchor world.Monster, living int) profile.Result {
	result.CowGroupAnchorUnitID = anchor.UnitID
	result.CowGroupLivingCount = living
	return result
}

func withCowCorpseContext(result profile.Result, anchor world.Monster, corpse world.CowCorpse, coverage int) profile.Result {
	result.CowCorpseAnchorDistanceTiles = world.Distance(anchor.Position, corpse.Position)
	result.CowCorpseCoverageCount = coverage
	return result
}

func (e *cowHoldExecutor) nearestCorpse(applyPackBoundary bool) (world.CowCorpse, float64, bool) {
	var selected world.CowCorpse
	var selectedDistance float64
	found := false
	for _, corpse := range e.state.CowCorpses {
		if e.corpseSkipped[corpse.UnitID] || e.attempts[corpse.UnitID] >= cowCorpseMaxAttempts ||
			applyPackBoundary && e.freshPackOnly && e.oldCorpseIDs[corpse.UnitID] ||
			!usableCowCorpse(e.state, corpse, e.config.AttackDistanceTiles) {
			continue
		}
		distance := positionDistanceSquared(e.state.Player.Position, corpse.Position)
		if !found || distance < selectedDistance || distance == selectedDistance && corpse.UnitID < selected.UnitID {
			selected, selectedDistance, found = corpse, distance, true
		}
	}
	return selected, selectedDistance, found
}

func (e *cowHoldExecutor) handleLivingProjection(result profile.Result, target world.Monster) profile.Result {
	if result.Status != profile.StatusPending || result.Reason != profile.RouteClearReasonTargetUnprojectable {
		return result
	}
	if e.livingSkipped == nil {
		e.livingSkipped = make(map[uint32]bool)
	}
	e.livingSkipped[target.UnitID] = true
	if _, found := e.selectActiveGroupTarget(e.livingSkipped); found {
		// Try another local cow on the next fresh snapshot. Only exhaustion of
		// every local candidate is allowed to enter the shared range gate.
		result.Reason = ""
	}
	return result
}

func (e *cowHoldExecutor) skipCorpse(unitID uint32) {
	if e.corpseSkipped == nil {
		e.corpseSkipped = make(map[uint32]bool)
	}
	e.corpseSkipped[unitID] = true
}

// ObserveRouteClearApproachProgress makes all living cows eligible for a new
// projection attempt only after the shared controller confirms movement.
func (e *cowHoldExecutor) ObserveRouteClearApproachProgress() {
	e.livingSkipped = nil
}

// ObserveNoProgressRetarget marks the ineffective living cow skipped and pins
// another active-group member when one remains. With only one candidate left it
// clears projection skips so the later approach stage can still try that cow.
func (e *cowHoldExecutor) ObserveNoProgressRetarget(currentUnitID uint32) (world.Monster, bool) {
	if currentUnitID == 0 {
		currentUnitID = e.anchorUnitID
	}
	if currentUnitID != 0 {
		if e.livingSkipped == nil {
			e.livingSkipped = make(map[uint32]bool)
		}
		e.livingSkipped[currentUnitID] = true
	}
	if selected, found := e.selectActiveGroupTarget(e.livingSkipped); found {
		return selected, true
	}
	e.livingSkipped = nil
	if currentUnitID != 0 {
		if monster, found := e.state.FindMonsterByUnitID(currentUnitID); found &&
			usableCowLiving(e.state, monster, e.config.AttackDistanceTiles) {
			e.anchorUnitID = monster.UnitID
			return monster, true
		}
	}
	return e.selectActiveGroupTarget(nil)
}

// ResetRouteClear clears only the current Hold generation. A later Hold may
// reconsider a still-current corpse, exactly as required by the Cow contract.
func (e *cowHoldExecutor) ResetRouteClear() {
	if e.delegate != nil {
		e.delegate.ResetRouteClear()
	}
	e.state = world.State{}
	e.openerSent = false
	e.combatPhase = cowHoldAwaitDensity
	e.cePending = 0
	e.attempts = nil
	e.known = nil
	e.packBoundarySet = false
	e.freshPackOnly = false
	e.activeGroupIDs = nil
	e.activeGroupOrigin = world.Position{}
	e.anchorUnitID = 0
	e.oldCorpseIDs = nil
	e.corpseSkipped = nil
	e.livingSkipped = nil
}

func usableCowCorpse(state world.State, corpse world.CowCorpse, radius float64) bool {
	return (corpse.NPCID == world.HellBovine || corpse.NPCID == world.CowKing) &&
		corpse.UnitID != 0 && corpse.ConsumptionKnown && !corpse.Consumed &&
		corpse.ObservedAt.Equal(state.At) && corpse.SnapshotGeneration == state.Generation &&
		positionDistanceSquared(state.Player.Position, corpse.Position) <= radius*radius
}

func usableCowLiving(state world.State, monster world.Monster, radius float64) bool {
	return (monster.NPCID == world.HellBovine || monster.NPCID == world.CowKing) && monster.UnitID != 0 &&
		positionDistanceSquared(state.Player.Position, monster.Position) <= radius*radius
}

func nearestCowLiving(state world.State, radius float64) (world.Monster, float64, bool) {
	selected, found := nearestCowLivingTarget(state, radius, nil)
	if !found {
		return world.Monster{}, 0, false
	}
	return selected, positionDistanceSquared(state.Player.Position, selected.Position), true
}

func nearestCowLivingTarget(state world.State, radius float64, skipped map[uint32]bool) (world.Monster, bool) {
	var selected world.Monster
	var selectedDistance float64
	found := false
	for _, monster := range state.Monsters {
		if skipped[monster.UnitID] || monster.NPCID != world.HellBovine && monster.NPCID != world.CowKing {
			continue
		}
		distance := positionDistanceSquared(state.Player.Position, monster.Position)
		if distance > radius*radius {
			continue
		}
		if !found || distance < selectedDistance || distance == selectedDistance && monster.UnitID < selected.UnitID {
			selected, selectedDistance, found = monster, distance, true
		}
	}
	return selected, found
}

func (e *cowHoldExecutor) countCowCorpseCoverage(corpse world.Position) int {
	count := 0
	for _, monster := range e.state.Monsters {
		if e.activeGroupIDs[monster.UnitID] && usableCowLiving(e.state, monster, e.config.AttackDistanceTiles) &&
			positionDistanceSquared(corpse, monster.Position) <= cowActiveGroupRadiusTiles*cowActiveGroupRadiusTiles {
			count++
		}
	}
	return count
}

func isCowKingMonster(monster world.Monster) bool {
	return monster.NPCID == world.CowKing && monster.MonsterTypeFlag == world.SuperUniqueMonsterFlag
}
