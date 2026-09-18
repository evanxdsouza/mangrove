package scheduler

import (
	"context"
	"log/slog"
	"time"

	"github.com/evanxdsouza/mangrove/internal/orchestrator"
	"github.com/evanxdsouza/mangrove/internal/store"
)

// ResourceSampler periodically records the same figures the admin
// dashboard's resource-budget view computes live (orchestrator.
// ComputeResourceBudget), turning the resource page from a right-now
// reading into a trend -- storage is cheap (one row every 5 minutes) and
// the numbers were already being computed on every dashboard load anyway.
type ResourceSampler struct {
	Orch     *orchestrator.Orchestrator
	Log      *slog.Logger
	interval time.Duration
	retain   time.Duration
}

func NewResourceSampler(orch *orchestrator.Orchestrator, log *slog.Logger) *ResourceSampler {
	return &ResourceSampler{Orch: orch, Log: log, interval: 5 * time.Minute, retain: 30 * 24 * time.Hour}
}

func (r *ResourceSampler) Run(ctx context.Context) {
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.tick(ctx)
		}
	}
}

func (r *ResourceSampler) tick(ctx context.Context) {
	budget, err := orchestrator.ComputeResourceBudget(ctx, r.Orch.Store, r.Orch.Exec, r.Orch.Config.DeploymentMemoryCeilingMB, r.Orch.Config.DataDir)
	if err != nil {
		r.Log.Warn("resource sampler: compute failed", "error", err)
		return
	}
	if err := r.Orch.Store.RecordResourceUsageSnapshot(ctx, store.ResourceUsageSnapshot{
		MemoryAllocatedMB: budget.MemoryAllocatedMB,
		MemoryUsedMB:      budget.MemoryUsedMB,
		MemoryCeilingMB:   budget.MemoryCeilingMB,
		RunningContainers: budget.RunningContainers,
		DiskTotalGB:       budget.DiskTotalGB,
		DiskUsedGB:        budget.DiskUsedGB,
	}); err != nil {
		r.Log.Warn("resource sampler: record failed", "error", err)
	}
}

// PruneOld bounds resource_usage_snapshots to a rolling 30-day window,
// same pattern as HealthChecker.PruneOldChecks.
func (r *ResourceSampler) PruneOld(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			n, err := r.Orch.Store.PruneOldResourceUsageSnapshots(ctx, time.Now().Add(-r.retain))
			if err != nil {
				r.Log.Warn("resource sampler: prune failed", "error", err)
			} else if n > 0 {
				r.Log.Info("pruned old resource usage snapshots", "rows", n)
			}
		}
	}
}
