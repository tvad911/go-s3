package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/go-chi/chi/v5"

	"gos3/internal/auth"
	"gos3/internal/metrics"
	"gos3/internal/s3"
)

var startTime = time.Now()

type AdminHandler struct {
	UserStore auth.UserStore
}

func NewAdminHandler(store auth.UserStore) *AdminHandler {
	return &AdminHandler{UserStore: store}
}

// EnsureRoot checks if the user in context is an admin/root
func EnsureRoot(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := auth.GetUser(r.Context())
		if !user.IsRoot {
			WriteError(w, r, s3.ErrAccessDenied)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (h *AdminHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.UserStore.ListUsers(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Strip sensitive fields before sending to client
	for _, u := range users {
		u.SecretKey = ""
		u.PasswordHash = ""
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(users)
}

func (h *AdminHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	var user auth.User
	if err := json.NewDecoder(io.LimitReader(r.Body, 2<<20)).Decode(&user); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := h.UserStore.CreateUser(r.Context(), &user); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
}

func (h *AdminHandler) GetUser(w http.ResponseWriter, r *http.Request) {
	username := chi.URLParam(r, "username")
	user, err := h.UserStore.GetUserByUsername(r.Context(), username)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}

func (h *AdminHandler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	username := chi.URLParam(r, "username")
	var user auth.User
	if err := json.NewDecoder(io.LimitReader(r.Body, 2<<20)).Decode(&user); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	user.Username = username

	if err := h.UserStore.UpdateUser(r.Context(), &user); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (h *AdminHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	username := chi.URLParam(r, "username")
	if err := h.UserStore.DeleteUser(r.Context(), username); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

type PresignRequest struct {
	Method  string `json:"method"`
	Bucket  string `json:"bucket"`
	Key     string `json:"key"`
	Expires int64  `json:"expires"`
}

type PresignResponse struct {
	URL string `json:"url"`
}

func (h *AdminHandler) GeneratePresignedURL(w http.ResponseWriter, r *http.Request) {
	var req PresignRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 2<<20)).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.Method == "" || req.Bucket == "" || req.Expires <= 0 {
		http.Error(w, "missing required fields", http.StatusBadRequest)
		return
	}

	// For the admin generating the URL, we use their credentials.
	// In a real scenario, they might want to generate it for a specific user,
	// but the plan says "Server tự generate presigned URL".
	user := auth.GetUser(r.Context())
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	scheme := "http://"
	if r.TLS != nil {
		scheme = "https://"
	}
	endpoint := scheme + r.Host

	// Hardcode region for now, should ideally be from config
	region := "us-east-1"

	urlStr := auth.GeneratePresignedURL(req.Method, endpoint, region, user.AccessKeyID, user.SecretKey, req.Bucket, req.Key, req.Expires)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(PresignResponse{URL: urlStr})
}

// ServerInfo returns general server info and storage stats
func (h *AdminHandler) ServerInfo(w http.ResponseWriter, r *http.Request) {
	var totalObjects, totalBytes int64
	
	metrics.ObjectsTotal.Range(func(key, value any) bool {
		totalObjects += value.(*atomic.Int64).Load()
		return true
	})
	
	metrics.StorageBytes.Range(func(key, value any) bool {
		totalBytes += value.(*atomic.Int64).Load()
		return true
	})

	info := map[string]interface{}{
		"version": "0.1.0",
		"uptime_seconds": int(time.Since(startTime).Seconds()),
		"storage": map[string]interface{}{
			"total_objects": totalObjects,
			"total_bytes": totalBytes,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(info)
}
