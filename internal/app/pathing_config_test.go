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
		WaypointUI: config.PathingWaypointUIConfig{
			BlackMarshX: 201,
			BlackMarshY: 343,
		},
		TownWalk: config.PathingTownWalkConfig{
			RouteFile:          "configs/routes/custom.yaml",
			ForceMoveKey:       "e",
			MoveIntervalMs:     651,
			SettleTimeoutMs:    351,
			StuckTimeoutMs:     3501,
			ArrivalDistance:    9,
			Act1WaypointPoints: []config.PathingPointConfig{{X: 1, Y: 2}, {X: 3, Y: 4}},
		},
	}

	got := mapPathingConfig(cfg)
	if got.Waypoint.MaxClickDistance != 17 {
		t.Fatalf("Waypoint.MaxClickDistance = %v, want 17", got.Waypoint.MaxClickDistance)
	}
	if got.WaypointUI.BlackMarshX != 201 || got.WaypointUI.BlackMarshY != 343 {
		t.Fatalf("WaypointUI = %+v, want 201/343", got.WaypointUI)
	}
	if got.TownWalk.RouteFile != "configs/routes/custom.yaml" || got.TownWalk.ForceMoveKey != "e" {
		t.Fatalf("TownWalk key/route = %+v", got.TownWalk)
	}
	if got.TownWalk.ArrivalDistance != 9 || len(got.TownWalk.Act1WaypointPoints) != 2 ||
		got.TownWalk.Act1WaypointPoints[0].X != 1 || got.TownWalk.Act1WaypointPoints[1].Y != 4 {
		t.Fatalf("TownWalk mapped = %+v", got.TownWalk)
	}
}
