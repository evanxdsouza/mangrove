package store

import (
	"context"
	"database/sql"
	"time"
)

// AuditEvent is one row in audit_log -- who did what to which resource,
// and when. ActorUserID is nil for an automated actor with no user row
// (a GitHub webhook-triggered deploy); ActorEmail is always populated
// (denormalized, so it stays readable if the actor's account is later
// deleted, or reads e.g. "github-webhook" for an automated one).
// WorkspaceID is nil for an org-level action (user account management).
type AuditEvent struct {
	ID           int64     `json:"id"`
	ActorUserID  *int64    `json:"actor_user_id,omitempty"`
	ActorEmail   string    `json:"actor_email"`
	Action       string    `json:"action"`
	ResourceType string    `json:"resource_type"`
	ResourceID   *int64    `json:"resource_id,omitempty"`
	WorkspaceID  *int64    `json:"workspace_id,omitempty"`
	Detail       string    `json:"detail,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

func (s *Store) RecordAuditEvent(ctx context.Context, e AuditEvent) error {
	_, err := s.DB.ExecContext(ctx,
		`INSERT INTO audit_log (actor_user_id, actor_email, action, resource_type, resource_id, workspace_id, detail)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		e.ActorUserID, e.ActorEmail, e.Action, e.ResourceType, e.ResourceID, e.WorkspaceID, nullIfEmpty(e.Detail),
	)
	return err
}

// ListAuditEventsForWorkspace returns a workspace's own audit trail,
// newest first, capped at limit rows -- what a workspace member (viewer+)
// sees, scoped to only the workspace they asked about. Ordered by id, not
// just created_at -- CURRENT_TIMESTAMP only has second resolution, and
// two events in the same second (routine under real load) would otherwise
// sort ambiguously.
func (s *Store) ListAuditEventsForWorkspace(ctx context.Context, workspaceID int64, limit int) ([]AuditEvent, error) {
	return s.queryAuditEvents(ctx, `WHERE workspace_id = ? ORDER BY id DESC LIMIT ?`, workspaceID, limit)
}

// ListAllAuditEvents returns every audit event across the whole box,
// newest first, capped at limit rows -- owner-only (internal/api/admin.go).
func (s *Store) ListAllAuditEvents(ctx context.Context, limit int) ([]AuditEvent, error) {
	return s.queryAuditEvents(ctx, `ORDER BY id DESC LIMIT ?`, limit)
}

func (s *Store) queryAuditEvents(ctx context.Context, whereOrderLimit string, args ...any) ([]AuditEvent, error) {
	rows, err := s.DB.QueryContext(ctx, `
		SELECT id, actor_user_id, actor_email, action, resource_type, resource_id, workspace_id, COALESCE(detail, ''), created_at
		FROM audit_log `+whereOrderLimit, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]AuditEvent, 0)
	for rows.Next() {
		var e AuditEvent
		var actorUserID, resourceID, workspaceID sql.NullInt64
		if err := rows.Scan(&e.ID, &actorUserID, &e.ActorEmail, &e.Action, &e.ResourceType, &resourceID, &workspaceID, &e.Detail, &e.CreatedAt); err != nil {
			return nil, err
		}
		if actorUserID.Valid {
			e.ActorUserID = &actorUserID.Int64
		}
		if resourceID.Valid {
			e.ResourceID = &resourceID.Int64
		}
		if workspaceID.Valid {
			e.WorkspaceID = &workspaceID.Int64
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
