package api

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	mangrovedb "github.com/evanxdsouza/mangrove/internal/db"
	"github.com/evanxdsouza/mangrove/internal/store"
)

// backupTestEnv is like roleTestEnv but also wires a real data dir with a
// master.key file on disk, since the backup handler reads both straight off
// the filesystem (see backup.go).
func newBackupTestEnv(t *testing.T) *roleTestEnv {
	t.Helper()
	dir := t.TempDir()
	db, err := mangrovedb.Open(filepath.Join(dir, "mangrove.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	st := store.New(db)

	if err := os.WriteFile(filepath.Join(dir, "master.key"), []byte("0123456789abcdef0123456789abcdef"), 0o600); err != nil {
		t.Fatalf("write master key: %v", err)
	}

	s := &Server{Store: st, Log: slog.New(slog.NewTextHandler(io.Discard, nil)), DataDir: dir}
	return &roleTestEnv{router: s.Router(), store: st}
}

func TestBackupRequiresOwner(t *testing.T) {
	env := newBackupTestEnv(t)
	memberCookie, _ := env.cookieFor(t, "member@example.com", "member")

	rec := env.do(http.MethodGet, "/api/admin/backup", memberCookie, nil)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for member on backup, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestBackupProducesValidTarGz(t *testing.T) {
	env := newBackupTestEnv(t)
	ownerCookie, _ := env.cookieFor(t, "owner@example.com", "owner")

	rec := env.do(http.MethodGet, "/api/admin/backup", ownerCookie, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 backing up as owner, got %d: %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/gzip" {
		t.Errorf("expected Content-Type application/gzip, got %q", ct)
	}
	if cd := rec.Header().Get("Content-Disposition"); cd == "" {
		t.Errorf("expected a Content-Disposition header, got none")
	}

	gz, err := gzip.NewReader(rec.Body)
	if err != nil {
		t.Fatalf("response body is not valid gzip: %v", err)
	}
	tr := tar.NewReader(gz)

	found := map[string]int64{}
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("reading tar: %v", err)
		}
		found[hdr.Name] = hdr.Size
	}

	for _, name := range []string{"mangrove.db", "master.key", "BACKUP_INFO.txt"} {
		if _, ok := found[name]; !ok {
			t.Errorf("expected %s in backup archive, got entries: %v", name, found)
		}
	}
	if found["master.key"] != int64(len("0123456789abcdef0123456789abcdef")) {
		t.Errorf("master.key size mismatch: got %d", found["master.key"])
	}
	if found["mangrove.db"] == 0 {
		t.Errorf("expected a non-empty mangrove.db snapshot")
	}
}
