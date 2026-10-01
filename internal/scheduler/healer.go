package scheduler

import (
	"context"
	"log/slog"
	"time"

	"github.com/evanxdsouza/mangrove/internal/models"
	"github.com/evanxdsouza/mangrove/internal/orchestrator"
)

// Healer automatically retries a deployment that failed outright, the
// deploy-level half of the self-healing feature (internal/scheduler/
// health.go's restart-on-repeated-health-failure is the other half). Ticks
// at the same cadence as Sleeper since its own backoff schedule (1 minute,
// then 5) needs sub-hourly resolution, unlike PreviewReaper's daily-scale
// sweep.
type Healer struct {
	Orch     *orchestrator.Orchestrator
	Log      *slog.Logger
	interval time.Duration
}

func NewHealer(orch *orchestrator.Orchestrator, log *slog.Logger) *Healer {
	return &Healer{Orch: orch, Log: log, interval: 1 * time.Minute}
}

func (h *Healer) Run(ctx context.Context) {
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

func (h *Healer) tick(ctx context.Context) {
	candidates, err := h.Orch.Store.ListAutoRetryCandidates(ctx)
	if err != nil {
		h.Log.Warn("healer: list auto-retry candidates failed", "error", err)
		return
	}
	for _, dep := range candidates {
		h.retry(ctx, dep)
	}
}

func (h *Healer) retry(ctx context.Context, dep models.Deployment) {
	// Count the attempt before it runs, not after: a retry that itself
	// fails must still count toward the cap, or a persistently broken
	// deployment would retry forever.
	if err := h.Orch.Store.IncrementDeploymentAutoRetryCount(ctx, dep.ID); err != nil {
		h.Log.Warn("healer: increment retry count failed", "deployment_id", dep.ID, "error", err)
		return
	}
	deployReq, err := h.Orch.BuildRedeployRequest(ctx, dep)
	if err != nil {
		h.Log.Warn("healer: resolve redeploy source failed, skipping this attempt", "deployment_id", dep.ID, "error", err)
		return
	}
	deployReq.TriggeredBy = "auto-retry"

	err = h.Orch.WithInflightDeploy(dep.ID, func(ctx context.Context) error {
		_, e := h.Orch.DispatchDeploy(ctx, dep, deployReq)
		return e
	})
	if err != nil {
		h.Log.Warn("healer: auto-retry deploy failed", "deployment_id", dep.ID, "slug", dep.Slug, "attempt", dep.AutoRetryCount+1, "error", err)
		return
	}
	h.Log.Info("healer: auto-retried a failed deploy", "deployment_id", dep.ID, "slug", dep.Slug, "attempt", dep.AutoRetryCount+1)
}
