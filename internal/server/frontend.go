package server

import (
	"io/fs"
	"net/http"
	"strings"
)

// WebDistFS holds the embedded frontend build output.
// When nil, FrontendHandler returns a placeholder response.
var WebDistFS fs.FS

// FrontendHandler returns an http.Handler that serves the embedded SPA.
// It serves static files from WebDistFS and falls back to index.html for
// SPA routing (any path that doesn't match a file).
func FrontendHandler() http.Handler {
	if WebDistFS == nil {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("Frontend not built. Run: cd web && bun run build"))
		})
	}

	fileServer := http.FileServer(http.FS(WebDistFS))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		// Try to serve the file directly
		if path != "/" && !strings.HasSuffix(path, "/") {
			if f, err := WebDistFS.Open(strings.TrimPrefix(path, "/")); err == nil {
				f.Close()
				fileServer.ServeHTTP(w, r)
				return
			}
		}

		// SPA fallback: serve index.html
		indexBytes, err := fs.ReadFile(WebDistFS, "index.html")
		if err != nil {
			http.Error(w, "index.html not found", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(indexBytes)
	})
}
