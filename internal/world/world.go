package world

import "log/slog"

// Model represents the interpreted game state (areas, entities, items).
type Model struct {
	log *slog.Logger
}

func NewModel(log *slog.Logger) *Model {
	return &Model{log: log.With("component", "world")}
}

func (m *Model) Ready() bool {
	m.log.Debug("world model placeholder ready")
	return true
}
