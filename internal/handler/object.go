package handler

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"gos3/internal/auth"
	"gos3/internal/s3"
	"gos3/internal/storage"
	"gos3/internal/storage/local"
)

func (h *S3Handler) PutObject(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	bucket := chi.URLParam(r, "bucket")
	key := chi.URLParam(r, "key")

	if len(key) > 1024 {
		WriteError(w, r, s3.ErrKeyTooLongError)
		return
	}

	meta := storage.ObjectMeta{
		ContentType:        r.Header.Get("Content-Type"),
		ContentEncoding:    r.Header.Get("Content-Encoding"),
		ContentDisposition: r.Header.Get("Content-Disposition"),
		ContentLanguage:    r.Header.Get("Content-Language"),
		CacheControl:       r.Header.Get("Cache-Control"),
		Expires:            r.Header.Get("Expires"),
		StorageClass:       r.Header.Get("x-amz-storage-class"),
		ServerSideEncryption: r.Header.Get("x-amz-server-side-encryption"),
		ObjectLockMode:       r.Header.Get("x-amz-object-lock-mode"),
		ObjectLockLegalHoldStatus: r.Header.Get("x-amz-object-lock-legal-hold"),
		ACL:                auth.ParseACL(r),
		UserMeta:           make(map[string]string),
	}

	if retainDate := r.Header.Get("x-amz-object-lock-retain-until-date"); retainDate != "" {
		if t, err := time.Parse(time.RFC3339, retainDate); err == nil {
			meta.ObjectLockRetainUntilDate = &t
		}
	}

	if meta.StorageClass == "" {
		meta.StorageClass = "STANDARD"
	}
	if !s3.IsValidStorageClass(meta.StorageClass) {
		WriteError(w, r, s3.ErrInvalidStorageClass)
		return
	}

	for k, v := range r.Header {
		if len(k) > 10 && strings.ToLower(k[:10]) == "x-amz-meta-" {
			meta.UserMeta[k] = v[0]
		}
	}

	var size int64
	cl := r.Header.Get("Content-Length")
	if cl == "" {
		WriteError(w, r, s3.ErrMissingContentLength)
		return
	}
	s, err := strconv.ParseInt(cl, 10, 64)
	if err != nil || s < 0 {
		WriteError(w, r, s3.ErrMissingContentLength)
		return
	}
	size = s

	if h.Config != nil && size > h.Config.Storage.MaxObjectSize {
		WriteError(w, r, s3.ErrEntityTooLarge)
		return
	}

	bInfo, err := h.MetaStore.GetBucket(bucket)
	if err == nil {
		if bInfo.Versioning == "Enabled" {
			meta.VersionID = uuid.New().String()
		} else if bInfo.Versioning == "Suspended" {
			meta.VersionID = "null"
		}
	}
	meta.IsLatest = true

	var body io.Reader = r.Body
	if local.IsAWSChunked(r.Header.Get("x-amz-content-sha256")) {
		body = local.NewAWSChunkedReader(body)
	}

	res, err := h.Backend.PutObject(ctx, bucket, key, body, size, meta)
	if err != nil {
		WriteError(w, r, err)
		return
	}

	w.Header().Set("ETag", `"`+res.ETag+`"`)
	if res.VersionID != "" && res.VersionID != "null" {
		w.Header().Set("x-amz-version-id", res.VersionID)
	}
	if meta.ServerSideEncryption != "" {
		w.Header().Set("x-amz-server-side-encryption", meta.ServerSideEncryption)
	}

	w.WriteHeader(http.StatusOK)
}

