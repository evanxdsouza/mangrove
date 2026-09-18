package store

import (
	"context"
	"database/sql"
	"time"
)

// WorkspaceIDForProject resolves a project ID to its workspace -- a direct
// column read, since projects.workspace_id is already the FK.
func (s *Store) WorkspaceIDForProject(ctx context.Context, projectID int64) (int64, error) {
	var workspaceID int64
	err := s.DB.QueryRowContext(ctx, `SELECT workspace_id FROM projects WHERE id = ?`, projectID).Scan(&workspaceID)
	if err == sql.ErrNoRows {
		return 0, ErrNotFound
	}
	return workspaceID, err
}

// WorkspaceIDForDeployment resolves a deployment ID to its workspace via
// deployments -> projects.
func (s *Store) WorkspaceIDForDeployment(ctx context.Context, deploymentID int64) (int64, error) {
	var workspaceID int64
	err := s.DB.QueryRowContext(ctx, `
		SELECT p.workspace_id
		FROM deployments d JOIN projects p ON p.id = d.project_id
		WHERE d.id = ?`, deploymentID).Scan(&workspaceID)
	if err == sql.ErrNoRows {
		return 0, ErrNotFound
	}
	return workspaceID, err
}

// WorkspaceIDForService resolves a service ID to its workspace via
// services -> deployments -> projects.
func (s *Store) WorkspaceIDForService(ctx context.Context, serviceID int64) (int64, error) {
	var workspaceID int64
	err := s.DB.QueryRowContext(ctx, `
		SELECT p.workspace_id
		FROM services sv
		JOIN deployments d ON d.id = sv.deployment_id
		JOIN projects p ON p.id = d.project_id
		WHERE sv.id = ?`, serviceID).Scan(&workspaceID)
	if err == sql.ErrNoRows {
		return 0, ErrNotFound
	}
	return workspaceID, err
}

// WorkspaceIDForCustomDomain resolves a custom domain ID to its workspace
// via custom_domains -> deployments -> projects.
func (s *Store) WorkspaceIDForCustomDomain(ctx context.Context, domainID int64) (int64, error) {
	var workspaceID int64
	err := s.DB.QueryRowContext(ctx, `
		SELECT p.workspace_id
		FROM custom_domains cd
		JOIN deployments d ON d.id = cd.deployment_id
		JOIN projects p ON p.id = d.project_id
		WHERE cd.id = ?`, domainID).Scan(&workspaceID)
	if err == sql.ErrNoRows {
		return 0, ErrNotFound
	}
	return workspaceID, err
}

// WorkspaceIDForDeployHistory resolves a deploy_history ID to its workspace
// via deploy_history -> deployments -> projects.
func (s *Store) WorkspaceIDForDeployHistory(ctx context.Context, historyID int64) (int64, error) {
	var workspaceID int64
	err := s.DB.QueryRowContext(ctx, `
		SELECT p.workspace_id
		FROM deploy_history dh
		JOIN deployments d ON d.id = dh.deployment_id
		JOIN projects p ON p.id = d.project_id
		WHERE dh.id = ?`, historyID).Scan(&workspaceID)
	if err == sql.ErrNoRows {
		return 0, ErrNotFound
	}
	return workspaceID, err
}

// WorkspaceRole returns the caller's role in a workspace. ok=false (with a
// nil error) means they have no workspace_members row there at all --
// distinct from an actual DB error, and distinct from a global owner's
// implicit access (callers check that separately; this only ever reflects
// an explicit row).
func (s *Store) WorkspaceRole(ctx context.Context, userID, workspaceID int64) (role string, ok bool, err error) {
	err = s.DB.QueryRowContext(ctx,
		`SELECT role FROM workspace_members WHERE workspace_id = ? AND user_id = ?`,
		workspaceID, userID,
	).Scan(&role)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return role, true, nil
}

// AddWorkspaceMember grants a user an explicit role in a workspace.
// Returns ErrDuplicate if they already have a membership row there --
// callers should route them to SetWorkspaceMemberRole instead.
func (s *Store) AddWorkspaceMember(ctx context.Context, workspaceID, userID int64, role string) error {
	_, err := s.DB.ExecContext(ctx,
		`INSERT INTO workspace_members (workspace_id, user_id, role) VALUES (?, ?, ?)`,
		workspaceID, userID, role,
	)
	if isUniqueConstraintErr(err) {
		return ErrDuplicate
	}
	return err
}

// SetWorkspaceMemberRole changes an existing member's role. Returns
// ErrNotFound if they have no membership row to update.
func (s *Store) SetWorkspaceMemberRole(ctx context.Context, workspaceID, userID int64, role string) error {
	res, err := s.DB.ExecContext(ctx,
		`UPDATE workspace_members SET role = ? WHERE workspace_id = ? AND user_id = ?`,
		role, workspaceID, userID,
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

// RemoveWorkspaceMember revokes a user's membership in a workspace. Does
// not touch their org account (internal/api/admin.go's user management is
// the only place that deletes an account outright).
func (s *Store) RemoveWorkspaceMember(ctx context.Context, workspaceID, userID int64) error {
	res, err := s.DB.ExecContext(ctx,
		`DELETE FROM workspace_members WHERE workspace_id = ? AND user_id = ?`,
		workspaceID, userID,
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

// WorkspaceMemberWithEmail is a workspace_members row joined with the
// member's email, for the workspace Members panel.
type WorkspaceMemberWithEmail struct {
	UserID    int64     `json:"user_id"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}

func (s *Store) ListWorkspaceMembers(ctx context.Context, workspaceID int64) ([]WorkspaceMemberWithEmail, error) {
	rows, err := s.DB.QueryContext(ctx, `
		SELECT wm.user_id, u.email, wm.role, wm.created_at
		FROM workspace_members wm JOIN users u ON u.id = wm.user_id
		WHERE wm.workspace_id = ?
		ORDER BY wm.created_at ASC`, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []WorkspaceMemberWithEmail
	for rows.Next() {
		var m WorkspaceMemberWithEmail
		if err := rows.Scan(&m.UserID, &m.Email, &m.Role, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}
