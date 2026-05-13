package handler

import (
	"encoding/xml"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"gos3/internal/s3"
	"gos3/internal/storage"
)

func (h *S3Handler) CreateBucket(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	bucket := chi.URLParam(r, "bucket")

	if err := s3.ValidateBucketName(bucket); err != nil {
		WriteError(w, r, err)
		return
	}

	region := "us-east-1"
	if r.ContentLength > 0 {
		var cfg s3.CreateBucketConfiguration
		if err := xml.NewDecoder(r.Body).Decode(&cfg); err == nil && cfg.LocationConstraint != "" {
			region = cfg.LocationConstraint
		}
	}

	if err := h.Backend.CreateBucket(ctx, bucket, region); err != nil {
		WriteError(w, r, err)
		return
	}

	w.Header().Set("Location", "/"+bucket)
	w.WriteHeader(http.StatusOK)
}

func (h *S3Handler) DeleteBucket(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	bucket := chi.URLParam(r, "bucket")

	if err := h.Backend.DeleteBucket(ctx, bucket); err != nil {
		WriteError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *S3Handler) HeadBucket(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	bucket := chi.URLParam(r, "bucket")

	exists, err := h.Backend.BucketExists(ctx, bucket)
	if err != nil {
		WriteError(w, r, err)
		return
	}
	if !exists {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (h *S3Handler) ListBuckets(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	buckets, err := h.Backend.ListBuckets(ctx)
	if err != nil {
		WriteError(w, r, err)
		return
	}

	res := s3.ListAllMyBucketsResult{
		Owner: s3.Owner{
			ID:          "admin", // Stub
			DisplayName: "admin",
		},
	}

	for _, b := range buckets {
		res.Buckets.Bucket = append(res.Buckets.Bucket, s3.Bucket{
			Name:         b.Name,
			CreationDate: b.CreationDate,
		})
	}

	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(xml.Header))
	xml.NewEncoder(w).Encode(res)
}

func (h *S3Handler) ListObjects(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	bucket := chi.URLParam(r, "bucket")

	q := r.URL.Query()

	// Check if V2
	if q.Get("list-type") == "2" {
		h.listObjectsV2(w, r, bucket)
		return
	}

	maxKeys := 1000
	if mk := q.Get("max-keys"); mk != "" {
		if v, err := strconv.Atoi(mk); err == nil && v > 0 {
			maxKeys = v
		}
	}

	opts := storage.ListOptions{
		Prefix:    q.Get("prefix"),
		Delimiter: q.Get("delimiter"),
		Marker:    q.Get("marker"),
		MaxKeys:   maxKeys,
	}

	result, err := h.Backend.ListObjects(ctx, bucket, opts)
	if err != nil {
		WriteError(w, r, err)
		return
	}

	res := s3.ListBucketResult{
		Name:        bucket,
		Prefix:      opts.Prefix,
		Marker:      opts.Marker,
		MaxKeys:     opts.MaxKeys,
		Delimiter:   opts.Delimiter,
		IsTruncated: result.IsTruncated,
		NextMarker:  result.NextMarker,
	}

	for _, obj := range result.Contents {
		res.Contents = append(res.Contents, s3.Object{
			Key:          obj.Key,
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

	for _, p := range result.CommonPrefixes {
		res.CommonPrefixes = append(res.CommonPrefixes, s3.CommonPrefix{Prefix: p})
	}

	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(xml.Header))
	xml.NewEncoder(w).Encode(res)
}

func (h *S3Handler) listObjectsV2(w http.ResponseWriter, r *http.Request, bucket string) {
	ctx := r.Context()
	q := r.URL.Query()

	maxKeys := 1000
	if mk := q.Get("max-keys"); mk != "" {
		if v, err := strconv.Atoi(mk); err == nil && v > 0 {
			maxKeys = v
		}
	}

	opts := storage.ListOptionsV2{
		Prefix:            q.Get("prefix"),
		Delimiter:         q.Get("delimiter"),
		ContinuationToken: q.Get("continuation-token"),
		StartAfter:        q.Get("start-after"),
		MaxKeys:           maxKeys,
		FetchOwner:        q.Get("fetch-owner") == "true",
	}

	result, err := h.Backend.ListObjectsV2(ctx, bucket, opts)
	if err != nil {
		WriteError(w, r, err)
		return
	}

	res := s3.ListBucketResultV2{
		Name:                  bucket,
		Prefix:                opts.Prefix,
		ContinuationToken:     opts.ContinuationToken,
		NextContinuationToken: result.NextContinuationToken,
		MaxKeys:               opts.MaxKeys,
		Delimiter:             opts.Delimiter,
		IsTruncated:           result.IsTruncated,
		KeyCount:              len(result.Contents),
	}

	for _, obj := range result.Contents {
		o := s3.Object{
			Key:          obj.Key,
			LastModified: obj.LastModified,
			ETag:         `"` + obj.ETag + `"`,
			Size:         obj.Size,
			StorageClass: obj.StorageClass,
		}
		if opts.FetchOwner {
			o.Owner = &s3.Owner{
				ID:          "admin",
				DisplayName: "admin",
			}
		}
		res.Contents = append(res.Contents, o)
	}

	for _, p := range result.CommonPrefixes {
		res.CommonPrefixes = append(res.CommonPrefixes, s3.CommonPrefix{Prefix: p})
	}

	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(xml.Header))
	xml.NewEncoder(w).Encode(res)
}
