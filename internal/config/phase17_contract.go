package config

import (
	"fmt"
	"math"
)

const (
	// DefaultRouteCombatImmediateRadiusTiles ist der Phase-17-Radius für akute Gefahren um den Spieler.
	DefaultRouteCombatImmediateRadiusTiles = 18.0
	// DefaultRouteCombatCorridorWidthTiles ist die Phase-17-Halbbreite des vorausliegenden Routenkorridors.
	DefaultRouteCombatCorridorWidthTiles = 7.0
	// DefaultRouteCombatLandingRadiusTiles ist der Phase-17-Sicherheitsradius um das effektive Bewegungsziel.
	DefaultRouteCombatLandingRadiusTiles = 10.0
	// DefaultRouteCombatAttackDistanceTiles ist die maximale bestätigte Angriffsdistanz aus der aktuellen Position.
	DefaultRouteCombatAttackDistanceTiles = 30.0
	// DefaultRouteCombatNoProgressTimeoutMs begrenzt objektiven Stillstand während eines Route-Clears.
	DefaultRouteCombatNoProgressTimeoutMs = 12000
	// DefaultRouteCombatTeleportManaReservePercent startet den Route-Mana-Hold.
	DefaultRouteCombatTeleportManaReservePercent = 20
	// DefaultRouteCombatResumeManaPercent beendet einen begonnenen Route-Mana-Hold.
	DefaultRouteCombatResumeManaPercent = 35
	// DefaultRouteCombatEmergencyManaPercent markiert kritisches Mana bei einer unmittelbaren Bedrohung.
	DefaultRouteCombatEmergencyManaPercent = 10
	// DefaultRouteCombatManaRecoveryTimeoutMs begrenzt die Wiederherstellung der Mobilitätsreserve.
	DefaultRouteCombatManaRecoveryTimeoutMs = 5000
)

// RouteCombatConfig ist der additive Operatorvertrag für bedrohungsbewusstes Route-Playback.
// Die Pointerform von Enabled hält fehlend und explizit false unterscheidbar.
type RouteCombatConfig struct {
	// Enabled ist der explizite Rollbackschalter; nil verlangt den run-ID-bewussten Default.
	Enabled *bool `yaml:"enabled,omitempty"`
	// ImmediateRadiusTiles begrenzt die akute Gefahrenzone um den Spieler.
	ImmediateRadiusTiles float64 `yaml:"immediate_radius_tiles"`
	// CorridorWidthTiles begrenzt den Abstand zur vorausliegenden Spieler-Ziel-Kante.
	CorridorWidthTiles float64 `yaml:"corridor_width_tiles"`
	// LandingRadiusTiles schützt den nächsten tatsächlichen Bewegungs- oder Recovery-Landepunkt.
	LandingRadiusTiles float64 `yaml:"landing_radius_tiles"`
	// AttackDistanceTiles ist die maximale sicher projizierbare Angriffsdistanz.
	AttackDistanceTiles float64 `yaml:"attack_distance_tiles"`
	// NoProgressTimeoutMs begrenzt Stillstand ohne objektiven Threat- oder Coverage-Fortschritt.
	NoProgressTimeoutMs int `yaml:"no_progress_timeout_ms"`
	// TeleportManaReservePercent startet den Mobilitäts-Hold.
	TeleportManaReservePercent int `yaml:"teleport_mana_reserve_percent"`
	// ResumeManaPercent beendet einen bereits begonnenen Mobilitäts-Hold.
	ResumeManaPercent int `yaml:"resume_mana_percent"`
	// EmergencyManaPercent markiert kritisches Mana bei unmittelbarer Bedrohung.
	EmergencyManaPercent int `yaml:"emergency_mana_percent"`
	// ManaRecoveryTimeoutMs begrenzt die bestätigte Wiederherstellung der Manareserve.
	ManaRecoveryTimeoutMs int `yaml:"mana_recovery_timeout_ms"`
}

// EnabledValue returns the resolved route-combat switch after defaults.
func (c RouteCombatConfig) EnabledValue() bool {
	return c.Enabled != nil && *c.Enabled
}

