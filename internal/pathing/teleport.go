package pathing

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/input"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/memory"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

const maxTeleportClientYFraction = 0.74

// ErrProjectionFailed indicates the target could not be mapped to client pixels.
var ErrProjectionFailed = fmt.Errorf("projection failed")

// TeleportMover casts Teleport at projected world coordinates using the YAML
// skill binding (`input.bindings.skills.teleport`). Casts are throttled by the
// configured move interval so the game can process each teleport.
type TeleportMover struct {
	log       *slog.Logger
	input     InputDriver
	bindings  input.BindingSource
	projector Projector
	interval  time.Duration
	lastCast  time.Time
}

// NewTeleportMover wires teleport casting to input, bindings, and projection.
func NewTeleportMover(log *slog.Logger, in InputDriver, bindings input.BindingSource, projector Projector, interval time.Duration) *TeleportMover {
	return &TeleportMover{
		log:       log.With("component", "pathing.teleport"),
		input:     in,
		bindings:  bindings,
		projector: projector,
		interval:  interval,
	}
}

// Ready reports whether the move interval since the last cast has elapsed.
func (m *TeleportMover) Ready(now time.Time) bool {
	return m.lastCast.IsZero() || now.Sub(m.lastCast) >= m.interval
}

// Reset clears the cast throttle, e.g. when a new goal starts.
func (m *TeleportMover) Reset() {
	m.lastCast = time.Time{}
}

// TeleportTo projects target relative to player and casts Teleport there.
// Returns the client coordinates used for the cast. ErrProjectionFailed is
// returned when the window is unbound or projection is invalid.
func (m *TeleportMover) TeleportTo(now time.Time, player, target world.Position) (clientX, clientY int, err error) {
	win, ok := m.input.Window()
	if !ok {
		return 0, 0, fmt.Errorf("teleport: window not bound: %w", ErrProjectionFailed)
	}
	clientX, clientY, ok = m.projector.Project(player, target, win)
	if !ok {
		return 0, 0, fmt.Errorf("teleport to %d,%d: %w", target.X, target.Y, ErrProjectionFailed)
	}
	rawClientX, rawClientY := clientX, clientY
	clientX, clientY = clampTeleportClientPoint(clientX, clientY, win)

	if err := m.input.CastSkillAt(m.bindings, memory.SkillTeleport, clientX, clientY); err != nil {
		return clientX, clientY, fmt.Errorf("teleport cast: %w", err)
	}
	m.lastCast = now

	m.log.Debug("teleport cast",
		"player_x", player.X,
		"player_y", player.Y,
		"target_x", target.X,
		"target_y", target.Y,
		"client_x", clientX,
		"client_y", clientY,
		"raw_client_x", rawClientX,
		"raw_client_y", rawClientY,
	)
	return clientX, clientY, nil
}

func clampTeleportClientPoint(clientX, clientY int, win input.WindowInfo) (int, int) {
	if win.ClientWidth > 0 {
		clientX = clampInt(clientX, 0, win.ClientWidth-1)
	}
	if win.ClientHeight > 0 {
		maxY := int(float64(win.ClientHeight) * maxTeleportClientYFraction)
		clientY = clampInt(clientY, 0, maxY)
	}
	return clientX, clientY
}

func isPlayableClientPoint(clientX, clientY int, win input.WindowInfo) bool {
	if win.ClientWidth <= 0 || win.ClientHeight <= 0 {
		return false
	}
	if clientX < 0 || clientX >= win.ClientWidth {
		return false
	}
	maxY := int(float64(win.ClientHeight) * maxTeleportClientYFraction)
	return clientY >= 0 && clientY <= maxY
}

func clampInt(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
