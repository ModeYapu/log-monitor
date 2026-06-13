package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/logmonitor/collector/storage"
)

// SessionHandler handles user session queries
type SessionHandler struct {
	db *storage.DB
}

// NewSessionHandler creates a new session handler
func NewSessionHandler(db *storage.DB) *SessionHandler {
	return &SessionHandler{
		db: db,
	}
}

// ListSessions returns a paginated list of sessions with filters
func (h *SessionHandler) ListSessions(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Parse query parameters
	filters := make(map[string]interface{})

	if appID := r.URL.Query().Get("app_id"); appID != "" {
		filters["app_id"] = appID
	}

	if userID := r.URL.Query().Get("user_id"); userID != "" {
		filters["user_id"] = userID
	}

	if startFrom := r.URL.Query().Get("start_from"); startFrom != "" {
		if ts, err := strconv.ParseInt(startFrom, 10, 64); err == nil {
			filters["start_from"] = ts
		}
	}

	if startTo := r.URL.Query().Get("start_to"); startTo != "" {
		if ts, err := strconv.ParseInt(startTo, 10, 64); err == nil {
			filters["start_to"] = ts
		}
	}

	if projectID := r.URL.Query().Get("project_id"); projectID != "" {
		if pid, err := strconv.ParseInt(projectID, 10, 64); err == nil {
			filters["project_id"] = pid
		}
	}

	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 1000 {
			limit = parsed
		}
	}

	offset := 0
	if o := r.URL.Query().Get("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	// Get sessions and total count
	sessions, err := h.db.GetSessionList(filters, limit, offset)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "Failed to get sessions",
		})
		return
	}

	total, err := h.db.GetSessionListCount(filters)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "Failed to get session count",
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"data":   sessions,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

// GetSessionDetail returns session detail with routing for sub-paths
func (h *SessionHandler) GetSessionDetail(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	path := r.URL.Path
	sessionID := strings.TrimPrefix(path, "/api/query/sessions/")

	// Check if requesting journey
	if strings.HasSuffix(sessionID, "/journey") {
		sessionID = strings.TrimSuffix(sessionID, "/journey")
		h.GetSessionJourney(w, r, sessionID)
		return
	}

	// Check if requesting events
	if strings.HasSuffix(sessionID, "/events") {
		sessionID = strings.TrimSuffix(sessionID, "/events")
		h.GetSessionEvents(w, r, sessionID)
		return
	}

	// Default: return session detail
	if sessionID == "" {
		http.Error(w, "Session ID required", http.StatusBadRequest)
		return
	}

	// Get limit for events
	eventLimit := 200
	if l := r.URL.Query().Get("event_limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 1000 {
			eventLimit = parsed
		}
	}

	// Get session summary from events
	filters := map[string]interface{}{"session_id": sessionID}
	summaries, err := h.db.GetSessionList(filters, 1, 0)
	if err != nil || len(summaries) == 0 {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "Session not found",
		})
		return
	}

	summary := summaries[0]

	// Get events for the session
	events, err := h.db.GetSessionEvents(sessionID, eventLimit)
	if err != nil {
		events = []storage.EventRecord{}
	}

	// Calculate stats
	errorCount := int64(0)
	warningCount := int64(0)
	pageSet := make(map[string]bool)
	eventTypes := make(map[string]int64)

	for _, e := range events {
		switch e.Level {
		case "error":
			errorCount++
		case "warn":
			warningCount++
		}
		if e.URL != "" {
			pageSet[e.URL] = true
		}
		eventTypes[e.Level]++
	}

	// Get top pages
	topPages := make([]string, 0, len(pageSet))
	for page := range pageSet {
		topPages = append(topPages, page)
	}

	// Check for recording
	recording, _ := h.db.GetRecording(sessionID)
	var recordingInfo map[string]interface{}
	if recording != nil {
		recordingInfo = map[string]interface{}{
			"exists":     true,
			"startTime":  recording.StartTime,
			"endTime":    recording.EndTime,
			"durationMs": recording.DurationMs,
			"eventCount": recording.EventCount,
			"status":     recording.Status,
		}
	} else {
		recordingInfo = map[string]interface{}{
			"exists": false,
		}
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"session": map[string]interface{}{
			"sessionId":   summary.SessionID,
			"appId":       summary.AppID,
			"userId":      summary.UserID,
			"startTime":   summary.StartTime,
			"endTime":     summary.EndTime,
			"durationMs":  summary.DurationMs,
			"eventCount":  summary.EventCount,
			"errorCount":  summary.ErrorCount,
			"lastUrl":     summary.LastURL,
			"recording":   recordingInfo,
		},
		"events": events,
		"stats": map[string]interface{}{
			"errorCount":   errorCount,
			"warningCount": warningCount,
			"pageCount":    len(pageSet),
			"topPages":     topPages,
			"eventTypes":   eventTypes,
		},
	})
}

