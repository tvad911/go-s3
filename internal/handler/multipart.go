package handler

import (
	"encoding/xml"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"gos3/internal/auth"
	"gos3/internal/s3"
	"gos3/internal/storage"
	"gos3/internal/storage/local"
)

func (h *S3Handler) CreateMultipartUpload(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	bucket := chi.URLParam(r, "bucket")
	key := getObjectKey(r)

	if len(key) > 1024 {
		WriteError(w, r, s3.ErrKeyTooLongError)
		return
	}

	meta := storage.ObjectMeta{
		ContentType:               r.Header.Get("Content-Type"),
		ContentEncoding:           r.Header.Get("Content-Encoding"),
		ContentDisposition:        r.Header.Get("Content-Disposition"),
		ContentLanguage:           r.Header.Get("Content-Language"),
		CacheControl:              r.Header.Get("Cache-Control"),
		Expires:                   r.Header.Get("Expires"),
		StorageClass:              r.Header.Get("x-amz-storage-class"),
		ServerSideEncryption:      r.Header.Get("x-amz-server-side-encryption"),
		ObjectLockMode:            r.Header.Get("x-amz-object-lock-mode"),
		ObjectLockLegalHoldStatus: r.Header.Get("x-amz-object-lock-legal-hold"),
		ACL:                       auth.ParseACL(r),
		UserMeta:                  make(map[string]string),
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
		if len(k) > 10 && k[:10] == "x-amz-meta" {
			meta.UserMeta[k] = v[0]
		}
	}

	uploadID, err := h.Backend.CreateMultipartUpload(ctx, bucket, key, meta)
	if err != nil {
		WriteError(w, r, err)
		return
	}

	res := s3.InitiateMultipartUploadResult{
		Bucket:   bucket,
		Key:      key,
		UploadId: uploadID,
	}

	if meta.ServerSideEncryption != "" {
		w.Header().Set("x-amz-server-side-encryption", meta.ServerSideEncryption)
	}

	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(xml.Header))
	xml.NewEncoder(w).Encode(res)
}

func (h *S3Handler) UploadPart(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	bucket := chi.URLParam(r, "bucket")
	key := getObjectKey(r)

	uploadID := r.URL.Query().Get("uploadId")
	partNum, err := strconv.Atoi(r.URL.Query().Get("partNumber"))
	if err != nil || partNum < 1 || partNum > 10000 {
		WriteError(w, r, s3.ErrInvalidPart)
		return
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

	var body io.Reader = r.Body
	if local.IsAWSChunked(r.Header.Get("x-amz-content-sha256")) {
		body = local.NewAWSChunkedReader(body)
	}

	part, err := h.Backend.UploadPart(ctx, bucket, key, uploadID, partNum, body, size)
	if err != nil {
		WriteError(w, r, err)
		return
	}

	w.Header().Set("ETag", `"`+part.ETag+`"`)

	if meta, err := h.MetaStore.GetMultipartUpload(bucket, key, uploadID); err == nil && meta.ServerSideEncryption != "" {
		w.Header().Set("x-amz-server-side-encryption", meta.ServerSideEncryption)
	}

	w.WriteHeader(http.StatusOK)
}

func (h *S3Handler) CompleteMultipartUpload(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	bucket := chi.URLParam(r, "bucket")
	key := getObjectKey(r)

	uploadID := r.URL.Query().Get("uploadId")

	var req s3.CompleteMultipartUpload
	if err := xml.NewDecoder(io.LimitReader(r.Body, 2<<20)).Decode(&req); err != nil {
		WriteError(w, r, s3.ErrMalformedXML)
		return
	}

	var parts []storage.CompletePart
	for _, p := range req.Parts {
		parts = append(parts, storage.CompletePart{
			PartNumber: p.PartNumber,
			ETag:       p.ETag, // Could contain quotes from some clients
		})
	}

	result, err := h.Backend.CompleteMultipartUpload(ctx, bucket, key, uploadID, parts)
	if err != nil {
		WriteError(w, r, err)
		return
	}

	res := s3.CompleteMultipartUploadResult{
		Location: "http://" + r.Host + "/" + bucket + "/" + key,
		Bucket:   bucket,
		Key:      key,
		ETag:     `"` + result.ETag + `"`,
	}

	if result.VersionID != "" && result.VersionID != "null" {
		w.Header().Set("x-amz-version-id", result.VersionID)
	}

	if meta, err := h.MetaStore.GetMultipartUpload(bucket, key, uploadID); err == nil && meta.ServerSideEncryption != "" {
		w.Header().Set("x-amz-server-side-encryption", meta.ServerSideEncryption)
	}

	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(xml.Header))
	xml.NewEncoder(w).Encode(res)

	// Trigger webhook
	h.fireWebhooks(ctx, "s3:ObjectCreated:CompleteMultipartUpload", bucket, key, 0, result.ETag)
}

