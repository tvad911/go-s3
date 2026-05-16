package handler_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"gos3/internal/auth"
	"gos3/internal/config"
	"gos3/internal/handler"
	"gos3/internal/server"
	"gos3/internal/storage/local"
	"gos3/internal/storage/metadata"
)

// testEnv holds all components needed for integration tests.
type testEnv struct {
	Server    *httptest.Server
	MetaStore metadata.Store
	Backend   *local.Backend
	Config    *config.Config
	Verifier  *auth.SigV4Verifier
	Handler   *handler.S3Handler
}

// setupTestEnv creates a fully wired test environment with real storage
// backed by t.TempDir(). The returned testEnv.Server is already started.
// Cleanup is automatic via t.Cleanup.
func setupTestEnv(t *testing.T) *testEnv {
	t.Helper()

	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	tempDir := filepath.Join(tmpDir, "tmp")
	metaDBPath := filepath.Join(tmpDir, "meta.db")

	// Create metadata store
	metaStore, err := metadata.NewBboltStore(metaDBPath)
	if err != nil {
		t.Fatalf("failed to create metadata store: %v", err)
	}
	t.Cleanup(func() { metaStore.Close() })

	// Create storage backend
	backend, err := local.NewBackend(dataDir, tempDir, metaStore)
	if err != nil {
		t.Fatalf("failed to create storage backend: %v", err)
	}

	// Create a test config with root credentials
	cfg := testConfig()

	// Seed root user into the store so SigV4 lookup works
	rootUser := &auth.User{
		Username:    "root",
		AccessKeyID: cfg.Auth.RootAccessKey,
		SecretKey:   cfg.Auth.RootSecretKey,
		IsRoot:      true,
		CreatedAt:   time.Now().UTC(),
	}
	if err := metaStore.CreateUser(context.Background(), rootUser); err != nil {
		t.Fatalf("failed to seed root user: %v", err)
	}

	// Create SigV4 verifier
	verifier := auth.NewSigV4Verifier(metaStore, metaStore, cfg.Auth.Region)

	// Create session config
	sessionCfg := auth.DefaultSessionConfig([]byte("test-signing-key-32-bytes-long!!"))

	// Build the full router (same as production)
	router := server.SetupRouter(cfg, backend, metaStore, verifier, nil, sessionCfg)

	// Start httptest server
	srv := httptest.NewServer(router)
	t.Cleanup(func() { srv.Close() })

	// Build handler for direct access if needed
	engine := auth.NewEngine(metaStore, metaStore)
	s3Handler := handler.NewS3Handler(backend, metaStore, verifier, cfg, nil, engine)

	return &testEnv{
		Server:    srv,
		MetaStore: metaStore,
		Backend:   backend,
		Config:    cfg,
		Verifier:  verifier,
		Handler:   s3Handler,
	}
}

// testConfig returns a minimal valid config for testing.
func testConfig() *config.Config {
	return &config.Config{
		Server: config.ServerConfig{
			Host:              "0.0.0.0",
			Port:              9000,
			ReadHeaderTimeout: 10 * time.Second,
			MaxHeaderBytes:    1 << 20,
		},
		Storage: config.StorageConfig{
			DataDir:       "/tmp/test-data",
			TempDir:       "/tmp/test-tmp",
			MetaDB:        "/tmp/test-meta.db",
			MaxObjectSize: 5 * 1024 * 1024 * 1024,
		},
		Auth: config.AuthConfig{
			RootAccessKey: "testaccesskey",
			RootSecretKey: "testsecretkey",
			Region:        "us-east-1",
		},
		Log: config.LogConfig{
			Level:  "info",
			Format: "text",
			Output: "stdout",
		},
		RateLimit: config.RateLimitConfig{
			Enabled:           false, // Disable rate limiting in tests
			RequestsPerSecond: 10000,
			Burst:             10000,
		},
		Metrics: config.MetricsConfig{
			Enabled: true,
			Path:    "/_metrics",
		},
		Admin: config.AdminConfig{
			APIEnabled: true,
			PathPrefix: "/_admin",
			UIEnabled:  false,
		},
	}
}

// signedRequest creates an HTTP request signed with SigV4 using root credentials.
// This helper makes integration tests concise by handling the signing boilerplate.
func signedRequest(t *testing.T, method, url string, body *http.Request) *http.Request {
	t.Helper()
	// For integration tests, we use the client library's signing.
	// But since handler_test is a black-box package, we rely on the test server
	// which includes the auth middleware. We send unsigned requests for routes
	// that don't need auth, or use the client package for signed requests.
	// This function is a placeholder — actual signing is done via the client package.
	return nil
}

// rootAccessKey returns the test root access key.
func rootAccessKey() string {
	return "testaccesskey"
}

// rootSecretKey returns the test root secret key.
func rootSecretKey() string {
	return "testsecretkey"
}
