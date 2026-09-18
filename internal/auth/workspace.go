package auth

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/evanxdsouza/mangrove/internal/store"
)

// workspaceRoleRank orders the three per-workspace roles so "at least
// editor" etc. is a simple integer comparison. Higher is more privileged.
var workspaceRoleRank = map[string]int{
	"viewer": 1,
	"editor": 2,
	"admin":  3,
}

// HasWorkspaceRole reports whether the context's authenticated caller has
// at least minRole ("viewer", "editor", or "admin") in workspaceID. A
// global owner (users.role, not a workspace_members row) always does --
// implicit admin everywhere, the same superset relationship RequireOwner
// already establishes for host-level routes. Exported so handlers that
// can't hang RequireWorkspaceRole off a single URL param (e.g. a
// workspace_id that arrives in the JSON body, or a check against two
// different workspaces in one request) can run the identical check inline.
func HasWorkspaceRole(ctx context.Context, st *store.Store, minRole string, workspaceID int64) (bool, error) {
	if role, _ := RoleFromContext(ctx); role == "owner" {
		return true, nil
	}
	userID, ok := UserIDFromContext(ctx)
	if !ok {
		return false, nil
	}
	memberRole, has, err := st.WorkspaceRole(ctx, userID, workspaceID)
	if err != nil {
		return false, err
	}
	if !has {
		return false, nil
	}
	return workspaceRoleRank[memberRole] >= workspaceRoleRank[minRole], nil
}

// RequireWorkspaceRole rejects any request whose caller doesn't have at
// least minRole in the workspace resolve identifies for this specific
// request. Must be mounted inside RequireAuth, same as RequireOwner.
//
// resolve turns the request's URL params into a workspace ID (e.g. "load
// {deploymentID}, look up its workspace"); it should return
// store.ErrNotFound for a resource that doesn't exist at all, which this
// middleware surfaces as 404 rather than 403 -- a caller with no access to
// a resource shouldn't be able to distinguish "exists, not yours" from
// "doesn't exist" via status code, matching how a 404 already behaves for
// a nonexistent project/deployment/service today.
func RequireWorkspaceRole(st *store.Store, minRole string, resolve func(r *http.Request) (int64, error)) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			workspaceID, err := resolve(r)
			if err == store.ErrNotFound {
				writeWorkspaceRoleError(w, http.StatusNotFound, "not found")
				return
			}
			if err != nil {
				writeWorkspaceRoleError(w, http.StatusInternalServerError, err.Error())
				return
			}

			ok, err := HasWorkspaceRole(r.Context(), st, minRole, workspaceID)
			if err != nil {
				writeWorkspaceRoleError(w, http.StatusInternalServerError, err.Error())
				return
			}
			if !ok {
				writeWorkspaceRoleError(w, http.StatusForbidden, minRole+" role (in this workspace) required")
				return
			}
			// Stashed so a handler that wants to record an audit event
			// (internal/api/audit.go) doesn't have to re-resolve the same
			// workspace ID this middleware already looked up.
			ctx := context.WithValue(r.Context(), workspaceIDContextKey, workspaceID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// WorkspaceIDFromContext returns the workspace ID RequireWorkspaceRole
// already resolved for this request, if any.
func WorkspaceIDFromContext(ctx context.Context) (int64, bool) {
	id, ok := ctx.Value(workspaceIDContextKey).(int64)
	return id, ok
}

func writeWorkspaceRoleError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
