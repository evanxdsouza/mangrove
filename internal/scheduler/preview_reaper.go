package scheduler

import (
	"context"
	"log/slog"
	"time"

	"github.com/evanxdsouza/mangrove/internal/orchestrator"
)

// PreviewReaper tears down PR-preview deployments that have gone stale --
// the backstop for internal/api/webhook.go's normal "PR closed" teardown,
// which depends on GitHub actually delivering that webhook. A delivery
// that's missed (misconfigured webhook, GitHub outage, Mangrove down at
// the moment it fired) or a PR simply left open forever otherwise leaves
// the preview running -- and counted against the deployment-memory
// admission budget -- indefinitely.
type PreviewReaper struct {
	Orch     *orchestrator.Orchestrator
	Log      *slog.Logger
	maxAge   time.Duration // 0 disables the sweep
	interval time.Duration
}

func NewPreviewReaper(orch *orchestrator.Orchestrator, maxAgeHours int, log *slog.Logger) *PreviewReaper {
	return &PreviewReaper{
		Orch:     orch,
		Log:      log,
		maxAge:   time.Duration(maxAgeHours) * time.Hour,
		interval: 1 * time.Hour,
	}
}

func (p *PreviewReaper) Run(ctx context.Context) {
	if p.maxAge <= 0 {
		return
	}
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.tick(ctx)
		}
	}
}

func (p *PreviewReaper) tick(ctx context.Context) {
	stale, err := p.Orch.Store.ListStalePreviewDeployments(ctx, time.Now().Add(-p.maxAge))
	if err != nil {
		p.Log.Warn("preview reaper: list stale previews failed", "error", err)
		return
	}
	for _, dep := range stale {
		if err := p.Orch.DeleteDeployment(ctx, dep.ID); err != nil {
			p.Log.Warn("preview reaper: teardown failed", "deployment_id", dep.ID, "pr", dep.PRNumber, "error", err)
			continue
		}
		p.Log.Info("preview reaper: tore down a stale preview", "deployment_id", dep.ID, "pr", dep.PRNumber, "slug", dep.Slug)
	}
}
