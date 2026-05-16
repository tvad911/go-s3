package local_test

import (
	"bytes"
	"context"
	"io"
	"path/filepath"
	"testing"

	"gos3/internal/auth"
	"gos3/internal/s3"
	"gos3/internal/storage"
	"gos3/internal/storage/local"
	"gos3/internal/storage/metadata"
)

// setupBackend creates a fully wired local.Backend with real bbolt metadata
// in a temporary directory. Everything is auto-cleaned via t.Cleanup.
func setupBackend(t *testing.T) (*local.Backend, metadata.Store) {
	t.Helper()

	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	tempDir := filepath.Join(tmpDir, "tmp")
	metaDBPath := filepath.Join(tmpDir, "meta.db")

	metaStore, err := metadata.NewBboltStore(metaDBPath)
	if err != nil {
		t.Fatalf("NewBboltStore: %v", err)
	}
	t.Cleanup(func() { metaStore.Close() })

	backend, err := local.NewBackend(dataDir, tempDir, metaStore)
	if err != nil {
		t.Fatalf("NewBackend: %v", err)
	}

	return backend, metaStore
}

// rootCtx returns a context with a root user set (required by CreateBucket).
func rootCtx() context.Context {
	u := &auth.User{Username: "root", IsRoot: true}
	return context.WithValue(context.Background(), auth.UserContextKey, u)
}

func TestBackend_CreateBucket_And_Exists(t *testing.T) {
	b, _ := setupBackend(t)
	ctx := rootCtx()

	if err := b.CreateBucket(ctx, "test-bucket", "us-east-1", "private", false); err != nil {
		t.Fatalf("CreateBucket: %v", err)
	}

	exists, err := b.BucketExists(ctx, "test-bucket")
	if err != nil {
		t.Fatalf("BucketExists: %v", err)
	}
	if !exists {
		t.Error("expected bucket to exist after creation")
	}
}

func TestBackend_CreateBucket_Duplicate(t *testing.T) {
	b, _ := setupBackend(t)
	ctx := rootCtx()

	b.CreateBucket(ctx, "dup-bucket", "us-east-1", "private", false)

	err := b.CreateBucket(ctx, "dup-bucket", "us-east-1", "private", false)
	if err != s3.ErrBucketAlreadyExists {
		t.Errorf("expected ErrBucketAlreadyExists, got %v", err)
	}
}

func TestBackend_DeleteBucket_Empty(t *testing.T) {
	b, _ := setupBackend(t)
	ctx := rootCtx()

	b.CreateBucket(ctx, "del-bucket", "us-east-1", "private", false)

	if err := b.DeleteBucket(ctx, "del-bucket"); err != nil {
		t.Fatalf("DeleteBucket: %v", err)
	}

	exists, _ := b.BucketExists(ctx, "del-bucket")
	if exists {
		t.Error("bucket should not exist after deletion")
	}
}

func TestBackend_DeleteBucket_NotExists(t *testing.T) {
	b, _ := setupBackend(t)
	ctx := rootCtx()

	err := b.DeleteBucket(ctx, "nonexistent")
	if err != s3.ErrNoSuchBucket {
		t.Errorf("expected ErrNoSuchBucket, got %v", err)
	}
}

func TestBackend_DeleteBucket_NonEmpty(t *testing.T) {
	b, _ := setupBackend(t)
	ctx := rootCtx()

	b.CreateBucket(ctx, "nonempty", "us-east-1", "private", false)
	b.PutObject(ctx, "nonempty", "file.txt", bytes.NewReader([]byte("data")), 4, storage.ObjectMeta{ContentType: "text/plain"})

	err := b.DeleteBucket(ctx, "nonempty")
	if err != s3.ErrBucketNotEmpty {
		t.Errorf("expected ErrBucketNotEmpty, got %v", err)
	}
}

func TestBackend_ListBuckets(t *testing.T) {
	b, _ := setupBackend(t)
	ctx := rootCtx()

	b.CreateBucket(ctx, "alpha", "us-east-1", "private", false)
	b.CreateBucket(ctx, "beta", "us-east-1", "private", false)

	buckets, err := b.ListBuckets(ctx)
	if err != nil {
		t.Fatalf("ListBuckets: %v", err)
	}
	if len(buckets) != 2 {
		t.Errorf("expected 2 buckets, got %d", len(buckets))
	}
}

