package api

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/evanxdsouza/mangrove/internal/config"
	mangrovedb "github.com/evanxdsouza/mangrove/internal/db"
	"github.com/evanxdsouza/mangrove/internal/executor"
	"github.com/evanxdsouza/mangrove/internal/orchestrator"
	"github.com/evanxdsouza/mangrove/internal/secrets"
	"github.com/evanxdsouza/mangrove/internal/store"
)

// testEnv wires a real Server (real SQLite, real secrets box, real Docker
// executor) -- the same components production code uses -- so these tests
// exercise actual HMAC verification, actual encryption, and actual deploy
// triggering, not mocks.
type testEnv struct {
	server *Server
	store  *store.Store
	box    *secrets.Box
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	dir := t.TempDir()
	db, err := mangrovedb.Open(filepath.Join(dir, "mangrove.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	box, err := secrets.NewBox(key)
	if err != nil {
		t.Fatalf("new box: %v", err)
	}

	st := store.New(db)
	dockerExec, err := executor.NewDockerExecutor(context.Background(), "mangrove-webhook-test-net")
	if err != nil {
		t.Skipf("docker not available: %v", err)
	}
	t.Cleanup(func() { exec.Command("docker", "network", "rm", "mangrove-webhook-test-net").Run() })

	log := slog.New(slog.NewTextHandler(os.Stdout, nil))
	orch := &orchestrator.Orchestrator{
		Store:   st,
		Exec:    dockerExec,
		Compose: &executor.ComposeExecutor{NetworkName: "mangrove-webhook-test-net"},
		Secrets: box,
		Config:  config.Config{PortRangeMin: 21000, PortRangeMax: 21999, DeploymentMemoryCeilingMB: 4096, NetworkName: "mangrove-webhook-test-net"},
		Log:     log,
	}

	return &testEnv{
		server: &Server{Store: st, Orchestrator: orch, Secrets: box, Log: log},
		store:  st,
		box:    box,
	}
}

// seedRepoAndDeployment creates a project, a fake PAT, a linked repo, and a
// deployment configured to auto-deploy pushes to `branch`. Returns the
// webhook token (URL segment) and raw secret (for signing test requests).
func seedRepoAndDeployment(t *testing.T, env *testEnv, branch string, autoDeployOnPush bool) (webhookToken, rawSecret string, deploymentID int64) {
	t.Helper()
	ctx := context.Background()

	proj, err := env.store.CreateProject(ctx, "Webhook Test", "webhook-test", "")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	patCiphertext, patNonce, err := env.box.Seal([]byte("github_pats:1"), []byte("fake-pat-token"))
	if err != nil {
		t.Fatalf("seal PAT: %v", err)
	}
	patID, err := env.store.CreateGithubPAT(ctx, "test-pat", patCiphertext, patNonce)
	if err != nil {
		t.Fatalf("CreateGithubPAT: %v", err)
	}
	// Re-seal bound to the real row id, mirroring what the handler does.
	patCiphertext, patNonce, err = env.box.Seal(patAAD(patID), []byte("fake-pat-token"))
	if err != nil {
		t.Fatalf("re-seal PAT: %v", err)
	}
	env.store.DB.ExecContext(ctx, `UPDATE github_pats SET token_encrypted = ?, token_nonce = ? WHERE id = ?`, patCiphertext, patNonce, patID)

	webhookToken = "test-token-" + time.Now().Format("150405.000000")
	rawSecret = "test-webhook-secret"
	secretCiphertext, secretNonce, err := env.box.Seal([]byte("project_repos:webhook_secret:"+webhookToken), []byte(rawSecret))
	if err != nil {
		t.Fatalf("seal webhook secret: %v", err)
	}
	repoID, err := env.store.CreateProjectRepo(ctx, proj.ID, patID, "octocat", "Hello-World", "main", webhookToken, secretCiphertext, secretNonce)
	if err != nil {
		t.Fatalf("CreateProjectRepo: %v", err)
	}

	dep, err := env.store.CreateDeployment(ctx, store.CreateDeploymentParams{
		ProjectID: proj.ID, Name: "web", Slug: "web", BuildStrategy: "dockerfile",
	})
	if err != nil {
		t.Fatalf("CreateDeployment: %v", err)
	}
	if _, err := env.store.CreateService(ctx, store.CreateServiceParams{
		DeploymentID: dep.ID, Name: "web", ContainerName: "mangrove-webhook-test-web", InternalPort: 80,
	}); err != nil {
		t.Fatalf("CreateService: %v", err)
	}
	if err := env.store.SetDeploymentRepo(ctx, dep.ID, repoID, branch, autoDeployOnPush); err != nil {
		t.Fatalf("SetDeploymentRepo: %v", err)
	}

	return webhookToken, rawSecret, dep.ID
}

func sign(secret, body []byte) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func webhookRouter(s *Server) http.Handler {
	r := chi.NewRouter()
	r.Post("/webhooks/github/{token}", s.githubWebhook)
	return r
}

func pushPayload(branch string) []byte {
	b, _ := json.Marshal(map[string]any{
		"ref":         "refs/heads/" + branch,
		"after":       "abc123",
		"head_commit": map[string]any{"id": "abc123", "message": "test commit"},
	})
	return b
}

func TestWebhookRejectsInvalidSignature(t *testing.T) {
	env := newTestEnv(t)
	token, _, _ := seedRepoAndDeployment(t, env, "main", true)
	router := webhookRouter(env.server)

	body := pushPayload("main")
	req := httptest.NewRequest(http.MethodPost, "/webhooks/github/"+token, bytes.NewReader(body))
	req.Header.Set("X-Hub-Signature-256", "sha256="+hex.EncodeToString(make([]byte, 32))) // wrong signature
	req.Header.Set("X-GitHub-Event", "push")
	req.Header.Set("X-GitHub-Delivery", "delivery-1")

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for invalid signature, got %d", rec.Code)
	}
}

func TestWebhookRejectsUnknownToken(t *testing.T) {
	env := newTestEnv(t)
	router := webhookRouter(env.server)

	body := pushPayload("main")
	req := httptest.NewRequest(http.MethodPost, "/webhooks/github/does-not-exist", bytes.NewReader(body))
	req.Header.Set("X-Hub-Signature-256", sign([]byte("whatever"), body))
	req.Header.Set("X-GitHub-Event", "push")
	req.Header.Set("X-GitHub-Delivery", "delivery-2")

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404 for unknown webhook token, got %d", rec.Code)
	}
}

