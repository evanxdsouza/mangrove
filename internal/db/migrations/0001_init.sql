-- Mangrove initial schema.
-- Org/workspace/user/session tables are stubs for future multi-tenancy:
-- v1 seeds exactly one org/workspace/owner and never checks role beyond
-- "is authenticated". See plan §2.

PRAGMA foreign_keys = ON;

CREATE TABLE organizations (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL,
    slug TEXT NOT NULL UNIQUE,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE workspaces (
    id INTEGER PRIMARY KEY,
    org_id INTEGER NOT NULL REFERENCES organizations(id),
    name TEXT NOT NULL,
    slug TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(org_id, slug)
);

CREATE TABLE users (
    id INTEGER PRIMARY KEY,
    org_id INTEGER NOT NULL REFERENCES organizations(id),
    email TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    role TEXT NOT NULL DEFAULT 'owner',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_login_at DATETIME
);

CREATE TABLE workspace_members (
    id INTEGER PRIMARY KEY,
    workspace_id INTEGER NOT NULL REFERENCES workspaces(id),
    user_id INTEGER NOT NULL REFERENCES users(id),
    role TEXT NOT NULL DEFAULT 'owner',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(workspace_id, user_id)
);

CREATE TABLE sessions (
    id INTEGER PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id),
    token_hash TEXT NOT NULL UNIQUE,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at DATETIME NOT NULL,
    ip_address TEXT,
    user_agent TEXT
);
CREATE INDEX idx_sessions_user ON sessions(user_id);
CREATE INDEX idx_sessions_expires ON sessions(expires_at);

