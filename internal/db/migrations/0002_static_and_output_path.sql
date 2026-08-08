-- Adds the "static" build strategy and its per-deployment config, plus a
-- host-path column for static builds' deploy artifacts (dockerfile/nixpacks/
-- image/compose artifacts keep using image_tag/image_id; a static build has
-- no image, just a directory Caddy serves via file_server).
--
-- SQLite can't ALTER a CHECK constraint in place, so build_strategy's enum
-- is extended by rebuilding the deployments table (see db.go's
-- applyMigration, which runs this with foreign_keys off around the DROP).

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
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','building','running','stopped','failed')),
    node_id INTEGER NOT NULL DEFAULT 1 REFERENCES nodes(id),
    created_by_user_id INTEGER REFERENCES users(id),
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_deployed_at DATETIME,
    UNIQUE(project_id, slug)
);

INSERT INTO deployments_new (
    id, project_id, name, slug, build_strategy, git_branch, project_repo_id,
    image_ref, root_path, dockerfile_path, compose_path,
    auto_deploy_on_push, is_public, password_protected, password_hash,
    image_retention_count, status, node_id, created_by_user_id,
    created_at, updated_at, last_deployed_at
)
SELECT
    id, project_id, name, slug, build_strategy, git_branch, project_repo_id,
    image_ref, root_path, dockerfile_path, compose_path,
    auto_deploy_on_push, is_public, password_protected, password_hash,
    image_retention_count, status, node_id, created_by_user_id,
    created_at, updated_at, last_deployed_at
FROM deployments;

DROP TABLE deployments;
ALTER TABLE deployments_new RENAME TO deployments;
CREATE INDEX idx_deployments_project ON deployments(project_id);

ALTER TABLE deploy_history_artifacts ADD COLUMN output_path TEXT;
