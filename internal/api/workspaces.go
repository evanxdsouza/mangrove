package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/evanxdsouza/mangrove/internal/store"
)

func (s *Server) listWorkspaces(w http.ResponseWriter, r *http.Request) {
	workspaces, err := s.Store.ListWorkspaceProjectCounts(r.Context())
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
	ws, err := s.Store.CreateWorkspace(r.Context(), req.Name, req.Slug)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, ws)
}

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
	w.WriteHeader(http.StatusNoContent)
}

type setProjectWorkspaceRequest struct {
	WorkspaceID int64 `json:"workspace_id"`
}

// setProjectWorkspace moves a project to another workspace.
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
	if err := s.Store.SetProjectWorkspace(r.Context(), projectID, req.WorkspaceID); err != nil {
		if err == store.ErrNotFound {
			writeError(w, http.StatusNotFound, "project not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
