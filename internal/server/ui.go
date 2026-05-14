package server

import (
	"io/fs"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"gos3/web"
)

// SetupUIRouter initializes the HTTP router for the Web Console.
func SetupUIRouter() *chi.Mux {
	r := chi.NewRouter()

	// Extract the "dist" sub-filesystem from the embedded FS
	subFS, err := fs.Sub(web.DistFS, "dist")
	if err != nil {
		slog.Error("failed to create sub filesystem for web UI", "error", err)
		return r // return empty router on error, though this should never happen with valid embed
	}

	// Serve the static files natively without path rewriting hacks
	fileServer := http.FileServer(http.FS(subFS))

	// Mount the fileserver at root
	r.Handle("/*", fileServer)

	return r
}
