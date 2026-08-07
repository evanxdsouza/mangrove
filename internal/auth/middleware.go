package auth

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/evanxdsouza/mangrove/internal/store"
)

type contextKey string

const userIDContextKey contextKey = "mangrove_user_id"

// RequireAuth rejects any request without a valid session cookie. There is
// no unauthenticated path through this middleware -- callers that need a
// public route (the webhook receiver, health check) must be mounted
// outside the group it wraps, not special-cased inside it.
func RequireAuth(st *store.Store) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(SessionCookieName)
			if err != nil {
				unauthorized(w)
				return
			}
			userID, err := ValidateSession(r.Context(), st, cookie.Value)
			if err != nil {
				unauthorized(w)
				return
			}
			ctx := context.WithValue(r.Context(), userIDContextKey, userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func unauthorized(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	json.NewEncoder(w).Encode(map[string]string{"error": "authentication required"})
}

// UserIDFromContext returns the authenticated user's ID, set by RequireAuth.
func UserIDFromContext(ctx context.Context) (int64, bool) {
	id, ok := ctx.Value(userIDContextKey).(int64)
	return id, ok
}
