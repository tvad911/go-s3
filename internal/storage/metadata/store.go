package metadata

import (
	"errors"
	"gos3/internal/auth"
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
	// General
	Close() error

	// Bucket operations
	CreateBucket(name, region, owner string) error
	DeleteBucket(name string) error
	GetBucket(name string) (*storage.BucketInfo, error)
	ListBuckets() ([]storage.BucketInfo, error)

	// Object operations
	PutObject(bucket, key string, meta storage.ObjectMeta) error
	GetObject(bucket, key string) (*storage.ObjectMeta, error)
	DeleteObject(bucket, key string) error
	ListObjects(bucket, prefix, delimiter, marker string, maxKeys int) ([]storage.ObjectInfo, []string, string, error)

	// Multipart operations
	CreateMultipartUpload(bucket, key string, meta storage.ObjectMeta) (string, error)
	GetMultipartUpload(bucket, key, uploadID string) (*storage.ObjectMeta, error) // Returns original meta
	DeleteMultipartUpload(bucket, key, uploadID string) error
	PutObjectPart(uploadID string, partNum int, partInfo storage.PartInfo) error
	ListObjectParts(uploadID string, partNumberMarker, maxParts int) ([]storage.PartInfo, int, error)
	ListMultipartUploads(bucket, prefix, delimiter, keyMarker, uploadIDMarker string, maxUploads int) ([]storage.UploadInfo, []string, string, string, error)

	// User management
	auth.UserStore
}
