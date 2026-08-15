package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

// newAPITestServer returns a bare Server with no DB/Docker/secrets wiring,
// for handler paths that don't touch them -- listTemplates reads only the
// in-memory template registry, and installTemplate's validation failures
// (unknown key, missing slug) return before ever reaching s.Orchestrator.
// A full successful install is exercised at the orchestrator level (see
// internal/orchestrator/templates_test.go) with a fake executor, rather
// than here against a real Docker daemon pulling real images.
func newAPITestServer(t *testing.T) *testEnv {
	t.Helper()
	return &testEnv{server: &Server{Log: slog.New(slog.NewTextHandler(io.Discard, nil))}}
}

// withURLParams injects chi route params directly, the same way the router
// would after matching "/projects/{projectID}/templates/{templateKey}/
// install" -- used here to exercise handlers without standing up the full
// authenticated router (see webhook_test.go's newTestEnv for why: it needs
// a real Docker daemon, and these handlers' own logic doesn't touch it at
// all for the cases tested below).
func withURLParams(r *http.Request, params map[string]string) *http.Request {
	rctx := chi.NewRouteContext()
	for k, v := range params {
		rctx.URLParams.Add(k, v)
	}
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

func TestListTemplatesReturnsAllBuiltIns(t *testing.T) {
	env := newAPITestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/templates", nil)
	rec := httptest.NewRecorder()
	env.server.listTemplates(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var out []templateSummary
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(out) != 15 {
		t.Fatalf("expected 15 templates, got %d", len(out))
	}
	for _, tpl := range out {
		if tpl.TotalMemoryMB <= 0 {
			t.Errorf("template %q: expected positive total_memory_mb, got %d", tpl.Key, tpl.TotalMemoryMB)
		}
		if len(tpl.Deployments) == 0 {
			t.Errorf("template %q: expected at least 1 deployment summary", tpl.Key)
		}
	}
}

// TestListTemplatesExposesPromptedEnvVars asserts that nephthys's required
// Slack/AI env vars come through in the gallery response -- this is what
// the install form reads to know which inputs to render before allowing
// submit (see TemplateGalleryModal.tsx's InstallTemplateForm).
func TestListTemplatesExposesPromptedEnvVars(t *testing.T) {
	env := newAPITestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/templates", nil)
	rec := httptest.NewRecorder()
	env.server.listTemplates(rec, req)

	var out []templateSummary
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	var nephthys *templateSummary
	for i := range out {
		if out[i].Key == "nephthys" {
			nephthys = &out[i]
		}
	}
	if nephthys == nil {
		t.Fatal("expected a nephthys template in the response")
	}
	var primary *templateDeploymentSummary
	for i := range nephthys.Deployments {
		if nephthys.Deployments[i].SlugSuffix == "" {
			primary = &nephthys.Deployments[i]
		}
	}
	if primary == nil {
		t.Fatal("expected nephthys to have a primary deployment")
	}
	if primary.BuildStrategy != "dockerfile" || primary.GitURL == "" {
		t.Errorf("expected nephthys primary deployment to be a dockerfile build with a git_url, got strategy=%q git_url=%q", primary.BuildStrategy, primary.GitURL)
	}
	if len(primary.PromptedEnv) == 0 {
		t.Fatal("expected nephthys primary deployment to expose prompted env vars")
	}
	foundRequired := false
	for _, ev := range primary.PromptedEnv {
		if ev.Key == "SLACK_BOT_TOKEN" {
			foundRequired = true
			if !ev.Required {
				t.Error("expected SLACK_BOT_TOKEN to be required")
			}
		}
	}
	if !foundRequired {
		t.Error("expected SLACK_BOT_TOKEN in prompted_env")
	}
}

func TestInstallTemplateRejectsUnknownKey(t *testing.T) {
	env := newAPITestServer(t)

	body, _ := json.Marshal(installTemplateRequest{Slug: "myapp"})
	req := httptest.NewRequest(http.MethodPost, "/api/projects/1/templates/not-a-real-template/install", bytes.NewReader(body))
	req = withURLParams(req, map[string]string{"projectID": "1", "templateKey": "not-a-real-template"})
	rec := httptest.NewRecorder()
	env.server.installTemplate(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404 for unknown template, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestInstallTemplateRejectsMissingSlug(t *testing.T) {
	env := newAPITestServer(t)

	body, _ := json.Marshal(installTemplateRequest{})
	req := httptest.NewRequest(http.MethodPost, "/api/projects/1/templates/postgres/install", bytes.NewReader(body))
	req = withURLParams(req, map[string]string{"projectID": "1", "templateKey": "postgres"})
	rec := httptest.NewRecorder()
	env.server.installTemplate(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing slug, got %d: %s", rec.Code, rec.Body.String())
	}
}
