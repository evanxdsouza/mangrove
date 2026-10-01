-- Self-healing: an opt-in per-deployment flag (self_heal_enabled) gating
-- two automated recovery behaviors --
-- (1) internal/scheduler/health.go restarts a service whose HTTP health
--     check fails consecutive_health_failures times in a row while the
--     deployment is otherwise "running" (a hung/deadlocked app that never
--     actually crashed, so Docker's own --restart policy never kicks in);
-- (2) internal/scheduler/healer.go automatically retries a deployment
--     that failed outright, with backoff, up to a small cap
--     (auto_retry_count), using internal/orchestrator's BuildRedeployRequest/
--     DispatchDeploy -- the same source-resolution and build-strategy
--     dispatch the manual "Redeploy" button already uses.
ALTER TABLE deployments ADD COLUMN self_heal_enabled BOOLEAN NOT NULL DEFAULT 0;
ALTER TABLE deployments ADD COLUMN auto_retry_count INTEGER NOT NULL DEFAULT 0;
ALTER TABLE services ADD COLUMN consecutive_health_failures INTEGER NOT NULL DEFAULT 0;

-- A deploy triggered by (2) above gets its own triggered_by value
-- ("auto-retry") so the
-- History tab shows self-healing activity distinctly from a human clicking
-- "Redeploy" -- SQLite can't ALTER a CHECK constraint in place, so this
-- rebuilds deploy_history the same way 0004_redeploy_trigger.sql did for
-- "redeploy" itself (see db.go's applyMigration, which runs this with
-- foreign_keys off around the DROP).
CREATE TABLE deploy_history_new (
    id INTEGER PRIMARY KEY,
    deployment_id INTEGER NOT NULL REFERENCES deployments(id),
    triggered_by TEXT NOT NULL CHECK (triggered_by IN ('push','manual','api','rollback','redeploy','promote','auto-retry')),
    triggered_by_user_id INTEGER REFERENCES users(id),
    webhook_event_id INTEGER,
    commit_sha TEXT,
    commit_message TEXT,
    git_ref TEXT,
    status TEXT NOT NULL DEFAULT 'queued' CHECK (status IN ('queued','building','healthchecking','success','failed','rolled_back')),
    started_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    finished_at DATETIME,
    is_current BOOLEAN NOT NULL DEFAULT 0,
    rollback_of_deploy_history_id INTEGER REFERENCES deploy_history_new(id),
    error_message TEXT
);

INSERT INTO deploy_history_new SELECT * FROM deploy_history;

DROP TABLE deploy_history;
ALTER TABLE deploy_history_new RENAME TO deploy_history;
CREATE INDEX idx_deploy_history_deployment ON deploy_history(deployment_id, started_at DESC);
