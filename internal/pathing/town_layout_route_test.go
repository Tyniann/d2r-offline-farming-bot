package pathing

import (
	"path/filepath"
	"testing"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

func TestLayoutBoundTownRouteRequiresExactFingerprint(t *testing.T) {
	const layout = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	path := filepath.Join(t.TempDir(), "edge.yaml")
	if err := SaveLayoutBoundTownRoute(path, "edge", layout, world.Position{X: 100, Y: 100}, 8, []world.Position{{X: 100, Y: 100}, {X: 110, Y: 90}}); err != nil {
		t.Fatal(err)
	}
	translated, err := LoadLayoutBoundTownRoute(path, "edge", layout, world.Position{X: 500, Y: 400})
	if err != nil {
		t.Fatal(err)
	}
	if translated[0] != (world.Position{X: 500, Y: 400}) || translated[1] != (world.Position{X: 510, Y: 390}) {
		t.Fatalf("translated=%+v", translated)
	}
	const other = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	if _, err := LoadLayoutBoundTownRoute(path, "edge", other, world.Position{X: 100, Y: 100}); err == nil {
		t.Fatal("mismatched layout accepted")
	}
	legacy := filepath.Join(t.TempDir(), "legacy.yaml")
	if err := SaveNamedTownRoute(legacy, "edge", 8, []world.Position{{X: 1, Y: 1}, {X: 2, Y: 2}}); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadLayoutBoundTownRoute(legacy, "edge", layout, world.Position{X: 100, Y: 100}); err == nil {
		t.Fatal("unbound legacy route accepted")
	}
}
