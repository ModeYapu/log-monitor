package handler

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/logmonitor/collector/middleware"
	"github.com/logmonitor/collector/model"
	"github.com/logmonitor/collector/storage"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// SessionHub manages a single cobrowsing session
type SessionHub struct {
	sessionID   string
	appID       string
	url         string
	ua          string
	startTime   int64
	userConn    *websocket.Conn
	viewerConns map[*websocket.Conn]bool
	events      []storage.RecordingEventData
	eventCount  int
	maxEvents   int // Maximum events to store to prevent memory leaks
	mu          sync.RWMutex
	closed      bool
	stopCh      chan struct{} // Channel to signal goroutines to stop
	db          CoBrowseDB
	parentHub   *CoBrowseHub // Reference to parent hub for removal
}

// CoBrowseHub manages all cobrowsing sessions
type CoBrowseHub struct {
	sessions       map[string]*SessionHub
	mu             sync.RWMutex
	db             CoBrowseDB
	auth           *middleware.AuthConfig
	allowedOrigins []string
	maxSessions    int // Maximum sessions to prevent resource exhaustion
}

// CoBrowseDB defines the database interface for cobrowsing
type CoBrowseDB interface {
	CreateRecording(recording storage.RecordingInfo) (int64, error)
	AddRecordingEvent(sessionID string, seq int, timestamp int64, eventData []byte) error
	GetRecording(sessionID string) (*storage.RecordingInfo, error)
	GetRecordings(limit, offset int, filters map[string]interface{}) ([]storage.RecordingInfo, error)
	GetRecordingEvents(sessionID string, limit, offset int) ([]storage.RecordingEventData, error)
	GetRecordingStats(sessionID string) (interface{}, error)
	DeleteRecording(sessionID string) error
	UpdateRecording(sessionID string, endTime int64, durationMs int64, eventCount int, status string) error
	GetSessionEvents(sessionID string, limit int) ([]storage.EventRecord, error)
	GetSessionErrorCount(sessionID string) (int64, error)
}

// NewCoBrowseHub creates a new cobrowse hub
func NewCoBrowseHub(db CoBrowseDB) *CoBrowseHub {
	return &CoBrowseHub{
		sessions:   make(map[string]*SessionHub),
		maxSessions: 1000, // Limit concurrent sessions to prevent resource exhaustion
		db:         db,
		auth:       middleware.NewAuthConfig(),
	}
}

// removeSession removes a session from the sessions map
func (h *CoBrowseHub) removeSession(sessionID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.sessions, sessionID)
	slog.Info("[CoBrowse] Session removed", "session", sessionID)
}

// SetAuthConfig sets the authentication configuration
func (h *CoBrowseHub) SetAuthConfig(auth *middleware.AuthConfig) {
	h.auth = auth
}

// SetAllowedOrigins configures allowed browser origins for admin viewer connections.
func (h *CoBrowseHub) SetAllowedOrigins(origins []string) {
	h.allowedOrigins = append([]string(nil), origins...)
}

// Close closes all active sessions
func (h *CoBrowseHub) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()

	for _, hub := range h.sessions {
		hub.close()
	}
	h.sessions = make(map[string]*SessionHub)
}

// GetLiveSessions returns all currently active sessions
func (h *CoBrowseHub) GetLiveSessions() []model.LiveSession {
	h.mu.RLock()
	defer h.mu.RUnlock()

	var sessions []model.LiveSession
	for _, hub := range h.sessions {
		hub.mu.RLock()
		if !hub.closed {
			sessions = append(sessions, model.LiveSession{
				SessionID:    hub.sessionID,
				AppID:        hub.appID,
				URL:          hub.url,
				UA:           hub.ua,
				ConnectedAt:  hub.startTime,
				ViewerCount:  len(hub.viewerConns),
				IsControlled: false,
			})
		}
		hub.mu.RUnlock()
	}
	return sessions
}

