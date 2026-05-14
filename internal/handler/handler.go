package handler

import (
	"net/http"

	"gos3/internal/auth"
	"gos3/internal/config"
	"gos3/internal/replication"
	"gos3/internal/s3"
	"gos3/internal/storage"
	"gos3/internal/storage/metadata"
)

// S3Handler handles S3 HTTP requests.
type S3Handler struct {
	Backend     storage.Backend
	MetaStore   metadata.Store
	Verifier     *auth.SigV4Verifier
	Config       *config.Config
	Replication  *replication.Service
	PolicyEngine *auth.Engine
}

// NewS3Handler creates a new S3Handler.
func NewS3Handler(backend storage.Backend, metaStore metadata.Store, verifier *auth.SigV4Verifier, cfg *config.Config, repl *replication.Service, engine *auth.Engine) *S3Handler {
	return &S3Handler{
		Backend:      backend,
		MetaStore:    metaStore,
		Verifier:     verifier,
		Config:       cfg,
		Replication:  repl,
		PolicyEngine: engine,
	}
}

// CheckPolicy evaluates the IAM Policy Engine.
// Returns nil if allowed, otherwise s3.ErrAccessDenied.
func (h *S3Handler) CheckPolicy(r *http.Request, action, bucket, object string) error {
	user := auth.GetUser(r.Context())
	resource := "arn:aws:s3:::" + bucket
	if object != "" {
		resource += "/" + object
	} else if bucket == "" {
		resource = "*"
	}

	allowed, err := h.PolicyEngine.IsAllowed(r.Context(), user, action, resource, bucket)
	if err != nil {
		// Log the error in a real app
		return s3.ErrAccessDenied
	}
	if !allowed {
		return s3.ErrAccessDenied
	}
	return nil
}
