package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/evanxdsouza/mangrove/internal/store"
)

// seedSleepDeployment creates a sleep-enabled (not password-protected)
// deployment with the given status, mirroring seedGatedDeployment in
// gate_test.go but for the wake flow instead of the password gate.
func seedSleepDeployment(t *testing.T, env *testEnv, status string) int64 {
	t.Helper()
	ctx := t.Context()

	proj, err := env.store.CreateProject(ctx, 1, "Sleep Test", "sleep-test", "")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	dep, err := env.store.CreateDeployment(ctx, store.CreateDeploymentParams{
		ProjectID: proj.ID, Name: "web", Slug: "sleep-web", BuildStrategy: "dockerfile",
	})
	if err != nil {
		t.Fatalf("CreateDeployment: %v", err)
	}
	if _, err := env.store.CreateService(ctx, store.CreateServiceParams{
		DeploymentID: dep.ID, Name: "web", ContainerName: "mangrove-sleep-web-web", InternalPort: 3000,
	}); err != nil {
		t.Fatalf("CreateService: %v", err)
	}
	if err := env.store.SetDeploymentSleepConfig(ctx, dep.ID, true, 30); err != nil {
		t.Fatalf("SetDeploymentSleepConfig: %v", err)
	}
	if err := env.store.UpdateDeploymentStatus(ctx, dep.ID, status); err != nil {
		t.Fatalf("UpdateDeploymentStatus: %v", err)
	}
	return dep.ID
}

func TestGateSleepingDeploymentServesWakingPage(t *testing.T) {
	env := newTestEnv(t)
	depID := seedSleepDeployment(t, env, "sleeping")
	h := env.server.gateIntercept(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("request should have been intercepted by the wake handler, not passed through to the real router")
	}))

	w := httptest.NewRecorder()
	h.ServeHTTP(w, gatedRequest(http.MethodGet, "/", depID, nil))

	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "Waking up") {
		t.Errorf("expected the waking-up page, got status %d body %s", w.Code, w.Body.String())
	}

	dep, err := env.store.GetDeployment(t.Context(), depID)
	if err != nil {
		t.Fatalf("GetDeployment: %v", err)
	}
	if dep.LastRequestAt == nil {
		t.Errorf("expected last_request_at to be bumped by the request reaching the wake handler")
	}
}

// TestGateRunningSleepDeploymentProxiesThrough confirms a sleep-enabled
// deployment that's actually running doesn't get the waking page -- it
// should fall straight through to the normal proxy-through path. There's no
// real container in this test, so getting as far as a 502 (not the waking
// page, not a 200) is the proof it tried to proxy for real -- the same
// pattern TestGatePublicPathsSkipGate uses in gate_test.go.
func TestGateRunningSleepDeploymentProxiesThrough(t *testing.T) {
	env := newTestEnv(t)
	depID := seedSleepDeployment(t, env, "running")
	h := env.server.gateIntercept(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	w := httptest.NewRecorder()
	h.ServeHTTP(w, gatedRequest(http.MethodGet, "/", depID, nil))

	if w.Code != http.StatusBadGateway || strings.Contains(w.Body.String(), "Waking up") {
		t.Errorf("expected a proxy-through attempt (502, no container), got status %d body %s", w.Code, w.Body.String())
	}
}
