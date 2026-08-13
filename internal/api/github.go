package api

import (
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/evanxdsouza/mangrove/internal/auth"
	"github.com/evanxdsouza/mangrove/internal/executor"
	ghclient "github.com/evanxdsouza/mangrove/internal/github"
	"github.com/evanxdsouza/mangrove/internal/store"
	"github.com/evanxdsouza/mangrove/internal/webhook"
)

// requestBaseURL returns Mangrove's own externally-visible origin, for
// building an absolute callback URL (the GitHub OAuth redirect_uri, and an
// auto-registered repo webhook's URL). MANGROVE_PUBLIC_URL, when set,
// always wins -- request-derived scheme detection is only a best-effort
// fallback, and is wrong behind an edge that doesn't forward
// X-Forwarded-Proto to this particular route (see config.Config.PublicURL).
func (s *Server) requestBaseURL(r *http.Request) string {
	if s.PublicURL != "" {
		return s.PublicURL
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		scheme = proto
	}
	return scheme + "://" + r.Host
}

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

	// Best-effort: an OAuth-sourced credential (unlike a hand-pasted PAT,
	// whose scopes we can't assume) came from a token that just granted
	// Mangrove `repo` scope, so it can also register the webhook itself via
	// GitHub's API -- skips the "paste this into GitHub's UI" step
	// entirely. On any failure (org restrictions, revoked scope, ...) we
	// silently fall back to the manual instructions the response already
	// carries; nothing about the link itself failed.
	webhookRegistered := false
	if pat, err := s.Store.GetGithubPAT(r.Context(), req.GithubPATID); err == nil && pat.Source == "oauth" {
		if ciphertext, nonce, err := s.Store.GetGithubPATEncrypted(r.Context(), req.GithubPATID); err == nil {
			if tok, err := s.Secrets.Open(patAAD(req.GithubPATID), ciphertext, nonce); err == nil {
				callbackURL := s.requestBaseURL(r) + "/webhooks/github/" + token
				if err := s.GithubRepos.CreateWebhook(r.Context(), string(tok), req.RepoOwner, req.RepoName, callbackURL, secret); err == nil {
					webhookRegistered = true
				}
			}
		}
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"id":                 id,
		"webhook_path":       "/webhooks/github/" + token,
		"webhook_secret":     secret,
		"webhook_registered": webhookRegistered,
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

// ---- GitHub OAuth ("Deploy from GitHub") ----

// startGithubOAuth begins the OAuth App web flow. It's meant to be opened
// as a real browser navigation (window.open, not fetch) -- a popup that
// carries the session cookie out to GitHub and back, not an API call --
// which is why this redirects (302) instead of returning JSON.
func (s *Server) startGithubOAuth(w http.ResponseWriter, r *http.Request) {
	if !s.GithubOAuthEnabled() {
		writeError(w, http.StatusNotImplemented, "GitHub OAuth is not configured on this server (set MANGROVE_GITHUB_OAUTH_CLIENT_ID/_SECRET)")
		return
	}
	userID, _ := auth.UserIDFromContext(r.Context())

	state, err := webhook.GenerateToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	redirectURI := s.requestBaseURL(r) + "/api/github/oauth/callback"
	if err := s.Store.CreateGithubOAuthState(r.Context(), state, userID, redirectURI, 10*time.Minute); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	http.Redirect(w, r, ghclient.AuthorizeURL(s.GithubOAuthClientID, redirectURI, state), http.StatusFound)
}

type oauthCallbackData struct {
	OK    bool
	Error string
}

// oauthCallbackPage is a tiny self-closing page: it hands the result back
// to the window that opened the popup via postMessage and closes itself,
// so the SPA never needs a dedicated client-side route for this. Built with
// html/template (not string concatenation) so the error message -- which
// can contain arbitrary text from GitHub or from Go error strings -- is
// safely escaped for both the HTML and the inline <script> JS-string
// context.
var oauthCallbackPage = template.Must(template.New("oauth-callback").Parse(`<!doctype html>
<html><head><title>GitHub</title></head><body style="font: 14px sans-serif; padding: 24px;">
<script>
(function() {
  var msg = { type: "mangrove-github-oauth", ok: {{.OK}}, error: {{.Error}} };
  if (window.opener) { window.opener.postMessage(msg, window.location.origin); }
  window.close();
})();
</script>
<p>{{if .OK}}Connected. This window should close automatically.{{else}}Something went wrong: {{.Error}}{{end}}</p>
</body></html>`))

func (s *Server) githubOAuthCallback(w http.ResponseWriter, r *http.Request) {
	respond := func(ok bool, msg string) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		oauthCallbackPage.Execute(w, oauthCallbackData{OK: ok, Error: msg})
	}

	if !s.GithubOAuthEnabled() {
		respond(false, "GitHub OAuth is not configured on this server")
		return
	}
	userID, _ := auth.UserIDFromContext(r.Context())

	if errParam := r.URL.Query().Get("error"); errParam != "" {
		respond(false, "GitHub denied the request: "+errParam)
		return
	}
	state := r.URL.Query().Get("state")
	code := r.URL.Query().Get("code")
	if state == "" || code == "" {
		respond(false, "missing code/state in callback")
		return
	}

	redirectURI, err := s.Store.ConsumeGithubOAuthState(r.Context(), state, userID)
	if err != nil {
		respond(false, "invalid or expired OAuth state -- try connecting again")
		return
	}

	token, err := s.GithubOAuth.Exchange(r.Context(), s.GithubOAuthClientID, s.GithubOAuthClientSecret, code, redirectURI)
	if err != nil {
		respond(false, "token exchange failed: "+err.Error())
		return
	}
	login, err := s.GithubOAuth.FetchLogin(r.Context(), token)
	if err != nil {
		respond(false, "couldn't read GitHub identity: "+err.Error())
		return
	}

	ciphertext, nonce, err := s.Secrets.Seal([]byte("github_pats:new:oauth:"+login), []byte(token))
	if err != nil {
		respond(false, "encrypt token: "+err.Error())
		return
	}
	id, err := s.Store.CreateGithubOAuthToken(r.Context(), login, ciphertext, nonce)
	if err != nil {
		respond(false, err.Error())
		return
	}
	// Re-seal with the real row ID bound as AAD, same two-step pattern
	// createGithubPAT uses (the ID doesn't exist until the insert above).
	if ciphertext, nonce, err = s.Secrets.Seal(patAAD(id), []byte(token)); err == nil {
		s.Store.DB.ExecContext(r.Context(), `UPDATE github_pats SET token_encrypted = ?, token_nonce = ? WHERE id = ?`, ciphertext, nonce, id)
	}

	respond(true, "")
}

