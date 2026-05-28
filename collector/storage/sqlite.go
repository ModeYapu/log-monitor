package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

// DB wraps the SQLite database connection
type DB struct {
	conn   *sql.DB
	path   string
	mu     sync.RWMutex
	closed bool
	stopCh chan struct{} // Channel to signal goroutines to stop
}

// Config holds database configuration
type Config struct {
	Path          string
	RetentionDays int
}

// NewDB creates a new database connection and initializes the schema
func NewDB(cfg Config) (*DB, error) {
	// Ensure directory exists
	dir := filepath.Dir(cfg.Path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	// Open database connection
	conn, err := sql.Open("sqlite", cfg.Path+"?_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Set connection pool settings
	conn.SetMaxOpenConns(1) // SQLite writes need single connection
	conn.SetMaxIdleConns(1)

	db := &DB{
		conn:   conn,
		path:   cfg.Path,
		stopCh: make(chan struct{}),
	}

	// Initialize schema
	if err := db.initSchema(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	// Start retention cleanup goroutine
	go db.retentionCleanup(cfg.RetentionDays)

	return db, nil
}

// initSchema creates the database tables and indexes if they don't exist
func (db *DB) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS events (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		app_id TEXT NOT NULL,
		release TEXT DEFAULT '',
		type TEXT NOT NULL,
		level TEXT NOT NULL,
		message TEXT NOT NULL,
		stack TEXT DEFAULT '',
		url TEXT DEFAULT '',
		line INTEGER DEFAULT 0,
		col INTEGER DEFAULT 0,
		tags TEXT DEFAULT '{}',
		extra TEXT DEFAULT '{}',
		ua TEXT DEFAULT '',
		screen TEXT DEFAULT '',
		viewport TEXT DEFAULT '',
		performance TEXT DEFAULT '{}',
		ip TEXT DEFAULT '',
		created_at INTEGER NOT NULL
	);

	CREATE INDEX IF NOT EXISTS idx_events_app_created ON events(app_id, created_at);
	CREATE INDEX IF NOT EXISTS idx_events_type ON events(app_id, type, created_at);
	CREATE INDEX IF NOT EXISTS idx_events_level ON events(app_id, level, created_at);
	CREATE INDEX IF NOT EXISTS idx_events_appid ON events(app_id);
	CREATE INDEX IF NOT EXISTS idx_events_level_only ON events(level);
	CREATE INDEX IF NOT EXISTS idx_events_timestamp ON events(created_at);

	CREATE TABLE IF NOT EXISTS alert_rules (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		app_id TEXT NOT NULL,
		name TEXT NOT NULL,
		condition_type TEXT NOT NULL,
		condition_config TEXT NOT NULL,
		notify_type TEXT NOT NULL,
		notify_config TEXT NOT NULL,
		enabled INTEGER DEFAULT 1,
		last_triggered_at INTEGER DEFAULT 0,
		cooldown_minutes INTEGER DEFAULT 30,
		created_at INTEGER NOT NULL
	);

	CREATE TABLE IF NOT EXISTS alert_logs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		rule_id INTEGER NOT NULL,
		app_id TEXT NOT NULL,
		message TEXT NOT NULL,
		created_at INTEGER NOT NULL
	);
	`

	_, err := db.conn.Exec(schema)
	return err
}

// escapeLike escapes special characters in LIKE queries to prevent SQL injection
func escapeLike(s string) string {
	// Escape backslash first, then % and _
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "%", "\\%")
	s = strings.ReplaceAll(s, "_", "\\_")
	return s
}

// InsertEvents batch inserts events into the database
func (db *DB) InsertEvents(events []EventRecord) error {
	if len(events) == 0 {
		return nil
	}

	db.mu.Lock()
	defer db.mu.Unlock()

	if db.closed {
		return fmt.Errorf("database is closed")
	}

	tx, err := db.conn.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO events (
			app_id, release, type, level, message, stack, url, line, col,
			tags, extra, ua, screen, viewport, performance, ip, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}

	defer stmt.Close()

	for _, e := range events {
		_, err := stmt.Exec(
			e.AppID, e.Release, e.Type, e.Level, e.Message, e.Stack,
			e.URL, e.Line, e.Col, e.Tags, e.Extra, e.UA, e.Screen,
			e.Viewport, e.Performance, e.IP, e.CreatedAt,
		)
		if err != nil {
			return fmt.Errorf("failed to insert event: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// QueryEvents retrieves events with pagination and filters
func (db *DB) QueryEvents(query QueryParams) (*QueryResult, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	if db.closed {
		return nil, fmt.Errorf("database is closed")
	}

	// Build query conditions
	whereClause := "WHERE app_id = ?"
	args := []interface{}{query.AppID}

	if query.Type != "" {
		whereClause += " AND type = ?"
		args = append(args, query.Type)
	}
	if query.Level != "" {
		whereClause += " AND level = ?"
		args = append(args, query.Level)
	}
	if query.StartTime > 0 {
		whereClause += " AND created_at >= ?"
		args = append(args, query.StartTime)
	}
	if query.EndTime > 0 {
		whereClause += " AND created_at <= ?"
		args = append(args, query.EndTime)
	}
	if query.Keyword != "" {
		whereClause += " AND message LIKE ? ESCAPE '\\'"
		args = append(args, "%"+escapeLike(query.Keyword)+"%")
	}

	// Count total
	countQuery := "SELECT COUNT(*) FROM events " + whereClause
	var total int64
	err := db.conn.QueryRow(countQuery, args...).Scan(&total)
	if err != nil {
		return nil, fmt.Errorf("failed to count events: %w", err)
	}

	// Query with pagination
	offset := (query.Page - 1) * query.PageSize
	dataQuery := `
		SELECT id, app_id, release, type, level, message, stack, url, line, col,
		       tags, extra, ua, screen, viewport, performance, ip, created_at
		FROM events ` + whereClause + `
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`
	args = append(args, query.PageSize, offset)

	rows, err := db.conn.Query(dataQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query events: %w", err)
	}
	defer rows.Close()

	var events []EventRecord
	for rows.Next() {
		var e EventRecord
		var id int64
		err := rows.Scan(
			&id, &e.AppID, &e.Release, &e.Type, &e.Level, &e.Message, &e.Stack,
			&e.URL, &e.Line, &e.Col, &e.Tags, &e.Extra, &e.UA, &e.Screen,
			&e.Viewport, &e.Performance, &e.IP, &e.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan event: %w", err)
		}
		events = append(events, e)
	}

	return &QueryResult{
		Total: total,
		Page:  query.Page,
		Size:  query.PageSize,
		Data:  events,
	}, nil
}

// GetApps returns list of all apps with basic stats
func (db *DB) GetApps() ([]AppStats, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	if db.closed {
		return nil, fmt.Errorf("database is closed")
	}

	query := `
		SELECT
			app_id,
			MAX(release) as release,
			MIN(created_at) as first_seen,
			MAX(created_at) as last_seen,
			SUM(CASE WHEN level = 'error' THEN 1 ELSE 0 END) as error_count,
			COUNT(*) as total_events
		FROM events
		GROUP BY app_id
		ORDER BY last_seen DESC
	`

	rows, err := db.conn.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query apps: %w", err)
	}
	defer rows.Close()

	var apps []AppStats
	for rows.Next() {
		var app AppStats
		err := rows.Scan(&app.AppID, &app.Release, &app.FirstSeen, &app.LastSeen, &app.ErrorCount, &app.TotalEvents)
		if err != nil {
			return nil, fmt.Errorf("failed to scan app: %w", err)
		}
		apps = append(apps, app)
	}

	return apps, nil
}

// GetStats returns statistics for an app
func (db *DB) GetStats(appID string) (*Stats, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	if db.closed {
		return nil, fmt.Errorf("database is closed")
	}

	stats := &Stats{}

	// Total events by level
	levelQuery := `
		SELECT
			level,
			COUNT(*) as count
		FROM events
		WHERE app_id = ?
		GROUP BY level
	`
	rows, err := db.conn.Query(levelQuery, appID)
	if err != nil {
		return nil, fmt.Errorf("failed to query level stats: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var level string
		var count int64
		if err := rows.Scan(&level, &count); err != nil {
			return nil, fmt.Errorf("failed to scan level stat: %w", err)
		}
		switch level {
		case "error":
			stats.ErrorCount = count
		case "warn":
			stats.WarnCount = count
		case "info":
			stats.InfoCount = count
		}
		stats.TotalEvents += count
	}

	// Top errors
	topErrorsQuery := `
		SELECT
			message,
			COUNT(*) as count,
			MAX(created_at) as last_seen
		FROM events
		WHERE app_id = ? AND level = 'error'
		GROUP BY message
		ORDER BY count DESC
		LIMIT 10
	`
	rows, err = db.conn.Query(topErrorsQuery, appID)
	if err != nil {
		return nil, fmt.Errorf("failed to query top errors: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var stat ErrorStat
		if err := rows.Scan(&stat.Message, &stat.Count, &stat.LastSeen); err != nil {
			return nil, fmt.Errorf("failed to scan top error: %w", err)
		}
		stats.TopErrors = append(stats.TopErrors, stat)
	}

	return stats, nil
}

// Close closes the database connection
func (db *DB) Close() error {
	db.mu.Lock()
	defer db.mu.Unlock()

	if db.closed {
		return nil
	}

	db.closed = true
	close(db.stopCh) // Signal goroutines to stop
	return db.conn.Close()
}

// retentionCleanup periodically deletes old events
func (db *DB) retentionCleanup(retentionDays int) {
	if retentionDays <= 0 {
		return
	}

	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	// Run once on startup
	db.cleanupOldData(retentionDays)

	for {
		select {
		case <-ticker.C:
			db.cleanupOldData(retentionDays)
		case <-db.stopCh:
			return
		}
	}
}

// cleanupOldData deletes events older than retention days
func (db *DB) cleanupOldData(retentionDays int) {
	cutoff := time.Now().AddDate(0, 0, -retentionDays).UnixMilli()

	db.mu.Lock()
	defer db.mu.Unlock()

	if db.closed {
		return
	}

	_, err := db.conn.Exec("DELETE FROM events WHERE created_at < ?", cutoff)
	if err != nil {
		fmt.Printf("Failed to cleanup old data: %v\n", err)
	}
}

// EventRecord represents a database event record
type EventRecord struct {
	AppID       string
	Release     string
	Type        string
	Level       string
	Message     string
	Stack       string
	URL         string
	Line        int
	Col         int
	Tags        string
	Extra       string
	UA          string
	Screen      string
	Viewport    string
	Performance string
	IP          string
	CreatedAt   int64
}

// QueryParams represents query parameters
type QueryParams struct {
	AppID     string
	Type      string
	Level     string
	StartTime int64
	EndTime   int64
	Keyword   string
	Page      int
	PageSize  int
}

// QueryResult represents query results
type QueryResult struct {
	Total int64
	Page  int
	Size  int
	Data  []EventRecord
}

// AppStats represents application statistics
type AppStats struct {
	AppID       string
	Release     string
	FirstSeen   int64
	LastSeen    int64
	ErrorCount  int64
	TotalEvents int64
}

// Stats represents application statistics
type Stats struct {
	TotalEvents int64
	ErrorCount  int64
	WarnCount   int64
	InfoCount   int64
	TopErrors   []ErrorStat
}

// ErrorStat represents error statistics
type ErrorStat struct {
	Message  string
	Count    int64
	LastSeen int64
}

// MarshalJSON converts EventRecord to JSON
func (e EventRecord) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"id":          0, // placeholder
		"appId":       e.AppID,
		"release":     e.Release,
		"type":        e.Type,
		"level":       e.Level,
		"message":     e.Message,
		"stack":       e.Stack,
		"url":         e.URL,
		"line":        e.Line,
		"col":         e.Col,
		"tags":        parseJSON(e.Tags),
		"extra":       parseJSON(e.Extra),
		"ua":          e.UA,
		"screen":      e.Screen,
		"viewport":    e.Viewport,
		"performance": parseJSON(e.Performance),
		"ip":          e.IP,
		"timestamp":   e.CreatedAt,
	})
}

func parseJSON(s string) map[string]interface{} {
	if s == "" || s == "{}" {
		return make(map[string]interface{})
	}
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(s), &result); err != nil {
		return make(map[string]interface{})
	}
	return result
}

// AlertRule represents an alert rule
type AlertRule struct {
	ID               int64
	AppID            string
	Name             string
	ConditionType    string
	ConditionConfig  string
	NotifyType       string
	NotifyConfig     string
	Enabled          int
	LastTriggeredAt  int64
	CooldownMinutes  int
	CreatedAt        int64
}

// AlertLog represents an alert log entry
type AlertLog struct {
	ID        int64
	RuleID    int64
	AppID     string
	Message   string
	CreatedAt int64
}

// CreateAlertRule creates a new alert rule
func (db *DB) CreateAlertRule(rule AlertRule) (int64, error) {
	db.mu.Lock()
	defer db.mu.Unlock()

	if db.closed {
		return 0, fmt.Errorf("database is closed")
	}

	result, err := db.conn.Exec(`
		INSERT INTO alert_rules (app_id, name, condition_type, condition_config, notify_type, notify_config, enabled, cooldown_minutes, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, rule.AppID, rule.Name, rule.ConditionType, rule.ConditionConfig, rule.NotifyType, rule.NotifyConfig, rule.Enabled, rule.CooldownMinutes, rule.CreatedAt)

	if err != nil {
		return 0, fmt.Errorf("failed to create alert rule: %w", err)
	}

	return result.LastInsertId()
}

// GetAlertRules retrieves alert rules for an app
func (db *DB) GetAlertRules(appID string) ([]AlertRule, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	if db.closed {
		return nil, fmt.Errorf("database is closed")
	}

	rows, err := db.conn.Query(`
		SELECT id, app_id, name, condition_type, condition_config, notify_type, notify_config, enabled, last_triggered_at, cooldown_minutes, created_at
		FROM alert_rules
		WHERE app_id = ?
		ORDER BY created_at DESC
	`, appID)

	if err != nil {
		return nil, fmt.Errorf("failed to get alert rules: %w", err)
	}
	defer rows.Close()

	var rules []AlertRule
	for rows.Next() {
		var rule AlertRule
		err := rows.Scan(
			&rule.ID, &rule.AppID, &rule.Name, &rule.ConditionType, &rule.ConditionConfig,
			&rule.NotifyType, &rule.NotifyConfig, &rule.Enabled, &rule.LastTriggeredAt,
			&rule.CooldownMinutes, &rule.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan alert rule: %w", err)
		}
		rules = append(rules, rule)
	}

	return rules, nil
}

// GetAllAlertRules retrieves all enabled alert rules
func (db *DB) GetAllAlertRules() ([]AlertRule, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	if db.closed {
		return nil, fmt.Errorf("database is closed")
	}

	rows, err := db.conn.Query(`
		SELECT id, app_id, name, condition_type, condition_config, notify_type, notify_config, enabled, last_triggered_at, cooldown_minutes, created_at
		FROM alert_rules
		WHERE enabled = 1
		ORDER BY created_at DESC
	`)

	if err != nil {
		return nil, fmt.Errorf("failed to get all alert rules: %w", err)
	}
	defer rows.Close()

	var rules []AlertRule
	for rows.Next() {
		var rule AlertRule
		err := rows.Scan(
			&rule.ID, &rule.AppID, &rule.Name, &rule.ConditionType, &rule.ConditionConfig,
			&rule.NotifyType, &rule.NotifyConfig, &rule.Enabled, &rule.LastTriggeredAt,
			&rule.CooldownMinutes, &rule.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan alert rule: %w", err)
		}
		rules = append(rules, rule)
	}

	return rules, nil
}

// UpdateAlertRuleLastTriggered updates the last triggered timestamp
func (db *DB) UpdateAlertRuleLastTriggered(id int64, timestamp int64) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	if db.closed {
		return fmt.Errorf("database is closed")
	}

	_, err := db.conn.Exec(`
		UPDATE alert_rules SET last_triggered_at = ? WHERE id = ?
	`, timestamp, id)

	if err != nil {
		return fmt.Errorf("failed to update alert rule: %w", err)
	}

	return nil
}

