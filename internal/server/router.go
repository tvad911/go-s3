package server

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"

	"gos3/internal/auth"
	"gos3/internal/config"
	"gos3/internal/handler"
	"gos3/internal/middleware"
	"gos3/internal/storage"
	"gos3/web"
)

// SetupRouter initializes and returns the main HTTP router for the server.
func SetupRouter(cfg *config.Config, backend storage.Backend, verifier *auth.SigV4Verifier) *chi.Mux {
	r := chi.NewRouter()

	s3Handler := handler.NewS3Handler(backend, backend.(auth.PolicyStore))
	adminHandler := handler.NewAdminHandler(backend.(auth.UserStore)) // backend should implement UserStore if we passed it correctly, or we need to pass store to SetupRouter

	r.Use(middleware.RealIP)
	r.Use(middleware.RequestID)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RateLimit(&cfg.RateLimit))
	r.Use(middleware.CORS(backend.(middleware.CORSStore)))
	r.Use(middleware.Auth(verifier))

	// Admin API
	r.Route(cfg.Admin.PathPrefix, func(r chi.Router) {
		r.Use(handler.EnsureRoot) // Must be root

		// User Management
		r.Get("/users", adminHandler.ListUsers)
		r.Post("/users", adminHandler.CreateUser)
		r.Get("/users/{username}", adminHandler.GetUser)
		r.Put("/users/{username}", adminHandler.UpdateUser)
		r.Delete("/users/{username}", adminHandler.DeleteUser)
		r.Post("/users/{username}/rotate-key", stubHandler("RotateUserKey"))
	})

	// Metrics and Health
	r.Get("/_health", stubHandler("HealthCheck"))
	r.Get(cfg.Metrics.Path, stubHandler("PrometheusMetrics"))

	// Web UI stub (Phase 1.7)
	r.Get("/_ui/*", func(w http.ResponseWriter, r *http.Request) {
		http.StripPrefix("/_ui/", http.FileServer(http.FS(web.DistFS))).ServeHTTP(w, r)
	})

	// S3 API Routes

	// Service
	r.Get("/", s3Handler.ListBuckets)

	// Bucket operations
	r.Route("/{bucket}", func(r chi.Router) {
		// Bucket CRUD
		r.Get("/", func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Query().Has("acl") {
				s3Handler.GetBucketAcl(w, r)
			} else if r.URL.Query().Has("cors") {
				s3Handler.GetBucketCors(w, r)
			} else if r.URL.Query().Has("policy") {
				s3Handler.GetBucketPolicy(w, r)
			} else if r.URL.Query().Has("uploads") {
				s3Handler.ListMultipartUploads(w, r)
			} else {
				s3Handler.ListObjects(w, r)
			}
		})
		r.Put("/", func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Query().Has("acl") {
				s3Handler.PutBucketAcl(w, r)
			} else if r.URL.Query().Has("cors") {
				s3Handler.PutBucketCors(w, r)
			} else if r.URL.Query().Has("policy") {
				s3Handler.PutBucketPolicy(w, r)
			} else {
				s3Handler.CreateBucket(w, r)
			}
		})
		r.Delete("/", func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Query().Has("cors") {
				s3Handler.DeleteBucketCors(w, r)
			} else if r.URL.Query().Has("policy") {
				s3Handler.DeleteBucketPolicy(w, r)
			} else {
				s3Handler.DeleteBucket(w, r)
			}
		})
		r.Head("/", s3Handler.HeadBucket)
		r.Post("/", func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Query().Has("delete") {
				s3Handler.DeleteObjects(w, r)
			} else {
				stubHandler("PostObject")(w, r)
			}
		})

		// Object operations
		r.Route("/{key:.*}", func(r chi.Router) {
			r.Get("/", func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Query().Has("acl") {
					s3Handler.GetObjectAcl(w, r)
				} else if r.URL.Query().Has("uploadId") {
					s3Handler.ListParts(w, r)
				} else {
					s3Handler.GetObject(w, r)
				}
			})
			r.Put("/", func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Query().Has("acl") {
					s3Handler.PutObjectAcl(w, r)
				} else if r.URL.Query().Has("partNumber") && r.URL.Query().Has("uploadId") {
					s3Handler.UploadPart(w, r)
				} else if r.Header.Get("x-amz-copy-source") != "" {
					s3Handler.CopyObject(w, r)
				} else {
					s3Handler.PutObject(w, r)
				}
			})
			r.Delete("/", func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Query().Has("uploadId") {
					s3Handler.AbortMultipartUpload(w, r)
				} else {
					s3Handler.DeleteObject(w, r)
				}
			})
			r.Head("/", s3Handler.HeadObject)
			r.Options("/", stubHandler("CORSPreflight"))
			r.Post("/", func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Query().Has("uploads") {
					s3Handler.CreateMultipartUpload(w, r)
				} else if r.URL.Query().Has("uploadId") {
					s3Handler.CompleteMultipartUpload(w, r)
				} else {
					stubHandler("PostObject")(w, r)
				}
			})
		})
	})

	return r
}

// stubHandler is a temporary handler for unimplemented endpoints.
func stubHandler(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotImplemented)
		w.Write([]byte(fmt.Sprintf("Endpoint %s not implemented yet\n", name)))
	}
}
