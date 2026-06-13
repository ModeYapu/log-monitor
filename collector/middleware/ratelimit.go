package middleware

import (
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"
)

// RateLimiter implements IP-based rate limiting using the token bucket algorithm.
type RateLimiter struct {
	mu              sync.RWMutex
	clients         map[string]*clientInfo
	maxRequests     int
	window          time.Duration
	cleanupInterval time.Duration
	stopCleanup     chan struct{}
}

type clientInfo struct {
	requests  []time.Time
	lastReset time.Time
}

// NewRateLimiter creates a new rate limiter.
// maxRequests: maximum number of requests allowed per window
// window: time window for rate limiting
func NewRateLimiter(maxRequests int, window time.Duration) *RateLimiter {
	rl := &RateLimiter{
		clients:         make(map[string]*clientInfo),
		maxRequests:     maxRequests,
		window:          window,
		cleanupInterval: time.Minute,
		stopCleanup:     make(chan struct{}),
	}

	// Start cleanup goroutine to remove stale entries
	go rl.cleanup()

	return rl
}

// cleanup removes stale client entries periodically.
func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(rl.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			rl.removeStaleEntries()
		case <-rl.stopCleanup:
			return
		}
	}
}

// removeStaleEntries removes client entries that haven't been used recently.
func (rl *RateLimiter) removeStaleEntries() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	staleThreshold := now.Add(-rl.cleanupInterval * 2)

	for ip, info := range rl.clients {
		if info.lastReset.Before(staleThreshold) {
			delete(rl.clients, ip)
		}
	}
}

// Stop stops the cleanup goroutine.
func (rl *RateLimiter) Stop() {
	close(rl.stopCleanup)
}

// Handler wraps an http.Handler with rate limiting.
// Returns 429 Too Many Requests when the rate limit is exceeded.
func (rl *RateLimiter) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := rl.getClientIP(r)

		if !rl.allowRequest(ip) {
			slog.Warn("Rate limit exceeded", "ip", ip)
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-RateLimit-Limited", "true")
			w.Header().Set("Retry-After", rl.window.String())
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"error":"too_many_requests","message":"请求过于频繁，请稍后再试"}`))
			return
		}

		next.ServeHTTP(w, r)
	})
}

// allowRequest checks if a request from the given IP is allowed.
func (rl *RateLimiter) allowRequest(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	client, exists := rl.clients[ip]

	if !exists {
		rl.clients[ip] = &clientInfo{
			requests:  []time.Time{now},
			lastReset: now,
		}
		return true
	}

	// Remove requests outside the current window
	windowStart := now.Add(-rl.window)
	validRequests := make([]time.Time, 0, len(client.requests))
	for _, reqTime := range client.requests {
		if reqTime.After(windowStart) {
			validRequests = append(validRequests, reqTime)
		}
	}

	// Check if limit exceeded
	if len(validRequests) >= rl.maxRequests {
		client.requests = validRequests
		client.lastReset = now
		return false
	}

	// Add current request
	client.requests = append(validRequests, now)
	client.lastReset = now
	return true
}

// getClientIP extracts the client IP from the request.
// It checks X-Real-IP and X-Forwarded-For headers before falling back to RemoteAddr.
func (rl *RateLimiter) getClientIP(r *http.Request) string {
	// Check X-Real-IP header
	if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
		if ip := net.ParseIP(realIP); ip != nil {
			return realIP
		}
	}

	// Check X-Forwarded-For header
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// X-Forwarded-For can contain multiple IPs, use the first one
		for i, part := range xff {
			if part == ' ' || part == ',' {
				return xff[:i]
			}
		}
		if ip := net.ParseIP(xff); ip != nil {
			return xff
		}
	}

	// Fall back to RemoteAddr
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
