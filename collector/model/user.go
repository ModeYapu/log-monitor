package model

import (
	"time"
)

// User represents a user in the system
type User struct {
	ID          int64     `json:"id"`
	Username    string    `json:"username"`
	DisplayName string    `json:"display_name"`
	Role        string    `json:"role"` // admin | user
	Enabled     bool      `json:"enabled"`
	LastLoginAt int64     `json:"last_login_at"`
	CreatedAt   int64     `json:"created_at"`
	UpdatedAt   int64     `json:"updated_at"`
}

// TokenClaims represents JWT token claims
type TokenClaims struct {
	UserID   int64  `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
}

// LoginRequest represents a login request
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// LoginResponse represents a login response
type LoginResponse struct {
	Token string       `json:"token"`
	User  *UserInfo    `json:"user"`
}

// UserInfo represents user info returned to client (without sensitive data)
type UserInfo struct {
	ID          int64  `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	Role        string `json:"role"`
}

// CreateUserRequest represents a create user request (admin only)
type CreateUserRequest struct {
	Username    string `json:"username"`
	Password    string `json:"password"`
	DisplayName string `json:"display_name"`
	Role        string `json:"role"`
}

// UpdateUserRequest represents an update user request (admin only)
type UpdateUserRequest struct {
	DisplayName string `json:"display_name"`
	Role        string `json:"role"`
	Enabled     bool   `json:"enabled"`
}

// ChangePasswordRequest represents a change password request
type ChangePasswordRequest struct {
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}

// ResetPasswordRequest represents a reset password request (admin only)
type ResetPasswordRequest struct {
	NewPassword string `json:"new_password"`
}

// Timestamps returns created_at and updated_at as current time
func Timestamps() (int64, int64) {
	now := time.Now().UnixMilli()
	return now, now
}
