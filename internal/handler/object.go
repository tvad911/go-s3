package handler

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"gos3/internal/auth"
	"gos3/internal/s3"
	"gos3/internal/storage"
	"gos3/internal/storage/local"
)

func (h *S3Handler) PutObject(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	bucket := chi.URLParam(r, "bucket")
	key := chi.URLParam(r, "key")

	meta := storage.ObjectMeta{
		ContentType:        r.Header.Get("Content-Type"),
		ContentEncoding:    r.Header.Get("Content-Encoding"),
		ContentDisposition: r.Header.Get("Content-Disposition"),
		ContentLanguage:    r.Header.Get("Content-Language"),
		CacheControl:       r.Header.Get("Cache-Control"),
		Expires:            r.Header.Get("Expires"),
		StorageClass:       r.Header.Get("x-amz-storage-class"),
		ACL:                auth.ParseACL(r),
		UserMeta:           make(map[string]string),
	}

	if meta.StorageClass == "" {
		meta.StorageClass = "STANDARD"
	}

	for k, v := range r.Header {
		if len(k) > 10 && strings.ToLower(k[:10]) == "x-amz-meta-" {
			meta.UserMeta[k] = v[0]
		}
	}

	var size int64
	if cl := r.Header.Get("Content-Length"); cl != "" {
		s, err := strconv.ParseInt(cl, 10, 64)
		if err == nil {
			size = s
		}
	}

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
	w.WriteHeader(http.StatusOK)
}

func (h *S3Handler) GetObject(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	bucket := chi.URLParam(r, "bucket")
	key := chi.URLParam(r, "key")

	opts := storage.GetOptions{}
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
		WriteError(w, r, err)
		return
	}
	defer obj.Content.Close()

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

	meta, err := h.Backend.HeadObject(ctx, bucket, key)
	if err != nil {
		WriteError(w, r, err)
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

	for k, v := range meta.UserMeta {
		w.Header().Set(k, v)
	}

	w.WriteHeader(http.StatusOK)
}

func (h *S3Handler) DeleteObject(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	bucket := chi.URLParam(r, "bucket")
	key := chi.URLParam(r, "key")

	if err := h.Backend.DeleteObject(ctx, bucket, key); err != nil {
		WriteError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *S3Handler) DeleteObjects(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	bucket := chi.URLParam(r, "bucket")

	var req s3.Delete
	if err := xml.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, r, s3.ErrMalformedXML)
		return
	}

	var keys []string
	for _, obj := range req.Objects {
		keys = append(keys, obj.Key)
	}

	result, err := h.Backend.DeleteObjects(ctx, bucket, keys)
	if err != nil {
		WriteError(w, r, err)
		return
	}

	res := s3.DeleteResult{}
	for _, k := range result.Deleted {
		if !req.Quiet {
			res.Deleted = append(res.Deleted, s3.DeletedItem{Key: k})
		}
	}
	for _, e := range result.Errors {
		res.Error = append(res.Error, s3.DeleteError{
			Key:     e.Key,
			Code:    e.Code,
			Message: e.Message,
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
			UserMeta:           make(map[string]string),
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

	_, err := h.Backend.HeadObject(ctx, bucket, key)
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
