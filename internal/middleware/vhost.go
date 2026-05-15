package middleware

import (
	"context"
	"net/http"
	"strings"

	"gos3/internal/storage/metadata"
)

// CustomDomain rewrites the request path if the Host matches a custom domain or virtual-hosted style domain.
func CustomDomain(metaStore metadata.Store, baseDomain string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			host := r.Host
			// Remove port if present
			if colonIdx := strings.LastIndex(host, ":"); colonIdx != -1 {
				host = host[:colonIdx]
			}

			var bucket string
			var isCustomDomain bool

			// 1. Virtual-hosted style (e.g., bucket.s3.domain.com)
			if baseDomain != "" && strings.HasSuffix(host, "."+baseDomain) {
				bucket = strings.TrimSuffix(host, "."+baseDomain)
			}

			// 2. CNAME mapping (Custom Domain)
			if bucket == "" {
				b, err := metaStore.GetCustomDomain(r.Context(), host)
				if err == nil && b != "" {
					bucket = b
					isCustomDomain = true
				}
			}

			if bucket != "" {
				// Ensure path starts with /
				originalPath := r.URL.Path
				if !strings.HasPrefix(originalPath, "/") {
					originalPath = "/" + originalPath
				}

				// Rewrite
				r.URL.Path = "/" + bucket + originalPath

				ctx := r.Context()
				if isCustomDomain {
					ctx = context.WithValue(ctx, "IsCustomDomain", true)
					ctx = context.WithValue(ctx, "CustomDomain", host)
				}
				r = r.WithContext(ctx)
			}

			next.ServeHTTP(w, r)
		})
	}
}
