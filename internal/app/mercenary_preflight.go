package app

import (
	"fmt"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/town"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

// consumeMercenaryPreflight runs once per queue runtime before the first productive
// tick. Dead or missing Mercs fail closed here; injured living Mercs may start.
func (rt *Runtime) consumeMercenaryPreflight(state world.State) error {
	if rt == nil || !rt.mercPreflightPending {
		return nil
	}
	// Offline character-select and loading share runTick with productive runs.
	// Merc memory is not authoritative until a confirmed in-game identity exists.
	if rt.Options.OfflineDifficulty != "" || rt.Options.OfflineExitTest {
		return nil
	}
	if !state.Valid || state.Phase != world.GamePhaseInGame {
		return nil
	}
	if !state.Identity.Valid {
		return nil
	}
	if state.Area.ID != world.RogueEncampment {
		return nil
	}
	runID := rt.Config.Session.Run
	if runID == "" {
		rt.mercPreflightPending = false
		return nil
	}
	runCfg, ok := rt.Config.Runs.Run(runID)
	if !ok {
		return fmt.Errorf("mercenary preflight: run %q unavailable", runID)
	}
	profileCfg, ok := rt.Config.Profiles[runCfg.Combat.Profile]
	if !ok {
		return fmt.Errorf("mercenary preflight: profile %q unavailable", runCfg.Combat.Profile)
	}
	enabled, rule := profileCfg.Resources.Mercenary.Resolve()
	reason := town.EvaluateMercenaryPreflight(town.MercenaryPolicy{
		Enabled: enabled, ThresholdPercent: rule.UseBelowPercent,
	}, state.Mercenary)
	rt.mercPreflightPending = false
	if reason != "" {
		return fmt.Errorf("%s", reason)
	}
	return nil
}

// isTerminalMercenaryFailure reports queue-terminal Merc reasons that must never
// enter a controlled retry or Save-and-Exit recovery path.
func isTerminalMercenaryFailure(reason string) bool {
	switch town.Reason(reason) {
	case town.ReasonMercenaryNotHired,
		town.ReasonMercenaryDeadAtStart,
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