func (c *RouteCombatConfig) applyDefaults(runID string) {
	if c.Enabled == nil {
		enabled := runID == "summoner"
		c.Enabled = &enabled
	}
	if c.ImmediateRadiusTiles == 0 {
		c.ImmediateRadiusTiles = DefaultRouteCombatImmediateRadiusTiles
	}
	if c.CorridorWidthTiles == 0 {
		c.CorridorWidthTiles = DefaultRouteCombatCorridorWidthTiles
	}
	if c.LandingRadiusTiles == 0 {
		c.LandingRadiusTiles = DefaultRouteCombatLandingRadiusTiles
	}
	if c.AttackDistanceTiles == 0 {
		c.AttackDistanceTiles = DefaultRouteCombatAttackDistanceTiles
	}
	if c.NoProgressTimeoutMs == 0 {
		c.NoProgressTimeoutMs = DefaultRouteCombatNoProgressTimeoutMs
	}
	if c.TeleportManaReservePercent == 0 {
		c.TeleportManaReservePercent = DefaultRouteCombatTeleportManaReservePercent
	}
	if c.ResumeManaPercent == 0 {
		c.ResumeManaPercent = DefaultRouteCombatResumeManaPercent
	}
	if c.EmergencyManaPercent == 0 {
		c.EmergencyManaPercent = DefaultRouteCombatEmergencyManaPercent
	}
	if c.ManaRecoveryTimeoutMs == 0 {
		c.ManaRecoveryTimeoutMs = DefaultRouteCombatManaRecoveryTimeoutMs
	}
}

func (c RouteCombatConfig) validate(runID, path string) error {
	if c.EnabledValue() && runID != "summoner" && runID != "cows" {
		return fmt.Errorf("%s.enabled requires a route-clear capable run", path)
	}
	for name, value := range map[string]float64{
		"immediate_radius_tiles": c.ImmediateRadiusTiles,
		"corridor_width_tiles":   c.CorridorWidthTiles,
		"landing_radius_tiles":   c.LandingRadiusTiles,
		"attack_distance_tiles":  c.AttackDistanceTiles,
	} {
		if value <= 0 || math.IsNaN(value) || math.IsInf(value, 0) {
			return fmt.Errorf("%s.%s must be finite and > 0", path, name)
		}
	}
	if !(c.CorridorWidthTiles < c.ImmediateRadiusTiles && c.ImmediateRadiusTiles < c.AttackDistanceTiles) {
		return fmt.Errorf("%s requires corridor_width_tiles < immediate_radius_tiles < attack_distance_tiles", path)
	}
	if c.LandingRadiusTiles >= c.AttackDistanceTiles {
		return fmt.Errorf("%s.landing_radius_tiles must be < attack_distance_tiles", path)
	}
	for name, value := range map[string]int{
		"teleport_mana_reserve_percent": c.TeleportManaReservePercent,
		"resume_mana_percent":           c.ResumeManaPercent,
		"emergency_mana_percent":        c.EmergencyManaPercent,
	} {
		if value < 1 || value > 100 {
			return fmt.Errorf("%s.%s must be within 1..100", path, name)
		}
	}
	if c.EmergencyManaPercent >= c.TeleportManaReservePercent || c.TeleportManaReservePercent > c.ResumeManaPercent {
		return fmt.Errorf("%s requires emergency_mana_percent < teleport_mana_reserve_percent <= resume_mana_percent", path)
	}
	if c.NoProgressTimeoutMs < 3000 || c.NoProgressTimeoutMs > 30000 {
		return fmt.Errorf("%s.no_progress_timeout_ms must be within 3000..30000", path)
	}
	if c.ManaRecoveryTimeoutMs < 1000 || c.ManaRecoveryTimeoutMs > 15000 {
		return fmt.Errorf("%s.mana_recovery_timeout_ms must be within 1000..15000", path)
	}
	if c.ManaRecoveryTimeoutMs > c.NoProgressTimeoutMs {
		return fmt.Errorf("%s.mana_recovery_timeout_ms must be <= no_progress_timeout_ms", path)
	}
	return nil
}
