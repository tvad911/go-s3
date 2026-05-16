package metadata_test

import (
	"context"
	"path/filepath"
	"testing"

	"gos3/internal/auth"
	"gos3/internal/s3"
	"gos3/internal/storage"
	"gos3/internal/storage/metadata"
)

// setupStore creates a new bbolt store in t.TempDir(). Auto-closed via t.Cleanup.
func setupStore(t *testing.T) metadata.Store {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	store, err := metadata.NewBboltStore(dbPath)
	if err != nil {
		t.Fatalf("NewBboltStore: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

func TestStore_Bucket_CRUD(t *testing.T) {
	s := setupStore(t)

	// Create
	if err := s.CreateBucket("test-b", "us-east-1", "admin", "private", false); err != nil {
		t.Fatalf("CreateBucket: %v", err)
	}

	// Get
	b, err := s.GetBucket("test-b")
	if err != nil {
		t.Fatalf("GetBucket: %v", err)
	}
	if b.Name != "test-b" || b.Region != "us-east-1" || b.Owner != "admin" {
		t.Errorf("unexpected bucket info: %+v", b)
	}

	// List
	buckets, err := s.ListBuckets()
	if err != nil {
		t.Fatalf("ListBuckets: %v", err)
	}
	if len(buckets) != 1 {
		t.Errorf("expected 1 bucket, got %d", len(buckets))
	}

	// Delete
	if err := s.DeleteBucket("test-b"); err != nil {
		t.Fatalf("DeleteBucket: %v", err)
	}

	_, err = s.GetBucket("test-b")
	if err != metadata.ErrBucketNotFound {
		t.Errorf("expected ErrBucketNotFound, got %v", err)
	}
}

func TestStore_Bucket_Duplicate(t *testing.T) {
	s := setupStore(t)
	s.CreateBucket("dup", "us-east-1", "admin", "private", false)

	err := s.CreateBucket("dup", "us-east-1", "admin", "private", false)
	if err != metadata.ErrBucketExists {
		t.Errorf("expected ErrBucketExists, got %v", err)
	}
}

func TestStore_Object_CRUD(t *testing.T) {
	s := setupStore(t)
	s.CreateBucket("obj-b", "us-east-1", "admin", "private", false)

	meta := storage.ObjectMeta{
		ContentType: "text/plain",
		ETag:        "abc123",
		Size:        42,
	}

	// Put
	if err := s.PutObject("obj-b", "file.txt", meta); err != nil {
		t.Fatalf("PutObject: %v", err)
	}

	// Get
	got, err := s.GetObject("obj-b", "file.txt", "")
	if err != nil {
		t.Fatalf("GetObject: %v", err)
	}
	if got.ContentType != "text/plain" || got.ETag != "abc123" || got.Size != 42 {
		t.Errorf("unexpected object meta: %+v", got)
	}

	// Delete
	if err := s.DeleteObject("obj-b", "file.txt", ""); err != nil {
		t.Fatalf("DeleteObject: %v", err)
	}

	_, err = s.GetObject("obj-b", "file.txt", "")
	if err != metadata.ErrObjectNotFound {
		t.Errorf("expected ErrObjectNotFound, got %v", err)
	}
}

func TestStore_Object_BucketNotFound(t *testing.T) {
	s := setupStore(t)

	err := s.PutObject("nonexistent", "file.txt", storage.ObjectMeta{})
	if err != metadata.ErrBucketNotFound {
		t.Errorf("expected ErrBucketNotFound, got %v", err)
	}
}

func TestStore_ListObjects(t *testing.T) {
	s := setupStore(t)
	s.CreateBucket("list-b", "us-east-1", "admin", "private", false)

	for _, key := range []string{"a.txt", "b.txt", "sub/c.txt"} {
		s.PutObject("list-b", key, storage.ObjectMeta{ETag: "x"})
	}

	objects, _, _, err := s.ListObjects("list-b", "", "", "", 1000)
	if err != nil {
		t.Fatalf("ListObjects: %v", err)
	}
	if len(objects) != 3 {
		t.Errorf("expected 3 objects, got %d", len(objects))
	}

	// With prefix
	objects, _, _, err = s.ListObjects("list-b", "sub/", "", "", 1000)
	if err != nil {
		t.Fatalf("ListObjects prefix: %v", err)
	}
	if len(objects) != 1 {
		t.Errorf("expected 1 object with prefix sub/, got %d", len(objects))
	}
}

func TestStore_ListObjects_Delimiter(t *testing.T) {
	s := setupStore(t)
	s.CreateBucket("delim-b", "us-east-1", "admin", "private", false)

	for _, key := range []string{"dir1/a.txt", "dir1/b.txt", "dir2/c.txt", "root.txt"} {
		s.PutObject("delim-b", key, storage.ObjectMeta{ETag: "x"})
	}

	objects, prefixes, _, err := s.ListObjects("delim-b", "", "/", "", 1000)
	if err != nil {
		t.Fatalf("ListObjects delimiter: %v", err)
	}
	if len(objects) != 1 {
		t.Errorf("expected 1 root object, got %d", len(objects))
	}
	if len(prefixes) != 2 {
		t.Errorf("expected 2 common prefixes (dir1/, dir2/), got %d: %v", len(prefixes), prefixes)
	}
}

func TestStore_Multipart_CRUD(t *testing.T) {
	s := setupStore(t)
	s.CreateBucket("mp-b", "us-east-1", "admin", "private", false)

	meta := storage.ObjectMeta{ContentType: "application/octet-stream"}
	uploadID, err := s.CreateMultipartUpload("mp-b", "big.bin", meta)
	if err != nil {
		t.Fatalf("CreateMultipartUpload: %v", err)
	}
	if uploadID == "" {
		t.Fatal("uploadID should not be empty")
	}

	// Get upload meta
	got, err := s.GetMultipartUpload("mp-b", "big.bin", uploadID)
	if err != nil {
		t.Fatalf("GetMultipartUpload: %v", err)
	}
	if got.ContentType != "application/octet-stream" {
		t.Errorf("ContentType = %q", got.ContentType)
	}

	// Put parts
	for i := 1; i <= 3; i++ {
		s.PutObjectPart(uploadID, i, storage.PartInfo{
			PartNumber: i,
			ETag:       "part-etag",
			Size:       1024,
		})
	}

	// List parts
	parts, _, err := s.ListObjectParts(uploadID, 0, 10)
	if err != nil {
		t.Fatalf("ListObjectParts: %v", err)
	}
	if len(parts) != 3 {
		t.Errorf("expected 3 parts, got %d", len(parts))
	}

	// Delete upload
	if err := s.DeleteMultipartUpload("mp-b", "big.bin", uploadID); err != nil {
		t.Fatalf("DeleteMultipartUpload: %v", err)
	}

	// Should be gone
	_, err = s.GetMultipartUpload("mp-b", "big.bin", uploadID)
	if err != metadata.ErrUploadNotFound {
		t.Errorf("expected ErrUploadNotFound, got %v", err)
	}
}

func TestStore_CORS(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()
	s.CreateBucket("cors-b", "us-east-1", "admin", "private", false)

	cors := &s3.CORSConfiguration{
		CORSRule: []s3.CORSRule{
			{
				AllowedOrigin: []string{"*"},
				AllowedMethod: []string{"GET", "PUT"},
			},
		},
	}

	if err := s.PutBucketCORS(ctx, "cors-b", cors); err != nil {
		t.Fatalf("PutBucketCORS: %v", err)
	}

	got, err := s.GetBucketCORS(ctx, "cors-b")
	if err != nil {
		t.Fatalf("GetBucketCORS: %v", err)
	}
	if len(got.CORSRule) != 1 {
		t.Errorf("expected 1 CORS rule, got %d", len(got.CORSRule))
	}

	if err := s.DeleteBucketCORS(ctx, "cors-b"); err != nil {
		t.Fatalf("DeleteBucketCORS: %v", err)
	}
}

func TestStore_Users(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	user := &auth.User{
		Username:    "testuser",
		AccessKeyID: "AKTEST",
		SecretKey:   "secret",
		IsRoot:      false,
	}

	// Create
	if err := s.CreateUser(ctx, user); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	// Get by username
	got, err := s.GetUserByUsername(ctx, "testuser")
	if err != nil {
		t.Fatalf("GetUserByUsername: %v", err)
	}
	if got.AccessKeyID != "AKTEST" {
		t.Errorf("AccessKeyID = %q", got.AccessKeyID)
	}

	// Get by access key
	got, err = s.GetUserByAccessKey(ctx, "AKTEST")
	if err != nil {
		t.Fatalf("GetUserByAccessKey: %v", err)
	}
	if got.Username != "testuser" {
		t.Errorf("Username = %q", got.Username)
	}

	// List
	users, err := s.ListUsers(ctx)
	if err != nil {
		t.Fatalf("ListUsers: %v", err)
	}
	if len(users) != 1 {
		t.Errorf("expected 1 user, got %d", len(users))
	}

	// Duplicate
	err = s.CreateUser(ctx, user)
	if err != auth.ErrUserExists {
		t.Errorf("expected ErrUserExists, got %v", err)
	}

	// Delete
	if err := s.DeleteUser(ctx, "testuser"); err != nil {
		t.Fatalf("DeleteUser: %v", err)
	}
	_, err = s.GetUserByUsername(ctx, "testuser")
	if err != auth.ErrUserNotFound {
		t.Errorf("expected ErrUserNotFound, got %v", err)
	}
}

func TestStore_BucketStats(t *testing.T) {
	s := setupStore(t)
	s.CreateBucket("stats-b", "us-east-1", "admin", "private", false)

	s.PutObject("stats-b", "a.txt", storage.ObjectMeta{Size: 100, ETag: "x"})
	s.PutObject("stats-b", "b.txt", storage.ObjectMeta{Size: 200, ETag: "y"})

	objCount, totalBytes, err := s.GetBucketStats("stats-b")
	if err != nil {
		t.Fatalf("GetBucketStats: %v", err)
	}
	if objCount != 2 {
		t.Errorf("expected 2 objects, got %d", objCount)
	}
	if totalBytes != 300 {
		t.Errorf("expected 300 bytes, got %d", totalBytes)
	}
}

func TestStore_UpdateBucket(t *testing.T) {
	s := setupStore(t)
	s.CreateBucket("upd-b", "us-east-1", "admin", "private", false)

	b, _ := s.GetBucket("upd-b")
	b.Versioning = "Enabled"
	if err := s.UpdateBucket(b); err != nil {
		t.Fatalf("UpdateBucket: %v", err)
	}

	got, _ := s.GetBucket("upd-b")
	if got.Versioning != "Enabled" {
		t.Errorf("Versioning = %q, want 'Enabled'", got.Versioning)
	}
}
