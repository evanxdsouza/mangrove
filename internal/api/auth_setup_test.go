package api

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	mangrovedb "github.com/evanxdsouza/mangrove/internal/db"
	"github.com/evanxdsouza/mangrove/internal/store"
)

// TestAuthSetupConcurrentCreatesOneOwner fires several first-run setup
// requests at once against an empty DB. Before setup's insert re-checked
// atomically, every one of them passed the count check and a fresh install
// ended up with several owners.
func TestAuthSetupConcurrentCreatesOneOwner(t *testing.T) {
	db, err := mangrovedb.Open(filepath.Join(t.TempDir(), "mangrove.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	st := store.New(db)
	s := &Server{Store: st, Log: slog.New(slog.NewTextHandler(io.Discard, nil))}

	const n = 8
	codes := make([]int, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			body := `{"email":"owner` + string(rune('a'+i)) + `@example.com","password":"correct horse battery"}`
			req := httptest.NewRequest(http.MethodPost, "/api/auth/setup", strings.NewReader(body))
			rec := httptest.NewRecorder()
			s.authSetup(rec, req)
			codes[i] = rec.Code
		}(i)
	}
	wg.Wait()

	created := 0
	for _, c := range codes {
		switch c {
		case http.StatusCreated:
			created++
		case http.StatusConflict:
		default:
			t.Errorf("unexpected status %d", c)
		}
	}
	if created != 1 {
		t.Fatalf("expected exactly 1 setup to succeed, got %d (codes %v)", created, codes)
	}
	users, err := st.CountUsers(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if users != 1 {
		t.Fatalf("expected 1 user, got %d", users)
	}
}
