package storage

import (
	cryptoRand "crypto/rand"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	mathRand "math/rand"
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

	CREATE TABLE IF NOT EXISTS issues (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		fingerprint TEXT NOT NULL,
		app_id TEXT NOT NULL,
		title TEXT NOT NULL,
		type TEXT NOT NULL DEFAULT 'error',
		status TEXT NOT NULL DEFAULT 'open',
		priority TEXT NOT NULL DEFAULT 'medium',
		assignee TEXT DEFAULT '',
		first_seen_at INTEGER NOT NULL,
		last_seen_at INTEGER NOT NULL,
		event_count INTEGER NOT NULL DEFAULT 0,
		user_count INTEGER NOT NULL DEFAULT 0,
		resolved_at INTEGER DEFAULT 0,
		created_at INTEGER NOT NULL,
		updated_at INTEGER NOT NULL
	);

	CREATE UNIQUE INDEX IF NOT EXISTS idx_issues_fingerprint ON issues(app_id, fingerprint);
	CREATE INDEX IF NOT EXISTS idx_issues_status ON issues(app_id, status, updated_at DESC);
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

	// Migration: Create projects and project_members tables (Slice 2: Multi-tenant)
	projectTablesMigrations := []string{
		`CREATE TABLE IF NOT EXISTS projects (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			slug TEXT NOT NULL UNIQUE,
			description TEXT DEFAULT '',
			api_key TEXT NOT NULL UNIQUE,
			retention_days INTEGER DEFAULT 30,
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL,
			deleted_at INTEGER DEFAULT 0
		)`,
		`CREATE TABLE IF NOT EXISTS project_members (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			project_id INTEGER NOT NULL,
			user_id INTEGER NOT NULL,
			role TEXT NOT NULL DEFAULT 'viewer',
			created_at INTEGER NOT NULL,
			UNIQUE(project_id, user_id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_project_members_project ON project_members(project_id)`,
		`CREATE INDEX IF NOT EXISTS idx_project_members_user ON project_members(user_id)`,
	}
	for _, m := range projectTablesMigrations {
		if _, mErr := db.conn.Exec(m); mErr != nil {
			log.Printf("Migration notice: %v", mErr)
		}
	}

	// Migration: Add project_id columns to existing tables
	projectIdMigrations := []string{
		`ALTER TABLE events ADD COLUMN project_id INTEGER DEFAULT NULL`,
		`ALTER TABLE issues ADD COLUMN project_id INTEGER DEFAULT NULL`,
		`ALTER TABLE alert_rules ADD COLUMN project_id INTEGER DEFAULT NULL`,
	}
	for _, m := range projectIdMigrations {
		if _, mErr := db.conn.Exec(m); mErr != nil && !strings.Contains(mErr.Error(), "duplicate column") {
			log.Printf("Migration notice: %v", mErr)
		}
	}

	return nil
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

// cleanupOldData deletes events older than retention days and cleans orphaned recording_events and alert_logs
func (db *DB) cleanupOldData(retentionDays int) CleanupResult {
	cutoff := time.Now().AddDate(0, 0, -retentionDays).UnixMilli()

	db.mu.Lock()
	defer db.mu.Unlock()

	result := CleanupResult{
		DeletedEvents:      0,
		DeletedScreenshots: 0,
		TotalFilesFreed:     0,
		TotalBytesFreed:     0,
		Duration:           0,
	}

	// Delete old events
	rows, err := db.conn.Exec("DELETE FROM events WHERE created_at < ?", cutoff)
	if err != nil {
		log.Printf("Failed to delete old events: %v", err)
	} else if rowsAffected, _ := rows.RowsAffected(); rowsAffected > 0 {
		result.DeletedEvents = rowsAffected
		log.Printf("Deleted %d old events (older than %d days)", rowsAffected, retentionDays)
	}

	// Clean orphaned recording_events
	rows, err = db.conn.Exec(`
		DELETE FROM recording_events
		WHERE session_id NOT IN (SELECT session_id FROM recordings)
	`)
	if err != nil {
		log.Printf("Failed to delete orphaned recording_events: %v", err)
	} else if rowsAffected, _ := rows.RowsAffected(); rowsAffected > 0 {
		orphanDeleted := rowsAffected
		result.TotalFilesFreed += orphanDeleted
		log.Printf("Deleted %d orphaned recording_events", orphanDeleted)
	}

	// Delete old alert logs
	alertsCutoff := time.Now().AddDate(0, 0, -retentionDays).UnixMilli()
	rows, err = db.conn.Exec("DELETE FROM alert_logs WHERE created_at < ?", alertsCutoff)
	if err != nil {
		log.Printf("Failed to delete old alert_logs: %v", err)
	} else if rowsAffected, _ := rows.RowsAffected(); rowsAffected > 0 {
		alertDeleted := rowsAffected
		result.TotalFilesFreed += alertDeleted
		log.Printf("Deleted %d old alert_logs (older than %d days)", alertDeleted, retentionDays)
	}

	return result
}

// cleanupOldDataInternal performs the actual cleanup operation
func (db *DB) cleanupOldDataInternal(retentionDays int) cleanupResultInternal {
	cutoff := time.Now().AddDate(0, 0, -retentionDays).UnixMilli()

	db.mu.Lock()
	defer db.mu.Unlock()

	if db.closed {
		return cleanupResultInternal{}
	}

	result := cleanupResultInternal{}

	// Delete old events
	rows, err := db.conn.Exec("DELETE FROM events WHERE created_at < ?", cutoff)
	if err != nil {
		log.Printf("Failed to delete old events: %v", err)
	} else if rowsAffected, _ := rows.RowsAffected(); rowsAffected > 0 {
		result.EventsDeleted = rowsAffected
		log.Printf("Deleted %d old events (older than %d days)", rowsAffected, retentionDays)
	}

	// Clean orphaned recording_events (events without a corresponding recording)
	rows, err = db.conn.Exec(`
		DELETE FROM recording_events
		WHERE session_id NOT IN (SELECT session_id FROM recordings)
	`)
	if err != nil {
		log.Printf("Failed to delete orphaned recording_events: %v", err)
	} else if rowsAffected, _ := rows.RowsAffected(); rowsAffected > 0 {
		orphanDeleted := rowsAffected
		result.RecordingEventsDeleted += orphanDeleted
		log.Printf("Deleted %d orphaned recording_events", orphanDeleted)
	}

	// Delete old alert logs
	alertsCutoff := time.Now().AddDate(0, 0, -retentionDays).UnixMilli()
	rows, err = db.conn.Exec("DELETE FROM alert_logs WHERE created_at < ?", alertsCutoff)
	if err != nil {
		log.Printf("Failed to delete old alert_logs: %v", err)
	} else if rowsAffected, _ := rows.RowsAffected(); rowsAffected > 0 {
		result.AlertLogsDeleted = rowsAffected
		log.Printf("Deleted %d old alert_logs (older than %d days)", rowsAffected, retentionDays)
	}

	return result
}

// escapeLike escapes special characters in LIKE queries to prevent SQL injection
func escapeLike(s string) string {
	// Escape backslash first, then % and _
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "%", "\\%")
	s = strings.ReplaceAll(s, "_", "\\_")
	return s
}

// parseJSON parses JSON string into map
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

// extractFilePattern extracts file pattern from stack trace
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

// calculateSimilarity calculates similarity between two strings using Jaccard index
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

// getMostCommonMessage returns the most common message from a concatenated string
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

// extractFirstMessage extracts the first message from a concatenated string
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

// splitAndDedup splits string by separator and returns deduplicated items
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

// countDistinctUsers counts distinct user IDs in events
func countDistinctUsers(events []EventRecord) int {
	seen := make(map[string]bool)
	for _, e := range events {
		if e.UserID != "" {
			seen[e.UserID] = true
		}
	}
	return len(seen)
}

// generateUUID generates a random UUID for API keys
func generateUUID() string {
	b := make([]byte, 16)
	_, err := cryptoRand.Read(b)
	if err != nil {
		// Fallback to timestamp-based UUID if crypto rand fails
		return fmt.Sprintf("%d-%d", time.Now().UnixNano(), mathRand.Int63())
	}

	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
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