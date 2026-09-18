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

// TestMemberForbiddenFromOwnerOnlyRoutes covers the routes that are
// global-owner-only regardless of workspace role -- host-level operations
// (user accounts, sessions, ports, pruning) with no per-workspace
// dimension at all. Delete project/deployment, access control, and secret
// env vars used to be in this list too, but since the workspace-roles pass
// they're workspace-admin-scoped, not global-owner-only -- see
// TestWorkspaceRoleThresholds and TestWorkspaceAdminScopedNotGlobal in
// workspace_roles_test.go for those.
func TestMemberForbiddenFromOwnerOnlyRoutes(t *testing.T) {
	env := newRoleTestEnv(t)
	memberCookie, _ := env.cookieFor(t, "member@example.com", "member")

	cases := []struct {
		name   string
		method string
		path   string
		body   any
	}{
		{"list users", http.MethodGet, "/api/admin/users", nil},
		{"create user", http.MethodPost, "/api/admin/users", map[string]any{"email": "x@example.com", "password": "password123", "role": "member"}},
		{"delete user", http.MethodDelete, "/api/admin/users/999", nil},
		{"list sessions", http.MethodGet, "/api/admin/sessions", nil},
		{"revoke session", http.MethodDelete, "/api/admin/sessions/999", nil},
		{"list ports", http.MethodGet, "/api/admin/ports", nil},
		{"reserve port", http.MethodPost, "/api/admin/ports", map[string]any{"port": 12345, "note": "test"}},
		{"release port", http.MethodDelete, "/api/admin/ports/12345", nil},
		{"trigger prune", http.MethodPost, "/api/admin/prune", nil},
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

func TestOwnerCanManageSessionsAndPorts(t *testing.T) {
	env := newRoleTestEnv(t)
	ownerCookie, _ := env.cookieFor(t, "owner@example.com", "owner")

	rec := env.do(http.MethodGet, "/api/admin/sessions", ownerCookie, nil)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 listing sessions as owner, got %d: %s", rec.Code, rec.Body.String())
	}

	rec = env.do(http.MethodGet, "/api/admin/ports", ownerCookie, nil)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 listing ports as owner, got %d: %s", rec.Code, rec.Body.String())
	}

	rec = env.do(http.MethodPost, "/api/admin/ports", ownerCookie, map[string]any{"port": 23456, "note": "test"})
	if rec.Code != http.StatusNoContent {
		t.Errorf("expected 204 reserving a port as owner, got %d: %s", rec.Code, rec.Body.String())
	}

	rec = env.do(http.MethodDelete, "/api/admin/ports/23456", ownerCookie, nil)
	if rec.Code != http.StatusNoContent {
		t.Errorf("expected 204 releasing a port as owner, got %d: %s", rec.Code, rec.Body.String())
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
