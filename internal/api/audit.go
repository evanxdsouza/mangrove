package api

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/evanxdsouza/mangrove/internal/auth"
	"github.com/evanxdsouza/mangrove/internal/store"
)

// auditActorWebhook is the recorded actor for an automated action with no
// human caller (a GitHub push/PR-preview-triggered deploy) -- there's no
// users row to point at, but "who deployed, and when" should still answer
// something readable rather than silently having no entry at all.
const auditActorWebhook = "github-webhook"

// resolveActor reads the authenticated caller out of ctx for an audit
// record: (userID, email) for a real session, or (nil, auditActorWebhook)
// for a request with none (the GitHub webhook receiver, which verifies by
// HMAC signature instead of a session cookie).
//
// Call this from a handler's own r.Context() -- NOT from inside a deploy
// that's already running under orchestrator.WithInflightDeploy's context,
// which is deliberately created via context.WithCancel(context.Background())
// (see cancel.go's BeginDeploy) so a deploy outlives the HTTP request that
// started it. That context never carries the request's auth values, which
// is exactly the bug this comment exists to stop someone reintroducing:
// resolve the actor in the handler and pass it through explicitly (see
// DeployRequest.ActorUserID/ActorEmail and auditDeploy in deployments.go)
// rather than reading it back out of ctx deep inside a deploy.
func (s *Server) resolveActor(ctx context.Context) (userID *int64, email string) {
	if id, ok := auth.UserIDFromContext(ctx); ok {
		if u, err := s.Store.GetUserByID(ctx, id); err == nil {
			return &id, u.Email
		}
		return &id, "unknown"
	}
	return nil, auditActorWebhook
}

// audit records a deployed/deleted/changed-access event: who (the
// authenticated caller, or auditActorWebhook for an automated trigger)
// did what to which resource, when -- see docs/multi-user.md's "Audit
// log" section for the full action/resource-type vocabulary. Logging
// failures are swallowed (logged, not surfaced) -- an audit write should
// never be the reason the real action it's recording fails.
//
// workspaceID is optional (nil for an org-level action like user
// management); pass auth.WorkspaceIDFromContext(ctx)'s value when the
// route already ran through RequireWorkspaceRole, which resolved it once
// already -- see workspace_resolvers.go's callers for the pattern.
func (s *Server) audit(ctx context.Context, action, resourceType string, resourceID int64, workspaceID *int64, detail string) {
	userID, email := s.resolveActor(ctx)
	s.recordAudit(ctx, userID, email, action, resourceType, resourceID, workspaceID, detail)
}

// recordAudit is audit() with the actor passed explicitly instead of read
// back from ctx -- for auditDeploy, which runs under a deploy's own
// detached context (see resolveActor's comment on why that context can
// never carry an actor of its own).
func (s *Server) recordAudit(ctx context.Context, actorUserID *int64, actorEmail, action, resourceType string, resourceID int64, workspaceID *int64, detail string) {
	if actorEmail == "" {
		actorEmail = auditActorWebhook
	}
	e := store.AuditEvent{
		ActorUserID:  actorUserID,
		ActorEmail:   actorEmail,
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   &resourceID,
		WorkspaceID:  workspaceID,
		Detail:       detail,
	}
	if err := s.Store.RecordAuditEvent(ctx, e); err != nil {
		s.Log.Warn("audit log write failed", "action", action, "resource_type", resourceType, "error", err)
	}
}

// auditWorkspaceScoped is audit(), but for a route with no single
// resource ID to hang RequireWorkspaceRole off, so workspaceID has to be
// resolved by the caller and passed explicitly instead of read back from
// context.
func (s *Server) auditWorkspaceScoped(ctx context.Context, action, resourceType string, resourceID, workspaceID int64, detail string) {
	s.audit(ctx, action, resourceType, resourceID, &workspaceID, detail)
}

// auditCtxWorkspace is audit(), reading the workspace ID back from
// context instead of taking it as a param -- the common case for any
// route already wrapped in RequireWorkspaceRole (which resolved it once
// to run the permission check in the first place; see workspace.go's
// WorkspaceIDFromContext).
func (s *Server) auditCtxWorkspace(ctx context.Context, action, resourceType string, resourceID int64, detail string) {
	var workspaceID *int64
	if id, ok := auth.WorkspaceIDFromContext(ctx); ok {
		workspaceID = &id
	}
	s.audit(ctx, action, resourceType, resourceID, workspaceID, detail)
}

// ---- read endpoints ----

// listWorkspaceAuditLog is viewer+ in the workspace (wired via
// RequireWorkspaceRole in router.go) -- a workspace's own members can see
// its history, scoped to only that workspace.
func (s *Server) listWorkspaceAuditLog(w http.ResponseWriter, r *http.Request) {
	workspaceID, err := parseID(chi.URLParam(r, "workspaceID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid workspace id")
		return
	}
	events, err := s.Store.ListAuditEventsForWorkspace(r.Context(), workspaceID, 500)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, events)
}

// listAllAuditLog is owner-only (wired via RequireOwner in router.go,
// under /admin like the rest of the host-level surface) -- every event
// across every workspace, including org-level ones (user management)
// that have no workspace_id at all.
func (s *Server) listAllAuditLog(w http.ResponseWriter, r *http.Request) {
	events, err := s.Store.ListAllAuditEvents(r.Context(), 1000)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, events)
}
