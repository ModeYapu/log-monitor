package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
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
		env TEXT DEFAULT '',
		build_id TEXT DEFAULT '',
		user_id TEXT DEFAULT '',
		session_id TEXT DEFAULT '',
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
	CREATE INDEX IF NOT EXISTS idx_events_release ON events(app_id, release);
	CREATE INDEX IF NOT EXISTS idx_events_env ON events(app_id, env);
	CREATE INDEX IF NOT EXISTS idx_events_session_id ON events(session_id);
	CREATE INDEX IF NOT EXISTS idx_events_user_id ON events(user_id);

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
		silenced_until INTEGER DEFAULT 0,
		fingerprint TEXT DEFAULT '',
		message_template TEXT DEFAULT '',
		created_at INTEGER NOT NULL
	);

	CREATE TABLE IF NOT EXISTS alert_logs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		rule_id INTEGER NOT NULL,
		app_id TEXT NOT NULL,
		message TEXT NOT NULL,
		created_at INTEGER NOT NULL
	);

	CREATE TABLE IF NOT EXISTS system_meta (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL,
		updated_at INTEGER NOT NULL
	);

	CREATE INDEX IF NOT EXISTS idx_alert_logs_created ON alert_logs(created_at);
	`

	_, err := db.conn.Exec(schema)
	if err != nil {
		return err
	}

	// Migration: Add message_template column if it doesn't exist
	_, err = db.conn.Exec(`ALTER TABLE alert_rules ADD COLUMN message_template TEXT DEFAULT ''`)
	if err != nil && !strings.Contains(err.Error(), "duplicate column") {
		log.Printf("Migration notice: %v", err)
	}

	// Migration: Add new event fields for P0 release/session tracking
	migrations := []string{
		`ALTER TABLE events ADD COLUMN release TEXT DEFAULT ''`,
		`ALTER TABLE events ADD COLUMN env TEXT DEFAULT ''`,
		`ALTER TABLE events ADD COLUMN build_id TEXT DEFAULT ''`,
		`ALTER TABLE events ADD COLUMN user_id TEXT DEFAULT ''`,
		`ALTER TABLE events ADD COLUMN session_id TEXT DEFAULT ''`,
		`ALTER TABLE alert_rules ADD COLUMN silenced_until INTEGER DEFAULT 0`,
		`ALTER TABLE alert_rules ADD COLUMN fingerprint TEXT DEFAULT ''`,
		// Feature 1: Error fingerprinting
		`ALTER TABLE events ADD COLUMN fingerprint TEXT DEFAULT ''`,
		// Feature 2: Breadcrumbs
		`ALTER TABLE events ADD COLUMN breadcrumbs TEXT DEFAULT '[]'`,
	}
	for _, m := range migrations {
		_, mErr := db.conn.Exec(m)
		if mErr != nil && !strings.Contains(mErr.Error(), "duplicate column") {
			log.Printf("Migration notice: %v", mErr)
		}
	}

	// Migration: Create indexes for new columns
	indexMigrations := []string{
		`CREATE INDEX IF NOT EXISTS idx_events_release ON events(app_id, release)`,
		`CREATE INDEX IF NOT EXISTS idx_events_env ON events(app_id, env)`,
		`CREATE INDEX IF NOT EXISTS idx_events_session_id ON events(session_id)`,
		`CREATE INDEX IF NOT EXISTS idx_events_user_id ON events(user_id)`,
		// Feature 1: Error fingerprinting index
		`CREATE INDEX IF NOT EXISTS idx_events_fingerprint ON events(app_id, fingerprint)`,
	}
	for _, m := range indexMigrations {
		if _, mErr := db.conn.Exec(m); mErr != nil {
			log.Printf("Migration notice: %v", mErr)
		}
	}

	return nil
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
			app_id, release, env, build_id, user_id, session_id,
			type, level, message, stack, url, line, col,
			tags, extra, ua, screen, viewport, performance, ip, created_at,
			fingerprint, breadcrumbs
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}

	defer stmt.Close()

	for _, e := range events {
		_, err := stmt.Exec(
			e.AppID, e.Release, e.Env, e.BuildID, e.UserID, e.SessionID,
			e.Type, e.Level, e.Message, e.Stack,
			e.URL, e.Line, e.Col, e.Tags, e.Extra, e.UA, e.Screen,
			e.Viewport, e.Performance, e.IP, e.CreatedAt,
			e.Fingerprint, e.Breadcrumbs,
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
	if query.Release != "" {
		whereClause += " AND release = ?"
		args = append(args, query.Release)
	}
	if query.Env != "" {
		whereClause += " AND env = ?"
		args = append(args, query.Env)
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
		SELECT id, app_id, release, env, build_id, user_id, session_id,
		       type, level, message, stack, url, line, col,
		       tags, extra, ua, screen, viewport, performance, ip, created_at,
		       fingerprint, breadcrumbs
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
			&id, &e.AppID, &e.Release, &e.Env, &e.BuildID, &e.UserID, &e.SessionID,
			&e.Type, &e.Level, &e.Message, &e.Stack,
			&e.URL, &e.Line, &e.Col, &e.Tags, &e.Extra, &e.UA, &e.Screen,
			&e.Viewport, &e.Performance, &e.IP, &e.CreatedAt,
			&e.Fingerprint, &e.Breadcrumbs,
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

// Conn returns the underlying SQL connection for direct queries
func (db *DB) Conn() *sql.DB {
	return db.conn
}

// retentionCleanup periodically deletes old events (runs daily at midnight)
func (db *DB) retentionCleanup(retentionDays int) {
	if retentionDays <= 0 {
		return
	}

	// Run once on startup
	db.cleanupOldData(retentionDays)

	// Calculate time until next midnight
	nextMidnight := time.Now().AddDate(1, 0, 0) // Tomorrow
	nextMidnight = time.Date(nextMidnight.Year(), nextMidnight.Month(), nextMidnight.Day(), 0, 0, 0, 0, nextMidnight.Location())
	initialDelay := nextMidnight.Sub(time.Now())

	// Create a timer for the first midnight
	timer := time.NewTimer(initialDelay)
	defer timer.Stop()

	log.Printf("[cleanup] Scheduled daily cleanup at midnight (first run in %v)", initialDelay)

	for {
		select {
		case <-timer.C:
			// Run cleanup
			db.cleanupOldData(retentionDays)
			// Reset timer for next day
			timer.Reset(24 * time.Hour)
		case <-db.stopCh:
			timer.Stop()
			return
		}
	}
}

// CleanupResult represents the result of a cleanup operation
type CleanupResult struct {
	EventsDeleted           int64
	RecordingEventsDeleted  int64
	AlertLogsDeleted        int64
	LastCleanupTime         int64
}

// cleanupOldData deletes events older than retention days and cleans orphaned recording_events and alert_logs
func (db *DB) cleanupOldData(retentionDays int) CleanupResult {
	cutoff := time.Now().AddDate(0, 0, -retentionDays).UnixMilli()

	db.mu.Lock()
	defer db.mu.Unlock()

	if db.closed {
		return CleanupResult{}
	}

	result := CleanupResult{
		LastCleanupTime: time.Now().UnixMilli(),
	}

	// Delete old events
	rows, err := db.conn.Exec("DELETE FROM events WHERE created_at < ?", cutoff)
	if err != nil {
		fmt.Printf("[cleanup] Failed to delete old events: %v\n", err)
	} else if rowsAffected, _ := rows.RowsAffected(); rowsAffected > 0 {
		result.EventsDeleted = rowsAffected
		fmt.Printf("[cleanup] Deleted %d old events (older than %d days)\n", rowsAffected, retentionDays)
	}

	// Delete old recording_events
	rows, err = db.conn.Exec("DELETE FROM recording_events WHERE created_at < ?", cutoff)
	if err != nil {
		fmt.Printf("[cleanup] Failed to delete old recording_events: %v\n", err)
	} else if rowsAffected, _ := rows.RowsAffected(); rowsAffected > 0 {
		result.RecordingEventsDeleted = rowsAffected
		fmt.Printf("[cleanup] Deleted %d old recording_events (older than %d days)\n", rowsAffected, retentionDays)
	}

	// Delete old alert_logs
	rows, err = db.conn.Exec("DELETE FROM alert_logs WHERE created_at < ?", cutoff)
	if err != nil {
		fmt.Printf("[cleanup] Failed to delete old alert_logs: %v\n", err)
	} else if rowsAffected, _ := rows.RowsAffected(); rowsAffected > 0 {
		result.AlertLogsDeleted = rowsAffected
		fmt.Printf("[cleanup] Deleted %d old alert_logs (older than %d days)\n", rowsAffected, retentionDays)
	}

	// Clean orphaned recording_events (events without a corresponding recording)
	rows, err = db.conn.Exec(`
		DELETE FROM recording_events
		WHERE session_id NOT IN (SELECT session_id FROM recordings)
	`)
	if err != nil {
		fmt.Printf("[cleanup] Failed to delete orphaned recording_events: %v\n", err)
	} else if rowsAffected, _ := rows.RowsAffected(); rowsAffected > 0 {
		orphanDeleted := rowsAffected
		result.RecordingEventsDeleted += orphanDeleted
		fmt.Printf("[cleanup] Deleted %d orphaned recording_events\n", orphanDeleted)
	}

	// Update last cleanup time in system_meta
	_, err = db.conn.Exec(`
		INSERT OR REPLACE INTO system_meta (key, value, updated_at)
		VALUES ('last_cleanup_time', ?, ?)
	`, result.LastCleanupTime, result.LastCleanupTime)
	if err != nil {
		fmt.Printf("[cleanup] Failed to update last_cleanup_time: %v\n", err)
	}

	return result
}

// CleanupOldDataWithDays manually deletes old data with configurable retention days
func (db *DB) CleanupOldDataWithDays(days int) CleanupResult {
	if days <= 0 {
		days = 30
	}
	return db.cleanupOldData(days)
}

// GetLastCleanupTime returns the last cleanup time from system_meta
func (db *DB) GetLastCleanupTime() int64 {
	db.mu.RLock()
	defer db.mu.RUnlock()

	if db.closed {
		return 0
	}

	var lastCleanup int64
	err := db.conn.QueryRow("SELECT value FROM system_meta WHERE key = 'last_cleanup_time'").Scan(&lastCleanup)
	if err != nil {
		return 0
	}
	return lastCleanup
}

// SetLastCleanupTime sets the last cleanup time in system_meta
func (db *DB) SetLastCleanupTime(timestamp int64) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	if db.closed {
		return fmt.Errorf("database is closed")
	}

	_, err := db.conn.Exec(`
		INSERT OR REPLACE INTO system_meta (key, value, updated_at)
		VALUES ('last_cleanup_time', ?, ?)
	`, timestamp, time.Now().UnixMilli())

	return err
}

// EventRecord represents a database event record
type EventRecord struct {
	ID          int64
	AppID       string
	Release     string
	Env         string
	BuildID     string
	UserID      string
	SessionID   string
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
	Fingerprint string // Feature 1: Error fingerprint
	Breadcrumbs string // Feature 2: Breadcrumbs JSON
}

// QueryParams represents query parameters
type QueryParams struct {
	AppID     string
	Release   string
	Env       string
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
	AppID       string `json:"app_id"`
	Release     string `json:"release"`
	FirstSeen   int64  `json:"first_seen"`
	LastSeen    int64  `json:"last_seen"`
	ErrorCount  int64  `json:"error_count"`
	TotalEvents int64  `json:"total_events"`
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
		"env":         e.Env,
		"buildId":     e.BuildID,
		"userId":      e.UserID,
		"sessionId":   e.SessionID,
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
	ID              int64
	AppID           string
	Name            string
	ConditionType   string
	ConditionConfig string
	NotifyType      string
	NotifyConfig    string
	Enabled         int
	LastTriggeredAt int64
	CooldownMinutes int
	SilencedUntil   int64
	Fingerprint     string
	MessageTemplate string
	CreatedAt       int64
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
		SELECT id, app_id, name, condition_type, condition_config, notify_type, notify_config, enabled, last_triggered_at, cooldown_minutes, silenced_until, fingerprint, created_at
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
			&rule.CooldownMinutes, &rule.SilencedUntil, &rule.Fingerprint, &rule.CreatedAt,
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
		SELECT id, app_id, name, condition_type, condition_config, notify_type, notify_config, enabled, last_triggered_at, cooldown_minutes, silenced_until, fingerprint, created_at
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
			&rule.CooldownMinutes, &rule.SilencedUntil, &rule.Fingerprint, &rule.CreatedAt,
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

// SilenceAlertRule silences an alert rule until a specified time
func (db *DB) SilenceAlertRule(id int64, until int64) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	if db.closed {
		return fmt.Errorf("database is closed")
	}

	_, err := db.conn.Exec(`
		UPDATE alert_rules SET silenced_until = ? WHERE id = ?
	`, until, id)

	if err != nil {
		return fmt.Errorf("failed to silence alert rule: %w", err)
	}

	return nil
}

// UnsilenceAlertRule unsilences an alert rule
func (db *DB) UnsilenceAlertRule(id int64) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	if db.closed {
		return fmt.Errorf("database is closed")
	}

	_, err := db.conn.Exec("UPDATE alert_rules SET silenced_until = 0 WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("failed to unsilence alert rule: %w", err)
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

// GetSessionEvents retrieves events associated with a session
func (db *DB) GetSessionEvents(sessionID string, limit int) ([]EventRecord, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	if db.closed {
		return nil, fmt.Errorf("database is closed")
	}

	if limit <= 0 || limit > 1000 {
		limit = 100
	}

	rows, err := db.conn.Query(`
		SELECT id, app_id, release, env, build_id, user_id, session_id,
		       type, level, message, stack, url, line, col,
		       tags, extra, ua, screen, viewport, performance, ip, created_at
		FROM events
		WHERE session_id = ?
		ORDER BY created_at ASC
		LIMIT ?
	`, sessionID, limit)

	if err != nil {
		return nil, fmt.Errorf("failed to get session events: %w", err)
	}
	defer rows.Close()

	var events []EventRecord
	for rows.Next() {
		var e EventRecord
		err := rows.Scan(
			&e.ID, &e.AppID, &e.Release, &e.Env, &e.BuildID, &e.UserID, &e.SessionID,
			&e.Type, &e.Level, &e.Message, &e.Stack,
			&e.URL, &e.Line, &e.Col, &e.Tags, &e.Extra, &e.UA, &e.Screen,
			&e.Viewport, &e.Performance, &e.IP, &e.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan event: %w", err)
		}
		events = append(events, e)
	}

	return events, nil
}

// GetSessionErrorCount returns the count of errors for a session
func (db *DB) GetSessionErrorCount(sessionID string) (int64, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	if db.closed {
		return 0, fmt.Errorf("database is closed")
	}

	var count int64
	err := db.conn.QueryRow(`
		SELECT COUNT(*) FROM events
		WHERE session_id = ? AND level = 'error'
	`, sessionID).Scan(&count)

	if err != nil {
		return 0, fmt.Errorf("failed to get session error count: %w", err)
	}

	return count, nil
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

// TopListParams represents parameters for top lists queries
type TopListParams struct {
	AppID     string
	StartTime int64
	EndTime   int64
	Limit     int
	SortBy    string
}

// TopError represents an error in the top errors list
type TopError struct {
	Message       string `json:"message"`
	Count         int64  `json:"count"`
	FirstSeen     int64  `json:"firstSeen"`
	LastSeen      int64  `json:"lastSeen"`
	AffectedUsers int64  `json:"affectedUsers"`
	SampleStack   string `json:"sampleStack,omitempty"`
}

// TopPage represents a page in the top pages list
type TopPage struct {
	URL         string  `json:"url"`
	ErrorCount  int64   `json:"errorCount"`
	TotalEvents int64   `json:"totalEvents"`
	ErrorRate   float64 `json:"errorRate"`
	FirstSeen   int64   `json:"firstSeen"`
	LastSeen    int64   `json:"lastSeen"`
}

// TopRelease represents a release in the top releases list
type TopRelease struct {
	Release     string  `json:"release"`
	Env         string  `json:"env"`
	ErrorCount  int64   `json:"errorCount"`
	TotalEvents int64   `json:"totalEvents"`
	ErrorRate   float64 `json:"errorRate"`
	FirstSeen   int64   `json:"firstSeen"`
	LastSeen    int64   `json:"lastSeen"`
	NewErrors   int64   `json:"newErrors"`
	IsLatest    bool    `json:"isLatest"`
}

// TopBrowser represents a browser in the top browsers list
type TopBrowser struct {
	Browser     string  `json:"browser"`
	Version     string  `json:"version"`
	ErrorCount  int64   `json:"errorCount"`
	TotalEvents int64   `json:"totalEvents"`
	ErrorRate   float64 `json:"errorRate"`
}

func (db *DB) GetTopErrors(params TopListParams) ([]TopError, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()
	if db.closed {
		return nil, fmt.Errorf("database is closed")
	}
	limit := params.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	whereClause := "WHERE app_id = ? AND level = 'error'"
	args := []interface{}{params.AppID}
	if params.StartTime > 0 {
		whereClause += " AND created_at >= ?"
		args = append(args, params.StartTime)
	}
	if params.EndTime > 0 {
		whereClause += " AND created_at <= ?"
		args = append(args, params.EndTime)
	}
	orderBy := "count DESC"
	switch params.SortBy {
	case "recent":
		orderBy = "last_seen DESC"
	case "impact":
		orderBy = "affected_users DESC"
	case "regression":
		orderBy = "first_seen DESC"
	}
	query := fmt.Sprintf(`
		SELECT message, COUNT(*) as count, MIN(created_at) as first_seen,
		MAX(created_at) as last_seen, COUNT(DISTINCT user_id) as affected_users,
		SUBSTR(GROUP_CONCAT(stack), 1, 500) as sample_stack
		FROM events %s GROUP BY message ORDER BY %s LIMIT ?`, whereClause, orderBy)
	args = append(args, limit)
	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query top errors: %w", err)
	}
	defer rows.Close()
	var results []TopError
	for rows.Next() {
		var e TopError
		var sampleStack sql.NullString
		err := rows.Scan(&e.Message, &e.Count, &e.FirstSeen, &e.LastSeen, &e.AffectedUsers, &sampleStack)
		if err != nil {
			return nil, fmt.Errorf("failed to scan top error: %w", err)
		}
		if sampleStack.Valid {
			e.SampleStack = sampleStack.String
		}
		results = append(results, e)
	}
	return results, nil
}

func (db *DB) GetTopPages(params TopListParams) ([]TopPage, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()
	if db.closed {
		return nil, fmt.Errorf("database is closed")
	}
	limit := params.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	whereClause := "WHERE app_id = ? AND url != ''"
	args := []interface{}{params.AppID}
	if params.StartTime > 0 {
		whereClause += " AND created_at >= ?"
		args = append(args, params.StartTime)
	}
	if params.EndTime > 0 {
		whereClause += " AND created_at <= ?"
		args = append(args, params.EndTime)
	}
	orderBy := "error_count DESC"
	if params.SortBy == "recent" {
		orderBy = "last_seen DESC"
	}
	query := fmt.Sprintf(`
		SELECT url, SUM(CASE WHEN level = 'error' THEN 1 ELSE 0 END) as error_count,
		COUNT(*) as total_events, CAST(SUM(CASE WHEN level = 'error' THEN 1 ELSE 0 END) AS REAL) / COUNT(*) as error_rate,
		MIN(created_at) as first_seen, MAX(created_at) as last_seen
		FROM events %s GROUP BY url ORDER BY %s LIMIT ?`, whereClause, orderBy)
	args = append(args, limit)
	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query top pages: %w", err)
	}
	defer rows.Close()
	var results []TopPage
	for rows.Next() {
		var p TopPage
		err := rows.Scan(&p.URL, &p.ErrorCount, &p.TotalEvents, &p.ErrorRate, &p.FirstSeen, &p.LastSeen)
		if err != nil {
			return nil, fmt.Errorf("failed to scan top page: %w", err)
		}
		results = append(results, p)
	}
	return results, nil
}

func (db *DB) GetTopReleases(params TopListParams) ([]TopRelease, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()
	if db.closed {
		return nil, fmt.Errorf("database is closed")
	}
	limit := params.Limit
	if limit <= 0 || limit > 50 {
		limit = 10
	}
	whereClause := "WHERE app_id = ? AND release != ''"
	args := []interface{}{params.AppID}
	if params.StartTime > 0 {
		whereClause += " AND created_at >= ?"
		args = append(args, params.StartTime)
	}
	if params.EndTime > 0 {
		whereClause += " AND created_at <= ?"
		args = append(args, params.EndTime)
	}
	orderBy := "error_count DESC"
	switch params.SortBy {
	case "regression":
		orderBy = "new_errors DESC"
	case "recent":
		orderBy = "first_seen DESC"
	case "error_rate":
		orderBy = "error_rate DESC"
	}
	var latestRelease sql.NullString
	db.conn.QueryRow("SELECT release FROM events WHERE app_id = ? AND release != '' ORDER BY created_at DESC LIMIT 1", params.AppID).Scan(&latestRelease)
	query := fmt.Sprintf(`
		SELECT release, COALESCE(MAX(env), '') as env,
		SUM(CASE WHEN level = 'error' THEN 1 ELSE 0 END) as error_count,
		COUNT(*) as total_events, CAST(SUM(CASE WHEN level = 'error' THEN 1 ELSE 0 END) AS REAL) / COUNT(*) as error_rate,
		MIN(created_at) as first_seen, MAX(created_at) as last_seen, 0 as new_errors
		FROM events %s GROUP BY release ORDER BY %s LIMIT ?`, whereClause, orderBy)
	args = append(args, limit)
	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query top releases: %w", err)
	}
	defer rows.Close()
	var results []TopRelease
	for rows.Next() {
		var r TopRelease
		var env sql.NullString
		err := rows.Scan(&r.Release, &env, &r.ErrorCount, &r.TotalEvents, &r.ErrorRate, &r.FirstSeen, &r.LastSeen, &r.NewErrors)
		if err != nil {
			return nil, fmt.Errorf("failed to scan top release: %w", err)
		}
		if env.Valid {
			r.Env = env.String
		}
		if latestRelease.Valid && r.Release == latestRelease.String {
			r.IsLatest = true
		}
		if r.NewErrors == 0 && params.SortBy == "regression" {
			var existingErrors, totalErrors int64
			db.conn.QueryRow(`SELECT COUNT(DISTINCT e1.message) FROM events e1
				INNER JOIN events e2 ON e1.message = e2.message
				WHERE e1.app_id = ? AND e1.release = ? AND e1.level = 'error'
				AND e2.app_id = ? AND e2.release < ? AND e2.level = 'error'`, params.AppID, r.Release, params.AppID, r.Release).Scan(&existingErrors)
			db.conn.QueryRow(`SELECT COUNT(DISTINCT message) FROM events WHERE app_id = ? AND release = ? AND level = 'error'`, params.AppID, r.Release).Scan(&totalErrors)
			r.NewErrors = totalErrors - existingErrors
			if r.NewErrors < 0 {
				r.NewErrors = 0
			}
		}
		results = append(results, r)
	}
	return results, nil
}

func (db *DB) GetTopBrowsers(params TopListParams) ([]TopBrowser, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()
	if db.closed {
		return nil, fmt.Errorf("database is closed")
	}
	limit := params.Limit
	if limit <= 0 || limit > 50 {
		limit = 10
	}
	whereClause := "WHERE app_id = ? AND ua != ''"
	args := []interface{}{params.AppID}
	if params.StartTime > 0 {
		whereClause += " AND created_at >= ?"
		args = append(args, params.StartTime)
	}
	if params.EndTime > 0 {
		whereClause += " AND created_at <= ?"
		args = append(args, params.EndTime)
	}
	query := fmt.Sprintf(`
		SELECT CASE WHEN ua LIKE '%%Chrome%%' AND ua LIKE '%%Safari%%' THEN 'Chrome'
		WHEN ua LIKE '%%Safari%%' AND ua NOT LIKE '%%Chrome%%' THEN 'Safari'
		WHEN ua LIKE '%%Firefox%%' THEN 'Firefox'
		WHEN ua LIKE '%%Edge%%' THEN 'Edge'
		WHEN ua LIKE '%%MSIE%%' OR ua LIKE '%%Trident%%' THEN 'Internet Explorer'
		ELSE 'Other' END as browser, 'unknown' as version,
		SUM(CASE WHEN level = 'error' THEN 1 ELSE 0 END) as error_count,
		COUNT(*) as total_events, CAST(SUM(CASE WHEN level = 'error' THEN 1 ELSE 0 END) AS REAL) / COUNT(*) as error_rate
		FROM events %s GROUP BY browser ORDER BY error_count DESC LIMIT ?`, whereClause)
	args = append(args, limit)
	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query top browsers: %w", err)
	}
	defer rows.Close()
	var results []TopBrowser
	for rows.Next() {
		var b TopBrowser
		err := rows.Scan(&b.Browser, &b.Version, &b.ErrorCount, &b.TotalEvents, &b.ErrorRate)
		if err != nil {
			return nil, fmt.Errorf("failed to scan top browser: %w", err)
		}
		results = append(results, b)
	}
	return results, nil
}

func (db *DB) GetErrorClusters(appID, errorMessage string, threshold float64, limit int) ([]ErrorCluster, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()
	if db.closed {
		return nil, fmt.Errorf("database is closed")
	}
	if limit <= 0 || limit > 50 {
		limit = 10
	}
	var targetStack sql.NullString
	err := db.conn.QueryRow(`SELECT stack FROM events WHERE app_id = ? AND message = ? AND level = 'error' ORDER BY created_at DESC LIMIT 1`, appID, errorMessage).Scan(&targetStack)
	if err != nil {
		return nil, fmt.Errorf("failed to find target error: %w", err)
	}
	var targetPattern string
	if targetStack.Valid && targetStack.String != "" {
		targetPattern = extractFilePattern(targetStack.String)
	}
	query := `SELECT SUBSTR(message, 1, 100) as pattern, COUNT(*) as count, MIN(created_at) as first_seen,
		MAX(created_at) as last_seen, COUNT(DISTINCT user_id) as affected_users,
		GROUP_CONCAT(DISTINCT message, '|||') as messages, SUBSTR(GROUP_CONCAT(stack), 1, 1000) as sample_stack
		FROM events WHERE app_id = ? AND level = 'error' GROUP BY SUBSTR(message, 1, 100) HAVING count > 0 ORDER BY count DESC LIMIT ?`
	rows, err := db.conn.Query(query, appID, limit*2)
	if err != nil {
		return nil, fmt.Errorf("failed to query error clusters: %w", err)
	}
	defer rows.Close()
	var clusters []ErrorCluster
	clusterID := 0
	for rows.Next() {
		var c ErrorCluster
		var messages, sampleStack sql.NullString
		err := rows.Scan(&c.Pattern, &c.Count, &c.FirstSeen, &c.LastSeen, &c.AffectedUsers, &messages, &sampleStack)
		if err != nil {
			continue
		}
		similarity := 1.0
		if targetPattern != "" && sampleStack.Valid && sampleStack.String != "" {
			similarity = calculateSimilarity(targetPattern, extractFilePattern(sampleStack.String))
		}
		if similarity >= threshold {
			clusterID++
			c.ClusterID = fmt.Sprintf("cluster-%d", clusterID)
			if messages.Valid {
				c.Message = getMostCommonMessage(messages.String)
			} else {
				c.Message = c.Pattern + "..."
			}
			sampleQuery := `SELECT id, app_id, release, env, build_id, user_id, session_id, type, level, message, stack, url, line, col,
				tags, extra, ua, screen, viewport, performance, ip, created_at FROM events WHERE app_id = ? AND level = 'error'
				AND SUBSTR(message, 1, 100) = ? ORDER BY created_at DESC LIMIT 3`
			sampleRows, err := db.conn.Query(sampleQuery, appID, c.Pattern)
			if err == nil {
				for sampleRows.Next() {
					var e EventRecord
					err := sampleRows.Scan(&e.ID, &e.AppID, &e.Release, &e.Env, &e.BuildID, &e.UserID, &e.SessionID,
						&e.Type, &e.Level, &e.Message, &e.Stack, &e.URL, &e.Line, &e.Col, &e.Tags, &e.Extra, &e.UA, &e.Screen,
						&e.Viewport, &e.Performance, &e.IP, &e.CreatedAt)
					if err == nil {
						c.SampleEvents = append(c.SampleEvents, e)
					}
				}
				sampleRows.Close()
			}
			clusters = append(clusters, c)
		}
		if len(clusters) >= limit {
			break
		}
	}
	return clusters, nil
}

func extractFilePattern(stack string) string {
	if stack == "" {
		return ""
	}
	parts := strings.Split(stack, "\n")
	if len(parts) == 0 {
		return ""
	}
	for _, part := range parts {
		if strings.Contains(part, ".js:") || strings.Contains(part, ".ts:") {
			idx := strings.LastIndex(part, "/")
			if idx >= 0 && idx < len(part)-1 {
				return part[idx+1:]
			}
			return part
		}
	}
	return parts[0]
}

func calculateSimilarity(a, b string) float64 {
	if a == b {
		return 1.0
	}
	if a == "" || b == "" {
		return 0.0
	}
	setA := make(map[rune]bool)
	setB := make(map[rune]bool)
	for _, r := range a {
		setA[r] = true
	}
	for _, r := range b {
		setB[r] = true
	}
	intersection := 0
	union := len(setA) + len(setB)
	for r := range setA {
		if setB[r] {
			intersection++
		}
	}
	if union == 0 {
		return 0.0
	}
	return float64(2*intersection) / float64(union)
}

func getMostCommonMessage(messages string) string {
	if messages == "" {
		return ""
	}
	parts := strings.Split(messages, "|||")
	if len(parts) == 1 {
		return parts[0]
	}
	counts := make(map[string]int)
	for _, msg := range parts {
		counts[msg]++
	}
	maxCount := 0
	var result string
	for msg, count := range counts {
		if count > maxCount {
			maxCount = count
			result = msg
		}
	}
	return result
}

// ==================== Feature 1: Error Clustering/Fingerprint ====================

// ErrorClusterResult represents an error cluster from the database
type ErrorClusterResult struct {
	Fingerprint    string   `json:"fingerprint"`
	Message        string   `json:"message"`
	Count          int64    `json:"count"`
	Users          int64    `json:"users"`
	FirstSeen      int64    `json:"firstSeen"`
	LastSeen       int64    `json:"lastSeen"`
	URLs           []string `json:"urls"`
	Releases       []string `json:"releases"`
}

// GetErrorClustersByTime retrieves error clusters grouped by fingerprint within time range
func (db *DB) GetErrorClustersByTime(appID string, startTime, endTime int64, limit int) ([]ErrorClusterResult, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	if db.closed {
		return nil, fmt.Errorf("database is closed")
	}

	if limit <= 0 || limit > 100 {
		limit = 20
	}

	whereClause := "WHERE app_id = ? AND type = 'error' AND fingerprint != ''"
	args := []interface{}{appID}

	if startTime > 0 {
		whereClause += " AND created_at >= ?"
		args = append(args, startTime)
	}
	if endTime > 0 {
		whereClause += " AND created_at <= ?"
		args = append(args, endTime)
	}

	query := `
		SELECT
			fingerprint,
			SUBSTR(GROUP_CONCAT(message), 1, 200) as messages,
			COUNT(*) as count,
			COUNT(DISTINCT user_id) as users,
			MIN(created_at) as first_seen,
			MAX(created_at) as last_seen,
			GROUP_CONCAT(DISTINCT url) as urls,
			GROUP_CONCAT(DISTINCT release) as releases
		FROM events ` + whereClause + `
		GROUP BY fingerprint
		ORDER BY count DESC
		LIMIT ?
	`
	args = append(args, limit)

	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query error clusters: %w", err)
	}
	defer rows.Close()

	var results []ErrorClusterResult
	for rows.Next() {
		var c ErrorClusterResult
		var messages, urls, releases sql.NullString

		err := rows.Scan(&c.Fingerprint, &messages, &c.Count, &c.Users, &c.FirstSeen, &c.LastSeen, &urls, &releases)
		if err != nil {
			return nil, fmt.Errorf("failed to scan error cluster: %w", err)
		}

		// Extract first message as representative
		if messages.Valid {
			c.Message = extractFirstMessage(messages.String)
		}

		// Parse URLs
		if urls.Valid && urls.String != "" {
			c.URLs = splitAndDedup(urls.String, ",", 10)
		}

		// Parse releases
		if releases.Valid && releases.String != "" {
			c.Releases = splitAndDedup(releases.String, ",", 5)
		}

		results = append(results, c)
	}

	return results, nil
}

// GetClusterEvents retrieves events for a specific fingerprint
func (db *DB) GetClusterEvents(appID, fingerprint string, page, pageSize int) ([]EventRecord, int64, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	if db.closed {
		return nil, 0, fmt.Errorf("database is closed")
	}

	if page < 1 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 1000 {
		pageSize = 50
	}

	// Count total
	var total int64
	err := db.conn.QueryRow(`
		SELECT COUNT(*) FROM events
		WHERE app_id = ? AND fingerprint = ?
	`, appID, fingerprint).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count cluster events: %w", err)
	}

	// Query with pagination
	offset := (page - 1) * pageSize
	query := `
		SELECT id, app_id, release, env, build_id, user_id, session_id,
		       type, level, message, stack, url, line, col,
		       tags, extra, ua, screen, viewport, performance, ip, created_at,
		       fingerprint, breadcrumbs
		FROM events
		WHERE app_id = ? AND fingerprint = ?
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`

	rows, err := db.conn.Query(query, appID, fingerprint, pageSize, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query cluster events: %w", err)
	}
	defer rows.Close()

	var events []EventRecord
	for rows.Next() {
		var e EventRecord
		err := rows.Scan(
			&e.ID, &e.AppID, &e.Release, &e.Env, &e.BuildID, &e.UserID, &e.SessionID,
			&e.Type, &e.Level, &e.Message, &e.Stack,
			&e.URL, &e.Line, &e.Col, &e.Tags, &e.Extra, &e.UA, &e.Screen,
			&e.Viewport, &e.Performance, &e.IP, &e.CreatedAt,
			&e.Fingerprint, &e.Breadcrumbs,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan cluster event: %w", err)
		}
		events = append(events, e)
	}

	return events, total, nil
}

// ClusterStats represents detailed statistics for a cluster
type ClusterStats struct {
	Fingerprint         string                 `json:"fingerprint"`
	TotalCount          int64                  `json:"totalCount"`
	UniqueUsers         int64                  `json:"uniqueUsers"`
	FirstSeen           int64                  `json:"firstSeen"`
	LastSeen            int64                  `json:"lastSeen"`
	ReleaseDistribution map[string]int64       `json:"releaseDistribution"`
	EnvDistribution     map[string]int64       `json:"envDistribution"`
	TimeSeries          []map[string]interface{} `json:"timeSeries"`
}

// GetClusterStats retrieves detailed statistics for a specific fingerprint
func (db *DB) GetClusterStats(appID, fingerprint string) (ClusterStats, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	if db.closed {
		return ClusterStats{}, fmt.Errorf("database is closed")
	}

	stats := ClusterStats{
		Fingerprint:         fingerprint,
		ReleaseDistribution: make(map[string]int64),
		EnvDistribution:     make(map[string]int64),
	}

	// Get basic stats
	err := db.conn.QueryRow(`
		SELECT
			COUNT(*) as count,
			COUNT(DISTINCT user_id) as users,
			MIN(created_at) as first_seen,
			MAX(created_at) as last_seen
		FROM events
		WHERE app_id = ? AND fingerprint = ?
	`, appID, fingerprint).Scan(&stats.TotalCount, &stats.UniqueUsers, &stats.FirstSeen, &stats.LastSeen)
	if err != nil {
		return ClusterStats{}, fmt.Errorf("failed to get cluster basic stats: %w", err)
	}

	// Get release distribution
	rows, err := db.conn.Query(`
		SELECT release, COUNT(*) as count
		FROM events
		WHERE app_id = ? AND fingerprint = ? AND release != ''
		GROUP BY release
		ORDER BY count DESC
	`, appID, fingerprint)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var release string
			var count int64
			if rows.Scan(&release, &count) == nil {
				stats.ReleaseDistribution[release] = count
			}
		}
	}

	// Get env distribution
	rows, err = db.conn.Query(`
		SELECT env, COUNT(*) as count
		FROM events
		WHERE app_id = ? AND fingerprint = ? AND env != ''
		GROUP BY env
	`, appID, fingerprint)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var env string
			var count int64
			if rows.Scan(&env, &count) == nil {
				stats.EnvDistribution[env] = count
			}
		}
	}

	// Get time series (last 24 hours by hour)
	now := time.Now().UnixMilli()
	dayAgo := now - 24*60*60*1000

	rows, err = db.conn.Query(`
		SELECT
			(created_at / 3600000) * 3600000 as hour,
			COUNT(*) as count
		FROM events
		WHERE app_id = ? AND fingerprint = ? AND created_at >= ?
		GROUP BY hour
		ORDER BY hour ASC
	`, appID, fingerprint, dayAgo)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var hour int64
			var count int64
			if rows.Scan(&hour, &count) == nil {
				stats.TimeSeries = append(stats.TimeSeries, map[string]interface{}{
					"timestamp": hour,
					"count":     count,
				})
			}
		}
	}

	return stats, nil
}

// Helper functions for error clustering

func extractFirstMessage(messages string) string {
	if messages == "" {
		return ""
	}
	parts := strings.Split(messages, ",")
	if len(parts) > 0 {
		return strings.TrimSpace(parts[0])
	}
	return messages
}

func splitAndDedup(s, sep string, maxItems int) []string {
	if s == "" {
		return []string{}
	}
	parts := strings.Split(s, sep)
	seen := make(map[string]bool)
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" && !seen[p] {
			seen[p] = true
			result = append(result, p)
			if len(result) >= maxItems {
				break
			}
		}
	}
	return result
}

// GetReleaseHealth retrieves crash-free rate and error count grouped by release
func (db *DB) GetReleaseHealth(appID string, startTime, endTime int64) (map[string]interface{}, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	if db.closed {
		return nil, fmt.Errorf("database is closed")
	}

	whereClause := "WHERE app_id = ? AND level = 'error'"
	args := []interface{}{appID}

	if startTime > 0 {
		whereClause += " AND created_at >= ?"
		args = append(args, startTime)
	}
	if endTime > 0 {
		whereClause += " AND created_at <= ?"
		args = append(args, endTime)
	}

	query := "SELECT release, COALESCE(env, '') as env, COUNT(DISTINCT session_id) as total_sessions, COUNT(DISTINCT CASE WHEN level = 'error' THEN session_id END) as crash_sessions, SUM(CASE WHEN level = 'error' THEN 1 ELSE 0 END) as error_count, MIN(created_at) as first_seen, MAX(created_at) as last_seen FROM events " + whereClause + " GROUP BY release, env ORDER BY last_seen DESC"

	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get release health: %w", err)
	}
	defer rows.Close()

	type releaseStat struct {
		Release       string
		Env           string
		TotalSessions int64
		CrashSessions int64
		ErrorCount    int64
		FirstSeen     int64
		LastSeen      int64
	}

	var stats []releaseStat
	var totalSessionsAll int64

	for rows.Next() {
		var s releaseStat
		err := rows.Scan(&s.Release, &s.Env, &s.TotalSessions, &s.CrashSessions, &s.ErrorCount, &s.FirstSeen, &s.LastSeen)
		if err != nil {
			return nil, fmt.Errorf("failed to scan release stat: %w", err)
		}
		stats = append(stats, s)
		totalSessionsAll += s.TotalSessions
	}

	releases := make([]map[string]interface{}, 0)
	for _, s := range stats {
		crashFreeRate := 0.0
		if s.TotalSessions > 0 {
			crashFreeRate = float64(s.TotalSessions-s.CrashSessions) / float64(s.TotalSessions) * 100
		}

		adoptionRate := 0.0
		if totalSessionsAll > 0 {
			adoptionRate = float64(s.TotalSessions) / float64(totalSessionsAll) * 100
		}

		releases = append(releases, map[string]interface{}{
			"release":        s.Release,
			"env":            s.Env,
			"totalSessions":  s.TotalSessions,
			"crashSessions":  s.CrashSessions,
			"crashFreeRate":  crashFreeRate,
			"errorCount":     s.ErrorCount,
			"firstSeen":      s.FirstSeen,
			"lastSeen":       s.LastSeen,
			"adoptionRate":   adoptionRate,
		})
	}

	return map[string]interface{}{
		"releases":      releases,
		"totalSessions": totalSessionsAll,
	}, nil
}

// GetSessionStats retrieves overall session statistics
func (db *DB) GetSessionStats(appID string, startTime, endTime int64) (map[string]interface{}, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	if db.closed {
		return nil, fmt.Errorf("database is closed")
	}

	whereClause := "WHERE app_id = ?"
	args := []interface{}{appID}

	if startTime > 0 {
		whereClause += " AND created_at >= ?"
		args = append(args, startTime)
	}
	if endTime > 0 {
		whereClause += " AND created_at <= ?"
		args = append(args, endTime)
	}

	query := "SELECT COUNT(DISTINCT session_id) as total_sessions, COUNT(DISTINCT CASE WHEN level = 'error' THEN session_id END) as crash_sessions, SUM(CASE WHEN level = 'error' THEN 1 ELSE 0 END) as error_count, MIN(created_at) as first_seen, MAX(created_at) as last_seen FROM events " + whereClause

	var totalSessions, crashSessions, errorCount, firstSeen, lastSeen int64
	err := db.conn.QueryRow(query, args...).Scan(&totalSessions, &crashSessions, &errorCount, &firstSeen, &lastSeen)
	if err != nil {
		return nil, fmt.Errorf("failed to get session stats: %w", err)
	}

	crashFreeRate := 0.0
	if totalSessions > 0 {
		crashFreeRate = float64(totalSessions-crashSessions) / float64(totalSessions) * 100
	}

	durationQuery := "SELECT AVG(duration) as avg_duration FROM (SELECT session_id, MAX(created_at) - MIN(created_at) as duration FROM events " + whereClause + " GROUP BY session_id)"

	var avgDuration float64
	err = db.conn.QueryRow(durationQuery, args...).Scan(&avgDuration)
	if err != nil {
		avgDuration = 0
	}

	return map[string]interface{}{
		"totalSessions":     totalSessions,
		"crashSessions":     crashSessions,
		"crashFreeRate":     crashFreeRate,
		"errorCount":        errorCount,
		"avgSessionDuration": avgDuration,
		"startTime":         firstSeen,
		"endTime":           lastSeen,
	}, nil
}
