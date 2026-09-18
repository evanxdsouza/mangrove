package api

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
)

type setAccessRequest struct {
	IsPublic          bool   `json:"is_public"`
	PasswordProtected bool   `json:"password_protected"`
	Password          string `json:"password"`
}

// setDeploymentAccess is the per-deployment public/internal-only and
// password-protection toggle. Password protection is enforced in front of
// the deployed app entirely (a styled gate page offering the password or a
// Mangrove account sign-in, see internal/api/gate.go), independent of
// whatever auth the app itself has. Applies immediately to a running
// deployment, not just on the next deploy.
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
	// Never the password itself -- just what changed.
	s.auditCtxWorkspace(r.Context(), "set_access_control", "deployment", deploymentID,
		fmt.Sprintf("is_public=%t password_protected=%t", req.IsPublic, req.PasswordProtected))
	w.WriteHeader(http.StatusNoContent)
}
