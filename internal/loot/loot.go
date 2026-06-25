package loot

import "log/slog"

// Filter applies pickit rules and inventory/stash logic.
type Filter struct {
	log *slog.Logger
}

func NewFilter(log *slog.Logger) *Filter {
	return &Filter{log: log.With("component", "loot")}
}

func (f *Filter) Ready() bool {
	f.log.Debug("loot filter placeholder ready")
	return true
}
