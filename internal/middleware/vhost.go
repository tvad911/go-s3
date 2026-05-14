package middleware

import (
	"context"
	"net/http"
	"strings"

	"gos3/internal/storage/metadata"
)

// CustomDomain rewrites the request path if the Host matches a custom domain.
func CustomDomain(metaStore metadata.Store) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			host := r.Host
			// Remove port if present
			if colonIdx := strings.LastIndex(host, ":"); colonIdx != -1 {
				host = host[:colonIdx]
			}

			// Try to get the bucket for this domain
			bucket, err := metaStore.GetCustomDomain(r.Context(), host)
			if err == nil && bucket != "" {
				// We found a custom domain match
				// Rewrite the URL Path to prepend the bucket name
				// e.g., /index.html -> /my-bucket/index.html

				// Ensure it starts with /
				originalPath := r.URL.Path
				if !strings.HasPrefix(originalPath, "/") {
					originalPath = "/" + originalPath
				}

				// Rewrite
				newPath := "/" + bucket + originalPath

				// Update request
				r.URL.Path = newPath

				// If chi router has already parsed path, this might be tricky,
				// but since we insert this middleware BEFORE routing (in SetupRouter),
				// chi will route based on the new r.URL.Path.

				// Add a context value so downstream handlers know it's a custom domain?
				// Maybe not necessary, but could be useful.
				ctx := context.WithValue(r.Context(), "IsCustomDomain", true)
				ctx = context.WithValue(ctx, "CustomDomain", host)
				r = r.WithContext(ctx)
			}

			next.ServeHTTP(w, r)
		})
	}
}
