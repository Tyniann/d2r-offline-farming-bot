package pathing

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// LoadRoute loads and validates one Route Contract v1 YAML file.
func LoadRoute(path string) (Route, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Route{}, fmt.Errorf("read route %q: %w", path, err)
	}
	var route Route
	if err := yaml.Unmarshal(data, &route); err != nil {
		return Route{}, fmt.Errorf("decode route %q: %w", path, err)
	}
	if err := route.Validate(); err != nil {
		return Route{}, fmt.Errorf("validate route %q: %w", path, err)
	}
	return route, nil
}

// SaveRoute validates and atomically publishes a new Route Contract v1 YAML file.
func SaveRoute(path string, route Route) error {
	if err := route.Validate(); err != nil {
		return fmt.Errorf("validate route before save: %w", err)
	}
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("publish route %q: file already exists", path)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect route target %q: %w", path, err)
	}
	data, err := yaml.Marshal(route)
	if err != nil {
		return fmt.Errorf("encode route %q: %w", route.ID, err)
	}
	dir := filepath.Dir(path)
	if mkdirErr := os.MkdirAll(dir, 0o755); mkdirErr != nil {
		return fmt.Errorf("create route directory %q: %w", dir, mkdirErr)
	}
	tmp, err := os.CreateTemp(dir, ".route-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary route in %q: %w", dir, err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write temporary route %q: %w", tmpPath, err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("sync temporary route %q: %w", tmpPath, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temporary route %q: %w", tmpPath, err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("publish route %q: %w", path, err)
	}
	return nil
}
