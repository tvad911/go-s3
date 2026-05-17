package handler

import (
	"encoding/json"
	"encoding/xml"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"gos3/internal/auth"
	"gos3/internal/s3"
	"gos3/internal/storage"
)

func (h *S3Handler) CreateBucket(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	bucket := chi.URLParam(r, "bucket")

	if err := h.CheckPolicy(r, "s3:CreateBucket", bucket, ""); err != nil {
		WriteError(w, r, err)
		return
	}

	if err := s3.ValidateBucketName(bucket); err != nil {
		WriteError(w, r, err)
		return
	}

	region := "us-east-1"
	if r.ContentLength > 0 {
		var cfg s3.CreateBucketConfiguration
		if err := xml.NewDecoder(io.LimitReader(r.Body, 2<<20)).Decode(&cfg); err == nil && cfg.LocationConstraint != "" {
			region = cfg.LocationConstraint
		}
	}

	acl := auth.ParseACL(r)
	objectLockEnabled := r.Header.Get("x-amz-bucket-object-lock-enabled") == "true"

	if err := h.Backend.CreateBucket(ctx, bucket, region, acl, objectLockEnabled); err != nil {
		WriteError(w, r, err)
		return
	}

	w.Header().Set("Location", "/"+bucket)
	w.WriteHeader(http.StatusOK)
}

func (h *S3Handler) DeleteBucket(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	bucket := chi.URLParam(r, "bucket")

	if err := h.CheckPolicy(r, "s3:DeleteBucket", bucket, ""); err != nil {
		WriteError(w, r, err)
		return
	}

	if err := h.Backend.DeleteBucket(ctx, bucket); err != nil {
		WriteError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *S3Handler) GetBucketLocation(w http.ResponseWriter, r *http.Request) {
	bucket := chi.URLParam(r, "bucket")

	if err := h.CheckPolicy(r, "s3:GetBucketLocation", bucket, ""); err != nil {
		WriteError(w, r, err)
		return
	}

	bInfo, err := h.MetaStore.GetBucket(bucket)
	if err != nil {
		WriteError(w, r, err)
		return
	}

	type LocationConstraint struct {
		XMLName xml.Name `xml:"http://s3.amazonaws.com/doc/2006-03-01/ LocationConstraint"`
		Value   string   `xml:",chardata"`
	}

	loc := LocationConstraint{
		Value: bInfo.Region,
	}
	
	// AWS S3 standard: "us-east-1" is sometimes returned as empty.
	// But it is safer to return the string "us-east-1" for compatibility with strict clients.
	if loc.Value == "" {
		loc.Value = "us-east-1"
	}

	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(xml.Header))
	xml.NewEncoder(w).Encode(loc)
}

func (h *S3Handler) HeadBucket(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	bucket := chi.URLParam(r, "bucket")

	if err := h.CheckPolicy(r, "s3:ListBucket", bucket, ""); err != nil {
		WriteError(w, r, err)
		return
	}

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

	if err := h.CheckPolicy(r, "s3:ListAllMyBuckets", "", ""); err != nil {
		WriteError(w, r, err)
		return
	}

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

	if err := h.CheckPolicy(r, "s3:ListBucket", bucket, ""); err != nil {
		WriteError(w, r, err)
		return
	}

	q := r.URL.Query()

	isWebsiteReq := strings.Contains(r.Header.Get("Accept"), "text/html") || ctx.Value(s3.CtxKeyIsCustomDomain) == true
	if isWebsiteReq {
		website, err := h.MetaStore.GetBucketWebsite(ctx, bucket)
		if err == nil && website != nil && website.IndexDocument.Suffix != "" {
			rctx := chi.RouteContext(ctx)
			rctx.URLParams.Add("key", website.IndexDocument.Suffix)
			h.GetObject(w, r)
			return
		}
	}

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

// GetBucketVersioning handles GET /bucket?versioning
func (h *S3Handler) GetBucketVersioning(w http.ResponseWriter, r *http.Request) {
	bucket := chi.URLParam(r, "bucket")

	bInfo, err := h.MetaStore.GetBucket(bucket)
	if err != nil {
		WriteError(w, r, err)
		return
	}

	res := s3.VersioningConfiguration{
		Status: bInfo.Versioning,
	}
	if res.Status == "" {
		res.Status = "" // S3 returns empty tag if never enabled
	}

	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(xml.Header))
	xml.NewEncoder(w).Encode(res)
}

