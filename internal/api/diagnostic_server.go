package api

import (
	"net/http"
	"strings"

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
		s.writeError(w, http.StatusNotImplemented, string(app.Phase15ReasonDiagnosticBundleFailed), "Diagnosepakete sind nicht verfügbar.", requestIDFrom(r), nil)
		return
	}
	var request DiagnosticBundleRequest
	if !s.decodeBody(w, r, &request) {
		return
	}
	result, err := backend.CreateDiagnosticBundle(request)
	if err != nil {
		code := app.Phase15ReasonDiagnosticBundleFailed
		if strings.Contains(err.Error(), string(app.Phase15ReasonDiagnosticContentRejected)) {
			code = app.Phase15ReasonDiagnosticContentRejected
		}
		s.writeError(w, http.StatusConflict, string(code), "Das lokale Diagnosepaket konnte nicht sicher erstellt werden.", requestIDFrom(r), nil)
		return
	}
	s.writeJSON(w, http.StatusCreated, result)
}
