package scheduler

import (
	"context"
	"log/slog"
	"time"

	"github.com/evanxdsouza/mangrove/internal/orchestrator"
)

// Sleeper puts idle, sleep-enabled deployments to sleep -- the backend half
// of the opt-in idle-sleep feature (see internal/orchestrator/sleep.go and
// internal/api/gate.go's wake handler, the other half: waking one back up
// on its next visit). Runs far more often than PreviewReaper's hourly sweep
// since a deployment's own sleep_idle_minutes can be as low as a few
// minutes -- a coarse hourly check would leave it running long past its
// configured threshold.
type Sleeper struct {
	Orch     *orchestrator.Orchestrator
	Log      *slog.Logger
	interval time.Duration
}

func NewSleeper(orch *orchestrator.Orchestrator, log *slog.Logger) *Sleeper {
	return &Sleeper{Orch: orch, Log: log, interval: 1 * time.Minute}
}

func (s *Sleeper) Run(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.tick(ctx)
		}
	}
}

func (s *Sleeper) tick(ctx context.Context) {
	candidates, err := s.Orch.Store.ListSleepCandidates(ctx)
	if err != nil {
		s.Log.Warn("sleeper: list candidates failed", "error", err)
		return
	}
	for _, dep := range candidates {
		if err := s.Orch.SleepDeployment(ctx, dep.ID); err != nil {
			s.Log.Warn("sleeper: sleep failed", "deployment_id", dep.ID, "slug", dep.Slug, "error", err)
			continue
		}
		s.Log.Info("sleeper: put an idle deployment to sleep", "deployment_id", dep.ID, "slug", dep.Slug, "idle_minutes", dep.SleepIdleMinutes)
	}
}
