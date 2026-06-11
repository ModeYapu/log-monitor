package handler

import (
	"encoding/json"
	"compress/gzip"
	"fmt"
	"github.com/logmonitor/collector/middleware"
	"net/http"
	"strconv"
	"strings"
)

type RecordingHandler struct {
	hub  *CoBrowseHub
	db   CoBrowseDB
}

// NewRecordingHandler creates a new recording handler
func NewRecordingHandler(hub *CoBrowseHub, db CoBrowseDB) *RecordingHandler {
	return &RecordingHandler{
		hub: hub,
		db:  db,
	}
}

// RegisterRoutes registers recording routes
func (h *RecordingHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/query/recordings", h.listRecordings)
	mux.HandleFunc("GET /api/query/recordings/", h.getRecordingWithRouting)
	mux.HandleFunc("DELETE /api/query/recordings/", h.deleteRecording)
	mux.HandleFunc("GET /api/query/live-sessions", h.getLiveSessions)
	mux.HandleFunc("GET /api/query/sessions/", h.getSessionEvents)
}

// listRecordings returns a list of recordings
func (h *RecordingHandler) listRecordings(w http.ResponseWriter, r *http.Request) {
	// Authenticate admin request
	if !h.hub.auth.AuthenticateAdmin(r) {
		middleware.WriteAuthError(w, "Invalid or missing admin token")
		return
	}

	limit := 50
	offset := 0
	filters := make(map[string]interface{})

	if l := r.URL.Query().Get("limit"); l != "" {
		fmt.Sscanf(l, "%d", &limit)
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		fmt.Sscanf(o, "%d", &offset)
	}

	// Parse filter parameters
	if appID := r.URL.Query().Get("app_id"); appID != "" {
		filters["app_id"] = appID
	}
	if status := r.URL.Query().Get("status"); status != "" {
		filters["status"] = status
	}
	if startFrom := r.URL.Query().Get("start_from"); startFrom != "" {
		if ts, err := parseTimestamp(startFrom); err == nil {
			filters["start_from"] = ts
		}
	}
	if startTo := r.URL.Query().Get("start_to"); startTo != "" {
		if ts, err := parseTimestamp(startTo); err == nil {
			filters["start_to"] = ts
		}
	}
	if minDuration := r.URL.Query().Get("min_duration"); minDuration != "" {
		if d, err := strconv.ParseInt(minDuration, 10, 64); err == nil {
			filters["min_duration"] = d
		}
	}
	if maxDuration := r.URL.Query().Get("max_duration"); maxDuration != "" {
		if d, err := strconv.ParseInt(maxDuration, 10, 64); err == nil {
			filters["max_duration"] = d
		}
	}
	if search := r.URL.Query().Get("search"); search != "" {
		filters["search"] = search
	}

	recordings, err := h.db.GetRecordings(limit, offset, filters)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"data": recordings,
	})
}

// parseTimestamp parses a timestamp string to int64
func parseTimestamp(s string) (int64, error) {
	var ts int64
	_, err := fmt.Sscanf(s, "%d", &ts)
	return ts, err
}

// getRecordingWithRouting routes recording requests to the appropriate handler
func (h *RecordingHandler) getRecordingWithRouting(w http.ResponseWriter, r *http.Request) {
	// Authenticate admin request
	if !h.hub.auth.AuthenticateAdmin(r) {
		middleware.WriteAuthError(w, "Invalid or missing admin token")
		return
	}

	path := r.URL.Path
	sessionID := strings.TrimPrefix(path, "/api/query/recordings/")
	if sessionID == "" {
		http.Error(w, "Session ID required", http.StatusBadRequest)
		return
	}

	// Check if requesting stats
	if sessionID == "stats" || strings.HasSuffix(sessionID, "/stats") {
		h.getRecordingStats(w, r)
		return
	}

	// Check if requesting events
	if r.URL.Query().Get("events") == "true" {
		h.getRecordingEvents(w, r, sessionID)
		return
	}

	// Get recording metadata
	h.getRecording(w, r, sessionID)
}

// getRecording returns a single recording with events
func (h *RecordingHandler) getRecording(w http.ResponseWriter, r *http.Request, sessionID string) {
	recording, err := h.db.GetRecording(sessionID)
	if err != nil {
		http.Error(w, "Recording not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(recording)
}

// getRecordingEvents returns events for a recording with gzip compression
func (h *RecordingHandler) getRecordingEvents(w http.ResponseWriter, r *http.Request, sessionID string) {
	limit := 1000
	offset := 0
	if l := r.URL.Query().Get("limit"); l != "" {
		fmt.Sscanf(l, "%d", &limit)
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		fmt.Sscanf(o, "%d", &offset)
	}

	events, err := h.db.GetRecordingEvents(sessionID, limit, offset)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Check if client accepts gzip encoding
	acceptsGzip := strings.Contains(r.Header.Get("Accept-Encoding"), "gzip")
	if acceptsGzip {
		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Set("Content-Type", "application/json")
		gz := gzip.NewWriter(w)
		defer gz.Close()
		json.NewEncoder(gz).Encode(map[string]interface{}{
			"sessionId": sessionID,
			"events":    events,
		})
	} else {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"sessionId": sessionID,
			"events":    events,
		})
	}
}

// getRecordingStats returns statistics for a recording
func (h *RecordingHandler) getRecordingStats(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	sessionID := strings.TrimSuffix(strings.TrimPrefix(path, "/api/query/recordings/"), "/stats")
	if sessionID == "" {
		http.Error(w, "Session ID required", http.StatusBadRequest)
		return
	}

	stats, err := h.db.GetRecordingStats(sessionID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// deleteRecording deletes a recording
func (h *RecordingHandler) deleteRecording(w http.ResponseWriter, r *http.Request) {
	// Authenticate admin request
	if !h.hub.auth.AuthenticateAdmin(r) {
		middleware.WriteAuthError(w, "Invalid or missing admin token")
		return
	}

	path := r.URL.Path
	sessionID := path[len("/api/query/recordings/"):]
	if sessionID == "" {
		http.Error(w, "Session ID required", http.StatusBadRequest)
		return
	}

	if err := h.db.DeleteRecording(sessionID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
	})
}

// getLiveSessions returns currently active sessions
func (h *RecordingHandler) getLiveSessions(w http.ResponseWriter, r *http.Request) {
	// Authenticate admin request
	if !h.hub.auth.AuthenticateAdmin(r) {
		middleware.WriteAuthError(w, "Invalid or missing admin token")
		return
	}

	sessions := h.hub.GetLiveSessions()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"data": sessions,
	})
}

// getSessionEvents returns events associated with a session
func (h *RecordingHandler) getSessionEvents(w http.ResponseWriter, r *http.Request) {
	// Authenticate admin request
	if !h.hub.auth.AuthenticateAdmin(r) {
		middleware.WriteAuthError(w, "Invalid or missing admin token")
		return
	}

	path := r.URL.Path
	sessionID := strings.TrimPrefix(path, "/api/query/sessions/")
	if sessionID == "" {
		http.Error(w, "Session ID required", http.StatusBadRequest)
		return
	}

	// Get limit from query param
	limit := 100
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 1000 {
			limit = l
		}
	}

	// Get events for this session
	events, err := h.db.GetSessionEvents(sessionID, limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Get error count
	errorCount, err := h.db.GetSessionErrorCount(sessionID)
	if err != nil {
		errorCount = 0
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"sessionId":   sessionID,
		"events":      events,
		"errorCount":  errorCount,
		"totalEvents": len(events),
	})
}