func TestBackend_PutObject_And_GetObject(t *testing.T) {
	b, _ := setupBackend(t)
	ctx := rootCtx()

	b.CreateBucket(ctx, "data-bucket", "us-east-1", "private", false)

	content := []byte("hello world, this is GoS3!")
	meta := storage.ObjectMeta{ContentType: "text/plain"}

	result, err := b.PutObject(ctx, "data-bucket", "test.txt", bytes.NewReader(content), int64(len(content)), meta)
	if err != nil {
		t.Fatalf("PutObject: %v", err)
	}
	if result.ETag == "" {
		t.Error("ETag should not be empty")
	}
	if result.Size != int64(len(content)) {
		t.Errorf("Size = %d, want %d", result.Size, len(content))
	}

	// GetObject
	obj, err := b.GetObject(ctx, "data-bucket", "test.txt", storage.GetOptions{})
	if err != nil {
		t.Fatalf("GetObject: %v", err)
	}
	defer obj.Content.Close()

	got, err := io.ReadAll(obj.Content)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Errorf("content mismatch: got %q, want %q", got, content)
	}
	if obj.ETag != result.ETag {
		t.Errorf("ETag mismatch: obj=%q, put=%q", obj.ETag, result.ETag)
	}
}

func TestBackend_PutObject_NoSuchBucket(t *testing.T) {
	b, _ := setupBackend(t)
	ctx := rootCtx()

	_, err := b.PutObject(ctx, "nonexistent", "file.txt", bytes.NewReader([]byte("data")), 4, storage.ObjectMeta{})
	if err != s3.ErrNoSuchBucket {
		t.Errorf("expected ErrNoSuchBucket, got %v", err)
	}
}

func TestBackend_GetObject_NoSuchKey(t *testing.T) {
	b, _ := setupBackend(t)
	ctx := rootCtx()

	b.CreateBucket(ctx, "empty-bucket", "us-east-1", "private", false)

	_, err := b.GetObject(ctx, "empty-bucket", "missing.txt", storage.GetOptions{})
	if err != s3.ErrNoSuchKey {
		t.Errorf("expected ErrNoSuchKey, got %v", err)
	}
}

func TestBackend_GetObject_NoSuchBucket(t *testing.T) {
	b, _ := setupBackend(t)
	ctx := rootCtx()

	_, err := b.GetObject(ctx, "nonexistent", "file.txt", storage.GetOptions{})
	if err != s3.ErrNoSuchBucket {
		t.Errorf("expected ErrNoSuchBucket, got %v", err)
	}
}

func TestBackend_HeadObject(t *testing.T) {
	b, _ := setupBackend(t)
	ctx := rootCtx()

	b.CreateBucket(ctx, "head-bucket", "us-east-1", "private", false)
	content := []byte("head-test-content")
	b.PutObject(ctx, "head-bucket", "file.txt", bytes.NewReader(content), int64(len(content)),
		storage.ObjectMeta{ContentType: "text/plain"})

	meta, err := b.HeadObject(ctx, "head-bucket", "file.txt", storage.GetOptions{})
	if err != nil {
		t.Fatalf("HeadObject: %v", err)
	}
	if meta.Size != int64(len(content)) {
		t.Errorf("Size = %d, want %d", meta.Size, len(content))
	}
	if meta.ContentType != "text/plain" {
		t.Errorf("ContentType = %q, want %q", meta.ContentType, "text/plain")
	}
}

func TestBackend_DeleteObject(t *testing.T) {
	b, _ := setupBackend(t)
	ctx := rootCtx()

	b.CreateBucket(ctx, "del-obj-bucket", "us-east-1", "private", false)
	b.PutObject(ctx, "del-obj-bucket", "file.txt", bytes.NewReader([]byte("data")), 4, storage.ObjectMeta{})

	if err := b.DeleteObject(ctx, "del-obj-bucket", "file.txt", ""); err != nil {
		t.Fatalf("DeleteObject: %v", err)
	}

	_, err := b.GetObject(ctx, "del-obj-bucket", "file.txt", storage.GetOptions{})
	if err != s3.ErrNoSuchKey {
		t.Errorf("expected ErrNoSuchKey after deletion, got %v", err)
	}
}

