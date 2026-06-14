package handler

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/logmonitor/collector/buffer"
	"github.com/logmonitor/collector/model"
	"github.com/logmonitor/collector/storage"
)

// ReportHandler handles log report requests from SDK.
// Depends on ProjectStore (interface) rather than the concrete *storage.DB.
type ReportHandler struct {
	writer       *buffer.Writer
	projectStore storage.ProjectStore
}

// NewReportHandler creates a new report handler
func NewReportHandler(writer *buffer.Writer, projectStore storage.ProjectStore) *ReportHandler {
	return &ReportHandler{
		writer:       writer,
		projectStore: projectStore,
	}
}

// ServeHTTP handles HTTP requests
func (h *ReportHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	// Limit request body size to prevent DoS attacks
	const maxRequestSize = 10 * 1024 * 1024 // 10MB
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestSize)

	// Read request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if len(body) == 0 {
		http.Error(w, "Empty request body", http.StatusBadRequest)
		return
	}

	// Parse request
	var req model.ReportRequest
	if err := json.Unmarshal(body, &req); err != nil {
		slog.Error("Failed to parse request", "error", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Validate request
	if req.AppID == "" {
		http.Error(w, "Missing appId", http.StatusBadRequest)
		return
	}

	if len(req.Events) == 0 {
		http.Error(w, "No events in request", http.StatusBadRequest)
		return
	}

	// Get project ID from request (via project_id or api_key)
	var projectID int64
	if req.ProjectID != 0 {
		projectID = req.ProjectID
	} else if req.APIKey != "" {
		// Look up project by API key
		project, err := h.projectStore.GetProjectByAPIKey(req.APIKey)
		if err != nil {
			slog.Warn("Invalid API key provided", "error", err)
			// Don't reject the request, just continue without project association
			projectID = 0
		} else {
			projectID = project.ID
		}
	}

	// Get client IP
	ip := r.RemoteAddr
	if forwardedFor := r.Header.Get("X-Forwarded-For"); forwardedFor != "" {
		ip = strings.Split(forwardedFor, ",")[0]
	}

	// Convert to storage records and buffer
	now := time.Now().UnixMilli()
	records := make([]storage.EventRecord, 0, len(req.Events))

	for _, e := range req.Events {
		// Fill in missing fields
		appID := e.AppID
		if appID == "" {
			appID = req.AppID
		}
		release := e.Release
		if release == "" {
			release = req.Release
		}
		createdAt := e.CreatedAt
		if createdAt == 0 {
			createdAt = now
		}
		eventIP := e.IP
		if eventIP == "" {
			eventIP = ip
		}

		// Convert to buffer record
		records = append(records, storage.EventRecord{
			AppID:       appID,
			Release:     release,
			Type:        e.Type,
			Level:       e.Level,
			Message:     truncateString(e.Message, 10000),
			Stack:       truncateString(e.Stack, 50000),
			URL:         truncateString(e.URL, 2000),
			Line:        e.Line,
			Col:         e.Col,
			Tags:        toJSON(e.Tags),
			Extra:       toJSON(e.Extra),
			UA:          truncateString(e.UA, 1000),
			Screen:      e.Screen,
			Viewport:    e.Viewport,
			Performance: toJSON(e.Performance),
			IP:          eventIP,
			CreatedAt:   createdAt,
			ProjectID:   projectID,
		})
	}

	// Write to buffer
	for _, r := range records {
		if err := h.writer.Write(r); err != nil {
			slog.Error("Failed to write event to buffer", "error", err)
		}
	}
	// Respond with success
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"count":   len(records),
	})
}

// truncateString truncates a string to max length
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}

// classifyEvent extracts additional context based on event type
// This enables specialized handling for different event categories
func classifyEvent(e model.Event) (string, map[string]interface{}) {
	// Returns: (subType, extractedContext)
	subType := ""
	context := make(map[string]interface{})

	switch e.Type {
	case model.EventTypeResource:
		// Extract resource URL, type, failure reason
		subType = "resource_error"
		if url, ok := e.Extra["url"].(string); ok {
			context["resource_url"] = url
		}
		if resType, ok := e.Tags["resource_type"].(string); ok {
			context["resource_type"] = resType
		}
		if reason, ok := e.Extra["reason"].(string); ok {
			context["failure_reason"] = reason
		}

	case model.EventTypeAPIError:
		// Extract API URL, status code, duration
		subType = "api_failure"
		if url, ok := e.Extra["url"].(string); ok {
			context["api_url"] = url
		}
		if status, ok := e.Extra["status_code"].(float64); ok {
			context["status_code"] = int(status)
		}
		if duration, ok := e.Extra["duration"].(float64); ok {
			context["duration_ms"] = duration
		}
		if method, ok := e.Extra["method"].(string); ok {
			context["method"] = method
		}

	case model.EventTypeUserAction:
		// Extract action name, target element
		subType = "user_behavior"
		if action, ok := e.Extra["action"].(string); ok {
			context["action_name"] = action
		}
		if target, ok := e.Extra["target"].(string); ok {
			context["target_selector"] = target
		}
		if page, ok := e.Extra["page"].(string); ok {
			context["page_url"] = page
		}
	}

	return subType, context
}

// toJSON converts a map to JSON string
func toJSON(m map[string]interface{}) string {
	if m == nil {
		return "{}"
	}
	data, err := json.Marshal(m)
	if err != nil {
		return "{}"
	}
	return string(data)
}
