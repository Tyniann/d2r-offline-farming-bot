package pathing

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/input"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/memory"
	"github.com/Tyniann/d2r-offline-farming-bot/internal/world"
)

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
	)
	return clientX, clientY, nil
}
