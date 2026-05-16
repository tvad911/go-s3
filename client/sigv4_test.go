package client_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"gos3/client"
)

func TestSigV4_SignGET(t *testing.T) {
	req, _ := http.NewRequest("GET", "http://s3.example.com/bucket/key", nil)
	req.Host = "s3.example.com"

	client.SignRequestV4(req, "AKID", "SECRET", "us-east-1", "s3", time.Now())

	auth := req.Header.Get("Authorization")
	if auth == "" {
		t.Fatal("Authorization header should be set")
	}
	if !strings.HasPrefix(auth, "AWS4-HMAC-SHA256") {
		t.Errorf("Authorization should start with AWS4-HMAC-SHA256, got %q", auth[:30])
	}

	// Must contain Credential, SignedHeaders, Signature
	for _, part := range []string{"Credential=AKID", "SignedHeaders=", "Signature="} {
		if !strings.Contains(auth, part) {
			t.Errorf("Authorization missing %q", part)
		}
	}

	// x-amz-date must be set
	if req.Header.Get("x-amz-date") == "" {
		t.Error("x-amz-date header should be set")
	}

	// x-amz-content-sha256 must be set
	if req.Header.Get("x-amz-content-sha256") == "" {
		t.Error("x-amz-content-sha256 header should be set")
	}
}

func TestSigV4_SignPUTWithBody(t *testing.T) {
	body := strings.NewReader("upload content")
	req, _ := http.NewRequest("PUT", "http://s3.example.com/bucket/key", body)
	req.Host = "s3.example.com"

	client.SignRequestV4(req, "AKID", "SECRET", "us-east-1", "s3", time.Now())

	payloadHash := req.Header.Get("x-amz-content-sha256")
	if payloadHash == "" {
		t.Error("x-amz-content-sha256 should be set for PUT with body")
	}
	// Should NOT be UNSIGNED-PAYLOAD since we have actual body
	// (the signing code reads body and hashes it)
	if payloadHash == "UNSIGNED-PAYLOAD" {
		t.Error("payload hash should be actual SHA256, not UNSIGNED-PAYLOAD")
	}

	auth := req.Header.Get("Authorization")
	if !strings.Contains(auth, "Signature=") {
		t.Error("Authorization missing Signature")
	}
}

func TestSigV4_DifferentCredentials(t *testing.T) {
	req1, _ := http.NewRequest("GET", "http://s3.example.com/", nil)
	req2, _ := http.NewRequest("GET", "http://s3.example.com/", nil)
	req1.Host = "s3.example.com"
	req2.Host = "s3.example.com"

	now := time.Now()
	client.SignRequestV4(req1, "AKID1", "SECRET1", "us-east-1", "s3", now)
	client.SignRequestV4(req2, "AKID2", "SECRET2", "us-east-1", "s3", now)

	auth1 := req1.Header.Get("Authorization")
	auth2 := req2.Header.Get("Authorization")

	if auth1 == auth2 {
		t.Error("different credentials should produce different signatures")
	}
}

func TestSigV4_RegionInCredential(t *testing.T) {
	req, _ := http.NewRequest("GET", "http://s3.example.com/", nil)
	req.Host = "s3.example.com"

	client.SignRequestV4(req, "AKID", "SECRET", "eu-west-1", "s3", time.Now())

	auth := req.Header.Get("Authorization")
	if !strings.Contains(auth, "eu-west-1") {
		t.Errorf("Authorization should contain region 'eu-west-1', got %q", auth)
	}
}

func TestSigV4_TimestampFormat(t *testing.T) {
	req, _ := http.NewRequest("GET", "http://s3.example.com/", nil)
	req.Host = "s3.example.com"

	fixedTime := time.Date(2026, 1, 15, 10, 30, 45, 0, time.UTC)
	client.SignRequestV4(req, "AKID", "SECRET", "us-east-1", "s3", fixedTime)

	amzDate := req.Header.Get("x-amz-date")
	if amzDate != "20260115T103045Z" {
		t.Errorf("x-amz-date = %q, want 20260115T103045Z", amzDate)
	}
}

func TestSigV4_EmptyPath(t *testing.T) {
	req, _ := http.NewRequest("GET", "http://s3.example.com", nil)
	req.Host = "s3.example.com"

	// Should not panic
	client.SignRequestV4(req, "AKID", "SECRET", "us-east-1", "s3", time.Now())

	if req.Header.Get("Authorization") == "" {
		t.Error("should sign request with empty path")
	}
}
