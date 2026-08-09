-- Adds "redeploy" to triggered_by's enum, for POST
-- /api/deployments/{id}/redeploy (re-runs the build pipeline against a
-- deployment's already-configured source, as opposed to "manual" which is
-- a plain POST .../deploy with caller-supplied git parameters).
--
-- SQLite can't ALTER a CHECK constraint in place, so triggered_by's enum is
-- extended by rebuilding deploy_history (see db.go's applyMigration, which
-- runs this with foreign_keys off around the DROP -- deploy_history is
-- referenced by deploy_history_artifacts.deploy_history_id and
-- webhook_events.deploy_history_id, and self-references itself via
-- rollback_of_deploy_history_id).

CREATE TABLE deploy_history_new (
    id INTEGER PRIMARY KEY,
    deployment_id INTEGER NOT NULL REFERENCES deployments(id),
    triggered_by TEXT NOT NULL CHECK (triggered_by IN ('push','manual','api','rollback','redeploy')),
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
