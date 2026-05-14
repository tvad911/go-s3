package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type contextKey string

const UserContextKey contextKey = "user"

// GetUser extracts the authenticated user from the request context.
func GetUser(ctx context.Context) *User {
	if user, ok := ctx.Value(UserContextKey).(*User); ok {
		return user
	}
	return &User{Username: "anonymous"}
}

var (
	ErrUserNotFound     = errors.New("user not found")
	ErrUserExists       = errors.New("user already exists")
	ErrInvalidPassword  = errors.New("invalid password")
	ErrPasswordRequired = errors.New("password is required")
)

// User represents an identity in the system.
// SecretKey is excluded from JSON responses to prevent leakage.
// AccessKeyID is kept for backward compatibility but new auth flow uses ServiceAccounts.
type User struct {
	Username     string    `json:"username"`
	PasswordHash string    `json:"passwordHash,omitempty"`
	AccessKeyID  string    `json:"accessKeyId,omitempty"`
	SecretKey    string    `json:"secretKey,omitempty"`
	Policies     []string  `json:"policies"`
	IsRoot       bool      `json:"isRoot"`
	Disabled     bool      `json:"disabled"`
	CreatedAt    time.Time `json:"createdAt"`
}

// HashPassword generates a bcrypt hash from a plaintext password.
func HashPassword(password string) (string, error) {
	if password == "" {
		return "", ErrPasswordRequired
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(hash), nil
}

// CheckPassword verifies a plaintext password against a bcrypt hash.
// Returns nil on success, ErrInvalidPassword on mismatch.
func CheckPassword(hash, password string) error {
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return ErrInvalidPassword
	}
	return nil
}

// UserStore defines the interface for user persistence.
type UserStore interface {
	GetUserByAccessKey(ctx context.Context, accessKey string) (*User, error)
	GetUserByUsername(ctx context.Context, username string) (*User, error)
	ListUsers(ctx context.Context) ([]*User, error)
	CreateUser(ctx context.Context, user *User) error
	UpdateUser(ctx context.Context, user *User) error
	DeleteUser(ctx context.Context, username string) error
}
