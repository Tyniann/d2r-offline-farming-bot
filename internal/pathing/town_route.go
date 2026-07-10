package pathing

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
	"gopkg.in/yaml.v3"
)

const act1WaypointRouteID = "act1-town-waypoint"

// TownRouteFile stores a recorded town route as world-coordinate samples.
type TownRouteFile struct {
	ID             string           `yaml:"id"`
	AreaID         uint32           `yaml:"area_id"`
	CreatedAt      time.Time        `yaml:"created_at"`
	SampleDistance float64          `yaml:"sample_distance_tiles"`
	Points         []TownRoutePoint `yaml:"points"`
}

// TownRoutePoint is a serializable world-coordinate point.
type TownRoutePoint struct {
	X uint32 `yaml:"x"`
	Y uint32 `yaml:"y"`
}

// LoadTownRoute loads and validates a recorded town route.
func LoadTownRoute(path string) ([]world.Position, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var route TownRouteFile
	if err := yaml.Unmarshal(data, &route); err != nil {
		return nil, fmt.Errorf("parse town route %q: %w", path, err)
	}
	return route.positions()
}

// SaveTownRoute writes a recorded town route.
func SaveTownRoute(path string, sampleDistance float64, points []world.Position) error {
	route := TownRouteFile{
		ID:             act1WaypointRouteID,
		AreaID:         uint32(world.RogueEncampment),
		CreatedAt:      time.Now().UTC(),
		SampleDistance: sampleDistance,
		Points:         make([]TownRoutePoint, len(points)),
	}
	for i, p := range points {
		route.Points[i] = TownRoutePoint{X: p.X, Y: p.Y}
	}
	if _, err := route.positions(); err != nil {
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

func (r TownRouteFile) positions() ([]world.Position, error) {
	if r.ID != act1WaypointRouteID {
		return nil, fmt.Errorf("town route id = %q, want %q", r.ID, act1WaypointRouteID)
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
