package handler

import (
	"encoding/json"
	"github.com/gorilla/websocket"
	"github.com/logmonitor/collector/model"
	"github.com/logmonitor/collector/storage"
	"log/slog"
	"sync"
	"time"
)

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
		case "webrtc-offer-request":
			// Admin requests intervention — forward to all viewers
			// Actually this comes FROM viewer, forward TO user (but user initiated the request)
			// This case shouldn't happen from user side, but handle gracefully
			slog.Warn("[CoBrowse] Unexpected webrtc-offer-request from user", "session", hub.sessionID)
		case "webrtc-offer", "webrtc-answer", "webrtc-ice", "webrtc-stop", "webrtc-rejected":
			// WebRTC signaling — forward to all viewers
			hub.forwardToViewers(message)
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

		switch msg.Type {
		case "control":
			// Forward control command to user
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
		case "webrtc-offer-request":
			// Admin requests intervention — forward to user
			hub.mu.RLock()
			userConn := hub.userConn
			hub.mu.RUnlock()

			if userConn != nil {
				if err := userConn.WriteMessage(websocket.TextMessage, message); err != nil {
					slog.Error("[CoBrowse] Failed to send webrtc-offer-request to user", "error", err)
				} else {
					slog.Info("[CoBrowse] WebRTC offer request forwarded", "session", hub.sessionID)
				}
			}
		case "webrtc-offer", "webrtc-answer", "webrtc-ice", "webrtc-stop", "webrtc-rejected":
			// WebRTC signaling — forward to user
			hub.mu.RLock()
			userConn := hub.userConn
			hub.mu.RUnlock()

			if userConn != nil {
				if err := userConn.WriteMessage(websocket.TextMessage, message); err != nil {
					slog.Error("[CoBrowse] Failed to forward WebRTC signaling to user", "type", msg.Type, "error", err)
				} else {
					slog.Info("[CoBrowse] WebRTC signaling forwarded to user", "type", msg.Type, "session", hub.sessionID)
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

// forwardToViewers forwards a raw message to all connected viewers
func (hub *SessionHub) forwardToViewers(message []byte) {
	hub.mu.RLock()
	defer hub.mu.RUnlock()

	for viewerConn := range hub.viewerConns {
		if err := viewerConn.WriteMessage(websocket.TextMessage, message); err != nil {
			slog.Error("[CoBrowse] Failed to forward to viewer", "error", err)
			delete(hub.viewerConns, viewerConn)
		}
	}
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
