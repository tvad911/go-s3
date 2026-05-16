package client_test

import (
	"context"
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gos3/client"
	"gos3/internal/auth"
	"gos3/internal/config"
	"gos3/internal/server"
	"gos3/internal/storage/local"
	"gos3/internal/storage/metadata"
)

const (
	testAccessKey = "testaccesskey"
	testSecretKey = "testsecretkey"
)

// setupClientTestEnv creates a test server + configured client.
func setupClientTestEnv(t *testing.T) *client.Client {
	t.Helper()

	tmpDir := t.TempDir()
	metaStore, err := metadata.NewBboltStore(filepath.Join(tmpDir, "meta.db"))
	if err != nil {
		t.Fatalf("NewBboltStore: %v", err)
	}
	t.Cleanup(func() { metaStore.Close() })

	backend, err := local.NewBackend(
		filepath.Join(tmpDir, "data"),
		filepath.Join(tmpDir, "tmp"),
		metaStore,
	)
	if err != nil {
		t.Fatalf("NewBackend: %v", err)
	}

	cfg := &config.Config{
		Server:  config.ServerConfig{Host: "0.0.0.0", Port: 9000, ReadHeaderTimeout: 10 * time.Second, MaxHeaderBytes: 1 << 20},
		Storage: config.StorageConfig{MaxObjectSize: 5 * 1024 * 1024 * 1024},
		Auth:    config.AuthConfig{RootAccessKey: testAccessKey, RootSecretKey: testSecretKey, Region: "us-east-1"},
		Log:     config.LogConfig{Level: "info", Format: "text", Output: "stdout"},
		RateLimit: config.RateLimitConfig{Enabled: false, RequestsPerSecond: 100000, Burst: 100000},
		Metrics:   config.MetricsConfig{Enabled: true, Path: "/_metrics"},
		Admin:     config.AdminConfig{APIEnabled: true, PathPrefix: "/_admin", UIEnabled: false},
	}

	rootUser := &auth.User{
		Username: "root", AccessKeyID: testAccessKey, SecretKey: testSecretKey,
		IsRoot: true, CreatedAt: time.Now().UTC(),
	}
	metaStore.CreateUser(context.Background(), rootUser)

	verifier := auth.NewSigV4Verifier(metaStore, metaStore, cfg.Auth.Region)
	sessionCfg := auth.DefaultSessionConfig([]byte("test-signing-key-32-bytes-long!!"))
	router := server.SetupRouter(cfg, backend, metaStore, verifier, nil, sessionCfg)

	srv := httptest.NewServer(router)
	t.Cleanup(func() { srv.Close() })

	c, err := client.New(client.Config{
		Endpoint:        srv.URL,
		AccessKeyID:     testAccessKey,
		SecretAccessKey: testSecretKey,
		Region:          "us-east-1",
		PathStyle:       true,
	})
	if err != nil {
		t.Fatalf("client.New: %v", err)
	}

	return c
}

func TestClient_MakeBucket_ListBuckets(t *testing.T) {
	c := setupClientTestEnv(t)
	ctx := context.Background()

	if err := c.MakeBucket(ctx, "test-bucket"); err != nil {
		t.Fatalf("MakeBucket: %v", err)
	}

	buckets, err := c.ListBuckets(ctx)
	if err != nil {
		t.Fatalf("ListBuckets: %v", err)
	}

	found := false
	for _, b := range buckets {
		if b.Name == "test-bucket" {
			found = true
			break
		}
	}
	if !found {
		t.Error("test-bucket not found in ListBuckets result")
	}
}

func TestClient_BucketExists(t *testing.T) {
	c := setupClientTestEnv(t)
	ctx := context.Background()

	c.MakeBucket(ctx, "exists-bucket")

	exists, err := c.BucketExists(ctx, "exists-bucket")
	if err != nil {
		t.Fatalf("BucketExists: %v", err)
	}
	if !exists {
		t.Error("expected bucket to exist")
	}

	exists, err = c.BucketExists(ctx, "nope-bucket")
	if err != nil {
		t.Fatalf("BucketExists: %v", err)
	}
	if exists {
		t.Error("expected bucket to NOT exist")
	}
}

func TestClient_PutObject_GetObject(t *testing.T) {
	c := setupClientTestEnv(t)
	ctx := context.Background()

	c.MakeBucket(ctx, "obj-bucket")

	content := "hello from Go client!"
	err := c.PutObject(ctx, "obj-bucket", "greet.txt",
		strings.NewReader(content), int64(len(content)),
		client.PutObjectOptions{ContentType: "text/plain"})
	if err != nil {
		t.Fatalf("PutObject: %v", err)
	}

	reader, info, err := c.GetObject(ctx, "obj-bucket", "greet.txt", client.GetObjectOptions{})
	if err != nil {
		t.Fatalf("GetObject: %v", err)
	}
	defer reader.Close()

	got, _ := io.ReadAll(reader)
	if string(got) != content {
		t.Errorf("content mismatch: got %q, want %q", got, content)
	}
	if info.ETag == "" {
		t.Error("ETag should not be empty")
	}
}

