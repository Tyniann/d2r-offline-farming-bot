package memory

import "log/slog"

// Reader will read and snapshot game memory regions.
type Reader struct {
	log *slog.Logger
}

func NewReader(log *slog.Logger) *Reader {
	return &Reader{log: log.With("component", "memory")}
}

func (r *Reader) Ready() bool {
	r.log.Debug("memory reader placeholder ready")
	return true
}
