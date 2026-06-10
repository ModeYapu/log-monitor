package middleware

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/logmonitor/collector/storage"
)

// ProjectContextKey is the context key for project ID
type ProjectContextKey struct{}

// RequireProjectAccess is middleware that verifies project access and adds project_id to context
func RequireProjectAccess(db *storage.DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check if project_id query parameter is provided
			projectIDStr := r.URL.Query().Get("project_id")
			if projectIDStr == "" {
				// No project specified, continue without project context
				next.ServeHTTP(w, r)
				return
			}

			// Parse project ID
			projectID, err := strconv.ParseInt(projectIDStr, 10, 64)
			if err != nil {
				http.Error(w, "Invalid project_id parameter", http.StatusBadRequest)
				return
			}

			// Get user info from context (set by JWT middleware)
			userID := getUserIDFromContext(r)
			userRole := getUserRoleFromContext(r)

			// Admin users can access all projects
			if userRole == "admin" {
				// Verify project exists
				project, err := db.GetProject(projectID)
				if err != nil {
					http.Error(w, "Project not found", http.StatusNotFound)
					return
				}

				if project.DeletedAt > 0 {
					http.Error(w, "Project has been deleted", http.StatusNotFound)
					return
				}

				// Add project to context and continue
				ctx := context.WithValue(r.Context(), ProjectContextKey{}, projectID)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			// Regular users must be project members
			if userID == 0 {
				http.Error(w, "User not authenticated", http.StatusUnauthorized)
				return
			}

			// Check if user is a member of the project
			role, err := db.GetUserRole(projectID, userID)
			if err != nil || role == "" {
				http.Error(w, "Access denied to this project", http.StatusForbidden)
				return
			}

			// Add project to context along with user's role
			ctx := context.WithValue(r.Context(), ProjectContextKey{}, projectID)
			ctx = context.WithValue(ctx, struct{}{}, role) // User's role in project
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireProjectMember checks if user is a member of the specified project
func RequireProjectMember(db *storage.DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract project ID from URL (for specific resource access)
			projectID := extractProjectIDFromURL(r)
			if projectID == 0 {
				// Try to get from context (set by RequireProjectAccess)
				if id := r.Context().Value(ProjectContextKey{}); id != nil {
					if pid, ok := id.(int64); ok {
						projectID = pid
					} else {
						next.ServeHTTP(w, r)
						return
					}
				} else {
					next.ServeHTTP(w, r)
					return
				}
			}

			// Get user info from context
			userID := getUserIDFromContext(r)
			userRole := getUserRoleFromContext(r)

			// Admin users can access all projects
			if userRole == "admin" {
				next.ServeHTTP(w, r)
				return
			}

			// Regular users must be project members
			if userID == 0 {
				http.Error(w, "User not authenticated", http.StatusUnauthorized)
				return
			}

			// Check if user is a member of the project
			role, err := db.GetUserRole(projectID, userID)
			if err != nil || role == "" {
				http.Error(w, "Access denied to this project", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// FilterByProject middleware adds project filtering to query handlers
func FilterByProject() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// This middleware modifies the request to add project filtering
			// It should be used after RequireProjectAccess

			// If project_id is in context, ensure it's used in queries
			// The actual filtering should be implemented in the query handlers

			next.ServeHTTP(w, r)
		})
	}
}

// Helper functions

func getUserIDFromContext(r *http.Request) int64 {
	if userID := r.Context().Value("user_id"); userID != nil {
		switch v := userID.(type) {
		case int64:
			return v
		case float64:
			return int64(v)
		case string:
			if id, err := strconv.ParseInt(v, 10, 64); err == nil {
				return id
			}
		}
	}
	return 0
}

func getUserRoleFromContext(r *http.Request) string {
	if role := r.Context().Value("user_role"); role != nil {
		if r, ok := role.(string); ok {
			return r
		}
	}
	return ""
}

func extractProjectIDFromURL(r *http.Request) int64 {
	// Extract project ID from URL patterns like /api/admin/projects/:id/...
	path := r.URL.Path

	// Pattern: /api/admin/projects/{id}/
	if strings.Contains(path, "/api/admin/projects/") {
		parts := strings.Split(path, "/")
		for i, part := range parts {
			if part == "projects" && i+1 < len(parts) {
				if id, err := strconv.ParseInt(parts[i+1], 10, 64); err == nil {
					return id
				}
			}
		}
	}

	// Pattern: /api/query/projects/{id}/
	if strings.Contains(path, "/api/query/projects/") {
		parts := strings.Split(path, "/")
		for i, part := range parts {
			if part == "projects" && i+1 < len(parts) {
				if id, err := strconv.ParseInt(parts[i+1], 10, 64); err == nil {
					return id
				}
			}
		}
	}

	return 0
}

// GetProjectIDFromContext retrieves the project ID from the request context
func GetProjectIDFromContext(r *http.Request) int64 {
	if projectID := r.Context().Value(ProjectContextKey{}); projectID != nil {
		switch v := projectID.(type) {
		case int64:
			return v
		case float64:
			return int64(v)
		case string:
			if id, err := strconv.ParseInt(v, 10, 64); err == nil {
				return id
			}
		}
	}
	return 0
}

// GetProjectRoleFromContext retrieves the user's role in the current project context
func GetProjectRoleFromContext(r *http.Request) string {
	// This would be set by RequireProjectAccess
	if role := r.Context().Value(struct{}{}); role != nil {
		if r, ok := role.(string); ok {
			return r
		}
	}
	return ""
}