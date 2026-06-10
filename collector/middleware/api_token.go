package middleware

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"log/slog"
	"net/http"
	"strings"
)

// Context keys for request context
type contextKey string

const (
	ProjectIDKey  contextKey = "project_id"
	APIKeyIDKey   contextKey = "api_key_id"
)

// APIKeyConfig holds API key authentication configuration
type APIKeyConfig struct {
	// Store is the interface to validate API keys
	Store APIKeyStore
	// Optional: JWT middleware for fallback
	JWT *JWT
}

// APIKeyStore defines the interface for API key validation
type APIKeyStore interface {
	ValidateAPIKey(apiKey string) (*APIKeyInfo, error)
}

// APIKeyInfo contains information about a validated API key
type APIKeyInfo struct {
	ProjectID int64
	Name      string
	ReadOnly  bool // If true, only allows read operations
}

// APIKey provides API key authentication middleware
type APIKey struct {
	config *APIKeyConfig
}

// NewAPIKey creates a new API key middleware
func NewAPIKey(config *APIKeyConfig) *APIKey {
	return &APIKey{
		config: config,
	}
}

// Handler wraps an http.Handler with API key authentication
func (a *APIKey) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Try API key authentication first
		apiKey := r.Header.Get("X-API-Key")
		if apiKey != "" {
			// Validate API key
			keyInfo, err := a.config.Store.ValidateAPIKey(apiKey)
			if err != nil {
				slog.Warn("Invalid API key", "error", err)
				http.Error(w, "Invalid API key", http.StatusUnauthorized)
				return
			}

			// Add project info to context
			ctx := context.WithValue(r.Context(), ProjectIDKey, keyInfo.ProjectID)
			ctx = context.WithValue(ctx, APIKeyIDKey, apiKey)

			// Check if request is read-only
			if keyInfo.ReadOnly && !isReadOnlyMethod(r.Method) {
				slog.Warn("API key is read-only", "method", r.Method)
				http.Error(w, "API key is read-only", http.StatusForbidden)
				return
			}

			// Continue with modified context
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		// Fall back to JWT authentication if configured
		if a.config.JWT != nil {
			a.config.JWT.Handler(next).ServeHTTP(w, r)
			return
		}

		// No authentication method worked
		http.Error(w, "Authentication required", http.StatusUnauthorized)
	})
}

// isReadOnlyMethod checks if the HTTP method is read-only
func isReadOnlyMethod(method string) bool {
	return method == http.MethodGet || method == http.MethodHead
}

// GetProjectIDFromContext retrieves the project ID from the request context
func GetProjectIDFromContext(r *http.Request) (int64, bool) {
	projectID, ok := r.Context().Value(ProjectIDKey).(int64)
	return projectID, ok
}

// GetAPIKeyFromContext retrieves the API key from the request context
func GetAPIKeyFromContext(r *http.Request) (string, bool) {
	apiKey, ok := r.Context().Value(APIKeyIDKey).(string)
	return apiKey, ok
}

// GenerateAPIKey generates a random API key
func GenerateAPIKey() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	// Format as lm_ followed by base64 encoded random bytes
	return "lm_" + base64.URLEncoding.EncodeToString(b), nil
}

// ValidateAPIKeyFormat validates the format of an API key
func ValidateAPIKeyFormat(apiKey string) bool {
	if apiKey == "" {
		return false
	}
	// API keys should start with "lm_" and be at least 36 characters long
	if !strings.HasPrefix(apiKey, "lm_") {
		return false
	}
	if len(apiKey) < 36 {
		return false
	}
	// Check that the part after "lm_" is valid base64
	part := apiKey[3:]
	_, err := base64.URLEncoding.DecodeString(part)
	return err == nil
}