// HandleUserConnection handles WebSocket connection from user (being controlled)
func (h *CoBrowseHub) HandleUserConnection(w http.ResponseWriter, r *http.Request) {
	// Check session limit to prevent resource exhaustion
	h.mu.RLock()
	if len(h.sessions) >= h.maxSessions {
		h.mu.RUnlock()
		http.Error(w, "Too many active sessions", http.StatusServiceUnavailable)
		return
	}
	h.mu.RUnlock()

	sessionID := getSessionIDFromRequest(r)
	if sessionID == "" {
		http.Error(w, "Session ID required", http.StatusBadRequest)
		return
	}

	// Get app ID and UA from query params
	appID := r.URL.Query().Get("appId")
	ua := r.URL.Query().Get("ua")
	url := r.URL.Query().Get("url")

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("[CoBrowse] Failed to upgrade user connection", "error", err)
		return
	}

	// Create session hub
	hub := &SessionHub{
		sessionID:   sessionID,
		appID:       appID,
		url:         url,
		ua:          ua,
		startTime:   time.Now().UnixMilli(),
		userConn:    conn,
		viewerConns: make(map[*websocket.Conn]bool),
		events:      make([]storage.RecordingEventData, 0, 1000),
		maxEvents:   10000, // Limit events to prevent memory leaks
		stopCh:      make(chan struct{}),
		db:          h.db,
		parentHub:   h,
	}

	// Register session
	h.mu.Lock()
	h.sessions[sessionID] = hub
	h.mu.Unlock()

	// Create recording in database
	recording := storage.RecordingInfo{
		SessionID:    sessionID,
		AppID:        appID,
		StartTime:    hub.startTime,
		URL:          url,
		UA:           ua,
		Status:       "recording",
		CreatedAt:    hub.startTime,
	}
	if _, err := h.db.CreateRecording(recording); err != nil {
		slog.Error("[CoBrowse] Failed to create recording", "error", err)
	}

	slog.Info("[CoBrowse] User connected", "session", sessionID, "app", appID)

	// Start message handler
	go hub.handleUserMessages(h.db)

	// Send ping to keep connection alive
	go hub.pingUser(conn)
}

// HandleViewerConnection handles WebSocket connection from admin (viewer/controller)
func (h *CoBrowseHub) HandleViewerConnection(w http.ResponseWriter, r *http.Request) {
	// Authenticate admin connection
	if !h.auth.AuthenticateWebSocket(r, true) {
		middleware.WriteAuthError(w, "Invalid or missing admin token")
		return
	}

	sessionID := getSessionIDFromRequest(r)
	if sessionID == "" {
		http.Error(w, "Session ID required", http.StatusBadRequest)
		return
	}

	viewerUpgrader := websocket.Upgrader{
		ReadBufferSize:  4096,
		WriteBufferSize: 4096,
		CheckOrigin: func(r *http.Request) bool {
			return h.isAllowedViewerOrigin(r)
		},
	}
	conn, err := viewerUpgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("[CoBrowse] Failed to upgrade viewer connection", "error", err)
		return
	}

	h.mu.RLock()
	hub, exists := h.sessions[sessionID]
	h.mu.RUnlock()

	if !exists {
		// Send error and close
		conn.WriteJSON(map[string]string{"type": "error", "message": "Session not found"})
		conn.Close()
		return
	}

	hub.mu.Lock()
	hub.viewerConns[conn] = true
	viewerCount := len(hub.viewerConns)
	hub.mu.Unlock()

	slog.Info("[CoBrowse] Viewer connected", "session", sessionID, "totalViewers", viewerCount)

	// Send all accumulated events to viewer for proper replay
	hub.mu.RLock()
	eventCount := len(hub.events)
	slog.Info("[CoBrowse] Sending events to viewer", "eventCount", eventCount, "session", sessionID)
	sentCount := 0
	if eventCount > 0 {
		// First event is always the full snapshot
		fullSnapshot := hub.events[0]
		if err := conn.WriteJSON(map[string]interface{}{
			"type": "rrweb-full-snapshot",
			"data": json.RawMessage(fullSnapshot.EventData),
		}); err == nil {
			sentCount++
		} else {
			slog.Error("[CoBrowse] Error sending snapshot to viewer", "error", err)
		}

		// Send recent incremental events (max 50 to avoid overwhelming viewer)
		start := 1
		if len(hub.events)-1 > 50 {
			start = len(hub.events) - 50
		}
		for i := start; i < len(hub.events); i++ {
			event := hub.events[i]
			if err := conn.WriteJSON(map[string]interface{}{
				"type": "rrweb-event",
				"data": json.RawMessage(event.EventData),
			}); err == nil {
				sentCount++
			}
		}
	}
	hub.mu.RUnlock()
	slog.Info("[CoBrowse] Sent events to viewer", "sentCount", sentCount, "eventCount", eventCount)

	// Handle viewer messages (control commands)
	go hub.handleViewerMessages(conn, hub)

	// Send ping to keep connection alive
	go hub.pingViewer(conn)
}

// handleUserMessages processes messages from the user
func (hub *SessionHub) handleUserMessages(db CoBrowseDB) {
	conn := hub.userConn
	defer func() {
		hub.close()
	}()

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				slog.Error("[CoBrowse] User connection error", "error", err)
			}
			break
		}

		// Parse message
		var msg model.CoBrowseMessage
		if err := json.Unmarshal(message, &msg); err != nil {
			slog.Error("[CoBrowse] Failed to parse message", "error", err)
			continue
		}

		slog.Debug("[CoBrowse] User message", "type", msg.Type, "length", len(message))

		switch msg.Type {
		case "rrweb-event":
			hub.handleRRWebEvent(&msg, db)
		case "rrweb-full-snapshot":
			hub.handleFullSnapshot(&msg, db)
		case "pong":
			// Keep alive response, ignore
		default:
			slog.Error("[CoBrowse] Unknown message type", "type", msg.Type)
		}
	}
}

