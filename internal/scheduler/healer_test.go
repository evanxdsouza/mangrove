package scheduler

import (
	"context"
	"errors"
	"io"
	"path/filepath"
	"testing"
	"time"

	"github.com/evanxdsouza/mangrove/internal/config"
	mangrovedb "github.com/evanxdsouza/mangrove/internal/db"
	"github.com/evanxdsouza/mangrove/internal/executor"
	"github.com/evanxdsouza/mangrove/internal/orchestrator"
	"github.com/evanxdsouza/mangrove/internal/store"
)

// fakeHealerExecutor fails Build deterministically -- Deploy() calls Build
// even for the image strategy (to resolve/tag the ref), so this is the
// earliest, cleanest point to make every retry attempt fail predictably
// without needing a real Docker daemon.
type fakeHealerExecutor struct {
	executor.Executor
	buildCalls int
}

func (f *fakeHealerExecutor) Build(ctx context.Context, spec executor.BuildSpec, logs io.Writer) (executor.BuildResult, error) {
	f.buildCalls++
	return executor.BuildResult{}, errors.New("simulated persistent build failure")
}

func newHealerTestOrchestrator(t *testing.T) (*orchestrator.Orchestrator, *store.Store, *fakeHealerExecutor) {
	t.Helper()
	dir := t.TempDir()
	db, err := mangrovedb.Open(filepath.Join(dir, "mangrove.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	st := store.New(db)
	fake := &fakeHealerExecutor{}
	return &orchestrator.Orchestrator{
		Store:  st,
		Exec:   fake,
		Log:    discardLogger(),
		Config: config.Config{PortRangeMin: 21000, PortRangeMax: 21999, DeploymentMemoryCeilingMB: 4096},
	}, st, fake
}

// seedFailedSelfHealDeployment creates a self-heal-enabled, image-strategy
// deployment (so BuildRedeployRequest needs no linked repo) already marked
// "failed", with updated_at backdated past backoffMinutes -- a candidate
// ListAutoRetryCandidates should pick up immediately rather than waiting out
// the real backoff window in the test.
func seedFailedSelfHealDeployment(t *testing.T, st *store.Store, slug string, backoffMinutes time.Duration) int64 {
	t.Helper()
	ctx := context.Background()

	res, err := st.DB.ExecContext(ctx, `INSERT INTO projects (workspace_id, name, slug) VALUES (1, 'p', ?)`, slug+"-proj")
	if err != nil {
		t.Fatalf("insert project: %v", err)
	}
	projectID, _ := res.LastInsertId()

	dep, err := st.CreateDeployment(ctx, store.CreateDeploymentParams{
		ProjectID: projectID, Name: "web", Slug: slug, BuildStrategy: "image", ImageRef: "nginx:alpine",
	})
	if err != nil {
		t.Fatalf("CreateDeployment: %v", err)
	}
	if _, err := st.CreateService(ctx, store.CreateServiceParams{
		DeploymentID: dep.ID, Name: "web", ContainerName: "mangrove-" + slug + "-web", InternalPort: 80,
	}); err != nil {
		t.Fatalf("CreateService: %v", err)
	}
	if err := st.SetSelfHealEnabled(ctx, dep.ID, true); err != nil {
		t.Fatalf("SetSelfHealEnabled: %v", err)
	}
	if err := st.UpdateDeploymentStatus(ctx, dep.ID, "failed"); err != nil {
		t.Fatalf("UpdateDeploymentStatus: %v", err)
	}
	backdated := time.Now().Add(-backoffMinutes).UTC().Format("2006-01-02 15:04:05")
	if _, err := st.DB.ExecContext(ctx, `UPDATE deployments SET updated_at = ? WHERE id = ?`, backdated, dep.ID); err != nil {
		t.Fatalf("backdate updated_at: %v", err)
	}
	return dep.ID
}

func TestHealerRetriesAndCapsFailedDeploys(t *testing.T) {
	orch, st, fake := newHealerTestOrchestrator(t)
	ctx := context.Background()

	depID := seedFailedSelfHealDeployment(t, st, "retry-me", 2*time.Minute)
	healer := NewHealer(orch, orch.Log)

	healer.tick(ctx)

	if fake.buildCalls != 1 {
		t.Fatalf("expected one retry attempt (Build called once), got %d", fake.buildCalls)
	}
	dep, err := st.GetDeployment(ctx, depID)
	if err != nil {
		t.Fatalf("GetDeployment: %v", err)
	}
	if dep.AutoRetryCount != 1 {
		t.Errorf("expected auto_retry_count=1 after one failed retry, got %d", dep.AutoRetryCount)
	}
	if dep.Status != "failed" {
		t.Errorf("expected status still 'failed' (the retry itself failed too), got %q", dep.Status)
	}

	// Immediately ticking again shouldn't retry yet -- the second attempt's
	// backoff is 5 minutes, and updated_at was just refreshed by the first
	// attempt's own failure.
	healer.tick(ctx)
	if fake.buildCalls != 1 {
		t.Errorf("expected no second retry before its own backoff elapses, got %d build calls", fake.buildCalls)
	}

	// Backdate past the second attempt's 5-minute backoff and let it retry
	// again -- this should be the last one (cap is 2).
	if _, err := st.DB.ExecContext(ctx, `UPDATE deployments SET updated_at = ? WHERE id = ?`,
		time.Now().Add(-6*time.Minute).UTC().Format("2006-01-02 15:04:05"), depID); err != nil {
		t.Fatalf("backdate updated_at: %v", err)
	}
	healer.tick(ctx)
	if fake.buildCalls != 2 {
		t.Fatalf("expected a second retry attempt, got %d build calls", fake.buildCalls)
	}
	dep, _ = st.GetDeployment(ctx, depID)
	if dep.AutoRetryCount != 2 {
		t.Errorf("expected auto_retry_count=2 after the second failed retry, got %d", dep.AutoRetryCount)
	}

	// The cap is 2 -- even backdated indefinitely, a third attempt must not
	// happen.
	if _, err := st.DB.ExecContext(ctx, `UPDATE deployments SET updated_at = ? WHERE id = ?`,
		time.Now().Add(-999*time.Minute).UTC().Format("2006-01-02 15:04:05"), depID); err != nil {
		t.Fatalf("backdate updated_at: %v", err)
	}
	healer.tick(ctx)
	if fake.buildCalls != 2 {
		t.Errorf("expected no third retry past the cap, got %d build calls", fake.buildCalls)
	}
}

func TestHealerIgnoresNonSelfHealAndNonFailedDeployments(t *testing.T) {
	orch, st, fake := newHealerTestOrchestrator(t)
	ctx := context.Background()

	// Self-heal disabled.
	dep1, _ := st.CreateDeployment(ctx, store.CreateDeploymentParams{ProjectID: 1, Name: "d1", Slug: "d1", BuildStrategy: "image", ImageRef: "nginx:alpine"})
	st.CreateService(ctx, store.CreateServiceParams{DeploymentID: dep1.ID, Name: "web", ContainerName: "mangrove-d1-web", InternalPort: 80})
	st.UpdateDeploymentStatus(ctx, dep1.ID, "failed")

	// Self-heal enabled but still running, not failed.
	depID2 := seedFailedSelfHealDeployment(t, st, "d2", 2*time.Minute)
	st.UpdateDeploymentStatus(ctx, depID2, "running")

	healer := NewHealer(orch, orch.Log)
	healer.tick(ctx)

	if fake.buildCalls != 0 {
		t.Errorf("expected no retry attempts, got %d", fake.buildCalls)
	}
}
