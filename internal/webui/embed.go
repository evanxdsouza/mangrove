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
		}
		fileServer.ServeHTTP(w, r)
	})
}
