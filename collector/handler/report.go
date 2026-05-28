package handler

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/logmonitor/collector/buffer"
	"github.com/logmonitor/collector/config"
	"github.com/logmonitor/collector/model"
	"github.com/logmonitor/collector/storage"
)

// ReportHandler handles log report requests from SDK
type ReportHandler struct {
	writer *buffer.Writer
}

// NewReportHandler creates a new report handler
func NewReportHandler(writer *buffer.Writer, cfg *config.ServerConfig) *ReportHandler {
	return &ReportHandler{
		writer: writer,
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
		log.Printf("Failed to parse request: %v", err)
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
		})
	}

	// Write to buffer
	for _, r := range records {
		if err := h.writer.Write(r); err != nil {
			log.Printf("Failed to write event to buffer: %v", err)
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
