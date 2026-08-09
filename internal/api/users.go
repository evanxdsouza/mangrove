package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/evanxdsouza/mangrove/internal/auth"
)

// listUsers backs the admin Team panel. Owner-only, enforced by the
// RequireOwner middleware wrapping this route in router.go.
func (s *Server) listUsers(w http.ResponseWriter, r *http.Request) {
	users, err := s.Store.ListUsers(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, users)
}

type createTeamUserRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

// createTeamUser is the owner's way of adding another account. There's no
// outbound email in this app, so "inviting" someone means the owner sets
// their initial email+password directly here and shares it out of band --
// same trust model authSetup already uses for the very first account.
func (s *Server) createTeamUser(w http.ResponseWriter, r *http.Request) {
	var req createTeamUserRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if req.Email == "" || len(req.Password) < 8 {
		writeError(w, http.StatusBadRequest, "email is required and password must be at least 8 characters")
		return
	}
	if req.Role != "owner" && req.Role != "member" {
		writeError(w, http.StatusBadRequest, "role must be 'owner' or 'member'")
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "hash password: "+err.Error())
		return
	}
	id, err := s.Store.CreateUser(r.Context(), req.Email, hash, req.Role)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": id, "email": req.Email, "role": req.Role})
}

// deleteTeamUser refuses two cases that would otherwise strand the
// system: removing yourself (use another owner's account, not a
// self-service path with no undo) and removing the last remaining owner
// (there's no recovery flow if that leaves zero owners).
func (s *Server) deleteTeamUser(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "userID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}
	if callerID, ok := auth.UserIDFromContext(r.Context()); ok && callerID == id {
		writeError(w, http.StatusBadRequest, "cannot delete your own account")
		return
	}

	target, err := s.Store.GetUserByID(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	if target.Role == "owner" {
		n, err := s.Store.CountOwners(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if n <= 1 {
			writeError(w, http.StatusBadRequest, "cannot delete the last remaining owner")
			return
		}
	}

	if err := s.Store.DeleteUser(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
