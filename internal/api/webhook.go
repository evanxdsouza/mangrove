package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"

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

			var historyID int64
			var deployErr error
			if dep.BuildStrategy == "compose" {
				historyID, deployErr = s.Orchestrator.DeployCompose(bgCtx, req)
			} else {
				historyID, deployErr = s.Orchestrator.Deploy(bgCtx, req)
			}
			if deployErr != nil {
				s.Log.Warn("webhook-triggered deploy failed", "deployment_id", depID, "deploy_history_id", historyID, "error", deployErr)
			} else {
				s.Log.Info("webhook-triggered deploy succeeded", "deployment_id", depID, "deploy_history_id", historyID)
			}
		}(deploymentID)
	}

	return true
}
