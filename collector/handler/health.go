package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"syscall"
	"time"

	"github.com/logmonitor/collector/storage"
)

// recentErrorWindow is the look-back window used to compute the recent error
// count surfaced by the health endpoint.
const recentErrorWindow = 15 * time.Minute

// HealthHandler handles release health, session stats queries, and system health checks.
// It depends only on storage interfaces (R013 migration), not the concrete *storage.DB.
type HealthHandler struct {
	analytics   storage.AnalyticsStore
	system      storage.SystemStore
	events      storage.EventStore
	dbPath      string
	startTime   time.Time
	workerCount int
}

// NewHealthHandler creates a new health handler backed by storage interfaces.
// dbPath is the on-disk SQLite path used to report DB size and free disk space.
func NewHealthHandler(dbPath string, analytics storage.AnalyticsStore, system storage.SystemStore, events storage.EventStore) *HealthHandler {
	return &HealthHandler{
		analytics: analytics,
		system:    system,
		events:    events,
		dbPath:    dbPath,
		startTime: time.Now(),
	}
}

// ComponentHealth represents the health status of a single component
type ComponentHealth struct {
	Status  string `json:"status"` // "ok" | "error"
	Message string `json:"message,omitempty"`
}

// SystemHealth represents system-level health metrics
type SystemHealth struct {
	Goroutines    int     `json:"goroutines"`
	MemoryAllocMB float64 `json:"memory_alloc_mb"`
	DBSizeMB      float64 `json:"db_size_mb"`
	DiskFreeMB    float64 `json:"disk_free_mb"`
	DiskTotalMB   float64 `json:"disk_total_mb"`
	WorkerCount   int     `json:"worker_count"`
}

// HealthResponse is the response structure for the health check endpoint
type HealthResponse struct {
	Status         string                     `json:"status"` // "healthy" | "degraded" | "unhealthy"
	Timestamp      int64                      `json:"timestamp"`
	Uptime         int64                      `json:"uptime_seconds"`
	Version        string                     `json:"version"`
	Components     map[string]ComponentHealth `json:"components"`
	System         SystemHealth               `json:"system"`
	RecentErrors   int64                      `json:"recent_errors_15m"`
	EventCount     int64                      `json:"event_count"`
}

// Health performs a comprehensive health check.
// GET /api/health — returns database connection status, disk usage, and recent error count.
func (h *HealthHandler) Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	response := HealthResponse{
		Timestamp:  time.Now().UnixMilli(),
		Uptime:     int64(time.Since(h.startTime).Seconds()),
		Version:    "1.0.0",
		Components: make(map[string]ComponentHealth),
		System:     SystemHealth{},
	}

	// Check database connectivity via the interface (no concrete *storage.DB).
	dbStatus := h.checkDatabase()
	response.Components["database"] = dbStatus

	// Disk usage (DB file size + filesystem free/total) and aggregate counts.
	dbSize, diskFree, diskTotal, eventCount := h.collectStorageMetrics()
	response.System.DBSizeMB = dbSize
	response.System.DiskFreeMB = diskFree
	response.System.DiskTotalMB = diskTotal
	response.EventCount = eventCount

	// Recent error count over the look-back window.
	response.RecentErrors = h.collectRecentErrors()

	// Collect runtime system metrics.
	response.System.Goroutines = runtime.NumGoroutine()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	response.System.MemoryAllocMB = float64(m.Alloc) / 1024 / 1024
	response.System.WorkerCount = h.workerCount
	// Determine overall status: any failing component degrades the system.
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

// checkDatabase verifies the database connection is alive through the SystemStore
// interface (Ping), avoiding any dependency on the concrete *storage.DB type.
func (h *HealthHandler) checkDatabase() ComponentHealth {
	if h.system == nil {
		return ComponentHealth{Status: "error", Message: "system store not configured"}
	}
	if err := h.system.Ping(); err != nil {
		slog.Warn("Database health check failed", "error", err)
		return ComponentHealth{Status: "error", Message: "database connection failed"}
	}
	return ComponentHealth{Status: "ok", Message: "connected"}
}

// collectStorageMetrics reports DB file size, filesystem free/total space, and the
// total event count. All values are best-effort: a missing file or stat error yields 0.
func (h *HealthHandler) collectStorageMetrics() (dbSizeMB, diskFreeMB, diskTotalMB float64, eventCount int64) {
	// DB file size from disk (the authoritative on-disk footprint).
	if h.dbPath != "" && h.dbPath != ":memory:" {
		if info, err := os.Stat(h.dbPath); err == nil {
			dbSizeMB = float64(info.Size()) / 1024 / 1024
		}
		// Filesystem capacity for the partition holding the DB.
		diskFreeMB, diskTotalMB = diskUsage(h.dbPath)
	}

	// Prefer storage stats for DB size (covers WAL/shm sidecar size) and counts.
	if h.system != nil {
		if stats, err := h.system.GetStorageStats(); err == nil && stats != nil {
			if stats.DatabaseSize > 0 {
				dbSizeMB = float64(stats.DatabaseSize) / 1024 / 1024
			}
			eventCount = stats.EventCount
		}
	}
	return dbSizeMB, diskFreeMB, diskTotalMB, eventCount
}

// collectRecentErrors returns the number of error-level events in the last
// recentErrorWindow. Failures are non-fatal and yield 0.
func (h *HealthHandler) collectRecentErrors() int64 {
	if h.events == nil {
		return 0
	}
	since := time.Now().Add(-recentErrorWindow).UnixMilli()
	count, err := h.events.CountRecentErrors(since)
	if err != nil {
		slog.Warn("Failed to count recent errors", "error", err)
		return 0
	}
	return count
}

// diskUsage returns (freeMB, totalMB) for the filesystem holding path via statvfs.
func diskUsage(path string) (freeMB, totalMB float64) {
	dir := path
	if info, err := os.Stat(path); err == nil && !info.IsDir() {
		dir = filepath.Dir(path)
	}
	var stat syscall.Statfs_t
	if err := syscall.Statfs(dir, &stat); err != nil {
		return 0, 0
	}
	// Bsize is the filesystem block size; blocks/avail are block counts.
	totalBytes := stat.Blocks * uint64(stat.Bsize)
	freeBytes := stat.Bavail * uint64(stat.Bsize)
	return float64(freeBytes) / 1024 / 1024, float64(totalBytes) / 1024 / 1024
}

// SetWorkerCount sets the worker count (called from main).
func (h *HealthHandler) SetWorkerCount(count int) {
	// Stored on the struct so the next Health() call reports it.
	h.workerCount = count
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

	result, err := h.analytics.GetReleaseHealth(appID, startTime, endTime)
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

	result, err := h.analytics.GetSessionStats(appID, startTime, endTime)
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
