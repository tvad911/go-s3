package middleware

import (
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"

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
