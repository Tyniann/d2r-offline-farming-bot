package sorceressblizzard

import (
	"context"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/profile"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

// ApproachActions nutzt die vorhandene bestätigte Kampf-Teleportaktion.
type ApproachActions interface {
	TeleportToward(time.Time, world.Player, world.Position, float64) (bool, error)
}

// EncounterExecutor ergänzt den gemeinsamen Executor um die Statikfeld-Annäherung.
// Mephisto ist der einzige Aktboss der registrierten Routen. Nur dessen erster
// Encounter-Hook führt die konfigurierte Sequenz aus; der zweite bleibt leer.
// Annäherung und Casts behalten ihren Zustand über Ticks, Reset beginnt ein neues
// Spiel. Ein verlorenes Ziel oder 20 Sekunden ohne Abschluss beendet den Hook.
type EncounterExecutor struct {
	*profile.Executor
	approach         ApproachActions
	startedAt        time.Time
	teleportAt       time.Time
	teleportSnapshot time.Time
	complete         bool
}

// NewEncounterExecutor bindet die Statikfeld-Annäherung an den gemeinsamen Executor.
func NewEncounterExecutor(exec *profile.Executor, approach ApproachActions) *EncounterExecutor {
	return &EncounterExecutor{Executor: exec, approach: approach}
}

// TickHook führt Town-Prebuffs unverändert aus und begrenzt Statikfeld auf Mephisto.
func (e *EncounterExecutor) TickHook(ctx context.Context, hook profile.Hook, state world.State, target profile.EncounterTarget, now time.Time) profile.Result {
	if hook != profile.HookBossEngage {
		return e.Executor.TickHook(ctx, hook, state, target, now)
	}
	result := profile.Result{Status: profile.StatusPending, Hook: hook}
	if ctx.Err() != nil {
		result.Status, result.Reason = profile.StatusFailed, "profile_cancelled"
		return result
	}
	if !state.Valid || state.Phase != world.GamePhaseInGame || !state.Identity.Valid {
		return result
	}
	if state.Identity.Class != world.CharacterClassSorceress {
		result.Status, result.Reason = profile.StatusFailed, "profile_class_mismatch"
		return result
	}
	var boss world.Monster
	for _, monster := range state.Monsters {
		if monster.UnitID == target.UnitID {
			boss = monster
			break
		}
	}
	if boss.UnitID == 0 {
		result.Status, result.Reason = profile.StatusFailed, "profile_target_missing"
		return result
	}
	if boss.NPCID != world.Mephisto || target.ActionIndex != 0 || e.complete {
		result.Status = profile.StatusComplete
		return result
	}
	if now.IsZero() {
		now = state.At
	}
	if e.startedAt.IsZero() {
		e.startedAt = now
	}
	if now.Sub(e.startedAt) >= 20*time.Second {
		result.Status, result.Reason = profile.StatusFailed, "profile_skill_failed"
		return result
	}
	// Die kleinste Reichweite stammt aus skills.txt, Static Field: ln12,
	// Param1=5, Param2=1. Vier Tiles lassen Abstand zur Level-1-Grenze.
	// Skill-Level sind im World Model nicht verfügbar; daher kein +Skills-Raten.
	if !e.teleportAt.IsZero() && (!state.At.After(e.teleportSnapshot) || now.Sub(e.teleportAt) < 500*time.Millisecond) {
		return result
	}
	if world.Distance(state.Player.Position, boss.Position) > 4 {
		if e.approach == nil {
			result.Status, result.Reason = profile.StatusFailed, "combat_not_wired"
			return result
		}
		sent, err := e.approach.TeleportToward(now, state.Player, boss.Position, 3)
		if err != nil {
			result.Status, result.Reason = profile.StatusFailed, "combat_action_failed"
			return result
		}
		if sent {
			e.teleportAt, e.teleportSnapshot = now, state.At
			result.Status = profile.StatusAction
		}
		return result
	}
	result = e.Executor.TickHook(ctx, hook, state, target, now)
	e.complete = result.Status == profile.StatusComplete
	return result
}

// Reset löscht Annäherung, Fristen und den Zustand des gemeinsamen Executors.
func (e *EncounterExecutor) Reset() {
	e.Executor.Reset()
	e.startedAt, e.teleportAt, e.teleportSnapshot = time.Time{}, time.Time{}, time.Time{}
	e.complete = false
}
