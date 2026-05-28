package middleware

import (
	"net/http"
)

// CORS provides CORS middleware
type CORS struct {
	allowedOrigins []string
}

// NewCORS creates a new CORS middleware
func NewCORS(allowedOrigins []string) *CORS {
	return &CORS{
		allowedOrigins: allowedOrigins,
	}
}

// Handler returns an http.Handler that adds CORS headers
func (c *CORS) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")

		// Set CORS headers
		if len(c.allowedOrigins) == 0 {
			// No restrictions - allow all origins
			w.Header().Set("Access-Control-Allow-Origin", "*")
		} else {
			// Check if origin is allowed
			allowed := false
			for _, allowedOrigin := range c.allowedOrigins {
				if allowedOrigin == "*" || allowedOrigin == origin {
					allowed = true
					w.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
					break
				}
			}
			if !allowed && origin != "" {
				// Origin not in whitelist
				next.ServeHTTP(w, r)
				return
			}
		}

		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Expose-Headers", "Content-Type")
		w.Header().Set("Access-Control-Max-Age", "86400")

		// Handle preflight request
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}
