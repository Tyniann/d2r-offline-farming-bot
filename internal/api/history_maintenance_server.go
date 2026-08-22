package api

import (
	"net/http"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/telemetry"
)

func (s *Server) historyMaintenanceBackend(w http.ResponseWriter, r *http.Request) (historyMaintenanceBackend, bool) {
	backend, ok := s.backend.(historyMaintenanceBackend)
	if !ok {
		s.writeError(w, http.StatusNotImplemented, string(telemetry.HistoryReasonUnavailable), requestIDFrom(r), nil)
	}
	return backend, ok
}

func (s *Server) handleHistoryDeleteAllPreview(w http.ResponseWriter, r *http.Request) {
	if !requireJSONMutation(w, r, s, http.MethodPost) {
		return
	}
	backend, ok := s.historyMaintenanceBackend(w, r)
	if !ok {
		return
	}
	var request HistoryDeletePreviewRequest
	if !s.decodeBody(w, r, &request) {
		return
	}
	response, err := backend.PreviewHistoryDeleteAll(request)
	if err != nil {
		s.writeHistoryBackendError(w, r, err)
		return
	}
	s.writeJSON(w, http.StatusOK, response)
}

func (s *Server) handleHistoryDeleteAllConfirm(w http.ResponseWriter, r *http.Request) {
	if !requireJSONMutation(w, r, s, http.MethodPost) {
		return
	}
	backend, ok := s.historyMaintenanceBackend(w, r)
	if !ok {
		return
	}
	var request HistoryDeleteConfirmRequest
	if !s.decodeBody(w, r, &request) {
		return
	}
	response, err := backend.ConfirmHistoryDeleteAll(request)
	if err != nil {
		s.writeHistoryBackendError(w, r, err)
		return
	}
	s.writeJSON(w, http.StatusOK, response)
}
