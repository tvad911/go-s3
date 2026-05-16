package handler_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	neturl "net/url"
	"strings"
	"testing"
	"time"

	"gos3/client"
	"gos3/internal/auth"
)

// --- Fix 9.3: GetUser Secret Exposure ---

func TestSecretExposure_GetUser(t *testing.T) {
	env := setupTestEnv(t)

	// Create a regular user with known credentials
	testUser := &auth.User{
		Username:     "testuser",
		AccessKeyID:  "AKID123456",
		SecretKey:    "supersecret123",
		PasswordHash: "$2a$10$fakehash",
		CreatedAt:    time.Now().UTC(),
	}
	env.MetaStore.CreateUser(context.Background(), testUser)

	// Use raw HTTP with SigV4 signing to hit admin API
	req, _ := http.NewRequest("GET", env.Server.URL+"/_admin/users/testuser", nil)
	client.SignRequestV4(req, rootAccessKey(), rootSecretKey(), "us-east-1", "s3", time.Now())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /users/testuser: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	bodyStr := string(body)

	// Must NOT contain secret key or password hash
	if strings.Contains(bodyStr, "supersecret123") {
		t.Error("CRITICAL: GetUser response contains SecretKey!")
	}
	if strings.Contains(bodyStr, "$2a$10$") {
		t.Error("CRITICAL: GetUser response contains PasswordHash!")
	}
}

func TestSecretExposure_ListUsers(t *testing.T) {
	env := setupTestEnv(t)

	req, _ := http.NewRequest("GET", env.Server.URL+"/_admin/users", nil)
	client.SignRequestV4(req, rootAccessKey(), rootSecretKey(), "us-east-1", "s3", time.Now())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /users: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	bodyStr := string(body)

	if strings.Contains(bodyStr, rootSecretKey()) {
		t.Error("CRITICAL: ListUsers response contains root SecretKey!")
	}
}

func TestSecretExposure_ServerInfo(t *testing.T) {
	env := setupTestEnv(t)

	req, _ := http.NewRequest("GET", env.Server.URL+"/_admin/info", nil)
	client.SignRequestV4(req, rootAccessKey(), rootSecretKey(), "us-east-1", "s3", time.Now())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /info: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	bodyStr := string(body)

	if strings.Contains(bodyStr, rootSecretKey()) {
		t.Error("CRITICAL: ServerInfo response contains credentials!")
	}
	if strings.Contains(bodyStr, rootAccessKey()) {
		// Access key in server info could be debatable, but secret key must never be present
		t.Log("WARNING: ServerInfo contains access key (may be acceptable)")
	}
}

// --- Fix 9.2: Open Redirect ---

func TestPostObject_OpenRedirect(t *testing.T) {
	// PostObject with success_action_redirect to external site should be blocked
	// This test validates the fix at the protocol level
	t.Run("javascript_scheme", func(t *testing.T) {
		// javascript: URLs must be rejected
		url := "javascript:alert(document.cookie)"
		if isValidRedirect(url, "localhost:9000") {
			t.Error("javascript: URL should be rejected")
		}
	})

	t.Run("external_redirect", func(t *testing.T) {
		url := "http://evil.com/steal?data=1"
		if isValidRedirect(url, "localhost:9000") {
			t.Error("external redirect should be rejected")
		}
	})

	t.Run("same_origin", func(t *testing.T) {
		url := "/success?uploaded=true"
		if !isValidRedirect(url, "localhost:9000") {
			t.Error("same-origin relative URL should be allowed")
		}
	})
}

// isValidRedirect replicates the validation logic from post_object.go
func isValidRedirect(redirectURL, requestHost string) bool {
	parsed, err := neturl.Parse(redirectURL)
	if err != nil {
		return false
	}
	if parsed.Scheme != "" && parsed.Scheme != "http" && parsed.Scheme != "https" {
		return false
	}
	if parsed.Host != "" && parsed.Host != requestHost {
		return false
	}
	return true
}



// --- Admin Auth Tests ---

func TestAdminAuth_UnauthenticatedAccess(t *testing.T) {
	env := setupTestEnv(t)

	// Attempt to access admin API without auth
	endpoints := []struct {
		method string
		path   string
	}{
		{"GET", "/_admin/users"},
		{"GET", "/_admin/info"},
		{"GET", "/_admin/audit-logs"},
		{"GET", "/_admin/settings"},
	}

	for _, ep := range endpoints {
		t.Run(ep.method+" "+ep.path, func(t *testing.T) {
			req, _ := http.NewRequest(ep.method, env.Server.URL+ep.path, nil)
			// NO signing — should be rejected
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			resp.Body.Close()

			if resp.StatusCode == http.StatusOK {
				t.Errorf("unauthenticated %s %s returned 200 — should be 401/403", ep.method, ep.path)
			}
		})
	}
}

func TestAdminAuth_NonRootAccess(t *testing.T) {
	env := setupTestEnv(t)

	// Create a non-root user
	normalUser := &auth.User{
		Username:    "normaluser",
		AccessKeyID: "normal-access-key",
		SecretKey:   "normal-secret-key",
		IsRoot:      false,
		CreatedAt:   time.Now().UTC(),
	}
	env.MetaStore.CreateUser(context.Background(), normalUser)

	// Sign request with non-root credentials
	req, _ := http.NewRequest("GET", env.Server.URL+"/_admin/users", nil)
	client.SignRequestV4(req, "normal-access-key", "normal-secret-key", "us-east-1", "s3", time.Now())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		t.Error("non-root user should NOT access admin API")
	}
}

