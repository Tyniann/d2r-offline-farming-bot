package app

import (
	"testing"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/config"
)

func TestMapPathingConfigMapsWaypointSettings(t *testing.T) {
	cfg := config.PathingConfig{
		StuckTimeoutMs:     8000,
		StuckProgressTiles: 3,
		MoveIntervalMs:     250,
		ArrivalDistance:    15,
		Projection: config.PathingProjectionConfig{
			PlayableCenterX: 0.5,
			PlayableCenterY: 0.52,
			TileWidth:       19.8,
			TileHeight:      9.9,
		},
		Click: config.PathingClickConfig{
			MaxHoverAttempts:  15,
			SpiralStep:        40,
			AnchorOffsetTiles: 2,
		},
		Explore: config.PathingExploreConfig{
			BearingCount:             8,
			StepDistanceTiles:        25,
			MaxEntranceClickDistance: 15,
		},
		Waypoint: config.PathingWaypointConfig{
			MaxClickDistance: 17,
		},
		TownWalk: config.PathingTownWalkConfig{
			ForceMoveKey:    "e",
			MoveIntervalMs:  651,
			SettleTimeoutMs: 351,
			StuckTimeoutMs:  3501,
			ArrivalDistance: 9,
		},
	}

	got := mapPathingConfig(cfg)
	if got.Waypoint.MaxClickDistance != 17 {
		t.Fatalf("Waypoint.MaxClickDistance = %v, want 17", got.Waypoint.MaxClickDistance)
	}
	if got.TownWalk.ForceMoveKey != "e" {
		t.Fatalf("TownWalk key = %+v", got.TownWalk)
	}
	if got.TownWalk.ArrivalDistance != 9 {
		t.Fatalf("TownWalk mapped = %+v", got.TownWalk)
	}
}
