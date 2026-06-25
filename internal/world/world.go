package world

import (
	"log/slog"
	"time"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/memory"
)

// Model holds the interpreted game state and the latest mapped snapshot.
type Model struct {
	log     *slog.Logger
	current State
}

// NewModel constructs a world model ready to receive snapshot updates.
func NewModel(log *slog.Logger) *Model {
	return &Model{log: log.With("component", "world")}
}

// Ready reports whether the world model is initialized.
func (m *Model) Ready() bool {
	m.log.Debug("world model initialized; state populated via Update")
	return true
}

// Update maps snap into world state, stores it as the current tick, and returns that state.
func (m *Model) Update(snap memory.Snapshot) State {
	m.current = FromSnapshot(snap)
	return m.current
}

// Current returns a value copy of the last state produced by Update.
// Before the first Update call the zero State is returned.
func (m *Model) Current() State {
	return m.current
}

// Reset sets current to an invalid state with the given reason and returns it.
// Area and Player are zero values; Reason is stored unchanged.
func (m *Model) Reset(at time.Time, reason string) State {
	m.current = State{
		At:     at,
		Valid:  false,
		Reason: reason,
		Phase:  GamePhaseUnknown,
	}
	return m.current
}
