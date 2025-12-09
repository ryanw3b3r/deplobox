package server

import (
	"log/slog"
	"net/http"
	"sync"

	"golang.org/x/time/rate"
)

// RateLimiter implements a simple token bucket rate limiter per IP address
type RateLimiter struct {
	limiters   map[string]*rate.Limiter
	mu         sync.RWMutex
	rateLimit  rate.Limit // Requests per second
	burstSize  int        // Maximum burst size
}

// NewRateLimiter creates a new rate limiter
// rateLimit: requests per second
// burstSize: maximum number of requests allowed in a burst
func NewRateLimiter(rateLimit rate.Limit, burstSize int) *RateLimiter {
	return &RateLimiter{
		limiters:  make(map[string]*rate.Limiter),
		rateLimit: rateLimit,
		burstSize: burstSize,
	}
}

// GetLimiter returns the rate limiter for a given IP address
// Creates a new limiter for the IP if one doesn't exist
func (rl *RateLimiter) GetLimiter(ip string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	limiter, exists := rl.limiters[ip]
	if !exists {
		limiter = rate.NewLimiter(rl.rateLimit, rl.burstSize)
		rl.limiters[ip] = limiter
	}

	return limiter
}

// NewRateLimitMiddleware creates middleware for global rate limiting
// hourLimit: requests per hour
func NewRateLimitMiddleware(hourLimit int, logger *slog.Logger) func(http.Handler) http.Handler {
	// Use the more restrictive limit (hour limit is usually tighter)
	// Convert to requests per second
	rps := rate.Limit(float64(hourLimit) / 3600.0)
	limiter := NewRateLimiter(rps, hourLimit)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := r.RemoteAddr

			if !limiter.GetLimiter(ip).Allow() {
				logger.Warn("Rate limit exceeded", "ip", ip, "path", r.URL.Path)
				http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// NewWebhookRateLimitMiddleware creates middleware for webhook-specific rate limiting
// limit: requests per minute
func NewWebhookRateLimitMiddleware(limit int, logger *slog.Logger) func(http.Handler) http.Handler {
	// Convert to requests per second
	rps := rate.Limit(float64(limit) / 60.0)
	limiter := NewRateLimiter(rps, limit)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := r.RemoteAddr

			if !limiter.GetLimiter(ip).Allow() {
				logger.Warn("Webhook rate limit exceeded", "ip", ip, "path", r.URL.Path)
				http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
