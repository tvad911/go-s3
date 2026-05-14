package server

import (
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/go-chi/chi/v5"
	"gos3/internal/config"
	"gos3/web"
)

// SetupUIRouter initializes the HTTP router for the Web Console.
// It proxies /api/v1/* and /_admin/* requests to the main API server.
func SetupUIRouter(cfg *config.Config) *chi.Mux {
	r := chi.NewRouter()

	// Reverse proxy to main API server for /api/v1/* and /_admin/* routes
	apiTarget, err := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", cfg.Server.Port))
	if err != nil {
		slog.Error("failed to parse API target URL", "error", err)
		return r
	}
	proxy := httputil.NewSingleHostReverseProxy(apiTarget)

	r.Handle("/api/v1/*", proxy)
	r.Handle("/_admin/*", proxy)
	r.Handle("/_health", proxy)

	// Extract the "dist" sub-filesystem from the embedded FS
	subFS, err := fs.Sub(web.DistFS, "dist")
	if err != nil {
		slog.Error("failed to create sub filesystem for web UI", "error", err)
		return r
	}

	// Serve static files or fallback to API proxy
	fileServer := http.FileServer(http.FS(subFS))
	r.Handle("/*", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "."
		}
		
		// If file exists in static dir, serve it
		_, err := fs.Stat(subFS, path)
		if err == nil {
			fileServer.ServeHTTP(w, r)
			return
		}
		
		// Otherwise, proxy to main API (e.g. for Presigned URLs and S3 operations)
		proxy.ServeHTTP(w, r)
	}))

	return r
}
