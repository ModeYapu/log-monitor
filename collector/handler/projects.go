package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/logmonitor/collector/storage"
)

// ProjectsHandler handles project-related requests
type ProjectsHandler struct {
	projectStore storage.ProjectStore
	db           *storage.DB // Keep for legacy methods
	userStorage  *storage.UserStorage
}

// NewProjectsHandler creates a new projects handler
func NewProjectsHandler(db *storage.DB, userStorage *storage.UserStorage) *ProjectsHandler {
	return &ProjectsHandler{
		projectStore: db,
		db:           db,
		userStorage:  userStorage,
	}
}

// NewProjectsHandlerWithStore creates a new projects handler with explicit store
func NewProjectsHandlerWithStore(projectStore storage.ProjectStore, db *storage.DB, userStorage *storage.UserStorage) *ProjectsHandler {
	return &ProjectsHandler{
		projectStore: projectStore,
		db:           db,
		userStorage:  userStorage,
	}
}

// CreateProject creates a new project (admin only)
func (h *ProjectsHandler) CreateProject(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Name        string `json:"name"`
		Slug        string `json:"slug"`
		Description string `json:"description"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate inputs
	if req.Name == "" {
		http.Error(w, "Project name is required", http.StatusBadRequest)
		return
	}

	if req.Slug == "" {
		// Generate slug from name
		req.Slug = strings.ToLower(strings.ReplaceAll(req.Name, " ", "-"))
	}

	// Check if slug already exists
	if _, err := h.db.GetProject(req.Slug); err == nil {
		http.Error(w, "Project with this slug already exists", http.StatusConflict)
		return
	}

	project, err := h.db.CreateProject(req.Name, req.Slug, req.Description)
	if err != nil {
		slog.Error("Failed to create project", "error", err)
		http.Error(w, "Failed to create project", http.StatusInternalServerError)
		return
	}

	// Get current user ID and add as owner
	userID := getUserIDFromContext(r)
	if userID != 0 {
		if err := h.db.AddProjectMember(project.ID, userID, "owner"); err != nil {
			slog.Warn("Failed to add user as project owner", "error", err)
		}
	}

	respondJSON(w, project)
}

// ListProjects lists all projects (admin) or user's projects (regular user)
func (h *ProjectsHandler) ListProjects(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID := getUserIDFromContext(r)
	userRole := getUserRoleFromContext(r)

	var projects []storage.Project
	var err error

	if userRole == "admin" {
		// Admin can see all projects
		projects, err = h.db.ListProjects(0)
	} else {
		// Regular users only see their projects
		projects, err = h.db.ListProjects(userID)
	}

	if err != nil {
		slog.Error("Failed to list projects", "error", err)
		http.Error(w, "Failed to list projects", http.StatusInternalServerError)
		return
	}

	// Add member counts and event counts for each project
	type ProjectWithStats struct {
		storage.Project
		MemberCount int `json:"member_count"`
		EventCount  int64 `json:"event_count"`
	}

	result := make([]ProjectWithStats, len(projects))
	for i, project := range projects {
		result[i].Project = project

		// Get member count
		members, err := h.db.GetProjectMembers(project.ID)
		if err == nil {
			result[i].MemberCount = len(members)
		}

		// Get event count (you may need to add this method to storage)
		// For now, set to 0
		result[i].EventCount = 0
	}

	respondJSON(w, result)
}

// GetProject retrieves a single project by ID
func (h *ProjectsHandler) GetProject(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract project ID from URL
	idStr := strings.TrimPrefix(r.URL.Path, "/api/admin/projects/")
	if idStr == "" {
		http.Error(w, "Project ID required", http.StatusBadRequest)
		return
	}

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid project ID", http.StatusBadRequest)
		return
	}

	project, err := h.db.GetProject(id)
	if err != nil {
		slog.Error("Failed to get project", "error", err)
		http.Error(w, "Project not found", http.StatusNotFound)
		return
	}

	// Check permission
	userID := getUserIDFromContext(r)
	userRole := getUserRoleFromContext(r)

	if userRole != "admin" {
		// Check if user is a member
		role, err := h.db.GetUserRole(project.ID, userID)
		if err != nil || role == "" {
			http.Error(w, "Access denied", http.StatusForbidden)
			return
		}
	}

	respondJSON(w, project)
}

// UpdateProject updates project details
func (h *ProjectsHandler) UpdateProject(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract project ID from URL
	idStr := strings.TrimPrefix(r.URL.Path, "/api/admin/projects/")
	if idStr == "" {
		http.Error(w, "Project ID required", http.StatusBadRequest)
		return
	}

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid project ID", http.StatusBadRequest)
		return
	}

	var req struct {
		Name          string `json:"name"`
		Description   string `json:"description"`
		RetentionDays int    `json:"retention_days"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Check permission
	userID := getUserIDFromContext(r)
	userRole := getUserRoleFromContext(r)

	if userRole != "admin" {
		// Check if user is owner or developer
		role, err := h.db.GetUserRole(id, userID)
		if err != nil || (role != "owner" && role != "developer") {
			http.Error(w, "Access denied", http.StatusForbidden)
			return
		}
	}

	updates := make(map[string]interface{})
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.Description != "" {
		updates["description"] = req.Description
	}
	if req.RetentionDays > 0 {
		updates["retention_days"] = req.RetentionDays
	}

	if len(updates) == 0 {
		http.Error(w, "No fields to update", http.StatusBadRequest)
		return
	}

	if err := h.db.UpdateProject(id, updates); err != nil {
		slog.Error("Failed to update project", "error", err)
		http.Error(w, "Failed to update project", http.StatusInternalServerError)
		return
	}

	// Return updated project
	project, err := h.db.GetProject(id)
	if err != nil {
		slog.Error("Failed to get updated project", "error", err)
		http.Error(w, "Failed to retrieve updated project", http.StatusInternalServerError)
		return
	}

	respondJSON(w, project)
}

