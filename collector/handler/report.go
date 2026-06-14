package handler

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/logmonitor/collector/buffer"
	"github.com/logmonitor/collector/middleware"
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

	// Resolve the project from the authenticated context. The API key
	// middleware (X-API-Key header) together with ProjectContext populates the
	// context; legacy SDKs that send the key in the request body are resolved
	// here as a fallback. The project_id is ALWAYS derived from a validated
	// API key — a client-supplied projectId is only honored when it matches.
	projectID := middleware.GetProjectIDFromContext(r)
	if projectID == 0 && req.APIKey != "" {
		project, err := h.projectStore.GetProjectByAPIKey(req.APIKey)
		if err != nil {
			slog.Warn("Invalid API key provided", "error", err)
			writeReportError(w, http.StatusUnauthorized, "invalid or missing API key")
			return
		}
		projectID = project.ID
	}
	if projectID == 0 {
		writeReportError(w, http.StatusUnauthorized, "authentication required: provide a valid X-API-Key header or apiKey field")
		return
	}
	// Enforce multi-tenant isolation: a client-supplied projectId must match
	// the project the resolved API key is authorized for.
	if req.ProjectID != 0 && req.ProjectID != projectID {
		slog.Warn("projectId does not match API key project",
			"requested", req.ProjectID, "authorized", projectID)
		writeReportError(w, http.StatusForbidden, "projectId does not match the API key's project")
		return
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

// writeReportError writes a JSON error response for ingestion requests.
// The Content-Type header is expected to already be set to application/json.
func writeReportError(w http.ResponseWriter, status int, message string) {
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{"error": message})
}

// truncateString truncates a string to at most maxLen runes. Truncating by rune
// (rather than byte) avoids splitting multi-byte UTF-8 sequences, which would
// otherwise yield invalid UTF-8 in the stored message.
func truncateString(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= maxLen {
		return s
	}
	return string(r[:maxLen])
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
