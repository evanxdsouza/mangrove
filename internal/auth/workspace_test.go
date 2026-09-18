package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/evanxdsouza/mangrove/internal/store"
)

func ctxWithUser(userID int64, role string) context.Context {
	ctx := context.WithValue(context.Background(), userIDContextKey, userID)
	ctx = context.WithValue(ctx, roleContextKey, role)
	return ctx
}

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
}

func TestRequireWorkspaceRoleThresholds(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()

	ownerHash, _ := HashPassword("password123")
	ownerID, err := st.CreateUser(ctx, "owner@example.com", ownerHash, "owner")
	if err != nil {
		t.Fatalf("CreateUser(owner): %v", err)
	}
	memberID, err := st.CreateUser(ctx, "member@example.com", ownerHash, "member")
	if err != nil {
		t.Fatalf("CreateUser(member): %v", err)
	}

	ws, err := st.CreateWorkspace(ctx, "Test", "test", ownerID)
	if err != nil {
		t.Fatalf("CreateWorkspace: %v", err)
	}
	resolve := func(r *http.Request) (int64, error) { return ws.ID, nil }

	cases := []struct {
		name       string
		membership string // "" = no workspace_members row at all
		minRole    string
		wantCode   int
	}{
		{"no membership blocked at viewer", "", "viewer", http.StatusForbidden},
		{"viewer passes viewer", "viewer", "viewer", http.StatusOK},
		{"viewer blocked at editor", "viewer", "editor", http.StatusForbidden},
		{"viewer blocked at admin", "viewer", "admin", http.StatusForbidden},
		{"editor passes viewer", "editor", "viewer", http.StatusOK},
		{"editor passes editor", "editor", "editor", http.StatusOK},
		{"editor blocked at admin", "editor", "admin", http.StatusForbidden},
		{"admin passes viewer", "admin", "viewer", http.StatusOK},
		{"admin passes editor", "admin", "editor", http.StatusOK},
		{"admin passes admin", "admin", "admin", http.StatusOK},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_ = st.RemoveWorkspaceMember(ctx, ws.ID, memberID)
			if c.membership != "" {
				if err := st.AddWorkspaceMember(ctx, ws.ID, memberID, c.membership); err != nil {
					t.Fatalf("AddWorkspaceMember: %v", err)
				}
			}

			h := RequireWorkspaceRole(st, c.minRole, resolve)(okHandler())
			req := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctxWithUser(memberID, "member"))
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != c.wantCode {
				t.Errorf("membership=%q minRole=%q: expected %d, got %d", c.membership, c.minRole, c.wantCode, rec.Code)
			}
		})
	}
}

func TestRequireWorkspaceRoleOwnerBypasses(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()

	ownerHash, _ := HashPassword("password123")
	ownerID, err := st.CreateUser(ctx, "owner@example.com", ownerHash, "owner")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	// A workspace this owner has no workspace_members row in at all -- the
	// point of the bypass is that it doesn't matter.
	otherOwnerID, err := st.CreateUser(ctx, "other-owner@example.com", ownerHash, "owner")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	otherWS, err := st.CreateWorkspace(ctx, "Other", "other", otherOwnerID)
	if err != nil {
		t.Fatalf("CreateWorkspace: %v", err)
	}

	resolve := func(r *http.Request) (int64, error) { return otherWS.ID, nil }
	h := RequireWorkspaceRole(st, "admin", resolve)(okHandler())
	req := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctxWithUser(ownerID, "owner"))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected a global owner to bypass the workspace role check entirely, got %d", rec.Code)
	}
}

func TestRequireWorkspaceRoleResolveErrors(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	hash, _ := HashPassword("password123")
	memberID, err := st.CreateUser(ctx, "member@example.com", hash, "member")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	t.Run("ErrNotFound is 404", func(t *testing.T) {
		resolve := func(r *http.Request) (int64, error) { return 0, store.ErrNotFound }
		h := RequireWorkspaceRole(st, "viewer", resolve)(okHandler())
		req := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctxWithUser(memberID, "member"))
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Errorf("expected 404, got %d", rec.Code)
		}
	})

	t.Run("other resolve error is 500", func(t *testing.T) {
		resolve := func(r *http.Request) (int64, error) { return 0, context.DeadlineExceeded }
		h := RequireWorkspaceRole(st, "viewer", resolve)(okHandler())
		req := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctxWithUser(memberID, "member"))
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusInternalServerError {
			t.Errorf("expected 500, got %d", rec.Code)
		}
	})
}
