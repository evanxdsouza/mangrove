// Package store is the single place that turns SQLite rows into
// internal/models types and back. API handlers and the orchestrator both
// depend on this rather than writing SQL themselves.
package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/evanxdsouza/mangrove/internal/models"
)

type Store struct {
	DB *sql.DB
}

func New(db *sql.DB) *Store { return &Store{DB: db} }

var ErrNotFound = fmt.Errorf("not found")

// ---- Workspaces ----

// There is exactly one organization (id 1, seeded in 0001_init.sql) -- the
// multi-org layer is intentionally out of scope. Every workspace belongs to
// it, so these methods hardcode org_id = 1 rather than threading an org
// through callers that have no concept of one.

func (s *Store) CreateWorkspace(ctx context.Context, name, slug string) (models.Workspace, error) {
	res, err := s.DB.ExecContext(ctx,
		`INSERT INTO workspaces (org_id, name, slug) VALUES (1, ?, ?)`,
		name, slug,
	)
	if err != nil {
		return models.Workspace{}, err
	}
	id, _ := res.LastInsertId()
	return s.GetWorkspace(ctx, id)
}

func (s *Store) GetWorkspace(ctx context.Context, id int64) (models.Workspace, error) {
	var w models.Workspace
	err := s.DB.QueryRowContext(ctx,
		`SELECT id, org_id, name, slug, created_at FROM workspaces WHERE id = ?`, id,
	).Scan(&w.ID, &w.OrgID, &w.Name, &w.Slug, &w.CreatedAt)
	if err == sql.ErrNoRows {
		return models.Workspace{}, ErrNotFound
	}
	return w, err
}

