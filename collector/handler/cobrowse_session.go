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

		slog.Info("[CoBrowse] User message received", "type", msg.Type, "session", hub.sessionID, "length", len(message))

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
	// Parse events outside of lock
	var events []model.RRWebEvent
	if err := json.Unmarshal(msg.Data, &events); err != nil {
		var event model.RRWebEvent
		if err := json.Unmarshal(msg.Data, &event); err != nil {
			slog.Error("[CoBrowse] Failed to parse rrweb event", "error", err)
			return
		}
		events = []model.RRWebEvent{event}
	}

	// Store events in memory and get broadcast data
	hub.mu.Lock()
	if hub.closed {
		hub.mu.Unlock()
		return
	}

	for _, event := range events {
		hub.eventCount++
		fullEventJSON, _ := json.Marshal(event)
		hub.events = append(hub.events, storage.RecordingEventData{
			SessionID: hub.sessionID,
			Seq:       hub.eventCount,
			Timestamp: event.Timestamp,
			EventData: string(fullEventJSON),
			CreatedAt: time.Now().UnixMilli(),
		})
	}

	if len(hub.events) > hub.maxEvents {
		removed := len(hub.events) - hub.maxEvents
		hub.events = hub.events[removed:]
	}

	// Prepare broadcast data while holding lock
	broadcastMsg := map[string]interface{}{"type": "rrweb-event", "data": msg.Data}
	data, _ := json.Marshal(broadcastMsg)
	viewerConns := make(map[*websocket.Conn]bool, len(hub.viewerConns))
	for conn := range hub.viewerConns {
		viewerConns[conn] = true
	}
	hub.mu.Unlock()

	// DB writes and broadcast outside of lock
	for _, event := range events {
		fullEventJSON, _ := json.Marshal(event)
		if err := db.AddRecordingEvent(hub.sessionID, hub.eventCount, event.Timestamp, fullEventJSON); err != nil {
			slog.Error("[CoBrowse] Failed to save event", "error", err)
		}
	}

	for viewerConn := range viewerConns {
		if err := viewerConn.WriteMessage(websocket.TextMessage, data); err != nil {
			slog.Error("[CoBrowse] Failed to send to viewer", "error", err)
			hub.mu.Lock()
			delete(hub.viewerConns, viewerConn)
			hub.mu.Unlock()
		}
	}
}

// handleFullSnapshot processes a full snapshot from the user
func (hub *SessionHub) handleFullSnapshot(msg *model.CoBrowseMessage, db CoBrowseDB) {
	hub.mu.Lock()
	if hub.closed {
		hub.mu.Unlock()
		return
	}

	recEvent := storage.RecordingEventData{
		SessionID: hub.sessionID,
		Seq:       0,
		Timestamp: time.Now().UnixMilli(),
		EventData: string(msg.Data),
		CreatedAt: time.Now().UnixMilli(),
	}

	if len(hub.events) == 0 || hub.events[0].Seq != 0 {
		hub.events = append([]storage.RecordingEventData{recEvent}, hub.events...)
	} else {
		hub.events[0] = recEvent
	}

	// Prepare broadcast data
	broadcastMsg := map[string]interface{}{"type": "rrweb-full-snapshot", "data": msg.Data}
	data, _ := json.Marshal(broadcastMsg)
	viewerConns := make(map[*websocket.Conn]bool, len(hub.viewerConns))
	for conn := range hub.viewerConns { viewerConns[conn] = true }
	hub.mu.Unlock()

	// DB write outside lock
	if err := db.AddRecordingEvent(hub.sessionID, 0, recEvent.Timestamp, msg.Data); err != nil {
		slog.Error("[CoBrowse] Failed to save full snapshot", "error", err)
	}

	for viewerConn := range viewerConns {
		if err := viewerConn.WriteMessage(websocket.TextMessage, data); err != nil {
			slog.Error("[CoBrowse] Failed to send snapshot", "error", err)
			hub.mu.Lock()
			delete(hub.viewerConns, viewerConn)
			hub.mu.Unlock()
		}
	}
}

// forwardToViewers forwards a raw message to all connected viewers
func (hub *SessionHub) forwardToViewers(message []byte) {
	hub.mu.RLock()
	defer hub.mu.RUnlock()

	var msg model.CoBrowseMessage
	json.Unmarshal(message, &msg)

	slog.Debug("[CoBrowse] Forwarding", "type", msg.Type, "viewers", len(hub.viewerConns), "session", hub.sessionID)

	for viewerConn := range hub.viewerConns {
		if err := viewerConn.WriteMessage(websocket.TextMessage, message); err != nil {
			slog.Error("[CoBrowse] Forward failed", "error", err)
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

	// Notify viewers that session is ending
	sessionRemovedMsg, _ := json.Marshal(map[string]string{"type": "session-removed"})
	for viewerConn := range hub.viewerConns {
		viewerConn.WriteMessage(websocket.TextMessage, sessionRemovedMsg)
	}

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
