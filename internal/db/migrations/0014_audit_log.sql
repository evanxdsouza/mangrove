-- Who deployed/deleted/changed access, and when. Matters more now that
-- workspace-admins (not just global owners) can do all three -- see
-- docs/multi-user.md. actor_email is denormalized (not just a FK to
-- users) so a record survives the actor's account being deleted later,
-- and so an automated actor with no user row (a GitHub webhook-triggered
-- deploy) still has a readable "who". Deliberately never pruned -- an
-- audit trail that quietly expires isn't one.
CREATE TABLE audit_log (
    id INTEGER PRIMARY KEY,
    actor_user_id INTEGER REFERENCES users(id),
    actor_email TEXT NOT NULL,
    action TEXT NOT NULL,
    resource_type TEXT NOT NULL,
    resource_id INTEGER,
    workspace_id INTEGER,
    detail TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_audit_log_created ON audit_log(created_at DESC);
CREATE INDEX idx_audit_log_workspace ON audit_log(workspace_id, created_at DESC);
