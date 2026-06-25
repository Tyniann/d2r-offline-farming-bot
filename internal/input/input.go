package input

import "log/slog"

// Controller will send keyboard and mouse actions to the game window.
type Controller struct {
	log *slog.Logger
}

func NewController(log *slog.Logger) *Controller {
	return &Controller{log: log.With("component", "input")}
}

func (c *Controller) Ready() bool {
	c.log.Debug("input controller placeholder ready")
	return true
}
