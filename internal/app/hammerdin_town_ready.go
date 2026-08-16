package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/profile"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/tasks"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

const hammerdinProfileID = "paladin_hammerdin"

// hammerdinTownReadyProfile intercepts HookFieldReady for paladin_hammerdin so
// CTA and Holy Shield run after waypoint arrival, before recorded-route playback.
// Town-ready and other hooks stay on the embedded Executor. Boss combat uses the
// shared Mephisto pipeline with an empty engage sequence.
type hammerdinTownReadyProfile struct {
	*profile.Executor
	prebuff *hammerdinPrebuff
}

func attachHammerdinTownReady(profileID string, exec *profile.Executor, bindings configBindingSource, in inputController) (tasks.ProfileActions, error) {
	if strings.TrimSpace(profileID) != hammerdinProfileID {
		return exec, nil
	}
	ctrl, ok := in.(hammerdinPrebuffInput)
	if !ok {
		return nil, fmt.Errorf("hammerdin field-ready prebuff: verified input not wired")
	}
	prebuff, err := newInferredHammerdinPrebuff(bindings, ctrl)
	if err != nil {
		return nil, err
	}
	return &hammerdinTownReadyProfile{Executor: exec, prebuff: prebuff}, nil
}

func (h *hammerdinTownReadyProfile) TickHook(ctx context.Context, hook profile.Hook, state world.State, target profile.EncounterTarget, now time.Time) profile.Result {
	if hook != profile.HookFieldReady {
		if h == nil || h.Executor == nil {
			return profile.Result{Status: profile.StatusFailed, Hook: hook, Reason: "profile_not_wired"}
		}
		return h.Executor.TickHook(ctx, hook, state, target, now)
	}
	if ctx.Err() != nil {
		return profile.Result{Status: profile.StatusFailed, Hook: hook, Reason: "profile_cancelled"}
	}
	if h == nil || h.prebuff == nil {
		return profile.Result{Status: profile.StatusFailed, Hook: hook, Reason: "profile_not_wired"}
	}
	if state.Phase == world.GamePhaseMenu || state.Phase == world.GamePhaseLoading {
		h.prebuff.reset()
		return profile.Result{Status: profile.StatusPending, Hook: hook}
	}
	if !state.Valid || state.Phase != world.GamePhaseInGame || !state.Identity.Valid {
		return profile.Result{Status: profile.StatusPending, Hook: hook}
	}
	if state.Identity.Class != world.CharacterClassPaladin {
		return profile.Result{Status: profile.StatusFailed, Hook: hook, Reason: "profile_class_mismatch"}
	}
	if state.Area.IsTown() {
		return profile.Result{Status: profile.StatusFailed, Hook: hook, Reason: reasonPrebuffRequiresField}
	}
	result, err := h.prebuff.tick(state, now)
	if err != nil {
		return profile.Result{Status: profile.StatusFailed, Hook: hook, Reason: hammerdinPrebuffReason(err)}
	}
	if result.Done {
		return profile.Result{Status: profile.StatusComplete, Hook: hook}
	}
	if result.Action != "" {
		return profile.Result{Status: profile.StatusAction, Hook: hook}
	}
	return profile.Result{Status: profile.StatusPending, Hook: hook}
}

func (h *hammerdinTownReadyProfile) Reset() {
	if h == nil {
		return
	}
	if h.Executor != nil {
		h.Executor.Reset()
	}
	if h.prebuff != nil {
		h.prebuff.reset()
	}
}

func hammerdinPrebuffReason(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	if i := strings.IndexByte(msg, ':'); i > 0 {
		code := strings.TrimSpace(msg[:i])
		if code != "" && !strings.Contains(code, " ") {
			return code
		}
	}
	return msg
}
