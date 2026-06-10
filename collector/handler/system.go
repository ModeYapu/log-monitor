package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/logmonitor/collector/storage"
)

// SystemHandler handles system-related requests
type SystemHandler struct {
	systemStore  storage.SystemStore
	db          *storage.DB // Keep for legacy methods
	dbPath      string
	startTime   time.Time
	retentionDays int
	version     string
}

// NewSystemHandler creates a new system handler
func NewSystemHandler(db *storage.DB, dbPath string, retentionDays int) *SystemHandler {
	return &SystemHandler{
		systemStore:   db,
		db:            db,
		dbPath:        dbPath,
		startTime:     time.Now(),
		retentionDays: retentionDays,
		version:       "1.0.0",
	}
}

// NewSystemHandlerWithStore creates a new system handler with explicit store
func NewSystemHandlerWithStore(systemStore storage.SystemStore, db *storage.DB, dbPath string, retentionDays int) *SystemHandler {
	return &SystemHandler{
		systemStore:   systemStore,
		db:            db,
		dbPath:        dbPath,
		startTime:     time.Now(),
		retentionDays: retentionDays,
		version:       "1.0.0",
	}
}

// SystemInfo represents system information
type SystemInfo struct {
	Status          string  `json:"status"`
	Version         string  `json:"version"`
	DBSize          int64   `json:"dbSize"`
	TotalEvents     int64   `json:"totalEvents"`
	TotalRecordings int64   `json:"totalRecordings"`
	RetentionDays   int     `json:"retentionDays"`
	Uptime          int64   `json:"uptime"`
	ServerTime      int64   `json:"serverTime"`
	LastCleanupTime int64   `json:"lastCleanupTime"`
}

// CleanupResult represents the result of a cleanup operation
type CleanupResult struct {
	EventsDeleted           int64 `json:"eventsDeleted"`
	RecordingEventsDeleted  int64 `json:"recordingEventsDeleted"`
	AlertLogsDeleted        int64 `json:"alertLogsDeleted"`
	LastCleanupTime         int64 `json:"lastCleanupTime"`
}

// GetSystemInfo returns system information
func (h *SystemHandler) GetSystemInfo(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Get database file size
	dbSize := int64(0)
	if info, err := os.Stat(h.dbPath); err == nil {
		dbSize = info.Size()
	}

	// Get total events count
	totalEvents := int64(0)
	if apps, err := h.db.GetApps(); err == nil {
		for _, app := range apps {
			totalEvents += app.TotalEvents
		}
	}

	// Get total recordings count
	totalRecordings := int64(0)
	if recordings, err := h.db.GetRecordings(1000, 0, nil); err == nil {
		totalRecordings = int64(len(recordings))
	}

	// Get last cleanup time
	lastCleanupTime := h.db.GetLastCleanupTime()

	info := SystemInfo{
		Status:          "ok",
		Version:         h.version,
		DBSize:          dbSize,
		TotalEvents:     totalEvents,
		TotalRecordings: totalRecordings,
		RetentionDays:   h.retentionDays,
		Uptime:          int64(time.Since(h.startTime).Seconds()),
		ServerTime:      time.Now().UnixMilli(),
		LastCleanupTime: lastCleanupTime,
	}

	json.NewEncoder(w).Encode(info)
}

// TriggerCleanup triggers an immediate cleanup of old data
func (h *SystemHandler) TriggerCleanup(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Support both POST and DELETE methods
	if r.Method != http.MethodPost && r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse days parameter from query string
	days := h.retentionDays
	if daysStr := r.URL.Query().Get("days"); daysStr != "" {
		if d, err := strconv.Atoi(daysStr); err == nil && d > 0 {
			days = d
		}
	}

	// Perform cleanup
	cleanupResult := h.db.CleanupOldDataWithDays(days)

	slog.Error("[cleanup] Manual cleanup triggered: %d days retention", days)
	slog.Error("[cleanup] Deleted: %d events, %d recording_events, %d alert_logs",
		cleanupResult.DeletedEvents, cleanupResult.DeletedScreenshots, cleanupResult.TotalFilesFreed)

	json.NewEncoder(w).Encode(cleanupResult)
}

// ==================== Slice 4: Admin APIs ====================

// AdminHandler handles admin-specific requests
type AdminHandler struct {
	db *storage.DB
}

// NewAdminHandler creates a new admin handler
func NewAdminHandler(db *storage.DB) *AdminHandler {
	return &AdminHandler{db: db}
}

// GetStorageStats returns storage statistics
func (h *AdminHandler) GetStorageStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	stats, err := h.db.GetStorageStats()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(stats)
}

// GetRetentionPolicy returns the current retention policy
func (h *AdminHandler) GetRetentionPolicy(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	policy, err := h.db.GetRetentionPolicy()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(policy)
}

// SetRetentionPolicy updates the retention policy
func (h *AdminHandler) SetRetentionPolicy(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPut && r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var policy storage.RetentionPolicy
	if err := json.NewDecoder(r.Body).Decode(&policy); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.db.SetRetentionPolicy(&policy); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	slog.Info("[admin] Retention policy updated: events=%d days, recordings=%d days, screenshots=%d days, alerts=%d days",
		policy.Events, policy.RecordingEvents, policy.Screenshots, policy.AlertLogs)

	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Retention policy updated successfully",
		"policy":  policy,
	})
}

// TriggerManualCleanup triggers manual cleanup with current retention policy
func (h *AdminHandler) TriggerManualCleanup(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get current retention policy
	policy, err := h.db.GetRetentionPolicy()
	if err != nil {
		http.Error(w, "Failed to get retention policy", http.StatusInternalServerError)
		return
	}

	// Perform cleanup with policy
	result, err := h.db.CleanupOldDataWithPolicy(policy)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	slog.Info("[admin] Manual cleanup completed: %d events, %d recording_events, %d alert_logs deleted, %d bytes freed",
		result.EventsDeleted, result.RecordingEventsDeleted, result.AlertLogsDeleted, result.FreedBytes)

	json.NewEncoder(w).Encode(result)
}
