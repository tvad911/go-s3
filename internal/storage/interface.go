package storage

import (
	"context"
	"io"
	"time"

	"gos3/internal/s3"
)

// BucketInfo represents information about a bucket.
type BucketInfo struct {
	Name         string
	CreationDate time.Time
	Region       string
	Owner        string
	ACL          string
}

// ObjectMeta contains metadata for an object.
type ObjectMeta struct {
	Size               int64
	ETag               string
	ContentType        string
	ContentEncoding    string
	ContentDisposition string
	ContentLanguage    string
	CacheControl       string
	Expires            string
	LastModified       time.Time
	UserMeta           map[string]string
	Tags               map[string]string
	StorageClass       string
	ACL                string
}

// PutResult represents the result of a PutObject operation.
type PutResult struct {
	ETag string
	Size int64
}

// GetOptions holds options for GetObject.
type GetOptions struct {
	RangeStart        *int64
	RangeEnd          *int64
	IfMatch           string
	IfNoneMatch       string
	IfModifiedSince   time.Time
	IfUnmodifiedSince time.Time
}

// Object represents a retrieved object with its metadata and content stream.
type Object struct {
	ObjectMeta
	Content    io.ReadCloser
	RangeStart int64
	RangeEnd   int64
}

// DeleteResult represents the result of a multi-object deletion.
type DeleteResult struct {
	Deleted []string
	Errors  []DeleteError
}

type DeleteError struct {
	Key     string
	Code    string
	Message string
}

// ListOptions holds options for ListObjects V1.
type ListOptions struct {
	Prefix    string
	Delimiter string
	Marker    string
	MaxKeys   int
}

// ListResult holds the result of ListObjects V1.
type ListResult struct {
	IsTruncated    bool
	Marker         string
	NextMarker     string
	Contents       []ObjectInfo
	CommonPrefixes []string
}

// ListOptionsV2 holds options for ListObjects V2.
type ListOptionsV2 struct {
	Prefix            string
	Delimiter         string
	ContinuationToken string
	StartAfter        string
	MaxKeys           int
	FetchOwner        bool
}

// ListResultV2 holds the result of ListObjects V2.
type ListResultV2 struct {
	IsTruncated           bool
	ContinuationToken     string
	NextContinuationToken string
	Contents              []ObjectInfo
	CommonPrefixes        []string
}

// ObjectInfo holds basic information about a listed object.
type ObjectInfo struct {
	Key          string
	LastModified time.Time
	ETag         string
	Size         int64
	StorageClass string
	Owner        string
}

// CopyResult holds the result of a CopyObject operation.
type CopyResult struct {
	ETag         string
	LastModified time.Time
}

// PartInfo holds information about an uploaded part.
type PartInfo struct {
	PartNumber   int
	ETag         string
	LastModified time.Time
	Size         int64
}

// CompletePart represents a part supplied by the user to complete a multipart upload.
type CompletePart struct {
	PartNumber int
	ETag       string
}

// CompleteResult holds the result of completing a multipart upload.
type CompleteResult struct {
	Location string
	Bucket   string
	Key      string
	ETag     string
}

// ListPartsOptions holds options for ListParts.
type ListPartsOptions struct {
	MaxParts         int
	PartNumberMarker int
}

// ListPartsResult holds the result of ListParts.
type ListPartsResult struct {
	IsTruncated          bool
	NextPartNumberMarker int
	Parts                []PartInfo
	Initiator            string
	Owner                string
}

// ListUploadsOptions holds options for ListMultipartUploads.
type ListUploadsOptions struct {
	Delimiter      string
	Prefix         string
	MaxUploads     int
	KeyMarker      string
	UploadIDMarker string
}

// UploadInfo holds information about a multipart upload.
type UploadInfo struct {
	Key       string
	UploadID  string
	Initiated time.Time
	Initiator string
	Owner     string
}

// ListUploadsResult holds the result of ListMultipartUploads.
type ListUploadsResult struct {
	IsTruncated        bool
	NextKeyMarker      string
	NextUploadIDMarker string
	Uploads            []UploadInfo
	CommonPrefixes     []string
}

// Backend is the main interface for the storage layer.
type Backend interface {
	// Bucket operations
	CreateBucket(ctx context.Context, bucket, region, acl string) error
	DeleteBucket(ctx context.Context, bucket string) error
	BucketExists(ctx context.Context, bucket string) (bool, error)
	ListBuckets(ctx context.Context) ([]BucketInfo, error)

	GetBucketCORS(ctx context.Context, bucket string) (*s3.CORSConfiguration, error)
	PutBucketCORS(ctx context.Context, bucket string, cors *s3.CORSConfiguration) error
	DeleteBucketCORS(ctx context.Context, bucket string) error

	// Object operations
	PutObject(ctx context.Context, bucket, key string, r io.Reader, size int64, meta ObjectMeta) (*PutResult, error)
	GetObject(ctx context.Context, bucket, key string, opts GetOptions) (*Object, error)
	HeadObject(ctx context.Context, bucket, key string) (*ObjectMeta, error)
	DeleteObject(ctx context.Context, bucket, key string) error
	DeleteObjects(ctx context.Context, bucket string, keys []string) (*DeleteResult, error)
	ListObjects(ctx context.Context, bucket string, opts ListOptions) (*ListResult, error)
	ListObjectsV2(ctx context.Context, bucket string, opts ListOptionsV2) (*ListResultV2, error)
	CopyObject(ctx context.Context, srcBucket, srcKey, dstBucket, dstKey string, meta *ObjectMeta) (*CopyResult, error)

	// Multipart operations
	CreateMultipartUpload(ctx context.Context, bucket, key string, meta ObjectMeta) (uploadID string, err error)
	UploadPart(ctx context.Context, bucket, key, uploadID string, partNum int, r io.Reader, size int64) (*PartInfo, error)
	CompleteMultipartUpload(ctx context.Context, bucket, key, uploadID string, parts []CompletePart) (*CompleteResult, error)
	AbortMultipartUpload(ctx context.Context, bucket, key, uploadID string) error
	ListParts(ctx context.Context, bucket, key, uploadID string, opts ListPartsOptions) (*ListPartsResult, error)
	ListMultipartUploads(ctx context.Context, bucket string, opts ListUploadsOptions) (*ListUploadsResult, error)
}