// listGithubRepos lists every repo a stored credential (OAuth-connected or
// a pasted PAT -- both work the same way here) can see, for the repo-picker
// step of "Deploy from GitHub".
func (s *Server) listGithubRepos(w http.ResponseWriter, r *http.Request) {
	patID, err := parseID(r.URL.Query().Get("github_pat_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "github_pat_id is required")
		return
	}
	ciphertext, nonce, err := s.Store.GetGithubPATEncrypted(r.Context(), patID)
	if err == store.ErrNotFound {
		writeError(w, http.StatusNotFound, "github credential not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	token, err := s.Secrets.Open(patAAD(patID), ciphertext, nonce)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "decrypt token: "+err.Error())
		return
	}
	repos, err := s.GithubRepos.ListRepos(r.Context(), string(token))
	if err != nil {
		writeError(w, http.StatusBadGateway, "github API: "+err.Error())
		return
	}
	s.Store.TouchGithubPATUsed(r.Context(), patID)
	writeJSON(w, http.StatusOK, repos)
}

type detectRepoRequest struct {
	GithubPATID int64  `json:"github_pat_id"`
	RepoOwner   string `json:"repo_owner"`
	RepoName    string `json:"repo_name"`
	Branch      string `json:"branch"`
	RootPath    string `json:"root_path"`
}

// detectGithubRepo shallow-clones the picked repo/branch and inspects it to
// guess a build strategy, paths, and prompted env vars -- the "auto
// detects" step of "Deploy from GitHub". A GithubPATID of 0 is valid (a
// public repo needs no auth token to clone).
func (s *Server) detectGithubRepo(w http.ResponseWriter, r *http.Request) {
	var req detectRepoRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if req.RepoOwner == "" || req.RepoName == "" {
		writeError(w, http.StatusBadRequest, "repo_owner and repo_name are required")
		return
	}
	if req.RootPath == "" {
		req.RootPath = "."
	}

	var token string
	if req.GithubPATID != 0 {
		ciphertext, nonce, err := s.Store.GetGithubPATEncrypted(r.Context(), req.GithubPATID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "github credential not found")
			return
		}
		tok, err := s.Secrets.Open(patAAD(req.GithubPATID), ciphertext, nonce)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "decrypt token: "+err.Error())
			return
		}
		token = string(tok)
	}

	src := executor.ContextSource{
		GitURL:    fmt.Sprintf("https://github.com/%s/%s.git", req.RepoOwner, req.RepoName),
		GitRef:    req.Branch,
		AuthToken: token,
	}
	result, err := executor.DetectBuildStrategy(r.Context(), src, req.RootPath)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "couldn't inspect repo: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}