func TestClient_FPutObject_FGetObject(t *testing.T) {
	c := setupClientTestEnv(t)
	ctx := context.Background()

	c.MakeBucket(ctx, "file-bucket")

	// Create local file
	srcFile := filepath.Join(t.TempDir(), "upload.txt")
	os.WriteFile(srcFile, []byte("file upload test"), 0644)

	if err := c.FPutObject(ctx, "file-bucket", "uploaded.txt", srcFile,
		client.PutObjectOptions{ContentType: "text/plain"}); err != nil {
		t.Fatalf("FPutObject: %v", err)
	}

	// Download to local file
	dstFile := filepath.Join(t.TempDir(), "downloaded.txt")
	if err := c.FGetObject(ctx, "file-bucket", "uploaded.txt", dstFile, client.GetObjectOptions{}); err != nil {
		t.Fatalf("FGetObject: %v", err)
	}

	got, _ := os.ReadFile(dstFile)
	if string(got) != "file upload test" {
		t.Errorf("downloaded content = %q, want %q", got, "file upload test")
	}
}

func TestClient_StatObject(t *testing.T) {
	c := setupClientTestEnv(t)
	ctx := context.Background()

	c.MakeBucket(ctx, "stat-bucket")
	content := "stat-me"
	c.PutObject(ctx, "stat-bucket", "file.txt",
		strings.NewReader(content), int64(len(content)),
		client.PutObjectOptions{ContentType: "text/plain"})

	info, err := c.StatObject(ctx, "stat-bucket", "file.txt")
	if err != nil {
		t.Fatalf("StatObject: %v", err)
	}
	if info.Size != int64(len(content)) {
		t.Errorf("Size = %d, want %d", info.Size, len(content))
	}
	if info.ETag == "" {
		t.Error("ETag should not be empty")
	}
}

func TestClient_RemoveObject(t *testing.T) {
	c := setupClientTestEnv(t)
	ctx := context.Background()

	c.MakeBucket(ctx, "rm-bucket")
	c.PutObject(ctx, "rm-bucket", "gone.txt",
		strings.NewReader("bye"), 3, client.PutObjectOptions{})

	if err := c.RemoveObject(ctx, "rm-bucket", "gone.txt"); err != nil {
		t.Fatalf("RemoveObject: %v", err)
	}

	// Verify gone
	_, _, err := c.GetObject(ctx, "rm-bucket", "gone.txt", client.GetObjectOptions{})
	if err == nil {
		t.Error("expected error after RemoveObject")
	}
}

func TestClient_CopyObject(t *testing.T) {
	c := setupClientTestEnv(t)
	ctx := context.Background()

	c.MakeBucket(ctx, "copy-src")
	c.MakeBucket(ctx, "copy-dst")
	c.PutObject(ctx, "copy-src", "original.txt",
		strings.NewReader("copy-me"), 7, client.PutObjectOptions{})

	if err := c.CopyObject(ctx, "copy-src", "original.txt", "copy-dst", "copied.txt"); err != nil {
		t.Fatalf("CopyObject: %v", err)
	}

	reader, _, err := c.GetObject(ctx, "copy-dst", "copied.txt", client.GetObjectOptions{})
	if err != nil {
		t.Fatalf("GetObject copied: %v", err)
	}
	defer reader.Close()
	got, _ := io.ReadAll(reader)
	if string(got) != "copy-me" {
		t.Errorf("copied content = %q, want %q", got, "copy-me")
	}
}

func TestClient_ListObjectsV2(t *testing.T) {
	c := setupClientTestEnv(t)
	ctx := context.Background()

	c.MakeBucket(ctx, "list-bucket")
	for _, key := range []string{"alpha.txt", "beta.txt", "gamma.txt"} {
		c.PutObject(ctx, "list-bucket", key,
			strings.NewReader("x"), 1, client.PutObjectOptions{})
	}

	var objects []client.Object
	for obj := range c.ListObjectsV2(ctx, "list-bucket", client.ListObjectsOptions{Recursive: true}) {
		objects = append(objects, obj)
	}

	if len(objects) != 3 {
		t.Errorf("expected 3 objects, got %d", len(objects))
	}
}

