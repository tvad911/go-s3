package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

// ServiceAccount represents an API key pair owned by a User.
// Each user can have multiple ServiceAccounts for different applications.
// Permissions are inherited entirely from the parent User's attached IAM Policies.
type ServiceAccount struct {
	ID          string     `json:"id"`
	AccessKeyID string     `json:"accessKeyId"`
	SecretKey   string     `json:"secretKey,omitempty"` // Only populated on creation
	ParentUser  string     `json:"parentUser"`
	Description string     `json:"description,omitempty"`
	Disabled    bool       `json:"disabled"`
	ExpiresAt   *time.Time `json:"expiresAt,omitempty"`
	CreatedAt   time.Time  `json:"createdAt"`
}

// IsExpired checks if this service account has expired.
func (sa *ServiceAccount) IsExpired() bool {
	if sa.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*sa.ExpiresAt)
}

// GenerateAccessKey creates a random 20-character access key ID.
func GenerateAccessKey() (string, error) {
	b := make([]byte, 10)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate access key: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// GenerateSecretKey creates a random 40-character secret key.
func GenerateSecretKey() (string, error) {
	b := make([]byte, 20)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate secret key: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// ServiceAccountStore defines the persistence interface for service accounts.
type ServiceAccountStore interface {
	// CreateServiceAccount persists a new service account.
	// The SecretKey must be hashed before storage; the raw key is only returned once.
	CreateServiceAccount(ctx context.Context, sa *ServiceAccount) error

	// GetServiceAccountByAccessKey looks up a service account by its access key ID.
	GetServiceAccountByAccessKey(ctx context.Context, accessKey string) (*ServiceAccount, error)

	// ListServiceAccountsByUser returns all service accounts for a given user.
	ListServiceAccountsByUser(ctx context.Context, parentUser string) ([]*ServiceAccount, error)

	// DeleteServiceAccount removes a service account by its ID.
	DeleteServiceAccount(ctx context.Context, id string) error

	// DisableServiceAccount toggles the disabled state of a service account.
	DisableServiceAccount(ctx context.Context, id string, disabled bool) error
}