func (h *S3Handler) GetObject(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	bucket := chi.URLParam(r, "bucket")
	key := chi.URLParam(r, "key")

	opts := storage.GetOptions{
		VersionID: r.URL.Query().Get("versionId"),
	}
	if rng := r.Header.Get("Range"); rng != "" {
		// Basic range parsing (bytes=start-end)
		if strings.HasPrefix(rng, "bytes=") {
			parts := strings.Split(rng[6:], "-")
			if len(parts) == 2 {
				if parts[0] != "" {
					start, err := strconv.ParseInt(parts[0], 10, 64)
					if err == nil {
						opts.RangeStart = &start
					}
				}
				if parts[1] != "" {
					end, err := strconv.ParseInt(parts[1], 10, 64)
					if err == nil {
						opts.RangeEnd = &end
					}
				}
			}
		}
	}

	opts.IfMatch = r.Header.Get("If-Match")
	opts.IfNoneMatch = r.Header.Get("If-None-Match")

	if ims := r.Header.Get("If-Modified-Since"); ims != "" {
		if t, err := http.ParseTime(ims); err == nil {
			opts.IfModifiedSince = t
		}
	}
	if ius := r.Header.Get("If-Unmodified-Since"); ius != "" {
		if t, err := http.ParseTime(ius); err == nil {
			opts.IfUnmodifiedSince = t
		}
	}

	obj, err := h.Backend.GetObject(ctx, bucket, key, opts)
	if err != nil {
		if errors.Is(err, s3.ErrNoSuchKey) && strings.Contains(r.Header.Get("Accept"), "text/html") {
			website, wErr := h.MetaStore.GetBucketWebsite(ctx, bucket)
			if wErr == nil && website != nil {
				if strings.HasSuffix(key, "/") && website.IndexDocument.Suffix != "" {
					idxKey := key + website.IndexDocument.Suffix
					if idxObj, idxErr := h.Backend.GetObject(ctx, bucket, idxKey, opts); idxErr == nil {
						obj = idxObj
						err = nil
					}
				}
				if err != nil && website.ErrorDocument.Key != "" {
					if errObj, errErr := h.Backend.GetObject(ctx, bucket, website.ErrorDocument.Key, opts); errErr == nil {
						w.Header().Set("Content-Type", errObj.ContentType)
						w.WriteHeader(http.StatusNotFound)
						io.Copy(w, errObj.Content)
						errObj.Content.Close()
						return
					}
				}
			}
		}
		if err != nil {
			WriteError(w, r, err)
			return
		}
	}
	defer obj.Content.Close()

	if obj.VersionID != "" && obj.VersionID != "null" {
		w.Header().Set("x-amz-version-id", obj.VersionID)
	}

	if obj.IsDeleteMarker {
		w.Header().Set("x-amz-delete-marker", "true")
		w.WriteHeader(http.StatusNotFound)
		return
	}

	if opts.IfMatch != "" && `"`+obj.ETag+`"` != opts.IfMatch {
		w.WriteHeader(http.StatusPreconditionFailed)
		return
	}
	if opts.IfNoneMatch != "" && `"`+obj.ETag+`"` == opts.IfNoneMatch {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	if !opts.IfModifiedSince.IsZero() && obj.LastModified.Before(opts.IfModifiedSince.Add(1*time.Second)) {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	if !opts.IfUnmodifiedSince.IsZero() && obj.LastModified.After(opts.IfUnmodifiedSince) {
		w.WriteHeader(http.StatusPreconditionFailed)
		return
	}

	w.Header().Set("ETag", `"`+obj.ETag+`"`)
	w.Header().Set("Last-Modified", obj.LastModified.Format(http.TimeFormat))
	w.Header().Set("Accept-Ranges", "bytes")

	if obj.ContentType != "" {
		w.Header().Set("Content-Type", obj.ContentType)
	}
	if obj.ContentEncoding != "" {
		w.Header().Set("Content-Encoding", obj.ContentEncoding)
	}
	if obj.ContentDisposition != "" {
		w.Header().Set("Content-Disposition", obj.ContentDisposition)
	}
	if obj.ContentLanguage != "" {
		w.Header().Set("Content-Language", obj.ContentLanguage)
	}
	if obj.CacheControl != "" {
		w.Header().Set("Cache-Control", obj.CacheControl)
	}
	if obj.Expires != "" {
		w.Header().Set("Expires", obj.Expires)
	}
	w.Header().Set("x-amz-storage-class", obj.StorageClass)
	if obj.ServerSideEncryption != "" {
		w.Header().Set("x-amz-server-side-encryption", obj.ServerSideEncryption)
	}
	if obj.ObjectLockMode != "" {
		w.Header().Set("x-amz-object-lock-mode", obj.ObjectLockMode)
	}
	if obj.ObjectLockRetainUntilDate != nil {
		w.Header().Set("x-amz-object-lock-retain-until-date", obj.ObjectLockRetainUntilDate.Format(time.RFC3339))
	}
	if obj.ObjectLockLegalHoldStatus != "" {
		w.Header().Set("x-amz-object-lock-legal-hold", obj.ObjectLockLegalHoldStatus)
	}

	for k, v := range obj.UserMeta {
		w.Header().Set(k, v)
	}

	if opts.RangeStart != nil || opts.RangeEnd != nil {
		w.Header().Set("Content-Length", strconv.FormatInt(obj.RangeEnd-obj.RangeStart+1, 10))
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", obj.RangeStart, obj.RangeEnd, obj.Size))
		w.WriteHeader(http.StatusPartialContent)
	} else {
		w.Header().Set("Content-Length", strconv.FormatInt(obj.Size, 10))
		w.WriteHeader(http.StatusOK)
	}

	io.Copy(w, obj.Content)
}

func (h *S3Handler) HeadObject(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	bucket := chi.URLParam(r, "bucket")
	key := chi.URLParam(r, "key")

	opts := storage.GetOptions{
		VersionID: r.URL.Query().Get("versionId"),
	}
	meta, err := h.Backend.HeadObject(ctx, bucket, key, opts)
	if err != nil {
		WriteError(w, r, err)
		return
	}

	if meta.VersionID != "" && meta.VersionID != "null" {
		w.Header().Set("x-amz-version-id", meta.VersionID)
	}

	if meta.IsDeleteMarker {
		w.Header().Set("x-amz-delete-marker", "true")
		w.WriteHeader(http.StatusNotFound)
		return
	}

	w.Header().Set("ETag", `"`+meta.ETag+`"`)
	w.Header().Set("Last-Modified", meta.LastModified.Format(http.TimeFormat))
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Content-Length", strconv.FormatInt(meta.Size, 10))

	if meta.ContentType != "" {
		w.Header().Set("Content-Type", meta.ContentType)
	}
	if meta.ContentEncoding != "" {
		w.Header().Set("Content-Encoding", meta.ContentEncoding)
	}
	if meta.ContentDisposition != "" {
		w.Header().Set("Content-Disposition", meta.ContentDisposition)
	}
	if meta.ContentLanguage != "" {
		w.Header().Set("Content-Language", meta.ContentLanguage)
	}
	if meta.CacheControl != "" {
		w.Header().Set("Cache-Control", meta.CacheControl)
	}
	if meta.Expires != "" {
		w.Header().Set("Expires", meta.Expires)
	}
	w.Header().Set("x-amz-storage-class", meta.StorageClass)
	if meta.ServerSideEncryption != "" {
		w.Header().Set("x-amz-server-side-encryption", meta.ServerSideEncryption)
	}
	if meta.ObjectLockMode != "" {
		w.Header().Set("x-amz-object-lock-mode", meta.ObjectLockMode)
	}
	if meta.ObjectLockRetainUntilDate != nil {
		w.Header().Set("x-amz-object-lock-retain-until-date", meta.ObjectLockRetainUntilDate.Format(time.RFC3339))
	}
	if meta.ObjectLockLegalHoldStatus != "" {
		w.Header().Set("x-amz-object-lock-legal-hold", meta.ObjectLockLegalHoldStatus)
	}

	for k, v := range meta.UserMeta {
		w.Header().Set(k, v)
	}

	w.WriteHeader(http.StatusOK)
}

func (h *S3Handler) DeleteObject(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	bucket := chi.URLParam(r, "bucket")
	key := chi.URLParam(r, "key")

	versionId := r.URL.Query().Get("versionId")

	if err := h.Backend.DeleteObject(ctx, bucket, key, versionId); err != nil {
		WriteError(w, r, err)
		return
	}

	// For accurate delete marker response, we could return x-amz-delete-marker: true,
	// but to keep it simple, we just return the version id deleted.
	if versionId != "" {
		w.Header().Set("x-amz-version-id", versionId)
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *S3Handler) DeleteObjects(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	bucket := chi.URLParam(r, "bucket")

	var req s3.Delete
	if err := xml.NewDecoder(io.LimitReader(r.Body, 2<<20)).Decode(&req); err != nil {
		WriteError(w, r, s3.ErrMalformedXML)
		return
	}

	var keys []storage.ObjectIdentifier
	for _, obj := range req.Objects {
		keys = append(keys, storage.ObjectIdentifier{
			Key:       obj.Key,
			VersionID: obj.VersionId,
		})
	}

	result, err := h.Backend.DeleteObjects(ctx, bucket, keys)
	if err != nil {
		WriteError(w, r, err)
		return
	}

	res := s3.DeleteResult{}
	for _, k := range result.Deleted {
		if !req.Quiet {
			res.Deleted = append(res.Deleted, s3.DeletedItem{
				Key:       k.Key,
				VersionId: k.VersionID,
				// For a full implementation, we'd check if a DeleteMarker was created 
				// and set DeleteMarker/DeleteMarkerVersionId accordingly.
				// Since we just return k.VersionID, we use it directly.
			})
		}
	}
	for _, e := range result.Errors {
		res.Error = append(res.Error, s3.DeleteError{
			Key:       e.Key,
			VersionId: e.VersionID,
			Code:      e.Code,
			Message:   e.Message,
		})
	}

	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(xml.Header))
	xml.NewEncoder(w).Encode(res)
}

func (h *S3Handler) CopyObject(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	bucket := chi.URLParam(r, "bucket")
	key := chi.URLParam(r, "key")

	copySource := r.Header.Get("x-amz-copy-source")
	if copySource == "" {
		WriteError(w, r, s3.ErrInvalidCopySource)
		return
	}

	// copySource is typically "/bucket/key" or "bucket/key"
	copySource = strings.TrimPrefix(copySource, "/")
	parts := strings.SplitN(copySource, "/", 2)
	if len(parts) != 2 {
		WriteError(w, r, s3.ErrInvalidCopySource)
		return
	}
	srcBucket, srcKey := parts[0], parts[1]

	var meta *storage.ObjectMeta
	if r.Header.Get("x-amz-metadata-directive") == "REPLACE" {
		m := storage.ObjectMeta{
			ContentType:        r.Header.Get("Content-Type"),
			ContentEncoding:    r.Header.Get("Content-Encoding"),
			ContentDisposition: r.Header.Get("Content-Disposition"),
			ContentLanguage:    r.Header.Get("Content-Language"),
			CacheControl:       r.Header.Get("Cache-Control"),
			Expires:            r.Header.Get("Expires"),
			StorageClass:       r.Header.Get("x-amz-storage-class"),
			ServerSideEncryption: r.Header.Get("x-amz-server-side-encryption"),
			ObjectLockMode:       r.Header.Get("x-amz-object-lock-mode"),
			ObjectLockLegalHoldStatus: r.Header.Get("x-amz-object-lock-legal-hold"),
			UserMeta:           make(map[string]string),
		}
		if m.StorageClass == "" {
			m.StorageClass = "STANDARD"
		}
		if !s3.IsValidStorageClass(m.StorageClass) {
			WriteError(w, r, s3.ErrInvalidStorageClass)
			return
		}
		if retainDate := r.Header.Get("x-amz-object-lock-retain-until-date"); retainDate != "" {
			if t, err := time.Parse(time.RFC3339, retainDate); err == nil {
				m.ObjectLockRetainUntilDate = &t
			}
		}
		for k, v := range r.Header {
			if len(k) > 10 && strings.ToLower(k[:10]) == "x-amz-meta-" {
				m.UserMeta[k] = v[0]
			}
		}
		meta = &m
	}

	result, err := h.Backend.CopyObject(ctx, srcBucket, srcKey, bucket, key, meta)
	if err != nil {
		WriteError(w, r, err)
		return
	}

	res := s3.CopyObjectResult{
		LastModified: result.LastModified,
		ETag:         `"` + result.ETag + `"`,
	}

	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(xml.Header))
	xml.NewEncoder(w).Encode(res)
}

// GetObjectAcl handles GET /bucket/key?acl
func (h *S3Handler) GetObjectAcl(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	bucket := chi.URLParam(r, "bucket")
	key := chi.URLParam(r, "key")

	opts := storage.GetOptions{
		VersionID: r.URL.Query().Get("versionId"),
	}
	_, err := h.Backend.HeadObject(ctx, bucket, key, opts)
	if err != nil {
		WriteError(w, r, err)
		return
	}

	res := s3.AccessControlPolicy{
		Owner: s3.Owner{
			ID:          "admin", // Stub
			DisplayName: "admin",
		},
	}
	res.AccessControlList.Grant = []s3.Grant{
		{
			Grantee: s3.Grantee{
				XMLNamespace: "http://www.w3.org/2001/XMLSchema-instance",
				XsiType:      "CanonicalUser",
				ID:           "admin",
				DisplayName:  "admin",
			},
			Permission: "FULL_CONTROL",
		},
	}

	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(xml.Header))
	xml.NewEncoder(w).Encode(res)
}

