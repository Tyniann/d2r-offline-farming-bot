package pathing

import "log/slog"

// Navigator will handle teleport and movement logic.
type Navigator struct {
	log *slog.Logger
}

func NewNavigator(log *slog.Logger) *Navigator {
	return &Navigator{log: log.With("component", "pathing")}
}

func (n *Navigator) Ready() bool {
	n.log.Debug("pathing navigator placeholder ready")
	return true
}
