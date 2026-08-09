package profile

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

// corpseExplosionSettle is the fastest consumption boundary directly backed
// by both Gate-20.0 live samples (0.80 and 0.90 seconds). A fresh complete
// snapshot remains mandatory, so this is not a blind input throttle.
const corpseExplosionSettle = 900 * time.Millisecond

const (
	// CorpseExplosionReasonSnapshotUnavailable means no complete current corpse
	// projection was available, so all combat input must wait.
	CorpseExplosionReasonSnapshotUnavailable = "corpse_explosion_snapshot_unavailable"
	// CorpseExplosionReasonTargetUnavailable means the authorized UnitID is not
	// exactly one usable corpse in the current complete projection.
	CorpseExplosionReasonTargetUnavailable = "corpse_explosion_target_unavailable"
	// CorpseExplosionReasonTargetUnprojectable means the current corpse has no
	// playable client projection and must be skipped by the active Cow hold.
	CorpseExplosionReasonTargetUnprojectable = "corpse_explosion_target_unprojectable"
	// CorpseExplosionReasonSettling means the cast is waiting for both the live-
	// validated settle interval and a newer complete snapshot.
	CorpseExplosionReasonSettling = "corpse_explosion_settling"
	// CorpseExplosionReasonSettled means the post-cast settle and freshness gate
	// completed without sending another input.
	CorpseExplosionReasonSettled = "corpse_explosion_settled"
)

// ConfigureCorpseExplosion binds the live-validated skill ID to the narrow cow
// task action. Timing remains an internal invariant rather than user tuning.
func (e *Executor) ConfigureCorpseExplosion(skillID uint16) error {
	if e == nil || e.definition.ID != "necro_bone_spear" || e.definition.CharacterClass != world.CharacterClassNecromancer || skillID == 0 {
		return fmt.Errorf("profile corpse explosion requires necro_bone_spear and a skill ID")
	}
	e.corpseExplosionSkillID = skillID
	return nil
}

// TickAuthorizedCorpseExplosion sends at most one positions-bound cast for the
// concrete corpse UnitID authorized by the task. The corpse must belong to the
// current complete State generation; disappearance between snapshots never
// authorizes this action.
func (e *Executor) TickAuthorizedCorpseExplosion(ctx context.Context, state world.State, authorizedUnitID uint32, now time.Time) Result {
	if ctx.Err() != nil {
		return Result{Status: StatusFailed, Reason: "profile_cancelled"}
	}
	if e == nil || e.corpseExplosionSkillID == 0 || e.actions == nil {
		return Result{Status: StatusFailed, Reason: "corpse_explosion_unavailable"}
	}
	if now.IsZero() {
		now = state.At
	}
	if pending := e.corpseExplosionPending; pending != nil {
		if now.Before(pending.readyAt) || !state.Valid || state.Phase != world.GamePhaseInGame || !state.CowCorpsesComplete ||
			state.Generation <= pending.snapshotGeneration || !state.At.After(pending.observedAt) {
			return Result{Status: StatusPending, Reason: CorpseExplosionReasonSettling, SkillID: e.corpseExplosionSkillID}
		}
		e.corpseExplosionPending = nil
		return Result{Status: StatusComplete, Reason: CorpseExplosionReasonSettled, SkillID: e.corpseExplosionSkillID}
	}
	if !state.Valid || state.Phase != world.GamePhaseInGame || !state.CowCorpsesComplete || state.At.IsZero() || state.Generation == 0 {
		return Result{Status: StatusPending, Reason: CorpseExplosionReasonSnapshotUnavailable, SkillID: e.corpseExplosionSkillID}
	}
	corpse, ok := state.FindCurrentCowCorpse(authorizedUnitID)
	if !ok || corpse.Position.X == 0 || corpse.Position.Y == 0 || !corpse.ConsumptionKnown || corpse.Consumed ||
		(corpse.NPCID != world.HellBovine && corpse.NPCID != world.CowKing) {
		return Result{Status: StatusComplete, Reason: CorpseExplosionReasonTargetUnavailable, SkillID: e.corpseExplosionSkillID}
	}
	if err := e.actions.CastSkillAtWorld(now, e.corpseExplosionSkillID, state.Player, corpse.Position); err != nil {
		if errors.Is(err, ErrSkillSelectionPending) {
			return Result{Status: StatusPending, SkillID: e.corpseExplosionSkillID, ActionKind: RouteClearActionCorpseExplosion, TargetUnitID: corpse.UnitID, TargetNPCID: corpse.NPCID}
		}
		if errors.Is(err, ErrCorpseExplosionTargetUnprojectable) {
			return Result{Status: StatusComplete, Reason: CorpseExplosionReasonTargetUnprojectable, SkillID: e.corpseExplosionSkillID, TargetUnitID: corpse.UnitID, TargetNPCID: corpse.NPCID}
		}
		e.log.Warn("Corpse Explosion fehlgeschlagen", "unit_id", corpse.UnitID, "snapshot_generation", state.Generation, "error", err)
		return Result{Status: StatusFailed, Reason: "corpse_explosion_input_failed", SkillID: e.corpseExplosionSkillID, TargetUnitID: corpse.UnitID, TargetNPCID: corpse.NPCID}
	}
	settleStartedAt := time.Now()
	if now.After(settleStartedAt) {
		settleStartedAt = now
	}
	e.corpseExplosionPending = &pendingCorpseExplosion{
		unitID: corpse.UnitID, snapshotGeneration: state.Generation,
		observedAt: state.At, readyAt: settleStartedAt.Add(corpseExplosionSettle),
	}
	e.log.Info("Corpse Explosion gesendet", "unit_id", corpse.UnitID, "snapshot_generation", state.Generation, "skill_id", e.corpseExplosionSkillID)
	return Result{Status: StatusAction, SkillID: e.corpseExplosionSkillID, ActionKind: RouteClearActionCorpseExplosion, TargetUnitID: corpse.UnitID, TargetNPCID: corpse.NPCID}
}
