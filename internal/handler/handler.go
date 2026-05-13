package handler

import (
	"gos3/internal/auth"
	"gos3/internal/storage"
)

// S3Handler handles S3 HTTP requests.
type S3Handler struct {
	Backend     storage.Backend
	PolicyStore auth.PolicyStore
}

// NewS3Handler creates a new S3Handler.
func NewS3Handler(backend storage.Backend, policyStore auth.PolicyStore) *S3Handler {
	return &S3Handler{
		Backend:     backend,
		PolicyStore: policyStore,
	}
}
