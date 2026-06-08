package middleware

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/logmonitor/collector/model"
)

// Context keys for request context
type contextKey string

const (
	UserIDKey   contextKey = "user_id"
	UsernameKey contextKey = "username"
	RoleKey     contextKey = "role"
)

// JWT provides JWT authentication middleware
type JWT struct {
	secret          []byte
	tokenExpireHours int
}

// NewJWT creates a new JWT middleware
func NewJWT(secret string, tokenExpireHours int) *JWT {
	if secret == "" {
		// Generate a random secret if not provided
		b := make([]byte, 32)
		if _, err := rand.Read(b); err != nil {
					slog.Error("Failed to generate JWT secret", "error", err)
			os.Exit(1)
		}
		secret = base64.StdEncoding.EncodeToString(b)
				slog.Info("Generated JWT secret (set auth.jwt_secret in config to persist)")
	}
	return &JWT{
		secret:          []byte(secret),
		tokenExpireHours: tokenExpireHours,
	}
}

// GetSecret returns the JWT secret
func (j *JWT) GetSecret() string {
	return string(j.secret)
}

// GenerateToken generates a JWT token for a user
func (j *JWT) GenerateToken(user *model.User) (string, error) {
	claims := jwt.MapClaims{
		"user_id":  user.ID,
		"username": user.Username,
		"role":     user.Role,
		"exp":      time.Now().Add(time.Duration(j.tokenExpireHours) * time.Hour).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(j.secret)
}

// ValidateToken validates a JWT token and returns the claims
func (j *JWT) ValidateToken(tokenString string) (*model.TokenClaims, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return j.secret, nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		return &model.TokenClaims{
			UserID:   int64(claims["user_id"].(float64)),
			Username: claims["username"].(string),
			Role:     claims["role"].(string),
		}, nil
	}

	return nil, jwt.ErrInvalidKey
}

// extractJWTToken extracts the JWT token from the Authorization header or query param
func extractJWTToken(r *http.Request) string {
	// Try Authorization header first
	authHeader := r.Header.Get("Authorization")
	if authHeader != "" {
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) == 2 && parts[0] == "Bearer" {
			return parts[1]
		}
	}

	// Try query parameter (for WebSocket connections)
	return r.URL.Query().Get("token")
}

// Handler returns an authentication middleware handler
func (j *JWT) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenString := extractJWTToken(r)
		if tokenString == "" {
			j.unauthorized(w)
			return
		}

		claims, err := j.ValidateToken(tokenString)
		if err != nil {
					slog.Warn("Token validation failed", "error", err)
			j.unauthorized(w)
			return
		}

		// Add user info to context
		ctx := context.WithValue(r.Context(), UserIDKey, claims.UserID)
		ctx = context.WithValue(ctx, UsernameKey, claims.Username)
		ctx = context.WithValue(ctx, RoleKey, claims.Role)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireAdmin is a middleware that requires admin role
func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		role := r.Context().Value(RoleKey)
		if role == nil || role.(string) != "admin" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": "Admin role required",
			})
			return
		}
		next.ServeHTTP(w, r)
	})
}

// unauthorized returns a 401 unauthorized response
func (j *JWT) unauthorized(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": "Unauthorized",
	})
}

// GetUserID returns the user ID from the request context
func GetUserID(r *http.Request) int64 {
	if id, ok := r.Context().Value(UserIDKey).(int64); ok {
		return id
	}
	return 0
}

// GetUsername returns the username from the request context
func GetUsername(r *http.Request) string {
	if username, ok := r.Context().Value(UsernameKey).(string); ok {
		return username
	}
	return ""
}

// GetRole returns the user role from the request context
func GetRole(r *http.Request) string {
	if role, ok := r.Context().Value(RoleKey).(string); ok {
		return role
	}
	return ""
}
