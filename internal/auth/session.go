package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrTokenExpired = errors.New("token expired")
	ErrTokenInvalid = errors.New("invalid token")
)

// SessionConfig holds JWT session configuration.
type SessionConfig struct {
	SigningKey     []byte
	TokenExpiry   time.Duration
	CookieName    string
	CookieSecure  bool
}

// DefaultSessionConfig returns a config with sensible defaults.
// signingKey should be at least 32 bytes; if empty, a random key is generated.
func DefaultSessionConfig(signingKey []byte) *SessionConfig {
	if len(signingKey) == 0 {
		signingKey = make([]byte, 32)
		rand.Read(signingKey)
	}
	return &SessionConfig{
		SigningKey:   signingKey,
		TokenExpiry:  24 * time.Hour,
		CookieName:   "gos3_token",
		CookieSecure: false,
	}
}

// Claims represents the JWT payload.
type Claims struct {
	Username string `json:"sub"`
	IsRoot   bool   `json:"root"`
	IssuedAt int64  `json:"iat"`
	ExpiresAt int64 `json:"exp"`
}

// jwtHeader is a fixed header for HS256.
type jwtHeader struct {
	Alg string `json:"alg"`
	Typ string `json:"typ"`
}

var fixedHeader = jwtHeader{Alg: "HS256", Typ: "JWT"}

// GenerateToken creates a signed JWT token string.
func GenerateToken(cfg *SessionConfig, username string, isRoot bool) (string, error) {
	now := time.Now()
	claims := Claims{
		Username:  username,
		IsRoot:    isRoot,
		IssuedAt:  now.Unix(),
		ExpiresAt: now.Add(cfg.TokenExpiry).Unix(),
	}

	headerJSON, err := json.Marshal(fixedHeader)
	if err != nil {
		return "", fmt.Errorf("marshal header: %w", err)
	}
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("marshal claims: %w", err)
	}

	headerB64 := base64.RawURLEncoding.EncodeToString(headerJSON)
	claimsB64 := base64.RawURLEncoding.EncodeToString(claimsJSON)

	signingInput := headerB64 + "." + claimsB64
	sig := signHMAC(cfg.SigningKey, []byte(signingInput))
	sigB64 := base64.RawURLEncoding.EncodeToString(sig)

	return signingInput + "." + sigB64, nil
}

// ValidateToken parses and validates a JWT token string.
func ValidateToken(cfg *SessionConfig, tokenStr string) (*Claims, error) {
	parts := strings.SplitN(tokenStr, ".", 3)
	if len(parts) != 3 {
		return nil, ErrTokenInvalid
	}

	// Verify signature
	signingInput := parts[0] + "." + parts[1]
	expectedSig := signHMAC(cfg.SigningKey, []byte(signingInput))
	actualSig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, ErrTokenInvalid
	}
	if !hmac.Equal(expectedSig, actualSig) {
		return nil, ErrTokenInvalid
	}

	// Decode claims
	claimsJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, ErrTokenInvalid
	}
	var claims Claims
	if err := json.Unmarshal(claimsJSON, &claims); err != nil {
		return nil, ErrTokenInvalid
	}

	// Check expiration
	if time.Now().Unix() > claims.ExpiresAt {
		return nil, ErrTokenExpired
	}

	return &claims, nil
}

// signHMAC computes HMAC-SHA256.
func signHMAC(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}