// handleViewerMessages processes messages from viewers (control commands)
func (hub *SessionHub) handleViewerMessages(conn *websocket.Conn, sessionHub *SessionHub) {
	defer func() {
		hub.mu.Lock()
		delete(hub.viewerConns, conn)
		hub.mu.Unlock()
		conn.Close()
		slog.Info("[CoBrowse] Viewer disconnected", "session", hub.sessionID)
	}()

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				slog.Error("[CoBrowse] Viewer connection error", "error", err)
			}
			break
		}

		// Parse control command
		var msg model.CoBrowseMessage
		if err := json.Unmarshal(message, &msg); err != nil {
			slog.Error("[CoBrowse] Failed to parse viewer message", "error", err)
			continue
		}

		// Forward control command to user
		if msg.Type == "control" {
			hub.mu.RLock()
			userConn := hub.userConn
			hub.mu.RUnlock()

			if userConn != nil {
				if err := userConn.WriteMessage(websocket.TextMessage, message); err != nil {
					slog.Error("[CoBrowse] Failed to send control to user", "error", err)
				} else {
					slog.Info("[CoBrowse] Control sent", "session", hub.sessionID, "action", msg.Action)
				}
			}
		}
	}
}

// handleRRWebEvent processes an rrweb event from the user
func (hub *SessionHub) handleRRWebEvent(msg *model.CoBrowseMessage, db CoBrowseDB) {
	hub.mu.Lock()
	defer hub.mu.Unlock()

	if hub.closed {
		return
	}

	// Parse events
	var events []model.RRWebEvent
	if err := json.Unmarshal(msg.Data, &events); err != nil {
		// Try single event
		var event model.RRWebEvent
		if err := json.Unmarshal(msg.Data, &event); err != nil {
			slog.Error("[CoBrowse] Failed to parse rrweb event", "error", err)
			return
		}
		events = []model.RRWebEvent{event}
	}

	// Store events
	for _, event := range events {
		hub.eventCount++
		// Serialize complete rrweb event for storage and replay
		fullEventJSON, _ := json.Marshal(event)
		recEvent := storage.RecordingEventData{
			SessionID: hub.sessionID,
			Seq:       hub.eventCount,
			Timestamp: event.Timestamp,
			EventData: string(fullEventJSON),
			CreatedAt: time.Now().UnixMilli(),
		}
		hub.events = append(hub.events, recEvent)

		// Save to database
		if err := db.AddRecordingEvent(hub.sessionID, hub.eventCount, event.Timestamp, fullEventJSON); err != nil {
			slog.Error("[CoBrowse] Failed to save event", "error", err)
		}
	}

	// Prevent unbounded memory growth by limiting events stored in memory
	if len(hub.events) > hub.maxEvents {
		// Remove oldest events to maintain the limit
		removed := len(hub.events) - hub.maxEvents
		hub.events = hub.events[removed:]
		slog.Info("[CoBrowse] Removed old events to prevent memory leak", "removed", removed, "session", hub.sessionID)
	}

	slog.Info("[CoBrowse] Stored events total, broadcasting to viewers", "eventCount", len(events), "session", hub.sessionID, "viewerCount", len(hub.viewerConns))

	// Broadcast to all viewers
	broadcastMsg := map[string]interface{}{
		"type": "rrweb-event",
		"data": msg.Data,
	}
	data, _ := json.Marshal(broadcastMsg)

	for viewerConn := range hub.viewerConns {
		if err := viewerConn.WriteMessage(websocket.TextMessage, data); err != nil {
			slog.Error("[CoBrowse] Failed to send to viewer", "error", err)
			delete(hub.viewerConns, viewerConn)
		}
	}
}

// handleFullSnapshot processes a full snapshot from the user
func (hub *SessionHub) handleFullSnapshot(msg *model.CoBrowseMessage, db CoBrowseDB) {
	hub.mu.Lock()
	defer hub.mu.Unlock()

	if hub.closed {
		return
	}

	// Store as first event
	recEvent := storage.RecordingEventData{
		SessionID: hub.sessionID,
		Seq:       0,
		Timestamp: time.Now().UnixMilli(),
		EventData: string(msg.Data),
		CreatedAt: time.Now().UnixMilli(),
	}

	// Replace existing events or append
	if len(hub.events) == 0 || hub.events[0].Seq != 0 {
		hub.events = append([]storage.RecordingEventData{recEvent}, hub.events...)
	} else {
		hub.events[0] = recEvent
	}

	// Save to database
	if err := db.AddRecordingEvent(hub.sessionID, 0, recEvent.Timestamp, msg.Data); err != nil {
		slog.Error("[CoBrowse] Failed to save full snapshot", "error", err)
	}

	// Broadcast to all viewers
	broadcastMsg := map[string]interface{}{
		"type": "rrweb-full-snapshot",
		"data": msg.Data,
	}
	data, _ := json.Marshal(broadcastMsg)

	for viewerConn := range hub.viewerConns {
		if err := viewerConn.WriteMessage(websocket.TextMessage, data); err != nil {
			slog.Error("[CoBrowse] Failed to send snapshot to viewer", "error", err)
			delete(hub.viewerConns, viewerConn)
		}
	}

	slog.Info("[CoBrowse] Full snapshot received", "session", hub.sessionID)
}

