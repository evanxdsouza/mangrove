package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/evanxdsouza/mangrove/internal/auth"
	"github.com/evanxdsouza/mangrove/internal/store"
)

// listProjects has no single resource ID to hang auth.RequireWorkspaceRole
// off (it can list across every workspace at once), so it checks role
// inline: with a workspace_id filter, viewer+ in that one workspace; with
// none, every project across every workspace the caller has any role in
// (a global owner still sees everything, unfiltered).
func (s *Server) listProjects(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserIDFromContext(r.Context())
	role, _ := auth.RoleFromContext(r.Context())
	isOwner := role == "owner"

	workspaceIDParam := r.URL.Query().Get("workspace_id")
	var projects []store.ProjectWithWorkspace
	var err error
	if workspaceIDParam != "" {
		var id int64
		if id, err = parseID(workspaceIDParam); err != nil {
			writeError(w, http.StatusBadRequest, "invalid workspace_id")
			return
		}
		ok, rerr := auth.HasWorkspaceRole(r.Context(), s.Store, "viewer", id)
		if rerr != nil {
			writeError(w, http.StatusInternalServerError, rerr.Error())
			return
		}
		if !ok {
			writeError(w, http.StatusForbidden, "viewer role (in this workspace) required")
			return
		}
		projects, err = s.Store.ListProjectsByWorkspace(r.Context(), id)
	} else {
		projects, err = s.Store.ListProjectsForUser(r.Context(), userID, isOwner)
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, projects)
}

type createProjectRequest struct {
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Description string `json:"description"`
	// WorkspaceID is which workspace the project belongs to; 0/omitted
	// means the default workspace (1).
	WorkspaceID int64 `json:"workspace_id"`
}

func (s *Server) createProject(w http.ResponseWriter, r *http.Request) {
	var req createProjectRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if req.Name == "" || req.Slug == "" {
		writeError(w, http.StatusBadRequest, "name and slug are required")
		return
	}
	workspaceID := req.WorkspaceID
	if workspaceID < 1 {
		workspaceID = 1
	}

	ok, err := auth.HasWorkspaceRole(r.Context(), s.Store, "editor", workspaceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !ok {
		writeError(w, http.StatusForbidden, "editor role (in this workspace) required")
		return
	}

	p, err := s.Store.CreateProject(r.Context(), workspaceID, req.Name, req.Slug, req.Description)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, p)
}

func (s *Server) getProject(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "projectID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return
	}
	p, err := s.Store.GetProject(r.Context(), id)
	if err == store.ErrNotFound {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (s *Server) deleteProject(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "projectID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return
	}
	if err := s.Orchestrator.DeleteProject(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
