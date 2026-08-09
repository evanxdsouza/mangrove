package auth

import (
	"context"
	"path/filepath"
	"testing"

	mangrovedb "github.com/evanxdsouza/mangrove/internal/db"
	"github.com/evanxdsouza/mangrove/internal/store"
)

func testStore(t *testing.T) *store.Store {
	t.Helper()
	dir := t.TempDir()
	db, err := mangrovedb.Open(filepath.Join(dir, "mangrove.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return store.New(db)
}

func TestCreateAndValidateSession(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()

	hash, _ := HashPassword("password123")
	userID, err := st.CreateUser(ctx, "evan@example.com", hash, "owner")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	token, err := CreateSession(ctx, st, userID, "127.0.0.1", "test-agent")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty session token")
	}

	gotUserID, gotRole, err := ValidateSession(ctx, st, token)
	if err != nil {
		t.Fatalf("ValidateSession: %v", err)
	}
	if gotUserID != userID {
		t.Errorf("got user %d, want %d", gotUserID, userID)
	}
	if gotRole != "owner" {
		t.Errorf("got role %q, want \"owner\"", gotRole)
	}
}

func TestValidateSessionRejectsUnknownToken(t *testing.T) {
	st := testStore(t)
	if _, _, err := ValidateSession(context.Background(), st, "not-a-real-token"); err == nil {
		t.Error("expected an error for an unknown session token")
	}
}

func TestDeleteSessionInvalidatesToken(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()

	hash, _ := HashPassword("password123")
	userID, _ := st.CreateUser(ctx, "evan@example.com", hash, "owner")
	token, err := CreateSession(ctx, st, userID, "127.0.0.1", "test-agent")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	if err := DeleteSession(ctx, st, token); err != nil {
		t.Fatalf("DeleteSession: %v", err)
	}
	if _, _, err := ValidateSession(ctx, st, token); err == nil {
		t.Error("expected ValidateSession to fail after DeleteSession")
	}
}

func TestSessionTokensAreUnpredictable(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 100; i++ {
		tok, err := NewSessionToken()
		if err != nil {
			t.Fatalf("NewSessionToken: %v", err)
		}
		if seen[tok] {
			t.Fatalf("duplicate token generated: %s", tok)
		}
		seen[tok] = true
	}
}
