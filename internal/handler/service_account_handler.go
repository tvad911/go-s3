package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"gos3/internal/auth"
	"gos3/internal/storage/metadata"
)

// ServiceAccountHandler handles service account CRUD for the Web Console.
type ServiceAccountHandler struct {
	store metadata.Store
}

// NewServiceAccountHandler creates a new ServiceAccountHandler.
func NewServiceAccountHandler(store metadata.Store) *ServiceAccountHandler {
	return &ServiceAccountHandler{store: store}
}

type createSARequest struct {
	Description string     `json:"description"`
	Policies    []string   `json:"policies,omitempty"`
	ExpiresAt   *time.Time `json:"expiresAt,omitempty"`
}

type createSAResponse struct {
	ID          string `json:"id"`
	AccessKeyID string `json:"accessKeyId"`
	SecretKey   string `json:"secretKey"`
	Description string `json:"description"`
}

// CreateServiceAccount generates a new access key pair for the authenticated user.
func (h *ServiceAccountHandler) CreateServiceAccount(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r.Context())
	if user.Username == "anonymous" {
		http.Error(w, `{"error":"not authenticated"}`, http.StatusUnauthorized)
		return
	}

	var req createSARequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 2<<20)).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	accessKey, err := auth.GenerateAccessKey()
	if err != nil {
		http.Error(w, `{"error":"failed to generate access key"}`, http.StatusInternalServerError)
		return
	}

	secretKey, err := auth.GenerateSecretKey()
	if err != nil {
		http.Error(w, `{"error":"failed to generate secret key"}`, http.StatusInternalServerError)
		return
	}

	sa := &auth.ServiceAccount{
		ID:          uuid.NewString(),
		AccessKeyID: accessKey,
		SecretKey:   secretKey,
		ParentUser:  user.Username,
		Description: req.Description,
		Policies:    req.Policies,
		ExpiresAt:   req.ExpiresAt,
		CreatedAt:   time.Now(),
	}

	if err := h.store.CreateServiceAccount(r.Context(), sa); err != nil {
		http.Error(w, `{"error":"failed to create service account"}`, http.StatusInternalServerError)
		return
	}

	// Return the secret key only once
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(createSAResponse{
		ID:          sa.ID,
		AccessKeyID: sa.AccessKeyID,
		SecretKey:   secretKey, // Only returned at creation time
		Description: sa.Description,
	})
}

// ListServiceAccounts returns all service accounts for the authenticated user.
func (h *ServiceAccountHandler) ListServiceAccounts(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r.Context())
	if user.Username == "anonymous" {
		http.Error(w, `{"error":"not authenticated"}`, http.StatusUnauthorized)
		return
	}

	accounts, err := h.store.ListServiceAccountsByUser(r.Context(), user.Username)
	if err != nil {
		http.Error(w, `{"error":"failed to list service accounts"}`, http.StatusInternalServerError)
		return
	}

	if accounts == nil {
		accounts = []*auth.ServiceAccount{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(accounts)
}

// DeleteServiceAccount removes a service account by ID.
func (h *ServiceAccountHandler) DeleteServiceAccount(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r.Context())
	if user.Username == "anonymous" {
		http.Error(w, `{"error":"not authenticated"}`, http.StatusUnauthorized)
		return
	}

	saID := chi.URLParam(r, "id")
	if saID == "" {
		http.Error(w, `{"error":"service account id is required"}`, http.StatusBadRequest)
		return
	}

	// Verify ownership: non-root users can only delete their own keys
	if !user.IsRoot {
		accounts, err := h.store.ListServiceAccountsByUser(r.Context(), user.Username)
		if err != nil {
			http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
			return
		}
		owned := false
		for _, a := range accounts {
			if a.ID == saID {
				owned = true
				break
			}
		}
		if !owned {
			http.Error(w, `{"error":"access denied"}`, http.StatusForbidden)
			return
		}
	}

	if err := h.store.DeleteServiceAccount(r.Context(), saID); err != nil {
		http.Error(w, `{"error":"failed to delete service account"}`, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
