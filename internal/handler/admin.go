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
	"gos3/internal/storage"
	"gos3/internal/storage/metadata"
)

var startTime = time.Now()

type AdminHandler struct {
	MetaStore metadata.Store
	Backend   storage.Backend
}

func NewAdminHandler(store metadata.Store, backend storage.Backend) *AdminHandler {
	return &AdminHandler{MetaStore: store, Backend: backend}
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
	users, err := h.MetaStore.ListUsers(r.Context())
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

	if err := h.MetaStore.CreateUser(r.Context(), &user); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
}

func (h *AdminHandler) GetUser(w http.ResponseWriter, r *http.Request) {
	username := chi.URLParam(r, "username")
	user, err := h.MetaStore.GetUserByUsername(r.Context(), username)
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

	if err := h.MetaStore.UpdateUser(r.Context(), &user); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (h *AdminHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	username := chi.URLParam(r, "username")
	if err := h.MetaStore.DeleteUser(r.Context(), username); err != nil {
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

	region := "us-east-1"

	accessKey := user.AccessKeyID
	secretKey := user.SecretKey

	if accessKey == "" || secretKey == "" {
		sas, err := h.MetaStore.ListServiceAccountsByUser(r.Context(), user.Username)
		if err != nil || len(sas) == 0 {
			http.Error(w, "no active service account found to sign the URL. Please create one in Access Keys.", http.StatusBadRequest)
			return
		}
		for _, sa := range sas {
			if !sa.Disabled {
				accessKey = sa.AccessKeyID
				secretKey = sa.SecretKey
				break
			}
		}
		if accessKey == "" {
			http.Error(w, "all service accounts are disabled", http.StatusBadRequest)
			return
		}
	}

	urlStr := auth.GeneratePresignedURL(req.Method, endpoint, region, accessKey, secretKey, req.Bucket, req.Key, req.Expires)

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

func (h *AdminHandler) BucketStats(w http.ResponseWriter, r *http.Request) {
	bucket := chi.URLParam(r, "bucket")
	objects, bytes, err := h.MetaStore.GetBucketStats(bucket)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	stats := map[string]interface{}{
		"bucket": bucket,
		"objects": objects,
		"bytes": bytes,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// RenameObject handles renaming (copy + delete) an object entirely server-side.
// This avoids presigned URL complications with encoding, Host headers, and SigV4.
func (h *AdminHandler) RenameObject(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Bucket string `json:"bucket"`
		OldKey string `json:"oldKey"`
		NewKey string `json:"newKey"`
	}

	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	if req.Bucket == "" || req.OldKey == "" || req.NewKey == "" {
		http.Error(w, `{"error":"bucket, oldKey, and newKey are required"}`, http.StatusBadRequest)
		return
	}

	if req.OldKey == req.NewKey {
		http.Error(w, `{"error":"old and new keys are identical"}`, http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	// Step 1: Copy source → destination
	_, err := h.Backend.CopyObject(ctx, req.Bucket, req.OldKey, req.Bucket, req.NewKey, nil)
	if err != nil {
		status := http.StatusInternalServerError
		if err == s3.ErrNoSuchKey || err == s3.ErrNoSuchBucket {
			status = http.StatusNotFound
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	// Step 2: Delete old object
	if err := h.Backend.DeleteObject(ctx, req.Bucket, req.OldKey, ""); err != nil {
		// Copy succeeded but delete failed — not ideal but object exists at newKey
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "copy succeeded but failed to delete original: " + err.Error(),
			"warning": "object exists at both old and new keys",
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "renamed successfully",
		"oldKey":  req.OldKey,
		"newKey":  req.NewKey,
	})
}

