-- Redo of GitHub auto-deploy (see internal/api/webhook.go rewrite) plus
-- staging environments.
--
-- environment/promotes_to_deployment_id turn a "staging environment" into
-- just another deployment row: a staging deployment tracks its own branch
-- under the same project_repos link, auto-deploys independently, and
-- records which production deployment it promotes into. No new
-- infrastructure needed -- see plan discussion for why the existing
-- one-project_repo-to-many-deployments schema already supports this.
--
-- webhook_registered persists whether CreateWebhook actually succeeded
-- (previously only returned once in the link response and never stored,
-- so the dashboard had no way to show "this repo's webhook isn't
-- registered" later).
ALTER TABLE deployments ADD COLUMN environment TEXT NOT NULL DEFAULT 'production';
ALTER TABLE deployments ADD COLUMN promotes_to_deployment_id INTEGER REFERENCES deployments(id);
ALTER TABLE project_repos ADD COLUMN webhook_registered BOOLEAN NOT NULL DEFAULT 0;

-- Adds "promote" to triggered_by's enum (a staging deployment's current
-- commit being deployed onto its production deployment) -- same
-- rebuild-the-table technique as 0004_redeploy_trigger.sql, since SQLite
-- can't ALTER a CHECK constraint in place.
CREATE TABLE deploy_history_new (
    id INTEGER PRIMARY KEY,
    deployment_id INTEGER NOT NULL REFERENCES deployments(id),
    triggered_by TEXT NOT NULL CHECK (triggered_by IN ('push','manual','api','rollback','redeploy','promote')),
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
