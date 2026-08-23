package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/evanxdsouza/mangrove/internal/models"
	"github.com/evanxdsouza/mangrove/internal/store"
)

func idStr(id int64) string { return strconv.FormatInt(id, 10) }

// stagingRouter wires just the routes these tests exercise, mirroring
// router.go's shape closely enough to catch a wiring mistake without
// pulling in auth middleware (these tests call handlers as an already-
// authenticated request would).
func stagingRouter(s *Server) http.Handler {
	r := chi.NewRouter()
	r.Route("/deployments/{deploymentID}", func(r chi.Router) {
		r.Get("/staging", s.listStagingDeployments)
		r.Post("/staging", s.createStagingDeployment)
		r.Post("/promote", s.promoteDeployment)
	})
	return r
}

// seedProductionDeployment creates a project, a linked repo (same shape as
// seedRepoAndDeployment in webhook_test.go), and a production deployment
// with one plain and one secret env var -- enough to exercise staging's
// clone-the-service-and-env-vars path, including the AAD-rebind for
// secrets (see createStagingDeployment).
func seedProductionDeployment(t *testing.T, env *testEnv) (dep models.Deployment, repoID int64) {
	t.Helper()
	ctx := context.Background()

	proj, err := env.store.CreateProject(ctx, 1, "Staging Test", "staging-test", "")
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
	patCiphertext, patNonce, err = env.box.Seal(patAAD(patID), []byte("fake-pat-token"))
	if err != nil {
		t.Fatalf("re-seal PAT: %v", err)
	}
	env.store.DB.ExecContext(ctx, `UPDATE github_pats SET token_encrypted = ?, token_nonce = ? WHERE id = ?`, patCiphertext, patNonce, patID)

	webhookToken := "staging-test-token-" + time.Now().Format("150405.000000")
	secretCiphertext, secretNonce, err := env.box.Seal([]byte("project_repos:webhook_secret:"+webhookToken), []byte("wh-secret"))
	if err != nil {
		t.Fatalf("seal webhook secret: %v", err)
	}
	repoID, err = env.store.CreateProjectRepo(ctx, proj.ID, patID, "octocat", "Hello-World", "main", webhookToken, secretCiphertext, secretNonce)
	if err != nil {
		t.Fatalf("CreateProjectRepo: %v", err)
	}

	dep, err = env.store.CreateDeployment(ctx, store.CreateDeploymentParams{
		ProjectID: proj.ID, Name: "web", Slug: "prod-web", BuildStrategy: "dockerfile",
	})
	if err != nil {
		t.Fatalf("CreateDeployment: %v", err)
	}
	svc, err := env.store.CreateService(ctx, store.CreateServiceParams{
		DeploymentID: dep.ID, Name: "web", ContainerName: "mangrove-prod-web-web", InternalPort: 3000,
	})
	if err != nil {
		t.Fatalf("CreateService: %v", err)
	}
	if err := env.store.CreatePlainEnvVar(ctx, svc.ID, "NODE_ENV", "production"); err != nil {
		t.Fatalf("CreatePlainEnvVar: %v", err)
	}
	secretAAD := []byte("env_vars:" + idStr(svc.ID) + ":API_KEY")
	secretCiphertext2, secretNonce2, err := env.box.Seal(secretAAD, []byte("super-secret-value"))
	if err != nil {
		t.Fatalf("seal secret env var: %v", err)
	}
	if err := env.store.CreateSecretEnvVar(ctx, svc.ID, "API_KEY", secretCiphertext2, secretNonce2); err != nil {
		t.Fatalf("CreateSecretEnvVar: %v", err)
	}
	if err := env.store.SetDeploymentRepo(ctx, dep.ID, repoID, "main", true); err != nil {
		t.Fatalf("SetDeploymentRepo: %v", err)
	}
	dep, err = env.store.GetDeployment(ctx, dep.ID)
	if err != nil {
		t.Fatalf("GetDeployment: %v", err)
	}
	return dep, repoID
}

