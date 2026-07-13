package pathing

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
	"gopkg.in/yaml.v3"
)

// TownRouteFile stores a recorded town route as world-coordinate samples.
type TownRouteFile struct {
	ID                string           `yaml:"id"`
	AreaID            uint32           `yaml:"area_id"`
	CreatedAt         time.Time        `yaml:"created_at"`
	SampleDistance    float64          `yaml:"sample_distance_tiles"`
	LayoutFingerprint string           `yaml:"layout_fingerprint,omitempty"`
	LayoutOriginX     uint32           `yaml:"layout_origin_x,omitempty"`
	LayoutOriginY     uint32           `yaml:"layout_origin_y,omitempty"`
	Points            []TownRoutePoint `yaml:"points"`
}

// TownRoutePoint is a serializable world-coordinate point.
type TownRoutePoint struct {
	X uint32 `yaml:"x"`
	Y uint32 `yaml:"y"`
}

// LoadNamedTownRoute loads and validates a recorded Act-1 route with the expected stable ID.
func LoadNamedTownRoute(path, expectedID string) ([]world.Position, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var route TownRouteFile
	if err := yaml.Unmarshal(data, &route); err != nil {
		return nil, fmt.Errorf("parse town route %q: %w", path, err)
	}
	return route.positions(expectedID)
}

// LoadLayoutBoundTownRoute loads an edge only when its Town layout matches exactly.
// Points are translated solely by the Stash-origin delta; the fingerprint still
// selects the rolled preset, while translation handles different absolute Town
// coordinate spaces across characters and difficulties.
func LoadLayoutBoundTownRoute(path, expectedID, expectedLayout string, currentOrigin world.Position) ([]world.Position, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var route TownRouteFile
	if err := yaml.Unmarshal(data, &route); err != nil {
		return nil, fmt.Errorf("parse town route %q: %w", path, err)
	}
	if len(expectedLayout) != 64 || route.LayoutFingerprint != expectedLayout {
		return nil, fmt.Errorf("town layout fingerprint mismatch: got %q, want %q", route.LayoutFingerprint, expectedLayout)
	}
	points, err := route.positions(expectedID)
	if err != nil {
		return nil, err
	}
	if route.LayoutOriginX == 0 || route.LayoutOriginY == 0 || currentOrigin.X == 0 || currentOrigin.Y == 0 {
		return nil, fmt.Errorf("layout-bound town route requires recorded and current Stash origins")
	}
	dx := int64(currentOrigin.X) - int64(route.LayoutOriginX)
	dy := int64(currentOrigin.Y) - int64(route.LayoutOriginY)
	for i := range points {
		x, y := int64(points[i].X)+dx, int64(points[i].Y)+dy
		if x <= 0 || y <= 0 {
			return nil, fmt.Errorf("translated town route point %d is invalid", i)
		}
		points[i] = world.Position{X: uint32(x), Y: uint32(y)}
	}
	return points, nil
}

// SaveNamedTownRoute writes a generic, stable-ID Act-1 anchor-edge recording.
func SaveNamedTownRoute(path, id string, sampleDistance float64, points []world.Position) error {
	return saveNamedTownRoute(path, id, "", world.Position{}, sampleDistance, points)
}

// SaveLayoutBoundTownRoute writes a graph edge bound to one exact Town layout.
// The Stash origin is stored with the fingerprint so later playback can
// translate the same preset without weakening layout identity.
func SaveLayoutBoundTownRoute(path, id, layout string, origin world.Position, sampleDistance float64, points []world.Position) error {
	if len(layout) != 64 {
		return fmt.Errorf("town layout fingerprint must be a SHA-256 hex string")
	}
	if origin.X == 0 || origin.Y == 0 {
		return fmt.Errorf("town layout Stash origin is required")
	}
	return saveNamedTownRoute(path, id, layout, origin, sampleDistance, points)
}

func saveNamedTownRoute(path, id, layout string, origin world.Position, sampleDistance float64, points []world.Position) error {
	route := TownRouteFile{
		ID:                id,
		AreaID:            uint32(world.RogueEncampment),
		CreatedAt:         time.Now().UTC(),
		SampleDistance:    sampleDistance,
		LayoutFingerprint: layout,
		LayoutOriginX:     origin.X,
		LayoutOriginY:     origin.Y,
		Points:            make([]TownRoutePoint, len(points)),
	}
	for i, p := range points {
		route.Points[i] = TownRoutePoint{X: p.X, Y: p.Y}
	}
	if _, err := route.positions(id); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create route dir: %w", err)
	}
	data, err := yaml.Marshal(route)
	if err != nil {
		return fmt.Errorf("marshal town route: %w", err)
	}
	return os.WriteFile(path, data, 0o644)
}

func (r TownRouteFile) positions(expectedID string) ([]world.Position, error) {
	if strings.TrimSpace(expectedID) == "" || r.ID != expectedID {
		return nil, fmt.Errorf("town route id = %q, want %q", r.ID, expectedID)
	}
	if world.AreaID(r.AreaID) != world.RogueEncampment {
		return nil, fmt.Errorf("town route area_id = %d, want %d", r.AreaID, world.RogueEncampment)
	}
	if len(r.Points) < 2 {
		return nil, fmt.Errorf("town route requires at least 2 points")
	}
	out := make([]world.Position, len(r.Points))
	for i, p := range r.Points {
		if p.X == 0 || p.Y == 0 {
			return nil, fmt.Errorf("town route point %d has zero coordinate", i)
		}
		out[i] = world.Position{X: p.X, Y: p.Y}
	}
	return out, nil
}