func (s *Store) ListWorkspaces(ctx context.Context) ([]models.Workspace, error) {
	rows, err := s.DB.QueryContext(ctx,
		`SELECT id, org_id, name, slug, created_at FROM workspaces ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.Workspace
	for rows.Next() {
		var w models.Workspace
		if err := rows.Scan(&w.ID, &w.OrgID, &w.Name, &w.Slug, &w.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

// WorkspaceProjectCount pairs a workspace with the number of projects in it
// -- what the workspaces dashboard shows alongside each workspace.
type WorkspaceProjectCount struct {
	Workspace    models.Workspace
	ProjectCount int
}

func (s *Store) ListWorkspaceProjectCounts(ctx context.Context) ([]WorkspaceProjectCount, error) {
	rows, err := s.DB.QueryContext(ctx, `
		SELECT w.id, w.org_id, w.name, w.slug, w.created_at,
		       (SELECT COUNT(*) FROM projects p WHERE p.workspace_id = w.id)
		FROM workspaces w ORDER BY w.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []WorkspaceProjectCount
	for rows.Next() {
		var w models.Workspace
		var n int
		if err := rows.Scan(&w.ID, &w.OrgID, &w.Name, &w.Slug, &w.CreatedAt, &n); err != nil {
			return nil, err
		}
		out = append(out, WorkspaceProjectCount{Workspace: w, ProjectCount: n})
	}
	return out, rows.Err()
}

// DeleteWorkspace removes a workspace, moving any projects still in it back
// to the default workspace (id 1) so they're never orphaned. Deleting the
// default workspace itself is refused -- it's the guaranteed-to-exist bucket
// every project falls back to.
func (s *Store) DeleteWorkspace(ctx context.Context, id int64) error {
	if id == 1 {
		return fmt.Errorf("cannot delete the default workspace")
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `UPDATE projects SET workspace_id = 1 WHERE workspace_id = ?`, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM workspace_members WHERE workspace_id = ?`, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM workspaces WHERE id = ?`, id); err != nil {
		return err
	}
	return tx.Commit()
}

// SetProjectWorkspace moves a project to a different workspace.
func (s *Store) SetProjectWorkspace(ctx context.Context, projectID, workspaceID int64) error {
	res, err := s.DB.ExecContext(ctx,
		`UPDATE projects SET workspace_id = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		workspaceID, projectID,
	)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// ---- Projects ----

func (s *Store) CreateProject(ctx context.Context, workspaceID int64, name, slug, description string) (models.Project, error) {
	res, err := s.DB.ExecContext(ctx,
		`INSERT INTO projects (workspace_id, name, slug, description) VALUES (?, ?, ?, ?)`,
		workspaceID, name, slug, description,
	)
	if err != nil {
		return models.Project{}, err
	}
	id, _ := res.LastInsertId()
	return s.GetProject(ctx, id)
}

func (s *Store) GetProject(ctx context.Context, id int64) (models.Project, error) {
	var p models.Project
	err := s.DB.QueryRowContext(ctx,
		`SELECT id, workspace_id, name, slug, COALESCE(description,''), created_at, updated_at FROM projects WHERE id = ?`, id,
	).Scan(&p.ID, &p.WorkspaceID, &p.Name, &p.Slug, &p.Description, &p.CreatedAt, &p.UpdatedAt)
	if err == sql.ErrNoRows {
		return models.Project{}, ErrNotFound
	}
	return p, err
}

// ProjectWithWorkspace is a project joined with the name/slug of the
// workspace it belongs to -- what project-listing endpoints return so the
// UI can group/filter without a second round trip per project.
type ProjectWithWorkspace struct {
	models.Project
	WorkspaceName string `json:"workspace_name"`
	WorkspaceSlug string `json:"workspace_slug"`
}

func (s *Store) ListProjects(ctx context.Context) ([]ProjectWithWorkspace, error) {
	rows, err := s.DB.QueryContext(ctx, `
		SELECT p.id, p.workspace_id, p.name, p.slug, COALESCE(p.description,''), p.created_at, p.updated_at,
		       w.name, w.slug
		FROM projects p JOIN workspaces w ON w.id = p.workspace_id
		ORDER BY p.created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ProjectWithWorkspace
	for rows.Next() {
		var pw ProjectWithWorkspace
		if err := rows.Scan(&pw.ID, &pw.WorkspaceID, &pw.Name, &pw.Slug, &pw.Description, &pw.CreatedAt, &pw.UpdatedAt,
			&pw.WorkspaceName, &pw.WorkspaceSlug); err != nil {
			return nil, err
		}
		out = append(out, pw)
	}
	return out, rows.Err()
}

// ListProjectsByWorkspace is ListProjects scoped to one workspace.
func (s *Store) ListProjectsByWorkspace(ctx context.Context, workspaceID int64) ([]ProjectWithWorkspace, error) {
	rows, err := s.DB.QueryContext(ctx, `
		SELECT p.id, p.workspace_id, p.name, p.slug, COALESCE(p.description,''), p.created_at, p.updated_at,
		       w.name, w.slug
		FROM projects p JOIN workspaces w ON w.id = p.workspace_id
		WHERE p.workspace_id = ?
		ORDER BY p.created_at DESC`, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ProjectWithWorkspace
	for rows.Next() {
		var pw ProjectWithWorkspace
		if err := rows.Scan(&pw.ID, &pw.WorkspaceID, &pw.Name, &pw.Slug, &pw.Description, &pw.CreatedAt, &pw.UpdatedAt,
			&pw.WorkspaceName, &pw.WorkspaceSlug); err != nil {
			return nil, err
		}
		out = append(out, pw)
	}
	return out, rows.Err()
}

// DeleteProjectCascade removes a project and everything that hangs off it
// -- deployments, services, deploy history, env vars, etc. Callers are
// responsible for stopping/removing any live containers and Caddy routes
// first (this is a pure DB operation with no knowledge of the executor or
// proxy); see Orchestrator.DeleteProject for the full teardown sequence.
// Foreign keys are enforced (PRAGMA foreign_keys=1 in db.go), so this
// deletes strictly leaves-first: artifacts/health checks/env vars/volumes
// before services, services before deploy_history and port_registry,
// deploy_history/services before deployments, deployments before
// project_repos, project_repos before projects.
func (s *Store) DeleteProjectCascade(ctx context.Context, id int64) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	const deploymentsSub = `(SELECT id FROM deployments WHERE project_id = ?)`
	const servicesSub = `(SELECT id FROM services WHERE deployment_id IN ` + deploymentsSub + `)`
	const deployHistorySub = `(SELECT id FROM deploy_history WHERE deployment_id IN ` + deploymentsSub + `)`

	var ports []int
	rows, err := tx.QueryContext(ctx, `SELECT host_port FROM services WHERE deployment_id IN `+deploymentsSub+` AND host_port IS NOT NULL`, id)
	if err != nil {
		return fmt.Errorf("collect ports: %w", err)
	}
	for rows.Next() {
		var p int
		if err := rows.Scan(&p); err != nil {
			rows.Close()
			return err
		}
		ports = append(ports, p)
	}
	rows.Close()

	stmts := []struct {
		query string
		args  []any
	}{
		{`DELETE FROM deploy_history_artifacts WHERE deploy_history_id IN ` + deployHistorySub, []any{id}},
		{`DELETE FROM health_checks WHERE service_id IN ` + servicesSub, []any{id}},
		{`DELETE FROM env_vars WHERE service_id IN ` + servicesSub, []any{id}},
		{`DELETE FROM volumes WHERE deployment_id IN ` + deploymentsSub, []any{id}},
		{`DELETE FROM webhook_events WHERE project_repo_id IN (SELECT id FROM project_repos WHERE project_id = ?)`, []any{id}},
		{`DELETE FROM notifications_log WHERE deployment_id IN ` + deploymentsSub, []any{id}},
		{`DELETE FROM services WHERE deployment_id IN ` + deploymentsSub, []any{id}},
		{`DELETE FROM deploy_history WHERE deployment_id IN ` + deploymentsSub, []any{id}},
		{`DELETE FROM deployments WHERE project_id = ?`, []any{id}},
		{`DELETE FROM project_repos WHERE project_id = ?`, []any{id}},
		{`DELETE FROM projects WHERE id = ?`, []any{id}},
	}
	for _, st := range stmts {
		if _, err := tx.ExecContext(ctx, st.query, st.args...); err != nil {
			return fmt.Errorf("cascade delete (%s): %w", st.query, err)
		}
	}
	if len(ports) > 0 {
		placeholders := make([]string, len(ports))
		args := make([]any, len(ports))
		for i, p := range ports {
			placeholders[i] = "?"
			args[i] = p
		}
		q := `DELETE FROM port_registry WHERE port IN (` + strings.Join(placeholders, ",") + `)`
		if _, err := tx.ExecContext(ctx, q, args...); err != nil {
			return fmt.Errorf("release ports: %w", err)
		}
	}

	return tx.Commit()
}

// ---- Deployments ----

type CreateDeploymentParams struct {
	ProjectID           int64
	Name                string
	Slug                string
	BuildStrategy       string
	GitBranch           string
	ImageRef            string
	RootPath            string
	DockerfilePath      string
	ComposePath         string
	StaticBuildCommand  string
	StaticOutputDir     string
	ImageRetentionCount int
	// Replicas is how many containers run for this single-service
	// deployment; 0/omitted means 1. Compose deployments must stay at 1.
	Replicas int
}

func (s *Store) CreateDeployment(ctx context.Context, p CreateDeploymentParams) (models.Deployment, error) {
	if p.RootPath == "" {
		p.RootPath = "."
	}
	if p.ImageRetentionCount == 0 {
		p.ImageRetentionCount = 5
	}
	if p.Replicas < 1 {
		p.Replicas = 1
	}
	res, err := s.DB.ExecContext(ctx,
		`INSERT INTO deployments (project_id, name, slug, build_strategy, git_branch, image_ref, root_path, dockerfile_path, compose_path, static_build_command, static_output_dir, image_retention_count, replicas)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.ProjectID, p.Name, p.Slug, p.BuildStrategy, p.GitBranch, p.ImageRef, p.RootPath, p.DockerfilePath, p.ComposePath, p.StaticBuildCommand, p.StaticOutputDir, p.ImageRetentionCount, p.Replicas,
	)
	if err != nil {
		return models.Deployment{}, err
	}
	id, _ := res.LastInsertId()
	return s.GetDeployment(ctx, id)
}

// deploymentColumns is shared by every query that scans a full
// models.Deployment row (GetDeployment, ListDeployments,
// ListAllDeployments), so the column list and scanDeploymentRow stay in
// lockstep with each other by construction.
const deploymentColumns = `id, project_id, name, slug, build_strategy, COALESCE(git_branch,''), project_repo_id,
	       COALESCE(image_ref,''), root_path, COALESCE(dockerfile_path,''), COALESCE(compose_path,''),
	       COALESCE(static_build_command,''), COALESCE(static_output_dir,''),
	       auto_deploy_on_push, is_public, password_protected, image_retention_count, replicas, status, node_id,
	       created_at, updated_at, last_deployed_at`

// rowScanner is satisfied by both *sql.Row and *sql.Rows, letting
// scanDeploymentRow serve single-row lookups and multi-row list queries
// with one scan implementation.
type rowScanner interface {
	Scan(dest ...any) error
}

func scanDeploymentRow(sc rowScanner) (models.Deployment, error) {
	var d models.Deployment
	var projectRepoID sql.NullInt64
	var lastDeployedAtT sql.NullTime
	err := sc.Scan(&d.ID, &d.ProjectID, &d.Name, &d.Slug, &d.BuildStrategy, &d.GitBranch, &projectRepoID,
		&d.ImageRef, &d.RootPath, &d.DockerfilePath, &d.ComposePath, &d.StaticBuildCommand, &d.StaticOutputDir,
		&d.AutoDeployOnPush, &d.IsPublic, &d.PasswordProtected, &d.ImageRetentionCount, &d.Replicas, &d.Status, &d.NodeID,
		&d.CreatedAt, &d.UpdatedAt, &lastDeployedAtT)
	if err != nil {
		return models.Deployment{}, err
	}
	if projectRepoID.Valid {
		d.ProjectRepoID = &projectRepoID.Int64
	}
	if lastDeployedAtT.Valid {
		t := lastDeployedAtT.Time
		d.LastDeployedAt = &t
	}
	return d, nil
}

func (s *Store) GetDeployment(ctx context.Context, id int64) (models.Deployment, error) {
	row := s.DB.QueryRowContext(ctx, `SELECT `+deploymentColumns+` FROM deployments WHERE id = ?`, id)
	d, err := scanDeploymentRow(row)
	if err == sql.ErrNoRows {
		return models.Deployment{}, ErrNotFound
	}
	if err != nil {
		return models.Deployment{}, err
	}
	return d, nil
}

func (s *Store) ListDeployments(ctx context.Context, projectID int64) ([]models.Deployment, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT `+deploymentColumns+` FROM deployments WHERE project_id = ? ORDER BY created_at DESC`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]models.Deployment, 0)
	for rows.Next() {
		d, err := scanDeploymentRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// DeleteDeployment removes a single deployment and everything that hangs
// off it -- services, deploy history, env vars, volumes, etc -- following
// the same leaves-first ordering as DeleteProjectCascade, just scoped to
// one deployment_id instead of a project's whole set. Callers are
// responsible for stopping/removing any live containers and Caddy routes
// first; see Orchestrator.DeleteDeployment for the full teardown sequence.
func (s *Store) DeleteDeployment(ctx context.Context, id int64) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	const servicesSub = `(SELECT id FROM services WHERE deployment_id = ?)`
	const deployHistorySub = `(SELECT id FROM deploy_history WHERE deployment_id = ?)`

	var ports []int
	rows, err := tx.QueryContext(ctx, `SELECT host_port FROM services WHERE deployment_id = ? AND host_port IS NOT NULL`, id)
	if err != nil {
		return fmt.Errorf("collect ports: %w", err)
	}
	for rows.Next() {
		var p int
		if err := rows.Scan(&p); err != nil {
			rows.Close()
			return err
		}
		ports = append(ports, p)
	}
	rows.Close()

	stmts := []struct {
		query string
		args  []any
	}{
		{`DELETE FROM deploy_history_artifacts WHERE deploy_history_id IN ` + deployHistorySub, []any{id}},
		{`DELETE FROM health_checks WHERE service_id IN ` + servicesSub, []any{id}},
		{`DELETE FROM env_vars WHERE service_id IN ` + servicesSub, []any{id}},
		{`DELETE FROM volumes WHERE deployment_id = ?`, []any{id}},
		{`DELETE FROM notifications_log WHERE deployment_id = ?`, []any{id}},
		{`DELETE FROM services WHERE deployment_id = ?`, []any{id}},
		{`DELETE FROM deploy_history WHERE deployment_id = ?`, []any{id}},
		{`DELETE FROM deployments WHERE id = ?`, []any{id}},
	}
	for _, st := range stmts {
		if _, err := tx.ExecContext(ctx, st.query, st.args...); err != nil {
			return fmt.Errorf("cascade delete (%s): %w", st.query, err)
		}
	}
	if len(ports) > 0 {
		placeholders := make([]string, len(ports))
		args := make([]any, len(ports))
		for i, p := range ports {
			placeholders[i] = "?"
			args[i] = p
		}
		q := `DELETE FROM port_registry WHERE port IN (` + strings.Join(placeholders, ",") + `)`
		if _, err := tx.ExecContext(ctx, q, args...); err != nil {
			return fmt.Errorf("release ports: %w", err)
		}
	}

	return tx.Commit()
}

// ListAllDeployments returns every deployment across every project --
// used by the pruning scheduler, which sweeps the whole host rather than
// one project at a time.
func (s *Store) ListAllDeployments(ctx context.Context) ([]models.Deployment, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT `+deploymentColumns+` FROM deployments ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]models.Deployment, 0)
	for rows.Next() {
		d, err := scanDeploymentRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// UpdateBuildConfigParams mirrors CreateDeploymentParams' build-related
// fields -- everything about *how* a deployment builds and starts, as
// opposed to its name/slug/access-control/replicas, which have their own
// dedicated update paths.
type UpdateBuildConfigParams struct {
	BuildStrategy      string
	GitBranch          string
	ImageRef           string
	RootPath           string
	DockerfilePath     string
	ComposePath        string
	StaticBuildCommand string
	StaticOutputDir    string
}

// UpdateDeploymentBuildConfig changes what a deployment builds/starts from
// without touching its identity (name/slug), access control, or replica
// count. This is the only way to fix a deployment whose strategy was
// mis-detected at creation time (e.g. a Vite/CRA frontend that "Deploy from
// GitHub" originally guessed as nixpacks, which then fails every build with
// "no start command could be found") short of deleting and recreating it --
// the change only takes effect on the next deploy/redeploy; it never
// touches whatever is currently running.
func (s *Store) UpdateDeploymentBuildConfig(ctx context.Context, id int64, p UpdateBuildConfigParams) error {
	if p.RootPath == "" {
		p.RootPath = "."
	}
	_, err := s.DB.ExecContext(ctx,
		`UPDATE deployments SET build_strategy = ?, git_branch = ?, image_ref = ?, root_path = ?, dockerfile_path = ?, compose_path = ?, static_build_command = ?, static_output_dir = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		p.BuildStrategy, p.GitBranch, p.ImageRef, p.RootPath, p.DockerfilePath, p.ComposePath, p.StaticBuildCommand, p.StaticOutputDir, id,
	)
	return err
}

func (s *Store) UpdateDeploymentStatus(ctx context.Context, id int64, status string) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE deployments SET status = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, status, id)
	return err
}

func (s *Store) TouchDeploymentDeployed(ctx context.Context, id int64) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE deployments SET last_deployed_at = CURRENT_TIMESTAMP WHERE id = ?`, id)
	return err
}

func (s *Store) SetDeploymentAccessControl(ctx context.Context, id int64, isPublic, passwordProtected bool, passwordHash string) error {
	_, err := s.DB.ExecContext(ctx,
		`UPDATE deployments SET is_public = ?, password_protected = ?, password_hash = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		isPublic, passwordProtected, nullIfEmpty(passwordHash), id,
	)
	return err
}

// GetDeploymentPasswordHash is kept separate from GetDeployment
// deliberately -- the bcrypt hash is only ever needed by the swap/proxy
// path, never by API responses.
func (s *Store) GetDeploymentPasswordHash(ctx context.Context, id int64) (string, error) {
	var hash sql.NullString
	err := s.DB.QueryRowContext(ctx, `SELECT password_hash FROM deployments WHERE id = ?`, id).Scan(&hash)
	if err == sql.ErrNoRows {
		return "", ErrNotFound
	}
	return hash.String, err
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// ---- Services ----

type CreateServiceParams struct {
	DeploymentID         int64
	Name                 string
	ContainerName        string
	InternalPort         int
	IsInternalOnly       bool
	CPULimitCores        float64
	MemoryLimitMB        int
	RestartPolicy        string
	HealthCheckPath      string
	HealthCheckIntervalS int
	HealthCheckTimeoutS  int
	// Command overrides the image's default CMD when non-empty -- see
	// executor.RunSpec.Command. Stored as a JSON array.
	Command []string
	// NoContainer is set by the static build strategy, whose "service" row
	// exists only to hold port/routing config -- no container ever runs
	// for it. It skips the CPU/memory zero-value defaulting below, which
	// otherwise exists to catch an omitted value on a real container; for
	// static, an omitted value is correct, and defaulting memory to 256MB
	// would wrongly eat into admission control's budget for a deployment
	// that consumes none.
	NoContainer bool
}

func (s *Store) CreateService(ctx context.Context, p CreateServiceParams) (models.Service, error) {
	if p.CPULimitCores == 0 && !p.NoContainer {
		p.CPULimitCores = 0.5
	}
	if p.MemoryLimitMB == 0 && !p.NoContainer {
		p.MemoryLimitMB = 256
	}
	if p.RestartPolicy == "" {
		p.RestartPolicy = "unless-stopped"
	}
	if p.HealthCheckIntervalS == 0 {
		p.HealthCheckIntervalS = 30
	}
	if p.HealthCheckTimeoutS == 0 {
		p.HealthCheckTimeoutS = 5
	}
	var commandJSON any
	if len(p.Command) > 0 {
		b, err := json.Marshal(p.Command)
		if err != nil {
			return models.Service{}, fmt.Errorf("encode command: %w", err)
		}
		commandJSON = string(b)
	}
	res, err := s.DB.ExecContext(ctx, `
		INSERT INTO services (deployment_id, name, container_name, internal_port, is_internal_only,
		                       cpu_limit_cores, memory_limit_mb, restart_policy, health_check_path,
		                       health_check_interval_s, health_check_timeout_s, command)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.DeploymentID, p.Name, p.ContainerName, p.InternalPort, p.IsInternalOnly,
		p.CPULimitCores, p.MemoryLimitMB, p.RestartPolicy, nullIfEmpty(p.HealthCheckPath),
		p.HealthCheckIntervalS, p.HealthCheckTimeoutS, commandJSON,
	)
	if err != nil {
		return models.Service{}, err
	}
	id, _ := res.LastInsertId()
	return s.GetService(ctx, id)
}

func (s *Store) GetService(ctx context.Context, id int64) (models.Service, error) {
	var svc models.Service
	var hostPort sql.NullInt64
	var commandJSON sql.NullString
	var replicaIDsJSON sql.NullString
	err := s.DB.QueryRowContext(ctx, `
		SELECT id, deployment_id, name, is_primary, COALESCE(image_tag_current,''), COALESCE(container_id_current,''),
		       container_name, internal_port, host_port, is_internal_only, cpu_limit_cores, memory_limit_mb,
		       restart_policy, COALESCE(health_check_path,''), health_check_interval_s, health_check_timeout_s,
		       command, COALESCE(replica_container_ids,''), status, created_at, updated_at
		FROM services WHERE id = ?`, id,
	).Scan(&svc.ID, &svc.DeploymentID, &svc.Name, &svc.IsPrimary, &svc.ImageTagCurrent, &svc.ContainerIDCurrent,
		&svc.ContainerName, &svc.InternalPort, &hostPort, &svc.IsInternalOnly, &svc.CPULimitCores, &svc.MemoryLimitMB,
		&svc.RestartPolicy, &svc.HealthCheckPath, &svc.HealthCheckIntervalS, &svc.HealthCheckTimeoutS,
		&commandJSON, &replicaIDsJSON, &svc.Status, &svc.CreatedAt, &svc.UpdatedAt)
	if err == sql.ErrNoRows {
		return models.Service{}, ErrNotFound
	}
	if err != nil {
		return models.Service{}, err
	}
	if hostPort.Valid {
		p := int(hostPort.Int64)
		svc.HostPort = &p
	}
	if commandJSON.Valid && commandJSON.String != "" {
		if err := json.Unmarshal([]byte(commandJSON.String), &svc.Command); err != nil {
			return models.Service{}, fmt.Errorf("decode command: %w", err)
		}
	}
	if replicaIDsJSON.Valid && replicaIDsJSON.String != "" {
		if err := json.Unmarshal([]byte(replicaIDsJSON.String), &svc.ReplicaContainerIDs); err != nil {
			return models.Service{}, fmt.Errorf("decode replica container ids: %w", err)
		}
	}
	return svc, nil
}

func (s *Store) ListServices(ctx context.Context, deploymentID int64) ([]models.Service, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id FROM services WHERE deployment_id = ? ORDER BY id`, deploymentID)
	if err != nil {
		return nil, err
	}
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	rows.Close()

	out := make([]models.Service, 0, len(ids))
	for _, id := range ids {
		svc, err := s.GetService(ctx, id)
		if err != nil {
			return nil, err
		}
		out = append(out, svc)
	}
	return out, nil
}

func (s *Store) UpdateServiceRuntime(ctx context.Context, id int64, imageTag, containerID, status string) error {
	_, err := s.DB.ExecContext(ctx,
		`UPDATE services SET image_tag_current = ?, container_id_current = ?, status = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		imageTag, containerID, status, id,
	)
	return err
}

// UpdateServiceReplicas records the full set of running replica container
// IDs for a service (primary first). It also refreshes container_id_current
// to the primary, keeping the two in lockstep for the single-container
// operations (logs/stats/exec/health) that read only the primary. A nil or
// single-element set clears replica_container_ids to NULL.
func (s *Store) UpdateServiceReplicas(ctx context.Context, id int64, containerIDs []string) error {
	var primary string
	if len(containerIDs) > 0 {
		primary = containerIDs[0]
	}
	var replicasJSON any
	if len(containerIDs) > 1 {
		b, err := json.Marshal(containerIDs)
		if err != nil {
			return fmt.Errorf("encode replica container ids: %w", err)
		}
		replicasJSON = string(b)
	}
	_, err := s.DB.ExecContext(ctx,
		`UPDATE services SET container_id_current = ?, replica_container_ids = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		nullIfEmpty(primary), replicasJSON, id,
	)
	return err
}

// UpdateDeploymentReplicas sets a deployment's replica count, used when
// scaling an already-created deployment up or down. The caller is
// responsible for actually creating/removing the containers and re-pointing
// the load balancer.
func (s *Store) UpdateDeploymentReplicas(ctx context.Context, id int64, replicas int) error {
	if replicas < 1 {
		replicas = 1
	}
	_, err := s.DB.ExecContext(ctx,
		`UPDATE deployments SET replicas = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, replicas, id)
	return err
}

func (s *Store) UpdateServiceInternalOnly(ctx context.Context, id int64, internalOnly bool) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE services SET is_internal_only = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, internalOnly, id)
	return err
}