func TestCreateStagingDeploymentClonesConfigAndEnvVars(t *testing.T) {
	env := newTestEnv(t)
	prod, _ := seedProductionDeployment(t, env)
	router := stagingRouter(env.server)

	body, _ := json.Marshal(map[string]any{"branch": "feature/staging-branch"})
	req := httptest.NewRequest(http.MethodPost, "/deployments/"+idStr(prod.ID)+"/staging", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Deployment models.Deployment `json:"deployment"`
		DeployErr  string            `json:"deploy_error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	staging := resp.Deployment

	if staging.Environment != "staging" {
		t.Errorf("expected environment=staging, got %q", staging.Environment)
	}
	if staging.PromotesToDeploymentID == nil || *staging.PromotesToDeploymentID != prod.ID {
		t.Errorf("expected promotes_to_deployment_id=%d, got %v", prod.ID, staging.PromotesToDeploymentID)
	}
	if staging.GitBranch != "feature/staging-branch" {
		t.Errorf("expected git_branch=feature/staging-branch, got %q", staging.GitBranch)
	}
	if !staging.AutoDeployOnPush {
		t.Error("expected auto_deploy_on_push=true on the new staging deployment")
	}
	if staging.Slug == prod.Slug {
		t.Error("expected a distinct slug for the staging deployment")
	}
	// The initial deploy attempt fails (fake PAT against real github.com),
	// same tolerated failure as TestWebhookPushToMatchingBranchTriggersDeploy
	// -- what matters here is that staging + its cloned resources exist.
	if resp.DeployErr == "" {
		t.Log("initial staging deploy unexpectedly succeeded or errored silently; not required for this test")
	}

	services, err := env.store.ListServices(context.Background(), staging.ID)
	if err != nil {
		t.Fatalf("ListServices: %v", err)
	}
	if len(services) != 1 {
		t.Fatalf("expected 1 cloned service, got %d", len(services))
	}
	stagingSvc := services[0]

	envRows, err := env.store.ListEnvVarRows(context.Background(), stagingSvc.ID)
	if err != nil {
		t.Fatalf("ListEnvVarRows: %v", err)
	}
	if len(envRows) != 2 {
		t.Fatalf("expected 2 cloned env vars, got %d", len(envRows))
	}
	for _, row := range envRows {
		if row.KeyName == "NODE_ENV" && row.ValuePlain.String != "production" {
			t.Errorf("expected cloned NODE_ENV=production, got %q", row.ValuePlain.String)
		}
		if row.KeyName == "API_KEY" {
			if !row.IsSecret {
				t.Error("expected API_KEY to remain marked secret after cloning")
			}
			aad := []byte("env_vars:" + idStr(stagingSvc.ID) + ":API_KEY")
			plaintext, err := env.box.Open(aad, row.ValueEncrypted, row.ValueNonce)
			if err != nil {
				t.Fatalf("decrypt cloned secret with new service's AAD: %v", err)
			}
			if string(plaintext) != "super-secret-value" {
				t.Errorf("expected cloned secret value to survive re-encryption, got %q", plaintext)
			}
		}
	}

	// listStagingDeployments should surface it under the production deployment.
	listReq := httptest.NewRequest(http.MethodGet, "/deployments/"+idStr(prod.ID)+"/staging", nil)
	listRec := httptest.NewRecorder()
	router.ServeHTTP(listRec, listReq)
	var listed []models.Deployment
	if err := json.Unmarshal(listRec.Body.Bytes(), &listed); err != nil {
		t.Fatalf("decode staging list: %v", err)
	}
	if len(listed) != 1 || listed[0].ID != staging.ID {
		t.Errorf("expected listStagingDeployments to return the new staging deployment, got %+v", listed)
	}
}

func TestPromoteDeploymentUsesStagingsExactCommit(t *testing.T) {
	env := newTestEnv(t)
	prod, _ := seedProductionDeployment(t, env)
	router := stagingRouter(env.server)

	ctx := context.Background()
	staging, err := env.store.CreateDeployment(ctx, store.CreateDeploymentParams{
		ProjectID: prod.ProjectID, Name: "web (staging)", Slug: "prod-web-staging",
		BuildStrategy: "dockerfile", Environment: "staging", PromotesToDeploymentID: &prod.ID,
	})
	if err != nil {
		t.Fatalf("CreateDeployment (staging): %v", err)
	}
	if _, err := env.store.CreateService(ctx, store.CreateServiceParams{
		DeploymentID: staging.ID, Name: "web", ContainerName: "mangrove-prod-web-staging-web", InternalPort: 3000,
	}); err != nil {
		t.Fatalf("CreateService: %v", err)
	}

	// Simulate a prior successful push-triggered deploy of staging landing
	// on a specific commit -- this is what promote should reuse exactly,
	// rather than redeploying production from its branch's tip.
	historyID, err := env.store.CreateDeployHistory(ctx, staging.ID, "push", "deadbeefcafe", "a staging commit", "feature/staging-branch")
	if err != nil {
		t.Fatalf("CreateDeployHistory: %v", err)
	}
	if err := env.store.MarkDeployHistoryCurrent(ctx, staging.ID, historyID); err != nil {
		t.Fatalf("MarkDeployHistoryCurrent: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/deployments/"+idStr(staging.ID)+"/promote", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	// The actual deploy fails (fake PAT against real github.com, same as
	// every other test here) -- 422 with a deploy_history_id is the
	// "dispatched but the build itself failed" shape, which is what we
	// want to inspect.
	if rec.Code != http.StatusOK && rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 200 or 422, got %d: %s", rec.Code, rec.Body.String())
	}

	deadline := time.Now().Add(20 * time.Second)
	var found *models.DeployHistory
	for time.Now().Before(deadline) {
		history, err := env.store.ListDeployHistory(context.Background(), prod.ID)
		if err == nil {
			for i := range history {
				if history[i].TriggeredBy == "promote" {
					found = &history[i]
					break
				}
			}
		}
		if found != nil {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	if found == nil {
		t.Fatal("expected a promote-triggered deploy_history row on the production deployment")
	}
	if found.CommitSHA != "deadbeefcafe" {
		t.Errorf("expected promote to carry staging's exact commit sha, got %q", found.CommitSHA)
	}
	if found.GitRef != "deadbeefcafe" {
		t.Errorf("expected promote to deploy production using the commit sha as GitRef (exact commit, not branch tip), got %q", found.GitRef)
	}
}

func TestPromoteDeploymentRejectsNonStagingDeployment(t *testing.T) {
	env := newTestEnv(t)
	prod, _ := seedProductionDeployment(t, env)
	router := stagingRouter(env.server)

	req := httptest.NewRequest(http.MethodPost, "/deployments/"+idStr(prod.ID)+"/promote", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422 for promoting a non-staging deployment, got %d: %s", rec.Code, rec.Body.String())
	}
}
