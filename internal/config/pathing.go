package config

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// PathingConfig holds YAML tuning for the teleport navigator (`pathing:`).
// Zero/absent values fall back to Phase-4.3 defaults in applyDefaults.
type PathingConfig struct {
	StuckTimeoutMs     int                     `yaml:"stuck_timeout_ms"`
	StuckProgressTiles float64                 `yaml:"stuck_progress_tiles"`
	MoveIntervalMs     int                     `yaml:"move_interval_ms"`
	ArrivalDistance    float64                 `yaml:"arrival_distance"`
	Projection         PathingProjectionConfig `yaml:"projection"`
	Click              PathingClickConfig      `yaml:"click"`
	Explore            PathingExploreConfig    `yaml:"explore"`

	sectionPresent bool `yaml:"-"`
}

// PathingProjectionConfig calibrates the player-centered relative projection.
type PathingProjectionConfig struct {
	PlayableCenterX float64 `yaml:"playable_center_x"`
	PlayableCenterY float64 `yaml:"playable_center_y"`
	TileWidth       float64 `yaml:"tile_width"`
	TileHeight      float64 `yaml:"tile_height"`
}

// PathingClickConfig tunes the hover-feedback entity click loop.
type PathingClickConfig struct {
	MaxHoverAttempts  int     `yaml:"max_hover_attempts"`
	SpiralStep        float64 `yaml:"spiral_step"`
	AnchorOffsetTiles float64 `yaml:"anchor_offset_tiles"`
}

// PathingExploreConfig tunes bearing exploration and the entity-click distance gate.
type PathingExploreConfig struct {
	BearingCount             int     `yaml:"bearing_count"`
	StepDistanceTiles        float64 `yaml:"step_distance_tiles"`
	MaxEntranceClickDistance float64 `yaml:"max_entrance_click_distance"`
}

// UnmarshalYAML records whether the pathing section was present.
func (c *PathingConfig) UnmarshalYAML(value *yaml.Node) error {
	type pathingConfigAlias PathingConfig
	var alias pathingConfigAlias
	if err := value.Decode(&alias); err != nil {
		return err
	}
	*c = PathingConfig(alias)
	c.sectionPresent = true
	return nil
}

func (c *PathingConfig) validate() error {
	if c.StuckTimeoutMs <= 0 {
		return fmt.Errorf("pathing.stuck_timeout_ms must be > 0")
	}
	if c.StuckProgressTiles <= 0 {
		return fmt.Errorf("pathing.stuck_progress_tiles must be > 0")
	}
	if c.MoveIntervalMs < 0 {
		return fmt.Errorf("pathing.move_interval_ms must be >= 0")
	}
	if c.ArrivalDistance <= 0 {
		return fmt.Errorf("pathing.arrival_distance must be > 0")
	}
	if c.Projection.PlayableCenterX < 0 || c.Projection.PlayableCenterX > 1 ||
		c.Projection.PlayableCenterY < 0 || c.Projection.PlayableCenterY > 1 {
		return fmt.Errorf("pathing.projection.playable_center_x/y must be within 0..1")
	}
	if c.Projection.TileWidth <= 0 || c.Projection.TileHeight <= 0 {
		return fmt.Errorf("pathing.projection.tile_width and tile_height must be > 0")
	}
	if c.Click.MaxHoverAttempts <= 0 {
		return fmt.Errorf("pathing.click.max_hover_attempts must be > 0")
	}
	if c.Click.SpiralStep <= 0 {
		return fmt.Errorf("pathing.click.spiral_step must be > 0")
	}
	if c.Click.AnchorOffsetTiles < 0 {
		return fmt.Errorf("pathing.click.anchor_offset_tiles must be >= 0")
	}
	if c.Explore.BearingCount <= 0 {
		return fmt.Errorf("pathing.explore.bearing_count must be > 0")
	}
	if c.Explore.StepDistanceTiles <= 0 {
		return fmt.Errorf("pathing.explore.step_distance_tiles must be > 0")
	}
	if c.Explore.MaxEntranceClickDistance <= 0 {
		return fmt.Errorf("pathing.explore.max_entrance_click_distance must be > 0")
	}
	return nil
}

func (c *PathingConfig) applyDefaults() {
	if c.StuckTimeoutMs == 0 {
		c.StuckTimeoutMs = 8000
	}
	if c.StuckProgressTiles == 0 {
		c.StuckProgressTiles = 3
	}
	if c.MoveIntervalMs == 0 {
		c.MoveIntervalMs = 250
	}
	if c.ArrivalDistance == 0 {
		c.ArrivalDistance = 15
	}
	if c.Projection.PlayableCenterX == 0 {
		c.Projection.PlayableCenterX = 0.5
	}
	if c.Projection.PlayableCenterY == 0 {
		c.Projection.PlayableCenterY = 0.52
	}
	if c.Projection.TileWidth == 0 {
		c.Projection.TileWidth = 19.8
	}
	if c.Projection.TileHeight == 0 {
		c.Projection.TileHeight = 9.9
	}
	if c.Click.MaxHoverAttempts == 0 {
		c.Click.MaxHoverAttempts = 15
	}
	if c.Click.SpiralStep == 0 {
		c.Click.SpiralStep = 40
	}
	if c.Click.AnchorOffsetTiles == 0 {
		c.Click.AnchorOffsetTiles = 2
	}
	if c.Explore.BearingCount == 0 {
		c.Explore.BearingCount = 8
	}
	if c.Explore.StepDistanceTiles == 0 {
		c.Explore.StepDistanceTiles = 25
	}
	if c.Explore.MaxEntranceClickDistance == 0 {
		c.Explore.MaxEntranceClickDistance = 15
	}
}
