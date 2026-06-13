package storage

import (
	"encoding/json"
	"fmt"
	"time"
)

// RecordingInfo represents a recording session
type RecordingInfo struct {
	ID           int64  `json:"id"`
	SessionID    string `json:"sessionId"`
	AppID        string `json:"appId"`
	StartTime    int64  `json:"startTime"`
	EndTime      int64  `json:"endTime"`
	DurationMs   int64  `json:"durationMs"`
	EventCount   int    `json:"eventCount"`
	FullSnapshot string `json:"fullSnapshot"`
	URL          string `json:"url"`
	UA           string `json:"ua"`
	Status       string `json:"status"`
	CreatedAt    int64  `json:"createdAt"`
}

// RecordingEventData represents a single recording event
type RecordingEventData struct {
	ID        int64  `json:"id"`
	SessionID string `json:"sessionId"`
	Seq       int    `json:"seq"`
	Timestamp int64  `json:"timestamp"`
	EventData string `json:"eventData"`
	CreatedAt int64  `json:"createdAt"`
}

// TimelineEvent represents a significant event in the recording timeline
type TimelineEvent struct {
	Seq       int    `json:"seq"`
	Timestamp int64  `json:"timestamp"`
	Offset    int64  `json:"offsetMs"` // ms from session start
	Type      string `json:"type"`     // e.g. "click", "scroll", "input", "navigation", "error", "custom"
	Label     string `json:"label"`    // human-readable description
	Data      string `json:"data"`     // raw event data JSON
}

// CreateRecording creates a new recording session
func (db *DB) CreateRecording(recording RecordingInfo) (int64, error) {

	if db.closed.Load() {
		return 0, fmt.Errorf("database is closed")
	}

	result, err := db.conn.Exec(`
		INSERT INTO recordings (session_id, app_id, start_time, end_time, duration_ms, event_count, full_snapshot, url, ua, status, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(session_id) DO UPDATE SET
			event_count = excluded.event_count,
			duration_ms = excluded.duration_ms,
			end_time = excluded.end_time,
			status = excluded.status
	`, recording.SessionID, recording.AppID, recording.StartTime, recording.EndTime,
		recording.DurationMs, recording.EventCount, recording.FullSnapshot,
		recording.URL, recording.UA, recording.Status, recording.CreatedAt)

	if err != nil {
		return 0, fmt.Errorf("failed to create recording: %w", err)
	}

	return result.LastInsertId()
}

// GetRecording retrieves a recording by session ID
func (db *DB) GetRecording(sessionID string) (*RecordingInfo, error) {

	if db.closed.Load() {
		return nil, fmt.Errorf("database is closed")
	}

	var recording RecordingInfo
	err := db.conn.QueryRow(`
		SELECT id, session_id, app_id, start_time, end_time, duration_ms, event_count, full_snapshot, url, ua, status, created_at
		FROM recordings
		WHERE session_id = ?
	`, sessionID).Scan(
		&recording.ID, &recording.SessionID, &recording.AppID, &recording.StartTime,
		&recording.EndTime, &recording.DurationMs, &recording.EventCount, &recording.FullSnapshot,
		&recording.URL, &recording.UA, &recording.Status, &recording.CreatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get recording: %w", err)
	}

	return &recording, nil
}

// GetRecordings retrieves recordings with pagination and filters
func (db *DB) GetRecordings(limit, offset int, filters map[string]interface{}) ([]RecordingInfo, error) {

	if db.closed.Load() {
		return nil, fmt.Errorf("database is closed")
	}

	// Build WHERE clause for filters
	whereClause := "WHERE 1=1"
	args := []interface{}{}

	if appID, ok := filters["app_id"].(string); ok && appID != "" {
		whereClause += " AND app_id = ?"
		args = append(args, appID)
	}

	if status, ok := filters["status"].(string); ok && status != "" {
		whereClause += " AND status = ?"
		args = append(args, status)
	}

	if startFrom, ok := filters["start_from"].(int64); ok && startFrom > 0 {
		whereClause += " AND start_time >= ?"
		args = append(args, startFrom)
	}

	if startTo, ok := filters["start_to"].(int64); ok && startTo > 0 {
		whereClause += " AND start_time <= ?"
		args = append(args, startTo)
	}

	if minDuration, ok := filters["min_duration"].(int64); ok && minDuration > 0 {
		whereClause += " AND duration_ms >= ?"
		args = append(args, minDuration)
	}

	if maxDuration, ok := filters["max_duration"].(int64); ok && maxDuration > 0 {
		whereClause += " AND duration_ms <= ?"
		args = append(args, maxDuration)
	}

	if search, ok := filters["search"].(string); ok && search != "" {
		whereClause += " AND (session_id LIKE ? OR url LIKE ?)"
		searchPattern := "%" + search + "%"
		args = append(args, searchPattern, searchPattern)
	}

	query := `
		SELECT id, session_id, app_id, start_time, end_time, duration_ms, event_count, full_snapshot, url, ua, status, created_at
		FROM recordings ` + whereClause + `
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?`

	args = append(args, limit, offset)

	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get recordings: %w", err)
	}
	defer rows.Close()

	var recordings []RecordingInfo
	for rows.Next() {
		var r RecordingInfo
		err := rows.Scan(
			&r.ID, &r.SessionID, &r.AppID, &r.StartTime,
			&r.EndTime, &r.DurationMs, &r.EventCount, &r.FullSnapshot,
			&r.URL, &r.UA, &r.Status, &r.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan recording: %w", err)
		}
		recordings = append(recordings, r)
	}

	return recordings, nil
}

