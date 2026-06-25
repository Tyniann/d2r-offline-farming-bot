package process

import "log/slog"

// Service finds and manages the D2R game process.
type Service struct {
	log         *slog.Logger
	processName string
}

func New(log *slog.Logger, processName string) *Service {
	return &Service{
		log:         log.With("component", "process"),
		processName: processName,
	}
}

// Ready reports whether the service is initialized for future use.
func (s *Service) Ready() bool {
	s.log.Debug("process service placeholder ready", "target", s.processName)
	return true
}
