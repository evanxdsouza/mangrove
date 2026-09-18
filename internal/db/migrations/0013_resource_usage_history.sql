-- Periodic snapshots of the same figures the admin resource-budget view
-- computes live (see orchestrator.ComputeResourceBudget), recorded every
-- 5 minutes by scheduler.ResourceSampler -- turns the resource page from
-- a right-now reading into a trend. Pruned to a rolling 30-day window by
-- the same sampler, same pattern as health_checks.
CREATE TABLE resource_usage_snapshots (
    id INTEGER PRIMARY KEY,
    memory_allocated_mb INTEGER NOT NULL,
    memory_used_mb REAL NOT NULL,
    memory_ceiling_mb INTEGER NOT NULL,
    running_containers INTEGER NOT NULL,
    disk_total_gb REAL NOT NULL,
    disk_used_gb REAL NOT NULL,
    recorded_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_resource_usage_snapshots_recorded ON resource_usage_snapshots(recorded_at DESC);
