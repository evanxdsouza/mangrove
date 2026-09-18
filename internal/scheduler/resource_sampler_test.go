package scheduler

import (
	"context"
	"testing"
	"time"
)

func TestResourceSamplerTickRecordsASnapshot(t *testing.T) {
	orch, st := newReaperTestOrchestrator(t) // reused from preview_reaper_test.go
	ctx := context.Background()

	sampler := NewResourceSampler(orch, orch.Log)
	sampler.tick(ctx)

	got, err := st.ListResourceUsageSnapshots(ctx, time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatalf("ListResourceUsageSnapshots: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 snapshot recorded by tick, got %d", len(got))
	}
}

func TestResourceSamplerPruneOldRespectsRetention(t *testing.T) {
	orch, st := newReaperTestOrchestrator(t)
	ctx := context.Background()

	sampler := NewResourceSampler(orch, orch.Log)
	sampler.tick(ctx) // one fresh snapshot, inside the retention window

	n, err := st.PruneOldResourceUsageSnapshots(ctx, time.Now().Add(-sampler.retain))
	if err != nil {
		t.Fatalf("PruneOldResourceUsageSnapshots: %v", err)
	}
	if n != 0 {
		t.Errorf("expected the fresh snapshot to survive pruning at the retention cutoff, but %d row(s) were removed", n)
	}
}
