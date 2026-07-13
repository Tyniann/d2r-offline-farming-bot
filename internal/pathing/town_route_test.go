package pathing

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

func TestSaveLoadNamedTownRoute(t *testing.T) {
	path := filepath.Join(t.TempDir(), "portal-cain.yaml")
	points := []world.Position{{X: 100, Y: 200}, {X: 110, Y: 210}}
	if err := SaveNamedTownRoute(path, "portal-cain", 8, points); err != nil {
		t.Fatal(err)
	}
	got, err := LoadNamedTownRoute(path, "portal-cain")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[1] != points[1] {
		t.Fatalf("points = %v", got)
	}
	if _, err := LoadNamedTownRoute(path, "other"); err == nil {
		t.Fatal("wrong expected edge id accepted")
	}
}

func TestLoadTownRouteRejectsInvalid(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"wrong id", "id: other\narea_id: 1\npoints:\n  - {x: 1, y: 2}\n  - {x: 3, y: 4}\n"},
		{"wrong area", "id: edge\narea_id: 6\npoints:\n  - {x: 1, y: 2}\n  - {x: 3, y: 4}\n"},
		{"too few points", "id: edge\narea_id: 1\npoints:\n  - {x: 1, y: 2}\n"},
		{"zero coordinate", "id: edge\narea_id: 1\npoints:\n  - {x: 0, y: 2}\n  - {x: 3, y: 4}\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "route.yaml")
			if err := os.WriteFile(path, []byte(tc.body), 0o644); err != nil {
				t.Fatal(err)
			}
			if _, err := LoadNamedTownRoute(path, "edge"); err == nil {
				t.Fatal("expected load error")
			}
		})
	}
}
