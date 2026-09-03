package webui

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestIndexNeverCached guards against exactly the bug that let a stale
// dashboard survive a redeploy indefinitely: index.html's *content*
// changes on every deploy while its URL never does, and embed.FS reports
// no ModTime (so http.FileServer sends no Last-Modified/ETag either) --
// without an explicit Cache-Control, whether a browser decides to cache it
// anyway is down to its own heuristics, not something this test should
// leave to chance.
func TestIndexNeverCached(t *testing.T) {
	h := Handler()

	for _, path := range []string{"/", "/projects/3"} { // "/projects/3" exercises the SPA-fallback path too
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if got := rec.Header().Get("Cache-Control"); got != "no-store" {
			t.Errorf("GET %s: Cache-Control = %q, want %q", path, got, "no-store")
		}
	}
}

// TestHashedAssetsCachedForever checks the other half of the same policy:
// vite fingerprints every file under assets/ with a content hash in its
// name, so unlike index.html it's correct -- not just permissible -- to
// cache those forever, a rebuild produces a new filename rather than a
// changed one under the same name. Skips if the frontend hasn't been
// built (dist/ ships only a placeholder index.html in git -- see
// embed.go's doc comment), rather than failing: this package must still
// build and pass tests before `npm run build` has ever run.
func TestHashedAssetsCachedForever(t *testing.T) {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		t.Fatalf("fs.Sub: %v", err)
	}
	entries, err := fs.ReadDir(sub, "assets")
	if err != nil || len(entries) == 0 {
		t.Skip("no built assets/ present (frontend not built) -- run `cd web && npm run build` to exercise this test")
	}

	h := Handler()
	req := httptest.NewRequest(http.MethodGet, "/assets/"+entries[0].Name(), nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /assets/%s: status %d", entries[0].Name(), rec.Code)
	}
	want := "public, max-age=31536000, immutable"
	if got := rec.Header().Get("Cache-Control"); got != want {
		t.Errorf("Cache-Control = %q, want %q", got, want)
	}
}
