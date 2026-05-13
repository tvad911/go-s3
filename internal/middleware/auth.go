package middleware

import (
	"context"
	"net/http"

	"gos3/internal/auth"
	"gos3/internal/handler"
	"gos3/internal/s3"
)

// Auth middleware validates SigV4 signatures.
func Auth(verifier *auth.SigV4Verifier) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip auth for OPTIONS (CORS preflight) and health/metrics/ui endpoints
			if r.Method == "OPTIONS" || r.URL.Path == "/_health" || r.URL.Path == "/_metrics" || len(r.URL.Path) >= 4 && r.URL.Path[:4] == "/_ui" {
				next.ServeHTTP(w, r)
				return
			}

			user, err := verifier.Verify(r)
			if err != nil {
				// If no auth header is provided, we can either reject or treat as anonymous
				if err == auth.ErrAuthHeaderMissing {
					// Treat as anonymous for now (Bucket Policy / ACL will deny if restricted)
					// But for safety in phase 3, we might deny all unauthenticated requests to S3 API
					// handler.WriteError(w, r, s3.ErrAccessDenied)
					// return
					user = &auth.User{Username: "anonymous"}
				} else {
					// Authentication failed
					handler.WriteError(w, r, s3.ErrSignatureDoesNotMatch) // Or appropriate mapping
					return
				}
			}

			// Store user in context
			ctx := context.WithValue(r.Context(), auth.UserContextKey, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
