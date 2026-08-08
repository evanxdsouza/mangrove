package scheduler

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/evanxdsouza/mangrove/internal/executor"
	"github.com/evanxdsouza/mangrove/internal/models"
	"github.com/evanxdsouza/mangrove/internal/store"
)

// Pruner runs Docker image/build-cache pruning on a schedule -- without
// this, 16GB of disk fills up fast on a box building images regularly (per
// plan). Each deployment keeps its own configured retention count; this
// never touches the currently-live image for a service (it's always among
// the newest N kept) or races a deploy in progress (Prune only ever
// removes untagged/dangling layers and images not in the keep list).
type Pruner struct {
	Store    *store.Store
	Exec     executor.Executor
	Log      *slog.Logger
	interval time.Duration
}

func NewPruner(st *store.Store, exec executor.Executor, log *slog.Logger) *Pruner {
	return &Pruner{Store: st, Exec: exec, Log: log, interval: 6 * time.Hour}
}

func (p *Pruner) Run(ctx context.Context) {
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

func (p *Pruner) tick(ctx context.Context) {
	deployments, err := p.Store.ListAllDeployments(ctx)
	if err != nil {
		p.Log.Warn("prune: failed to list deployments", "error", err)
		return
	}

	for _, dep := range deployments {
		services, err := p.Store.ListServices(ctx, dep.ID)
		if err != nil {
			p.Log.Warn("prune: failed to list services", "deployment_id", dep.ID, "error", err)
			continue
		}
		for _, svc := range services {
			artifacts, err := p.Store.ListArtifactsForService(ctx, svc.ID)
			if err != nil {
				p.Log.Warn("prune: failed to list artifacts", "service_id", svc.ID, "error", err)
				continue
			}
			if len(artifacts) <= dep.ImageRetentionCount {
				continue
			}

			if dep.BuildStrategy == string(executor.StrategyStatic) {
				p.pruneStaticOutputs(svc.ID, artifacts[dep.ImageRetentionCount:])
				continue
			}

			keepTags := make([]string, 0, dep.ImageRetentionCount)
			for i := 0; i < dep.ImageRetentionCount && i < len(artifacts); i++ {
				keepTags = append(keepTags, artifacts[i].ImageTag)
			}
			result, err := p.Exec.Prune(ctx, executor.PruneOptions{KeepImageTags: keepTags})
			if err != nil {
				p.Log.Warn("prune failed", "service_id", svc.ID, "error", err)
				continue
			}
			if result.ImagesRemoved > 0 {
				p.Log.Info("pruned images", "service_id", svc.ID, "removed", result.ImagesRemoved, "reclaimed_mb", result.SpaceReclaimedMB)
			}
		}
	}
}

// pruneStaticOutputs removes the on-disk output directories for artifacts
// past the retention window -- the static-strategy equivalent of Docker
// image pruning, since there's no image for p.Exec.Prune to touch. stale is
// already sorted newest-first by ListArtifactsForService and pre-sliced to
// just the entries beyond the keep count by the caller.
func (p *Pruner) pruneStaticOutputs(serviceID int64, stale []models.DeployArtifact) {
	removed := 0
	for _, a := range stale {
		if a.OutputPath == "" {
			continue
		}
		if err := os.RemoveAll(a.OutputPath); err != nil {
			p.Log.Warn("static output prune failed", "service_id", serviceID, "path", a.OutputPath, "error", err)
			continue
		}
		removed++
	}
	if removed > 0 {
		p.Log.Info("pruned static output directories", "service_id", serviceID, "removed", removed)
	}
}
