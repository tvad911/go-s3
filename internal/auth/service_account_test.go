package auth_test

import (
	"testing"
	"time"

	"gos3/internal/auth"
)

func TestGenerateAccessKey_Uniqueness(t *testing.T) {
	key1, err := auth.GenerateAccessKey()
	if err != nil {
		t.Fatalf("GenerateAccessKey: %v", err)
	}
	key2, err := auth.GenerateAccessKey()
	if err != nil {
		t.Fatalf("GenerateAccessKey: %v", err)
	}

	if key1 == key2 {
		t.Error("two generated access keys should be different")
	}
}

func TestGenerateAccessKey_Length(t *testing.T) {
	key, err := auth.GenerateAccessKey()
	if err != nil {
		t.Fatalf("GenerateAccessKey: %v", err)
	}
	// 10 bytes → 20 hex chars
	if len(key) != 20 {
		t.Errorf("AccessKey length = %d, want 20", len(key))
	}
}

func TestGenerateSecretKey_Length(t *testing.T) {
	key, err := auth.GenerateSecretKey()
	if err != nil {
		t.Fatalf("GenerateSecretKey: %v", err)
	}
	// 20 bytes → 40 hex chars
	if len(key) != 40 {
		t.Errorf("SecretKey length = %d, want 40", len(key))
	}
}

func TestServiceAccount_IsExpired_NotExpired(t *testing.T) {
	future := time.Now().Add(1 * time.Hour)
	sa := &auth.ServiceAccount{
		ExpiresAt: &future,
	}
	if sa.IsExpired() {
		t.Error("service account with future expiry should not be expired")
	}
}

func TestServiceAccount_IsExpired_Expired(t *testing.T) {
	past := time.Now().Add(-1 * time.Hour)
	sa := &auth.ServiceAccount{
		ExpiresAt: &past,
	}
	if !sa.IsExpired() {
		t.Error("service account with past expiry should be expired")
	}
}

func TestServiceAccount_IsExpired_NoExpiry(t *testing.T) {
	sa := &auth.ServiceAccount{
		ExpiresAt: nil,
	}
	if sa.IsExpired() {
		t.Error("service account without expiry should never expire")
	}
}

func TestHashPassword_And_CheckPassword(t *testing.T) {
	hash, err := auth.HashPassword("mypassword")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if hash == "" {
		t.Error("hash should not be empty")
	}
	if hash == "mypassword" {
		t.Error("hash should not be plaintext")
	}

	// Check correct password
	if err := auth.CheckPassword(hash, "mypassword"); err != nil {
		t.Errorf("CheckPassword with correct pw: %v", err)
	}

	// Check wrong password
	if err := auth.CheckPassword(hash, "wrongpassword"); err != auth.ErrInvalidPassword {
		t.Errorf("CheckPassword with wrong pw: expected ErrInvalidPassword, got %v", err)
	}
}

func TestHashPassword_Empty(t *testing.T) {
	_, err := auth.HashPassword("")
	if err != auth.ErrPasswordRequired {
		t.Errorf("expected ErrPasswordRequired, got %v", err)
	}
}
