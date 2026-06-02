package middleware

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"

	"github.com/logmonitor/collector/model"
)

type JWTTokenValidator interface {
	ValidateToken(tokenString string) (*model.TokenClaims, error)
}

// AuthConfig holds authentication configuration
type AuthConfig struct {
	AdminTokens  map[string]bool  // Valid admin tokens
	UserTokens   map[string]bool  // Valid user tokens (optional)
	Enabled      bool
	JWTValidator JWTTokenValidator
	mu           sync.RWMutex // Protects token maps from concurrent access
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

// SetJWTValidator enables JWT-backed authentication for admin APIs.
func (c *AuthConfig) SetJWTValidator(validator JWTTokenValidator) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.JWTValidator = validator
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
	valid := c.AdminTokens[token]
	validator := c.JWTValidator
	c.mu.RUnlock()

	if valid {
		return true
	}

	if validator != nil {
		claims, err := validator.ValidateToken(token)
		return err == nil && claims.Role == "admin"
	}

	return false
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
	adminValid := c.AdminTokens[token]
	userValid := c.UserTokens[token]
	userTokenCount := len(c.UserTokens)
	validator := c.JWTValidator
	c.mu.RUnlock()

	if isAdmin {
		if adminValid {
			return true
		}
		if validator != nil {
			claims, err := validator.ValidateToken(token)
			return err == nil && claims.Role == "admin"
		}
		return false
	}

	// User connections are more permissive
	if userTokenCount == 0 {
		return true
	}

	return userValid || adminValid
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
