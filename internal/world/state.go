package world

import (
	"time"
)

// GamePhase describes high-level client state for a tick.
// The zero value is GamePhaseUnknown.
type GamePhase int

// GamePhase values for menu, loading, and in-game states.
const (
	GamePhaseUnknown GamePhase = iota
	GamePhaseMenu
	GamePhaseLoading
	GamePhaseInGame
)

// String returns a stable label for structured logging.
func (p GamePhase) String() string {
	switch p {
	case GamePhaseMenu:
		return "menu"
	case GamePhaseLoading:
		return "loading"
	case GamePhaseInGame:
		return "in_game"
	default:
		return "unknown"
	}
}

// State is an immutable per-tick world snapshot.
// When Valid is false, Reason may be set and Area/Player are zero values that must not be read.
type State struct {
	At        time.Time // Snapshot timestamp.
	Phase     GamePhase // High-level client phase from memory probe.
	Valid     bool      // False when Area/Player are zero values.
	Reason    string    // Short invalid reason when Valid is false; may be empty.
	Area      Area      // Resolved area when Valid is true.
	Player    Player    // Main-player vitals when Valid is true.
	Objects   []Object
	Entrances []Entrance
	Monsters  []Monster
	Items     []Item
	Hover     HoverInfo // Unit currently under the mouse cursor; zero value when none.
	UI        UIState   // Read-only menu flags used for fail-closed UI actions.
}
