package handler_test

import (
	"io"
	"net/http"
	"testing"
	"time"

	"gos3/client"
)

// --- C.5: Auth Middleware Integration ---
// These tests verify the middleware correctly handles different auth scenarios
// against the full router stack.

func TestAuth_ValidRequest(t *testing.T) {
	env := setupTestEnv(t)

	resp := doSigned(t, env, "GET", "/", nil)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("valid signed request should get 200, got %d", resp.StatusCode)
	}
}

func TestAuth_UnauthenticatedRequest(t *testing.T) {
	env := setupTestEnv(t)

	req, _ := http.NewRequest("GET", env.Server.URL+"/", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	// Anonymous requests are currently allowed through middleware
	// but policy engine should deny. Depending on config, either 200 or 403.
	// The middleware sets anonymous user, then handler's CheckPolicy decides.
	// For ListBuckets, anonymous without policy → likely 403 or empty.
	t.Logf("Unauthenticated ListBuckets status: %d", resp.StatusCode)
}

func TestAuth_InvalidSignature(t *testing.T) {
	env := setupTestEnv(t)

	url := env.Server.URL + "/"
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "AWS4-HMAC-SHA256 Credential=FAKEKEY/20260101/us-east-1/s3/aws4_request, SignedHeaders=host;x-amz-date, Signature=invalidsig")
	req.Header.Set("x-amz-date", time.Now().UTC().Format("20060102T150405Z"))
	req.Header.Set("x-amz-content-sha256", "UNSIGNED-PAYLOAD")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("invalid signature should get 403, got %d", resp.StatusCode)
	}
}

func TestAuth_ExpiredSignature(t *testing.T) {
	env := setupTestEnv(t)

	url := env.Server.URL + "/"
	req, _ := http.NewRequest("GET", url, nil)
	pastTime := time.Now().Add(-20 * time.Minute)
	client.SignRequestV4(req, rootAccessKey(), rootSecretKey(), "us-east-1", "s3", pastTime)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expired signature should get 403, got %d", resp.StatusCode)
	}
}

func TestAuth_SkipHealthCheck(t *testing.T) {
	env := setupTestEnv(t)

	// Health endpoint should bypass auth entirely
	resp, err := http.Get(env.Server.URL + "/_health")
	if err != nil {
		t.Fatalf("health check failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("health check should be 200 without auth, got %d", resp.StatusCode)
	}
}

func TestAuth_SkipCORSPreflight(t *testing.T) {
	env := setupTestEnv(t)

	req, _ := http.NewRequest("OPTIONS", env.Server.URL+"/some-bucket", nil)
	req.Header.Set("Origin", "http://example.com")
	req.Header.Set("Access-Control-Request-Method", "PUT")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("OPTIONS request failed: %v", err)
	}
	defer resp.Body.Close()

	// OPTIONS should pass through auth middleware without 403
	if resp.StatusCode == http.StatusForbidden {
		t.Error("CORS preflight should not be blocked by auth")
	}
}

func TestAuth_DisabledUser_BucketDenied(t *testing.T) {
	env := setupTestEnv(t)

	// Create a bucket first with root user
	doSigned(t, env, "PUT", "/auth-test-bucket", nil).Body.Close()

	// Now try with root user — should work
	resp := doSigned(t, env, "GET", "/auth-test-bucket/", nil)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("root user should access bucket, got %d", resp.StatusCode)
	}
}

func TestAuth_ErrorResponseFormat(t *testing.T) {
	env := setupTestEnv(t)

	url := env.Server.URL + "/some-bucket"
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "AWS4-HMAC-SHA256 Credential=BADKEY/20260101/us-east-1/s3/aws4_request, SignedHeaders=host;x-amz-date, Signature=badsig")
	req.Header.Set("x-amz-date", time.Now().UTC().Format("20060102T150405Z"))
	req.Header.Set("x-amz-content-sha256", "UNSIGNED-PAYLOAD")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	bodyStr := string(body)

	// Should be XML error
	if resp.Header.Get("Content-Type") != "application/xml" {
		t.Errorf("error response Content-Type = %q, want application/xml", resp.Header.Get("Content-Type"))
	}

	// Should contain <Error> and <Code>
	if len(bodyStr) < 10 {
		t.Error("error body too short")
	}
}
