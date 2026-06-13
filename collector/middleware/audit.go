package middleware

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/logmonitor/collector/model"
)

// AuditStore defines the interface for audit log operations
type AuditStore interface {
	InsertAuditLog(log *model.AuditLog) error
}

// AuditConfig holds the audit middleware configuration
type AuditConfig struct {
	Store          AuditStore
	AsyncChannel   chan *model.AuditLog
	BufferSize     int
	IgnorePaths    []string // Paths to ignore (e.g., /api/health)
}

// AuditMiddleware provides audit logging for HTTP requests
type AuditMiddleware struct {
	config *AuditConfig
}

// NewAuditMiddleware creates a new audit middleware
func NewAuditMiddleware(config *AuditConfig) *AuditMiddleware {
	if config.BufferSize <= 0 {
		config.BufferSize = 1000
	}
	config.AsyncChannel = make(chan *model.AuditLog, config.BufferSize)

	// Start background worker to process audit logs
	go auditLogWorker(config.AsyncChannel, config.Store)

	return &AuditMiddleware{config: config}
}

// auditLogWorker processes audit logs asynchronously
func auditLogWorker(ch <-chan *model.AuditLog, store AuditStore) {
	for log := range ch {
		if err := store.InsertAuditLog(log); err != nil {
			slog.Error("Failed to insert audit log", "error", err, "action", log.Action, "resource", log.Resource)
		}
	}
}

// Handler returns an HTTP handler that logs write operations
func (a *AuditMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip read-only methods
		if isReadOnlyMethod(r.Method) {
			next.ServeHTTP(w, r)
			return
		}

		// Check if path should be ignored
		if a.shouldIgnorePath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		// Wrap the response writer to capture status code
		wrapped := &auditResponseWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}

		// Call the next handler
		next.ServeHTTP(wrapped, r)

		// Only log successful requests (2xx status codes)
		if wrapped.statusCode >= 200 && wrapped.statusCode < 300 {
			a.logAction(r, wrapped.statusCode)
		}
	})
}

// shouldIgnorePath checks if a path should be ignored
func (a *AuditMiddleware) shouldIgnorePath(path string) bool {
	for _, ignorePath := range a.config.IgnorePaths {
		if strings.HasPrefix(path, ignorePath) {
			return true
		}
	}
	return false
}

// logAction creates and queues an audit log entry
func (a *AuditMiddleware) logAction(r *http.Request, statusCode int) {
	// Extract user info from context
	userID := GetUserID(r)
	username := GetUsername(r)
	projectID := GetProjectIDFromContext(r)

	// Map HTTP method to action
	action := httpMethodToAction(r.Method)

	// Extract resource and resource_id from URL
	resource, resourceID := extractResourceFromPath(r.URL.Path, r.Method)

	// Build audit log
	log := &model.AuditLog{
		ProjectID:  projectID,
		UserID:     userID,
		Username:   username,
		Action:     action,
		Resource:   resource,
		ResourceID: resourceID,
		Detail:     buildDetail(r),
		IP:         getClientIP(r),
		UserAgent:  r.UserAgent(),
	}

	// Send to async channel (non-blocking)
	select {
	case a.config.AsyncChannel <- log:
		// Successfully queued
	default:
		// Channel full, log warning
		slog.Warn("Audit log channel full, dropping log entry", "action", action, "resource", resource)
	}
}

// auditResponseWriter wraps http.ResponseWriter to capture status code
type auditResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (w *auditResponseWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

// httpMethodToAction maps HTTP methods to audit action types
func httpMethodToAction(method string) string {
	switch method {
	case http.MethodPost:
		return model.AuditActionCreate
	case http.MethodPut, http.MethodPatch:
		return model.AuditActionUpdate
	case http.MethodDelete:
		return model.AuditActionDelete
	default:
		return "unknown"
	}
}

// extractResourceFromPath extracts resource type and ID from the URL path
func extractResourceFromPath(path, method string) (string, string) {
	// Common API patterns
	parts := strings.Split(strings.Trim(path, "/"), "/")

	if len(parts) == 0 {
		return "unknown", ""
	}

	// Determine resource based on URL structure
	var resource string
	var resourceID string

	// Check for /api/admin/xxx pattern
	if len(parts) >= 2 && parts[0] == "api" && parts[1] == "admin" {
		if len(parts) >= 3 {
			resource = normalizeResourceName(parts[2])
			// Extract ID from URL (e.g., /api/admin/projects/123)
			if len(parts) >= 4 && isIDPattern(parts[3]) {
				resourceID = parts[3]
			}
		}
	} else if len(parts) >= 2 && parts[0] == "api" && parts[1] == "query" {
		if len(parts) >= 3 {
			resource = normalizeResourceName(parts[2])
			if len(parts) >= 4 && isIDPattern(parts[3]) {
				resourceID = parts[3]
			}
		}
	} else if len(parts) >= 2 && parts[0] == "api" {
		// Generic /api/xxx pattern
		resource = normalizeResourceName(parts[1])
		if len(parts) >= 3 && isIDPattern(parts[2]) {
			resourceID = parts[2]
		}
	} else {
		resource = "unknown"
	}

	// Handle special cases
	if resource == "issues" && method == http.MethodPost {
		// POST /api/query/issues?action=resolve
		resource = model.AuditResourceIssue
	}

	return resource, resourceID
}

// normalizeResourceName converts URL resource names to audit resource constants
func normalizeResourceName(name string) string {
	switch name {
	case "projects", "project":
		return model.AuditResourceProject
	case "users", "user":
		return model.AuditResourceUser
	case "alerts", "alert":
		return model.AuditResourceAlert
	case "issues", "issue":
		return model.AuditResourceIssue
	case "sourcemaps", "sourcemap":
		return model.AuditResourceSourceMap
	case "webhooks", "webhook":
		return model.AuditResourceWebhook
	case "query", "logs":
		return model.AuditResourceEvent
	default:
		return name
	}
}

// isIDPattern checks if a string looks like an ID (numeric or UUID)
func isIDPattern(s string) bool {
	// Check for numeric ID
	if len(s) > 0 && len(s) < 20 {
		isNumeric := true
		for _, c := range s {
			if c < '0' || c > '9' {
				isNumeric = false
				break
			}
		}
		if isNumeric {
			return true
		}
	}
	// Check for common patterns (UUIDs, etc.)
	return strings.Contains(s, "-") && len(s) > 8
}

// buildDetail builds a detail string for the audit log
func buildDetail(r *http.Request) string {
	detail := r.Method + " " + r.URL.Path

	// Add query params if present (excluding sensitive data)
	if r.URL.RawQuery != "" && !containsSensitiveData(r.URL.RawQuery) {
		detail += "?" + r.URL.RawQuery
	}

	return detail
}

// containsSensitiveData checks if query string contains sensitive data
func containsSensitiveData(query string) bool {
	sensitiveKeys := []string{"password", "token", "secret", "key"}
	lowerQuery := strings.ToLower(query)
	for _, key := range sensitiveKeys {
		if strings.Contains(lowerQuery, key) {
			return true
		}
	}
	return false
}

// getClientIP extracts the client IP from the request
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header first
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// X-Forwarded-For can contain multiple IPs, take the first one
		if idx := strings.Index(xff, ","); idx != -1 {
			return strings.TrimSpace(xff[:idx])
		}
		return xff
	}

	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	// Fall back to RemoteAddr
	return r.RemoteAddr
}

// Close closes the audit middleware and stops the background worker
func (a *AuditMiddleware) Close() {
	close(a.config.AsyncChannel)
}
