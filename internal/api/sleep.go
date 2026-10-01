package api

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
)

type setSleepConfigRequest struct {
	Enabled     bool `json:"enabled"`
	IdleMinutes int  `json:"idle_minutes"`
}

// setDeploymentSleep is the per-deployment idle-sleep toggle: opt a
// deployment into auto-stopping after IdleMinutes with no traffic, auto-
// waking on its next visit. See internal/orchestrator/sleep.go,
// internal/scheduler/sleeper.go, and internal/api/gate.go's wake handler
// for the rest of the feature.
func (s *Server) setDeploymentSleep(w http.ResponseWriter, r *http.Request) {
	deploymentID, err := parseID(chi.URLParam(r, "deploymentID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid deployment id")
		return
	}
	var req setSleepConfigRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	if err := s.Orchestrator.SetSleepConfig(r.Context(), deploymentID, req.Enabled, req.IdleMinutes); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.auditCtxWorkspace(r.Context(), "set_sleep_config", "deployment", deploymentID,
		fmt.Sprintf("enabled=%t idle_minutes=%d", req.Enabled, req.IdleMinutes))
	w.WriteHeader(http.StatusNoContent)
}
