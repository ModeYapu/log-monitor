package handler

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/logmonitor/collector/model"
	"github.com/logmonitor/collector/middleware"
	"github.com/logmonitor/collector/storage"
	"golang.org/x/crypto/bcrypt"
)

// AuthHandler handles authentication requests
type AuthHandler struct {
	userStorage *storage.UserStorage
	jwt         *middleware.JWT
}

// NewAuthHandler creates a new auth handler
func NewAuthHandler(userStorage *storage.UserStorage, jwt *middleware.JWT) *AuthHandler {
	return &AuthHandler{
		userStorage: userStorage,
		jwt:         jwt,
	}
}

// RegisterRoutes registers auth routes
func (h *AuthHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/auth/login", h.Login)
	mux.HandleFunc("GET /api/auth/me", h.Me)
	mux.HandleFunc("PUT /api/auth/password", h.ChangePassword)
	mux.HandleFunc("GET /api/users", h.ListUsers)
	mux.HandleFunc("POST /api/users", h.CreateUser)
	mux.HandleFunc("PUT /api/users/", h.UpdateUser)
	mux.HandleFunc("DELETE /api/users/", h.DeleteUser)
}

// Login handles user login
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req model.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "Invalid request",
		})
		return
	}

	// Get user from database
	user, passwordHash, err := h.userStorage.GetUserByUsername(req.Username)
	if err != nil {
		log.Printf("Login failed: %v", err)
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "用户名或密码错误",
		})
		return
	}

	if user == nil {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "用户名或密码错误",
		})
		return
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password)); err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "用户名或密码错误",
		})
		return
	}

	// Check if user is enabled
	if !user.Enabled {
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "用户已被禁用",
		})
		return
	}

	// Generate JWT token
	token, err := h.jwt.GenerateToken(user)
	if err != nil {
		log.Printf("Failed to generate token: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "登录失败",
		})
		return
	}

	// Update last login
	_ = h.userStorage.UpdateLastLogin(user.ID)

	json.NewEncoder(w).Encode(model.LoginResponse{
		Token: token,
		User: &model.UserInfo{
			ID:          user.ID,
			Username:    user.Username,
			DisplayName: user.DisplayName,
			Role:        user.Role,
		},
	})
}

// Me returns the current user info
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	userID := middleware.GetUserID(r)
	if userID == 0 {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "Unauthorized",
		})
		return
	}

	user, err := h.userStorage.GetUserByID(userID)
	if err != nil {
		log.Printf("Failed to get user: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "Failed to get user info",
		})
		return
	}

	if user == nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "User not found",
		})
		return
	}

	json.NewEncoder(w).Encode(model.UserInfo{
		ID:          user.ID,
		Username:    user.Username,
		DisplayName: user.DisplayName,
		Role:        user.Role,
	})
}

// ChangePassword handles password change
func (h *AuthHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	userID := middleware.GetUserID(r)
	if userID == 0 {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "Unauthorized",
		})
		return
	}

	var req model.ChangePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "Invalid request",
		})
		return
	}

	// Get user with password hash
	user, passwordHash, err := h.userStorage.GetUserByUsername(middleware.GetUsername(r))
	if err != nil || user == nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "Failed to get user",
		})
		return
	}

	// Verify old password
	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.OldPassword)); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "原密码错误",
		})
		return
	}

	// Hash new password
	newPasswordHash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("Failed to hash password: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "密码修改失败",
		})
		return
	}

	// Update password
	if err := h.userStorage.UpdatePassword(userID, string(newPasswordHash)); err != nil {
		log.Printf("Failed to update password: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "密码修改失败",
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "密码修改成功",
	})
}

// ListUsers returns all users (admin only)
func (h *AuthHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	users, err := h.userStorage.ListUsers()
	if err != nil {
		log.Printf("Failed to list users: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "Failed to list users",
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"data": users,
	})
}

