package auth_test

import (
	"bytes"
	"context"
	"net/http"
	"testing"
	"time"

	"gos3/client"
	"gos3/internal/auth"
)

// mockUserStore implements auth.UserStore for testing.
type mockUserStore struct {
	users map[string]*auth.User // keyed by username
}

func newMockUserStore() *mockUserStore {
	return &mockUserStore{users: make(map[string]*auth.User)}
}

func (m *mockUserStore) GetUserByAccessKey(_ context.Context, accessKey string) (*auth.User, error) {
	for _, u := range m.users {
		if u.AccessKeyID == accessKey {
			return u, nil
		}
	}
	return nil, auth.ErrUserNotFound
}

func (m *mockUserStore) GetUserByUsername(_ context.Context, username string) (*auth.User, error) {
	if u, ok := m.users[username]; ok {
		return u, nil
	}
	return nil, auth.ErrUserNotFound
}

func (m *mockUserStore) ListUsers(_ context.Context) ([]*auth.User, error) {
	var res []*auth.User
	for _, u := range m.users {
		res = append(res, u)
	}
	return res, nil
}

func (m *mockUserStore) CreateUser(_ context.Context, user *auth.User) error {
	if _, ok := m.users[user.Username]; ok {
		return auth.ErrUserExists
	}
	m.users[user.Username] = user
	return nil
}

func (m *mockUserStore) UpdateUser(_ context.Context, user *auth.User) error {
	m.users[user.Username] = user
	return nil
}

func (m *mockUserStore) DeleteUser(_ context.Context, username string) error {
	delete(m.users, username)
	return nil
}

// mockSAStore implements auth.ServiceAccountStore (empty — no SAs for basic SigV4 tests).
type mockSAStore struct{}

func (m *mockSAStore) CreateServiceAccount(_ context.Context, _ *auth.ServiceAccount) error {
	return nil
}

func (m *mockSAStore) GetServiceAccountByAccessKey(_ context.Context, _ string) (*auth.ServiceAccount, error) {
	return nil, auth.ErrUserNotFound
}

func (m *mockSAStore) ListServiceAccountsByUser(_ context.Context, _ string) ([]*auth.ServiceAccount, error) {
	return nil, nil
}

func (m *mockSAStore) DeleteServiceAccount(_ context.Context, _ string) error { return nil }

func (m *mockSAStore) DisableServiceAccount(_ context.Context, _ string, _ bool) error { return nil }

// setupVerifier creates a SigV4Verifier with a test user.
func setupVerifier(t *testing.T) (*auth.SigV4Verifier, *auth.User) {
	t.Helper()
	store := newMockUserStore()
	user := &auth.User{
		Username:    "testuser",
		AccessKeyID: "AKIAIOSFODNN7EXAMPLE",
		SecretKey:   "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
		IsRoot:      false,
	}
	store.CreateUser(context.Background(), user)

	v := auth.NewSigV4Verifier(store, &mockSAStore{}, "us-east-1")
	return v, user
}

// signTestReq creates and signs a request for testing.
func signTestReq(method, url, accessKey, secretKey string, body []byte) *http.Request {
	var bodyReader *bytes.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}
	var req *http.Request
	if bodyReader != nil {
		req, _ = http.NewRequest(method, url, bodyReader)
	} else {
		req, _ = http.NewRequest(method, url, nil)
	}
	req.Host = "s3.example.com"
	client.SignRequestV4(req, accessKey, secretKey, "us-east-1", "s3", time.Now())
	return req
}

func TestSigV4_ValidSignature(t *testing.T) {
	v, expectedUser := setupVerifier(t)

	req := signTestReq("GET", "http://s3.example.com/", expectedUser.AccessKeyID, expectedUser.SecretKey, nil)

	user, err := v.Verify(req)
	if err != nil {
		t.Fatalf("Verify failed: %v", err)
	}
	if user.Username != expectedUser.Username {
		t.Errorf("Username = %q, want %q", user.Username, expectedUser.Username)
	}
}

func TestSigV4_InvalidAccessKey(t *testing.T) {
	v, _ := setupVerifier(t)

	req := signTestReq("GET", "http://s3.example.com/", "INVALID_KEY", "somesecret", nil)

	_, err := v.Verify(req)
	if err == nil {
		t.Fatal("expected error for invalid access key")
	}
}

func TestSigV4_WrongSecretKey(t *testing.T) {
	v, user := setupVerifier(t)

	req := signTestReq("GET", "http://s3.example.com/", user.AccessKeyID, "WRONG_SECRET_KEY_HERE!!!!!!!!!!!!", nil)

	_, err := v.Verify(req)
	if err != auth.ErrSignatureDoesNotMatch {
		t.Errorf("expected ErrSignatureDoesNotMatch, got %v", err)
	}
}

func TestSigV4_ExpiredRequest(t *testing.T) {
	v, user := setupVerifier(t)

	// Sign with a time 20 minutes in the past
	req, _ := http.NewRequest("GET", "http://s3.example.com/", nil)
	req.Host = "s3.example.com"
	pastTime := time.Now().Add(-20 * time.Minute)
	client.SignRequestV4(req, user.AccessKeyID, user.SecretKey, "us-east-1", "s3", pastTime)

	_, err := v.Verify(req)
	if err != auth.ErrRequestTimeTooSkewed {
		t.Errorf("expected ErrRequestTimeTooSkewed, got %v", err)
	}
}

func TestSigV4_MissingAuthHeader(t *testing.T) {
	v, _ := setupVerifier(t)

	req, _ := http.NewRequest("GET", "http://s3.example.com/", nil)
	req.Host = "s3.example.com"

	_, err := v.Verify(req)
	if err != auth.ErrAuthHeaderMissing {
		t.Errorf("expected ErrAuthHeaderMissing, got %v", err)
	}
}

func TestSigV4_MalformedAuthHeader(t *testing.T) {
	v, _ := setupVerifier(t)

	req, _ := http.NewRequest("GET", "http://s3.example.com/", nil)
	req.Host = "s3.example.com"
	req.Header.Set("Authorization", "Bearer some-token")

	_, err := v.Verify(req)
	if err != auth.ErrAuthHeaderMalformed {
		t.Errorf("expected ErrAuthHeaderMalformed, got %v", err)
	}
}

func TestSigV4_WithBody(t *testing.T) {
	v, user := setupVerifier(t)

	body := []byte("hello world")
	req := signTestReq("PUT", "http://s3.example.com/bucket/key", user.AccessKeyID, user.SecretKey, body)

	authedUser, err := v.Verify(req)
	if err != nil {
		t.Fatalf("Verify with body failed: %v", err)
	}
	if authedUser.Username != user.Username {
		t.Errorf("Username = %q, want %q", authedUser.Username, user.Username)
	}
}

func TestSigV4_FutureRequest(t *testing.T) {
	v, user := setupVerifier(t)

	req, _ := http.NewRequest("GET", "http://s3.example.com/", nil)
	req.Host = "s3.example.com"
	futureTime := time.Now().Add(20 * time.Minute)
	client.SignRequestV4(req, user.AccessKeyID, user.SecretKey, "us-east-1", "s3", futureTime)

	_, err := v.Verify(req)
	if err != auth.ErrRequestTimeTooSkewed {
		t.Errorf("expected ErrRequestTimeTooSkewed for future request, got %v", err)
	}
}
