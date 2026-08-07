// Package scheduler runs Mangrove's background tickers: health checks and
// (from a later commit) image/build-cache pruning.
package scheduler

import (
	"context"
	"log/slog"
	"time"

	"github.com/evanxdsouza/mangrove/internal/executor"
	"github.com/evanxdsouza/mangrove/internal/store"
)

// HealthChecker polls every running service's configured HTTP health check
// on its own interval and records the result, backing the dashboard's
// uptime/status view and the deploy-history health signal.
type HealthChecker struct {
	Store    *store.Store
	Exec     executor.Executor
	Log      *slog.Logger
	interval time.Duration // how often the ticker itself wakes up to check what's due
}

func NewHealthChecker(st *store.Store, exec executor.Executor, log *slog.Logger) *HealthChecker {
	return &HealthChecker{Store: st, Exec: exec, Log: log, interval: 10 * time.Second}
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
