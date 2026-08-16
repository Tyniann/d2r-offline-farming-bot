package app

import (
	"context"
	"fmt"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/tasks"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/town"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

type runReadinessError struct{ reason string }

func (e *runReadinessError) Error() string { return e.reason }

func failRunReadiness(reason string) error { return &runReadinessError{reason: reason} }

// consumeRunReadiness runs once per queue runtime before the first productive
// task tick. It owns Merc recovery and the Cow-only Town reserve without
// duplicating either Town service implementation.
func (rt *Runtime) consumeRunReadiness(ctx context.Context, state world.State) (bool, error) {
	if rt == nil || !rt.runReadinessPending || !rt.productiveRunActive {
		return true, nil
	}
	// Offline character-select and loading share runTick with productive runs.
	// Merc memory is not authoritative until a confirmed in-game identity exists.
	if rt.Options.OfflineDifficulty != "" || rt.Options.OfflineExitTest {
		return true, nil
	}
	if !state.Valid || state.Phase != world.GamePhaseInGame {
		return false, nil
	}
	if !state.Identity.Valid {
		return false, nil
	}
	if state.Area.ID != world.RogueEncampment {
		return false, nil
	}
	runID := rt.Config.Session.Run
	if runID == "" {
		rt.runReadinessPending = false
		return true, nil
	}
	if _, ok := rt.Config.Runs.Run(runID); !ok {
		return false, fmt.Errorf("mercenary preflight: run %q unavailable", runID)
	}
	profileID, err := rt.resolvedCombatProfileID()
	if err != nil {
		return false, fmt.Errorf("mercenary preflight: %w", err)
	}
	profileCfg, ok := rt.Config.Profiles[profileID]
	if !ok {
		return false, fmt.Errorf("mercenary preflight: profile %q unavailable", profileID)
	}
	enabled, rule := profileCfg.Resources.Mercenary.Resolve()
	// Build-required availability is independent from the optional in-run
	// potion policy. The Hammerdin may disable Merc potions, never the Merc gate.
	enabled = enabled || profileCfg.RequiresMercenary
	reason := town.EvaluateMercenaryPreflight(town.MercenaryPolicy{
		Enabled: enabled, ThresholdPercent: rule.UseBelowPercent,
	}, state.Mercenary)
	needsTownPreparation := reason == town.ReasonMercenaryDeadAtStart || runID == string(tasks.RunIDCows)
	if reason != "" && reason != town.ReasonMercenaryDeadAtStart {
		rt.runReadinessPending = false
		return false, failRunReadiness(string(reason))
	}
	if needsTownPreparation {
		if rt.taskDeps.Town == nil {
			return false, fmt.Errorf("run readiness: town preparation is not wired")
		}
		result := rt.taskDeps.Town.Tick(ctx, state)
		if !result.Done {
			return false, nil
		}
		rt.taskDeps.Town.Reset()
		if result.Status != "complete" {
			return false, failRunReadiness(result.Reason)
		}
		rt.runReadinessPending = false
		return true, nil
	}
	rt.runReadinessPending = false
	return true, nil
}

// isTerminalMercenaryFailure reports queue-terminal Merc reasons that must never
// enter a controlled retry or Save-and-Exit recovery path.
func isTerminalMercenaryFailure(reason string) bool {
	switch town.Reason(reason) {
	case town.ReasonMercenaryNotHired,
		town.ReasonMercenaryStateInvalid,
		town.ReasonMercenaryReviveInsufficientGold,
		town.ReasonMercenaryReviveVerifyTimeout,
		town.ReasonMercenaryHealVerifyTimeout,
		town.ReasonMercenaryHealStateInvalid,
		town.ReasonMercenaryReviveStateInvalid:
		return true
	default:
		return false
	}
}
