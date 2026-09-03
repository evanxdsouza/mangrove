// Package webui embeds the built dashboard SPA (web/dist, copied here by
// `npm run build` -- see web/vite.config.ts's outDir) into the Go binary,
// so Mangrove ships as one file with no separate static-asset directory to
// deploy alongside it.
//
// dist/ ships a placeholder index.html in git (everything else in it is
// gitignored) so `go build` always succeeds even before the frontend has
// ever been built -- see .gitignore.
package webui

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed all:dist
var distFS embed.FS

// Handler serves the SPA: real files by path, falling back to index.html
// for any path that doesn't match a built asset so client-side routing
// (internal/web/src/router.tsx) works on a hard refresh of a deep link
// like /projects/3.
func Handler() http.Handler {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		panic("webui: dist subdirectory missing from embed: " + err.Error())
	}
	fileServer := http.FileServer(http.FS(sub))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "."
		}
		if info, err := fs.Stat(sub, path); err != nil || info.IsDir() {
			r.URL.Path = "/"
			path = "index.html"
		}
		// Vite's build fingerprints every file under assets/ with a content
		// hash in its name (index-<hash>.js), so it's safe -- correct,
		// even -- to cache those forever: a rebuild produces a new
		// filename, never a changed one under the same name. index.html
		// (and the SPA-fallback path above, which serves it too) is the
		// opposite: it's the one thing whose *content* changes on every
		// deploy while its URL never does, so it must never be cached at
		// all -- an already-open tab or a browser that decided to cache it
		// anyway (embed.FS reports no ModTime, so http.FileServer sends no
		// Last-Modified/ETag either, which without an explicit
		// Cache-Control here leaves a browser's own heuristics as the only
		// thing deciding whether "no validator" means "cache away" or
		// "always refetch" -- not something to leave to chance) would keep
		// serving a stale dashboard indefinitely after an update, since a
		// SPA has no way to notice its own JS bundle changed at runtime.
		if strings.HasPrefix(path, "assets/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			w.Header().Set("Cache-Control", "no-store")
		}
		fileServer.ServeHTTP(w, r)
	})
}
