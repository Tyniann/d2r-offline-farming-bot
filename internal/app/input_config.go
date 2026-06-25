package app

import (
	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/input"
)

// mapInputConfig converts YAML input settings to the input package keyboard config.
func mapInputConfig(cfg config.InputConfig) input.KeyboardConfig {
	return input.KeyboardConfig{
		KeyDelayMsMin: cfg.KeyDelayMsMin,
		KeyDelayMsMax: cfg.KeyDelayMsMax,
		ComboHoldMs:   cfg.ComboHoldMs,
		Skills:        cfg.Skills.Slots(),
		Belt:          cfg.Belt.Slots(),
		TownPortal:    cfg.TownPortal,
	}
}

// mapSafetyConfig converts YAML input safety settings to the input package safety config.
func mapSafetyConfig(cfg config.InputConfig) input.SafetyConfig {
	return input.SafetyConfig{
		Enabled:     cfg.Enabled,
		PauseHotkey: cfg.PauseHotkey,
		StopHotkey:  cfg.StopHotkey,
	}
}
