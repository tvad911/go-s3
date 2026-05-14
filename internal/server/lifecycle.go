package server

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"gos3/internal/s3"
	"gos3/internal/storage"
)

func (s *Server) startLifecycleWorker(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	// Run once on startup
	s.runLifecycleCleanup(ctx)

	for {
		select {
		case <-ctx.Done():
			slog.Info("stopping lifecycle worker")
			return
		case <-ticker.C:
			s.runLifecycleCleanup(ctx)
		}
	}
}

func (s *Server) runLifecycleCleanup(ctx context.Context) {
	slog.Info("running lifecycle cleanup")

	buckets, err := s.backend.ListBuckets(ctx)
	if err != nil {
		slog.Error("failed to list buckets for lifecycle cleanup", "error", err)
		return
	}

	for _, bucket := range buckets {
		lifecycle, err := s.backend.GetBucketLifecycle(ctx, bucket.Name)
		if err != nil {
			if err == s3.ErrNoSuchLifecycleConfiguration {
				continue
			}
			slog.Error("failed to get bucket lifecycle", "bucket", bucket.Name, "error", err)
			continue
		}

		for _, rule := range lifecycle.Rules {
			if rule.Status != "Enabled" {
				continue
			}

			// Apply Expiration
			if rule.Expiration != nil && rule.Expiration.Days > 0 {
				s.applyExpirationRule(ctx, bucket.Name, rule)
			}

			// Apply AbortIncompleteMultipartUpload
			if rule.AbortIncompleteMultipartUpload != nil && rule.AbortIncompleteMultipartUpload.DaysAfterInitiation > 0 {
				s.applyAbortIncompleteMultipartUploadRule(ctx, bucket.Name, rule)
			}
		}
	}
}

func (s *Server) applyExpirationRule(ctx context.Context, bucket string, rule s3.LifecycleRule) {
	cutoff := time.Now().Add(-time.Duration(rule.Expiration.Days) * 24 * time.Hour)

	// In a real implementation we would paginate properly.
	// For now we'll just request a large number of versions.
	opts := storage.ListVersionsOptions{
		Prefix:  rule.Filter.Prefix,
		MaxKeys: 10000,
	}

	for {
		result, err := s.backend.ListObjectVersions(ctx, bucket, opts)
		if err != nil {
			slog.Error("failed to list object versions for lifecycle", "bucket", bucket, "error", err)
			return
		}

		for _, obj := range result.Objects {
			if obj.LastModified.Before(cutoff) {
				// According to S3, expiration rule for versioned bucket deletes the specific version.
				// For non-versioned bucket, it deletes the object (creates a delete marker if versioning was enabled later).
				err := s.backend.DeleteObject(ctx, bucket, obj.Key, obj.VersionID)
				if err != nil {
					slog.Error("failed to expire object", "bucket", bucket, "key", obj.Key, "version", obj.VersionID, "error", err)
				} else {
					slog.Info("expired object", "bucket", bucket, "key", obj.Key, "version", obj.VersionID)
				}
			}
		}

		// Also expire delete markers
		for _, dm := range result.DeleteMarkers {
			if dm.LastModified.Before(cutoff) {
				err := s.backend.DeleteObject(ctx, bucket, dm.Key, dm.VersionID)
				if err != nil {
					slog.Error("failed to expire delete marker", "bucket", bucket, "key", dm.Key, "version", dm.VersionID, "error", err)
				} else {
					slog.Info("expired delete marker", "bucket", bucket, "key", dm.Key, "version", dm.VersionID)
				}
			}
		}

		if !result.IsTruncated {
			break
		}
		opts.KeyMarker = result.NextKeyMarker
		opts.VersionIdMarker = result.NextVersionIdMarker
	}
}

func (s *Server) applyAbortIncompleteMultipartUploadRule(ctx context.Context, bucket string, rule s3.LifecycleRule) {
	cutoff := time.Now().Add(-time.Duration(rule.AbortIncompleteMultipartUpload.DaysAfterInitiation) * 24 * time.Hour)

	opts := storage.ListUploadsOptions{
		Prefix:     rule.Filter.Prefix,
		MaxUploads: 10000,
	}

	for {
		result, err := s.backend.ListMultipartUploads(ctx, bucket, opts)
		if err != nil {
			slog.Error("failed to list multipart uploads for lifecycle", "bucket", bucket, "error", err)
			return
		}

		for _, upload := range result.Uploads {
			if upload.Initiated.Before(cutoff) {
				if strings.HasPrefix(upload.Key, rule.Filter.Prefix) {
					err := s.backend.AbortMultipartUpload(ctx, bucket, upload.Key, upload.UploadID)
					if err != nil {
						slog.Error("failed to abort incomplete multipart upload", "bucket", bucket, "key", upload.Key, "uploadId", upload.UploadID, "error", err)
					} else {
						slog.Info("aborted incomplete multipart upload", "bucket", bucket, "key", upload.Key, "uploadId", upload.UploadID)
					}
				}
			}
		}

		if !result.IsTruncated {
			break
		}
		opts.KeyMarker = result.NextKeyMarker
		opts.UploadIDMarker = result.NextUploadIDMarker
	}
}
