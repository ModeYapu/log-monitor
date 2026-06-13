package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"time"

	"github.com/logmonitor/collector/storage"
)

// HealthHandler handles release health, session stats queries, and system health checks
type HealthHandler struct {
	db        *storage.DB
	startTime time.Time
}

// NewHealthHandler creates a new health handler
func NewHealthHandler(db *storage.DB) *HealthHandler {
	return &HealthHandler{
		db:        db,
		startTime: time.Now(),
	}
}

// ComponentHealth represents the health status of a single component
type ComponentHealth struct {
	Status  string `json:"status"`  // "ok" | "error"
	Message string `json:"message,omitempty"`
}

// SystemHealth represents system-level health metrics
type SystemHealth struct {
	Goroutines    int     `json:"goroutines"`
	MemoryAllocMB float64 `json:"memory_alloc_mb"`
	DBSizeMB      float64 `json:"db_size_mb"`
	WorkerCount   int     `json:"worker_count"`
}

// HealthResponse is the response structure for the health check endpoint
type HealthResponse struct {
	Status     string                     `json:"status"`     // "healthy" | "degraded" | "unhealthy"
	Timestamp  int64                      `json:"timestamp"`
	Uptime     int64                      `json:"uptime_seconds"`
	Version    string                     `json:"version"`
	Components map[string]ComponentHealth `json:"components"`
	System     SystemHealth               `json:"system"`
}

// Health performs a comprehensive health check
// GET /api/health
func (h *HealthHandler) Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	response := HealthResponse{
		Timestamp:  time.Now().UnixMilli(),
		Uptime:     int64(time.Since(h.startTime).Seconds()),
		Version:    "1.0.0",
		Components: make(map[string]ComponentHealth),
		System:     SystemHealth{},
	}

	// Check database health
	dbStatus, dbSize := h.checkDatabase()
	response.Components["database"] = dbStatus
	response.System.DBSizeMB = dbSize

	// Collect system metrics
	response.System.Goroutines = runtime.NumGoroutine()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	response.System.MemoryAllocMB = float64(m.Alloc) / 1024 / 1024

	// Worker count will be set from main
	response.System.WorkerCount = 0

	// Determine overall status
	overallStatus := "healthy"
	for _, comp := range response.Components {
		if comp.Status != "ok" {
			overallStatus = "degraded"
			break
		}
	}
	response.Status = overallStatus

	json.NewEncoder(w).Encode(response)
}

// checkDatabase checks the database connection and returns file size
func (h *HealthHandler) checkDatabase() (ComponentHealth, float64) {
	// Try to ping the database
	var result int
	err := h.db.Conn().QueryRow("SELECT 1").Scan(&result)
	if err != nil {
		slog.Warn("Database health check failed", "error", err)
		return ComponentHealth{
			Status:  "error",
			Message: "database connection failed",
		}, 0
	}

	// Get DB path - for now we'll use a default path
	// In production, this should be passed in via RouterConfig
	dbPath := "./data/collector.db"
	if info, err := os.Stat(dbPath); err == nil {
		sizeMB := float64(info.Size()) / 1024 / 1024
		return ComponentHealth{
			Status:  "ok",
			Message: "connected",
		}, sizeMB
	}

	return ComponentHealth{
		Status:  "ok",
		Message: "connected",
	}, 0
}

// SetWorkerCount sets the worker count (called from main)
func (h *HealthHandler) SetWorkerCount(count int) {
	// This is a placeholder - the worker count will be passed in SystemHealth
	// In the actual implementation, we might want to store this
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
		slog.Error("Failed to get release health", "error", err)
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
		slog.Error("Failed to get session stats", "error", err)
		http.Error(w, "Failed to get session stats", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(result)
}

// RegisterRoutes registers all health routes
func (h *HealthHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/query/release-health", h.GetReleaseHealth)
	mux.HandleFunc("GET /api/query/session-stats", h.GetSessionStats)
	mux.HandleFunc("GET /api/health", h.Health)
}