// DeleteProject soft deletes a project
func (h *ProjectsHandler) DeleteProject(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract project ID from URL
	idStr := strings.TrimPrefix(r.URL.Path, "/api/admin/projects/")
	if idStr == "" {
		http.Error(w, "Project ID required", http.StatusBadRequest)
		return
	}

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid project ID", http.StatusBadRequest)
		return
	}

	// Check permission (admin only)
	userRole := getUserRoleFromContext(r)
	if userRole != "admin" {
		http.Error(w, "Access denied", http.StatusForbidden)
		return
	}

	if err := h.db.DeleteProject(id); err != nil {
		slog.Error("Failed to delete project", "error", err)
		http.Error(w, "Failed to delete project", http.StatusInternalServerError)
		return
	}

	respondJSON(w, map[string]string{"status": "deleted"})
}

// RegenerateApiKey regenerates the API key for a project
func (h *ProjectsHandler) RegenerateApiKey(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract project ID from URL
	idStr := strings.TrimPrefix(r.URL.Path, "/api/admin/projects/")
	idStr = strings.TrimSuffix(idStr, "/api-key")
	if idStr == "" {
		http.Error(w, "Project ID required", http.StatusBadRequest)
		return
	}

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid project ID", http.StatusBadRequest)
		return
	}

	// Check permission
	userID := getUserIDFromContext(r)
	userRole := getUserRoleFromContext(r)

	if userRole != "admin" {
		// Check if user is owner
		role, err := h.db.GetUserRole(id, userID)
		if err != nil || role != "owner" {
			http.Error(w, "Access denied", http.StatusForbidden)
			return
		}
	}

	newKey, err := h.db.RegenerateApiKey(id)
	if err != nil {
		slog.Error("Failed to regenerate API key", "error", err)
		http.Error(w, "Failed to regenerate API key", http.StatusInternalServerError)
		return
	}

	respondJSON(w, map[string]string{"api_key": newKey})
}

// ListMembers lists all members of a project
func (h *ProjectsHandler) ListMembers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract project ID from URL
	idStr := strings.TrimPrefix(r.URL.Path, "/api/admin/projects/")
	idStr = strings.TrimSuffix(idStr, "/members")
	if idStr == "" {
		http.Error(w, "Project ID required", http.StatusBadRequest)
		return
	}

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid project ID", http.StatusBadRequest)
		return
	}

	// Check permission
	userID := getUserIDFromContext(r)
	userRole := getUserRoleFromContext(r)

	if userRole != "admin" {
		// Check if user is a member
		role, err := h.db.GetUserRole(id, userID)
		if err != nil || role == "" {
			http.Error(w, "Access denied", http.StatusForbidden)
			return
		}
	}

	members, err := h.db.GetProjectMembers(id)
	if err != nil {
		slog.Error("Failed to list project members", "error", err)
		http.Error(w, "Failed to list project members", http.StatusInternalServerError)
		return
	}

	// Enrich member data with user information
	type MemberWithUser struct {
		storage.ProjectMember
		Username    string `json:"username"`
		DisplayName string `json:"display_name"`
	}

	result := make([]MemberWithUser, len(members))
	for i, member := range members {
		result[i].ProjectMember = member

		// Get user info
		user, err := h.userStorage.GetUserByID(member.UserID)
		if err == nil && user != nil {
			result[i].Username = user.Username
			result[i].DisplayName = user.DisplayName
		}
	}

	respondJSON(w, result)
}

