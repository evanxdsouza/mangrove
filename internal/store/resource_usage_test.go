package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	mangrovedb "github.com/evanxdsouza/mangrove/internal/db"
)

func TestResourceUsageSnapshots(t *testing.T) {
	dir := t.TempDir()
	db, err := mangrovedb.Open(filepath.Join(dir, "mangrove.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	st := New(db)
	ctx := context.Background()

	snap := ResourceUsageSnapshot{
		MemoryAllocatedMB: 512, MemoryUsedMB: 256.5, MemoryCeilingMB: 1536,
		RunningContainers: 3, DiskTotalGB: 100, DiskUsedGB: 42.1,
	}
	if err := st.RecordResourceUsageSnapshot(ctx, snap); err != nil {
		t.Fatalf("RecordResourceUsageSnapshot: %v", err)
	}

	got, err := st.ListResourceUsageSnapshots(ctx, time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatalf("ListResourceUsageSnapshots: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 snapshot, got %d", len(got))
	}
	if got[0].MemoryAllocatedMB != 512 || got[0].DiskUsedGB != 42.1 || got[0].RunningContainers != 3 {
		t.Errorf("snapshot fields didn't round-trip: %+v", got[0])
	}

	// A window that starts after the snapshot excludes it.
	future, err := st.ListResourceUsageSnapshots(ctx, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("ListResourceUsageSnapshots (future window): %v", err)
	}
	if len(future) != 0 {
		t.Errorf("expected 0 snapshots in a future window, got %d", len(future))
	}

	n, err := st.PruneOldResourceUsageSnapshots(ctx, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("PruneOldResourceUsageSnapshots: %v", err)
	}
	if n != 1 {
		t.Errorf("expected 1 row pruned, got %d", n)
	}
	remaining, err := st.ListResourceUsageSnapshots(ctx, time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatalf("ListResourceUsageSnapshots (after prune): %v", err)
	}
	if len(remaining) != 0 {
		t.Errorf("expected 0 snapshots after pruning, got %d", len(remaining))
	}
}
