package pathing

import (
	"testing"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/input"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

func TestRelativeProjector_PlayerAtCenter(t *testing.T) {
	p := DefaultRelativeProjector()
	win := input.WindowInfo{ClientWidth: 1280, ClientHeight: 720}
	player := world.Position{X: 5000, Y: 5000}

	x, y, ok := p.Project(player, player, win)
	if !ok {
		t.Fatal("Project() ok = false, want true")
	}
	wantX := int(1280 * p.PlayableCenterX)
	wantY := int(720 * p.PlayableCenterY)
	if x != wantX || y != wantY {
		t.Errorf("Project(same) = (%d,%d), want (%d,%d)", x, y, wantX, wantY)
	}
}

func TestRelativeProjector_IsometricEast(t *testing.T) {
	p := DefaultRelativeProjector()
	win := input.WindowInfo{ClientWidth: 1280, ClientHeight: 720}
	player := world.Position{X: 1000, Y: 1000}
	target := world.Position{X: 1010, Y: 1000} // +10 tiles east

	cx, cy, ok := p.Project(player, target, win)
	if !ok {
		t.Fatal("Project() ok = false, want true")
	}
	baseX, baseY, _ := p.Project(player, player, win)
	dx := cx - baseX
	dy := cy - baseY

	// East: screen moves right (+X) and down (+Y) in isometric view.
	if dx <= 0 {
		t.Errorf("east delta clientX = %d, want > 0", dx)
	}
	if dy <= 0 {
		t.Errorf("east delta clientY = %d, want > 0", dy)
	}
	wantDX := int(10 * p.TileWidth)
	if dx != wantDX {
		t.Errorf("east clientX delta = %d, want %d", dx, wantDX)
	}
	wantDY := int(10 * p.TileHeight)
	if dy != wantDY {
		t.Errorf("east clientY delta = %d, want %d", dy, wantDY)
	}
}

func TestRelativeProjector_TileDeltaInt32(t *testing.T) {
	p := DefaultRelativeProjector()
	win := input.WindowInfo{ClientWidth: 800, ClientHeight: 600}

	// Positions near uint16-style boundaries: signed delta must not wrap via uint32.
	player := world.Position{X: 65530, Y: 100}
	target := world.Position{X: 65540, Y: 100}

	dx, dy := tileDelta(player, target)
	if dx != 10 || dy != 0 {
		t.Fatalf("tileDelta() = (%d,%d), want (10,0)", dx, dy)
	}

	_, _, ok := p.Project(player, target, win)
	if !ok {
		t.Fatal("Project() ok = false, want true")
	}
}

func TestRelativeProjector_InvalidWindow(t *testing.T) {
	p := DefaultRelativeProjector()
	player := world.Position{X: 1, Y: 1}

	_, _, ok := p.Project(player, player, input.WindowInfo{})
	if ok {
		t.Error("Project() ok = true, want false for zero client size")
	}
}

func TestRelativeProjector_InvalidScale(t *testing.T) {
	p := RelativeProjector{
		PlayableCenterX: 0.5,
		PlayableCenterY: 0.5,
		TileWidth:       0,
		TileHeight:      9.9,
	}
	win := input.WindowInfo{ClientWidth: 800, ClientHeight: 600}
	player := world.Position{X: 1, Y: 1}

	_, _, ok := p.Project(player, player, win)
	if ok {
		t.Error("Project() ok = true, want false for zero TileWidth")
	}
}

func TestRelativeProjector_Mode(t *testing.T) {
	if DefaultRelativeProjector().Mode() != ProjectionRelative {
		t.Errorf("Mode() = %q, want %q", DefaultRelativeProjector().Mode(), ProjectionRelative)
	}
}
