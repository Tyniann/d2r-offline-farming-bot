package pathing

import "testing"

func TestDefaultConfigValid(t *testing.T) {
	if err := DefaultConfig().Validate(); err != nil {
		t.Fatalf("DefaultConfig().Validate() error = %v", err)
	}
}

func TestConfigValidateRejectsInvalidValues(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*Config)
	}{
		{"zero stuck timeout", func(c *Config) { c.StuckTimeout = 0 }},
		{"zero progress tiles", func(c *Config) { c.StuckProgressTiles = 0 }},
		{"negative move interval", func(c *Config) { c.MoveInterval = -1 }},
		{"zero arrival distance", func(c *Config) { c.ArrivalDistance = 0 }},
		{"zero tile width", func(c *Config) { c.Projection.TileWidth = 0 }},
		{"center out of range", func(c *Config) { c.Projection.PlayableCenterX = 1.5 }},
		{"zero hover attempts", func(c *Config) { c.Click.MaxHoverAttempts = 0 }},
		{"zero spiral step", func(c *Config) { c.Click.SpiralStepDegrees = 0 }},
		{"negative anchor offset", func(c *Config) { c.Click.AnchorOffsetTiles = -1 }},
		{"zero bearing count", func(c *Config) { c.Explore.BearingCount = 0 }},
		{"zero step distance", func(c *Config) { c.Explore.StepDistanceTiles = 0 }},
		{"zero click distance", func(c *Config) { c.Explore.MaxEntranceClickDistance = 0 }},
		{"zero waypoint distance", func(c *Config) { c.Waypoint.MaxClickDistance = 0 }},
		{"zero portal appear timeout", func(c *Config) { c.TownPortal.AppearTimeout = 0 }},
		{"zero portal click distance", func(c *Config) { c.TownPortal.MaxClickDistance = 0 }},
		{"negative waypoint ui x", func(c *Config) { c.WaypointUI.BlackMarshX = -1 }},
		{"negative waypoint ui y", func(c *Config) { c.WaypointUI.BlackMarshY = -1 }},
		{"missing force move key", func(c *Config) { c.TownWalk.ForceMoveKey = "" }},
		{"zero town move interval", func(c *Config) { c.TownWalk.MoveInterval = 0 }},
		{"zero town settle timeout", func(c *Config) { c.TownWalk.SettleTimeout = 0 }},
		{"zero town stuck timeout", func(c *Config) { c.TownWalk.StuckTimeout = 0 }},
		{"zero town arrival distance", func(c *Config) { c.TownWalk.ArrivalDistance = 0 }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := DefaultConfig()
			tc.mutate(&cfg)
			if err := cfg.Validate(); err == nil {
				t.Fatalf("Validate() expected error for %s", tc.name)
			}
		})
	}
}
