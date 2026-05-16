package handler

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"sync/atomic"
	"time"

	"runtime"
	"syscall"

	"github.com/go-chi/chi/v5"

	"gos3/internal/auth"
	"gos3/internal/config"
	"gos3/internal/metrics"
	"gos3/internal/s3"
	"gos3/internal/storage"
	"gos3/internal/storage/metadata"
)

var startTime = time.Now()

type AdminHandler struct {
	MetaStore    metadata.Store
	Backend      storage.Backend
	PolicyEngine *auth.Engine
	Config       *config.Config
}

func NewAdminHandler(store metadata.Store, backend storage.Backend, engine *auth.Engine, cfg *config.Config) *AdminHandler {
	return &AdminHandler{MetaStore: store, Backend: backend, PolicyEngine: engine, Config: cfg}
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

func (h *AdminHandler) recordAudit(r *http.Request, action, target, details string) {
	user := auth.GetUser(r.Context())
	username := "system"
	if user.Username != "" {
		username = user.Username
	}

	log := metadata.AuditLog{
		User:    username,
		Action:  action,
		Target:  target,
		Details: details,
		IP:      r.RemoteAddr,
	}

	// Fire and forget
	go func() {
		if err := h.MetaStore.RecordAuditLog(context.Background(), &log); err != nil {
			slog.Error("failed to record audit log", "error", err, "action", action)
		}
	}()
}

func (h *AdminHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	if err := h.CheckAdminPolicy(r, "admin:ListUsers"); err != nil {
		WriteError(w, r, err)
		return
	}

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
	if err := h.CheckAdminPolicy(r, "admin:CreateUser"); err != nil {
		WriteError(w, r, err)
		return
	}

	var user auth.User
	if err := json.NewDecoder(io.LimitReader(r.Body, 2<<20)).Decode(&user); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := h.MetaStore.CreateUser(r.Context(), &user); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.recordAudit(r, "CreateUser", user.Username, "")
	w.WriteHeader(http.StatusCreated)
}

func (h *AdminHandler) GetUser(w http.ResponseWriter, r *http.Request) {
	if err := h.CheckAdminPolicy(r, "admin:GetUser"); err != nil {
		WriteError(w, r, err)
		return
	}

	username := chi.URLParam(r, "username")
	user, err := h.MetaStore.GetUserByUsername(r.Context(), username)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	// Strip sensitive fields before sending to client
	user.SecretKey = ""
	user.PasswordHash = ""

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}

func (h *AdminHandler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	if err := h.CheckAdminPolicy(r, "admin:UpdateUser"); err != nil {
		WriteError(w, r, err)
		return
	}

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

	h.recordAudit(r, "UpdateUser", username, "")
	w.WriteHeader(http.StatusOK)
}

func (h *AdminHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	if err := h.CheckAdminPolicy(r, "admin:DeleteUser"); err != nil {
		WriteError(w, r, err)
		return
	}

	username := chi.URLParam(r, "username")
	if err := h.MetaStore.DeleteUser(r.Context(), username); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.recordAudit(r, "DeleteUser", username, "")
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
	if err := h.CheckAdminPolicy(r, "admin:ServerInfo"); err != nil {
		WriteError(w, r, err)
		return
	}

	var totalObjects, totalBytes int64

	metrics.ObjectsTotal.Range(func(key, value any) bool {
		totalObjects += value.(*atomic.Int64).Load()
		return true
	})

	metrics.StorageBytes.Range(func(key, value any) bool {
		totalBytes += value.(*atomic.Int64).Load()
		return true
	})

	buckets, _ := h.MetaStore.ListBuckets()
	users, _ := h.MetaStore.ListUsers(r.Context())

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	var diskTotal, diskFree uint64
	var stat syscall.Statfs_t
	if h.Config != nil && h.Config.Storage.DataDir != "" {
		if err := syscall.Statfs(h.Config.Storage.DataDir, &stat); err == nil {
			diskTotal = stat.Blocks * uint64(stat.Bsize)
			diskFree = stat.Bavail * uint64(stat.Bsize)
		}
	}

	info := map[string]interface{}{
		"version":        "0.1.0",
		"uptime_seconds": int(time.Since(startTime).Seconds()),
		"storage": map[string]interface{}{
			"total_objects": totalObjects,
			"total_bytes":   totalBytes,
			"buckets":       len(buckets),
			"disk_total":    diskTotal,
			"disk_free":     diskFree,
			"disk_used":     diskTotal - diskFree,
		},
		"system": map[string]interface{}{
			"cpu_cores":     runtime.NumCPU(),
			"goroutines":    runtime.NumGoroutine(),
			"ram_alloc":     m.Alloc,
			"ram_sys":       m.Sys,
			"ram_heap_sys":  m.HeapSys,
			"users_count":   len(users),
		},
		"config": map[string]interface{}{
			"port":          h.Config.Server.Port,
			"data_dir":      h.Config.Storage.DataDir,
			"max_size":      h.Config.Storage.MaxObjectSize,
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
		"bucket":  bucket,
		"objects": objects,
		"bytes":   bytes,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// RenameObject handles renaming (copy + delete) an object entirely server-side.
// This avoids presigned URL complications with encoding, Host headers, and SigV4.
func (h *AdminHandler) RenameObject(w http.ResponseWriter, r *http.Request) {
	if err := h.CheckAdminPolicy(r, "admin:RenameObject"); err != nil {
		WriteError(w, r, err)
		return
	}

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

	h.recordAudit(r, "RenameObject", req.Bucket+"/"+req.OldKey, "NewName: "+req.NewKey)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "renamed successfully",
		"oldKey":  req.OldKey,
		"newKey":  req.NewKey,
	})
}

func (h *AdminHandler) ListAuditLogs(w http.ResponseWriter, r *http.Request) {
	if err := h.CheckAdminPolicy(r, "admin:ListAuditLogs"); err != nil {
		WriteError(w, r, err)
		return
	}

	logs, err := h.MetaStore.ListAuditLogs(r.Context(), 100) // limit to 100 recent logs
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if logs == nil {
		logs = []metadata.AuditLog{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(logs)
}

func (h *AdminHandler) GetSettings(w http.ResponseWriter, r *http.Request) {
	if err := h.CheckAdminPolicy(r, "admin:GetSettings"); err != nil {
		WriteError(w, r, err)
		return
	}

	settings, err := h.MetaStore.ListSettings(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if settings == nil {
		settings = map[string]string{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(settings)
}

func (h *AdminHandler) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	if err := h.CheckAdminPolicy(r, "admin:UpdateSettings"); err != nil {
		WriteError(w, r, err)
		return
	}

	var req map[string]string
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	for k, v := range req {
		if err := h.MetaStore.PutSetting(r.Context(), k, v); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	h.recordAudit(r, "UpdateSettings", "global", "updated settings")
	w.WriteHeader(http.StatusOK)
}
