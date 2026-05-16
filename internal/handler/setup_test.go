package handler_test

import (
	"io"
	"net/http"
	"testing"
)

// TestSetupTestEnv_HealthCheck verifies that setupTestEnv creates a working
// httptest.Server with the full GoS3 router. The /_health endpoint is
// unauthenticated and should return 200 OK with {"status":"ok"}.
func TestSetupTestEnv_HealthCheck(t *testing.T) {
	env := setupTestEnv(t)

	resp, err := http.Get(env.Server.URL + "/_health")
	if err != nil {
		t.Fatalf("GET /_health failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}

	expected := `{"status":"ok"}`
	if string(body) != expected {
		t.Errorf("expected body %q, got %q", expected, string(body))
	}
}

// TestSetupTestEnv_MetricsEndpoint verifies the metrics endpoint is reachable.
func TestSetupTestEnv_MetricsEndpoint(t *testing.T) {
	env := setupTestEnv(t)

	resp, err := http.Get(env.Server.URL + "/_metrics")
	if err != nil {
		t.Fatalf("GET /_metrics failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
}

// TestSetupTestEnv_UnauthenticatedS3Request verifies that S3 API endpoints
// reject unauthenticated requests (no Authorization header).
func TestSetupTestEnv_UnauthenticatedS3Request(t *testing.T) {
	env := setupTestEnv(t)

	// GET / (ListBuckets) without auth should return 403
	resp, err := http.Get(env.Server.URL + "/")
	if err != nil {
		t.Fatalf("GET / failed: %v", err)
	}
	defer resp.Body.Close()

	// The auth middleware should reject with 403
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected status 403 for unauthenticated request, got %d", resp.StatusCode)
	}
}