func TestBackend_CopyObject(t *testing.T) {
	b, _ := setupBackend(t)
	ctx := rootCtx()

	b.CreateBucket(ctx, "src-bucket", "us-east-1", "private", false)
	b.CreateBucket(ctx, "dst-bucket", "us-east-1", "private", false)

	content := []byte("copy-me")
	b.PutObject(ctx, "src-bucket", "original.txt", bytes.NewReader(content), int64(len(content)),
		storage.ObjectMeta{ContentType: "text/plain"})

	copyResult, err := b.CopyObject(ctx, "src-bucket", "original.txt", "dst-bucket", "copied.txt", nil)
	if err != nil {
		t.Fatalf("CopyObject: %v", err)
	}
	if copyResult.ETag == "" {
		t.Error("CopyObject ETag should not be empty")
	}

	// Verify destination
	obj, err := b.GetObject(ctx, "dst-bucket", "copied.txt", storage.GetOptions{})
	if err != nil {
		t.Fatalf("GetObject destination: %v", err)
	}
	defer obj.Content.Close()

	got, _ := io.ReadAll(obj.Content)
	if !bytes.Equal(got, content) {
		t.Errorf("copied content = %q, want %q", got, content)
	}

	// Source should still exist
	_, err = b.GetObject(ctx, "src-bucket", "original.txt", storage.GetOptions{})
	if err != nil {
		t.Errorf("source should still exist: %v", err)
	}
}

func TestBackend_ListObjects_Prefix(t *testing.T) {
	b, _ := setupBackend(t)
	ctx := rootCtx()

	b.CreateBucket(ctx, "list-bucket", "us-east-1", "private", false)

	for _, key := range []string{"photos/a.jpg", "photos/b.jpg", "docs/readme.md", "root.txt"} {
		b.PutObject(ctx, "list-bucket", key, bytes.NewReader([]byte("x")), 1, storage.ObjectMeta{})
	}

	result, err := b.ListObjects(ctx, "list-bucket", storage.ListOptions{Prefix: "photos/", MaxKeys: 1000})
	if err != nil {
		t.Fatalf("ListObjects: %v", err)
	}
	if len(result.Contents) != 2 {
		t.Errorf("expected 2 objects with prefix 'photos/', got %d", len(result.Contents))
	}
}

func TestBackend_ListObjects_Delimiter(t *testing.T) {
	b, _ := setupBackend(t)
	ctx := rootCtx()

	b.CreateBucket(ctx, "delim-bucket", "us-east-1", "private", false)

	for _, key := range []string{"photos/a.jpg", "photos/b.jpg", "docs/readme.md", "root.txt"} {
		b.PutObject(ctx, "delim-bucket", key, bytes.NewReader([]byte("x")), 1, storage.ObjectMeta{})
	}

	result, err := b.ListObjects(ctx, "delim-bucket", storage.ListOptions{Delimiter: "/", MaxKeys: 1000})
	if err != nil {
		t.Fatalf("ListObjects: %v", err)
	}

	// Should have 1 object (root.txt) and 2 common prefixes (photos/, docs/)
	if len(result.Contents) != 1 {
		t.Errorf("expected 1 object at root level, got %d", len(result.Contents))
	}
	if len(result.CommonPrefixes) != 2 {
		t.Errorf("expected 2 common prefixes, got %d: %v", len(result.CommonPrefixes), result.CommonPrefixes)
	}
}

func TestBackend_GetObject_Range(t *testing.T) {
	b, _ := setupBackend(t)
	ctx := rootCtx()

	b.CreateBucket(ctx, "range-bucket", "us-east-1", "private", false)

	content := []byte("0123456789")
	b.PutObject(ctx, "range-bucket", "data.bin", bytes.NewReader(content), int64(len(content)), storage.ObjectMeta{})

	start := int64(2)
	end := int64(5)
	obj, err := b.GetObject(ctx, "range-bucket", "data.bin", storage.GetOptions{
		RangeStart: &start,
		RangeEnd:   &end,
	})
	if err != nil {
		t.Fatalf("GetObject range: %v", err)
	}
	defer obj.Content.Close()

	got, _ := io.ReadAll(obj.Content)
	want := "2345"
	if string(got) != want {
		t.Errorf("range content = %q, want %q", got, want)
	}
}

func TestBackend_PutObject_Overwrite(t *testing.T) {
	b, _ := setupBackend(t)
	ctx := rootCtx()

	b.CreateBucket(ctx, "overwrite-bucket", "us-east-1", "private", false)

	r1, _ := b.PutObject(ctx, "overwrite-bucket", "file.txt", bytes.NewReader([]byte("version1")), 8, storage.ObjectMeta{})
	r2, _ := b.PutObject(ctx, "overwrite-bucket", "file.txt", bytes.NewReader([]byte("version2")), 8, storage.ObjectMeta{})

	if r1.ETag == r2.ETag {
		t.Error("ETags should differ for different content")
	}

	obj, _ := b.GetObject(ctx, "overwrite-bucket", "file.txt", storage.GetOptions{})
	defer obj.Content.Close()
	got, _ := io.ReadAll(obj.Content)
	if string(got) != "version2" {
		t.Errorf("expected latest content 'version2', got %q", got)
	}
}
