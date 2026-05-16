package handler

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"

	"gos3/internal/auth"
	"gos3/internal/storage/metadata"
)

// AuthHandler handles authentication endpoints for the Web Console.
type AuthHandler struct {
	store      metadata.Store
	sessionCfg *auth.SessionConfig
}

// NewAuthHandler creates a new AuthHandler.
func NewAuthHandler(store metadata.Store, sessionCfg *auth.SessionConfig) *AuthHandler {
	return &AuthHandler{
		store:      store,
		sessionCfg: sessionCfg,
	}
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type loginResponse struct {
	Username string `json:"username"`
	IsRoot   bool   `json:"isRoot"`
	Token    string `json:"token,omitempty"`
}

type meResponse struct {
	Username string   `json:"username"`
	IsRoot   bool     `json:"isRoot"`
	Policies []string `json:"policies"`
}

// Login authenticates a user by username/password and sets a session cookie.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 2<<20)).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	if req.Username == "" || req.Password == "" {
		http.Error(w, `{"error":"username and password are required"}`, http.StatusBadRequest)
		return
	}

	user, err := h.store.GetUserByUsername(r.Context(), req.Username)
	if err != nil {
		// Fallback: check if the username provided is actually an Access Key
		sa, saErr := h.store.GetServiceAccountByAccessKey(r.Context(), req.Username)
		if saErr == nil && !sa.Disabled && !sa.IsExpired() {
			if subtle.ConstantTimeCompare([]byte(sa.SecretKey), []byte(req.Password)) == 1 {
				user, err = h.store.GetUserByUsername(r.Context(), sa.ParentUser)
				if err == nil {
					goto TokenGeneration
				}
			}
		}
		slog.Warn("login failed: user/access_key not found or invalid", "username", req.Username)
		http.Error(w, `{"error":"invalid username or password"}`, http.StatusUnauthorized)
		return
	}

	if user.Disabled {
		http.Error(w, `{"error":"account is disabled"}`, http.StatusForbidden)
		return
	}

	// Verify password
	if err := auth.CheckPassword(user.PasswordHash, req.Password); err != nil {
		slog.Warn("login failed: invalid password", "username", req.Username)
		http.Error(w, `{"error":"invalid username or password"}`, http.StatusUnauthorized)
		return
	}

TokenGeneration:
	sessionID := uuid.New().String()
	session := &auth.Session{
		ID:        sessionID,
		Username:  user.Username,
		IsRoot:    user.IsRoot,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(h.sessionCfg.TokenExpiry),
		IPAddress: r.RemoteAddr,
		UserAgent: r.UserAgent(),
	}

	if err := h.store.CreateSession(r.Context(), session); err != nil {
		slog.Error("failed to create session", "error", err)
		http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
		return
	}

	// Generate JWT token
	token, err := auth.GenerateToken(h.sessionCfg, sessionID, user.Username, user.IsRoot)
	if err != nil {
		slog.Error("failed to generate token", "error", err)
		http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
		return
	}

	// Set HttpOnly cookie
	http.SetCookie(w, &http.Cookie{
		Name:     h.sessionCfg.CookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   h.sessionCfg.CookieSecure,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   int(h.sessionCfg.TokenExpiry.Seconds()),
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(loginResponse{
		Username: user.Username,
		IsRoot:   user.IsRoot,
	})
}

// Logout clears the session cookie and removes the session from the DB.
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(h.sessionCfg.CookieName)
	if err == nil && cookie.Value != "" {
		if claims, err := auth.ValidateToken(h.sessionCfg, cookie.Value); err == nil {
			_ = h.store.DeleteSession(r.Context(), claims.SessionID)
		}
	}
	http.SetCookie(w, &http.Cookie{
		Name:     h.sessionCfg.CookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   h.sessionCfg.CookieSecure,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
	})

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"ok"}`))
}

// Me returns the current authenticated user's info.
// Requires JWT cookie middleware to have set user in context.
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r.Context())
	if user.Username == "anonymous" {
		http.Error(w, `{"error":"not authenticated"}`, http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(meResponse{
		Username: user.Username,
		IsRoot:   user.IsRoot,
		Policies: user.Policies,
	})
}

// JWTAuthMiddleware creates a middleware that validates JWT cookie and injects user into context.
// This is for Web Console API routes. S3 API routes continue using SigV4.
func JWTAuthMiddleware(sessionCfg *auth.SessionConfig, store metadata.Store) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(sessionCfg.CookieName)
			if err != nil {
				http.Error(w, `{"error":"not authenticated"}`, http.StatusUnauthorized)
				return
			}

			claims, err := auth.ValidateToken(sessionCfg, cookie.Value)
			if err != nil {
				http.Error(w, `{"error":"invalid or expired token"}`, http.StatusUnauthorized)
				return
			}

			// Validate session in DB
			if _, err := store.GetSession(r.Context(), claims.SessionID); err != nil {
				http.Error(w, `{"error":"session expired or revoked"}`, http.StatusUnauthorized)
				return
			}

			// Lookup user from DB to get latest state (policies, disabled status)
			user, err := store.GetUserByUsername(r.Context(), claims.Username)
			if err != nil {
				http.Error(w, `{"error":"user not found"}`, http.StatusUnauthorized)
				return
			}

			if user.Disabled {
				http.Error(w, `{"error":"account is disabled"}`, http.StatusForbidden)
				return
			}

			// Inject user into context
			ctx := r.Context()
			ctx = SetUserContext(ctx, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// SetUserContext sets the user in context. Reusable by both JWT and SigV4 middleware.
func SetUserContext(ctx context.Context, user *auth.User) context.Context {
	return context.WithValue(ctx, auth.UserContextKey, user)
}

// JWTAuthFallbackMiddleware checks if the request has a valid JWT cookie.
// If it does, it overwrites the user in the context. This allows routes protected
// by SigV4 to fallback to JWT if the user is using the Web Console.
func JWTAuthFallbackMiddleware(sessionCfg *auth.SessionConfig, store metadata.Store) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if cookie, err := r.Cookie(sessionCfg.CookieName); err == nil && cookie.Value != "" {
				if claims, err := auth.ValidateToken(sessionCfg, cookie.Value); err == nil {
					if _, err := store.GetSession(r.Context(), claims.SessionID); err == nil {
						if user, err := store.GetUserByUsername(r.Context(), claims.Username); err == nil && !user.Disabled {
							ctx := SetUserContext(r.Context(), user)
							next.ServeHTTP(w, r.WithContext(ctx))
							return
						}
					}
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}
