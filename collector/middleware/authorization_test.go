package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestAuthorizationMiddlewareViewer tests viewer role permissions
func TestAuthorizationMiddlewareViewer(t *testing.T) {
	// Create a test handler that indicates it was called
	called := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	authMiddleware := RequireProjectAuthorization(handler)

	tests := []struct {
		name           string
		method         string
		shouldAllow    bool
		projectRole    string
		globalRole     string
	}{
		{name: "viewer GET allowed", method: http.MethodGet, shouldAllow: true, projectRole: "viewer"},
		{name: "viewer HEAD allowed", method: http.MethodHead, shouldAllow: true, projectRole: "viewer"},
		{name: "viewer OPTIONS allowed", method: http.MethodOptions, shouldAllow: true, projectRole: "viewer"},
		{name: "viewer POST denied", method: http.MethodPost, shouldAllow: false, projectRole: "viewer"},
		{name: "viewer PUT denied", method: http.MethodPut, shouldAllow: false, projectRole: "viewer"},
		{name: "viewer PATCH denied", method: http.MethodPatch, shouldAllow: false, projectRole: "viewer"},
		{name: "viewer DELETE denied", method: http.MethodDelete, shouldAllow: false, projectRole: "viewer"},
		{name: "admin bypass GET", method: http.MethodGet, shouldAllow: true, projectRole: "viewer", globalRole: "admin"},
		{name: "admin bypass DELETE", method: http.MethodDelete, shouldAllow: true, projectRole: "viewer", globalRole: "admin"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			called = false
			req := httptest.NewRequest(tt.method, "/api/test", nil)

			// Set up context with project role
			ctx := req.Context()
			ctx = context.WithValue(ctx, ProjectRoleContextKey{}, tt.projectRole)
			if tt.globalRole != "" {
				ctx = context.WithValue(ctx, RoleKey, tt.globalRole)
			}
			req = req.WithContext(ctx)

			// Create response recorder
			rr := httptest.NewRecorder()

			// Call middleware
			authMiddleware.ServeHTTP(rr, req)

			// Check result
			if tt.shouldAllow {
				if !called {
					t.Errorf("Expected handler to be called for %s", tt.method)
				}
				if rr.Code != http.StatusOK {
					t.Errorf("Expected status OK for allowed request, got %d", rr.Code)
				}
			} else {
				if called {
					t.Errorf("Expected handler NOT to be called for %s", tt.method)
				}
				if rr.Code != http.StatusForbidden {
					t.Errorf("Expected status Forbidden for denied request, got %d", rr.Code)
				}
			}
		})
	}
}

// TestAuthorizationMiddlewareDeveloper tests developer role permissions
func TestAuthorizationMiddlewareDeveloper(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	authMiddleware := RequireProjectAuthorization(handler)

	tests := []struct {
		name        string
		method      string
		shouldAllow bool
	}{
		{name: "developer GET allowed", method: http.MethodGet, shouldAllow: true},
		{name: "developer HEAD allowed", method: http.MethodHead, shouldAllow: true},
		{name: "developer OPTIONS allowed", method: http.MethodOptions, shouldAllow: true},
		{name: "developer POST allowed", method: http.MethodPost, shouldAllow: true},
		{name: "developer PUT allowed", method: http.MethodPut, shouldAllow: true},
		{name: "developer PATCH denied", method: http.MethodPatch, shouldAllow: false},
		{name: "developer DELETE denied", method: http.MethodDelete, shouldAllow: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/api/test", nil)

			// Set up context with developer role
			ctx := req.Context()
			ctx = context.WithValue(ctx, ProjectRoleContextKey{}, "developer")
			req = req.WithContext(ctx)

			rr := httptest.NewRecorder()
			authMiddleware.ServeHTTP(rr, req)

			if tt.shouldAllow && rr.Code != http.StatusOK {
				t.Errorf("Expected status OK for %s, got %d", tt.method, rr.Code)
			}
			if !tt.shouldAllow && rr.Code != http.StatusForbidden {
				t.Errorf("Expected status Forbidden for %s, got %d", tt.method, rr.Code)
			}
		})
	}
}

