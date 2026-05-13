package middleware

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"gos3/internal/s3"
)

// CORSStore defines the interface for fetching bucket CORS configurations.
type CORSStore interface {
	GetBucketCORS(ctx context.Context, bucket string) (*s3.CORSConfiguration, error)
}

// CORS returns a middleware that handles S3 CORS configurations.
func CORS(store CORSStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin == "" {
				// Not a CORS request
				next.ServeHTTP(w, r)
				return
			}

			// Try to get bucket name from chi context
			bucket := chi.URLParam(r, "bucket")
			if bucket == "" {
				// If chi hasn't routed yet, try manual extraction
				path := strings.TrimPrefix(r.URL.Path, "/")
				parts := strings.SplitN(path, "/", 2)
				if len(parts) > 0 && parts[0] != "" && parts[0] != "_admin" {
					bucket = parts[0]
				}
			}

			var corsConfig *s3.CORSConfiguration
			if bucket != "" && store != nil {
				// Ignore error, if no CORS is configured, we'll use a default or deny
				corsConfig, _ = store.GetBucketCORS(r.Context(), bucket)
			}

			// If no specific bucket config, allow all for development (or could be strict)
			// S3 default is no CORS allowed unless configured. We'll be permissive if not configured for now,
			// or strict if requested. Let's implement strict but with a fallback.
			
			if corsConfig != nil && len(corsConfig.CORSRule) > 0 {
				applyCORSRules(w, r, origin, corsConfig.CORSRule)
			} else {
				// Fallback: permissive CORS for local dev
				w.Header().Set("Access-Control-Allow-Origin", "*")
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, HEAD, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "*")
				w.Header().Set("Access-Control-Expose-Headers", "ETag, x-amz-request-id, x-amz-id-2")
			}

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusOK)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func applyCORSRules(w http.ResponseWriter, r *http.Request, origin string, rules []s3.CORSRule) {
	reqMethod := r.Header.Get("Access-Control-Request-Method")
	if reqMethod == "" {
		reqMethod = r.Method
	}

	for _, rule := range rules {
		if !matchList(rule.AllowedOrigin, origin) {
			continue
		}
		if !matchList(rule.AllowedMethod, reqMethod) {
			continue
		}

		w.Header().Set("Access-Control-Allow-Origin", origin)
		if len(rule.AllowedMethod) > 0 {
			w.Header().Set("Access-Control-Allow-Methods", strings.Join(rule.AllowedMethod, ", "))
		}
		if len(rule.AllowedHeader) > 0 {
			w.Header().Set("Access-Control-Allow-Headers", strings.Join(rule.AllowedHeader, ", "))
		} else {
			// If not specified, echo back requested headers
			if reqHeaders := r.Header.Get("Access-Control-Request-Headers"); reqHeaders != "" {
				w.Header().Set("Access-Control-Allow-Headers", reqHeaders)
			}
		}
		if len(rule.ExposeHeader) > 0 {
			w.Header().Set("Access-Control-Expose-Headers", strings.Join(rule.ExposeHeader, ", "))
		}
		if rule.MaxAgeSeconds > 0 {
			w.Header().Set("Access-Control-Max-Age", strconv.Itoa(rule.MaxAgeSeconds))
		}
		return
	}
}

func matchList(patterns []string, value string) bool {
	for _, p := range patterns {
		if p == "*" || p == value {
			return true
		}
		if strings.HasSuffix(p, "*") {
			prefix := strings.TrimSuffix(p, "*")
			if strings.HasPrefix(value, prefix) {
				return true
			}
		}
	}
	return false
}
