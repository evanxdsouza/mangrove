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
	"github.com/evanxdsouza/mangrove/internal/orchestrator"
	"github.com/evanxdsouza/mangrove/internal/store"
)

// newTestHealthChecker wraps st/exec in a minimal Orchestrator -- health.go
// needs one (not just a bare Store/Executor) to trigger a self-healing
// RestartDeployment.
func newTestHealthChecker(st *store.Store, exec executor.Executor, log *slog.Logger) *HealthChecker {
	orch := &orchestrator.Orchestrator{Store: st, Exec: exec, Log: log}
	return NewHealthChecker(orch, log)
}

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
	restarted       []string
}

func (f *fakeExecutor) HealthCheck(ctx context.Context, containerRef string, cfg executor.HealthCheckSpec) (executor.HealthStatus, error) {
	f.calls = append(f.calls, containerRef)
	if resp, ok := f.healthResponses[containerRef]; ok {
		return resp, nil
	}
	return executor.HealthStatus{Healthy: false}, nil
}

func (f *fakeExecutor) Restart(ctx context.Context, containerRef string, timeout time.Duration) error {
	f.restarted = append(f.restarted, containerRef)
	return nil
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

// seedSelfHealingService is seedRunningService plus self_heal_enabled on
// the deployment and the deployment's own status actually set to "running"
// (seedRunningService's plain INSERT leaves it at the schema default,
// "pending", which recordFailureStreak's dep.Status=="running" guard would
// correctly refuse to act on).
func seedSelfHealingService(t *testing.T, st *store.Store) (serviceID, deploymentID int64) {
	t.Helper()
	ctx := context.Background()
	serviceID = seedRunningService(t, st)
	if err := st.DB.QueryRowContext(ctx, `SELECT deployment_id FROM services WHERE id = ?`, serviceID).Scan(&deploymentID); err != nil {
		t.Fatalf("resolve deployment id: %v", err)
	}
	if _, err := st.DB.ExecContext(ctx, `UPDATE deployments SET status = 'running', self_heal_enabled = 1 WHERE id = ?`, deploymentID); err != nil {
		t.Fatalf("enable self-heal: %v", err)
	}
	return serviceID, deploymentID
}

func TestTickChecksRunningServiceAndRecordsResult(t *testing.T) {
	st := testStore(t)
	svcID := seedRunningService(t, st)

	fake := &fakeExecutor{healthResponses: map[string]executor.HealthStatus{
		"container123": {Healthy: true, StatusCode: 200, ResponseTimeMS: 12},
	}}
	hc := newTestHealthChecker(st, fake, discardLogger())

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
	hc := newTestHealthChecker(st, fake, discardLogger())

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
	hc := newTestHealthChecker(st, fake, discardLogger())
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

// TestSelfHealRestartsAfterConsecutiveFailures guards the health-check half
// of self-healing: a self-heal-enabled, running deployment whose service
// fails healthFailureRestartThreshold checks in a row gets its container
// restarted, and the failure streak resets afterward so it needs a fresh
// full streak before firing again.
func TestSelfHealRestartsAfterConsecutiveFailures(t *testing.T) {
	st := testStore(t)
	seedSelfHealingService(t, st)
	ctx := context.Background()

	fake := &fakeExecutor{healthResponses: map[string]executor.HealthStatus{
		"container123": {Healthy: false, StatusCode: 503},
	}}
	hc := newTestHealthChecker(st, fake, discardLogger())

	for i := 0; i < healthFailureRestartThreshold; i++ {
		services, err := st.ListRunningServicesWithHealthCheck(ctx)
		if err != nil || len(services) != 1 {
			t.Fatalf("ListRunningServicesWithHealthCheck: %v (got %d)", err, len(services))
		}
		hc.checkOne(ctx, services[0])
	}

	if len(fake.restarted) != 1 || fake.restarted[0] != "container123" {
		t.Errorf("expected exactly one Restart() call against container123 after %d consecutive failures, got %v", healthFailureRestartThreshold, fake.restarted)
	}

	services, err := st.ListRunningServicesWithHealthCheck(ctx)
	if err != nil || len(services) != 1 {
		t.Fatalf("ListRunningServicesWithHealthCheck after restart: %v (got %d)", err, len(services))
	}
	if services[0].ConsecutiveHealthFailures != 0 {
		t.Errorf("expected failure streak reset to 0 after restart, got %d", services[0].ConsecutiveHealthFailures)
	}
}

// TestSelfHealNotTriggeredWhenDisabled confirms a service with no self-heal
// opt-in never gets auto-restarted, no matter how many checks fail in a
// row -- it's opt-in, not a default behavior change for every existing
// deployment.
func TestSelfHealNotTriggeredWhenDisabled(t *testing.T) {
	st := testStore(t)
	seedRunningService(t, st) // self_heal_enabled defaults to 0
	ctx := context.Background()

	fake := &fakeExecutor{healthResponses: map[string]executor.HealthStatus{
		"container123": {Healthy: false, StatusCode: 503},
	}}
	hc := newTestHealthChecker(st, fake, discardLogger())

	for i := 0; i < healthFailureRestartThreshold+2; i++ {
		services, err := st.ListRunningServicesWithHealthCheck(ctx)
		if err != nil || len(services) != 1 {
			t.Fatalf("ListRunningServicesWithHealthCheck: %v (got %d)", err, len(services))
		}
		hc.checkOne(ctx, services[0])
	}

	if len(fake.restarted) != 0 {
		t.Errorf("expected no restart with self-heal disabled, got %v", fake.restarted)
	}
}

// TestSelfHealFailureStreakResetsOnSuccess confirms a single healthy check
// clears an in-progress failure streak rather than it persisting toward the
// threshold across unrelated, separated incidents.
func TestSelfHealFailureStreakResetsOnSuccess(t *testing.T) {
	st := testStore(t)
	seedSelfHealingService(t, st)
	ctx := context.Background()

	fake := &fakeExecutor{healthResponses: map[string]executor.HealthStatus{
		"container123": {Healthy: false, StatusCode: 503},
	}}
	hc := newTestHealthChecker(st, fake, discardLogger())

	for i := 0; i < healthFailureRestartThreshold-1; i++ {
		services, _ := st.ListRunningServicesWithHealthCheck(ctx)
		hc.checkOne(ctx, services[0])
	}
	services, _ := st.ListRunningServicesWithHealthCheck(ctx)
	if services[0].ConsecutiveHealthFailures != healthFailureRestartThreshold-1 {
		t.Fatalf("expected a %d-failure streak before the healthy check, got %d", healthFailureRestartThreshold-1, services[0].ConsecutiveHealthFailures)
	}

	fake.healthResponses["container123"] = executor.HealthStatus{Healthy: true, StatusCode: 200}
	hc.checkOne(ctx, services[0])

	services, _ = st.ListRunningServicesWithHealthCheck(ctx)
	if services[0].ConsecutiveHealthFailures != 0 {
		t.Errorf("expected the streak to reset to 0 after a healthy check, got %d", services[0].ConsecutiveHealthFailures)
	}
	if len(fake.restarted) != 0 {
		t.Errorf("expected no restart -- the streak never reached the threshold, got %v", fake.restarted)
	}
}