// TestAuthorizationMiddlewareOwner tests owner role permissions
func TestAuthorizationMiddlewareOwner(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	authMiddleware := RequireProjectAuthorization(handler)

	methods := []string{
		http.MethodGet,
		http.MethodHead,
		http.MethodOptions,
		http.MethodPost,
		http.MethodPut,
		http.MethodPatch,
		http.MethodDelete,
	}

	for _, method := range methods {
		t.Run("owner "+method+" allowed", func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/test", nil)

			// Set up context with owner role
			ctx := req.Context()
			ctx = context.WithValue(ctx, ProjectRoleContextKey{}, "owner")
			req = req.WithContext(ctx)

			rr := httptest.NewRecorder()
			authMiddleware.ServeHTTP(rr, req)

			if rr.Code != http.StatusOK {
				t.Errorf("Expected status OK for owner %s, got %d", method, rr.Code)
			}
		})
	}
}

// TestAuthorizationMiddlewareAdminBypass tests that admin role bypasses project restrictions
func TestAuthorizationMiddlewareAdminBypass(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	authMiddleware := RequireProjectAuthorization(handler)

	methods := []string{
		http.MethodGet,
		http.MethodPost,
		http.MethodPut,
		http.MethodDelete,
	}

	for _, method := range methods {
		t.Run("admin bypass "+method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/test", nil)

			// Set up context with admin global role (no project role needed)
			ctx := req.Context()
			ctx = context.WithValue(ctx, RoleKey, "admin")
			req = req.WithContext(ctx)

			rr := httptest.NewRecorder()
			authMiddleware.ServeHTTP(rr, req)

			if rr.Code != http.StatusOK {
				t.Errorf("Expected status OK for admin %s, got %d", method, rr.Code)
			}
		})
	}
}

// TestAuthorizationMiddlewareNoProjectRole tests behavior when no project role is set
func TestAuthorizationMiddlewareNoProjectRole(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	authMiddleware := RequireProjectAuthorization(handler)

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	// No project role set in context

	rr := httptest.NewRecorder()
	authMiddleware.ServeHTTP(rr, req)

	// Should allow when no project role is set (non-project endpoint)
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status OK when no project role set, got %d", rr.Code)
	}
}

// TestAuthorizationMiddlewareInvalidRole tests behavior with invalid role
func TestAuthorizationMiddlewareInvalidRole(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	authMiddleware := RequireProjectAuthorization(handler)

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)

	// Set up context with invalid role
	ctx := req.Context()
	ctx = context.WithValue(ctx, ProjectRoleContextKey{}, "invalid_role")
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	authMiddleware.ServeHTTP(rr, req)

	// Should deny with invalid role
	if rr.Code != http.StatusForbidden {
		t.Errorf("Expected status Forbidden for invalid role, got %d", rr.Code)
	}
}

// TestRequireRoleMiddleware tests the RequireRole middleware
func TestRequireRoleMiddleware(t *testing.T) {
	tests := []struct {
		name        string
		minRole     string
		userRole    string
		globalRole  string
		shouldAllow bool
	}{
		{name: "owner passes owner requirement", minRole: "owner", userRole: "owner", shouldAllow: true},
		{name: "owner passes developer requirement", minRole: "developer", userRole: "owner", shouldAllow: true},
		{name: "owner passes viewer requirement", minRole: "viewer", userRole: "owner", shouldAllow: true},
		{name: "developer passes developer requirement", minRole: "developer", userRole: "developer", shouldAllow: true},
		{name: "developer passes viewer requirement", minRole: "viewer", userRole: "developer", shouldAllow: true},
		{name: "developer fails owner requirement", minRole: "owner", userRole: "developer", shouldAllow: false},
		{name: "viewer passes viewer requirement", minRole: "viewer", userRole: "viewer", shouldAllow: true},
		{name: "viewer fails developer requirement", minRole: "developer", userRole: "viewer", shouldAllow: false},
		{name: "viewer fails owner requirement", minRole: "owner", userRole: "viewer", shouldAllow: false},
		{name: "admin bypasses any requirement", minRole: "owner", userRole: "viewer", globalRole: "admin", shouldAllow: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			middleware := RequireRole(tt.minRole)(handler)

			req := httptest.NewRequest(http.MethodGet, "/api/test", nil)

			ctx := req.Context()
			ctx = context.WithValue(ctx, ProjectRoleContextKey{}, tt.userRole)
			if tt.globalRole != "" {
				ctx = context.WithValue(ctx, RoleKey, tt.globalRole)
			}
			req = req.WithContext(ctx)

			rr := httptest.NewRecorder()
			middleware.ServeHTTP(rr, req)

			if tt.shouldAllow && rr.Code != http.StatusOK {
				t.Errorf("Expected status OK, got %d", rr.Code)
			}
			if !tt.shouldAllow && rr.Code != http.StatusForbidden {
				t.Errorf("Expected status Forbidden, got %d", rr.Code)
			}
		})
	}
}
