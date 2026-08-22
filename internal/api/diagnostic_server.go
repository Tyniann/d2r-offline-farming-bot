package api

import (
	"errors"
	"net/http"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/app"
)

type diagnosticBackend interface {
	CreateDiagnosticBundle(DiagnosticBundleRequest) (DiagnosticBundleDTO, error)
}

func (s *Server) handleDiagnosticBundle(w http.ResponseWriter, r *http.Request) {
	if !requireJSONPost(w, r, s, true) {
		return
	}
	backend, ok := s.backend.(diagnosticBackend)
	if !ok {
		s.writeError(w, http.StatusNotImplemented, string(app.Phase15ReasonDiagnosticBundleFailed), requestIDFrom(r), nil)
		return
	}
	var request DiagnosticBundleRequest
	if !s.decodeBody(w, r, &request) {
		return
	}
	result, err := backend.CreateDiagnosticBundle(request)
	if err != nil {
		code := app.Phase15ReasonDiagnosticBundleFailed
		if errors.Is(err, app.ErrDiagnosticContentRejected) {
			code = app.Phase15ReasonDiagnosticContentRejected
		}
		s.logger.Warn("Diagnostic bundle request rejected", "code", code, "request_id", requestIDFrom(r), "error", err)
		s.writeError(w, http.StatusConflict, string(code), requestIDFrom(r), nil)
		return
	}
	s.writeJSON(w, http.StatusCreated, result)
}
