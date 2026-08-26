package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/evanxdsouza/mangrove/internal/models"
	"github.com/evanxdsouza/mangrove/internal/orchestrator"
	"github.com/evanxdsouza/mangrove/internal/store"
	"github.com/evanxdsouza/mangrove/internal/webhook"
)

const maxWebhookBodyBytes = 5 << 20 // 5MB -- generous for a push payload, small enough to bound abuse

type githubPushPayload struct {
	Ref        string `json:"ref"`
	After      string `json:"after"`
	HeadCommit struct {
		ID      string `json:"id"`
		Message string `json:"message"`
	} `json:"head_commit"`
}

type githubPullRequestPayload struct {
	Action      string `json:"action"`
	Number      int    `json:"number"`
	PullRequest struct {
		Title string `json:"title"`
		Head  struct {
			Ref string `json:"ref"`
			SHA string `json:"sha"`
		} `json:"head"`
	} `json:"pull_request"`
}

// githubWebhook is Mangrove's single always-on public endpoint besides the
// dashboard itself (see plan §5). It is deliberately NOT behind
// auth.RequireAuth -- GitHub can't log in -- so every step here matters:
// verify the signature before any side effect, and de-duplicate by
// delivery ID before triggering anything.
func (s *Server) githubWebhook(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")

	r.Body = http.MaxBytesReader(w, r.Body, maxWebhookBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusRequestEntityTooLarge, "request body too large or unreadable")
		return
	}

	repo, err := s.Store.GetProjectRepoByWebhookToken(r.Context(), token)
	if err == store.ErrNotFound {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	secretAAD := []byte("project_repos:webhook_secret:" + token)
	secret, err := s.Secrets.Open(secretAAD, repo.SecretCiphertext, repo.SecretNonce)
	if err != nil {
		s.Log.Warn("failed to decrypt webhook secret", "project_repo_id", repo.ID, "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	sigValid := webhook.VerifySignature(secret, body, r.Header.Get("X-Hub-Signature-256"))
	if !sigValid {
		// Signature check happens before any other side effect -- reject
		// here, don't even record the delivery.
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	deliveryID := r.Header.Get("X-GitHub-Delivery")
	eventType := r.Header.Get("X-GitHub-Event")
	if deliveryID == "" {
		writeError(w, http.StatusBadRequest, "missing X-GitHub-Delivery header")
		return
	}

	// Idempotency: GitHub retries on non-2xx/timeout. A replayed delivery
	// ID is acknowledged without reprocessing.
	exists, err := s.Store.WebhookDeliveryExists(r.Context(), deliveryID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if exists {
		w.WriteHeader(http.StatusOK)
		return
	}

	eventID, err := s.Store.CreateWebhookEvent(r.Context(), deliveryID, eventType, &repo.ID, sigValid, string(body))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	switch eventType {
	case "ping":
		s.Store.UpdateWebhookEventStatus(r.Context(), eventID, "processed", nil)
		w.WriteHeader(http.StatusOK)
		return
	case "push":
		if s.handlePushEvent(r.Context(), eventID, repo, body) {
			w.WriteHeader(http.StatusAccepted)
		} else {
			w.WriteHeader(http.StatusOK)
		}
		return
	case "pull_request":
		if s.handlePullRequestEvent(r.Context(), eventID, repo, body) {
			w.WriteHeader(http.StatusAccepted)
		} else {
			w.WriteHeader(http.StatusOK)
		}
		return
	default:
		s.Store.UpdateWebhookEventStatus(r.Context(), eventID, "ignored_no_match", nil)
		w.WriteHeader(http.StatusOK)
		return
	}
}

// handlePushEvent matches the push against every deployment configured to
// auto-deploy this repo+branch, and kicks off each as a background deploy
// -- the handler itself must not block on a build. Returns true only if at
// least one deploy was actually enqueued, so the caller can return 202
// (accepted, work queued) vs 200 (received, nothing to do) per plan §5.
func (s *Server) handlePushEvent(ctx context.Context, eventID int64, repo store.ProjectRepoWithSecret, body []byte) bool {
	var payload githubPushPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		s.Store.UpdateWebhookEventStatus(ctx, eventID, "rejected", nil)
		return false
	}
	branch := webhook.BranchFromRef(payload.Ref)
	if branch == "" {
		s.Store.UpdateWebhookEventStatus(ctx, eventID, "ignored_no_match", nil)
		return false
	}

	deploymentIDs, err := s.Store.ListDeploymentsForRepoBranch(ctx, repo.ID, branch)
	if err != nil || len(deploymentIDs) == 0 {
		s.Store.UpdateWebhookEventStatus(ctx, eventID, "ignored_no_match", nil)
		return false
	}

	ptCiphertext, ptNonce, err := s.Store.GetGithubPATEncrypted(ctx, repo.GithubPATID)
	if err != nil {
		s.Log.Error("webhook: failed to load PAT for deploy", "error", err)
		s.Store.UpdateWebhookEventStatus(ctx, eventID, "rejected", nil)
		return false
	}
	patAADBytes := patAAD(repo.GithubPATID)
	token, err := s.Secrets.Open(patAADBytes, ptCiphertext, ptNonce)
	if err != nil {
		s.Log.Error("webhook: failed to decrypt PAT", "error", err)
		s.Store.UpdateWebhookEventStatus(ctx, eventID, "rejected", nil)
		return false
	}
	s.Store.TouchGithubPATUsed(ctx, repo.GithubPATID)

	gitURL := fmt.Sprintf("https://github.com/%s/%s.git", repo.RepoOwner, repo.RepoName)

	s.Store.UpdateWebhookEventStatus(ctx, eventID, "processed", nil)

	for _, deploymentID := range deploymentIDs {
		go func(depID int64) {
			bgCtx := context.Background()
			dep, err := s.Store.GetDeployment(bgCtx, depID)
			if err != nil {
				s.Log.Error("webhook-triggered deploy: load deployment failed", "deployment_id", depID, "error", err)
				return
			}

			req := orchestrator.DeployRequest{
				DeploymentID:  depID,
				TriggeredBy:   "push",
				GitURL:        gitURL,
				GitRef:        branch,
				CommitSHA:     payload.After,
				CommitMessage: payload.HeadCommit.Message,
				AuthToken:     string(token),
			}

			// WithInflightDeploy is the same guard triggerDeploy/
			// redeployDeployment use: without it, a rapid double-push (or a
			// push landing while a manual redeploy is already running)
			// could start two concurrent deploys of the same deployment,
			// racing on the same service row and deploy_history.is_current.
			// A push that loses the race isn't retried -- the deployment is
			// already mid-deploy from the earlier trigger, so nothing was
			// actually missed.
			var historyID int64
			deployErr := s.Orchestrator.WithInflightDeploy(depID, func(ctx context.Context) error {
				var e error
				historyID, e = s.dispatchDeploy(ctx, dep, req)
				return e
			})
			if errors.Is(deployErr, orchestrator.ErrDeployInProgress) {
				s.Log.Info("webhook-triggered deploy skipped: already in progress", "deployment_id", depID)
			} else if deployErr != nil {
				s.Log.Warn("webhook-triggered deploy failed", "deployment_id", depID, "deploy_history_id", historyID, "error", deployErr)
			} else {
				s.Log.Info("webhook-triggered deploy succeeded", "deployment_id", depID, "deploy_history_id", historyID)
			}
		}(deploymentID)
	}

	return true
}

// handlePullRequestEvent drives per-PR preview deployments: opened/
// reopened/synchronize creates or redeploys a preview deployment tracking
// the PR's head commit and posts/updates a bot comment on the PR with its
// status and URL; closed tears the preview down. Fans out to every
// production deployment on this repo that opted in (pr_previews_enabled)
// -- almost always exactly one, but not assumed to be, same as push.
func (s *Server) handlePullRequestEvent(ctx context.Context, eventID int64, repo store.ProjectRepoWithSecret, body []byte) bool {
	var payload githubPullRequestPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		s.Store.UpdateWebhookEventStatus(ctx, eventID, "rejected", nil)
		return false
	}
	switch payload.Action {
	case "opened", "reopened", "synchronize", "closed":
	default:
		// GitHub sends many other pull_request actions (labeled, assigned,
		// review_requested, ...) that don't affect a preview's deploy state.
		s.Store.UpdateWebhookEventStatus(ctx, eventID, "ignored_no_match", nil)
		return false
	}

	sources, err := s.Store.ListProductionDeploymentsWithPRPreviewsEnabled(ctx, repo.ID)
	if err != nil || len(sources) == 0 {
		s.Store.UpdateWebhookEventStatus(ctx, eventID, "ignored_no_match", nil)
		return false
	}

	ptCiphertext, ptNonce, err := s.Store.GetGithubPATEncrypted(ctx, repo.GithubPATID)
	if err != nil {
		s.Log.Error("pull_request webhook: failed to load PAT for deploy", "error", err)
		s.Store.UpdateWebhookEventStatus(ctx, eventID, "rejected", nil)
		return false
	}
	token, err := s.Secrets.Open(patAAD(repo.GithubPATID), ptCiphertext, ptNonce)
	if err != nil {
		s.Log.Error("pull_request webhook: failed to decrypt PAT", "error", err)
		s.Store.UpdateWebhookEventStatus(ctx, eventID, "rejected", nil)
		return false
	}
	s.Store.TouchGithubPATUsed(ctx, repo.GithubPATID)

	gitURL := fmt.Sprintf("https://github.com/%s/%s.git", repo.RepoOwner, repo.RepoName)
	prNumber := payload.Number
	s.Store.UpdateWebhookEventStatus(ctx, eventID, "processed", nil)

	for _, source := range sources {
		go func(source models.Deployment) {
			bgCtx := context.Background()
			if payload.Action == "closed" {
				s.tearDownPreview(bgCtx, source, prNumber, string(token), repo)
				return
			}
			s.syncPreview(bgCtx, source, prNumber, payload.PullRequest.Head.Ref, payload.PullRequest.Head.SHA, gitURL, string(token), repo)
		}(source)
	}

	return true
}

// syncPreview creates (on this PR's first opened/synchronize delivery) or
// redeploys the preview deployment tracking prNumber, then posts/updates
// the bot comment on the PR with the result. Deploy() blocks until the
// build and health check settle, so "post 'building'" then "post the
// result" is one synchronous round trip here, not a background poll.
func (s *Server) syncPreview(ctx context.Context, source models.Deployment, prNumber int, branch, commitSHA, gitURL, token string, repo store.ProjectRepoWithSecret) {
	preview, err := s.findOrCreatePreviewDeployment(ctx, source, prNumber, branch)
	if err != nil {
		s.Log.Warn("pr preview: create/find failed", "deployment_id", source.ID, "pr", prNumber, "error", err)
		return
	}

	previewURL := fmt.Sprintf("https://%s.%s", preview.Slug, s.Orchestrator.Config.BaseDomain)
	s.upsertPreviewComment(ctx, preview, repo, token, prNumber, previewCommentBody(previewStateBuilding, previewURL, ""))

	req := orchestrator.DeployRequest{
		DeploymentID: preview.ID,
		TriggeredBy:  "push",
		GitURL:       gitURL,
		GitRef:       branch,
		CommitSHA:    commitSHA,
		AuthToken:    token,
	}
	deployErr := s.Orchestrator.WithInflightDeploy(preview.ID, func(ctx context.Context) error {
		_, e := s.dispatchDeploy(ctx, preview, req)
		return e
	})
	if errors.Is(deployErr, orchestrator.ErrDeployInProgress) {
		// A rapid double-push landed while the first was still building --
		// that in-flight call owns the comment update, this one just backs off.
		s.Log.Info("pr preview: deploy skipped, already in progress", "deployment_id", preview.ID)
		return
	}

	state, msg := previewStateSuccess, ""
	if deployErr != nil {
		state, msg = previewStateFailed, deployErr.Error()
		s.Log.Warn("pr preview: deploy failed", "deployment_id", preview.ID, "pr", prNumber, "error", deployErr)
	} else {
		s.Log.Info("pr preview: deployed", "deployment_id", preview.ID, "pr", prNumber)
	}
	// Re-fetch: the "building" comment above may have just persisted a new
	// comment id onto this row.
	if preview, err = s.Store.GetDeployment(ctx, preview.ID); err != nil {
		return
	}
	s.upsertPreviewComment(ctx, preview, repo, token, prNumber, previewCommentBody(state, previewURL, msg))
}

// tearDownPreview deletes the preview deployment tracking prNumber, if one
// exists (a PR closed without ever pushing after opt-in has none), and
// updates its bot comment to say so -- rather than leaving a stale
// "deployed" comment pointing at a URL that no longer resolves.
func (s *Server) tearDownPreview(ctx context.Context, source models.Deployment, prNumber int, token string, repo store.ProjectRepoWithSecret) {
	preview, err := s.Store.GetPreviewDeployment(ctx, source.ID, prNumber)
	if err == store.ErrNotFound {
		return
	}
	if err != nil {
		s.Log.Warn("pr preview: load for teardown failed", "deployment_id", source.ID, "pr", prNumber, "error", err)
		return
	}
	if err := s.Orchestrator.DeleteDeployment(ctx, preview.ID); err != nil {
		s.Log.Warn("pr preview: teardown failed", "deployment_id", preview.ID, "pr", prNumber, "error", err)
		return
	}
	s.Log.Info("pr preview: torn down", "deployment_id", preview.ID, "pr", prNumber)
	s.upsertPreviewComment(ctx, preview, repo, token, prNumber, previewCommentBody(previewStateClosed, "", ""))
}

// upsertPreviewComment posts (or, once one exists, edits in place) the bot
// comment on prNumber -- one live-updating comment per PR rather than a new
// one on every push. Best-effort, same convention as postCommitStatus: a
// GitHub API hiccup here must never fail the underlying deploy/teardown.
func (s *Server) upsertPreviewComment(ctx context.Context, preview models.Deployment, repo store.ProjectRepoWithSecret, token string, prNumber int, body string) {
	if s.GithubComments == nil {
		return
	}
	commentID, err := s.GithubComments.UpsertComment(ctx, token, repo.RepoOwner, repo.RepoName, prNumber, preview.GithubPRCommentID, body)
	if err != nil {
		s.Log.Warn("pr preview: comment post/update failed", "deployment_id", preview.ID, "pr", prNumber, "error", err)
		return
	}
	if preview.GithubPRCommentID == nil || *preview.GithubPRCommentID != commentID {
		if err := s.Store.SetGithubPRCommentID(ctx, preview.ID, commentID); err != nil {
			s.Log.Warn("pr preview: failed to persist comment id", "deployment_id", preview.ID, "error", err)
		}
	}
}

type previewState string

const (
	previewStateBuilding previewState = "building"
	previewStateSuccess  previewState = "success"
	previewStateFailed   previewState = "failed"
	previewStateClosed   previewState = "closed"
)

// previewCommentBody renders the bot's PR comment body, Vercel-style: one
// comment per PR, edited in place as the preview deployment's state changes.
func previewCommentBody(state previewState, previewURL, errMsg string) string {
	switch state {
	case previewStateBuilding:
		return "**Mangrove preview** -- building...\n\nThis comment updates automatically once the deploy finishes."
	case previewStateSuccess:
		return fmt.Sprintf("**Mangrove preview** -- deployed: %s", previewURL)
	case previewStateFailed:
		return fmt.Sprintf("**Mangrove preview** -- deploy failed: %s", errMsg)
	case previewStateClosed:
		return "**Mangrove preview** -- torn down (pull request closed)."
	default:
		return "**Mangrove preview**"
	}
}