// CreateUser creates a new user (admin only)
func (h *AuthHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req model.CreateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "Invalid request",
		})
		return
	}

	// Validate request
	if req.Username == "" || req.Password == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "用户名和密码不能为空",
		})
		return
	}

	if req.Role != "admin" && req.Role != "user" {
		req.Role = "user"
	}

	if req.DisplayName == "" {
		req.DisplayName = req.Username
	}

	// Check if user already exists
	_, _, err := h.userStorage.GetUserByUsername(req.Username)
	if err == nil {
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "用户名已存在",
		})
		return
	}

	// Hash password
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("Failed to hash password: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "创建用户失败",
		})
		return
	}

	// Create user
	userID, err := h.userStorage.CreateUser(req.Username, string(passwordHash), req.DisplayName, req.Role)
	if err != nil {
		log.Printf("Failed to create user: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "创建用户失败",
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "用户创建成功",
		"user": map[string]interface{}{
			"id":       userID,
			"username": req.Username,
			"role":     req.Role,
		},
	})
}

// UpdateUser updates a user (admin only)
func (h *AuthHandler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Extract user ID from path
	// Path format: /api/users/{id}
	// We need to parse the ID
	// The path is already trimmed by http.ServeMux pattern matching
	// So r.URL.Path will be like /{id}
	idStr := r.URL.Path[len("/api/users/"):]
	var id int64
	if _, err := fmt.Sscanf(idStr, "%d", &id); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "Invalid user ID",
		})
		return
	}

	var req model.UpdateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "Invalid request",
		})
		return
	}

	// Validate role
	if req.Role != "admin" && req.Role != "user" {
		req.Role = "user"
	}

	// Get current user
	currentUserID := middleware.GetUserID(r)
	currentRole := middleware.GetRole(r)

	// Check if trying to disable/delete self
	if id == currentUserID && !req.Enabled {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "不能禁用自己的账号",
		})
		return
	}

	// Check if trying to change own role
	if id == currentUserID && req.Role != currentRole {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "不能修改自己的角色",
		})
		return
	}

	// Check if target user is admin and current user is also admin
	// (Prevent admins from disabling other admins)
	targetUser, err := h.userStorage.GetUserByID(id)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "Failed to get user",
		})
		return
	}

	if targetUser == nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "用户不存在",
		})
		return
	}

	if targetUser.Role == "admin" && id != currentUserID {
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "不能修改其他管理员",
		})
		return
	}

	// Update user
	if err := h.userStorage.UpdateUser(id, req.DisplayName, req.Role, req.Enabled); err != nil {
		log.Printf("Failed to update user: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "更新用户失败",
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "用户更新成功",
	})
}

// DeleteUser deletes a user (admin only)
func (h *AuthHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Extract user ID from path
	idStr := r.URL.Path[len("/api/users/"):]
	var id int64
	if _, err := fmt.Sscanf(idStr, "%d", &id); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "Invalid user ID",
		})
		return
	}

	currentUserID := middleware.GetUserID(r)

	// Check if trying to delete self
	if id == currentUserID {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "不能删除自己的账号",
		})
		return
	}

	// Get target user
	targetUser, err := h.userStorage.GetUserByID(id)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "Failed to get user",
		})
		return
	}

	if targetUser == nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "用户不存在",
		})
		return
	}

	// Prevent deleting admin users
	if targetUser.Role == "admin" {
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "不能删除管理员",
		})
		return
	}

	// Delete user
	if err := h.userStorage.DeleteUser(id); err != nil {
		log.Printf("Failed to delete user: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "删除用户失败",
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "用户删除成功",
	})
}

// ResetPassword resets a user's password (admin only)
func (h *AuthHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Extract user ID from path
	idStr := r.URL.Path[len("/api/users/"):]
	var id int64
	if _, err := fmt.Sscanf(idStr, "%d", &id); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "Invalid user ID",
		})
		return
	}

	var req model.ResetPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "Invalid request",
		})
		return
	}

	if req.NewPassword == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "新密码不能为空",
		})
		return
	}

	// Hash new password
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("Failed to hash password: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "密码重置失败",
		})
		return
	}

	// Update password
	if err := h.userStorage.UpdatePassword(id, string(passwordHash)); err != nil {
		log.Printf("Failed to update password: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "密码重置失败",
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "密码重置成功",
	})
}
