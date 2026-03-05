package main

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed all:static
var staticFS embed.FS

// newStaticHandler returns an http.Handler that serves the embedded frontend
// SPA from the static/ directory. If the directory is empty (dev mode, no
// frontend build copied in) it returns nil so the caller can skip registration.
func newStaticHandler() http.Handler {
	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		return nil
	}

	// Only serve the SPA when a real frontend build is present.
	if _, err := fs.Stat(sub, "index.html"); err != nil {
		return nil
	}

	fileServer := http.FileServer(http.FS(sub))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// If the path maps to an existing file, serve it directly.
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}
		if _, err := fs.Stat(sub, path); err == nil {
			fileServer.ServeHTTP(w, r)
			return
		}
		// SPA fallback: serve index.html for any non-file path.
		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	})
}

