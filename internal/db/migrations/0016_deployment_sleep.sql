-- Opt-in idle sleep: a deployment with sleep_enabled whose last_request_at
-- is older than sleep_idle_minutes gets its containers stopped by
-- internal/scheduler/sleeper.go and its status set to "sleeping" (distinct
-- from a user-initiated "stopped" -- see internal/orchestrator/sleep.go).
-- Its Caddy route is left pointed at Mangrove's own wake handler
-- (internal/api/gate.go, proxy.RouteOptions.Sleepable) so the next visitor
-- wakes it back up instead of getting connection-refused.
--
-- "sleeping" is a new status enum value, which SQLite can't add to an
-- existing CHECK constraint in place -- this rebuilds deployments the same
-- way 0011_custom_domain_port_mode.sql rebuilt port_registry for the same
-- reason (see db.go's applyMigration, which runs this with foreign_keys off
-- around the DROP).
CREATE TABLE deployments_new (
    id INTEGER PRIMARY KEY,
    project_id INTEGER NOT NULL REFERENCES projects(id),
    name TEXT NOT NULL,
    slug TEXT NOT NULL,
    build_strategy TEXT NOT NULL CHECK (build_strategy IN ('dockerfile','nixpacks','compose','image','static')),
    git_branch TEXT,
    project_repo_id INTEGER REFERENCES project_repos(id),
    image_ref TEXT,
    root_path TEXT NOT NULL DEFAULT '.',
    dockerfile_path TEXT,
    compose_path TEXT,
    static_build_command TEXT,
    static_output_dir TEXT,
    auto_deploy_on_push BOOLEAN NOT NULL DEFAULT 0,
    is_public BOOLEAN NOT NULL DEFAULT 0,
    password_protected BOOLEAN NOT NULL DEFAULT 0,
    password_hash TEXT,
    image_retention_count INTEGER NOT NULL DEFAULT 5,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','building','running','stopped','failed','sleeping')),
    node_id INTEGER NOT NULL DEFAULT 1 REFERENCES nodes(id),
    created_by_user_id INTEGER REFERENCES users(id),
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_deployed_at DATETIME,
    replicas INTEGER NOT NULL DEFAULT 1,
    environment TEXT NOT NULL DEFAULT 'production',
    promotes_to_deployment_id INTEGER REFERENCES deployments(id),
    pr_previews_enabled BOOLEAN NOT NULL DEFAULT 0,
    pr_number INTEGER,
    github_pr_comment_id INTEGER,
    public_paths TEXT NOT NULL DEFAULT '',
    sleep_enabled BOOLEAN NOT NULL DEFAULT 0,
    sleep_idle_minutes INTEGER NOT NULL DEFAULT 30,
    last_request_at TIMESTAMP NULL,
    UNIQUE(project_id, slug)
);

INSERT INTO deployments_new (
    id, project_id, name, slug, build_strategy, git_branch, project_repo_id, image_ref, root_path,
    dockerfile_path, compose_path, static_build_command, static_output_dir, auto_deploy_on_push,
    is_public, password_protected, password_hash, image_retention_count, status, node_id,
    created_by_user_id, created_at, updated_at, last_deployed_at, replicas, environment,
    promotes_to_deployment_id, pr_previews_enabled, pr_number, github_pr_comment_id, public_paths
)
SELECT
    id, project_id, name, slug, build_strategy, git_branch, project_repo_id, image_ref, root_path,
    dockerfile_path, compose_path, static_build_command, static_output_dir, auto_deploy_on_push,
    is_public, password_protected, password_hash, image_retention_count, status, node_id,
    created_by_user_id, created_at, updated_at, last_deployed_at, replicas, environment,
    promotes_to_deployment_id, pr_previews_enabled, pr_number, github_pr_comment_id, public_paths
FROM deployments;

DROP TABLE deployments;
ALTER TABLE deployments_new RENAME TO deployments;

-- DROP TABLE drops its indexes too -- recreate the two deployments had.
CREATE INDEX idx_deployments_project ON deployments(project_id);
CREATE UNIQUE INDEX idx_deployments_preview_pr ON deployments(promotes_to_deployment_id, pr_number) WHERE pr_number IS NOT NULL;
