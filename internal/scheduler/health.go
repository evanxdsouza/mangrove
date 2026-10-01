// Package scheduler runs Mangrove's background tickers: health checks and
// (from a later commit) image/build-cache pruning.
package scheduler

import (
	"context"
	"log/slog"
	"time"

	"github.com/evanxdsouza/mangrove/internal/executor"
	"github.com/evanxdsouza/mangrove/internal/orchestrator"
	"github.com/evanxdsouza/mangrove/internal/store"
)

// healthFailureRestartThreshold is how many consecutive failed health
// checks a service can rack up, while self-heal is enabled on its
// deployment, before HealthChecker restarts its container -- a hung or
// deadlocked app that never actually exits, so Docker's own --restart
// policy (which only reacts to a dead process) never kicks in. Each check
// only happens once per the service's own health_check_interval_s, so this
// also doubles as a natural cooldown between restarts: a freshly-restarted
// service needs a full new streak (at minimum
// healthFailureRestartThreshold * health_check_interval_s) before it can
// trigger another one. See docs/self-healing.md.
const healthFailureRestartThreshold = 3

// HealthChecker polls every running service's configured HTTP health check
// on its own interval and records the result, backing the dashboard's
// uptime/status view and the deploy-history health signal. For a
// self-heal-enabled deployment, a service that fails
// healthFailureRestartThreshold checks in a row also gets restarted
// automatically -- see checkOne.
type HealthChecker struct {
	Orch     *orchestrator.Orchestrator
	Store    *store.Store
	Exec     executor.Executor
	Log      *slog.Logger
	interval time.Duration // how often the ticker itself wakes up to check what's due
}

func NewHealthChecker(orch *orchestrator.Orchestrator, log *slog.Logger) *HealthChecker {
	return &HealthChecker{Orch: orch, Store: orch.Store, Exec: orch.Exec, Log: log, interval: 10 * time.Second}
}

// Run blocks, ticking until ctx is canceled.
func (h *HealthChecker) Run(ctx context.Context) {
	ticker := time.NewTicker(h.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			h.tick(ctx)
		}
	}
}

func (h *HealthChecker) tick(ctx context.Context) {
	services, err := h.Store.ListRunningServicesWithHealthCheck(ctx)
	if err != nil {
		h.Log.Warn("health checker: list services failed", "error", err)
		return
	}
	for _, svc := range services {
		due, err := h.dueForCheck(ctx, svc.ID, svc.HealthCheckIntervalS)
		if err != nil || !due {
			continue
		}
		h.checkOne(ctx, svc)
	}
}

func (h *HealthChecker) dueForCheck(ctx context.Context, serviceID int64, intervalS int) (bool, error) {
	_, lastCheckedAt, err := h.Store.LatestHealthCheck(ctx, serviceID)
	if err != nil {
		return false, err
	}
	if lastCheckedAt.IsZero() {
		return true, nil
	}
	return time.Since(lastCheckedAt) >= time.Duration(intervalS)*time.Second, nil
}

func (h *HealthChecker) checkOne(ctx context.Context, svc store.RunnableService) {
	status, err := h.Exec.HealthCheck(ctx, svc.ContainerID, executor.HealthCheckSpec{
		Path:           svc.HealthCheckPath,
		Port:           svc.InternalPort,
		TimeoutSeconds: svc.HealthCheckTimeoutS,
	})
	if err != nil {
		h.Store.RecordHealthCheck(ctx, svc.ID, "error", 0, 0, err.Error())
		h.recordFailureStreak(ctx, svc, true)
		return
	}

	result := "unhealthy"
	if status.Healthy {
		result = "healthy"
	} else if status.Error != "" && status.StatusCode == 0 {
		result = "timeout"
	}
	if err := h.Store.RecordHealthCheck(ctx, svc.ID, result, status.ResponseTimeMS, status.StatusCode, status.Error); err != nil {
		h.Log.Warn("failed to record health check", "service_id", svc.ID, "error", err)
	}
	h.recordFailureStreak(ctx, svc, !status.Healthy)
}

// recordFailureStreak updates svc's consecutive-failure counter and, once a
// self-heal-enabled deployment's service crosses healthFailureRestartThreshold,
// triggers an automatic restart. Guarded on the deployment still being
// "running" -- a deploy already in flight (building/healthchecking) handles
// its own health-check gating in Deploy() and must not be interrupted by
// this entirely separate mechanism.
func (h *HealthChecker) recordFailureStreak(ctx context.Context, svc store.RunnableService, failed bool) {
	count := 0
	if failed {
		count = svc.ConsecutiveHealthFailures + 1
	}
	if err := h.Store.SetServiceHealthFailureStreak(ctx, svc.ID, count); err != nil {
		h.Log.Warn("health checker: update failure streak failed", "service_id", svc.ID, "error", err)
	}
	if !failed || !svc.SelfHealEnabled || count < healthFailureRestartThreshold {
		return
	}
	dep, err := h.Store.GetDeployment(ctx, svc.DeploymentID)
	if err != nil || dep.Status != "running" {
		return
	}
	h.Log.Info("health checker: self-healing restart after repeated failed health checks", "service_id", svc.ID, "deployment_id", svc.DeploymentID, "consecutive_failures", count)
	if err := h.Orch.RestartDeployment(ctx, svc.DeploymentID); err != nil {
		h.Log.Warn("health checker: self-healing restart failed", "deployment_id", svc.DeploymentID, "error", err)
	}
	// Reset regardless of outcome: a failed restart attempt shouldn't spin
	// on every subsequent tick retrying the restart itself -- the service
	// needs a fresh full streak (naturally rate-limited by the health-check
	// interval, see healthFailureRestartThreshold's doc comment) before
	// this fires again.
	if err := h.Store.SetServiceHealthFailureStreak(ctx, svc.ID, 0); err != nil {
		h.Log.Warn("health checker: reset failure streak after restart failed", "service_id", svc.ID, "error", err)
	}
}

// PruneOldChecks periodically bounds the health_checks table (per plan §2).
func (h *HealthChecker) PruneOldChecks(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			n, err := h.Store.PruneOldHealthChecks(ctx, time.Now().Add(-7*24*time.Hour))
			if err != nil {
				h.Log.Warn("prune old health checks failed", "error", err)
			} else if n > 0 {
				h.Log.Info("pruned old health checks", "rows", n)
			}
		}
	}
}
