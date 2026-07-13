package config

import (
	"fmt"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/input"
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
	Waypoint           PathingWaypointConfig   `yaml:"waypoint"`
	TownPortal         PathingTownPortalConfig `yaml:"town_portal"`
	WaypointUI         PathingWaypointUIConfig `yaml:"waypoint_ui"`
	TownWalk           PathingTownWalkConfig   `yaml:"town_walk"`

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

// PathingWaypointConfig tunes hover-confirmed waypoint object clicks.
type PathingWaypointConfig struct {
	// MaxClickDistance gates waypoint object clicks by tile distance.
	MaxClickDistance float64 `yaml:"max_click_distance"`
}

// PathingTownPortalConfig tunes discovery and hover-confirmed portal entry.
type PathingTownPortalConfig struct {
	AppearTimeoutMs  int     `yaml:"appear_timeout_ms"`
	MaxClickDistance float64 `yaml:"max_click_distance"`
}

// PathingWaypointUIConfig holds fixed client-relative waypoint menu coordinates.
type PathingWaypointUIConfig struct {
	// BlackMarshX is the client-relative X coordinate for Black Marsh.
	BlackMarshX int `yaml:"black_marsh_x"`
	// BlackMarshY is the client-relative Y coordinate for Black Marsh.
	BlackMarshY int `yaml:"black_marsh_y"`
}

// PathingPointConfig is a YAML world-coordinate point.
// PathingTownWalkConfig tunes layout-bound Act-1 graph-edge walking.
type PathingTownWalkConfig struct {
	ForceMoveKey    string  `yaml:"force_move_key"`
	MoveIntervalMs  int     `yaml:"move_interval_ms"`
	SettleTimeoutMs int     `yaml:"settle_timeout_ms"`
	StuckTimeoutMs  int     `yaml:"stuck_timeout_ms"`
	ArrivalDistance float64 `yaml:"arrival_distance_tiles"`
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
	if c.Waypoint.MaxClickDistance <= 0 {
		return fmt.Errorf("pathing.waypoint.max_click_distance must be > 0")
	}
	if c.TownPortal.AppearTimeoutMs <= 0 {
		return fmt.Errorf("pathing.town_portal.appear_timeout_ms must be > 0")
	}
	if c.TownPortal.MaxClickDistance <= 0 {
		return fmt.Errorf("pathing.town_portal.max_click_distance must be > 0")
	}
	if c.WaypointUI.BlackMarshX < 0 || c.WaypointUI.BlackMarshY < 0 {
		return fmt.Errorf("pathing.waypoint_ui.black_marsh_x/y must be >= 0")
	}
	if c.TownWalk.ForceMoveKey == "" {
		return fmt.Errorf("pathing.town_walk.force_move_key is required")
	}
	if err := input.ValidateKeyStrings(c.TownWalk.ForceMoveKey); err != nil {
		return fmt.Errorf("pathing.town_walk.force_move_key: %w", err)
	}
	if c.TownWalk.MoveIntervalMs <= 0 {
		return fmt.Errorf("pathing.town_walk.move_interval_ms must be > 0")
	}
	if c.TownWalk.SettleTimeoutMs <= 0 {
		return fmt.Errorf("pathing.town_walk.settle_timeout_ms must be > 0")
	}
	if c.TownWalk.StuckTimeoutMs <= 0 {
		return fmt.Errorf("pathing.town_walk.stuck_timeout_ms must be > 0")
	}
	if c.TownWalk.ArrivalDistance <= 0 {
		return fmt.Errorf("pathing.town_walk.arrival_distance_tiles must be > 0")
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
	if c.Waypoint.MaxClickDistance == 0 {
		c.Waypoint.MaxClickDistance = 15
	}
	if c.TownPortal.AppearTimeoutMs == 0 {
		c.TownPortal.AppearTimeoutMs = 2000
	}
	if c.TownPortal.MaxClickDistance == 0 {
		c.TownPortal.MaxClickDistance = 15
	}
	if c.WaypointUI.BlackMarshX == 0 {
		c.WaypointUI.BlackMarshX = 200
	}
	if c.WaypointUI.BlackMarshY == 0 {
		c.WaypointUI.BlackMarshY = 342
	}
	if c.TownWalk.ForceMoveKey == "" {
		c.TownWalk.ForceMoveKey = "e"
	}
	if c.TownWalk.MoveIntervalMs == 0 {
		c.TownWalk.MoveIntervalMs = 650
	}
	if c.TownWalk.SettleTimeoutMs == 0 {
		c.TownWalk.SettleTimeoutMs = 350
	}
	if c.TownWalk.StuckTimeoutMs == 0 {
		c.TownWalk.StuckTimeoutMs = 3500
	}
	if c.TownWalk.ArrivalDistance == 0 {
		c.TownWalk.ArrivalDistance = 8
	}
}
