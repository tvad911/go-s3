package handler

import (
	"gos3/internal/auth"
	"gos3/internal/config"
	"gos3/internal/storage"
	"gos3/internal/storage/metadata"
)

// S3Handler handles S3 HTTP requests.
type S3Handler struct {
	Backend   storage.Backend
	MetaStore metadata.Store
	Verifier  *auth.SigV4Verifier
	Config    *config.Config
}

// NewS3Handler creates a new S3Handler.
func NewS3Handler(backend storage.Backend, metaStore metadata.Store, verifier *auth.SigV4Verifier, cfg *config.Config) *S3Handler {
	return &S3Handler{
		Backend:   backend,
		MetaStore: metaStore,
		Verifier:  verifier,
		Config:    cfg,
	}
}
