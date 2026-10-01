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

// fakeSleeperExecutor stubs just enough of executor.Executor for
// SleepDeployment's path (Stop) -- embedding the interface the same way
// fakeReaperExecutor does, so anything this test doesn't exercise panics
// loudly instead of silently nil-dereferencing.
type fakeSleeperExecutor struct {
	executor.Executor
	stopped []string
}

func (f *fakeSleeperExecutor) Stop(ctx context.Context, ref string, timeout time.Duration) error {
	f.stopped = append(f.stopped, ref)
	return nil
}

func newSleeperTestOrchestrator(t *testing.T) (*orchestrator.Orchestrator, *store.Store, *fakeSleeperExecutor) {
	t.Helper()
	dir := t.TempDir()
	db, err := mangrovedb.Open(filepath.Join(dir, "mangrove.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	st := store.New(db)
	fake := &fakeSleeperExecutor{}
	return &orchestrator.Orchestrator{
		Store: st,
		Exec:  fake,
		Log:   slog.New(slog.NewTextHandler(os.Stderr, nil)),
	}, st, fake
}

// seedSleepCandidate inserts a running, single-service deployment with
// sleep_enabled and a backdated last_request_at, for exercising the
// sleeper's idle query directly against real SQL -- mirrors
// seedPreviewDeployment's approach in preview_reaper_test.go.
func seedSleepCandidate(t *testing.T, st *store.Store, slug string, sleepEnabled bool, idleMinutes int, lastRequestAge time.Duration, status string) int64 {
	t.Helper()
	ctx := context.Background()

	res, err := st.DB.ExecContext(ctx, `INSERT INTO projects (workspace_id, name, slug) VALUES (1, 'p', ?)`, slug+"-proj")
	if err != nil {
		t.Fatalf("insert project: %v", err)
	}
	projectID, _ := res.LastInsertId()

	lastRequestAt := time.Now().Add(-lastRequestAge).UTC().Format("2006-01-02 15:04:05")
	res, err = st.DB.ExecContext(ctx,
		`INSERT INTO deployments (project_id, name, slug, build_strategy, status, sleep_enabled, sleep_idle_minutes, last_request_at)
		 VALUES (?, 'd', ?, 'dockerfile', ?, ?, ?, ?)`,
		projectID, slug, status, sleepEnabled, idleMinutes, lastRequestAt,
	)
	if err != nil {
		t.Fatalf("insert deployment: %v", err)
	}
	depID, _ := res.LastInsertId()

	_, err = st.DB.ExecContext(ctx,
		`INSERT INTO services (deployment_id, name, container_name, container_id_current, status, internal_port)
		 VALUES (?, 'web', ?, ?, 'running', 80)`,
		depID, "mangrove-"+slug+"-web", "container-"+slug,
	)
	if err != nil {
		t.Fatalf("insert service: %v", err)
	}
	return depID
}

func TestSleeperPutsOnlyIdleSleepEnabledDeploymentsToSleep(t *testing.T) {
	orch, st, fake := newSleeperTestOrchestrator(t)
	ctx := context.Background()

	idleSleepable := seedSleepCandidate(t, st, "idle-sleepable", true, 30, 45*time.Minute, "running")
	freshSleepable := seedSleepCandidate(t, st, "fresh-sleepable", true, 30, 5*time.Minute, "running")
	idleNotEnabled := seedSleepCandidate(t, st, "idle-not-enabled", false, 30, 45*time.Minute, "running")
	idleButStopped := seedSleepCandidate(t, st, "idle-but-stopped", true, 30, 45*time.Minute, "stopped")

	sleeper := NewSleeper(orch, orch.Log)
	sleeper.tick(ctx)

	dep, err := st.GetDeployment(ctx, idleSleepable)
	if err != nil {
		t.Fatalf("GetDeployment(idleSleepable): %v", err)
	}
	if dep.Status != "sleeping" {
		t.Errorf("expected the idle sleep-enabled deployment to be put to sleep, got status %q", dep.Status)
	}
	if len(fake.stopped) != 1 || fake.stopped[0] != "container-idle-sleepable" {
		t.Errorf("expected Stop() called on the idle deployment's container, got %v", fake.stopped)
	}

	if dep, err := st.GetDeployment(ctx, freshSleepable); err != nil || dep.Status != "running" {
		t.Errorf("expected the fresh (not-yet-idle) deployment left running, got status=%q err=%v", dep.Status, err)
	}
	if dep, err := st.GetDeployment(ctx, idleNotEnabled); err != nil || dep.Status != "running" {
		t.Errorf("expected the idle-but-not-sleep-enabled deployment left running, got status=%q err=%v", dep.Status, err)
	}
	if dep, err := st.GetDeployment(ctx, idleButStopped); err != nil || dep.Status != "stopped" {
		t.Errorf("expected the already-stopped deployment left alone (not swept as a sleep candidate), got status=%q err=%v", dep.Status, err)
	}
}
