package store

import (
	"context"
	"database/sql"
	"time"
)

// ---- GitHub PATs ----

type GithubPAT struct {
	ID          int64      `json:"id"`
	Label       string     `json:"label"`
	Source      string     `json:"source"` // "pat" (pasted) or "oauth" (connected via GitHub OAuth)
	GithubLogin string     `json:"github_login,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	LastUsedAt  *time.Time `json:"last_used_at,omitempty"`
}

func (s *Store) CreateGithubPAT(ctx context.Context, label string, ciphertext, nonce []byte) (int64, error) {
	res, err := s.DB.ExecContext(ctx,
		`INSERT INTO github_pats (org_id, label, token_encrypted, token_nonce, source) VALUES (1, ?, ?, ?, 'pat')`,
		label, ciphertext, nonce,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// CreateGithubOAuthToken stores a token obtained through the GitHub OAuth
// flow -- same table and encryption as a pasted PAT (see
// internal/api/github.go's re-seal-with-real-id pattern), just tagged with
// where it came from and which account it belongs to.
func (s *Store) CreateGithubOAuthToken(ctx context.Context, login string, ciphertext, nonce []byte) (int64, error) {
	res, err := s.DB.ExecContext(ctx,
		`INSERT INTO github_pats (org_id, label, token_encrypted, token_nonce, source, github_login) VALUES (1, ?, ?, ?, 'oauth', ?)`,
		"GitHub (@"+login+")", ciphertext, nonce, login,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) ListGithubPATs(ctx context.Context) ([]GithubPAT, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id, label, source, COALESCE(github_login,''), created_at, last_used_at FROM github_pats ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []GithubPAT
	for rows.Next() {
		var p GithubPAT
		var lastUsed sql.NullTime
		if err := rows.Scan(&p.ID, &p.Label, &p.Source, &p.GithubLogin, &p.CreatedAt, &lastUsed); err != nil {
			return nil, err
		}
		if lastUsed.Valid {
			p.LastUsedAt = &lastUsed.Time
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// GetGithubPAT returns a credential's metadata (label/source/login) without
// touching the encrypted token -- used where callers need to branch on
// Source (e.g. "was this OAuth-obtained, so can it also register a
// webhook?") but don't need to decrypt anything.
func (s *Store) GetGithubPAT(ctx context.Context, id int64) (GithubPAT, error) {
	var p GithubPAT
	var lastUsed sql.NullTime
	err := s.DB.QueryRowContext(ctx,
		`SELECT id, label, source, COALESCE(github_login,''), created_at, last_used_at FROM github_pats WHERE id = ?`, id,
	).Scan(&p.ID, &p.Label, &p.Source, &p.GithubLogin, &p.CreatedAt, &lastUsed)
	if err == sql.ErrNoRows {
		return GithubPAT{}, ErrNotFound
	}
	if err != nil {
		return GithubPAT{}, err
	}
	if lastUsed.Valid {
		p.LastUsedAt = &lastUsed.Time
	}
	return p, nil
}

func (s *Store) DeleteGithubPAT(ctx context.Context, id int64) error {
	_, err := s.DB.ExecContext(ctx, `DELETE FROM github_pats WHERE id = ?`, id)
	return err
}

func (s *Store) GetGithubPATEncrypted(ctx context.Context, id int64) (ciphertext, nonce []byte, err error) {
	err = s.DB.QueryRowContext(ctx, `SELECT token_encrypted, token_nonce FROM github_pats WHERE id = ?`, id).Scan(&ciphertext, &nonce)
	if err == sql.ErrNoRows {
		return nil, nil, ErrNotFound
	}
	return ciphertext, nonce, err
}

func (s *Store) TouchGithubPATUsed(ctx context.Context, id int64) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE github_pats SET last_used_at = CURRENT_TIMESTAMP WHERE id = ?`, id)
	return err
}

// ---- Project repos ----

type ProjectRepo struct {
	ID            int64     `json:"id"`
	ProjectID     int64     `json:"project_id"`
	GithubPATID   int64     `json:"github_pat_id"`
	RepoOwner     string    `json:"repo_owner"`
	RepoName      string    `json:"repo_name"`
	DefaultBranch string    `json:"default_branch"`
	WebhookToken  string    `json:"webhook_token"`
	CreatedAt     time.Time `json:"created_at"`
}

