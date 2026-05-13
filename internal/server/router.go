package server

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"

	"gos3/internal/config"
	"gos3/internal/middleware"
	"gos3/web"
)

// SetupRouter initializes and returns the main HTTP router for the server.
func SetupRouter(cfg *config.Config) *chi.Mux {
	r := chi.NewRouter()

	r.Use(middleware.RealIP)
	r.Use(middleware.RequestID)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RateLimit(&cfg.RateLimit))
	// TODO: Add auth and cors
	// r.Use(mw.CORS)
	// r.Use(mw.Auth)

	// Admin API
	r.Route(cfg.Admin.PathPrefix, func(r chi.Router) {
		r.Get("/info", stubHandler("GetAdminInfo"))
		r.Post("/presign", stubHandler("GeneratePresignedURL"))
		r.Route("/users", func(r chi.Router) {
			r.Get("/", stubHandler("ListUsers"))
			r.Post("/", stubHandler("CreateUser"))
			r.Get("/{username}", stubHandler("GetUser"))
			r.Put("/{username}", stubHandler("UpdateUser"))
			r.Delete("/{username}", stubHandler("DeleteUser"))
			r.Post("/{username}/rotate-key", stubHandler("RotateUserKey"))
		})
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
	r.Get("/", stubHandler("ListBuckets"))

	// Bucket operations
	r.Route("/{bucket}", func(r chi.Router) {
		// Bucket CRUD
		r.Put("/", stubHandler("CreateBucket"))
		r.Delete("/", stubHandler("DeleteBucket"))
		r.Head("/", stubHandler("HeadBucket"))
		r.Post("/", stubHandler("DeleteObjectsOrPostObject")) // POST ?delete or HTML form upload
		r.Get("/", stubHandler("ListObjects"))                // Dispatch by query: ?list-type=2, ?versions, ?uploads, etc.

		// Object operations
		r.Route("/{key:.*}", func(r chi.Router) {
			r.Get("/", stubHandler("GetObject"))
			r.Put("/", stubHandler("PutObject")) // Covers CopyObject via x-amz-copy-source and UploadPart via ?partNumber
			r.Delete("/", stubHandler("DeleteObject"))
			r.Head("/", stubHandler("HeadObject"))
			r.Options("/", stubHandler("CORSPreflight"))
			r.Post("/", stubHandler("CreateCompleteOrMultipart")) // S3 Select stub, Create/Complete/Abort Multipart based on query
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
