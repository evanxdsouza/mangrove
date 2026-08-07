// Package store is the single place that turns SQLite rows into
// internal/models types and back. API handlers and the orchestrator both
// depend on this rather than writing SQL themselves.
package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/evanxdsouza/mangrove/internal/models"
)

type Store struct {
	DB *sql.DB
}

func New(db *sql.DB) *Store { return &Store{DB: db} }

var ErrNotFound = fmt.Errorf("not found")

// ---- Projects ----

func (s *Store) CreateProject(ctx context.Context, name, slug, description string) (models.Project, error) {
	res, err := s.DB.ExecContext(ctx,
		`INSERT INTO projects (workspace_id, name, slug, description) VALUES (1, ?, ?, ?)`,
		name, slug, description,
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

func (s *Store) ListProjects(ctx context.Context) ([]models.Project, error) {
	rows, err := s.DB.QueryContext(ctx,
		`SELECT id, workspace_id, name, slug, COALESCE(description,''), created_at, updated_at FROM projects ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.Project
	for rows.Next() {
		var p models.Project
		if err := rows.Scan(&p.ID, &p.WorkspaceID, &p.Name, &p.Slug, &p.Description, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Store) DeleteProject(ctx context.Context, id int64) error {
	_, err := s.DB.ExecContext(ctx, `DELETE FROM projects WHERE id = ?`, id)
	return err
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
	ImageRetentionCount int
}

func (s *Store) CreateDeployment(ctx context.Context, p CreateDeploymentParams) (models.Deployment, error) {
	if p.RootPath == "" {
		p.RootPath = "."
	}
	if p.ImageRetentionCount == 0 {
		p.ImageRetentionCount = 5
	}
	res, err := s.DB.ExecContext(ctx,
		`INSERT INTO deployments (project_id, name, slug, build_strategy, git_branch, image_ref, root_path, dockerfile_path, compose_path, image_retention_count)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.ProjectID, p.Name, p.Slug, p.BuildStrategy, p.GitBranch, p.ImageRef, p.RootPath, p.DockerfilePath, p.ComposePath, p.ImageRetentionCount,
	)
	if err != nil {
		return models.Deployment{}, err
	}
	id, _ := res.LastInsertId()
	return s.GetDeployment(ctx, id)
}

func (s *Store) GetDeployment(ctx context.Context, id int64) (models.Deployment, error) {
	var d models.Deployment
	var projectRepoID, lastDeployedAt sql.NullInt64
	var lastDeployedAtT sql.NullTime
	err := s.DB.QueryRowContext(ctx, `
		SELECT id, project_id, name, slug, build_strategy, COALESCE(git_branch,''), project_repo_id,
		       COALESCE(image_ref,''), root_path, COALESCE(dockerfile_path,''), COALESCE(compose_path,''),
		       auto_deploy_on_push, is_public, password_protected, image_retention_count, status, node_id,
		       created_at, updated_at, last_deployed_at
		FROM deployments WHERE id = ?`, id,
	).Scan(&d.ID, &d.ProjectID, &d.Name, &d.Slug, &d.BuildStrategy, &d.GitBranch, &projectRepoID,
		&d.ImageRef, &d.RootPath, &d.DockerfilePath, &d.ComposePath,
		&d.AutoDeployOnPush, &d.IsPublic, &d.PasswordProtected, &d.ImageRetentionCount, &d.Status, &d.NodeID,
		&d.CreatedAt, &d.UpdatedAt, &lastDeployedAtT)
	if err == sql.ErrNoRows {
		return models.Deployment{}, ErrNotFound
	}
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
	_ = lastDeployedAt
	return d, nil
}

func (s *Store) ListDeployments(ctx context.Context, projectID int64) ([]models.Deployment, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id FROM deployments WHERE project_id = ? ORDER BY created_at DESC`, projectID)
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

	out := make([]models.Deployment, 0, len(ids))
	for _, id := range ids {
		d, err := s.GetDeployment(ctx, id)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, nil
}

func (s *Store) DeleteDeployment(ctx context.Context, id int64) error {
	_, err := s.DB.ExecContext(ctx, `DELETE FROM deployments WHERE id = ?`, id)
	return err
}

// ListAllDeployments returns every deployment across every project --
// used by the pruning scheduler, which sweeps the whole host rather than
// one project at a time.
func (s *Store) ListAllDeployments(ctx context.Context) ([]models.Deployment, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id FROM deployments ORDER BY id`)
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

	out := make([]models.Deployment, 0, len(ids))
	for _, id := range ids {
		d, err := s.GetDeployment(ctx, id)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, nil
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
}

func (s *Store) CreateService(ctx context.Context, p CreateServiceParams) (models.Service, error) {
	if p.CPULimitCores == 0 {
		p.CPULimitCores = 0.5
	}
	if p.MemoryLimitMB == 0 {
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
	res, err := s.DB.ExecContext(ctx, `
		INSERT INTO services (deployment_id, name, container_name, internal_port, is_internal_only,
		                       cpu_limit_cores, memory_limit_mb, restart_policy, health_check_path,
		                       health_check_interval_s, health_check_timeout_s)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.DeploymentID, p.Name, p.ContainerName, p.InternalPort, p.IsInternalOnly,
		p.CPULimitCores, p.MemoryLimitMB, p.RestartPolicy, nullIfEmpty(p.HealthCheckPath),
		p.HealthCheckIntervalS, p.HealthCheckTimeoutS,
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
	err := s.DB.QueryRowContext(ctx, `
		SELECT id, deployment_id, name, is_primary, COALESCE(image_tag_current,''), COALESCE(container_id_current,''),
		       container_name, internal_port, host_port, is_internal_only, cpu_limit_cores, memory_limit_mb,
		       restart_policy, COALESCE(health_check_path,''), health_check_interval_s, health_check_timeout_s,
		       status, created_at, updated_at
		FROM services WHERE id = ?`, id,
	).Scan(&svc.ID, &svc.DeploymentID, &svc.Name, &svc.IsPrimary, &svc.ImageTagCurrent, &svc.ContainerIDCurrent,
		&svc.ContainerName, &svc.InternalPort, &hostPort, &svc.IsInternalOnly, &svc.CPULimitCores, &svc.MemoryLimitMB,
		&svc.RestartPolicy, &svc.HealthCheckPath, &svc.HealthCheckIntervalS, &svc.HealthCheckTimeoutS,
		&svc.Status, &svc.CreatedAt, &svc.UpdatedAt)
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

func (s *Store) UpdateServiceInternalOnly(ctx context.Context, id int64, internalOnly bool) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE services SET is_internal_only = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, internalOnly, id)
	return err
}

func (s *Store) UpdateServiceStatus(ctx context.Context, id int64, status string) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE services SET status = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, status, id)
	return err
}

// SumConfiguredMemoryMB returns the total memory_limit_mb across every
// service belonging to a non-stopped deployment -- the figure admission
// control checks against the deployment memory ceiling, and what the
// resource-budget dashboard view shows as "allocated".
func (s *Store) SumConfiguredMemoryMB(ctx context.Context, excludeDeploymentID int64) (int, error) {
	var total int
	err := s.DB.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(s.memory_limit_mb), 0)
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

func (s *Store) CreateDeployArtifact(ctx context.Context, deployHistoryID, serviceID int64, imageTag, imageID, buildLogPath string) error {
	_, err := s.DB.ExecContext(ctx,
		`INSERT INTO deploy_history_artifacts (deploy_history_id, service_id, image_tag, image_id, build_log_path) VALUES (?, ?, ?, ?, ?)`,
		deployHistoryID, serviceID, imageTag, nullIfEmpty(imageID), nullIfEmpty(buildLogPath),
	)
	return err
}

// ListArtifactsForService returns every build produced for a service,
// newest first -- the rollback list, and what image-retention pruning
// walks past the keep count.
func (s *Store) ListArtifactsForService(ctx context.Context, serviceID int64) ([]models.DeployArtifact, error) {
	rows, err := s.DB.QueryContext(ctx, `
		SELECT a.id, a.deploy_history_id, a.service_id, a.image_tag, COALESCE(a.image_id,''), COALESCE(a.build_log_path,'')
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
		if err := rows.Scan(&a.ID, &a.DeployHistoryID, &a.ServiceID, &a.ImageTag, &a.ImageID, &a.BuildLogPath); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *Store) GetArtifactForServiceAtDeployHistory(ctx context.Context, deployHistoryID, serviceID int64) (models.DeployArtifact, error) {
	var a models.DeployArtifact
	err := s.DB.QueryRowContext(ctx, `
		SELECT id, deploy_history_id, service_id, image_tag, COALESCE(image_id,''), COALESCE(build_log_path,'')
		FROM deploy_history_artifacts WHERE deploy_history_id = ? AND service_id = ?`, deployHistoryID, serviceID,
	).Scan(&a.ID, &a.DeployHistoryID, &a.ServiceID, &a.ImageTag, &a.ImageID, &a.BuildLogPath)
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

func (s *Store) CreateUser(ctx context.Context, email, passwordHash string) (int64, error) {
	res, err := s.DB.ExecContext(ctx, `INSERT INTO users (org_id, email, password_hash) VALUES (1, ?, ?)`, email, passwordHash)
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
	if _, err := s.DB.ExecContext(ctx, `INSERT INTO workspace_members (workspace_id, user_id) VALUES (1, ?)`, id); err != nil {
		return 0, err
	}
	return id, nil
}

type UserAuth struct {
	ID           int64
	Email        string
	PasswordHash string
}

func (s *Store) GetUserByEmail(ctx context.Context, email string) (UserAuth, error) {
	var u UserAuth
	err := s.DB.QueryRowContext(ctx, `SELECT id, email, password_hash FROM users WHERE email = ?`, email).
		Scan(&u.ID, &u.Email, &u.PasswordHash)
	if err == sql.ErrNoRows {
		return UserAuth{}, ErrNotFound
	}
	return u, err
}

func (s *Store) TouchUserLogin(ctx context.Context, userID int64) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE users SET last_login_at = CURRENT_TIMESTAMP WHERE id = ?`, userID)
	return err
}

func (s *Store) CreateSession(ctx context.Context, userID int64, tokenHash string, expiresAt time.Time, ip, userAgent string) error {
	_, err := s.DB.ExecContext(ctx,
		`INSERT INTO sessions (user_id, token_hash, expires_at, ip_address, user_agent) VALUES (?, ?, ?, ?, ?)`,
		userID, tokenHash, expiresAt, ip, userAgent,
	)
	return err
}

func (s *Store) GetSessionByTokenHash(ctx context.Context, tokenHash string) (userID int64, expiresAt time.Time, err error) {
	err = s.DB.QueryRowContext(ctx, `SELECT user_id, expires_at FROM sessions WHERE token_hash = ?`, tokenHash).
		Scan(&userID, &expiresAt)
	if err == sql.ErrNoRows {
		return 0, time.Time{}, ErrNotFound
	}
	return userID, expiresAt, err
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
		SELECT COALESCE(SUM(s.memory_limit_mb), 0)
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
