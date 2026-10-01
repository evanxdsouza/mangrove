package api

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
)

type setSelfHealRequest struct {
	Enabled bool `json:"enabled"`
}

// setDeploymentSelfHeal is the per-deployment self-healing opt-in: a
// service whose health check fails repeatedly gets restarted automatically
// (internal/scheduler/health.go), and a deploy that failed outright gets
// retried with backoff, up to a small cap (internal/scheduler/healer.go).
func (s *Server) setDeploymentSelfHeal(w http.ResponseWriter, r *http.Request) {
	deploymentID, err := parseID(chi.URLParam(r, "deploymentID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid deployment id")
		return
	}
	var req setSelfHealRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	if err := s.Store.SetSelfHealEnabled(r.Context(), deploymentID, req.Enabled); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.auditCtxWorkspace(r.Context(), "set_self_heal", "deployment", deploymentID, fmt.Sprintf("enabled=%t", req.Enabled))
	w.WriteHeader(http.StatusNoContent)
}