// AddRecordingEvent adds an event to a recording session
func (db *DB) AddRecordingEvent(sessionID string, seq int, timestamp int64, eventData []byte) error {

	if db.closed.Load() {
		return fmt.Errorf("database is closed")
	}

	_, err := db.conn.Exec(`
		INSERT INTO recording_events (session_id, seq, timestamp, event_data, created_at)
		VALUES (?, ?, ?, ?, ?)
	`, sessionID, seq, timestamp, string(eventData), time.Now().UnixMilli())

	if err != nil {
		return fmt.Errorf("failed to add recording event: %w", err)
	}

	return nil
}

// GetRecordingEvents retrieves events for a recording session
func (db *DB) GetRecordingEvents(sessionID string, limit, offset int) ([]RecordingEventData, error) {

	if db.closed.Load() {
		return nil, fmt.Errorf("database is closed")
	}

	rows, err := db.conn.Query(`
		SELECT id, session_id, seq, timestamp, event_data, created_at
		FROM recording_events
		WHERE session_id = ?
		ORDER BY seq ASC
		LIMIT ? OFFSET ?
	`, sessionID, limit, offset)

	if err != nil {
		return nil, fmt.Errorf("failed to get recording events: %w", err)
	}
	defer rows.Close()

	var events []RecordingEventData
	for rows.Next() {
		var e RecordingEventData
		err := rows.Scan(&e.ID, &e.SessionID, &e.Seq, &e.Timestamp, &e.EventData, &e.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan recording event: %w", err)
		}
		events = append(events, e)
	}

	return events, nil
}

// DeleteRecording deletes a recording and its events
func (db *DB) DeleteRecording(sessionID string) error {

	if db.closed.Load() {
		return fmt.Errorf("database is closed")
	}

	// Delete events first
	_, err := db.conn.Exec(`DELETE FROM recording_events WHERE session_id = ?`, sessionID)
	if err != nil {
		return fmt.Errorf("failed to delete recording events: %w", err)
	}

	// Delete recording
	_, err = db.conn.Exec(`DELETE FROM recordings WHERE session_id = ?`, sessionID)
	if err != nil {
		return fmt.Errorf("failed to delete recording: %w", err)
	}

	return nil
}

// UpdateRecording updates a recording's status and metadata
func (db *DB) UpdateRecording(sessionID string, endTime int64, durationMs int64, eventCount int, status string) error {

	if db.closed.Load() {
		return fmt.Errorf("database is closed")
	}

	_, err := db.conn.Exec(`
		UPDATE recordings
		SET end_time = ?, duration_ms = ?, event_count = ?, status = ?
		WHERE session_id = ?
	`, endTime, durationMs, eventCount, status, sessionID)

	if err != nil {
		return fmt.Errorf("failed to update recording: %w", err)
	}

	return nil
}

// GetRecordingStats returns statistics for a recording session
func (db *DB) GetRecordingStats(sessionID string) (interface{}, error) {

	if db.closed.Load() {
		return nil, fmt.Errorf("database is closed")
	}

	// Get total event count and size
	var totalEvents int64
	var totalSize int64
	err := db.conn.QueryRow(`
		SELECT COUNT(*), COALESCE(SUM(LENGTH(event_data)), 0)
		FROM recording_events
		WHERE session_id = ?
	`, sessionID).Scan(&totalEvents, &totalSize)
	if err != nil {
		return nil, fmt.Errorf("failed to get recording stats: %w", err)
	}

	// Get event type distribution
	typeRows, err := db.conn.Query(`
		SELECT
			json_extract(event_data, '$.type') as event_type,
			COUNT(*) as count
		FROM recording_events
		WHERE session_id = ?
		GROUP BY event_type
	`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get event types: %w", err)
	}
	defer typeRows.Close()

	eventTypes := make(map[string]int64)
	for typeRows.Next() {
		var eventType string
		var count int64
		if err := typeRows.Scan(&eventType, &count); err != nil {
			return nil, fmt.Errorf("failed to scan event type: %w", err)
		}
		eventTypes[eventType] = count
	}

	// Get time range
	var startTime, endTime int64
	err = db.conn.QueryRow(`
		SELECT MIN(timestamp), MAX(timestamp)
		FROM recording_events
		WHERE session_id = ?
	`, sessionID).Scan(&startTime, &endTime)
	if err != nil {
		return nil, fmt.Errorf("failed to get time range: %w", err)
	}

	return &struct {
		SessionID   string           `json:"sessionId"`
		TotalEvents int64            `json:"totalEvents"`
		TotalSize   int64            `json:"totalSize"`
		EventTypes  map[string]int64 `json:"eventTypes"`
		TimeRange   struct {
			StartTime int64 `json:"startTime"`
			EndTime   int64 `json:"endTime"`
		} `json:"timeRange"`
	}{
		SessionID:   sessionID,
		TotalEvents: totalEvents,
		TotalSize:   totalSize,
		EventTypes:  eventTypes,
		TimeRange: struct {
			StartTime int64 `json:"startTime"`
			EndTime   int64 `json:"endTime"`
		}{
			StartTime: startTime,
			EndTime:   endTime,
		},
	}, nil
}

