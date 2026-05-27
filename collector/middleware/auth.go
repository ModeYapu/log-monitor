package middleware

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"
)

// AuthConfig holds authentication configuration
type AuthConfig struct {
	AdminTokens  map[string]bool // Valid admin tokens
	UserTokens   map[string]bool // Valid user tokens (optional)
	Enabled      bool
	mu           sync.RWMutex    // Protects token maps from concurrent access
}

// NewAuthConfig creates a new auth config
func NewAuthConfig() *AuthConfig {
	return &AuthConfig{
		AdminTokens: make(map[string]bool),
		UserTokens:  make(map[string]bool),
		Enabled:     true,
	}
}

// AddAdminToken adds an admin token
func (c *AuthConfig) AddAdminToken(token string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.AdminTokens[token] = true
}

// AddUserToken adds a user token
func (c *AuthConfig) AddUserToken(token string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.UserTokens[token] = true
}

// AuthenticateAdmin checks if the request has a valid admin token
func (c *AuthConfig) AuthenticateAdmin(r *http.Request) bool {
	if !c.Enabled {
		return true
	}

	token := extractToken(r)
	if token == "" {
		return false
	}

	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.AdminTokens[token]
}

// AuthenticateUser checks if the request has a valid user token
func (c *AuthConfig) AuthenticateUser(r *http.Request) bool {
	if !c.Enabled {
		return true
	}

	token := extractToken(r)
	if token == "" {
		return false
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	// User tokens are optional - if not configured, allow all
	if len(c.UserTokens) == 0 {
		return true
	}

	return c.UserTokens[token]
}

// AuthenticateWebSocket checks WebSocket connection authentication
func (c *AuthConfig) AuthenticateWebSocket(r *http.Request, isAdmin bool) bool {
	if !c.Enabled {
		return true
	}

	// Check query param for token
	token := r.URL.Query().Get("token")
	if token == "" {
		// Check Authorization header
		token = extractToken(r)
	}

	if token == "" {
		return false
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	if isAdmin {
		return c.AdminTokens[token]
	}

	// User connections are more permissive
	if len(c.UserTokens) == 0 {
		return true
	}

	return c.UserTokens[token] || c.AdminTokens[token]
}

// extractToken extracts token from Authorization header
func extractToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return ""
	}

	// Support "Bearer <token>" format
	parts := strings.Split(auth, " ")
	if len(parts) == 2 && strings.ToLower(parts[0]) == "bearer" {
		return parts[1]
	}

	return auth
}

// WriteAuthError writes an authentication error response
func WriteAuthError(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": "unauthorized",
		"message": message,
	})
}
