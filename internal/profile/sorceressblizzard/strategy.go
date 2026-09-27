// Package sorceressblizzard implementiert das Blizzard-Profil über die gemeinsame Kampf-Pipeline.
package sorceressblizzard

import (
	"fmt"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/memory"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/profile"
)

// ID ist der stabile Profilbezeichner für die Blizzard-Zauberin.
const ID = "sorceress_blizzard"

// NewFactory registriert eine Route mit Blizzard als Standardangriff und lokalem Clear.
// Nur Summoner und Kuh-Level erlauben zusätzlich Kampf während der Routenwiedergabe.
func NewFactory(runID string) profile.StrategyFactory {
	return func() profile.RunStrategy { return &strategy{runID: runID} }
}

type strategy struct{ runID string }

// ProfileID liefert den stabilen Profilbezeichner.
func (s *strategy) ProfileID() string { return ID }

// RunID liefert die registrierte Route.
func (s *strategy) RunID() string { return s.runID }

// RequiredSkills liefert die sechs Pflichtskills für Preflight und Bindings.
func (s *strategy) RequiredSkills() []string {
	return []string{"teleport", "town_portal", "blizzard", "ice_blast", "static_field", "frozen_armor"}
}

// RequiresRouteClear erlaubt Reisekampf nur in Summoner und Kuh-Level.
func (s *strategy) RequiresRouteClear() bool { return s.runID == "summoner" || s.runID == "cows" }

// SupportsLocalRecoveryClear aktiviert die bestehende begrenzte Portal-Recovery.
func (s *strategy) SupportsLocalRecoveryClear() {}

// Configure bindet Blizzard ohne Fluch oder Kadaverexplosion an den lokalen Clear.
func (s *strategy) Configure(exec *profile.Executor, standardAttackID uint16, actions profile.RouteCombatActions) error {
	if exec == nil || actions == nil || standardAttackID != memory.MustSkillID("blizzard") {
		return fmt.Errorf("blizzard strategy requires executor, local clear and Blizzard standard attack")
	}
	return exec.ConfigureRouteClear(profile.RouteClearSingleTarget, 0, standardAttackID, actions)
}