func TestClient_ListObjects_WithPrefix(t *testing.T) {
	c := setupClientTestEnv(t)
	ctx := context.Background()

	c.MakeBucket(ctx, "pfx-bucket")
	for _, key := range []string{"docs/a.md", "docs/b.md", "imgs/c.png"} {
		c.PutObject(ctx, "pfx-bucket", key,
			strings.NewReader("x"), 1, client.PutObjectOptions{})
	}

	var objects []client.Object
	for obj := range c.ListObjectsV2(ctx, "pfx-bucket", client.ListObjectsOptions{
		Prefix:    "docs/",
		Recursive: true,
	}) {
		objects = append(objects, obj)
	}

	if len(objects) != 2 {
		t.Errorf("expected 2 objects with prefix docs/, got %d", len(objects))
	}
}

func TestClient_RemoveBucket(t *testing.T) {
	c := setupClientTestEnv(t)
	ctx := context.Background()

	c.MakeBucket(ctx, "del-bucket")

	if err := c.RemoveBucket(ctx, "del-bucket"); err != nil {
		t.Fatalf("RemoveBucket: %v", err)
	}

	exists, _ := c.BucketExists(ctx, "del-bucket")
	if exists {
		t.Error("bucket should not exist after RemoveBucket")
	}
}

func TestClient_RemoveBucket_NonEmpty(t *testing.T) {
	c := setupClientTestEnv(t)
	ctx := context.Background()

	c.MakeBucket(ctx, "nonempty-bucket")
	c.PutObject(ctx, "nonempty-bucket", "file.txt",
		strings.NewReader("data"), 4, client.PutObjectOptions{})

	err := c.RemoveBucket(ctx, "nonempty-bucket")
	if err == nil {
		t.Error("expected error when removing non-empty bucket")
	}
}

func TestClient_PresignGetObject(t *testing.T) {
	c := setupClientTestEnv(t)
	ctx := context.Background()

	c.MakeBucket(ctx, "presign-bucket")
	content := "presigned-content"
	c.PutObject(ctx, "presign-bucket", "secret.txt",
		strings.NewReader(content), int64(len(content)), client.PutObjectOptions{})

	presignURL, err := c.PresignGetObject("presign-bucket", "secret.txt", 60*time.Second)
	if err != nil {
		t.Fatalf("PresignGetObject: %v", err)
	}

	// Verify URL contains required query params
	for _, param := range []string{"X-Amz-Algorithm", "X-Amz-Credential", "X-Amz-Date", "X-Amz-Expires", "X-Amz-Signature"} {
		if !strings.Contains(presignURL, param) {
			t.Errorf("presigned URL missing param %q", param)
		}
	}
}

func TestClient_RemoveObjects(t *testing.T) {
	c := setupClientTestEnv(t)
	ctx := context.Background()

	c.MakeBucket(ctx, "bulk-rm")
	for _, key := range []string{"a.txt", "b.txt", "c.txt"} {
		c.PutObject(ctx, "bulk-rm", key,
			strings.NewReader("x"), 1, client.PutObjectOptions{})
	}

	if err := c.RemoveObjects(ctx, "bulk-rm", []string{"a.txt", "b.txt", "c.txt"}); err != nil {
		t.Fatalf("RemoveObjects: %v", err)
	}

	// All should be gone
	var objects []client.Object
	for obj := range c.ListObjectsV2(ctx, "bulk-rm", client.ListObjectsOptions{Recursive: true}) {
		objects = append(objects, obj)
	}
	if len(objects) != 0 {
		t.Errorf("expected 0 objects after bulk remove, got %d", len(objects))
	}
}

func TestClient_PutObject_WithUserMeta(t *testing.T) {
	c := setupClientTestEnv(t)
	ctx := context.Background()

	c.MakeBucket(ctx, "meta-bucket")
	err := c.PutObject(ctx, "meta-bucket", "tagged.txt",
		strings.NewReader("meta"), 4,
		client.PutObjectOptions{
			ContentType: "text/markdown",
			UserMeta:    map[string]string{"author": "gos3"},
		})
	if err != nil {
		t.Fatalf("PutObject with meta: %v", err)
	}

	info, err := c.StatObject(ctx, "meta-bucket", "tagged.txt")
	if err != nil {
		t.Fatalf("StatObject: %v", err)
	}
	if info.ContentType != "text/markdown" {
		t.Errorf("ContentType = %q, want text/markdown", info.ContentType)
	}
}

func TestClient_GetObject_NotFound(t *testing.T) {
	c := setupClientTestEnv(t)
	ctx := context.Background()

	c.MakeBucket(ctx, "nf-bucket")

	_, _, err := c.GetObject(ctx, "nf-bucket", "missing.txt", client.GetObjectOptions{})
	if err == nil {
		t.Error("expected error for non-existent object")
	}
}
