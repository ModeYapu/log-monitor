package handler

import (
	"encoding/json"
	"github.com/gorilla/websocket"
	"github.com/logmonitor/collector/middleware"
	"github.com/logmonitor/collector/storage"
	"log/slog"
	"net/http"
	"time"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func (h *CoBrowseHub) removeSession(sessionID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.sessions, sessionID)
	slog.Info("[CoBrowse] Session removed", "session", sessionID)
}

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

	// Check for existing session (reconnect scenario)
	h.mu.Lock()
	existingHub, exists := h.sessions[sessionID]

	if exists {
		if existingHub.userConn != nil {
			// Reconnect: replace old user connection
			oldConn := existingHub.userConn
			existingHub.userConn = conn
			h.mu.Unlock()

			go func() {
				oldConn.WriteMessage(websocket.CloseMessage,
					websocket.FormatCloseMessage(websocket.CloseGoingAway, "replaced by new connection"))
				oldConn.Close()
			}()

			slog.Info("[CoBrowse] User reconnected", "session", sessionID)
			existingHub.resetStopCh()
			go existingHub.handleUserMessages(h.db)
			go existingHub.pingUser(conn)
			return
		}
		// Old session closed — clean up and create new
		delete(h.sessions, sessionID)
	}

	// Create new session hub
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
		SessionID: sessionID,
		AppID:     appID,
		StartTime: hub.startTime,
		URL:       url,
		UA:        ua,
		Status:    "recording",
		CreatedAt: hub.startTime,
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

	// Broadcast viewer count to all viewers
	viewerCountMsg, _ := json.Marshal(map[string]interface{}{"type": "viewer-count", "count": viewerCount})
	hub.mu.RLock()
	for c := range hub.viewerConns {
		c.WriteMessage(websocket.TextMessage, viewerCountMsg)
	}
	hub.mu.RUnlock()

	// Handle viewer messages (control commands)
	go hub.handleViewerMessages(conn, hub)

	// Send ping to keep connection alive
	go hub.pingViewer(conn)
}
