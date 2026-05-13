package metadata

import (
	"context"
	"errors"

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

// Store defines the interface for the metadata database.
type Store interface {
	// CORSStore must be defined before use
	GetBucketCORS(ctx context.Context, bucket string) (*s3.CORSConfiguration, error)
	PutBucketCORS(ctx context.Context, bucket string, cors *s3.CORSConfiguration) error
	DeleteBucketCORS(ctx context.Context, bucket string) error
	// General
	Close() error

	// Bucket operations
	CreateBucket(name, region, owner, acl string) error
	UpdateBucket(bucket *storage.BucketInfo) error
	DeleteBucket(name string) error
	GetBucket(name string) (*storage.BucketInfo, error)
	ListBuckets() ([]storage.BucketInfo, error)

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
}
