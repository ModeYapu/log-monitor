package storage

import (
	"database/sql"
	"fmt"
	"time"
)

// SystemStore methods for DB

// GetStorageStats returns storage statistics
func (db *DB) GetStorageStats() (*StorageStats, error) {

	if db.closed.Load() {
		return nil, fmt.Errorf("database is closed")
	}

	// Get database file size
	var dbSize int64
	err := db.conn.QueryRow("SELECT page_count * page_size as size FROM pragma_page_count(), pragma_page_size()").Scan(&dbSize)
	if err != nil {
		return nil, fmt.Errorf("failed to get database size: %w", err)
	}

	// Count total events
	var eventCount int64
	err = db.conn.QueryRow("SELECT COUNT(*) FROM events").Scan(&eventCount)
	if err != nil {
		return nil, fmt.Errorf("failed to count events: %w", err)
	}

	// Count total issues
	var issueCount int64
	err = db.conn.QueryRow("SELECT COUNT(*) FROM issues").Scan(&issueCount)
	if err != nil {
		return nil, fmt.Errorf("failed to count issues: %w", err)
	}

	// Get oldest and newest event timestamps
	var oldestEvent, newestEvent sql.NullInt64
	err = db.conn.QueryRow("SELECT MIN(created_at), MAX(created_at) FROM events").Scan(&oldestEvent, &newestEvent)
	if err != nil {
		return nil, fmt.Errorf("failed to get event timestamps: %w", err)
	}

	return &StorageStats{
		DatabaseSize:    dbSize,
		EventCount:      eventCount,
		IssueCount:      issueCount,
		OldestEventTime: oldestEvent.Int64,
		NewestEventTime: newestEvent.Int64,
	}, nil
}

// GetRetentionPolicySimple returns the retention policy in days
func (db *DB) GetRetentionPolicySimple() (int, error) {

	if db.closed.Load() {
		return 0, fmt.Errorf("database is closed")
	}

	var days sql.NullInt64
	err := db.conn.QueryRow("SELECT CAST(value AS INTEGER) FROM system_meta WHERE key = 'retention_days'").Scan(&days)
	if err != nil {
		if err == sql.ErrNoRows {
			return 30, nil // Default 30 days
		}
		return 0, fmt.Errorf("failed to get retention policy: %w", err)
	}

	return int(days.Int64), nil
}

// SetRetentionPolicySimple sets the retention policy in days
func (db *DB) SetRetentionPolicySimple(days int) error {

	if db.closed.Load() {
		return fmt.Errorf("database is closed")
	}

	_, err := db.conn.Exec(`
		INSERT OR REPLACE INTO system_meta (key, value, updated_at)
		VALUES ('retention_days', ?, ?)
	`, days, time.Now().UnixMilli())

	if err != nil {
		return fmt.Errorf("failed to set retention policy: %w", err)
	}

	return nil
}

// TriggerManualCleanup triggers a manual cleanup operation
func (db *DB) TriggerManualCleanup() error {

	if db.closed.Load() {
		return fmt.Errorf("database is closed")
	}

	// Get retention policy
	var days sql.NullInt64
	err := db.conn.QueryRow("SELECT CAST(value AS INTEGER) FROM system_meta WHERE key = 'retention_days'").Scan(&days)
	if err != nil {
		return fmt.Errorf("failed to get retention policy: %w", err)
	}

	// Perform cleanup
	_ = db.cleanupOldData(int(days.Int64))

	// Update last cleanup time
	_, err = db.conn.Exec(`
		INSERT OR REPLACE INTO system_meta (key, value, updated_at)
		VALUES ('last_cleanup_time', ?, ?)
	`, time.Now().UnixMilli(), time.Now().UnixMilli())

	if err != nil {
		return fmt.Errorf("failed to update last cleanup time: %w", err)
	}

	return nil
}

// GetLastCleanupTime returns the timestamp of the last cleanup
func (db *DB) GetLastCleanupTime() int64 {

	if db.closed.Load() {
		return 0
	}

	var lastCleanup sql.NullInt64
	err := db.conn.QueryRow("SELECT CAST(value AS INTEGER) FROM system_meta WHERE key = 'last_cleanup_time'").Scan(&lastCleanup)
	if err != nil {
		return 0
	}

	return lastCleanup.Int64
}

// SetLastCleanupTime sets the timestamp of the last cleanup
func (db *DB) SetLastCleanupTime(timestamp int64) error {

	if db.closed.Load() {
		return fmt.Errorf("database is closed")
	}

	_, err := db.conn.Exec(`
		INSERT OR REPLACE INTO system_meta (key, value, updated_at)
		VALUES ('last_cleanup_time', ?, ?)
	`, timestamp, time.Now().UnixMilli())

	if err != nil {
		return fmt.Errorf("failed to set last cleanup time: %w", err)
	}

	return nil
}

// CleanupOldDataWithDays performs cleanup with specified retention days
func (db *DB) CleanupOldDataWithDays(days int) CleanupResult {
	return db.cleanupOldData(days)
}

// GetRetentionPolicy retrieves the retention policy
func (db *DB) GetRetentionPolicy() (*RetentionPolicy, error) {
	events, err := db.GetRetentionPolicySimple()
	if err != nil {
		return nil, err
	}

	return &RetentionPolicy{
		Events:          events,
		RecordingsDays:  events,
		ScreenshotsDays: events / 2,
		AlertLogs:       events,
	}, nil
}

// SetRetentionPolicy sets the retention policy
func (db *DB) SetRetentionPolicy(policy *RetentionPolicy) error {
	if policy.Events < 1 || policy.Events > 365 {
		return fmt.Errorf("events days must be between 1 and 365")
	}

	return db.SetRetentionPolicySimple(policy.Events)
}

// CleanupOldDataWithPolicy performs cleanup with the given retention policy
func (db *DB) CleanupOldDataWithPolicy(policy *RetentionPolicy) (*CleanupResultDetail, error) {
	result := db.cleanupOldData(policy.Events)

	return &CleanupResultDetail{
		EventsDeleted:         result.DeletedEvents,
		RecordingEventsDeleted: 0, // Not tracked separately
		ScreenshotsDeleted:    0, // Not implemented yet
		AlertLogsDeleted:      0, // Not tracked separately
		FreedBytes:           result.TotalBytesFreed,
		LastCleanupTime:       time.Now().UnixMilli(),
	}, nil
}

// CleanupResultDetail represents detailed cleanup operation result
type CleanupResultDetail struct {
	EventsDeleted         int64 `json:"events_deleted"`
	RecordingEventsDeleted int64 `json:"recording_events_deleted"`
	ScreenshotsDeleted    int64 `json:"screenshots_deleted"`
	AlertLogsDeleted      int64 `json:"alert_logs_deleted"`
	FreedBytes           int64 `json:"freed_bytes"`
	LastCleanupTime       int64 `json:"last_cleanup_time"`
}