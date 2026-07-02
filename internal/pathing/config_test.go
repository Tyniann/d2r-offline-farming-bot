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
