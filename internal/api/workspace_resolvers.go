package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/evanxdsouza/mangrove/internal/store"
)

// The resolve* functions below are the resource->workspace lookups
// auth.RequireWorkspaceRole needs per route. A malformed URL param is
// folded into store.ErrNotFound (404) rather than surfaced as a distinct
// 400 -- a badly-formed ID can never correspond to a real resource either,
// so there's no meaningful difference to the caller.

func (s *Server) resolveProjectWorkspace(r *http.Request) (int64, error) {
	id, err := parseID(chi.URLParam(r, "projectID"))
	if err != nil {
		return 0, store.ErrNotFound
	}
	return s.Store.WorkspaceIDForProject(r.Context(), id)
}

func (s *Server) resolveDeploymentWorkspace(r *http.Request) (int64, error) {
	id, err := parseID(chi.URLParam(r, "deploymentID"))
	if err != nil {
		return 0, store.ErrNotFound
	}
	return s.Store.WorkspaceIDForDeployment(r.Context(), id)
}

func (s *Server) resolveServiceWorkspace(r *http.Request) (int64, error) {
	id, err := parseID(chi.URLParam(r, "serviceID"))
	if err != nil {
		return 0, store.ErrNotFound
	}
	return s.Store.WorkspaceIDForService(r.Context(), id)
}

func (s *Server) resolveDomainWorkspace(r *http.Request) (int64, error) {
	id, err := parseID(chi.URLParam(r, "domainID"))
	if err != nil {
		return 0, store.ErrNotFound
	}
	return s.Store.WorkspaceIDForCustomDomain(r.Context(), id)
}

func (s *Server) resolveDeployHistoryWorkspace(r *http.Request) (int64, error) {
	id, err := parseID(chi.URLParam(r, "historyID"))
	if err != nil {
		return 0, store.ErrNotFound
	}
	return s.Store.WorkspaceIDForDeployHistory(r.Context(), id)
}

// resolveWorkspaceIDParam resolves the {workspaceID} URL param itself, for
// routes that act directly on a workspace (delete, membership management)
// rather than on some other resource that belongs to one.
func (s *Server) resolveWorkspaceIDParam(r *http.Request) (int64, error) {
	id, err := parseID(chi.URLParam(r, "workspaceID"))
	if err != nil {
		return 0, store.ErrNotFound
	}
	if _, err := s.Store.GetWorkspace(r.Context(), id); err != nil {
		return 0, err
	}
	return id, nil
}