func (s *Store) CreateProjectRepo(ctx context.Context, projectID, patID int64, owner, repo, defaultBranch, webhookToken string, secretCiphertext, secretNonce []byte) (int64, error) {
	res, err := s.DB.ExecContext(ctx, `
		INSERT INTO project_repos (project_id, github_pat_id, repo_owner, repo_name, default_branch, webhook_token, webhook_secret_encrypted, webhook_secret_nonce)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		projectID, patID, owner, repo, defaultBranch, webhookToken, secretCiphertext, secretNonce,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) GetProjectRepoByID(ctx context.Context, id int64) (ProjectRepo, error) {
	var pr ProjectRepo
	err := s.DB.QueryRowContext(ctx, `
		SELECT id, project_id, github_pat_id, repo_owner, repo_name, default_branch, webhook_token, created_at
		FROM project_repos WHERE id = ?`, id,
	).Scan(&pr.ID, &pr.ProjectID, &pr.GithubPATID, &pr.RepoOwner, &pr.RepoName, &pr.DefaultBranch, &pr.WebhookToken, &pr.CreatedAt)
	if err == sql.ErrNoRows {
		return ProjectRepo{}, ErrNotFound
	}
	return pr, err
}

func (s *Store) GetProjectRepoByProject(ctx context.Context, projectID int64) (ProjectRepo, error) {
	var pr ProjectRepo
	err := s.DB.QueryRowContext(ctx, `
		SELECT id, project_id, github_pat_id, repo_owner, repo_name, default_branch, webhook_token, created_at
		FROM project_repos WHERE project_id = ?`, projectID,
	).Scan(&pr.ID, &pr.ProjectID, &pr.GithubPATID, &pr.RepoOwner, &pr.RepoName, &pr.DefaultBranch, &pr.WebhookToken, &pr.CreatedAt)
	if err == sql.ErrNoRows {
		return ProjectRepo{}, ErrNotFound
	}
	return pr, err
}

// GetProjectRepoByWebhookToken also returns the decrypted-ready secret
// fields -- the webhook handler needs the raw ciphertext/nonce to verify
// the request signature before doing anything else.
type ProjectRepoWithSecret struct {
	ProjectRepo
	SecretCiphertext []byte
	SecretNonce      []byte
}

func (s *Store) GetProjectRepoByWebhookToken(ctx context.Context, token string) (ProjectRepoWithSecret, error) {
	var pr ProjectRepoWithSecret
	err := s.DB.QueryRowContext(ctx, `
		SELECT id, project_id, github_pat_id, repo_owner, repo_name, default_branch, webhook_token, created_at,
		       webhook_secret_encrypted, webhook_secret_nonce
		FROM project_repos WHERE webhook_token = ?`, token,
	).Scan(&pr.ID, &pr.ProjectID, &pr.GithubPATID, &pr.RepoOwner, &pr.RepoName, &pr.DefaultBranch, &pr.WebhookToken, &pr.CreatedAt,
		&pr.SecretCiphertext, &pr.SecretNonce)
	if err == sql.ErrNoRows {
		return ProjectRepoWithSecret{}, ErrNotFound
	}
	return pr, err
}

func (s *Store) DeleteProjectRepo(ctx context.Context, id int64) error {
	_, err := s.DB.ExecContext(ctx, `DELETE FROM project_repos WHERE id = ?`, id)
	return err
}

// SetDeploymentRepo links a deployment to a project_repos row and the
// branch that should trigger auto-deploy -- configurable per deployment,
// not hardcoded to main.
func (s *Store) SetDeploymentRepo(ctx context.Context, deploymentID, projectRepoID int64, branch string, autoDeployOnPush bool) error {
	_, err := s.DB.ExecContext(ctx,
		`UPDATE deployments SET project_repo_id = ?, git_branch = ?, auto_deploy_on_push = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		projectRepoID, branch, autoDeployOnPush, deploymentID,
	)
	return err
}

// ListDeploymentsForRepoBranch finds every deployment configured to
// auto-deploy a given project_repos + branch combination -- what a `push`
// webhook event fans out to.
func (s *Store) ListDeploymentsForRepoBranch(ctx context.Context, projectRepoID int64, branch string) ([]int64, error) {
	rows, err := s.DB.QueryContext(ctx,
		`SELECT id FROM deployments WHERE project_repo_id = ? AND git_branch = ? AND auto_deploy_on_push = 1`,
		projectRepoID, branch,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// ---- Webhook events ----

func (s *Store) CreateWebhookEvent(ctx context.Context, deliveryID, eventType string, projectRepoID *int64, signatureValid bool, payloadJSON string) (int64, error) {
	res, err := s.DB.ExecContext(ctx, `
		INSERT INTO webhook_events (source, delivery_id, event_type, project_repo_id, signature_valid, payload_json, status)
		VALUES ('github', ?, ?, ?, ?, ?, 'received')`,
		deliveryID, eventType, projectRepoID, signatureValid, payloadJSON,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) WebhookDeliveryExists(ctx context.Context, deliveryID string) (bool, error) {
	var n int
	err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM webhook_events WHERE delivery_id = ?`, deliveryID).Scan(&n)
	return n > 0, err
}

func (s *Store) UpdateWebhookEventStatus(ctx context.Context, id int64, status string, deployHistoryID *int64) error {
	_, err := s.DB.ExecContext(ctx,
		`UPDATE webhook_events SET status = ?, deploy_history_id = ?, processed_at = CURRENT_TIMESTAMP WHERE id = ?`,
		status, deployHistoryID, id,
	)
	return err
}

// ---- GitHub OAuth CSRF state ----

// CreateGithubOAuthState records a random, single-use state value before
// redirecting the user's browser to GitHub, bound to both the user who
// started the flow and the exact redirect_uri used (GitHub requires it to
// match on token exchange).
func (s *Store) CreateGithubOAuthState(ctx context.Context, state string, userID int64, redirectURI string, ttl time.Duration) error {
	_, err := s.DB.ExecContext(ctx,
		`INSERT INTO github_oauth_states (state, user_id, redirect_uri, expires_at) VALUES (?, ?, ?, ?)`,
		state, userID, redirectURI, time.Now().Add(ttl),
	)
	return err
}

// ConsumeGithubOAuthState validates and deletes a state value in one shot
// (single-use, whether or not it's valid) -- returns the redirect_uri it
// was created with, or ErrNotFound if the state is missing, already used,
// expired, or belongs to a different user than the callback's session.
func (s *Store) ConsumeGithubOAuthState(ctx context.Context, state string, userID int64) (redirectURI string, err error) {
	err = s.DB.QueryRowContext(ctx,
		`SELECT redirect_uri FROM github_oauth_states WHERE state = ? AND user_id = ? AND expires_at > CURRENT_TIMESTAMP`,
		state, userID,
	).Scan(&redirectURI)
	s.DB.ExecContext(ctx, `DELETE FROM github_oauth_states WHERE state = ?`, state)
	if err == sql.ErrNoRows {
		return "", ErrNotFound
	}
	return redirectURI, err
}
