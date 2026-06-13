package middleware

import (
	"encoding/json"
	"net/http"
)

// Role permissions mapping
// - owner: full access (GET, POST, PUT, DELETE, PATCH)
// - developer: read and write access (GET, POST, PUT)
// - viewer: read-only access (GET, HEAD, OPTIONS)

var rolePermissions = map[string]map[string]bool{
	"owner": {
		http.MethodGet:     true,
		http.MethodHead:    true,
		http.MethodOptions: true,
		http.MethodPost:    true,
		http.MethodPut:     true,
		http.MethodPatch:   true,
		http.MethodDelete:  true,
	},
	"developer": {
		http.MethodGet:     true,
		http.MethodHead:    true,
		http.MethodOptions: true,
		http.MethodPost:    true,
		http.MethodPut:     true,
		http.MethodPatch:   false,
		http.MethodDelete:  false,
	},
	"viewer": {
		http.MethodGet:     true,
		http.MethodHead:    true,
		http.MethodOptions: true,
		http.MethodPost:    false,
		http.MethodPut:     false,
		http.MethodPatch:   false,
		http.MethodDelete:  false,
	},
}

// RequireProjectAuthorization checks if the user has the required role-based permissions
// for the current HTTP method on a project resource.
// Admin users bypass project-level authorization.
func RequireProjectAuthorization(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if user is admin - admins bypass project authorization
		if GetRole(r) == "admin" {
			next.ServeHTTP(w, r)
			return
		}

		// Get the user's role in the current project context
		projectRole := GetProjectRoleFromContext(r)

		// If no project role is set, the request might not be project-scoped
		// In this case, allow the request to proceed (other middleware will handle auth)
		if projectRole == "" {
			next.ServeHTTP(w, r)
			return
		}

		// Check if the role is valid
		permissions, roleExists := rolePermissions[projectRole]
		if !roleExists {
			writeAuthError(w, "Invalid project role")
			return
		}

		// Normalize the method (uppercase)
		method := r.Method

		// Check if the method is allowed for this role
		allowed, methodExists := permissions[method]
		if !methodExists || !allowed {
			writeAuthError(w, "Insufficient permissions for this operation")
			return
		}

		next.ServeHTTP(w, r)
	})
}

// RequireRole is a more specific middleware that requires a minimum role level
// Use this when you want to enforce specific role requirements
func RequireRole(minRole string) func(http.Handler) http.Handler {
	roleHierarchy := map[string]int{
		"viewer":    1,
		"developer": 2,
		"owner":     3,
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check if user is admin - admins bypass project authorization
			if GetRole(r) == "admin" {
				next.ServeHTTP(w, r)
				return
			}

			projectRole := GetProjectRoleFromContext(r)

			// If no project role, deny
			if projectRole == "" {
				writeAuthError(w, "Project role required")
				return
			}

			userLevel, userHasRole := roleHierarchy[projectRole]
			minLevel, minHasRole := roleHierarchy[minRole]

			if !userHasRole || !minHasRole || userLevel < minLevel {
				writeAuthError(w, "Insufficient role permissions")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// writeAuthError writes an authorization error response
func writeAuthError(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error":   "forbidden",
		"message": message,
	})
}