-- Hook for future remote workers; seeded with exactly one local row.
CREATE TABLE nodes (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL,
    kind TEXT NOT NULL DEFAULT 'local' CHECK (kind IN ('local','ssh','grpc_agent')),
    address TEXT,
    status TEXT NOT NULL DEFAULT 'online',
    is_default BOOLEAN NOT NULL DEFAULT 1,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE projects (
    id INTEGER PRIMARY KEY,
    workspace_id INTEGER NOT NULL REFERENCES workspaces(id),
    name TEXT NOT NULL,
    slug TEXT NOT NULL,
    description TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(workspace_id, slug)
);

CREATE TABLE github_pats (
    id INTEGER PRIMARY KEY,
    org_id INTEGER NOT NULL REFERENCES organizations(id),
    label TEXT NOT NULL,
    token_encrypted BLOB NOT NULL,
    token_nonce BLOB NOT NULL,
    key_version INTEGER NOT NULL DEFAULT 1,
    created_by_user_id INTEGER REFERENCES users(id),
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_used_at DATETIME
);

CREATE TABLE project_repos (
    id INTEGER PRIMARY KEY,
    project_id INTEGER NOT NULL REFERENCES projects(id),
    github_pat_id INTEGER NOT NULL REFERENCES github_pats(id),
    repo_owner TEXT NOT NULL,
    repo_name TEXT NOT NULL,
    default_branch TEXT NOT NULL DEFAULT 'main',
    webhook_token TEXT NOT NULL UNIQUE,
    webhook_secret_encrypted BLOB NOT NULL,
    webhook_secret_nonce BLOB NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE deployments (
    id INTEGER PRIMARY KEY,
    project_id INTEGER NOT NULL REFERENCES projects(id),
    name TEXT NOT NULL,
    slug TEXT NOT NULL,
    build_strategy TEXT NOT NULL CHECK (build_strategy IN ('dockerfile','nixpacks','compose','image')),
    git_branch TEXT,
    project_repo_id INTEGER REFERENCES project_repos(id),
    image_ref TEXT,
    root_path TEXT NOT NULL DEFAULT '.',
    dockerfile_path TEXT,
    compose_path TEXT,
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
CREATE INDEX idx_deployments_project ON deployments(project_id);

CREATE TABLE port_registry (
    id INTEGER PRIMARY KEY,
    port INTEGER NOT NULL UNIQUE,
    status TEXT NOT NULL DEFAULT 'allocated' CHECK (status IN ('allocated','reserved','free')),
    allocation_type TEXT NOT NULL CHECK (allocation_type IN ('service_public','service_direct_publish','system','manual_external')),
    service_id INTEGER,
    note TEXT,
    reserved_by_user_id INTEGER REFERENCES users(id),
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- One row per running container: 1 for dockerfile/nixpacks, N for compose stacks.
CREATE TABLE services (
    id INTEGER PRIMARY KEY,
    deployment_id INTEGER NOT NULL REFERENCES deployments(id),
    name TEXT NOT NULL,
    is_primary BOOLEAN NOT NULL DEFAULT 1,
    image_tag_current TEXT,
    container_id_current TEXT,
    container_name TEXT NOT NULL,
    internal_port INTEGER,
    host_port INTEGER REFERENCES port_registry(port),
    is_internal_only BOOLEAN NOT NULL DEFAULT 1,
    cpu_limit_cores REAL NOT NULL DEFAULT 0.5,
    memory_limit_mb INTEGER NOT NULL DEFAULT 256,
    restart_policy TEXT NOT NULL DEFAULT 'unless-stopped',
    health_check_path TEXT,
    health_check_interval_s INTEGER NOT NULL DEFAULT 30,
    health_check_timeout_s INTEGER NOT NULL DEFAULT 5,
    status TEXT NOT NULL DEFAULT 'stopped' CHECK (status IN ('stopped','starting','running','unhealthy','failed')),
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(deployment_id, name)
);
CREATE INDEX idx_services_deployment ON services(deployment_id);

CREATE TABLE volumes (
    id INTEGER PRIMARY KEY,
    deployment_id INTEGER NOT NULL REFERENCES deployments(id),
    service_id INTEGER REFERENCES services(id),
    name TEXT NOT NULL,
    docker_volume_name TEXT NOT NULL UNIQUE,
    mount_path TEXT NOT NULL,
    driver TEXT NOT NULL DEFAULT 'local',
    size_estimate_mb INTEGER,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_volumes_deployment ON volumes(deployment_id);

-- One deploy event per deployment (covers all services in a compose stack together).
CREATE TABLE deploy_history (
    id INTEGER PRIMARY KEY,
    deployment_id INTEGER NOT NULL REFERENCES deployments(id),
    triggered_by TEXT NOT NULL CHECK (triggered_by IN ('push','manual','api','rollback')),
    triggered_by_user_id INTEGER REFERENCES users(id),
    webhook_event_id INTEGER,
    commit_sha TEXT,
    commit_message TEXT,
    git_ref TEXT,
    status TEXT NOT NULL DEFAULT 'queued' CHECK (status IN ('queued','building','healthchecking','success','failed','rolled_back')),
    started_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    finished_at DATETIME,
    is_current BOOLEAN NOT NULL DEFAULT 0,
    rollback_of_deploy_history_id INTEGER REFERENCES deploy_history(id),
    error_message TEXT
);
CREATE INDEX idx_deploy_history_deployment ON deploy_history(deployment_id, started_at DESC);

CREATE TABLE deploy_history_artifacts (
    id INTEGER PRIMARY KEY,
    deploy_history_id INTEGER NOT NULL REFERENCES deploy_history(id),
    service_id INTEGER NOT NULL REFERENCES services(id),
    image_tag TEXT NOT NULL,
    image_id TEXT,
    build_log_path TEXT
);
CREATE INDEX idx_artifacts_deploy_history ON deploy_history_artifacts(deploy_history_id);
CREATE INDEX idx_artifacts_service ON deploy_history_artifacts(service_id);

CREATE TABLE env_vars (
    id INTEGER PRIMARY KEY,
    service_id INTEGER NOT NULL REFERENCES services(id),
    key_name TEXT NOT NULL,
    is_secret BOOLEAN NOT NULL DEFAULT 0,
    value_plain TEXT,
    value_encrypted BLOB,
    value_nonce BLOB,
    key_version INTEGER,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(service_id, key_name)
);

CREATE TABLE webhook_events (
    id INTEGER PRIMARY KEY,
    source TEXT NOT NULL DEFAULT 'github',
    delivery_id TEXT NOT NULL UNIQUE,
    event_type TEXT NOT NULL,
    project_repo_id INTEGER REFERENCES project_repos(id),
    signature_valid BOOLEAN NOT NULL,
    payload_json TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'received' CHECK (status IN ('received','verified','rejected','processed','ignored_no_match')),
    deploy_history_id INTEGER REFERENCES deploy_history(id),
    received_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    processed_at DATETIME
);

CREATE TABLE health_checks (
    id INTEGER PRIMARY KEY,
    service_id INTEGER NOT NULL REFERENCES services(id),
    checked_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    status TEXT NOT NULL CHECK (status IN ('healthy','unhealthy','timeout','error')),
    response_time_ms INTEGER,
    http_status INTEGER,
    error_message TEXT
);
CREATE INDEX idx_health_checks_service ON health_checks(service_id, checked_at DESC);

CREATE TABLE notifications_log (
    id INTEGER PRIMARY KEY,
    type TEXT NOT NULL,
    deployment_id INTEGER REFERENCES deployments(id),
    channel TEXT NOT NULL DEFAULT 'email',
    recipient TEXT NOT NULL,
    subject TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('sent','failed')),
    error_message TEXT,
    sent_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Seed: one org, one workspace, one local node. No default user —
-- the admin account is created interactively on first run (Phase 2).
INSERT INTO organizations (id, name, slug) VALUES (1, 'Personal', 'personal');
INSERT INTO workspaces (id, org_id, name, slug) VALUES (1, 1, 'Default', 'default');
INSERT INTO nodes (id, name, kind, is_default) VALUES (1, 'local', 'local', 1);
