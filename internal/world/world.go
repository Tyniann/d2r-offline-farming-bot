package world

import (
	"log/slog"
	"slices"
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

// Update maps snap into world state, stores a cloned copy, and returns an independent clone.
func (m *Model) Update(snap memory.Snapshot) State {
	m.current = cloneState(FromSnapshot(snap))
	return cloneState(m.current)
}

// Current returns a value copy of the last state produced by Update.
// Before the first Update call the zero State is returned.
func (m *Model) Current() State {
	return cloneState(m.current)
}

// Reset sets current to an invalid state with the given reason and returns it.
// Area and Player are zero values; Reason is stored unchanged.
func (m *Model) Reset(at time.Time, reason string) State {
	m.current = State{
		At:        at,
		Valid:     false,
		Reason:    reason,
		Phase:     GamePhaseUnknown,
		Objects:   make([]Object, 0),
		Entrances: make([]Entrance, 0),
		Monsters:  make([]Monster, 0),
	}
	return cloneState(m.current)
}

func cloneState(s State) State {
	s.Objects = slices.Clone(s.Objects)
	s.Entrances = slices.Clone(s.Entrances)
	s.Monsters = slices.Clone(s.Monsters)
	return s
}