// DeleteAlertRule deletes an alert rule
func (db *DB) DeleteAlertRule(id int64) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	if db.closed {
		return fmt.Errorf("database is closed")
	}

	_, err := db.conn.Exec("DELETE FROM alert_rules WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("failed to delete alert rule: %w", err)
	}

	return nil
}

// CreateAlertLog creates an alert log entry
func (db *DB) CreateAlertLog(log AlertLog) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	if db.closed {
		return fmt.Errorf("database is closed")
	}

	_, err := db.conn.Exec(`
		INSERT INTO alert_logs (rule_id, app_id, message, created_at)
		VALUES (?, ?, ?, ?)
	`, log.RuleID, log.AppID, log.Message, log.CreatedAt)

	if err != nil {
		return fmt.Errorf("failed to create alert log: %w", err)
	}

	return nil
}

// GetAlertLogs retrieves alert logs for an app
func (db *DB) GetAlertLogs(appID string, limit int) ([]AlertLog, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	if db.closed {
		return nil, fmt.Errorf("database is closed")
	}

	rows, err := db.conn.Query(`
		SELECT id, rule_id, app_id, message, created_at
		FROM alert_logs
		WHERE app_id = ?
		ORDER BY created_at DESC
		LIMIT ?
	`, appID, limit)

	if err != nil {
		return nil, fmt.Errorf("failed to get alert logs: %w", err)
	}
	defer rows.Close()

	var logs []AlertLog
	for rows.Next() {
		var log AlertLog
		err := rows.Scan(&log.ID, &log.RuleID, &log.AppID, &log.Message, &log.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan alert log: %w", err)
		}
		logs = append(logs, log)
	}

	return logs, nil
}

