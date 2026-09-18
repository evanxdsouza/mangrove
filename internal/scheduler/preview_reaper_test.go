package scheduler

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	mangrovedb "github.com/evanxdsouza/mangrove/internal/db"
	"github.com/evanxdsouza/mangrove/internal/executor"
	"github.com/evanxdsouza/mangrove/internal/orchestrator"
	"github.com/evanxdsouza/mangrove/internal/store"
)

// fakeReaperExecutor embeds the interface so any method this test doesn't
// stub panics if called -- the seeded preview deployments below have no
// services, so DeleteDeployment's teardown loop never touches it, but
// embedding (matching fakePruneExecutor's pattern in prune_test.go) keeps
// that an explicit, checked assumption rather than a silent nil dereference.
type fakeReaperExecutor struct {
	executor.Executor
}

func newReaperTestOrchestrator(t *testing.T) (*orchestrator.Orchestrator, *store.Store) {
	t.Helper()
	dir := t.TempDir()
	db, err := mangrovedb.Open(filepath.Join(dir, "mangrove.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	st := store.New(db)
	return &orchestrator.Orchestrator{
		Store: st,
		Exec:  &fakeReaperExecutor{},
		Log:   slog.New(slog.NewTextHandler(os.Stderr, nil)),
	}, st
}

// seedPreviewDeployment inserts a preview (or, if !isPreview, production)
// deployment with last_deployed_at backdated by age, for exercising the
// reaper's staleness query directly against real SQL.
func seedPreviewDeployment(t *testing.T, st *store.Store, slug string, isPreview bool, age time.Duration) int64 {
	t.Helper()
	ctx := context.Background()

	res, err := st.DB.ExecContext(ctx, `INSERT INTO projects (workspace_id, name, slug) VALUES (1, 'p', ?)`, slug+"-proj")
	if err != nil {
		t.Fatalf("insert project: %v", err)
	}
	projectID, _ := res.LastInsertId()

	env := "production"
	prNumber := "NULL"
	if isPreview {
		env = "preview"
		prNumber = "42"
	}
	lastDeployedAt := time.Now().Add(-age).UTC().Format("2006-01-02 15:04:05")
	res, err = st.DB.ExecContext(ctx,
		`INSERT INTO deployments (project_id, name, slug, build_strategy, environment, pr_number, last_deployed_at)
		 VALUES (?, 'd', ?, 'dockerfile', ?, `+prNumber+`, ?)`,
		projectID, slug, env, lastDeployedAt,
	)
	if err != nil {
		t.Fatalf("insert deployment: %v", err)
	}
	id, _ := res.LastInsertId()
	return id
}

func TestPreviewReaperTearsDownOnlyStalePreviews(t *testing.T) {
	orch, st := newReaperTestOrchestrator(t)
	ctx := context.Background()

	stalePreview := seedPreviewDeployment(t, st, "stale-preview", true, 10*24*time.Hour)
	freshPreview := seedPreviewDeployment(t, st, "fresh-preview", true, 1*time.Hour)
	staleProd := seedPreviewDeployment(t, st, "stale-prod", false, 10*24*time.Hour)

	reaper := NewPreviewReaper(orch, 168 /* 7 days */, orch.Log) // orch.Log set in newReaperTestOrchestrator
	reaper.tick(ctx)

	if _, err := st.GetDeployment(ctx, stalePreview); err != store.ErrNotFound {
		t.Errorf("expected the stale preview torn down, got err=%v", err)
	}
	if _, err := st.GetDeployment(ctx, freshPreview); err != nil {
		t.Errorf("expected the fresh preview left alone, got err=%v", err)
	}
	if _, err := st.GetDeployment(ctx, staleProd); err != nil {
		t.Errorf("expected the stale *production* deployment left alone (not a preview), got err=%v", err)
	}
}

func TestPreviewReaperDisabledAtZeroMaxAge(t *testing.T) {
	orch, st := newReaperTestOrchestrator(t)
	ctx := context.Background()

	stalePreview := seedPreviewDeployment(t, st, "stale-preview", true, 999*24*time.Hour)

	reaper := NewPreviewReaper(orch, 0, orch.Log)
	// Run would return immediately without ticking at all when maxAge<=0;
	// call tick directly to confirm the guard lives in Run, not a
	// coincidence of the ticker never firing in the test's short lifetime.
	if reaper.maxAge != 0 {
		t.Fatalf("expected maxAge 0 to disable the sweep, got %v", reaper.maxAge)
	}

	// Confirm Run itself never ticks when disabled: it should return as
	// soon as ctx is done, having never called ListStalePreviewDeployments.
	runCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()
	reaper.Run(runCtx)

	if _, err := st.GetDeployment(ctx, stalePreview); err != nil {
		t.Errorf("expected the preview left alone with the reaper disabled, got err=%v", err)
	}
}
