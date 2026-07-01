package pathing

import (
	"math"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/input"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

// ProjectionMode identifies how world tiles are mapped to client pixels.
type ProjectionMode string

// ProjectionRelative uses player-centered isometric deltas (Koolo-style).
// Precision for entity clicks comes from the hover feedback loop, not from the projection.
const ProjectionRelative ProjectionMode = "relative"

// Projector maps world tile positions to client pixels within the bound window.
type Projector interface {
	Project(player, target world.Position, win input.WindowInfo) (clientX, clientY int, ok bool)
	Mode() ProjectionMode
}

// RelativeProjector treats the player as the playable-area center and projects
// isometric screen offsets from signed tile deltas (target − player).
type RelativeProjector struct {
	PlayableCenterX float64 // Fraction 0..1 of client width; default 0.5.
	PlayableCenterY float64 // Fraction 0..1 of client height; default ~0.52 for UI offset.
	TileWidth       float64 // Horizontal isometric scale; calibrate via pathing-test.
	TileHeight      float64 // Vertical isometric scale; typically TileWidth / 2.
}

// DefaultRelativeProjector returns RelativeProjector with Phase 4.3 calibration defaults.
// TileWidth/TileHeight 19.8/9.9 match the Koolo reference for 1280×720 windowed mode.
func DefaultRelativeProjector() RelativeProjector {
	return RelativeProjector{
		PlayableCenterX: 0.5,
		PlayableCenterY: 0.52,
		TileWidth:       19.8,
		TileHeight:      9.9,
	}
}

// Mode reports ProjectionRelative.
func (RelativeProjector) Mode() ProjectionMode {
	return ProjectionRelative
}

// Project maps target relative to player using isometric deltas.
// Tile deltas use signed int32 subtraction to avoid uint32 wrap at large coordinates.
func (p RelativeProjector) Project(player, target world.Position, win input.WindowInfo) (int, int, bool) {
	if win.ClientWidth <= 0 || win.ClientHeight <= 0 {
		return 0, 0, false
	}
	if p.TileWidth <= 0 || p.TileHeight <= 0 {
		return 0, 0, false
	}
	if p.PlayableCenterX < 0 || p.PlayableCenterX > 1 || p.PlayableCenterY < 0 || p.PlayableCenterY > 1 {
		return 0, 0, false
	}

	dx, dy := tileDelta(player, target)
	centerX := float64(win.ClientWidth) * p.PlayableCenterX
	centerY := float64(win.ClientHeight) * p.PlayableCenterY

	clientX := centerX + float64(dx-dy)*p.TileWidth
	clientY := centerY + float64(dx+dy)*p.TileHeight

	return int(math.Round(clientX)), int(math.Round(clientY)), true
}

// tileDelta returns signed tile offsets from player to target.
func tileDelta(player, target world.Position) (dx, dy int32) {
	return int32(target.X) - int32(player.X), int32(target.Y) - int32(player.Y)
}
