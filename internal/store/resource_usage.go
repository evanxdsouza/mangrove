package store

import (
	"context"
	"time"
)

// ResourceUsageSnapshot mirrors orchestrator.ResourceBudget -- kept as its
// own type here (rather than store importing orchestrator, which already
// imports store) with json tags for the /api/admin/resource-history
// response.
type ResourceUsageSnapshot struct {
	ID                int64     `json:"id"`
	MemoryAllocatedMB int       `json:"memory_allocated_mb"`
	MemoryUsedMB      float64   `json:"memory_used_mb"`
	MemoryCeilingMB   int       `json:"memory_ceiling_mb"`
	RunningContainers int       `json:"running_containers"`
	DiskTotalGB       float64   `json:"disk_total_gb"`
	DiskUsedGB        float64   `json:"disk_used_gb"`
	RecordedAt        time.Time `json:"recorded_at"`
}

func (s *Store) RecordResourceUsageSnapshot(ctx context.Context, snap ResourceUsageSnapshot) error {
	_, err := s.DB.ExecContext(ctx,
		`INSERT INTO resource_usage_snapshots
		 (memory_allocated_mb, memory_used_mb, memory_ceiling_mb, running_containers, disk_total_gb, disk_used_gb)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		snap.MemoryAllocatedMB, snap.MemoryUsedMB, snap.MemoryCeilingMB, snap.RunningContainers, snap.DiskTotalGB, snap.DiskUsedGB,
	)
	return err
}

// ListResourceUsageSnapshots returns every snapshot recorded at or after
// since, oldest first (chart-ready order).
func (s *Store) ListResourceUsageSnapshots(ctx context.Context, since time.Time) ([]ResourceUsageSnapshot, error) {
	rows, err := s.DB.QueryContext(ctx, `
		SELECT id, memory_allocated_mb, memory_used_mb, memory_ceiling_mb, running_containers, disk_total_gb, disk_used_gb, recorded_at
		FROM resource_usage_snapshots
		WHERE recorded_at >= ?
		ORDER BY recorded_at ASC`, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]ResourceUsageSnapshot, 0)
	for rows.Next() {
		var snap ResourceUsageSnapshot
		if err := rows.Scan(&snap.ID, &snap.MemoryAllocatedMB, &snap.MemoryUsedMB, &snap.MemoryCeilingMB,
			&snap.RunningContainers, &snap.DiskTotalGB, &snap.DiskUsedGB, &snap.RecordedAt); err != nil {
			return nil, err
		}
		out = append(out, snap)
	}
	return out, rows.Err()
}

// PruneOldResourceUsageSnapshots bounds the table's growth, same pattern
// as PruneOldHealthChecks.
func (s *Store) PruneOldResourceUsageSnapshots(ctx context.Context, olderThan time.Time) (int64, error) {
	res, err := s.DB.ExecContext(ctx, `DELETE FROM resource_usage_snapshots WHERE recorded_at < ?`, olderThan)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
