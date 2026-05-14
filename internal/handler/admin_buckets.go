package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"gos3/internal/auth"
	"gos3/internal/s3"
	"gos3/internal/storage"
)

// ListBuckets for Admin UI (returns JSON instead of XML)
func (h *AdminHandler) ListBuckets(w http.ResponseWriter, r *http.Request) {
	if err := h.CheckAdminPolicy(r, "admin:ListBuckets"); err != nil {
		WriteError(w, r, err)
		return
	}

	buckets, err := h.MetaStore.ListBuckets()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Convert to JSON friendly format
	res := make([]map[string]interface{}, 0)
	for _, b := range buckets {
		res = append(res, map[string]interface{}{
			"name":         b.Name,
			"creationDate": b.CreationDate,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(res)
}

// CreateBucket for Admin UI
func (h *AdminHandler) CreateBucket(w http.ResponseWriter, r *http.Request) {
	if err := h.CheckAdminPolicy(r, "admin:CreateBucket"); err != nil {
		WriteError(w, r, err)
		return
	}

	bucket := chi.URLParam(r, "bucket")
	// For web console, owner is empty or root
	err := h.MetaStore.CreateBucket(bucket, "us-east-1", "", "", false)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.recordAudit(r, "CreateBucket", bucket, "")
	w.WriteHeader(http.StatusCreated)
}

// DeleteBucket for Admin UI
func (h *AdminHandler) DeleteBucket(w http.ResponseWriter, r *http.Request) {
	if err := h.CheckAdminPolicy(r, "admin:DeleteBucket"); err != nil {
		WriteError(w, r, err)
		return
	}

	bucket := chi.URLParam(r, "bucket")
	err := h.MetaStore.DeleteBucket(bucket)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.recordAudit(r, "DeleteBucket", bucket, "")
	w.WriteHeader(http.StatusNoContent)
}

// ListObjects for Admin UI (JSON format of ListObjectsV2)
func (h *AdminHandler) ListObjects(w http.ResponseWriter, r *http.Request) {
	if err := h.CheckAdminPolicy(r, "admin:ListObjects"); err != nil {
		WriteError(w, r, err)
		return
	}

	bucket := chi.URLParam(r, "bucket")
	prefix := r.URL.Query().Get("prefix")
	delimiter := r.URL.Query().Get("delimiter")
	marker := r.URL.Query().Get("marker")

	maxKeys := 1000
	if mk := r.URL.Query().Get("maxKeys"); mk != "" {
		if parsed, err := strconv.Atoi(mk); err == nil && parsed > 0 {
			maxKeys = parsed
		}
	}

	objects, prefixes, nextMarker, err := h.MetaStore.ListObjects(bucket, prefix, delimiter, marker, maxKeys)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var resContents []map[string]interface{}
	for _, obj := range objects {
		resContents = append(resContents, map[string]interface{}{
			"key":          obj.Key,
			"size":         obj.Size,
			"lastModified": obj.LastModified,
		})
	}

	res := map[string]interface{}{
		"contents":       resContents,
		"commonPrefixes": prefixes,
		"isTruncated":    nextMarker != "",
		"nextMarker":     nextMarker,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(res)
}

// GetBucketPolicy for Admin UI
func (h *AdminHandler) GetBucketPolicy(w http.ResponseWriter, r *http.Request) {
	if err := h.CheckAdminPolicy(r, "admin:GetBucketPolicy"); err != nil {
		WriteError(w, r, err)
		return
	}

	bucket := chi.URLParam(r, "bucket")
	policy, err := h.MetaStore.GetBucketPolicy(r.Context(), bucket)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(policy)
}

// PutBucketPolicy for Admin UI
func (h *AdminHandler) PutBucketPolicy(w http.ResponseWriter, r *http.Request) {
	if err := h.CheckAdminPolicy(r, "admin:PutBucketPolicy"); err != nil {
		WriteError(w, r, err)
		return
	}

	bucket := chi.URLParam(r, "bucket")

	var policy auth.Policy
	if err := json.NewDecoder(r.Body).Decode(&policy); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := h.MetaStore.PutBucketPolicy(r.Context(), bucket, &policy); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.recordAudit(r, "PutBucketPolicy", bucket, "")
	w.WriteHeader(http.StatusNoContent)
}

// DeleteBucketPolicy for Admin UI
func (h *AdminHandler) DeleteBucketPolicy(w http.ResponseWriter, r *http.Request) {
	if err := h.CheckAdminPolicy(r, "admin:DeleteBucketPolicy"); err != nil {
		WriteError(w, r, err)
		return
	}

	bucket := chi.URLParam(r, "bucket")
	if err := h.MetaStore.DeleteBucketPolicy(r.Context(), bucket); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.recordAudit(r, "DeleteBucketPolicy", bucket, "")
	w.WriteHeader(http.StatusNoContent)
}

// GetBucketCORS for Admin UI
func (h *AdminHandler) GetBucketCORS(w http.ResponseWriter, r *http.Request) {
	if err := h.CheckAdminPolicy(r, "admin:GetBucketCORS"); err != nil {
		WriteError(w, r, err)
		return
	}

	bucket := chi.URLParam(r, "bucket")
	cors, err := h.MetaStore.GetBucketCORS(r.Context(), bucket)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(cors)
}

// PutBucketCORS for Admin UI
func (h *AdminHandler) PutBucketCORS(w http.ResponseWriter, r *http.Request) {
	if err := h.CheckAdminPolicy(r, "admin:PutBucketCORS"); err != nil {
		WriteError(w, r, err)
		return
	}

	bucket := chi.URLParam(r, "bucket")

	var cors s3.CORSConfiguration
	if err := json.NewDecoder(r.Body).Decode(&cors); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := h.MetaStore.PutBucketCORS(r.Context(), bucket, &cors); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.recordAudit(r, "PutBucketCORS", bucket, "")
	w.WriteHeader(http.StatusNoContent)
}

// DeleteBucketCORS for Admin UI
func (h *AdminHandler) DeleteBucketCORS(w http.ResponseWriter, r *http.Request) {
	if err := h.CheckAdminPolicy(r, "admin:DeleteBucketCORS"); err != nil {
		WriteError(w, r, err)
		return
	}

	bucket := chi.URLParam(r, "bucket")
	if err := h.MetaStore.DeleteBucketCORS(r.Context(), bucket); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.recordAudit(r, "DeleteBucketCORS", bucket, "")
	w.WriteHeader(http.StatusNoContent)
}

// DeleteObjects Admin API for bulk delete
func (h *AdminHandler) DeleteObjects(w http.ResponseWriter, r *http.Request) {
	if err := h.CheckAdminPolicy(r, "admin:DeleteObjects"); err != nil {
		WriteError(w, r, err)
		return
	}

	bucket := chi.URLParam(r, "bucket")

	var keys []string
	if err := json.NewDecoder(r.Body).Decode(&keys); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var objIds []storage.ObjectIdentifier
	for _, key := range keys {
		objIds = append(objIds, storage.ObjectIdentifier{Key: key})
	}

	if _, err := h.Backend.DeleteObjects(r.Context(), bucket, objIds); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.recordAudit(r, "DeleteObjects", bucket, fmt.Sprintf("Deleted %d objects", len(keys)))
	w.WriteHeader(http.StatusNoContent)
}

// GetBucketLifecycle for Admin UI
func (h *AdminHandler) GetBucketLifecycle(w http.ResponseWriter, r *http.Request) {
	if err := h.CheckAdminPolicy(r, "admin:GetBucketLifecycle"); err != nil {
		WriteError(w, r, err)
		return
	}

	bucket := chi.URLParam(r, "bucket")
	lc, err := h.MetaStore.GetBucketLifecycle(r.Context(), bucket)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(lc)
}

// PutBucketLifecycle for Admin UI
func (h *AdminHandler) PutBucketLifecycle(w http.ResponseWriter, r *http.Request) {
	if err := h.CheckAdminPolicy(r, "admin:PutBucketLifecycle"); err != nil {
		WriteError(w, r, err)
		return
	}

	bucket := chi.URLParam(r, "bucket")

	var lc s3.LifecycleConfiguration
	if err := json.NewDecoder(r.Body).Decode(&lc); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := h.MetaStore.PutBucketLifecycle(r.Context(), bucket, &lc); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.recordAudit(r, "PutBucketLifecycle", bucket, "")
	w.WriteHeader(http.StatusNoContent)
}

// DeleteBucketLifecycle for Admin UI
func (h *AdminHandler) DeleteBucketLifecycle(w http.ResponseWriter, r *http.Request) {
	if err := h.CheckAdminPolicy(r, "admin:DeleteBucketLifecycle"); err != nil {
		WriteError(w, r, err)
		return
	}

	bucket := chi.URLParam(r, "bucket")
	if err := h.MetaStore.DeleteBucketLifecycle(r.Context(), bucket); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.recordAudit(r, "DeleteBucketLifecycle", bucket, "")
	w.WriteHeader(http.StatusNoContent)
}
