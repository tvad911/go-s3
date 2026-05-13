package auth

import (
	"context"
	"errors"
	"time"
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
	ErrUserNotFound = errors.New("user not found")
	ErrUserExists   = errors.New("user already exists")
)

type User struct {
	Username    string    `json:"username"`
	AccessKeyID string    `json:"accessKeyId"`
	SecretKey   string    `json:"secretKey"`
	Policies    []string  `json:"policies"`
	IsRoot      bool      `json:"isRoot"`
	Disabled    bool      `json:"disabled"`
	CreatedAt   time.Time `json:"createdAt"`
}

type UserStore interface {
	GetUserByAccessKey(ctx context.Context, accessKey string) (*User, error)
	GetUserByUsername(ctx context.Context, username string) (*User, error)
	ListUsers(ctx context.Context) ([]*User, error)
	CreateUser(ctx context.Context, user *User) error
	UpdateUser(ctx context.Context, user *User) error
	DeleteUser(ctx context.Context, username string) error
}
