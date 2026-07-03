package config

import "testing"

func TestPathingConfigDefaults(t *testing.T) {
	var cfg PathingConfig
	cfg.applyDefaults()

	if cfg.StuckTimeoutMs != 8000 {
		t.Fatalf("StuckTimeoutMs = %d, want 8000", cfg.StuckTimeoutMs)
	}
	if cfg.StuckProgressTiles != 3 {
		t.Fatalf("StuckProgressTiles = %v, want 3", cfg.StuckProgressTiles)
	}
	if cfg.MoveIntervalMs != 250 {
		t.Fatalf("MoveIntervalMs = %d, want 250", cfg.MoveIntervalMs)
	}
	if cfg.ArrivalDistance != 15 {
		t.Fatalf("ArrivalDistance = %v, want 15", cfg.ArrivalDistance)
	}
	if cfg.Projection.PlayableCenterX != 0.5 || cfg.Projection.PlayableCenterY != 0.52 {
		t.Fatalf("PlayableCenter = %v/%v, want 0.5/0.52", cfg.Projection.PlayableCenterX, cfg.Projection.PlayableCenterY)
	}
	if cfg.Projection.TileWidth != 19.8 || cfg.Projection.TileHeight != 9.9 {
		t.Fatalf("Tile = %v/%v, want 19.8/9.9", cfg.Projection.TileWidth, cfg.Projection.TileHeight)
	}
	if cfg.Click.MaxHoverAttempts != 15 || cfg.Click.SpiralStep != 40 || cfg.Click.AnchorOffsetTiles != 2 {
		t.Fatalf("Click = %+v, want defaults 15/40/2", cfg.Click)
	}
	if cfg.Explore.BearingCount != 8 || cfg.Explore.StepDistanceTiles != 25 || cfg.Explore.MaxEntranceClickDistance != 15 {
		t.Fatalf("Explore = %+v, want defaults 8/25/15", cfg.Explore)
	}
	if cfg.Waypoint.MaxClickDistance != 15 {
		t.Fatalf("Waypoint.MaxClickDistance = %v, want 15", cfg.Waypoint.MaxClickDistance)
	}
	if cfg.WaypointUI.BlackMarshX != 200 || cfg.WaypointUI.BlackMarshY != 342 {
		t.Fatalf("WaypointUI = %+v, want 200/342", cfg.WaypointUI)
	}
	if cfg.TownWalk.ForceMoveKey != "e" || cfg.TownWalk.RouteFile != "configs/routes/act1-town-waypoint.yaml" {
		t.Fatalf("TownWalk key/route = %+v, want e/default route", cfg.TownWalk)
	}
	if cfg.TownWalk.MoveIntervalMs != 650 || cfg.TownWalk.SettleTimeoutMs != 350 ||
		cfg.TownWalk.StuckTimeoutMs != 3500 || cfg.TownWalk.ArrivalDistance != 8 {
		t.Fatalf("TownWalk timings = %+v, want defaults", cfg.TownWalk)
	}

	if err := cfg.validate(); err != nil {
		t.Fatalf("validate() after defaults error = %v", err)
	}
}

func TestPathingConfigPartialOverrideKeepsDefaults(t *testing.T) {
	cfg := PathingConfig{StuckTimeoutMs: 12000}
	cfg.applyDefaults()
	if cfg.StuckTimeoutMs != 12000 {
		t.Fatalf("StuckTimeoutMs = %d, want override 12000", cfg.StuckTimeoutMs)
	}
	if cfg.Explore.BearingCount != 8 {
		t.Fatalf("BearingCount = %d, want default 8", cfg.Explore.BearingCount)
	}
}

func TestPathingConfigValidateRejectsInvalid(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*PathingConfig)
	}{
		{"negative stuck timeout", func(c *PathingConfig) { c.StuckTimeoutMs = -1 }},
		{"negative progress tiles", func(c *PathingConfig) { c.StuckProgressTiles = -1 }},
		{"negative move interval", func(c *PathingConfig) { c.MoveIntervalMs = -1 }},
		{"negative arrival distance", func(c *PathingConfig) { c.ArrivalDistance = -1 }},
		{"center out of range", func(c *PathingConfig) { c.Projection.PlayableCenterY = 2 }},
		{"negative tile width", func(c *PathingConfig) { c.Projection.TileWidth = -1 }},
		{"negative hover attempts", func(c *PathingConfig) { c.Click.MaxHoverAttempts = -1 }},
		{"negative spiral step", func(c *PathingConfig) { c.Click.SpiralStep = -1 }},
		{"negative anchor offset", func(c *PathingConfig) { c.Click.AnchorOffsetTiles = -1 }},
		{"negative bearing count", func(c *PathingConfig) { c.Explore.BearingCount = -1 }},
		{"negative step distance", func(c *PathingConfig) { c.Explore.StepDistanceTiles = -1 }},
		{"negative click distance", func(c *PathingConfig) { c.Explore.MaxEntranceClickDistance = -1 }},
		{"negative waypoint distance", func(c *PathingConfig) { c.Waypoint.MaxClickDistance = -1 }},
		{"negative waypoint ui x", func(c *PathingConfig) { c.WaypointUI.BlackMarshX = -1 }},
		{"negative waypoint ui y", func(c *PathingConfig) { c.WaypointUI.BlackMarshY = -1 }},
		{"missing town route", func(c *PathingConfig) { c.TownWalk.RouteFile = "" }},
		{"invalid force move key", func(c *PathingConfig) { c.TownWalk.ForceMoveKey = "mouse4" }},
		{"zero town move interval", func(c *PathingConfig) { c.TownWalk.MoveIntervalMs = 0 }},
		{"zero town settle timeout", func(c *PathingConfig) { c.TownWalk.SettleTimeoutMs = 0 }},
		{"zero town stuck timeout", func(c *PathingConfig) { c.TownWalk.StuckTimeoutMs = 0 }},
		{"zero town arrival distance", func(c *PathingConfig) { c.TownWalk.ArrivalDistance = 0 }},
		{"zero town point coordinate", func(c *PathingConfig) {
			c.TownWalk.Act1WaypointPoints = []PathingPointConfig{{X: 0, Y: 1}}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var cfg PathingConfig
			cfg.applyDefaults()
			tc.mutate(&cfg)
			if err := cfg.validate(); err == nil {
				t.Fatalf("validate() expected error for %s", tc.name)
			}
		})
	}
}