func (s *Store) UpdateServiceStatus(ctx context.Context, id int64, status string) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE services SET status = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, status, id)
	return err
}

// ---- Volumes ----
//
// A volume row's docker_volume_name is created once (at deployment-creation
// time, e.g. by template installation) and reused by every subsequent
// deploy of that service -- unlike container names, which embed the
// deploy_history id and change on every blue/green swap. Docker creates the
// named volume automatically the first time a container mounts it, so
// there's no separate "docker volume create" step; see
// orchestrator.Deploy's RunSpec.Volumes wiring.

type CreateVolumeParams struct {
	DeploymentID     int64
	ServiceID        *int64
	Name             string
	DockerVolumeName string
	MountPath        string
}

func (s *Store) CreateVolume(ctx context.Context, p CreateVolumeParams) (models.Volume, error) {
	res, err := s.DB.ExecContext(ctx,
		`INSERT INTO volumes (deployment_id, service_id, name, docker_volume_name, mount_path) VALUES (?, ?, ?, ?, ?)`,
		p.DeploymentID, p.ServiceID, p.Name, p.DockerVolumeName, p.MountPath,
	)
	if err != nil {
		return models.Volume{}, err
	}
	id, _ := res.LastInsertId()
	v := models.Volume{ID: id, DeploymentID: p.DeploymentID, ServiceID: p.ServiceID, Name: p.Name, DockerVolumeName: p.DockerVolumeName, MountPath: p.MountPath, Driver: "local"}
	return v, nil
}