// AddMember adds a member to a project
func (h *ProjectsHandler) AddMember(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract project ID from URL
	idStr := strings.TrimPrefix(r.URL.Path, "/api/admin/projects/")
	idStr = strings.TrimSuffix(idStr, "/members")
	if idStr == "" {
		http.Error(w, "Project ID required", http.StatusBadRequest)
		return
	}

	projectID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid project ID", http.StatusBadRequest)
		return
	}

	var req struct {
		UserID int64  `json:"user_id"`
		Role   string `json:"role"` // owner|developer|viewer
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate role
	if req.Role != "owner" && req.Role != "developer" && req.Role != "viewer" {
		http.Error(w, "Invalid role. Must be owner, developer, or viewer", http.StatusBadRequest)
		return
	}

	// Check permission
	userID := getUserIDFromContext(r)
	userRole := getUserRoleFromContext(r)

	if userRole != "admin" {
		// Only owners can add members
		role, err := h.db.GetUserRole(projectID, userID)
		if err != nil || role != "owner" {
			http.Error(w, "Access denied", http.StatusForbidden)
			return
		}
	}

	if err := h.db.AddProjectMember(projectID, req.UserID, req.Role); err != nil {
		slog.Error("Failed to add project member", "error", err)
		http.Error(w, "Failed to add project member", http.StatusInternalServerError)
		return
	}

	respondJSON(w, map[string]string{"status": "added"})
}

// UpdateMemberRole updates a member's role
func (h *ProjectsHandler) UpdateMemberRole(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract project ID and user ID from URL
	path := strings.TrimPrefix(r.URL.Path, "/api/admin/projects/")
	parts := strings.Split(path, "/")
	if len(parts) < 3 {
		http.Error(w, "Invalid URL format", http.StatusBadRequest)
		return
	}

	projectID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		http.Error(w, "Invalid project ID", http.StatusBadRequest)
		return
	}

	targetUserID, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	var req struct {
		Role string `json:"role"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate role
	if req.Role != "owner" && req.Role != "developer" && req.Role != "viewer" {
		http.Error(w, "Invalid role. Must be owner, developer, or viewer", http.StatusBadRequest)
		return
	}

	// Check permission
	userID := getUserIDFromContext(r)
	userRole := getUserRoleFromContext(r)

	if userRole != "admin" {
		// Only owners can update roles
		role, err := h.db.GetUserRole(projectID, userID)
		if err != nil || role != "owner" {
			http.Error(w, "Access denied", http.StatusForbidden)
			return
		}
	}

	if err := h.db.UpdateProjectMemberRole(projectID, targetUserID, req.Role); err != nil {
		slog.Error("Failed to update member role", "error", err)
		http.Error(w, "Failed to update member role", http.StatusInternalServerError)
		return
	}

	respondJSON(w, map[string]string{"status": "updated"})
}

// RemoveMember removes a member from a project
func (h *ProjectsHandler) RemoveMember(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract project ID and user ID from URL
	path := strings.TrimPrefix(r.URL.Path, "/api/admin/projects/")
	parts := strings.Split(path, "/")
	if len(parts) < 3 {
		http.Error(w, "Invalid URL format", http.StatusBadRequest)
		return
	}

	projectID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		http.Error(w, "Invalid project ID", http.StatusBadRequest)
		return
	}

	targetUserID, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	// Check permission
	userID := getUserIDFromContext(r)
	userRole := getUserRoleFromContext(r)

	if userRole != "admin" {
		// Only owners can remove members
		role, err := h.db.GetUserRole(projectID, userID)
		if err != nil || role != "owner" {
			http.Error(w, "Access denied", http.StatusForbidden)
			return
		}
	}

	if err := h.db.RemoveProjectMember(projectID, targetUserID); err != nil {
		slog.Error("Failed to remove project member", "error", err)
		http.Error(w, "Failed to remove project member", http.StatusInternalServerError)
		return
	}

	respondJSON(w, map[string]string{"status": "removed"})
}

// Helper functions

func getUserIDFromContext(r *http.Request) int64 {
	// This should be extracted from JWT context
	// For now, return 0 (no user)
	// TODO: Implement proper JWT context extraction
	if userID := r.Context().Value("user_id"); userID != nil {
		if id, ok := userID.(int64); ok {
			return id
		}
	}
	return 0
}

func getUserRoleFromContext(r *http.Request) string {
	// This should be extracted from JWT context
	// For now, return empty string (no role)
	// TODO: Implement proper JWT context extraction
	if role := r.Context().Value("user_role"); role != nil {
		if r, ok := role.(string); ok {
			return r
		}
	}
	return ""
}

func respondJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		slog.Error("Failed to encode JSON response", "error", err)
	}
}