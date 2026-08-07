package api

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/evanxdsouza/mangrove/internal/store"
	"github.com/evanxdsouza/mangrove/internal/webhook"
)

type createPATRequest struct {
	Label string `json:"label"`
	Token string `json:"token"`
}

// createGithubPAT stores a Personal Access Token encrypted at rest. A PAT
// (rather than a GitHub App) is the deliberate v1 choice for this
// single-user deployment -- see the plan's GitHub integration section.
func (s *Server) createGithubPAT(w http.ResponseWriter, r *http.Request) {
	var req createPATRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if req.Label == "" || req.Token == "" {
		writeError(w, http.StatusBadRequest, "label and token are required")
		return
	}

	aad := []byte("github_pats:new:" + req.Label)
	ciphertext, nonce, err := s.Secrets.Seal(aad, []byte(req.Token))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "encrypt token: "+err.Error())
		return
	}
	id, err := s.Store.CreateGithubPAT(r.Context(), req.Label, ciphertext, nonce)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	// Re-seal with the real row ID bound as AAD (we didn't have it until
	// the insert above) so the ciphertext is bound to its actual row.
	ciphertext, nonce, err = s.Secrets.Seal(patAAD(id), []byte(req.Token))
	if err == nil {
		s.Store.DB.ExecContext(r.Context(), `UPDATE github_pats SET token_encrypted = ?, token_nonce = ? WHERE id = ?`, ciphertext, nonce, id)
	}

	writeJSON(w, http.StatusCreated, map[string]any{"id": id, "label": req.Label})
}

func patAAD(id int64) []byte {
	return []byte("github_pats:" + strconv.FormatInt(id, 10))
}

func (s *Server) listGithubPATs(w http.ResponseWriter, r *http.Request) {
	pats, err := s.Store.ListGithubPATs(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, pats)
}

func (s *Server) deleteGithubPAT(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "patID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := s.Store.DeleteGithubPAT(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type linkRepoRequest struct {
	GithubPATID   int64  `json:"github_pat_id"`
	RepoOwner     string `json:"repo_owner"`
	RepoName      string `json:"repo_name"`
	DefaultBranch string `json:"default_branch"`
}

// linkProjectRepo connects a project to a GitHub repo, generating the
// opaque webhook token (URL path segment) and a separate random HMAC
// secret, both returned once here -- the caller must copy the webhook URL
// and secret into GitHub's repo settings now, since the secret is never
// returned again after this call.
func (s *Server) linkProjectRepo(w http.ResponseWriter, r *http.Request) {
	projectID, err := parseID(chi.URLParam(r, "projectID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return
	}
	var req linkRepoRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if req.RepoOwner == "" || req.RepoName == "" {
		writeError(w, http.StatusBadRequest, "repo_owner and repo_name are required")
		return
	}
	if req.DefaultBranch == "" {
		req.DefaultBranch = "main"
	}

	token, err := webhook.GenerateToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	secret, err := webhook.GenerateToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	secretCiphertext, secretNonce, err := s.Secrets.Seal([]byte("project_repos:webhook_secret:"+token), []byte(secret))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "encrypt webhook secret: "+err.Error())
		return
	}

	id, err := s.Store.CreateProjectRepo(r.Context(), projectID, req.GithubPATID, req.RepoOwner, req.RepoName, req.DefaultBranch, token, secretCiphertext, secretNonce)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"id":             id,
		"webhook_path":   "/webhooks/github/" + token,
		"webhook_secret": secret,
	})
}

func (s *Server) getProjectRepo(w http.ResponseWriter, r *http.Request) {
	projectID, err := parseID(chi.URLParam(r, "projectID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return
	}
	pr, err := s.Store.GetProjectRepoByProject(r.Context(), projectID)
	if err == store.ErrNotFound {
		writeError(w, http.StatusNotFound, "no repo linked to this project")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	// Never returns the secret again after creation.
	writeJSON(w, http.StatusOK, map[string]any{
		"id":             pr.ID,
		"repo_owner":     pr.RepoOwner,
		"repo_name":      pr.RepoName,
		"default_branch": pr.DefaultBranch,
		"webhook_path":   "/webhooks/github/" + pr.WebhookToken,
	})
}

type setDeploymentRepoRequest struct {
	ProjectRepoID    int64  `json:"project_repo_id"`
	Branch           string `json:"branch"`
	AutoDeployOnPush bool   `json:"auto_deploy_on_push"`
}

func (s *Server) setDeploymentRepo(w http.ResponseWriter, r *http.Request) {
	deploymentID, err := parseID(chi.URLParam(r, "deploymentID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid deployment id")
		return
	}
	var req setDeploymentRepoRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if req.Branch == "" {
		writeError(w, http.StatusBadRequest, "branch is required")
		return
	}
	if err := s.Store.SetDeploymentRepo(r.Context(), deploymentID, req.ProjectRepoID, req.Branch, req.AutoDeployOnPush); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
