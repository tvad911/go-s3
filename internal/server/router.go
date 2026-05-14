package server

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"

	"gos3/internal/auth"
	"gos3/internal/config"
	"gos3/internal/handler"
	"gos3/internal/metrics"
	"gos3/internal/middleware"
	"gos3/internal/replication"
	"gos3/internal/storage"
	"gos3/internal/storage/metadata"
	"gos3/web"
)

// SetupRouter initializes and returns the main HTTP router for the server.
func SetupRouter(cfg *config.Config, backend storage.Backend, metaStore metadata.Store, verifier *auth.SigV4Verifier, repl *replication.Service, sessionCfg *auth.SessionConfig) *chi.Mux {
	r := chi.NewRouter()

	adminHandler := handler.NewAdminHandler(metaStore, backend)
	authHandler := handler.NewAuthHandler(metaStore, sessionCfg)
	saHandler := handler.NewServiceAccountHandler(metaStore)
	s3Handler := handler.NewS3Handler(backend, metaStore, verifier, cfg, repl)

	r.Use(middleware.RealIP)
	r.Use(middleware.RequestID)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RateLimit(&cfg.RateLimit))
	r.Use(middleware.CORS(metaStore))

	// Web Console Auth API — No SigV4 required, these use username/password
	r.Route("/api/v1", func(r chi.Router) {
		// Public endpoints (no auth required)
		r.Post("/login", authHandler.Login)
		r.Post("/logout", authHandler.Logout)

		// Protected endpoints (JWT cookie required)
		r.Group(func(r chi.Router) {
			r.Use(handler.JWTAuthMiddleware(sessionCfg, metaStore))

			r.Get("/me", authHandler.Me)

			// Service Account management
			r.Post("/service-accounts", saHandler.CreateServiceAccount)
			r.Get("/service-accounts", saHandler.ListServiceAccounts)
			r.Delete("/service-accounts/{id}", saHandler.DeleteServiceAccount)
		})
	})

	// S3 + Admin API — requires SigV4 auth
	r.Group(func(r chi.Router) {
		r.Use(middleware.Auth(verifier))
		r.Use(middleware.AdvancedRateLimit(&cfg.RateLimit))

		// Admin API
		r.Route(cfg.Admin.PathPrefix, func(r chi.Router) {
			r.Use(handler.JWTAuthFallbackMiddleware(sessionCfg, metaStore))
			r.Use(handler.EnsureRoot) // Must be root

			// User Management
			r.Get("/users", adminHandler.ListUsers)
			r.Post("/users", adminHandler.CreateUser)
			r.Get("/users/{username}", adminHandler.GetUser)
			r.Put("/users/{username}", adminHandler.UpdateUser)
			r.Delete("/users/{username}", adminHandler.DeleteUser)
			r.Post("/users/{username}/rotate-key", stubHandler("RotateUserKey"))

			// Presign
			r.Post("/presign", adminHandler.GeneratePresignedURL)

			// Rename Object (server-side copy + delete)
			r.Post("/rename", adminHandler.RenameObject)

			// Server Info
			r.Get("/info", adminHandler.ServerInfo)
			r.Get("/audit-logs", adminHandler.ListAuditLogs)

			// Buckets Stats
			r.Get("/buckets/{bucket}/stats", adminHandler.BucketStats)

			// Web UI Buckets/Objects API
			r.Get("/buckets", adminHandler.ListBuckets)
			r.Put("/buckets/{bucket}", adminHandler.CreateBucket)
			r.Delete("/buckets/{bucket}", adminHandler.DeleteBucket)
			r.Get("/buckets/{bucket}/objects", adminHandler.ListObjects)

			// Web UI Bucket Policy API
			r.Get("/buckets/{bucket}/policy", adminHandler.GetBucketPolicy)
			r.Put("/buckets/{bucket}/policy", adminHandler.PutBucketPolicy)
			r.Delete("/buckets/{bucket}/policy", adminHandler.DeleteBucketPolicy)

			// Web UI Bucket CORS API
			r.Get("/buckets/{bucket}/cors", adminHandler.GetBucketCORS)
			r.Put("/buckets/{bucket}/cors", adminHandler.PutBucketCORS)
			r.Delete("/buckets/{bucket}/cors", adminHandler.DeleteBucketCORS)
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
				} else if r.URL.Query().Has("versioning") {
					s3Handler.GetBucketVersioning(w, r)
				} else if r.URL.Query().Has("lifecycle") {
					s3Handler.GetBucketLifecycle(w, r)
				} else if r.URL.Query().Has("website") {
					s3Handler.GetBucketWebsite(w, r)
				} else if r.URL.Query().Has("versions") {
					s3Handler.ListObjectVersions(w, r)
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
				} else if r.URL.Query().Has("versioning") {
					s3Handler.PutBucketVersioning(w, r)
				} else if r.URL.Query().Has("lifecycle") {
					s3Handler.PutBucketLifecycle(w, r)
				} else if r.URL.Query().Has("website") {
					s3Handler.PutBucketWebsite(w, r)
				} else {
					s3Handler.CreateBucket(w, r)
				}
			})
			r.Delete("/", func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Query().Has("cors") {
					s3Handler.DeleteBucketCors(w, r)
				} else if r.URL.Query().Has("lifecycle") {
					s3Handler.DeleteBucketLifecycle(w, r)
				} else if r.URL.Query().Has("website") {
					s3Handler.DeleteBucketWebsite(w, r)
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
					s3Handler.PostObject(w, r)
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
	})
	// Metrics and Health (no auth required)
	r.Get("/_health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})
	r.Get(cfg.Metrics.Path, metrics.Handler().ServeHTTP)

	// Web UI (no auth required — the UI handles its own login)
	r.Get("/_ui/*", func(w http.ResponseWriter, r *http.Request) {
		http.StripPrefix("/_ui/", http.FileServer(http.FS(web.DistFS))).ServeHTTP(w, r)
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