// Recording related types and methods

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

// CreateRecording creates a new recording session
func (db *DB) CreateRecording(recording RecordingInfo) (int64, error) {
	db.mu.Lock()
	defer db.mu.Unlock()

	if db.closed {
		return 0, fmt.Errorf("database is closed")
	}

	result, err := db.conn.Exec(`
		INSERT INTO recordings (session_id, app_id, start_time, end_time, duration_ms, event_count, full_snapshot, url, ua, status, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
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
	db.mu.RLock()
	defer db.mu.RUnlock()

	if db.closed {
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
	db.mu.RLock()
	defer db.mu.RUnlock()

	if db.closed {
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
	db.mu.Lock()
	defer db.mu.Unlock()

	if db.closed {
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
	db.mu.RLock()
	defer db.mu.RUnlock()

	if db.closed {
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
	db.mu.Lock()
	defer db.mu.Unlock()

	if db.closed {
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
	db.mu.Lock()
	defer db.mu.Unlock()

	if db.closed {
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

// EnsureCobrowseTables creates the cobrowsing tables if they don't exist
// This is called separately to support adding new tables to existing databases
func (db *DB) EnsureCobrowseTables() error {
	db.mu.Lock()
	defer db.mu.Unlock()

	if db.closed {
		return fmt.Errorf("database is closed")
	}

	schema := `
	CREATE TABLE IF NOT EXISTS recordings (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		session_id TEXT NOT NULL UNIQUE,
		app_id TEXT NOT NULL,
		start_time INTEGER NOT NULL,
		end_time INTEGER DEFAULT 0,
		duration_ms INTEGER DEFAULT 0,
		event_count INTEGER DEFAULT 0,
		full_snapshot TEXT DEFAULT '',
		url TEXT DEFAULT '',
		ua TEXT DEFAULT '',
		status TEXT DEFAULT 'recording',
		created_at INTEGER NOT NULL
	);

	CREATE TABLE IF NOT EXISTS recording_events (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		session_id TEXT NOT NULL,
		seq INTEGER NOT NULL,
		timestamp INTEGER NOT NULL,
		event_data TEXT NOT NULL,
		created_at INTEGER NOT NULL
	);

	CREATE INDEX IF NOT EXISTS idx_recording_events_session_seq ON recording_events(session_id, seq);
	CREATE INDEX IF NOT EXISTS idx_recording_events_timestamp ON recording_events(timestamp);
	CREATE INDEX IF NOT EXISTS idx_recordings_appid ON recordings(app_id);
	CREATE INDEX IF NOT EXISTS idx_recordings_status ON recordings(status);
	CREATE INDEX IF NOT EXISTS idx_recordings_start_time ON recordings(start_time);
	CREATE INDEX IF NOT EXISTS idx_recording_events_session ON recording_events(session_id, seq);
	`

	_, err := db.conn.Exec(schema)
	return err
}

// GetRecordingStats returns statistics for a recording session
func (db *DB) GetRecordingStats(sessionID string) (interface{}, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	if db.closed {
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
		SessionID   string            `json:"sessionId"`
		TotalEvents int64             `json:"totalEvents"`
		TotalSize   int64             `json:"totalSize"`
		EventTypes  map[string]int64  `json:"eventTypes"`
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