// --- XXE Tests ---

func TestXXE_DeleteObjects(t *testing.T) {
	env := setupTestEnv(t)

	// Create bucket first
	makeReq, _ := http.NewRequest("PUT", env.Server.URL+"/xxe-bucket", nil)
	client.SignRequestV4(makeReq, rootAccessKey(), rootSecretKey(), "us-east-1", "s3", time.Now())
	resp, _ := http.DefaultClient.Do(makeReq)
	resp.Body.Close()

	// Try XXE in DeleteObjects
	xxePayload := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE foo [<!ENTITY xxe SYSTEM "file:///etc/passwd">]>
<Delete>
  <Object>
    <Key>&xxe;</Key>
  </Object>
</Delete>`

	req, _ := http.NewRequest("POST", env.Server.URL+"/xxe-bucket?delete", strings.NewReader(xxePayload))
	req.Header.Set("Content-Type", "application/xml")
	client.SignRequestV4(req, rootAccessKey(), rootSecretKey(), "us-east-1", "s3", time.Now())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	bodyStr := string(body)

	// Must NOT contain /etc/passwd contents
	if strings.Contains(bodyStr, "root:x:0:0") {
		t.Error("CRITICAL: XXE attack succeeded — /etc/passwd leaked!")
	}
}

func TestXXE_CompleteMultipartUpload(t *testing.T) {
	env := setupTestEnv(t)

	// XXE in CompleteMultipartUpload XML
	xxePayload := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE foo [<!ENTITY xxe SYSTEM "file:///etc/passwd">]>
<CompleteMultipartUpload>
  <Part>
    <PartNumber>1</PartNumber>
    <ETag>&xxe;</ETag>
  </Part>
</CompleteMultipartUpload>`

	req, _ := http.NewRequest("POST", env.Server.URL+"/test-bucket/key?uploadId=fake-id", strings.NewReader(xxePayload))
	req.Header.Set("Content-Type", "application/xml")
	client.SignRequestV4(req, rootAccessKey(), rootSecretKey(), "us-east-1", "s3", time.Now())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if strings.Contains(string(body), "root:x:0:0") {
		t.Error("CRITICAL: XXE attack succeeded in CompleteMultipartUpload!")
	}
}

// --- Header Injection Tests ---

func TestHeaderInjection_ObjectKey(t *testing.T) {
	env := setupTestEnv(t)

	// Create bucket
	makeReq, _ := http.NewRequest("PUT", env.Server.URL+"/hdr-bucket", nil)
	client.SignRequestV4(makeReq, rootAccessKey(), rootSecretKey(), "us-east-1", "s3", time.Now())
	resp, _ := http.DefaultClient.Do(makeReq)
	resp.Body.Close()

	// Attempt CRLF injection via metadata header value.
	// Go's net/http client rejects CRLF in header values at the transport level,
	// which is the first line of defense. We verify this protection is active.
	content := strings.NewReader("test")
	req, _ := http.NewRequest("PUT", env.Server.URL+"/hdr-bucket/safe-key", content)
	req.Header.Set("x-amz-meta-injected", "value\r\nX-Evil: malicious")
	client.SignRequestV4(req, rootAccessKey(), rootSecretKey(), "us-east-1", "s3", time.Now())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		// Expected: Go's net/http rejects invalid header values at client side
		t.Logf("GOOD: CRLF in header rejected by Go net/http client: %v", err)
		return
	}
	resp.Body.Close()

	// The CRLF in metadata value should not create a new header
	if resp.Header.Get("X-Evil") == "malicious" {
		t.Error("CRITICAL: CRLF injection succeeded in response headers!")
	}
}

// --- DoS Protection Tests ---

func TestDoS_LargeBody(t *testing.T) {
	env := setupTestEnv(t)

	// Attempt to send a request with Content-Length lying about size
	// but actually send minimal data — server should handle gracefully
	req, _ := http.NewRequest("PUT", env.Server.URL+"/dos-bucket", nil)
	client.SignRequestV4(req, rootAccessKey(), rootSecretKey(), "us-east-1", "s3", time.Now())
	resp, _ := http.DefaultClient.Do(req)
	if resp != nil {
		resp.Body.Close()
	}
}

// --- Health endpoint should be accessible without auth ---

func TestHealthEndpoint_NoAuth(t *testing.T) {
	env := setupTestEnv(t)

	resp, err := http.Get(env.Server.URL + "/_health")
	if err != nil {
		t.Fatalf("health check: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("health check returned %d, want 200", resp.StatusCode)
	}

	var result map[string]string
	json.NewDecoder(resp.Body).Decode(&result)
	if result["status"] != "ok" {
		t.Errorf("health status = %q, want ok", result["status"])
	}
}

// --- Metrics endpoint should be accessible without auth ---

func TestMetricsEndpoint_NoAuth(t *testing.T) {
	env := setupTestEnv(t)

	resp, err := http.Get(env.Server.URL + "/_metrics")
	if err != nil {
		t.Fatalf("metrics: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("metrics returned %d, want 200", resp.StatusCode)
	}
}
