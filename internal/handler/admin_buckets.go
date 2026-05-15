package handler

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

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
		objects, bytes, _ := h.MetaStore.GetBucketStats(b.Name)
		res = append(res, map[string]interface{}{
			"name":         b.Name,
			"creationDate": b.CreationDate,
			"objects":      objects,
			"bytes":        bytes,
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

// DownloadFolder Admin API to download a folder as ZIP
func (h *AdminHandler) DownloadFolder(w http.ResponseWriter, r *http.Request) {
	if err := h.CheckAdminPolicy(r, "admin:DownloadFolder"); err != nil {
		WriteError(w, r, err)
		return
	}

	bucket := chi.URLParam(r, "bucket")
	prefix := r.URL.Query().Get("prefix")

	if prefix == "" {
		http.Error(w, "prefix is required", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/zip")
	fileName := strings.TrimSuffix(prefix, "/")
	if fileName == "" {
		fileName = bucket
	} else {
		parts := strings.Split(fileName, "/")
		fileName = parts[len(parts)-1]
	}
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s.zip\"", fileName))

	zw := zip.NewWriter(w)
	defer zw.Close()

	ctx := r.Context()
	marker := ""
	for {
		objects, _, nextMarker, err := h.MetaStore.ListObjects(bucket, prefix, "", marker, 1000)
		if err != nil {
			slog.Error("failed to list objects for zip", "error", err)
			return
		}

		for _, obj := range objects {
			if obj.Key == prefix {
				continue
			}

			zipPath := strings.TrimPrefix(obj.Key, prefix)
			f, err := zw.Create(zipPath)
			if err != nil {
				slog.Error("failed to create zip file entry", "error", err)
				return
			}

			objData, err := h.Backend.GetObject(ctx, bucket, obj.Key, storage.GetOptions{})
			if err != nil {
				slog.Error("failed to get object for zip", "error", err)
				return
			}

			if objData != nil && objData.Content != nil {
				_, err = io.Copy(f, objData.Content)
				objData.Content.Close()
				if err != nil {
					slog.Error("failed to copy object to zip", "error", err)
					return
				}
			}
		}

		if nextMarker == "" {
			break
		}
		marker = nextMarker
	}
	h.recordAudit(r, "DownloadFolder", bucket, "Prefix: "+prefix)
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

// GetBucketWebsite for Admin UI
func (h *AdminHandler) GetBucketWebsite(w http.ResponseWriter, r *http.Request) {
	if err := h.CheckAdminPolicy(r, "admin:GetBucketWebsite"); err != nil {
		WriteError(w, r, err)
		return
	}

	bucket := chi.URLParam(r, "bucket")
	website, err := h.MetaStore.GetBucketWebsite(r.Context(), bucket)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(website)
}

// PutBucketWebsite for Admin UI
func (h *AdminHandler) PutBucketWebsite(w http.ResponseWriter, r *http.Request) {
	if err := h.CheckAdminPolicy(r, "admin:PutBucketWebsite"); err != nil {
		WriteError(w, r, err)
		return
	}

	bucket := chi.URLParam(r, "bucket")

	var website s3.WebsiteConfiguration
	if err := json.NewDecoder(r.Body).Decode(&website); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := h.MetaStore.PutBucketWebsite(r.Context(), bucket, &website); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.recordAudit(r, "PutBucketWebsite", bucket, "")
	w.WriteHeader(http.StatusNoContent)
}

// DeleteBucketWebsite for Admin UI
func (h *AdminHandler) DeleteBucketWebsite(w http.ResponseWriter, r *http.Request) {
	if err := h.CheckAdminPolicy(r, "admin:DeleteBucketWebsite"); err != nil {
		WriteError(w, r, err)
		return
	}

	bucket := chi.URLParam(r, "bucket")
	if err := h.MetaStore.DeleteBucketWebsite(r.Context(), bucket); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.recordAudit(r, "DeleteBucketWebsite", bucket, "")
	w.WriteHeader(http.StatusNoContent)
}

// GetBucketCustomDomains for Admin UI
func (h *AdminHandler) GetBucketCustomDomains(w http.ResponseWriter, r *http.Request) {
	if err := h.CheckAdminPolicy(r, "admin:GetBucketWebsite"); err != nil {
		WriteError(w, r, err)
		return
	}

	bucket := chi.URLParam(r, "bucket")
	domains, err := h.MetaStore.GetBucketCustomDomains(r.Context(), bucket)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if domains == nil {
		domains = []string{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(domains)
}

// PutBucketCustomDomain for Admin UI
func (h *AdminHandler) PutBucketCustomDomain(w http.ResponseWriter, r *http.Request) {
	if err := h.CheckAdminPolicy(r, "admin:PutBucketWebsite"); err != nil {
		WriteError(w, r, err)
		return
	}

	bucket := chi.URLParam(r, "bucket")

	var req struct {
		Domain string `json:"domain"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.Domain == "" {
		http.Error(w, "domain is required", http.StatusBadRequest)
		return
	}

	if err := h.MetaStore.PutCustomDomain(r.Context(), req.Domain, bucket); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.recordAudit(r, "PutCustomDomain", bucket, req.Domain)
	w.WriteHeader(http.StatusNoContent)
}

// DeleteBucketCustomDomain for Admin UI
func (h *AdminHandler) DeleteBucketCustomDomain(w http.ResponseWriter, r *http.Request) {
	if err := h.CheckAdminPolicy(r, "admin:DeleteBucketWebsite"); err != nil {
		WriteError(w, r, err)
		return
	}

	bucket := chi.URLParam(r, "bucket")
	domain := chi.URLParam(r, "domain")

	if err := h.MetaStore.DeleteCustomDomain(r.Context(), domain); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.recordAudit(r, "DeleteCustomDomain", bucket, domain)
	w.WriteHeader(http.StatusNoContent)
}

// GetBucketNotification for Admin UI
func (h *AdminHandler) GetBucketNotification(w http.ResponseWriter, r *http.Request) {
	if err := h.CheckAdminPolicy(r, "admin:GetBucketNotification"); err != nil {
		WriteError(w, r, err)
		return
	}

	bucket := chi.URLParam(r, "bucket")
	notification, err := h.MetaStore.GetBucketNotification(r.Context(), bucket)
	if err != nil {
		// Return empty list instead of 404
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(s3.NotificationConfiguration{Webhooks: []s3.WebhookConfiguration{}})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(notification)
}

// PutBucketNotification for Admin UI
func (h *AdminHandler) PutBucketNotification(w http.ResponseWriter, r *http.Request) {
	if err := h.CheckAdminPolicy(r, "admin:PutBucketNotification"); err != nil {
		WriteError(w, r, err)
		return
	}

	bucket := chi.URLParam(r, "bucket")

	var config s3.NotificationConfiguration
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := h.MetaStore.PutBucketNotification(r.Context(), bucket, &config); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.recordAudit(r, "PutBucketNotification", bucket, "")
	w.WriteHeader(http.StatusNoContent)
}

// DeleteBucketNotification for Admin UI
func (h *AdminHandler) DeleteBucketNotification(w http.ResponseWriter, r *http.Request) {
	if err := h.CheckAdminPolicy(r, "admin:DeleteBucketNotification"); err != nil {
		WriteError(w, r, err)
		return
	}

	bucket := chi.URLParam(r, "bucket")

	if err := h.MetaStore.DeleteBucketNotification(r.Context(), bucket); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.recordAudit(r, "DeleteBucketNotification", bucket, "")
	w.WriteHeader(http.StatusNoContent)
}

// GetBucketVersioning for Admin UI
func (h *AdminHandler) GetBucketVersioning(w http.ResponseWriter, r *http.Request) {
	if err := h.CheckAdminPolicy(r, "admin:GetBucketVersioning"); err != nil {
		WriteError(w, r, err)
		return
	}

	bucket := chi.URLParam(r, "bucket")
	bInfo, err := h.MetaStore.GetBucket(bucket)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": bInfo.Versioning})
}

// PutBucketVersioning for Admin UI
func (h *AdminHandler) PutBucketVersioning(w http.ResponseWriter, r *http.Request) {
	if err := h.CheckAdminPolicy(r, "admin:PutBucketVersioning"); err != nil {
		WriteError(w, r, err)
		return
	}

	bucket := chi.URLParam(r, "bucket")
	var req struct {
		Status string `json:"status"` // "Enabled" or "Suspended"
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	bInfo, err := h.MetaStore.GetBucket(bucket)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	bInfo.Versioning = req.Status
	if err := h.MetaStore.UpdateBucket(bInfo); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.recordAudit(r, "PutBucketVersioning", bucket, req.Status)
	w.WriteHeader(http.StatusNoContent)
}