// PutBucketVersioning handles PUT /bucket?versioning
func (h *S3Handler) PutBucketVersioning(w http.ResponseWriter, r *http.Request) {
	bucket := chi.URLParam(r, "bucket")

	bInfo, err := h.MetaStore.GetBucket(bucket)
	if err != nil {
		WriteError(w, r, err)
		return
	}

	var req s3.VersioningConfiguration
	if err := xml.NewDecoder(io.LimitReader(r.Body, 2<<20)).Decode(&req); err != nil {
		WriteError(w, r, s3.ErrMalformedXML)
		return
	}

	if req.Status != "Enabled" && req.Status != "Suspended" {
		WriteError(w, r, s3.ErrMalformedXML) // close enough for invalid status
		return
	}

	bInfo.Versioning = req.Status
	if err := h.MetaStore.UpdateBucket(bInfo); err != nil {
		WriteError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// PutBucketPolicy handles PUT /bucket?policy
func (h *S3Handler) PutBucketPolicy(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	bucket := chi.URLParam(r, "bucket")

	if err := h.CheckPolicy(r, "s3:PutBucketPolicy", bucket, ""); err != nil {
		WriteError(w, r, err)
		return
	}

	var policy auth.Policy
	if err := json.NewDecoder(io.LimitReader(r.Body, 2<<20)).Decode(&policy); err != nil {
		WriteError(w, r, err) // map to ErrMalformedPolicy
		return
	}

	if err := h.MetaStore.PutBucketPolicy(ctx, bucket, &policy); err != nil {
		WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// GetBucketPolicy handles GET /bucket?policy
func (h *S3Handler) GetBucketPolicy(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	bucket := chi.URLParam(r, "bucket")

	if err := h.CheckPolicy(r, "s3:GetBucketPolicy", bucket, ""); err != nil {
		WriteError(w, r, err)
		return
	}

	policy, err := h.MetaStore.GetBucketPolicy(ctx, bucket)
	if err != nil {
		WriteError(w, r, err) // Should map to s3.ErrNoSuchBucketPolicy
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(policy)
}

// DeleteBucketPolicy handles DELETE /bucket?policy
func (h *S3Handler) DeleteBucketPolicy(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	bucket := chi.URLParam(r, "bucket")

	if err := h.CheckPolicy(r, "s3:DeleteBucketPolicy", bucket, ""); err != nil {
		WriteError(w, r, err)
		return
	}

	if err := h.MetaStore.DeleteBucketPolicy(ctx, bucket); err != nil {
		WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// GetBucketAcl handles GET /bucket?acl
func (h *S3Handler) GetBucketAcl(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	bucket := chi.URLParam(r, "bucket")

	exists, err := h.Backend.BucketExists(ctx, bucket)
	if err != nil {
		WriteError(w, r, err)
		return
	}
	if !exists {
		WriteError(w, r, s3.ErrNoSuchBucket)
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

// PutBucketAcl handles PUT /bucket?acl
func (h *S3Handler) PutBucketAcl(w http.ResponseWriter, r *http.Request) {
	WriteError(w, r, s3.ErrNotImplemented)
}

// GetBucketCors handles GET /bucket?cors
func (h *S3Handler) GetBucketCors(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	bucket := chi.URLParam(r, "bucket")

	cors, err := h.Backend.GetBucketCORS(ctx, bucket)
	if err != nil {
		WriteError(w, r, err) // Should map to NoSuchCORSConfiguration if not found
		return
	}

	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(xml.Header))
	xml.NewEncoder(w).Encode(cors)
}

// PutBucketCors handles PUT /bucket?cors
func (h *S3Handler) PutBucketCors(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	bucket := chi.URLParam(r, "bucket")

	var cors s3.CORSConfiguration
	if err := xml.NewDecoder(io.LimitReader(r.Body, 2<<20)).Decode(&cors); err != nil {
		WriteError(w, r, s3.ErrMalformedXML)
		return
	}

	if err := h.Backend.PutBucketCORS(ctx, bucket, &cors); err != nil {
		WriteError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// DeleteBucketCors handles DELETE /bucket?cors
func (h *S3Handler) DeleteBucketCors(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	bucket := chi.URLParam(r, "bucket")

	if err := h.Backend.DeleteBucketCORS(ctx, bucket); err != nil {
		WriteError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GetBucketLifecycle handles GET /bucket?lifecycle
func (h *S3Handler) GetBucketLifecycle(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	bucket := chi.URLParam(r, "bucket")

	lifecycle, err := h.Backend.GetBucketLifecycle(ctx, bucket)
	if err != nil {
		WriteError(w, r, err)
		return
	}

	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(xml.Header))
	xml.NewEncoder(w).Encode(lifecycle)
}

// PutBucketLifecycle handles PUT /bucket?lifecycle
func (h *S3Handler) PutBucketLifecycle(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	bucket := chi.URLParam(r, "bucket")

	var lifecycle s3.LifecycleConfiguration
	if err := xml.NewDecoder(io.LimitReader(r.Body, 2<<20)).Decode(&lifecycle); err != nil {
		WriteError(w, r, s3.ErrMalformedXML)
		return
	}

	if err := h.Backend.PutBucketLifecycle(ctx, bucket, &lifecycle); err != nil {
		WriteError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// DeleteBucketLifecycle handles DELETE /bucket?lifecycle
func (h *S3Handler) DeleteBucketLifecycle(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	bucket := chi.URLParam(r, "bucket")

	if err := h.Backend.DeleteBucketLifecycle(ctx, bucket); err != nil {
		WriteError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GetBucketWebsite handles GET /bucket?website
func (h *S3Handler) GetBucketWebsite(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	bucket := chi.URLParam(r, "bucket")

	website, err := h.MetaStore.GetBucketWebsite(ctx, bucket)
	if err != nil {
		WriteError(w, r, err)
		return
	}

	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(xml.Header))
	xml.NewEncoder(w).Encode(website)
}

// PutBucketWebsite handles PUT /bucket?website
func (h *S3Handler) PutBucketWebsite(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	bucket := chi.URLParam(r, "bucket")

	var website s3.WebsiteConfiguration
	if err := xml.NewDecoder(io.LimitReader(r.Body, 2<<20)).Decode(&website); err != nil {
		WriteError(w, r, s3.ErrMalformedXML)
		return
	}

	if err := h.MetaStore.PutBucketWebsite(ctx, bucket, &website); err != nil {
		WriteError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// DeleteBucketWebsite handles DELETE /bucket?website
func (h *S3Handler) DeleteBucketWebsite(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	bucket := chi.URLParam(r, "bucket")

	if err := h.MetaStore.DeleteBucketWebsite(ctx, bucket); err != nil {
		WriteError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