// ListVolumesForService returns every volume mounted into a service's
// container -- what orchestrator.Deploy turns into RunSpec.Volumes.
func (s *Store) ListVolumesForService(ctx context.Context, serviceID int64) ([]models.Volume, error) {
	rows, err := s.DB.QueryContext(ctx, `
		SELECT id, deployment_id, service_id, name, docker_volume_name, mount_path, driver, size_estimate_mb
		FROM volumes WHERE service_id = ? ORDER BY id`, serviceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.Volume
	for rows.Next() {
		var v models.Volume
		var serviceID sql.NullInt64
		var sizeEstimateMB sql.NullInt64
		if err := rows.Scan(&v.ID, &v.DeploymentID, &serviceID, &v.Name, &v.DockerVolumeName, &v.MountPath, &v.Driver, &sizeEstimateMB); err != nil {
			return nil, err
		}
		if serviceID.Valid {
			v.ServiceID = &serviceID.Int64
		}
		if sizeEstimateMB.Valid {
			mb := int(sizeEstimateMB.Int64)
			v.SizeEstimateMB = &mb
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// SumConfiguredMemoryMB returns the total memory_limit_mb across every
// service belonging to a non-stopped deployment -- the figure admission
// control checks against the deployment memory ceiling, and what the
// resource-budget dashboard view shows as "allocated".
func (s *Store) SumConfiguredMemoryMB(ctx context.Context, excludeDeploymentID int64) (int, error) {
	var total int
	err := s.DB.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(s.memory_limit_mb * d.replicas), 0)
		FROM services s
		JOIN deployments d ON d.id = s.deployment_id
		WHERE d.status != 'stopped' AND d.id != ?`, excludeDeploymentID,
	).Scan(&total)
	return total, err
}

// ---- Deploy history ----

func (s *Store) CreateDeployHistory(ctx context.Context, deploymentID int64, triggeredBy string, commitSHA, commitMessage, gitRef string) (int64, error) {
	res, err := s.DB.ExecContext(ctx,
		`INSERT INTO deploy_history (deployment_id, triggered_by, commit_sha, commit_message, git_ref, status) VALUES (?, ?, ?, ?, ?, 'queued')`,
		deploymentID, triggeredBy, nullIfEmpty(commitSHA), nullIfEmpty(commitMessage), nullIfEmpty(gitRef),
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) UpdateDeployHistoryStatus(ctx context.Context, id int64, status string, errMsg string) error {
	if status == "success" || status == "failed" || status == "rolled_back" {
		_, err := s.DB.ExecContext(ctx,
			`UPDATE deploy_history SET status = ?, error_message = ?, finished_at = CURRENT_TIMESTAMP WHERE id = ?`,
			status, nullIfEmpty(errMsg), id,
		)
		return err
	}
	_, err := s.DB.ExecContext(ctx, `UPDATE deploy_history SET status = ?, error_message = ? WHERE id = ?`, status, nullIfEmpty(errMsg), id)
	return err
}

// SweepStuckDeploys marks any deploy_history row left in an in-progress
// state (queued/building/healthchecking) as failed. These can only be
// leftovers from a Mangrove process that crashed or was killed mid-deploy
// -- called once at startup so the UI never shows a deploy as eternally
// "in progress" when nothing is actually working on it. This is the
// honest scope of "durable" here: the record survives and is correctly
// marked, not that the interrupted build resumes -- a fresh push or
// manual deploy simply retries.
func (s *Store) SweepStuckDeploys(ctx context.Context) (int64, error) {
	res, err := s.DB.ExecContext(ctx,
		`UPDATE deploy_history SET status = 'failed', error_message = 'interrupted by a Mangrove restart', finished_at = CURRENT_TIMESTAMP
		 WHERE status IN ('queued', 'building', 'healthchecking')`,
	)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (s *Store) MarkDeployHistoryCurrent(ctx context.Context, deploymentID, deployHistoryID int64) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `UPDATE deploy_history SET is_current = 0 WHERE deployment_id = ?`, deploymentID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE deploy_history SET is_current = 1 WHERE id = ?`, deployHistoryID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) ListDeployHistory(ctx context.Context, deploymentID int64) ([]models.DeployHistory, error) {
	rows, err := s.DB.QueryContext(ctx, `
		SELECT id, deployment_id, triggered_by, COALESCE(commit_sha,''), COALESCE(commit_message,''), COALESCE(git_ref,''),
		       status, started_at, finished_at, is_current, rollback_of_deploy_history_id, COALESCE(error_message,'')
		FROM deploy_history WHERE deployment_id = ? ORDER BY started_at DESC`, deploymentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.DeployHistory
	for rows.Next() {
		var h models.DeployHistory
		var finishedAt sql.NullTime
		var rollbackOf sql.NullInt64
		if err := rows.Scan(&h.ID, &h.DeploymentID, &h.TriggeredBy, &h.CommitSHA, &h.CommitMessage, &h.GitRef,
			&h.Status, &h.StartedAt, &finishedAt, &h.IsCurrent, &rollbackOf, &h.ErrorMessage); err != nil {
			return nil, err
		}
		if finishedAt.Valid {
			t := finishedAt.Time
			h.FinishedAt = &t
		}
		if rollbackOf.Valid {
			h.RollbackOfDeployHistoryID = &rollbackOf.Int64
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

func (s *Store) GetDeployHistory(ctx context.Context, id int64) (models.DeployHistory, error) {
	var h models.DeployHistory
	var finishedAt sql.NullTime
	var rollbackOf sql.NullInt64
	err := s.DB.QueryRowContext(ctx, `
		SELECT id, deployment_id, triggered_by, COALESCE(commit_sha,''), COALESCE(commit_message,''), COALESCE(git_ref,''),
		       status, started_at, finished_at, is_current, rollback_of_deploy_history_id, COALESCE(error_message,'')
		FROM deploy_history WHERE id = ?`, id,
	).Scan(&h.ID, &h.DeploymentID, &h.TriggeredBy, &h.CommitSHA, &h.CommitMessage, &h.GitRef,
		&h.Status, &h.StartedAt, &finishedAt, &h.IsCurrent, &rollbackOf, &h.ErrorMessage)
	if err == sql.ErrNoRows {
		return models.DeployHistory{}, ErrNotFound
	}
	if err != nil {
		return models.DeployHistory{}, err
	}
	if finishedAt.Valid {
		t := finishedAt.Time
		h.FinishedAt = &t
	}
	if rollbackOf.Valid {
		h.RollbackOfDeployHistoryID = &rollbackOf.Int64
	}
	return h, nil
}

func (s *Store) SetRollbackOf(ctx context.Context, deployHistoryID, rollbackOfID int64) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE deploy_history SET rollback_of_deploy_history_id = ? WHERE id = ?`, rollbackOfID, deployHistoryID)
	return err
}

// ---- Deploy artifacts ----

// CreateDeployArtifact records what a deploy produced. outputPath is set
// instead of imageTag/imageID for a static-strategy deploy (see
// models.DeployArtifact.OutputPath); pass "" for the strategies that build
// an image.
func (s *Store) CreateDeployArtifact(ctx context.Context, deployHistoryID, serviceID int64, imageTag, imageID, buildLogPath, outputPath string) error {
	_, err := s.DB.ExecContext(ctx,
		`INSERT INTO deploy_history_artifacts (deploy_history_id, service_id, image_tag, image_id, build_log_path, output_path) VALUES (?, ?, ?, ?, ?, ?)`,
		deployHistoryID, serviceID, imageTag, nullIfEmpty(imageID), nullIfEmpty(buildLogPath), nullIfEmpty(outputPath),
	)
	return err
}

// ListArtifactsForService returns every build produced for a service,
// newest first -- the rollback list, and what image/static-output
// retention pruning walks past the keep count.
func (s *Store) ListArtifactsForService(ctx context.Context, serviceID int64) ([]models.DeployArtifact, error) {
	rows, err := s.DB.QueryContext(ctx, `
		SELECT a.id, a.deploy_history_id, a.service_id, a.image_tag, COALESCE(a.image_id,''), COALESCE(a.build_log_path,''), COALESCE(a.output_path,'')
		FROM deploy_history_artifacts a
		JOIN deploy_history h ON h.id = a.deploy_history_id
		WHERE a.service_id = ?
		ORDER BY h.started_at DESC`, serviceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.DeployArtifact
	for rows.Next() {
		var a models.DeployArtifact
		if err := rows.Scan(&a.ID, &a.DeployHistoryID, &a.ServiceID, &a.ImageTag, &a.ImageID, &a.BuildLogPath, &a.OutputPath); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *Store) GetArtifactForServiceAtDeployHistory(ctx context.Context, deployHistoryID, serviceID int64) (models.DeployArtifact, error) {
	var a models.DeployArtifact
	err := s.DB.QueryRowContext(ctx, `
		SELECT id, deploy_history_id, service_id, image_tag, COALESCE(image_id,''), COALESCE(build_log_path,''), COALESCE(output_path,'')
		FROM deploy_history_artifacts WHERE deploy_history_id = ? AND service_id = ?`, deployHistoryID, serviceID,
	).Scan(&a.ID, &a.DeployHistoryID, &a.ServiceID, &a.ImageTag, &a.ImageID, &a.BuildLogPath, &a.OutputPath)
	if err == sql.ErrNoRows {
		return models.DeployArtifact{}, ErrNotFound
	}
	return a, err
}

// ---- Health checks ----

func (s *Store) RecordHealthCheck(ctx context.Context, serviceID int64, status string, responseTimeMS int64, httpStatus int, errMsg string) error {
	_, err := s.DB.ExecContext(ctx,
		`INSERT INTO health_checks (service_id, status, response_time_ms, http_status, error_message) VALUES (?, ?, ?, ?, ?)`,
		serviceID, status, responseTimeMS, nullIfZero(httpStatus), nullIfEmpty(errMsg),
	)
	return err
}

func nullIfZero(n int) any {
	if n == 0 {
		return nil
	}
	return n
}

type HealthCheckEntry struct {
	Status         string    `json:"status"`
	CheckedAt      time.Time `json:"checked_at"`
	ResponseTimeMS int64     `json:"response_time_ms"`
	HTTPStatus     int       `json:"http_status,omitempty"`
	ErrorMessage   string    `json:"error_message,omitempty"`
}

// ListRecentHealthChecks backs the dashboard's uptime view -- a rolling
// window of recent pings for one service, newest first.
func (s *Store) ListRecentHealthChecks(ctx context.Context, serviceID int64, limit int) ([]HealthCheckEntry, error) {
	rows, err := s.DB.QueryContext(ctx, `
		SELECT status, checked_at, response_time_ms, COALESCE(http_status, 0), COALESCE(error_message, '')
		FROM health_checks WHERE service_id = ? ORDER BY checked_at DESC LIMIT ?`, serviceID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []HealthCheckEntry
	for rows.Next() {
		var e HealthCheckEntry
		if err := rows.Scan(&e.Status, &e.CheckedAt, &e.ResponseTimeMS, &e.HTTPStatus, &e.ErrorMessage); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *Store) LatestHealthCheck(ctx context.Context, serviceID int64) (status string, checkedAt time.Time, err error) {
	err = s.DB.QueryRowContext(ctx,
		`SELECT status, checked_at FROM health_checks WHERE service_id = ? ORDER BY checked_at DESC LIMIT 1`, serviceID,
	).Scan(&status, &checkedAt)
	if err == sql.ErrNoRows {
		return "unknown", time.Time{}, nil
	}
	return status, checkedAt, err
}

// RunnableService is the minimal projection the health-check scheduler
// needs -- avoids pulling the full models.Service (and its extra queries)
// just to poll a container.
type RunnableService struct {
	ID                   int64
	ContainerID          string // the live, versioned container -- container_name is only a naming prefix, not a running container
	InternalPort         int
	HealthCheckPath      string
	HealthCheckIntervalS int
	HealthCheckTimeoutS  int
}

// ListRunningServicesWithHealthCheck returns every service that is running
// and has an HTTP health check configured -- what the scheduler polls.
func (s *Store) ListRunningServicesWithHealthCheck(ctx context.Context) ([]RunnableService, error) {
	rows, err := s.DB.QueryContext(ctx, `
		SELECT id, container_id_current, internal_port, health_check_path, health_check_interval_s, health_check_timeout_s
		FROM services
		WHERE status = 'running' AND container_id_current IS NOT NULL AND container_id_current != ''
		  AND health_check_path IS NOT NULL AND health_check_path != ''`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []RunnableService
	for rows.Next() {
		var r RunnableService
		if err := rows.Scan(&r.ID, &r.ContainerID, &r.InternalPort, &r.HealthCheckPath, &r.HealthCheckIntervalS, &r.HealthCheckTimeoutS); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// PruneOldHealthChecks deletes health_checks rows older than `before`,
// keeping the table bounded (per plan §2).
func (s *Store) PruneOldHealthChecks(ctx context.Context, before time.Time) (int64, error) {
	res, err := s.DB.ExecContext(ctx, `DELETE FROM health_checks WHERE checked_at < ?`, before)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// ---- Env vars ----

// EnvVarRow is the raw row shape, kept separate from models.EnvVar so
// callers must explicitly decrypt secret values (via internal/secrets)
// rather than accidentally handling ciphertext as if it were plaintext.
type EnvVarRow struct {
	ID             int64
	ServiceID      int64
	KeyName        string
	IsSecret       bool
	ValuePlain     sql.NullString
	ValueEncrypted []byte
	ValueNonce     []byte
}

func (s *Store) CreatePlainEnvVar(ctx context.Context, serviceID int64, key, value string) error {
	_, err := s.DB.ExecContext(ctx,
		`INSERT INTO env_vars (service_id, key_name, is_secret, value_plain) VALUES (?, ?, 0, ?)
		 ON CONFLICT(service_id, key_name) DO UPDATE SET value_plain = excluded.value_plain, is_secret = 0, updated_at = CURRENT_TIMESTAMP`,
		serviceID, key, value,
	)
	return err
}

func (s *Store) CreateSecretEnvVar(ctx context.Context, serviceID int64, key string, ciphertext, nonce []byte) error {
	_, err := s.DB.ExecContext(ctx,
		`INSERT INTO env_vars (service_id, key_name, is_secret, value_encrypted, value_nonce, key_version) VALUES (?, ?, 1, ?, ?, 1)
		 ON CONFLICT(service_id, key_name) DO UPDATE SET value_encrypted = excluded.value_encrypted, value_nonce = excluded.value_nonce, is_secret = 1, updated_at = CURRENT_TIMESTAMP`,
		serviceID, key, ciphertext, nonce,
	)
	return err
}

func (s *Store) ListEnvVarRows(ctx context.Context, serviceID int64) ([]EnvVarRow, error) {
	rows, err := s.DB.QueryContext(ctx,
		`SELECT id, service_id, key_name, is_secret, value_plain, value_encrypted, value_nonce FROM env_vars WHERE service_id = ? ORDER BY key_name`,
		serviceID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []EnvVarRow
	for rows.Next() {
		var r EnvVarRow
		if err := rows.Scan(&r.ID, &r.ServiceID, &r.KeyName, &r.IsSecret, &r.ValuePlain, &r.ValueEncrypted, &r.ValueNonce); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) DeleteEnvVar(ctx context.Context, serviceID int64, key string) error {
	_, err := s.DB.ExecContext(ctx, `DELETE FROM env_vars WHERE service_id = ? AND key_name = ?`, serviceID, key)
	return err
}

// ---- Users & sessions ----

func (s *Store) CountUsers(ctx context.Context) (int, error) {
	var n int
	err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&n)
	return n, err
}

// CreateUser creates a user with the given role ("owner" or "member").
// Callers must pass an explicit role -- there's no safe default, since
// getting this wrong either locks a real admin out of admin actions or
// grants a member owner-level power by accident.
func (s *Store) CreateUser(ctx context.Context, email, passwordHash, role string) (int64, error) {
	res, err := s.DB.ExecContext(ctx, `INSERT INTO users (org_id, email, password_hash, role) VALUES (1, ?, ?, ?)`, email, passwordHash, role)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	// Every user is, for now, a member of the single default workspace --
	// the multi-tenancy stub this exists for isn't wired up until org/team
	// support is built.
	if _, err := s.DB.ExecContext(ctx, `INSERT INTO workspace_members (workspace_id, user_id, role) VALUES (1, ?, ?)`, id, role); err != nil {
		return 0, err
	}
	return id, nil
}

type UserAuth struct {
	ID           int64
	Email        string
	PasswordHash string
	Role         string
}

type UserWithRole struct {
	ID          int64      `json:"id"`
	Email       string     `json:"email"`
	Role        string     `json:"role"`
	CreatedAt   time.Time  `json:"created_at"`
	LastLoginAt *time.Time `json:"last_login_at,omitempty"`
}

// ListUsers backs the admin Team panel -- every account, oldest first (so
// the original owner created by authSetup always shows up top).
func (s *Store) ListUsers(ctx context.Context) ([]UserWithRole, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id, email, role, created_at, last_login_at FROM users ORDER BY created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []UserWithRole
	for rows.Next() {
		var u UserWithRole
		var lastLogin sql.NullTime
		if err := rows.Scan(&u.ID, &u.Email, &u.Role, &u.CreatedAt, &lastLogin); err != nil {
			return nil, err
		}
		if lastLogin.Valid {
			t := lastLogin.Time
			u.LastLoginAt = &t
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// CountOwners is used to refuse deleting the last remaining owner -- doing
// so would permanently lock everyone out of owner-gated actions with no
// way back in (there's no email-based recovery flow).
func (s *Store) CountOwners(ctx context.Context) (int, error) {
	var n int
	err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE role = 'owner'`).Scan(&n)
	return n, err
}

// DeleteUser removes a user and their sessions/workspace membership. Does
// not touch anything they created (github_pats.created_by_user_id,
// deployments.created_by_user_id, etc) -- those columns are nullable
// precisely so a user can be removed without cascading into unrelated
// resources.
func (s *Store) DeleteUser(ctx context.Context, id int64) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DELETE FROM sessions WHERE user_id = ?`, id); err != nil {
		return fmt.Errorf("delete sessions: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM workspace_members WHERE user_id = ?`, id); err != nil {
		return fmt.Errorf("delete workspace_members: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM users WHERE id = ?`, id); err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	return tx.Commit()
}

func (s *Store) GetUserByEmail(ctx context.Context, email string) (UserAuth, error) {
	var u UserAuth
	err := s.DB.QueryRowContext(ctx, `SELECT id, email, password_hash, role FROM users WHERE email = ?`, email).
		Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Role)
	if err == sql.ErrNoRows {
		return UserAuth{}, ErrNotFound
	}
	return u, err
}

func (s *Store) TouchUserLogin(ctx context.Context, userID int64) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE users SET last_login_at = CURRENT_TIMESTAMP WHERE id = ?`, userID)
	return err
}

func (s *Store) GetUserByID(ctx context.Context, id int64) (UserAuth, error) {
	var u UserAuth
	err := s.DB.QueryRowContext(ctx, `SELECT id, email, password_hash, role FROM users WHERE id = ?`, id).
		Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Role)
	if err == sql.ErrNoRows {
		return UserAuth{}, ErrNotFound
	}
	return u, err
}

func (s *Store) UpdateUserPassword(ctx context.Context, id int64, passwordHash string) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE users SET password_hash = ? WHERE id = ?`, passwordHash, id)
	return err
}

func (s *Store) CreateSession(ctx context.Context, userID int64, tokenHash string, expiresAt time.Time, ip, userAgent string) error {
	_, err := s.DB.ExecContext(ctx,
		`INSERT INTO sessions (user_id, token_hash, expires_at, ip_address, user_agent) VALUES (?, ?, ?, ?, ?)`,
		userID, tokenHash, expiresAt, ip, userAgent,
	)
	return err
}

// GetSessionByTokenHash returns the session's user, that user's current
// role, and the session's expiry, in one query -- so RequireAuth doesn't
// need a second round trip just to find out if the caller is an owner.
func (s *Store) GetSessionByTokenHash(ctx context.Context, tokenHash string) (userID int64, role string, expiresAt time.Time, err error) {
	err = s.DB.QueryRowContext(ctx, `
		SELECT sess.user_id, u.role, sess.expires_at
		FROM sessions sess JOIN users u ON u.id = sess.user_id
		WHERE sess.token_hash = ?`, tokenHash).
		Scan(&userID, &role, &expiresAt)
	if err == sql.ErrNoRows {
		return 0, "", time.Time{}, ErrNotFound
	}
	return userID, role, expiresAt, err
}

func (s *Store) DeleteSessionByTokenHash(ctx context.Context, tokenHash string) error {
	_, err := s.DB.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash = ?`, tokenHash)
	return err
}

type SessionInfo struct {
	ID        int64
	UserEmail string
	CreatedAt time.Time
	ExpiresAt time.Time
	IPAddress string
	UserAgent string
}

// ListActiveSessions backs the admin panel's session list + revoke UI.
func (s *Store) ListActiveSessions(ctx context.Context) ([]SessionInfo, error) {
	rows, err := s.DB.QueryContext(ctx, `
		SELECT sess.id, u.email, sess.created_at, sess.expires_at, COALESCE(sess.ip_address,''), COALESCE(sess.user_agent,'')
		FROM sessions sess JOIN users u ON u.id = sess.user_id
		WHERE sess.expires_at > CURRENT_TIMESTAMP
		ORDER BY sess.created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []SessionInfo
	for rows.Next() {
		var si SessionInfo
		if err := rows.Scan(&si.ID, &si.UserEmail, &si.CreatedAt, &si.ExpiresAt, &si.IPAddress, &si.UserAgent); err != nil {
			return nil, err
		}
		out = append(out, si)
	}
	return out, rows.Err()
}

type Node struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	Address   string `json:"address,omitempty"`
	Status    string `json:"status"`
	IsDefault bool   `json:"is_default"`
}

// ListNodes backs the admin panel's node list -- today always exactly one
// local row, and the hook for a future remote worker to appear here.
func (s *Store) ListNodes(ctx context.Context) ([]Node, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id, name, kind, COALESCE(address,''), status, is_default FROM nodes ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Node
	for rows.Next() {
		var n Node
		if err := rows.Scan(&n.ID, &n.Name, &n.Kind, &n.Address, &n.Status, &n.IsDefault); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

// SumConfiguredMemoryMBAll is SumConfiguredMemoryMB without excluding any
// deployment -- the admin panel's "currently allocated" figure.
func (s *Store) SumConfiguredMemoryMBAll(ctx context.Context) (int, error) {
	var total int
	err := s.DB.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(s.memory_limit_mb * d.replicas), 0)
		FROM services s JOIN deployments d ON d.id = s.deployment_id
		WHERE d.status != 'stopped'`,
	).Scan(&total)
	return total, err
}

func (s *Store) CountRunningContainers(ctx context.Context) (int, error) {
	var n int
	err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM services WHERE status = 'running'`).Scan(&n)
	return n, err
}

// ListRunningContainerIDs returns the live Docker container ID of every
// running service -- what the admin resource budget sums `docker stats`
// over to report actual (not just allocated) memory usage.
func (s *Store) ListRunningContainerIDs(ctx context.Context) ([]string, error) {
	rows, err := s.DB.QueryContext(ctx, `
		SELECT container_id_current FROM services
		WHERE status = 'running' AND container_id_current IS NOT NULL AND container_id_current != ''`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

func (s *Store) DeleteSessionByID(ctx context.Context, id int64) error {
	_, err := s.DB.ExecContext(ctx, `DELETE FROM sessions WHERE id = ?`, id)
	return err
}

type Notification struct {
	ID           int64     `json:"id"`
	Type         string    `json:"type"`
	DeploymentID *int64    `json:"deployment_id,omitempty"`
	Channel      string    `json:"channel"`
	Recipient    string    `json:"recipient"`
	Subject      string    `json:"subject"`
	Status       string    `json:"status"`
	ErrorMessage string    `json:"error_message,omitempty"`
	SentAt       time.Time `json:"sent_at"`
}

func (s *Store) CreateNotification(ctx context.Context, n Notification) error {
	_, err := s.DB.ExecContext(ctx,
		`INSERT INTO notifications_log (type, deployment_id, channel, recipient, subject, status, error_message) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		n.Type, n.DeploymentID, n.Channel, n.Recipient, n.Subject, n.Status, nullIfEmpty(n.ErrorMessage),
	)
	return err
}

func (s *Store) ListNotifications(ctx context.Context, limit int) ([]Notification, error) {
	rows, err := s.DB.QueryContext(ctx, `
		SELECT id, type, deployment_id, channel, recipient, subject, status, COALESCE(error_message,''), sent_at
		FROM notifications_log ORDER BY sent_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Notification
	for rows.Next() {
		var n Notification
		var depID sql.NullInt64
		if err := rows.Scan(&n.ID, &n.Type, &depID, &n.Channel, &n.Recipient, &n.Subject, &n.Status, &n.ErrorMessage, &n.SentAt); err != nil {
			return nil, err
		}
		if depID.Valid {
			n.DeploymentID = &depID.Int64
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func (s *Store) DeleteExpiredSessions(ctx context.Context) (int64, error) {
	res, err := s.DB.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at <= CURRENT_TIMESTAMP`)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