// GetRecordingTimeline extracts timeline events from recording events
func (db *DB) GetRecordingTimeline(sessionID string) ([]TimelineEvent, error) {
	if db.closed.Load() {
		return nil, fmt.Errorf("database is closed")
	}

	rows, err := db.conn.Query(`
		SELECT seq, timestamp, event_data
		FROM recording_events
		WHERE session_id = ?
		ORDER BY seq ASC
	`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get recording events: %w", err)
	}
	defer rows.Close()

	var timeline []TimelineEvent
	var firstTimestamp int64 = 0

	for rows.Next() {
		var seq int
		var timestamp int64
		var eventData string

		err := rows.Scan(&seq, &timestamp, &eventData)
		if err != nil {
			return nil, fmt.Errorf("failed to scan recording event: %w", err)
		}

		// Track first timestamp for offset calculation
		if firstTimestamp == 0 {
			firstTimestamp = timestamp
		}

		// Parse rrweb event data
		var rrwebEvent map[string]interface{}
		if err := json.Unmarshal([]byte(eventData), &rrwebEvent); err != nil {
			// If parsing fails, add as generic event
			timeline = append(timeline, TimelineEvent{
				Seq:       seq,
				Timestamp: timestamp,
				Offset:    timestamp - firstTimestamp,
				Type:      "custom",
				Label:     "Unknown Event",
				Data:      eventData,
			})
			continue
		}

		// Extract rrweb event type
		eventType, _ := rrwebEvent["type"].(float64)

		// Classify event and extract label
		timelineType := "custom"
		label := "Event"

		switch int(eventType) {
		case 2: // DomContentLoaded
			timelineType = "navigation"
			label = "DOM Content Loaded"
		case 4: // Meta
			timelineType = "meta"
			label = "Meta Update"
		case 5: // Load
			timelineType = "navigation"
			label = "Page Loaded"
		case 6: // FullSnapshot
			timelineType = "snapshot"
			label = "Full Snapshot"
		case 7: // IncrementalSnapshot
			// Look at data.source for more specific classification
			if data, ok := rrwebEvent["data"].(map[string]interface{}); ok {
				if source, ok := data["source"].(float64); ok {
					switch int(source) {
					case 0: // mutation
						timelineType = "mutation"
						label = "DOM Mutation"
					case 1: // mouseMove
						timelineType = "mousemove"
						label = "Mouse Move"
					case 2: // mouseInteraction
						// Check interaction type
						if typeVal, ok := data["type"].(float64); ok {
							switch int(typeVal) {
							case 0: // MouseUp (click)
								timelineType = "click"
								label = "Click"
							case 1: // MouseDown
								timelineType = "mousedown"
								label = "Mouse Down"
							case 2: // Click
								timelineType = "click"
								label = "Click"
							case 3: // ContextMenu
								timelineType = "contextmenu"
								label = "Context Menu"
							default:
								timelineType = "interaction"
								label = "Mouse Interaction"
							}
						}
					case 3: // scroll
						timelineType = "scroll"
						label = "Scroll"
					case 4: // viewportResize
						timelineType = "resize"
						label = "Viewport Resize"
					case 5: // input
						timelineType = "input"
						label = "Input"
						if inputType, ok := data["type"].(float64); ok {
							switch int(inputType) {
							case 0:
								label = "Text Input"
							case 1:
								label = "Option Change"
							}
						}
					case 6: // mediaInteraction
						timelineType = "media"
						label = "Media Interaction"
					case 7: // styleSheetRule
						timelineType = "style"
						label = "Style Sheet Rule"
					case 8: // canvasMutation
						timelineType = "canvas"
						label = "Canvas Mutation"
					case 9: // font
						timelineType = "font"
						label = "Font Loading"
					}
				}
			}
		}

		timeline = append(timeline, TimelineEvent{
			Seq:       seq,
			Timestamp: timestamp,
			Offset:    timestamp - firstTimestamp,
			Type:      timelineType,
			Label:     label,
			Data:      eventData,
		})
	}

	return timeline, nil
}
