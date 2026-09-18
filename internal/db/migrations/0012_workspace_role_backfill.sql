-- Backfills workspace_members for every (workspace, user) pair that
-- doesn't already have a row, preserving today's de facto "every member
-- can touch everything, everywhere" access as the box upgrades onto real
-- per-workspace roles (admin/editor/viewer) -- existing owners become
-- admin, existing members become editor. INSERT OR IGNORE respects the
-- UNIQUE(workspace_id, user_id) constraint, so it's a no-op for the rows
-- CreateUser already inserts into workspace 1.
INSERT OR IGNORE INTO workspace_members (workspace_id, user_id, role, created_at)
SELECT w.id, u.id, CASE WHEN u.role = 'owner' THEN 'admin' ELSE 'editor' END, CURRENT_TIMESTAMP
FROM workspaces w CROSS JOIN users u;