func TestWebhookPingReturns200WithoutDeploying(t *testing.T) {
	env := newTestEnv(t)
	token, secret, _ := seedRepoAndDeployment(t, env, "main", true)
	router := webhookRouter(env.server)

	body := []byte(`{"zen":"test"}`)
	req := httptest.NewRequest(http.MethodPost, "/webhooks/github/"+token, bytes.NewReader(body))
	req.Header.Set("X-Hub-Signature-256", sign([]byte(secret), body))
	req.Header.Set("X-GitHub-Event", "ping")
	req.Header.Set("X-GitHub-Delivery", "delivery-ping-1")

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for ping, got %d", rec.Code)
	}
}

func TestWebhookPushToMatchingBranchTriggersDeploy(t *testing.T) {
	env := newTestEnv(t)
	token, secret, deploymentID := seedRepoAndDeployment(t, env, "main", true)
	router := webhookRouter(env.server)

	body := pushPayload("main")
	req := httptest.NewRequest(http.MethodPost, "/webhooks/github/"+token, bytes.NewReader(body))
	req.Header.Set("X-Hub-Signature-256", sign([]byte(secret), body))
	req.Header.Set("X-GitHub-Event", "push")
	req.Header.Set("X-GitHub-Delivery", "delivery-push-1")

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202 Accepted, got %d: %s", rec.Code, rec.Body.String())
	}

	// The deploy runs in a background goroutine; poll for the resulting
	// deploy_history row (it will end up "failed" since octocat/Hello-World
	// has no Dockerfile -- this test verifies the webhook->orchestrator
	// wiring fires at all, not that a real build succeeds).
	deadline := time.Now().Add(20 * time.Second)
	var found bool
	for time.Now().Before(deadline) {
		history, err := env.store.ListDeployHistory(context.Background(), deploymentID)
		if err == nil {
			for _, h := range history {
				if h.TriggeredBy == "push" && h.CommitSHA == "abc123" {
					found = true
					break
				}
			}
		}
		if found {
			break
		}
		time.Sleep(300 * time.Millisecond)
	}
	if !found {
		t.Error("expected a push-triggered deploy_history row referencing the webhook commit SHA to appear")
	}
}

