package loot

import (
	"log/slog"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

const pickupStableTicks = 2

// PickupStatus is the per-tick or terminal outcome of a hover-confirmed pickup.
type PickupStatus string

// PickupStatus values returned by [PickupExecutor].
const (
	PickupPending          PickupStatus = "pending"
	PickupPickedUp         PickupStatus = "picked_up"
	PickupTargetUnstable   PickupStatus = "target_unstable"
	PickupTargetLost       PickupStatus = "target_lost"
	PickupTooFar           PickupStatus = "too_far"
	PickupHoverNotFound    PickupStatus = "hover_not_found"
	PickupProjectionFailed PickupStatus = "projection_failed"
	PickupFailed           PickupStatus = "pickup_failed"
	PickupMonsterNearby    PickupStatus = "monster_nearby"
	PickupInvalidWorld     PickupStatus = "invalid_world"
	PickupInputBlocked     PickupStatus = "input_blocked"
)

// PickupConfig tunes hover-confirmed item pickup safety limits.
type PickupConfig struct {
	MaxRetries                int
	MaxDistanceTiles          float64
	VerifyTicks               int
	VerifyTimeout             time.Duration
	MonsterAbortDistanceTiles float64
}

// PickupTarget is a frozen ground-item target selected before pickup starts.
type PickupTarget struct {
	UnitID        uint32
	TxtFileNo     uint32
	Code          string
	Name          string
	Quality       world.ItemQuality
	IdentityKind  world.ItemIdentityKind
	IdentityKey   string
	IdentityValid bool
	Pickit        PickitResult
	Position      world.Position
	AreaID        world.AreaID
}

// PickupClickTarget is the target shape passed to a hover-confirming clicker.
type PickupClickTarget struct {
	UnitID   uint32
	Position world.Position
	Name     string
}

// PickupClickStatus is the loot-local click-loop outcome used by PickupExecutor.
type PickupClickStatus string

// PickupClickStatus values are intentionally local to loot so pathing details
// remain behind an adapter.
const (
	PickupClickPending          PickupClickStatus = "pending"
	PickupClickHit              PickupClickStatus = "hit"
	PickupClickTooFar           PickupClickStatus = "too_far"
	PickupClickHoverNotFound    PickupClickStatus = "hover_not_found"
	PickupClickProjectionFailed PickupClickStatus = "projection_failed"
)

// PickupClickResult reports one click-loop tick.
type PickupClickResult struct {
	Status  PickupClickStatus
	Attempt int
	Done    bool
}

// PickupClicker confirms hover and performs the actual click for a frozen item.
type PickupClicker interface {
	Reset()
	Tick(state world.State, target PickupClickTarget, maxDistance float64) (PickupClickResult, error)
}

// PickupResult reports the executor state after one tick.
type PickupResult struct {
	Status       PickupStatus
	Done         bool
	Attempted    bool
	Target       PickupTarget
	Retry        int
	HoverAttempt int
}

type pickupPhase int

const (
	pickupPhaseStabilize pickupPhase = iota
	pickupPhaseClick
	pickupPhaseVerify
	pickupPhaseDone
)

type verifyFinding string

const (
	verifyFindingNone         verifyFinding = ""
	verifyFindingGroundAbsent verifyFinding = "ground_absent"
	verifyFindingInventory    verifyFinding = "inventory"
)

// PickupExecutor advances one frozen item pickup through stabilize, click, and verify phases.
type PickupExecutor struct {
	log     *slog.Logger
	cfg     PickupConfig
	clicker PickupClicker
	target  PickupTarget

	phase          pickupPhase
	stableTicks    int
	retriesStarted int
	verifyDeadline time.Time
	verifyFinding  verifyFinding
	verifyTicks    int
	last           PickupResult
}

// NewPickupExecutor creates a hover-confirmed pickup executor for one frozen target.
func NewPickupExecutor(log *slog.Logger, cfg PickupConfig, clicker PickupClicker, target PickupTarget) *PickupExecutor {
	if log == nil {
		log = slog.Default()
	}
	return &PickupExecutor{
		log:     log.With("component", "loot.pickup"),
		cfg:     cfg,
		clicker: clicker,
		target:  target,
		phase:   pickupPhaseStabilize,
	}
}

// Tick advances the pickup executor by one world snapshot.
func (e *PickupExecutor) Tick(state world.State, now time.Time) PickupResult {
	if e == nil {
		return PickupResult{Status: PickupInvalidWorld, Done: true}
	}
	if e.phase == pickupPhaseDone {
		return e.last
	}
	if !isValidInGame(state) {
		if e.phase == pickupPhaseVerify && !e.verifyDeadline.IsZero() && now.Before(e.verifyDeadline) {
			return e.pending(0)
		}
		return e.finish(PickupInvalidWorld, 0)
	}

	switch e.phase {
	case pickupPhaseStabilize:
		return e.tickStabilize(state, now)
	case pickupPhaseClick:
		return e.tickClick(state, now)
	case pickupPhaseVerify:
		return e.tickVerify(state, now)
	default:
		return e.finish(PickupInvalidWorld, 0)
	}
}

func (e *PickupExecutor) tickStabilize(state world.State, now time.Time) PickupResult {
	item, found, unstable := e.findTargetGround(state)
	if unstable {
		e.stableTicks = 0
		return e.finish(PickupTargetUnstable, 0)
	}
	if !found {
		e.stableTicks = 0
		return e.finish(PickupTargetLost, 0)
	}
	if status, ok := e.preClickAbort(state, item); ok {
		return e.finish(status, 0)
	}

	e.stableTicks++
	if e.stableTicks < pickupStableTicks {
		return e.pending(0)
	}

	if e.retriesStarted >= e.cfg.MaxRetries {
		return e.finish(PickupFailed, 0)
	}
	e.retriesStarted++
	e.phase = pickupPhaseClick
	e.clicker.Reset()
	e.log.Info("loot pickup started",
		"unit_id", e.target.UnitID,
		"name", e.target.Name,
		"code", e.target.Code,
		"retry", e.retriesStarted,
		"distance", world.Distance(state.Player.Position, item.Position),
	)
	return e.tickClick(state, now)
}

func (e *PickupExecutor) tickClick(state world.State, now time.Time) PickupResult {
	item, found, unstable := e.findTargetGround(state)
	if unstable {
		return e.finish(PickupTargetUnstable, 0)
	}
	if !found {
		return e.finish(PickupTargetLost, 0)
	}
	if status, ok := e.preClickAbort(state, item); ok {
		return e.finish(status, 0)
	}

	res, err := e.clicker.Tick(state, PickupClickTarget{
		UnitID:   e.target.UnitID,
		Position: e.target.Position,
		Name:     e.target.Name,
	}, e.cfg.MaxDistanceTiles)
	if err != nil {
		return e.finish(PickupInputBlocked, res.Attempt)
	}
	if !res.Done {
		return e.pending(res.Attempt)
	}

	switch res.Status {
	case PickupClickHit:
		e.phase = pickupPhaseVerify
		e.verifyDeadline = now.Add(e.cfg.VerifyTimeout)
		e.verifyFinding = verifyFindingNone
		e.verifyTicks = 0
		e.log.Info("loot pickup click confirmed",
			"unit_id", e.target.UnitID,
			"name", e.target.Name,
			"code", e.target.Code,
			"retry", e.retriesStarted,
			"hover_attempts", res.Attempt,
		)
		result := e.pending(res.Attempt)
		result.Attempted = true
		return result
	case PickupClickTooFar:
		return e.finish(PickupTooFar, res.Attempt)
	case PickupClickHoverNotFound:
		return e.retryOrFinish(PickupHoverNotFound, res.Attempt)
	case PickupClickProjectionFailed:
		return e.finish(PickupProjectionFailed, res.Attempt)
	default:
		return e.finish(PickupFailed, res.Attempt)
	}
}

func (e *PickupExecutor) tickVerify(state world.State, now time.Time) PickupResult {
	finding := e.terminalFinding(state)
	if finding != verifyFindingNone {
		if finding != e.verifyFinding {
			e.verifyFinding = finding
			e.verifyTicks = 0
		}
		e.verifyTicks++
		if e.verifyTicks >= e.cfg.VerifyTicks {
			e.log.Info("loot pickup complete",
				"unit_id", e.target.UnitID,
				"name", e.target.Name,
				"code", e.target.Code,
				"finding", string(finding),
				"retry", e.retriesStarted,
			)
			return e.finish(PickupPickedUp, 0)
		}
	} else {
		e.verifyFinding = verifyFindingNone
		e.verifyTicks = 0
	}

	if !e.verifyDeadline.IsZero() && !now.Before(e.verifyDeadline) {
		return e.retryOrFinish(PickupFailed, 0)
	}
	return e.pending(0)
}

func (e *PickupExecutor) terminalFinding(state world.State) verifyFinding {
	if state.Area.ID != e.target.AreaID {
		return verifyFindingNone
	}
	seenSameUnit := false
	for _, item := range state.Items {
		if item.UnitID != e.target.UnitID {
			continue
		}
		seenSameUnit = true
		if item.TxtFileNo == e.target.TxtFileNo &&
			item.Location == world.ItemLocationInventory &&
			item.PlayerOwned &&
			item.Page == 0 {
			return verifyFindingInventory
		}
		if item.TxtFileNo == e.target.TxtFileNo &&
			item.Code == e.target.Code &&
			item.Location == world.ItemLocationGround &&
			item.Position == e.target.Position {
			return verifyFindingNone
		}
	}
	if seenSameUnit {
		return verifyFindingNone
	}
	return verifyFindingGroundAbsent
}

func (e *PickupExecutor) preClickAbort(state world.State, item world.Item) (PickupStatus, bool) {
	if world.Distance(state.Player.Position, item.Position) > e.cfg.MaxDistanceTiles {
		return PickupTooFar, true
	}
	if monsterNearby(state, e.cfg.MonsterAbortDistanceTiles) {
		return PickupMonsterNearby, true
	}
	return PickupPending, false
}

func (e *PickupExecutor) retryOrFinish(status PickupStatus, attempt int) PickupResult {
	e.clicker.Reset()
	if e.retriesStarted >= e.cfg.MaxRetries {
		return e.finish(status, attempt)
	}
	e.phase = pickupPhaseStabilize
	e.stableTicks = 0
	e.verifyDeadline = time.Time{}
	e.verifyFinding = verifyFindingNone
	e.verifyTicks = 0
	return e.pending(attempt)
}

func (e *PickupExecutor) finish(status PickupStatus, attempt int) PickupResult {
	e.phase = pickupPhaseDone
	e.last = PickupResult{
		Status:       status,
		Done:         true,
		Target:       e.target,
		Retry:        e.retriesStarted,
		HoverAttempt: attempt,
	}
	if status != PickupPickedUp {
		e.log.Warn("loot pickup failed",
			"unit_id", e.target.UnitID,
			"name", e.target.Name,
			"code", e.target.Code,
			"reason", string(status),
			"retry", e.retriesStarted,
			"hover_attempts", attempt,
		)
	}
	return e.last
}

func (e *PickupExecutor) pending(attempt int) PickupResult {
	return PickupResult{
		Status:       PickupPending,
		Target:       e.target,
		Retry:        e.retriesStarted,
		HoverAttempt: attempt,
	}
}

func (e *PickupExecutor) findTargetGround(state world.State) (world.Item, bool, bool) {
	if state.Area.ID != e.target.AreaID {
		return world.Item{}, false, false
	}
	for _, item := range state.Items {
		if item.UnitID != e.target.UnitID {
			continue
		}
		if item.TxtFileNo != e.target.TxtFileNo ||
			item.Code != e.target.Code ||
			item.Location != world.ItemLocationGround ||
			item.Position != e.target.Position {
			return item, false, true
		}
		return item, true, false
	}
	return world.Item{}, false, false
}

func isValidInGame(state world.State) bool {
	return state.Valid && state.Phase == world.GamePhaseInGame
}

func monsterNearby(state world.State, maxDistance float64) bool {
	if maxDistance <= 0 {
		return false
	}
	for _, monster := range state.Monsters {
		if world.Distance(state.Player.Position, monster.Position) <= maxDistance {
			return true
		}
	}
	return false
}

// SelectPickupCandidate chooses the nearest pickup-capable ground item from a decision report.
func SelectPickupCandidate(state world.State, report DecisionReport) (PickupTarget, bool) {
	return SelectPickupCandidateExcluding(state, report, nil)
}

// SelectPickupCandidateExcluding chooses the nearest pickup-capable ground item
// while ignoring unit IDs already skipped in the current pickup phase.
func SelectPickupCandidateExcluding(state world.State, report DecisionReport, skipped map[uint32]bool) (PickupTarget, bool) {
	if !isValidInGame(state) {
		return PickupTarget{}, false
	}
	var best PickupTarget
	var bestDist float64
	found := false
	for _, decision := range report.Decisions {
		if decision.Stage != DecisionStagePickCandidate ||
			decision.Kind != DecisionKindPickCandidate ||
			!decision.CanFit {
			continue
		}
		if skipped[decision.UnitID] {
			continue
		}
		item, ok := findGroundItemByUnitID(state, decision.UnitID)
		if !ok {
			continue
		}
		dist := world.Distance(state.Player.Position, item.Position)
		if !found || dist < bestDist || (dist == bestDist && item.UnitID < best.UnitID) {
			found = true
			bestDist = dist
			best = PickupTarget{
				UnitID:    item.UnitID,
				TxtFileNo: item.TxtFileNo,
				Code:      item.Code,
				Name:      item.Name,
				Quality:   item.Quality, IdentityKind: item.IdentityKind, IdentityKey: item.IdentityKey, IdentityValid: item.IdentityValid,
				Pickit:   decision.Pickit,
				Position: item.Position,
				AreaID:   state.Area.ID,
			}
		}
	}
	return best, found
}

func findGroundItemByUnitID(state world.State, unitID uint32) (world.Item, bool) {
	for _, item := range state.GroundItems() {
		if item.UnitID == unitID {
			return item, true
		}
	}
	return world.Item{}, false
}
