package tasks

import "log/slog"

// Runner executes high-level run state machines.
type Runner struct {
	log *slog.Logger
}

func NewRunner(log *slog.Logger) *Runner {
	return &Runner{log: log.With("component", "tasks")}
}

func (r *Runner) Ready() bool {
	r.log.Debug("task runner placeholder ready")
	return true
}