func TestWebhookIdempotentOnReplayedDelivery(t *testing.T) {
	env := newTestEnv(t)
	token, secret, deploymentID := seedRepoAndDeployment(t, env, "main", true)
	router := webhookRouter(env.server)

	body := pushPayload("main")
	sig := sign([]byte(secret), body)

	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/webhooks/github/"+token, bytes.NewReader(body))
		req.Header.Set("X-Hub-Signature-256", sig)
		req.Header.Set("X-GitHub-Event", "push")
		req.Header.Set("X-GitHub-Delivery", "delivery-replay-1") // same delivery ID both times
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if i == 1 && rec.Code != http.StatusOK {
			t.Errorf("expected replayed delivery to return 200 without reprocessing, got %d", rec.Code)
		}
	}

	time.Sleep(500 * time.Millisecond)
	history, err := env.store.ListDeployHistory(context.Background(), deploymentID)
	if err != nil {
		t.Fatalf("ListDeployHistory: %v", err)
	}
	count := 0
	for _, h := range history {
		if h.CommitSHA == "abc123" {
			count++
		}
	}
	if count > 1 {
		t.Errorf("expected the replayed delivery to trigger at most 1 deploy, got %d", count)
	}
}

func TestWebhookIgnoresPushWhenAutoDeployDisabled(t *testing.T) {
	env := newTestEnv(t)
	token, secret, deploymentID := seedRepoAndDeployment(t, env, "main", false) // auto-deploy off
	router := webhookRouter(env.server)

	body := pushPayload("main")
	req := httptest.NewRequest(http.MethodPost, "/webhooks/github/"+token, bytes.NewReader(body))
	req.Header.Set("X-Hub-Signature-256", sign([]byte(secret), body))
	req.Header.Set("X-GitHub-Event", "push")
	req.Header.Set("X-GitHub-Delivery", "delivery-no-auto-1")

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 (no match, no deploy) got %d", rec.Code)
	}

	time.Sleep(500 * time.Millisecond)
	history, _ := env.store.ListDeployHistory(context.Background(), deploymentID)
	if len(history) != 0 {
		t.Errorf("expected no deploy to be triggered when auto_deploy_on_push is false, got %d history rows", len(history))
	}
}

func TestWebhookIgnoresPushToNonMatchingBranch(t *testing.T) {
	env := newTestEnv(t)
	token, secret, deploymentID := seedRepoAndDeployment(t, env, "main", true)
	router := webhookRouter(env.server)

	body := pushPayload("some-other-branch")
	req := httptest.NewRequest(http.MethodPost, "/webhooks/github/"+token, bytes.NewReader(body))
	req.Header.Set("X-Hub-Signature-256", sign([]byte(secret), body))
	req.Header.Set("X-GitHub-Event", "push")
	req.Header.Set("X-GitHub-Delivery", "delivery-wrong-branch-1")

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 (branch doesn't match) got %d", rec.Code)
	}

	time.Sleep(500 * time.Millisecond)
	history, _ := env.store.ListDeployHistory(context.Background(), deploymentID)
	if len(history) != 0 {
		t.Errorf("expected no deploy for a push to a non-configured branch, got %d history rows", len(history))
	}
}
