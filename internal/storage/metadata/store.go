package metadata

import (
	"context"
	"errors"
	"io"
	"time"

	"gos3/internal/auth"
	"gos3/internal/s3"
	"gos3/internal/storage"
)

var (
	ErrBucketNotFound = errors.New("bucket not found")
	ErrBucketExists   = errors.New("bucket already exists")
	ErrObjectNotFound = errors.New("object not found")
	ErrUploadNotFound = errors.New("upload not found")
)

type AuditLog struct {
	ID        string    `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	User      string    `json:"user"`
	Action    string    `json:"action"`
	Target    string    `json:"target"`
	Details   string    `json:"details,omitempty"`
	IP        string    `json:"ip"`
}

type AuditStore interface {
	RecordAuditLog(ctx context.Context, log *AuditLog) error
	ListAuditLogs(ctx context.Context, limit int) ([]AuditLog, error)
}

type SettingStore interface {
	GetSetting(ctx context.Context, key string) (string, error)
	PutSetting(ctx context.Context, key string, value string) error
	DeleteSetting(ctx context.Context, key string) error
	ListSettings(ctx context.Context) (map[string]string, error)
}

// Store defines the interface for the metadata database.
type Store interface {
	// CORSStore must be defined before use
	GetBucketCORS(ctx context.Context, bucket string) (*s3.CORSConfiguration, error)
	PutBucketCORS(ctx context.Context, bucket string, cors *s3.CORSConfiguration) error
	DeleteBucketCORS(ctx context.Context, bucket string) error

	GetBucketLifecycle(ctx context.Context, bucket string) (*s3.LifecycleConfiguration, error)
	PutBucketLifecycle(ctx context.Context, bucket string, lifecycle *s3.LifecycleConfiguration) error
	DeleteBucketLifecycle(ctx context.Context, bucket string) error

	GetBucketWebsite(ctx context.Context, bucket string) (*s3.WebsiteConfiguration, error)
	PutBucketWebsite(ctx context.Context, bucket string, website *s3.WebsiteConfiguration) error
	DeleteBucketWebsite(ctx context.Context, bucket string) error

	PutCustomDomain(ctx context.Context, domain string, bucket string) error
	GetCustomDomain(ctx context.Context, domain string) (string, error)
	DeleteCustomDomain(ctx context.Context, domain string) error
	GetBucketCustomDomains(ctx context.Context, bucket string) ([]string, error)

	GetBucketNotification(ctx context.Context, bucket string) (*s3.NotificationConfiguration, error)
	PutBucketNotification(ctx context.Context, bucket string, config *s3.NotificationConfiguration) error
	DeleteBucketNotification(ctx context.Context, bucket string) error

	// Backup
	BackupTo(w io.Writer) error

	// General
	Close() error

	// Bucket operations
	CreateBucket(name, region, owner, acl string, objectLockEnabled bool) error
	UpdateBucket(bucket *storage.BucketInfo) error
	DeleteBucket(name string) error
	GetBucket(name string) (*storage.BucketInfo, error)
	ListBuckets() ([]storage.BucketInfo, error)
	GetBucketStats(name string) (objects int64, bytes int64, err error)

	// Object operations
	PutObject(bucket, key string, meta storage.ObjectMeta) error
	GetObject(bucket, key, versionId string) (*storage.ObjectMeta, error)
	DeleteObject(bucket, key, versionId string) error
	ListObjects(bucket, prefix, delimiter, marker string, maxKeys int) ([]storage.ObjectInfo, []string, string, error)
	ListObjectVersions(bucket, prefix, delimiter, keyMarker, versionIdMarker string, maxKeys int) ([]storage.ObjectInfo, []string, string, string, error)

	// Multipart operations
	CreateMultipartUpload(bucket, key string, meta storage.ObjectMeta) (string, error)
	GetMultipartUpload(bucket, key, uploadID string) (*storage.ObjectMeta, error) // Returns original meta
	DeleteMultipartUpload(bucket, key, uploadID string) error
	PutObjectPart(uploadID string, partNum int, partInfo storage.PartInfo) error
	ListObjectParts(uploadID string, partNumberMarker, maxParts int) ([]storage.PartInfo, int, error)
	ListMultipartUploads(bucket, prefix, delimiter, keyMarker, uploadIDMarker string, maxUploads int) ([]storage.UploadInfo, []string, string, string, error)

	// User management
	auth.UserStore
	auth.PolicyStore
	auth.IAMPolicyStore
	auth.ServiceAccountStore
	auth.SessionStore
	AuditStore
	SettingStore
}