func (h *S3Handler) AbortMultipartUpload(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	bucket := chi.URLParam(r, "bucket")
	key := getObjectKey(r)

	uploadID := r.URL.Query().Get("uploadId")

	if err := h.Backend.AbortMultipartUpload(ctx, bucket, key, uploadID); err != nil {
		WriteError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *S3Handler) ListParts(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	bucket := chi.URLParam(r, "bucket")
	key := getObjectKey(r)

	uploadID := r.URL.Query().Get("uploadId")

	opts := storage.ListPartsOptions{}
	if m := r.URL.Query().Get("part-number-marker"); m != "" {
		if v, err := strconv.Atoi(m); err == nil {
			opts.PartNumberMarker = v
		}
	}
	opts.MaxParts = 1000
	if m := r.URL.Query().Get("max-parts"); m != "" {
		if v, err := strconv.Atoi(m); err == nil && v > 0 {
			opts.MaxParts = v
		}
	}

	result, err := h.Backend.ListParts(ctx, bucket, key, uploadID, opts)
	if err != nil {
		WriteError(w, r, err)
		return
	}

	res := s3.ListPartsResult{
		Bucket:               bucket,
		Key:                  key,
		UploadId:             uploadID,
		PartNumberMarker:     opts.PartNumberMarker,
		NextPartNumberMarker: result.NextPartNumberMarker,
		MaxParts:             opts.MaxParts,
		IsTruncated:          result.IsTruncated,
		Initiator: s3.Owner{
			ID:          "admin",
			DisplayName: "admin",
		},
		Owner: s3.Owner{
			ID:          "admin",
			DisplayName: "admin",
		},
	}

	for _, p := range result.Parts {
		res.Parts = append(res.Parts, s3.PartInfo{
			PartNumber:   p.PartNumber,
			LastModified: p.LastModified,
			ETag:         `"` + p.ETag + `"`,
			Size:         p.Size,
		})
	}

	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(xml.Header))
	xml.NewEncoder(w).Encode(res)
}

func (h *S3Handler) ListMultipartUploads(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	bucket := chi.URLParam(r, "bucket")

	q := r.URL.Query()
	opts := storage.ListUploadsOptions{
		Prefix:         q.Get("prefix"),
		Delimiter:      q.Get("delimiter"),
		KeyMarker:      q.Get("key-marker"),
		UploadIDMarker: q.Get("upload-id-marker"),
		MaxUploads:     1000,
	}

	if m := q.Get("max-uploads"); m != "" {
		if v, err := strconv.Atoi(m); err == nil && v > 0 {
			opts.MaxUploads = v
		}
	}

	result, err := h.Backend.ListMultipartUploads(ctx, bucket, opts)
	if err != nil {
		WriteError(w, r, err)
		return
	}

	res := s3.ListMultipartUploadsResult{
		Bucket:             bucket,
		KeyMarker:          opts.KeyMarker,
		UploadIdMarker:     opts.UploadIDMarker,
		NextKeyMarker:      result.NextKeyMarker,
		NextUploadIdMarker: result.NextUploadIDMarker,
		MaxUploads:         opts.MaxUploads,
		IsTruncated:        result.IsTruncated,
	}

	for _, u := range result.Uploads {
		res.Uploads = append(res.Uploads, s3.Upload{
			Key:      u.Key,
			UploadId: u.UploadID,
			Initiator: s3.Owner{
				ID:          u.Initiator,
				DisplayName: u.Initiator,
			},
			Owner: s3.Owner{
				ID:          u.Owner,
				DisplayName: u.Owner,
			},
			Initiated: u.Initiated,
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
