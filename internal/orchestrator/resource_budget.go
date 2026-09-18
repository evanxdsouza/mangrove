package orchestrator

import (
	"context"
	"syscall"

	"github.com/evanxdsouza/mangrove/internal/executor"
	"github.com/evanxdsouza/mangrove/internal/store"
)

// ResourceBudget is the live "how much of the box is Mangrove using right
// now" snapshot.
type ResourceBudget struct {
	MemoryAllocatedMB int
	MemoryUsedMB      float64
	MemoryCeilingMB   int
	RunningContainers int
	DiskTotalGB       float64
	DiskUsedGB        float64
}

// ComputeResourceBudget is shared by the on-demand admin dashboard endpoint
// (internal/api/admin.go's getResourceBudget) and scheduler.ResourceSampler's
// periodic history recording, so the two never drift apart on what "usage"
// means. exec == nil skips the live docker-stats memory sum (test
// environments with no executor wired up) but still returns allocation/
// count/disk figures from the DB and syscall.Statfs.
func ComputeResourceBudget(ctx context.Context, st *store.Store, exec executor.Executor, memoryCeilingMB int, dataDir string) (ResourceBudget, error) {
	allocated, err := st.SumConfiguredMemoryMBAll(ctx)
	if err != nil {
		return ResourceBudget{}, err
	}
	running, err := st.CountRunningContainers(ctx)
	if err != nil {
		return ResourceBudget{}, err
	}

	budget := ResourceBudget{
		MemoryAllocatedMB: allocated,
		MemoryCeilingMB:   memoryCeilingMB,
		RunningContainers: running,
	}

	// Actual memory usage is summed from `docker stats` over every running
	// service's live container. Best-effort per container: one that exited
	// between the DB listing and the stats call is skipped rather than
	// failing the whole computation.
	if exec != nil {
		containerIDs, err := st.ListRunningContainerIDs(ctx)
		if err != nil {
			return ResourceBudget{}, err
		}
		for _, id := range containerIDs {
			stats, err := exec.Stats(ctx, id)
			if err != nil {
				continue
			}
			budget.MemoryUsedMB += stats.MemUsageMB
		}
	}

	var stat syscall.Statfs_t
	if err := syscall.Statfs(dataDir, &stat); err == nil {
		blockSize := float64(stat.Bsize)
		totalBytes := float64(stat.Blocks) * blockSize
		freeBytes := float64(stat.Bfree) * blockSize
		const gb = 1024 * 1024 * 1024
		budget.DiskTotalGB = totalBytes / gb
		budget.DiskUsedGB = (totalBytes - freeBytes) / gb
	}

	return budget, nil
}
