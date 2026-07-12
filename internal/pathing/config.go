package pathing

import (
	"fmt"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

// Config holds tuning parameters for the navigator, projection, entity clicks,
// and exploration. Use [DefaultConfig] and overlay YAML values via internal/config.
type Config struct {
	// StuckTimeout aborts navigation when no progress happened for this duration.
	StuckTimeout time.Duration
	// StuckProgressTiles is the minimum position delta that counts as progress.
	StuckProgressTiles float64
	// MoveInterval is the minimum delay between teleport casts.
	MoveInterval time.Duration
	// ArrivalDistance is the tile distance that satisfies position goals.
	ArrivalDistance float64

	Projection ProjectionConfig
	Click      ClickConfig
	Explore    ExploreConfig
	Waypoint   WaypointConfig
	TownPortal TownPortalConfig
	WaypointUI WaypointUIConfig
	TownWalk   TownWalkConfig
}

// ProjectionConfig calibrates the relative (player-centered) projection.
type ProjectionConfig struct {
	PlayableCenterX float64 // Fraction 0..1 of client width; default 0.5.
	PlayableCenterY float64 // Fraction 0..1 of client height; default 0.52.
	TileWidth       float64 // Horizontal isometric scale; default 19.8 (1280×720).
	TileHeight      float64 // Vertical isometric scale; default 9.9.
}

// ClickConfig tunes the hover-feedback click loop.
type ClickConfig struct {
	// MaxHoverAttempts is how many mouse positions are tried before hover_not_found.
	MaxHoverAttempts int
	// SpiralStepDegrees is the spiral angle advance per attempt (Koolo spiral).
	SpiralStepDegrees float64
	// AnchorOffsetTiles shifts the click point from the ground tile toward the
	// visible body of entrances/objects (Koolo uses ~2 tiles).
	AnchorOffsetTiles float64
}

// ExploreConfig tunes bearing-based layout exploration.
type ExploreConfig struct {
	// BearingCount is the number of compass directions to rotate through.
	BearingCount int
	// StepDistanceTiles is the teleport step length per explore move.
	StepDistanceTiles float64
	// MaxEntranceClickDistance gates the switch from bearing explore to the
	// entity click loop; the entity must be visible on screen (Koolo: 10–15).
	MaxEntranceClickDistance float64
}

// WaypointConfig tunes hover-confirmed waypoint object actions.
type WaypointConfig struct {
	// MaxClickDistance gates waypoint object clicks; the object must be visible.
	MaxClickDistance float64
}

// TownPortalConfig tunes player-cast portal discovery and hover-confirmed entry.
type TownPortalConfig struct {
	// AppearTimeout bounds how long the cast portal may remain absent.
	AppearTimeout time.Duration
	// MaxClickDistance gates portal clicks by tile distance.
	MaxClickDistance float64
}

// WaypointUIConfig holds fixed client-relative coordinates in the waypoint menu.
type WaypointUIConfig struct {
	// BlackMarshX is the client X coordinate for the Black Marsh waypoint row.
	BlackMarshX int
	// BlackMarshY is the client Y coordinate for the Black Marsh waypoint row.
	BlackMarshY int
}

// TownWalkConfig tunes force-move walking inside Rogue Encampment.
type TownWalkConfig struct {
	// RouteFile is the optional recorded override path.
	RouteFile string
	// ForceMoveKey is the in-game Force Move key, usually "e".
	ForceMoveKey string
	// MoveInterval is the minimum delay between Force Move clicks.
	MoveInterval time.Duration
	// SettleTimeout is the grace period after a move input before stuck checks.
	SettleTimeout time.Duration
	// StuckTimeout aborts when the player makes no progress for this duration.
	StuckTimeout time.Duration
	// ArrivalDistance is the tile distance that satisfies a route point.
	ArrivalDistance float64
	// Act1WaypointPoints optionally overrides the built-in Act-1 preset route.
	Act1WaypointPoints []world.Position
}

// DefaultConfig returns Phase 4.3 defaults matching configs/config.example.yaml.
func DefaultConfig() Config {
	return Config{
		StuckTimeout:       8000 * time.Millisecond,
		StuckProgressTiles: 3,
		MoveInterval:       250 * time.Millisecond,
		ArrivalDistance:    15,
		Projection: ProjectionConfig{
			PlayableCenterX: 0.5,
			PlayableCenterY: 0.52,
			TileWidth:       19.8,
			TileHeight:      9.9,
		},
		Click: ClickConfig{
			MaxHoverAttempts:  15,
			SpiralStepDegrees: 40,
			AnchorOffsetTiles: 2,
		},
		Explore: ExploreConfig{
			BearingCount:             8,
			StepDistanceTiles:        25,
			MaxEntranceClickDistance: 15,
		},
		Waypoint: WaypointConfig{
			MaxClickDistance: 15,
		},
		TownPortal: TownPortalConfig{
			AppearTimeout:    2 * time.Second,
			MaxClickDistance: 15,
		},
		WaypointUI: WaypointUIConfig{
			BlackMarshX: 200,
			BlackMarshY: 342,
		},
		TownWalk: TownWalkConfig{
			RouteFile:       "configs/routes/town/act1/waypoint/normal.yaml",
			ForceMoveKey:    "e",
			MoveInterval:    650 * time.Millisecond,
			SettleTimeout:   350 * time.Millisecond,
			StuckTimeout:    3500 * time.Millisecond,
			ArrivalDistance: 8,
		},
	}
}

// Validate reports configuration errors that would break navigation.
func (c Config) Validate() error {
	if c.StuckTimeout <= 0 {
		return fmt.Errorf("pathing.stuck_timeout_ms must be > 0")
	}
	if c.StuckProgressTiles <= 0 {
		return fmt.Errorf("pathing.stuck_progress_tiles must be > 0")
	}
	if c.MoveInterval < 0 {
		return fmt.Errorf("pathing.move_interval_ms must be >= 0")
	}
	if c.ArrivalDistance <= 0 {
		return fmt.Errorf("pathing.arrival_distance must be > 0")
	}
	if c.Projection.TileWidth <= 0 || c.Projection.TileHeight <= 0 {
		return fmt.Errorf("pathing.projection.tile_width and tile_height must be > 0")
	}
	if c.Projection.PlayableCenterX < 0 || c.Projection.PlayableCenterX > 1 ||
		c.Projection.PlayableCenterY < 0 || c.Projection.PlayableCenterY > 1 {
		return fmt.Errorf("pathing.projection.playable_center_x/y must be within 0..1")
	}
	if c.Click.MaxHoverAttempts <= 0 {
		return fmt.Errorf("pathing.click.max_hover_attempts must be > 0")
	}
	if c.Click.SpiralStepDegrees <= 0 {
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
	if c.TownPortal.AppearTimeout <= 0 {
		return fmt.Errorf("pathing.town_portal.appear_timeout_ms must be > 0")
	}
	if c.TownPortal.MaxClickDistance <= 0 {
		return fmt.Errorf("pathing.town_portal.max_click_distance must be > 0")
	}
	if c.WaypointUI.BlackMarshX < 0 || c.WaypointUI.BlackMarshY < 0 {
		return fmt.Errorf("pathing.waypoint_ui.black_marsh_x/y must be >= 0")
	}
	if c.TownWalk.RouteFile == "" {
		return fmt.Errorf("pathing.town_walk.route_file is required")
	}
	if c.TownWalk.ForceMoveKey == "" {
		return fmt.Errorf("pathing.town_walk.force_move_key is required")
	}
	if c.TownWalk.MoveInterval <= 0 {
		return fmt.Errorf("pathing.town_walk.move_interval_ms must be > 0")
	}
	if c.TownWalk.SettleTimeout <= 0 {
		return fmt.Errorf("pathing.town_walk.settle_timeout_ms must be > 0")
	}
	if c.TownWalk.StuckTimeout <= 0 {
		return fmt.Errorf("pathing.town_walk.stuck_timeout_ms must be > 0")
	}
	if c.TownWalk.ArrivalDistance <= 0 {
		return fmt.Errorf("pathing.town_walk.arrival_distance_tiles must be > 0")
	}
	return nil
}

// Projector builds the relative projector from the projection calibration.
func (c Config) Projector() RelativeProjector {
	return RelativeProjector{
		PlayableCenterX: c.Projection.PlayableCenterX,
		PlayableCenterY: c.Projection.PlayableCenterY,
		TileWidth:       c.Projection.TileWidth,
		TileHeight:      c.Projection.TileHeight,
	}
}
