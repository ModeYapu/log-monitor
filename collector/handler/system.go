package handler

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/logmonitor/collector/storage"
)

// SystemHandler handles system-related requests
type SystemHandler struct {
	db          *storage.DB
	dbPath      string
	startTime   time.Time
	retentionDays int
}

// NewSystemHandler creates a new system handler
func NewSystemHandler(db *storage.DB, dbPath string, retentionDays int) *SystemHandler {
	return &SystemHandler{
		db:            db,
		dbPath:        dbPath,
		startTime:     time.Now(),
		retentionDays: retentionDays,
	}
}

// SystemInfo represents system information
type SystemInfo struct {
	Status          string  `json:"status"`
	DBSize          int64   `json:"dbSize"`
	TotalEvents     int64   `json:"totalEvents"`
	TotalRecordings int64   `json:"totalRecordings"`
	RetentionDays   int     `json:"retentionDays"`
	Uptime          int64   `json:"uptime"`
	ServerTime      int64   `json:"serverTime"`
}

// CleanupResult represents the result of a cleanup operation
type CleanupResult struct {
	EventsDeleted           int64 `json:"eventsDeleted"`
	RecordingEventsDeleted  int64 `json:"recordingEventsDeleted"`
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

	info := SystemInfo{
		Status:          "ok",
		DBSize:          dbSize,
		TotalEvents:     totalEvents,
		TotalRecordings: totalRecordings,
		RetentionDays:   h.retentionDays,
		Uptime:          int64(time.Since(h.startTime).Seconds()),
		ServerTime:      time.Now().UnixMilli(),
	}

	json.NewEncoder(w).Encode(info)
}

// TriggerCleanup triggers an immediate cleanup of old data
func (h *SystemHandler) TriggerCleanup(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Calculate cutoff time
	cutoff := time.Now().AddDate(0, 0, -h.retentionDays).UnixMilli()

	// Delete old events
	result1, err := h.db.Conn().Exec("DELETE FROM events WHERE created_at < ?", cutoff)
	eventsDeleted := int64(0)
	if err != nil {
		log.Printf("[cleanup] Failed to delete old events: %v", err)
	} else {
		eventsDeleted, _ = result1.RowsAffected()
	}

	// Delete orphaned recording events
	result2, err := h.db.Conn().Exec(`
		DELETE FROM recording_events
		WHERE session_id NOT IN (SELECT session_id FROM recordings)
	`)
	recordingEventsDeleted := int64(0)
	if err != nil {
		log.Printf("[cleanup] Failed to delete orphaned recording_events: %v", err)
	} else {
		recordingEventsDeleted, _ = result2.RowsAffected()
	}

	result := CleanupResult{
		EventsDeleted:          eventsDeleted,
		RecordingEventsDeleted: recordingEventsDeleted,
	}

	log.Printf("[cleanup] Manual cleanup: %d events, %d recording_events deleted",
		eventsDeleted, recordingEventsDeleted)

	json.NewEncoder(w).Encode(result)
}
