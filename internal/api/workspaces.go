package api

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/evanxdsouza/mangrove/internal/auth"
	"github.com/evanxdsouza/mangrove/internal/store"
)

func (s *Server) listWorkspaces(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserIDFromContext(r.Context())
	role, _ := auth.RoleFromContext(r.Context())
	workspaces, err := s.Store.ListWorkspaceProjectCountsForUser(r.Context(), userID, role == "owner")
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, workspaces)
}

type createWorkspaceRequest struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
}

// createWorkspace is open to any authenticated user -- creating a new
// organizational grouping is low-stakes, and the creator becomes its admin
// in the same transaction (store.CreateWorkspace), so nobody else gets
// implicit access to what they made.
func (s *Server) createWorkspace(w http.ResponseWriter, r *http.Request) {
	var req createWorkspaceRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if req.Name == "" || req.Slug == "" {
		writeError(w, http.StatusBadRequest, "name and slug are required")
		return
	}
	userID, _ := auth.UserIDFromContext(r.Context())
	ws, err := s.Store.CreateWorkspace(r.Context(), req.Name, req.Slug, userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, ws)
}

// deleteWorkspace requires admin+ in the workspace being deleted (or a
// global owner) -- wired via auth.RequireWorkspaceRole in router.go.
func (s *Server) deleteWorkspace(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "workspaceID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid workspace id")
		return
	}
	if err := s.Store.DeleteWorkspace(r.Context(), id); err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	s.auditCtxWorkspace(r.Context(), "delete", "workspace", id, "")
	w.WriteHeader(http.StatusNoContent)
}

type setProjectWorkspaceRequest struct {
	WorkspaceID int64 `json:"workspace_id"`
}

// setProjectWorkspace moves a project to another workspace. Requires
// admin+ in *both* the project's current workspace (you're taking it out
// of a workspace you control) and the destination (you're bringing it into
// one you control) -- otherwise a workspace-admin could exfiltrate a
// project into a workspace they don't manage, or pull one in from a
// workspace they have no say over.
func (s *Server) setProjectWorkspace(w http.ResponseWriter, r *http.Request) {
	projectID, err := parseID(chi.URLParam(r, "projectID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return
	}
	var req setProjectWorkspaceRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if req.WorkspaceID < 1 {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}

	currentWorkspaceID, err := s.Store.WorkspaceIDForProject(r.Context(), projectID)
	if err == store.ErrNotFound {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	for _, workspaceID := range []int64{currentWorkspaceID, req.WorkspaceID} {
		ok, err := auth.HasWorkspaceRole(r.Context(), s.Store, "admin", workspaceID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if !ok {
			writeError(w, http.StatusForbidden, "admin role (in both the source and destination workspace) required")
			return
		}
	}

	if err := s.Store.SetProjectWorkspace(r.Context(), projectID, req.WorkspaceID); err != nil {
		if err == store.ErrNotFound {
			writeError(w, http.StatusNotFound, "project not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.audit(r.Context(), "move_project", "project", projectID, &req.WorkspaceID,
		fmt.Sprintf("from workspace %d to %d", currentWorkspaceID, req.WorkspaceID))
	w.WriteHeader(http.StatusNoContent)
}

// ---- Workspace membership ----
//
// All four routes below require admin+ in the workspace (or a global
// owner), wired via auth.RequireWorkspaceRole in router.go.

func (s *Server) listWorkspaceMembers(w http.ResponseWriter, r *http.Request) {
	workspaceID, err := parseID(chi.URLParam(r, "workspaceID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid workspace id")
		return
	}
	members, err := s.Store.ListWorkspaceMembers(r.Context(), workspaceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, members)
}

type addWorkspaceMemberRequest struct {
	Email string `json:"email"`
	Role  string `json:"role"`
}

// addWorkspaceMember grants an *existing* org account a role in this
// workspace, looked up by exact email match -- deliberately not a picker
// over every org account, since listing the full user roster is reserved
// for global owners (internal/admin.go's listUsers, docs/multi-user.md).
// This lets a workspace-admin who isn't a global owner add a known
// colleague without that broader visibility.
func (s *Server) addWorkspaceMember(w http.ResponseWriter, r *http.Request) {
	workspaceID, err := parseID(chi.URLParam(r, "workspaceID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid workspace id")
		return
	}
	var req addWorkspaceMemberRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if !isValidWorkspaceRole(req.Role) {
		writeError(w, http.StatusBadRequest, "role must be one of admin, editor, viewer")
		return
	}

	user, err := s.Store.GetUserByEmail(r.Context(), req.Email)
	if err == store.ErrNotFound {
		writeError(w, http.StatusNotFound, "no account with that email")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err := s.Store.AddWorkspaceMember(r.Context(), workspaceID, user.ID, req.Role); err != nil {
		if err == store.ErrDuplicate {
			writeError(w, http.StatusConflict, "user is already a member of this workspace")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.auditCtxWorkspace(r.Context(), "add_member", "workspace", workspaceID, req.Email+" as "+req.Role)
	w.WriteHeader(http.StatusCreated)
}

type setWorkspaceMemberRoleRequest struct {
	Role string `json:"role"`
}

func (s *Server) setWorkspaceMemberRole(w http.ResponseWriter, r *http.Request) {
	workspaceID, err := parseID(chi.URLParam(r, "workspaceID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid workspace id")
		return
	}
	userID, err := parseID(chi.URLParam(r, "userID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}
	var req setWorkspaceMemberRoleRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if !isValidWorkspaceRole(req.Role) {
		writeError(w, http.StatusBadRequest, "role must be one of admin, editor, viewer")
		return
	}
	if err := s.Store.SetWorkspaceMemberRole(r.Context(), workspaceID, userID, req.Role); err != nil {
		if err == store.ErrNotFound {
			writeError(w, http.StatusNotFound, "not a member of this workspace")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.auditCtxWorkspace(r.Context(), "set_member_role", "workspace", workspaceID, fmt.Sprintf("user %d -> %s", userID, req.Role))
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) removeWorkspaceMember(w http.ResponseWriter, r *http.Request) {
	workspaceID, err := parseID(chi.URLParam(r, "workspaceID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid workspace id")
		return
	}
	userID, err := parseID(chi.URLParam(r, "userID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}
	if err := s.Store.RemoveWorkspaceMember(r.Context(), workspaceID, userID); err != nil {
		if err == store.ErrNotFound {
			writeError(w, http.StatusNotFound, "not a member of this workspace")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.auditCtxWorkspace(r.Context(), "remove_member", "workspace", workspaceID, fmt.Sprintf("user %d", userID))
	w.WriteHeader(http.StatusNoContent)
}

func isValidWorkspaceRole(role string) bool {
	return role == "admin" || role == "editor" || role == "viewer"
}
