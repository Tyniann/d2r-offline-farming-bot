package app

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/memory"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

const (
	reasonProfileRequiredSkillsMissing = "profile_required_skills_missing"
	reasonProfileSkillsReadUnavailable = "profile_skills_read_unavailable"
)

// verifyProfileSkillsOnce waits for one complete PlayerSkills snapshot and
// compares required combat-profile skills. Incomplete lists never produce a
// Missing result. On Missing or read timeout the queue stops with ExitRequired.
func (rt *Runtime) verifyProfileSkillsOnce(parent context.Context, profileID string, required []config.RequiredSkillConfig) SupervisorRunResult {
	if rt == nil {
		return SupervisorRunResult{Disposition: QueueRunStop, Reason: reasonProfileSkillsReadUnavailable, ExitRequired: true}
	}
	requiredIDs, labels, err := resolveRequiredSkillIDs(required)
	if err != nil {
		rt.Log.Error("profile skills gate failed", "error", err)
		return SupervisorRunResult{Disposition: QueueRunStop, Reason: reasonProfileSkillsReadUnavailable, ExitRequired: true}
	}
	ctx, cancel := context.WithTimeout(parent, time.Duration(rt.Config.Session.StateTimeoutMs)*time.Millisecond)
	defer cancel()
	rt.startShutdownSignals(ctx, cancel)
	defer func() {
		rt.Input.Unbind()
		_ = rt.Process.Detach()
	}()
	hotkeys, hotkeyErr := rt.startHotkeys(ctx)
	if hotkeyErr != nil {
		return queueRuntimeTerminal(hotkeyErr)
	}
	defer rt.stopHotkeys(cancel)
	state := &runState{}
	ticker := time.NewTicker(time.Duration(rt.Config.Runtime.PollIntervalMs) * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			if errors.Is(ctx.Err(), context.Canceled) {
				return queueRuntimeTerminal(ctx.Err())
			}
			rt.Log.Error("profile skills gate timed out waiting for complete skill list",
				"profile", profileID, "incomplete_reason", rt.World.Current().Player.SkillsIncompleteReason)
			return SupervisorRunResult{Disposition: QueueRunStop, Reason: reasonProfileSkillsReadUnavailable, ExitRequired: true}
		case event := <-hotkeys:
			rt.handleHotkeyEvent(event, cancel)
		case <-ticker.C:
			if pollErr := rt.pollQueueSnapshot(ctx, state); pollErr != nil && !errors.Is(pollErr, context.Canceled) {
				return queueRuntimeTerminal(pollErr)
			}
			player := rt.World.Current().Player
			if !player.SkillsComplete {
				continue
			}
			missing := missingRequiredSkills(player, requiredIDs, labels)
			if len(missing) == 0 {
				rt.Log.Info("profile skills gate passed", "profile", profileID, "skills", len(requiredIDs))
				return SupervisorRunResult{}
			}
			rt.Log.Error("profile required skills missing", "profile", profileID, "missing", strings.Join(missing, ", "))
			profileLabel := profileID
			if profileCfg, ok := rt.Config.Profiles[profileID]; ok && strings.TrimSpace(profileCfg.DisplayName) != "" {
				profileLabel = strings.TrimSpace(profileCfg.DisplayName)
			}
			return profileRequiredSkillsMissingResult(rt.Config.Session.Character, profileLabel, missing)
		}
	}
}

func profileRequiredSkillsMissingResult(characterName, profileID string, missing []string) SupervisorRunResult {
	character := strings.TrimSpace(characterName)
	if character == "" {
		character = "Der Charakter"
	}
	profileLabel := strings.TrimSpace(profileID)
	if profileLabel == "" {
		profileLabel = "das Kampfprofil"
	}
	return SupervisorRunResult{
		Disposition:  QueueRunStop,
		Reason:       reasonProfileRequiredSkillsMissing,
		Detail:       fmt.Sprintf("%s fehlen für %s: %s. Die Queue wurde beendet.", character, profileLabel, strings.Join(missing, ", ")),
		ExitRequired: true,
	}
}

func resolveRequiredSkillIDs(required []config.RequiredSkillConfig) ([]uint16, map[uint16]string, error) {
	ids := make([]uint16, 0, len(required))
	labels := make(map[uint16]string, len(required))
	for _, entry := range required {
		id, err := memory.ParseSkillTestName(entry.Skill)
		if err != nil {
			return nil, nil, fmt.Errorf("required skill %q: %w", entry.Skill, err)
		}
		ids = append(ids, id)
		label := strings.TrimSpace(entry.DisplayName)
		if label == "" {
			label = memory.SkillName(id)
		}
		labels[id] = label
	}
	return ids, labels, nil
}

func missingRequiredSkills(player world.Player, required []uint16, labels map[uint16]string) []string {
	if !player.SkillsComplete {
		return nil
	}
	missing := make([]string, 0)
	seen := make(map[uint16]bool, len(required))
	for _, id := range required {
		if seen[id] {
			continue
		}
		seen[id] = true
		if playerHasRequiredSkill(player, id) {
			continue
		}
		label := labels[id]
		if label == "" {
			label = memory.SkillName(id)
		}
		missing = append(missing, label)
	}
	sort.Strings(missing)
	return missing
}

func playerHasRequiredSkill(player world.Player, requiredID uint16) bool {
	if player.SkillsKnown[requiredID] {
		return true
	}
	if !isTownPortalEquivalent(requiredID) {
		return false
	}
	for _, evidenceID := range skillTownPortalEquivalentIDs {
		if player.SkillsKnown[evidenceID] {
			return true
		}
	}
	return false
}