// GetSessionEvents returns events for a session
func (h *SessionHandler) GetSessionEvents(w http.ResponseWriter, r *http.Request, sessionID string) {
	if sessionID == "" {
		http.Error(w, "Session ID required", http.StatusBadRequest)
		return
	}

	limit := 200
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 1000 {
			limit = parsed
		}
	}

	events, err := h.db.GetSessionEvents(sessionID, limit)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "Failed to get events",
		})
		return
	}

	errorCount, err := h.db.GetSessionErrorCount(sessionID)
	if err != nil {
		errorCount = 0
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"sessionId":   sessionID,
		"events":      events,
		"errorCount":  errorCount,
		"totalEvents": len(events),
	})
}

// GetSessionJourney returns journey data for visualization
func (h *SessionHandler) GetSessionJourney(w http.ResponseWriter, r *http.Request, sessionID string) {
	if sessionID == "" {
		http.Error(w, "Session ID required", http.StatusBadRequest)
		return
	}

	events, err := h.db.GetSessionJourney(sessionID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "Failed to get journey",
		})
		return
	}

	// Build nodes (unique pages) and edges (transitions)
	type JourneyNode struct {
		ID         int   `json:"id"`
		URL        string `json:"url"`
		Label      string `json:"label"`
		FirstVisit int64 `json:"firstVisit"`
		VisitCount int   `json:"visitCount"`
		ErrorCount int   `json:"errorCount"`
	}

	type JourneyEdge struct {
		From      int   `json:"from"`
		To        int   `json:"to"`
		Count     int   `json:"count"`
		Timestamp int64 `json:"timestamp"`
	}

	type JourneyError struct {
		Timestamp int64  `json:"timestamp"`
		Message   string `json:"message"`
		URL       string `json:"url"`
	}

	nodeMap := make(map[string]*JourneyNode)
	nodeIndex := 0
	var edges []JourneyEdge
	var errors []JourneyError

	prevURL := ""
	var prevNodeID int

	for _, e := range events {
		// Track errors
		if e.Level == "error" {
			errors = append(errors, JourneyError{
				Timestamp: e.CreatedAt,
				Message:   e.Message,
				URL:       e.URL,
			})
		}

		// Track page transitions
		if e.URL != "" && e.URL != prevURL {
			// Find or create node
			var nodeID int
			if node, exists := nodeMap[e.URL]; exists {
				nodeID = node.ID
				node.VisitCount++
				if e.Level == "error" {
					node.ErrorCount++
				}
			} else {
				nodeID = nodeIndex
				label := e.URL
				if len(label) > 50 {
					label = "..." + label[len(label)-47:]
				}
				nodeMap[e.URL] = &JourneyNode{
					ID:         nodeID,
					URL:        e.URL,
					Label:      label,
					FirstVisit: e.CreatedAt,
					VisitCount: 1,
					ErrorCount: 0,
				}
				if e.Level == "error" {
					nodeMap[e.URL].ErrorCount = 1
				}
				nodeIndex++
			}

			// Create edge from previous page
			if prevURL != "" {
				// Find existing edge or create new one
				edgeFound := false
				for i, edge := range edges {
					if edge.From == prevNodeID && edge.To == nodeID {
						edges[i].Count++
						if e.CreatedAt > edge.Timestamp {
							edges[i].Timestamp = e.CreatedAt
						}
						edgeFound = true
						break
					}
				}
				if !edgeFound {
					edges = append(edges, JourneyEdge{
						From:      prevNodeID,
						To:        nodeID,
						Count:     1,
						Timestamp: e.CreatedAt,
					})
				}
			}

			prevURL = e.URL
			prevNodeID = nodeID
		}
	}

	// Convert node map to slice
	var nodes []JourneyNode
	for _, node := range nodeMap {
		nodes = append(nodes, *node)
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"sessionId": sessionID,
		"nodes":     nodes,
		"edges":     edges,
		"errors":    errors,
	})
}
