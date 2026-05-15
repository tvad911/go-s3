package handler

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"gos3/internal/auth"
	"gos3/internal/config"
	"gos3/internal/replication"
	"gos3/internal/s3"
	"gos3/internal/storage"
	"gos3/internal/storage/metadata"
)

// S3Handler handles S3 HTTP requests.
type S3Handler struct {
	Backend      storage.Backend
	MetaStore    metadata.Store
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

// getObjectKey extracts the key parameter from chi.URLParam and restores the trailing slash if the original request path had it.
// This is necessary because chi.URLParam strips trailing slashes on wildcard matches.
func getObjectKey(r *http.Request) string {
	key := chi.URLParam(r, "*")
	if key == "" {
		key = chi.URLParam(r, "key") // Fallback just in case
	}
	// If the original path ends with / but the extracted key doesn't, restore it.
	if strings.HasSuffix(r.URL.Path, "/") && !strings.HasSuffix(key, "/") {
		key += "/"
	}
	return key
}

// CheckPolicy evaluates the IAM Policy Engine.
// Returns nil if allowed, otherwise s3.ErrAccessDenied.
func (h *S3Handler) CheckPolicy(r *http.Request, action, bucket, object string) error {
	user := auth.GetUser(r.Context())
	if user.IsRoot {
		return nil
	}

	if bucket != "" {
		if action == "s3:CreateBucket" {
			// Anyone authenticated can create a bucket (or you could restrict this)
			// But anonymous cannot
			if user.Username == "anonymous" {
				return s3.ErrAccessDenied
			}
			return nil
		}
		
		if bInfo, err := h.MetaStore.GetBucket(bucket); err == nil {
			if bInfo.Owner != "" && bInfo.Owner == user.Username {
				return nil
			}
			// Also check if the user is a service account of the owner
			// Actually, User.Username is the parent user's username if it's a ServiceAccount?
			// Wait, GetUser() resolves Service Account to its Parent User username?
			// Let's check auth.SigV4Verifier
		}
	}

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
