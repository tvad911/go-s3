package server

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"gos3/web"
)

// SetupUIRouter initializes the HTTP router for the Web Console.
func SetupUIRouter() *chi.Mux {
	r := chi.NewRouter()

	// Serve embedded files
	// Extract the "dist" sub-filesystem from the embedded FS
	// Go 1.16+ embed
	// Note: We need a file server for the root directory of the `web/dist` folder.
	// Since embed.FS includes the path `dist/*`, we must strip prefix or serve properly.

	fs := http.FileServer(http.FS(web.DistFS))

	r.Get("/*", func(w http.ResponseWriter, req *http.Request) {
		// Clean up the path
		path := req.URL.Path
		if path == "/" {
			path = "/index.html"
		}
		
		// Map the URL path to the embedded path
		embedPath := "dist" + path

		// Check if file exists in embed.FS
		f, err := web.DistFS.Open(embedPath)
		if err != nil {
			// Try to fallback to index.html for SPA routing if needed
			// But for a simple console, returning 404 is fine if not found
			f, err = web.DistFS.Open("dist/index.html")
			if err != nil {
				http.NotFound(w, req)
				return
			}
			embedPath = "dist/index.html"
		}
		f.Close()

		req.URL.Path = embedPath
		
		// For proper MIME types on CSS/JS
		if strings.HasSuffix(path, ".css") {
			w.Header().Set("Content-Type", "text/css")
		} else if strings.HasSuffix(path, ".js") {
			w.Header().Set("Content-Type", "application/javascript")
		}

		fs.ServeHTTP(w, req)
	})

	return r
}
