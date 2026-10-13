package scheduler

import (
	"context"
	"log/slog"
	"time"

	"github.com/evanxdsouza/mangrove/internal/orchestrator"
)

// RouteKeeper restores deployment routes Caddy has lost -- see
// orchestrator.RestoreMissingRoutes. Caddy can restart on its own (a
// package upgrade, a crash, someone running `systemctl restart caddy`)
// without Mangrove restarting, so a check only at Mangrove's own startup
// isn't enough. Each tick is one GET against Caddy's admin API unless
// something is actually missing.
type RouteKeeper struct {
	Orch     *orchestrator.Orchestrator
	Log      *slog.Logger
	interval time.Duration
	failing  bool
}

func NewRouteKeeper(orch *orchestrator.Orchestrator, log *slog.Logger) *RouteKeeper {
	return &RouteKeeper{Orch: orch, Log: log, interval: 1 * time.Minute}
}

func (k *RouteKeeper) Run(ctx context.Context) {
	ticker := time.NewTicker(k.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			k.tick(ctx)
		}
	}
}

func (k *RouteKeeper) tick(ctx context.Context) {
	n, err := k.Orch.RestoreMissingRoutes(ctx)
	if err != nil {
		// Logged once per outage, not every minute.
		if !k.failing {
			k.Log.Warn("route keeper: caddy unreachable; will retry", "error", err)
		}
		k.failing = true
		return
	}
	k.failing = false
	if n > 0 {
		k.Log.Info("route keeper: restored routes caddy had lost", "deployments", n)
	}
}