// PutObjectAcl handles PUT /bucket/key?acl
func (h *S3Handler) PutObjectAcl(w http.ResponseWriter, r *http.Request) {
	WriteError(w, r, s3.ErrNotImplemented)
}

// ListObjectVersions handles GET /bucket?versions
func (h *S3Handler) ListObjectVersions(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	bucket := chi.URLParam(r, "bucket")

	q := r.URL.Query()

	maxKeys := 1000
	if mk := q.Get("max-keys"); mk != "" {
		if v, err := strconv.Atoi(mk); err == nil && v > 0 {
			maxKeys = v
		}
	}

	opts := storage.ListVersionsOptions{
		Prefix:          q.Get("prefix"),
		Delimiter:       q.Get("delimiter"),
		KeyMarker:       q.Get("key-marker"),
		VersionIdMarker: q.Get("version-id-marker"),
		MaxKeys:         maxKeys,
	}

	result, err := h.Backend.ListObjectVersions(ctx, bucket, opts)
	if err != nil {
		WriteError(w, r, err)
		return
	}

	res := s3.ListVersionsResult{
		Name:                bucket,
		Prefix:              opts.Prefix,
		KeyMarker:           opts.KeyMarker,
		VersionIdMarker:     opts.VersionIdMarker,
		MaxKeys:             opts.MaxKeys,
		Delimiter:           opts.Delimiter,
		IsTruncated:         result.IsTruncated,
		NextKeyMarker:       result.NextKeyMarker,
		NextVersionIdMarker: result.NextVersionIdMarker,
	}

	for _, obj := range result.Objects {
		res.Version = append(res.Version, s3.ObjectVersion{
			Key:          obj.Key,
			VersionId:    obj.VersionID,
			IsLatest:     obj.IsLatest,
			LastModified: obj.LastModified,
			ETag:         `"` + obj.ETag + `"`,
			Size:         obj.Size,
			StorageClass: obj.StorageClass,
			Owner: &s3.Owner{
				ID:          "admin",
				DisplayName: "admin",
			},
		})
	}

	for _, dm := range result.DeleteMarkers {
		res.DeleteMarker = append(res.DeleteMarker, s3.ObjectVersion{
			Key:          dm.Key,
			VersionId:    dm.VersionID,
			IsLatest:     dm.IsLatest,
			LastModified: dm.LastModified,
			Owner: &s3.Owner{
				ID:          "admin",
				DisplayName: "admin",
			},
		})
	}

	for _, p := range result.CommonPrefixes {
		res.CommonPrefixes = append(res.CommonPrefixes, s3.CommonPrefix{Prefix: p})
	}

	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(xml.Header))
	xml.NewEncoder(w).Encode(res)
}