// close closes the session and cleans up resources
func (hub *SessionHub) close() {
	hub.mu.Lock()
	defer hub.mu.Unlock()

	if hub.closed {
		return
	}
	hub.closed = true

	// Signal all goroutines to stop
	close(hub.stopCh)

	// Close user connection
	if hub.userConn != nil {
		hub.userConn.Close()
	}

	// Close all viewer connections
	for viewerConn := range hub.viewerConns {
		viewerConn.Close()
	}
	hub.viewerConns = make(map[*websocket.Conn]bool)

	// Update recording in database with final stats
	if hub.db != nil && hub.sessionID != "" {
		endTime := time.Now().UnixMilli()
		durationMs := endTime - hub.startTime
		eventCount := len(hub.events)
		if err := hub.db.UpdateRecording(hub.sessionID, endTime, durationMs, eventCount, "completed"); err != nil {
			slog.Error("[CoBrowse] Failed to update recording on close", "error", err)
		} else {
			slog.Info("[CoBrowse] Recording updated", "session", hub.sessionID, "eventCount", eventCount, "durationMs", durationMs)
		}
	}

	// Remove from parent hub's sessions map to prevent memory leaks
	if hub.parentHub != nil {
		hub.parentHub.removeSession(hub.sessionID)
	}
}

// pingUser sends periodic ping to user connection
func (hub *SessionHub) pingUser(conn *websocket.Conn) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			hub.mu.RLock()
			if hub.closed || conn == nil {
				hub.mu.RUnlock()
				return
			}
			hub.mu.RUnlock()

			if err := conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"ping"}`)); err != nil {
				slog.Error("[CoBrowse] Ping failed", "error", err)
				return
			}
		case <-hub.stopCh:
			return
		}
	}
}

// pingViewer sends periodic ping to viewer connection
func (hub *SessionHub) pingViewer(conn *websocket.Conn) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			hub.mu.RLock()
			if hub.closed {
				hub.mu.RUnlock()
				return
			}
			hub.mu.RUnlock()

			if err := conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"ping"}`)); err != nil {
				return
			}
		case <-hub.stopCh:
			return
		}
	}
}

// RegisterRoutes registers cobrowse routes
func (h *CoBrowseHub) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/ws/cobrowse/", func(w http.ResponseWriter, r *http.Request) {
		// Extract session ID from path
		// Path format: /ws/cobrowse/{sessionId} or /ws/cobrowse/{sessionId}/view
		path := r.URL.Path

		// Normalize path
		path = strings.TrimPrefix(path, "/ws/cobrowse/")
		if path == "" {
			http.Error(w, "Session ID required", http.StatusBadRequest)
			return
		}

		// Check if viewer connection
		if strings.HasSuffix(path, "/view") {
			sessionID := strings.TrimSuffix(path, "/view")
			r = r.WithContext(context.WithValue(r.Context(), "sessionId", sessionID))
			h.HandleViewerConnection(w, r)
			return
		}

		// User connection
		sessionID := path
		r = r.WithContext(context.WithValue(r.Context(), "sessionId", sessionID))
		h.HandleUserConnection(w, r)
	})
}

func (h *CoBrowseHub) isAllowedViewerOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}

	if len(h.allowedOrigins) == 0 {
		parsed, err := url.Parse(origin)
		return err == nil && strings.EqualFold(parsed.Host, r.Host)
	}

	for _, allowedOrigin := range h.allowedOrigins {
		if allowedOrigin == "*" || allowedOrigin == origin {
			return true
		}
	}

	return false
}

// getSessionIDFromRequest extracts session ID from request
func getSessionIDFromRequest(r *http.Request) string {
	// Try context first
	if sessionID := r.Context().Value("sessionId"); sessionID != nil {
		if s, ok := sessionID.(string); ok {
			return s
		}
	}
	// Try PathValue (Go 1.22+)
	if sessionID := r.PathValue("sessionId"); sessionID != "" {
		return sessionID
	}
	// Fallback to query param
	return r.URL.Query().Get("sessionId")
}

// RecordingHandler handles recording-related HTTP requests
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
