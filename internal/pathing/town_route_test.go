package pathing

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

func TestSaveLoadTownRoute(t *testing.T) {
	path := filepath.Join(t.TempDir(), "routes", "act1.yaml")
	points := []world.Position{{X: 1, Y: 2}, {X: 3, Y: 4}}
	if err := SaveTownRoute(path, 8, points); err != nil {
		t.Fatal(err)
	}
	got, err := LoadTownRoute(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(points) || got[0] != points[0] || got[1] != points[1] {
		t.Fatalf("points = %v, want %v", got, points)
	}
}

func TestLoadTownRouteRejectsInvalid(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"wrong id", "id: other\narea_id: 1\npoints:\n  - {x: 1, y: 2}\n  - {x: 3, y: 4}\n"},
		{"wrong area", "id: act1-town-waypoint\narea_id: 6\npoints:\n  - {x: 1, y: 2}\n  - {x: 3, y: 4}\n"},
		{"too few points", "id: act1-town-waypoint\narea_id: 1\npoints:\n  - {x: 1, y: 2}\n"},
		{"zero coordinate", "id: act1-town-waypoint\narea_id: 1\npoints:\n  - {x: 0, y: 2}\n  - {x: 3, y: 4}\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "route.yaml")
			if err := os.WriteFile(path, []byte(tc.body), 0o644); err != nil {
				t.Fatal(err)
			}
			if _, err := LoadTownRoute(path); err == nil {
				t.Fatal("expected load error")
			}
		})
	}
}
