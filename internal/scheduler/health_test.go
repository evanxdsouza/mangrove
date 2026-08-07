package scheduler

import (
	"context"
	"io"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	mangrovedb "github.com/evanxdsouza/mangrove/internal/db"
	"github.com/evanxdsouza/mangrove/internal/executor"
	"github.com/evanxdsouza/mangrove/internal/store"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// fakeExecutor implements executor.Executor with a scripted HealthCheck
// response, so the scheduler's due-for-check/record logic can be tested
// without a real Docker daemon.
type fakeExecutor struct {
	executor.Executor
	healthResponses map[string]executor.HealthStatus
	calls           []string
}

func (f *fakeExecutor) HealthCheck(ctx context.Context, containerRef string, cfg executor.HealthCheckSpec) (executor.HealthStatus, error) {
	f.calls = append(f.calls, containerRef)
	if resp, ok := f.healthResponses[containerRef]; ok {
		return resp, nil
	}
	return executor.HealthStatus{Healthy: false}, nil
}

func testStore(t *testing.T) *store.Store {
	t.Helper()
	dir := t.TempDir()
	db, err := mangrovedb.Open(filepath.Join(dir, "mangrove.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return store.New(db)
}

func seedRunningService(t *testing.T, st *store.Store) int64 {
	t.Helper()
	ctx := context.Background()
	res, err := st.DB.ExecContext(ctx, `INSERT INTO projects (workspace_id, name, slug) VALUES (1, 'p', 'p')`)
	if err != nil {
		t.Fatalf("insert project: %v", err)
	}
	projectID, _ := res.LastInsertId()
	res, err = st.DB.ExecContext(ctx, `INSERT INTO deployments (project_id, name, slug, build_strategy) VALUES (?, 'd', 'd', 'dockerfile')`, projectID)
	if err != nil {
		t.Fatalf("insert deployment: %v", err)
	}
	deploymentID, _ := res.LastInsertId()
	res, err = st.DB.ExecContext(ctx, `
		INSERT INTO services (deployment_id, name, container_name, container_id_current, status, internal_port, health_check_path, health_check_interval_s, health_check_timeout_s)
		VALUES (?, 'web', 'mangrove-p-web', 'container123', 'running', 80, '/', 30, 5)`, deploymentID)
	if err != nil {
		t.Fatalf("insert service: %v", err)
	}
	id, _ := res.LastInsertId()
	return id
}

func TestTickChecksRunningServiceAndRecordsResult(t *testing.T) {
	st := testStore(t)
	svcID := seedRunningService(t, st)

	fake := &fakeExecutor{healthResponses: map[string]executor.HealthStatus{
		"container123": {Healthy: true, StatusCode: 200, ResponseTimeMS: 12},
	}}
	hc := NewHealthChecker(st, fake, discardLogger())

	hc.tick(context.Background())

	status, _, err := st.LatestHealthCheck(context.Background(), svcID)
	if err != nil {
		t.Fatalf("LatestHealthCheck: %v", err)
	}
	if status != "healthy" {
		t.Errorf("expected status healthy, got %q", status)
	}
	if len(fake.calls) != 1 || fake.calls[0] != "container123" {
		t.Errorf("expected exactly one HealthCheck call against container123, got %v", fake.calls)
	}
}

func TestTickSkipsServiceNotYetDue(t *testing.T) {
	st := testStore(t)
	svcID := seedRunningService(t, st)

	fake := &fakeExecutor{healthResponses: map[string]executor.HealthStatus{"container123": {Healthy: true}}}
	hc := NewHealthChecker(st, fake, discardLogger())

	hc.tick(context.Background()) // first check runs (no prior record)
	if len(fake.calls) != 1 {
		t.Fatalf("expected 1 call after first tick, got %d", len(fake.calls))
	}

	hc.tick(context.Background()) // interval is 30s, so immediately re-ticking should be a no-op
	if len(fake.calls) != 1 {
		t.Errorf("expected still 1 call (not due yet), got %d", len(fake.calls))
	}
	_ = svcID
}

func TestCheckOneRecordsUnhealthyOnFailedStatus(t *testing.T) {
	st := testStore(t)
	svcID := seedRunningService(t, st)

	fake := &fakeExecutor{healthResponses: map[string]executor.HealthStatus{
		"container123": {Healthy: false, StatusCode: 503},
	}}
	hc := NewHealthChecker(st, fake, discardLogger())
	hc.tick(context.Background())

	status, _, err := st.LatestHealthCheck(context.Background(), svcID)
	if err != nil {
		t.Fatalf("LatestHealthCheck: %v", err)
	}
	if status != "unhealthy" {
		t.Errorf("expected status unhealthy, got %q", status)
	}
}

func TestPruneOldChecksRemovesOldRows(t *testing.T) {
	st := testStore(t)
	svcID := seedRunningService(t, st)
	ctx := context.Background()

	st.RecordHealthCheck(ctx, svcID, "healthy", 5, 200, "")
	// Backdate it directly since RecordHealthCheck always uses CURRENT_TIMESTAMP.
	st.DB.ExecContext(ctx, `UPDATE health_checks SET checked_at = ? WHERE service_id = ?`, time.Now().Add(-10*24*time.Hour), svcID)

	n, err := st.PruneOldHealthChecks(ctx, time.Now().Add(-7*24*time.Hour))
	if err != nil {
		t.Fatalf("PruneOldHealthChecks: %v", err)
	}
	if n != 1 {
		t.Errorf("expected 1 row pruned, got %d", n)
	}
}
