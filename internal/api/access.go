package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

type setAccessRequest struct {
	IsPublic          bool   `json:"is_public"`
	PasswordProtected bool   `json:"password_protected"`
	Password          string `json:"password"`
}

// setDeploymentAccess is the per-deployment public/internal-only and
// password-protection toggle -- enforced at the Caddy proxy layer,
// independent of whatever auth the deployed app itself has. Applies
// immediately to a running deployment, not just on the next deploy.
func (s *Server) setDeploymentAccess(w http.ResponseWriter, r *http.Request) {
	deploymentID, err := parseID(chi.URLParam(r, "deploymentID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid deployment id")
		return
	}
	var req setAccessRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	if err := s.Orchestrator.SetAccessControl(r.Context(), deploymentID, req.IsPublic, req.PasswordProtected, req.Password); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
