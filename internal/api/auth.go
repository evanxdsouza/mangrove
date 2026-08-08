package api

import (
	"net/http"
	"strings"

	"github.com/evanxdsouza/mangrove/internal/auth"
	"github.com/evanxdsouza/mangrove/internal/store"
)

// authStatus tells the SPA whether first-run admin creation is still
// needed. Deliberately unauthenticated -- there is no other way to bootstrap
// the very first login -- but reveals nothing beyond a boolean.
func (s *Server) authStatus(w http.ResponseWriter, r *http.Request) {
	n, err := s.Store.CountUsers(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"setup_required": n == 0})
}

type setupRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// authSetup creates the one and only admin account. It refuses once any
// user already exists -- this is a one-time bootstrap endpoint, not a
// general "create user" API (there is no multi-user story yet; see the
// org/workspace stubs in the schema for where that will eventually hang).
func (s *Server) authSetup(w http.ResponseWriter, r *http.Request) {
	n, err := s.Store.CountUsers(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if n > 0 {
		writeError(w, http.StatusConflict, "setup has already been completed")
		return
	}

	var req setupRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	if req.Email == "" || len(req.Password) < 8 {
		writeError(w, http.StatusBadRequest, "email is required and password must be at least 8 characters")
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "hash password: "+err.Error())
		return
	}
	userID, err := s.Store.CreateUser(r.Context(), req.Email, hash)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.startSession(w, r, userID)
	writeJSON(w, http.StatusCreated, map[string]any{"id": userID, "email": req.Email})
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (s *Server) authLogin(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))

	user, err := s.Store.GetUserByEmail(r.Context(), req.Email)
	if err == store.ErrNotFound {
		// Same response as a wrong password -- don't leak which part was wrong.
		writeError(w, http.StatusUnauthorized, "invalid email or password")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	ok, err := auth.VerifyPassword(req.Password, user.PasswordHash)
	if err != nil || !ok {
		writeError(w, http.StatusUnauthorized, "invalid email or password")
		return
	}

	s.Store.TouchUserLogin(r.Context(), user.ID)
	s.startSession(w, r, user.ID)
	writeJSON(w, http.StatusOK, map[string]any{"id": user.ID, "email": user.Email})
}

func (s *Server) authLogout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(auth.SessionCookieName); err == nil {
		auth.DeleteSession(r.Context(), s.Store, cookie.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     auth.SessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	w.WriteHeader(http.StatusNoContent)
}

type changePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

// authChangePassword requires the current password even though the
// request is already authenticated via session cookie -- a stolen
// session shouldn't be enough on its own to lock the real admin out by
// rotating the password.
func (s *Server) authChangePassword(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	var req changePasswordRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if len(req.NewPassword) < 8 {
		writeError(w, http.StatusBadRequest, "new_password must be at least 8 characters")
		return
	}

	user, err := s.Store.GetUserByID(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	ok, err = auth.VerifyPassword(req.CurrentPassword, user.PasswordHash)
	if err != nil || !ok {
		writeError(w, http.StatusUnauthorized, "current password is incorrect")
		return
	}

	hash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "hash password: "+err.Error())
		return
	}
	if err := s.Store.UpdateUserPassword(r.Context(), userID, hash); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) authMe(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": userID})
}

func (s *Server) startSession(w http.ResponseWriter, r *http.Request, userID int64) {
	token, err := auth.CreateSession(r.Context(), s.Store, userID, r.RemoteAddr, r.UserAgent())
	if err != nil {
		s.Log.Error("failed to create session", "error", err)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     auth.SessionCookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   int(auth.SessionTTL.Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		// Secure is intentionally not forced here: Mangrove sits behind
		// Caddy on localhost in the common deployment, and hard-requiring
		// HTTPS on the cookie would break the loopback-only Phase 1
		// listener. Caddy terminates TLS for real deployments (see task 9).
	})
}
