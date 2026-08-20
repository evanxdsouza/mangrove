package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/evanxdsouza/mangrove/internal/auth"
	mangrovedb "github.com/evanxdsouza/mangrove/internal/db"
	"github.com/evanxdsouza/mangrove/internal/store"
)

// roleTestEnv wires a real router against a real SQLite store -- no Docker
// executor needed, since these tests only exercise the RequireOwner
// middleware and the user-management handlers, none of which touch
// s.Orchestrator (a request rejected by RequireOwner never reaches a
// handler that would).
type roleTestEnv struct {
	router http.Handler
	store  *store.Store
}

func newRoleTestEnv(t *testing.T) *roleTestEnv {
	t.Helper()
	dir := t.TempDir()
	db, err := mangrovedb.Open(filepath.Join(dir, "mangrove.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	st := store.New(db)

	s := &Server{Store: st, Log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	return &roleTestEnv{router: s.Router(), store: st}
}

// cookieFor creates a user with the given role and returns a session
// cookie for them, for use in requests against env.router.
func (env *roleTestEnv) cookieFor(t *testing.T, email, role string) (*http.Cookie, int64) {
	t.Helper()
	hash, err := auth.HashPassword("password123")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	userID, err := env.store.CreateUser(context.Background(), email, hash, role)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	token, err := auth.CreateSession(context.Background(), env.store, userID, "127.0.0.1", "test-agent")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	return &http.Cookie{Name: auth.SessionCookieName, Value: token}, userID
}

func (env *roleTestEnv) do(method, path string, cookie *http.Cookie, body any) *httptest.ResponseRecorder {
	var reader io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		reader = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	env.router.ServeHTTP(rec, req)
	return rec
}

func TestMemberForbiddenFromOwnerOnlyRoutes(t *testing.T) {
	env := newRoleTestEnv(t)
	memberCookie, _ := env.cookieFor(t, "member@example.com", "member")

	cases := []struct {
		name   string
		method string
		path   string
		body   any
	}{
		{"delete project", http.MethodDelete, "/api/projects/1", nil},
		{"delete deployment", http.MethodDelete, "/api/deployments/1", nil},
		{"set access control", http.MethodPost, "/api/deployments/1/access", map[string]any{"is_public": true}},
		{"update build config", http.MethodPost, "/api/deployments/1/build-config", map[string]any{"build_strategy": "static"}},
		{"list users", http.MethodGet, "/api/admin/users", nil},
		{"create user", http.MethodPost, "/api/admin/users", map[string]any{"email": "x@example.com", "password": "password123", "role": "member"}},
		{"delete user", http.MethodDelete, "/api/admin/users/999", nil},
		{"set secret env var", http.MethodPut, "/api/services/1/env/API_KEY", map[string]any{"value": "x", "is_secret": true}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec := env.do(c.method, c.path, memberCookie, c.body)
			if rec.Code != http.StatusForbidden {
				t.Errorf("expected 403 for member on %s %s, got %d: %s", c.method, c.path, rec.Code, rec.Body.String())
			}
		})
	}
}

func TestOwnerCanManageUsers(t *testing.T) {
	env := newRoleTestEnv(t)
	ownerCookie, _ := env.cookieFor(t, "owner@example.com", "owner")

	rec := env.do(http.MethodGet, "/api/admin/users", ownerCookie, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 listing users as owner, got %d: %s", rec.Code, rec.Body.String())
	}
	var users []store.UserWithRole
	if err := json.Unmarshal(rec.Body.Bytes(), &users); err != nil {
		t.Fatalf("decode users: %v", err)
	}
	if len(users) != 1 || users[0].Role != "owner" {
		t.Fatalf("expected exactly the seed owner, got %+v", users)
	}

	rec = env.do(http.MethodPost, "/api/admin/users", ownerCookie, map[string]any{
		"email": "teammate@example.com", "password": "password123", "role": "member",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 creating user as owner, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestDeleteUserRejectsSelfDelete(t *testing.T) {
	env := newRoleTestEnv(t)
	ownerCookie, ownerID := env.cookieFor(t, "owner@example.com", "owner")

	rec := env.do(http.MethodDelete, "/api/admin/users/"+itoa(ownerID), ownerCookie, nil)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 deleting your own account, got %d: %s", rec.Code, rec.Body.String())
	}

	_, memberID := env.cookieFor(t, "member@example.com", "member")
	rec = env.do(http.MethodDelete, "/api/admin/users/"+itoa(memberID), ownerCookie, nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204 deleting a member, got %d: %s", rec.Code, rec.Body.String())
	}
}

func itoa(id int64) string {
	b, _ := json.Marshal(id)
	return string(b)
}
