package app

import (
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/input"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/pathing"
)

// Empfohlene Client-Größe für die Relative-Projektion (Koolo-Kalibrierung 19.8/9.9).
const (
	recommendedClientWidth  = 1280
	recommendedClientHeight = 720
)

// warnPathingResolution logs a warning when the bound window deviates from the
// recommended 1280×720 windowed size that the projection defaults are calibrated for.
func (rt *Runtime) warnPathingResolution() {
	ctrl, ok := rt.Input.(interface {
		Window() (input.WindowInfo, bool)
	})
	if !ok {
		return
	}
	win, ok := ctrl.Window()
	if !ok {
		return
	}
	if win.ClientWidth == recommendedClientWidth && win.ClientHeight == recommendedClientHeight {
		return
	}
	rt.Log.Warn("Fenstergröße weicht von der Empfehlung ab — Projektion ggf. ungenau",
		"client_width", win.ClientWidth,
		"client_height", win.ClientHeight,
		"empfohlen", "1280×720 (windowed)",
		"hinweis", "tile_width/tile_height und playable_center in pathing.projection kalibrieren",
	)
}

// mapPathingConfig converts YAML pathing settings into the pathing package config.
func mapPathingConfig(cfg config.PathingConfig) pathing.Config {
	return pathing.Config{
		StuckTimeout:       time.Duration(cfg.StuckTimeoutMs) * time.Millisecond,
		StuckProgressTiles: cfg.StuckProgressTiles,
		MoveInterval:       time.Duration(cfg.MoveIntervalMs) * time.Millisecond,
		ArrivalDistance:    cfg.ArrivalDistance,
		Projection: pathing.ProjectionConfig{
			PlayableCenterX: cfg.Projection.PlayableCenterX,
			PlayableCenterY: cfg.Projection.PlayableCenterY,
			TileWidth:       cfg.Projection.TileWidth,
			TileHeight:      cfg.Projection.TileHeight,
		},
		Click: pathing.ClickConfig{
			MaxHoverAttempts:  cfg.Click.MaxHoverAttempts,
			SpiralStepDegrees: cfg.Click.SpiralStep,
			AnchorOffsetTiles: cfg.Click.AnchorOffsetTiles,
		},
		Explore: pathing.ExploreConfig{
			BearingCount:             cfg.Explore.BearingCount,
			StepDistanceTiles:        cfg.Explore.StepDistanceTiles,
			MaxEntranceClickDistance: cfg.Explore.MaxEntranceClickDistance,
		},
		Waypoint: pathing.WaypointConfig{
			MaxClickDistance: cfg.Waypoint.MaxClickDistance,
		},
		TownPortal: pathing.TownPortalConfig{
			AppearTimeout:    time.Duration(cfg.TownPortal.AppearTimeoutMs) * time.Millisecond,
			MaxClickDistance: cfg.TownPortal.MaxClickDistance,
		},
		TownWalk: pathing.TownWalkConfig{
			ForceMoveKey:    cfg.TownWalk.ForceMoveKey,
			MoveInterval:    time.Duration(cfg.TownWalk.MoveIntervalMs) * time.Millisecond,
			SettleTimeout:   time.Duration(cfg.TownWalk.SettleTimeoutMs) * time.Millisecond,
			StuckTimeout:    time.Duration(cfg.TownWalk.StuckTimeoutMs) * time.Millisecond,
			ArrivalDistance: cfg.TownWalk.ArrivalDistance,
		},
	}
}
