package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/logmonitor/collector/storage"
)

// HealthHandler handles release health and session stats queries
type HealthHandler struct {
	db *storage.DB
}

// NewHealthHandler creates a new health handler
func NewHealthHandler(db *storage.DB) *HealthHandler {
	return &HealthHandler{
		db: db,
	}
}

// GetReleaseHealth retrieves crash-free rate by release
// GET /api/query/release-health?appId=&startTime=&endTime=
func (h *HealthHandler) GetReleaseHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	appID := r.URL.Query().Get("appId")
	if appID == "" {
		http.Error(w, "Missing appId parameter", http.StatusBadRequest)
		return
	}

	// Parse time range
	var startTime, endTime int64
	if startTimeStr := r.URL.Query().Get("startTime"); startTimeStr != "" {
		if ts, err := strconv.ParseInt(startTimeStr, 10, 64); err == nil {
			startTime = ts
		}
	}
	if endTimeStr := r.URL.Query().Get("endTime"); endTimeStr != "" {
		if ts, err := strconv.ParseInt(endTimeStr, 10, 64); err == nil {
			endTime = ts
		}
	}

	result, err := h.db.GetReleaseHealth(appID, startTime, endTime)
	if err != nil {
		slog.Error("Failed to get release health: %v", err)
		http.Error(w, "Failed to get release health", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(result)
}

// GetSessionStats retrieves overall session statistics
// GET /api/query/session-stats?appId=&startTime=&endTime=
func (h *HealthHandler) GetSessionStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	appID := r.URL.Query().Get("appId")
	if appID == "" {
		http.Error(w, "Missing appId parameter", http.StatusBadRequest)
		return
	}

	// Parse time range
	var startTime, endTime int64
	if startTimeStr := r.URL.Query().Get("startTime"); startTimeStr != "" {
		if ts, err := strconv.ParseInt(startTimeStr, 10, 64); err == nil {
			startTime = ts
		}
	}
	if endTimeStr := r.URL.Query().Get("endTime"); endTimeStr != "" {
		if ts, err := strconv.ParseInt(endTimeStr, 10, 64); err == nil {
			endTime = ts
		}
	}

	result, err := h.db.GetSessionStats(appID, startTime, endTime)
	if err != nil {
		slog.Error("Failed to get session stats: %v", err)
		http.Error(w, "Failed to get session stats", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(result)
}

// RegisterRoutes registers all health routes
func (h *HealthHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/query/release-health", h.GetReleaseHealth)
	mux.HandleFunc("GET /api/query/session-stats", h.GetSessionStats)
}
