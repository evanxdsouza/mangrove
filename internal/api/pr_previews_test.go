package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/evanxdsouza/mangrove/internal/store"
)

// pullRequestPayload builds a minimal pull_request webhook body -- just the
// fields handlePullRequestEvent reads.
func pullRequestPayload(action string, number int, headRef, headSHA string) []byte {
	b, _ := json.Marshal(map[string]any{
		"action": action,
		"number": number,
		"pull_request": map[string]any{
			"title": "test PR",
			"head":  map[string]any{"ref": headRef, "sha": headSHA},
		},
	})
	return b
}

func TestWebhookPullRequestOpenedCreatesPreview(t *testing.T) {
	env := newTestEnv(t)
	token, secret, deploymentID := seedRepoAndDeployment(t, env, "main", false)
	ctx := context.Background()
	if err := env.store.SetPRPreviewsEnabled(ctx, deploymentID, true); err != nil {
		t.Fatalf("SetPRPreviewsEnabled: %v", err)
	}
	router := webhookRouter(env.server)

	body := pullRequestPayload("opened", 42, "feature-x", "def456")
	req := httptest.NewRequest(http.MethodPost, "/webhooks/github/"+token, bytes.NewReader(body))
	req.Header.Set("X-Hub-Signature-256", sign([]byte(secret), body))
	req.Header.Set("X-GitHub-Event", "pull_request")
	req.Header.Set("X-GitHub-Delivery", "delivery-pr-1")

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202 Accepted, got %d: %s", rec.Code, rec.Body.String())
	}

	// syncPreview runs in a background goroutine (deploy dispatch will fail
	// against octocat/Hello-World, same as the push tests -- this verifies
	// the preview row gets created and pointed at the PR's head branch, not
	// that a real build succeeds).
	deadline := time.Now().Add(20 * time.Second)
	var preview = struct {
		found  bool
		branch string
	}{}
	for time.Now().Before(deadline) {
		p, err := env.store.GetPreviewDeployment(ctx, deploymentID, 42)
		if err == nil {
			preview.found = true
			preview.branch = p.GitBranch
			break
		}
		if err != store.ErrNotFound {
			t.Fatalf("GetPreviewDeployment: %v", err)
		}
		time.Sleep(300 * time.Millisecond)
	}
	if !preview.found {
		t.Fatal("expected a preview deployment tracking PR #42 to appear")
	}
	if preview.branch != "feature-x" {
		t.Errorf("expected preview to track branch %q, got %q", "feature-x", preview.branch)
	}
}

func TestWebhookIgnoresPullRequestWhenPreviewsDisabled(t *testing.T) {
	env := newTestEnv(t)
	token, secret, deploymentID := seedRepoAndDeployment(t, env, "main", false)
	// pr_previews_enabled defaults to false -- deliberately not enabling it.
	router := webhookRouter(env.server)

	body := pullRequestPayload("opened", 7, "feature-y", "aaa111")
	req := httptest.NewRequest(http.MethodPost, "/webhooks/github/"+token, bytes.NewReader(body))
	req.Header.Set("X-Hub-Signature-256", sign([]byte(secret), body))
	req.Header.Set("X-GitHub-Event", "pull_request")
	req.Header.Set("X-GitHub-Delivery", "delivery-pr-2")

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK (nothing to do), got %d: %s", rec.Code, rec.Body.String())
	}

	if _, err := env.store.GetPreviewDeployment(context.Background(), deploymentID, 7); err != store.ErrNotFound {
		t.Errorf("expected no preview deployment to be created, got err=%v", err)
	}
}

func TestWebhookPullRequestClosedTearsDownPreview(t *testing.T) {
	env := newTestEnv(t)
	token, secret, deploymentID := seedRepoAndDeployment(t, env, "main", false)
	ctx := context.Background()
	if err := env.store.SetPRPreviewsEnabled(ctx, deploymentID, true); err != nil {
		t.Fatalf("SetPRPreviewsEnabled: %v", err)
	}

	// Seed the preview deployment directly (via the same helper the
	// webhook path uses) rather than round-tripping through an "opened"
	// delivery + a real build -- this test only cares about teardown.
	source, err := env.store.GetDeployment(ctx, deploymentID)
	if err != nil {
		t.Fatalf("GetDeployment: %v", err)
	}
	preview, err := env.server.findOrCreatePreviewDeployment(ctx, source, 99, "feature-z")
	if err != nil {
		t.Fatalf("findOrCreatePreviewDeployment: %v", err)
	}

	router := webhookRouter(env.server)
	body := pullRequestPayload("closed", 99, "feature-z", "ccc333")
	req := httptest.NewRequest(http.MethodPost, "/webhooks/github/"+token, bytes.NewReader(body))
	req.Header.Set("X-Hub-Signature-256", sign([]byte(secret), body))
	req.Header.Set("X-GitHub-Event", "pull_request")
	req.Header.Set("X-GitHub-Delivery", "delivery-pr-3")

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202 Accepted, got %d: %s", rec.Code, rec.Body.String())
	}

	deadline := time.Now().Add(20 * time.Second)
	var torndown bool
	for time.Now().Before(deadline) {
		if _, err := env.store.GetDeployment(ctx, preview.ID); err == store.ErrNotFound {
			torndown = true
			break
		}
		time.Sleep(300 * time.Millisecond)
	}
	if !torndown {
		t.Error("expected the preview deployment to be deleted after the PR closed")
	}
}
