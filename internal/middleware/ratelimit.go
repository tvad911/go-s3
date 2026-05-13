package middleware

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"

	"gos3/internal/auth"

	"gos3/internal/config"
	"gos3/internal/handler"
	"gos3/internal/s3"
)

// client holds the rate limiter for a specific IP.
type client struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// RateLimit is a middleware that applies rate limiting globally per IP.
func RateLimit(cfg *config.RateLimitConfig) func(http.Handler) http.Handler {
	if !cfg.Enabled {
		return func(next http.Handler) http.Handler {
			return next
		}
	}

	var (
		mu      sync.Mutex
		clients = make(map[string]*client)
	)

	// Background cleanup of inactive clients
	go func() {
		for {
			time.Sleep(time.Minute)
			mu.Lock()
			for ip, c := range clients {
				if time.Since(c.lastSeen) > 3*time.Minute {
					delete(clients, ip)
				}
			}
			mu.Unlock()
		}
	}()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := r.RemoteAddr

			mu.Lock()
			if _, found := clients[ip]; !found {
				clients[ip] = &client{
					limiter: rate.NewLimiter(rate.Limit(cfg.RequestsPerSecond), cfg.Burst),
				}
			}
			clients[ip].lastSeen = time.Now()

			if !clients[ip].limiter.Allow() {
				mu.Unlock()
				handler.WriteError(w, r, s3.ErrSlowDown)
				return
			}
			mu.Unlock()

			next.ServeHTTP(w, r)
		})
	}
}

type advancedClient struct {
	readLimiter  *rate.Limiter
	writeLimiter *rate.Limiter
	lastSeen     time.Time
}

// AdvancedRateLimit applies per-user and per-bucket rate limiting based on the config.
func AdvancedRateLimit(cfg *config.RateLimitConfig) func(http.Handler) http.Handler {
	if !cfg.PerUserEnabled && !cfg.PerBucketEnabled {
		return func(next http.Handler) http.Handler {
			return next
		}
	}

	var (
		mu            sync.Mutex
		userClients   = make(map[string]*advancedClient)
		bucketClients = make(map[string]*advancedClient)
	)

	// Background cleanup of inactive clients
	go func() {
		for {
			time.Sleep(time.Minute)
			mu.Lock()
			for k, c := range userClients {
				if time.Since(c.lastSeen) > 3*time.Minute {
					delete(userClients, k)
				}
			}
			for k, c := range bucketClients {
				if time.Since(c.lastSeen) > 3*time.Minute {
					delete(bucketClients, k)
				}
			}
			mu.Unlock()
		}
	}()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			writeOp := r.Method == "PUT" || r.Method == "POST" || r.Method == "DELETE"

			// Per-user rate limiting
			if cfg.PerUserEnabled {
				user := auth.GetUser(r.Context())
				id := user.AccessKeyID
				if id == "" {
					id = user.Username
				}

				if id != "" && id != "anonymous" {
					mu.Lock()
					uc, found := userClients[id]
					if !found {
						uc = &advancedClient{
							readLimiter:  rate.NewLimiter(rate.Limit(cfg.PerUserReadRPS), cfg.PerUserBurst),
							writeLimiter: rate.NewLimiter(rate.Limit(cfg.PerUserWriteRPS), cfg.PerUserBurst),
						}
						userClients[id] = uc
					}
					uc.lastSeen = time.Now()

					var allowed bool
					if writeOp {
						allowed = uc.writeLimiter.Allow()
					} else {
						allowed = uc.readLimiter.Allow()
					}
					mu.Unlock()

					if !allowed {
						handler.WriteError(w, r, s3.ErrSlowDown)
						return
					}
				}
			}

			// Per-bucket rate limiting
			if cfg.PerBucketEnabled {
				pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
				bucket := ""
				if len(pathParts) > 0 && pathParts[0] != "" {
					bucket = pathParts[0]
				}

				if bucket != "" && !strings.HasPrefix(bucket, "_") {
					mu.Lock()
					bc, found := bucketClients[bucket]
					if !found {
						bc = &advancedClient{
							readLimiter:  rate.NewLimiter(rate.Limit(cfg.PerBucketReadRPS), cfg.PerBucketBurst),
							writeLimiter: rate.NewLimiter(rate.Limit(cfg.PerBucketWriteRPS), cfg.PerBucketBurst),
						}
						bucketClients[bucket] = bc
					}
					bc.lastSeen = time.Now()

					var allowed bool
					if writeOp {
						allowed = bc.writeLimiter.Allow()
					} else {
						allowed = bc.readLimiter.Allow()
					}
					mu.Unlock()

					if !allowed {
						handler.WriteError(w, r, s3.ErrSlowDown)
						return
					}
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}
