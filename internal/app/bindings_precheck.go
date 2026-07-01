package app

import (
	"fmt"
	"log/slog"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/memory"
)

// BindingsPrecheck validates configured skill bindings before input or pathing modes run.
// Teleport must be configured on right mouse (hard stop). Town portal emits a warning only.
func BindingsPrecheck(log *slog.Logger, bindings configBindingSource, snap memory.Snapshot, inputActive bool) error {
	if !inputActive {
		return nil
	}
	if !snap.Valid || snap.Phase != memory.GamePhaseInGame {
		return nil
	}

	teleportCast, err := bindings.Resolve(memory.SkillTeleport)
	if err != nil {
		log.Warn("teleport binding not configured",
			"left_skill_id", snap.PlayerSkills.LeftSkill,
			"right_skill_id", snap.PlayerSkills.RightSkill,
		)
		return fmt.Errorf("teleport not configured: set input.bindings.skills.teleport.key/button in YAML")
	}
	if teleportCast.CastButton != "right" {
		log.Warn("teleport resolved to unsafe mouse button",
			"select_key", teleportCast.SelectKey,
			"cast_button", teleportCast.CastButton,
		)
		return fmt.Errorf("teleport binding unsafe: expected right-skill binding, got %s on %s", teleportCast.SelectKey, teleportCast.CastButton)
	}
	log.Info("teleport binding configured",
		"select_key", teleportCast.SelectKey,
		"cast_button", teleportCast.CastButton,
	)
	if _, ok := bindings.TownPortalSkillID(); !ok {
		log.Warn("town portal binding not configured; portal actions will fail until input.bindings.skills.town_portal is set")
	}
	if !memory.IsBasicLeftSkill(snap.PlayerSkills.LeftSkill) {
		log.Warn("left mouse skill is not attack or throw",
			"left_skill", memory.SkillName(snap.PlayerSkills.LeftSkill),
			"left_skill_id", snap.PlayerSkills.LeftSkill,
		)
	}
	return nil
}
