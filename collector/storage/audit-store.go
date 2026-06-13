package storage

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/logmonitor/collector/model"
)

// InsertAuditLog inserts a new audit log entry
func (db *DB) InsertAuditLog(log *model.AuditLog) error {
	if db.closed.Load() {
		return fmt.Errorf("database is closed")
	}

	now := time.Now().UnixMilli()
	if log.CreatedAt == 0 {
		log.CreatedAt = now
	}

	query := `
		INSERT INTO audit_logs (
			project_id, user_id, username, action, resource, resource_id,
			detail, ip, user_agent, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	result, err := db.conn.Exec(query,
		log.ProjectID, log.UserID, log.Username, log.Action, log.Resource,
		log.ResourceID, log.Detail, log.IP, log.UserAgent, log.CreatedAt,
	)
	if err != nil {
		slog.Error("Failed to insert audit log", "error", err)
		return fmt.Errorf("failed to insert audit log: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	log.ID = id

	return nil
}

// QueryAuditLogs retrieves audit logs with pagination and filters
func (db *DB) QueryAuditLogs(filter model.AuditFilter) ([]*model.AuditLog, int64, error) {
	if db.closed.Load() {
		return nil, 0, fmt.Errorf("database is closed")
	}

	// Build WHERE clause
	whereClause := "WHERE 1=1"
	args := []interface{}{}

	if filter.ProjectID > 0 {
		whereClause += " AND project_id = ?"
		args = append(args, filter.ProjectID)
	}

	if filter.UserID > 0 {
		whereClause += " AND user_id = ?"
		args = append(args, filter.UserID)
	}

	if filter.Action != "" {
		whereClause += " AND action = ?"
		args = append(args, filter.Action)
	}

	if filter.Resource != "" {
		whereClause += " AND resource = ?"
		args = append(args, filter.Resource)
	}

	if filter.StartDate > 0 {
		whereClause += " AND created_at >= ?"
		args = append(args, filter.StartDate)
	}

	if filter.EndDate > 0 {
		whereClause += " AND created_at <= ?"
		args = append(args, filter.EndDate)
	}

	// Count total
	var total int64
	countQuery := "SELECT COUNT(*) FROM audit_logs " + whereClause
	err := db.conn.QueryRow(countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count audit logs: %w", err)
	}

	// Pagination
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PageSize <= 0 || filter.PageSize > 100 {
		filter.PageSize = 20
	}
	offset := (filter.Page - 1) * filter.PageSize

	// Query audit logs
	query := `
		SELECT id, project_id, user_id, username, action, resource, resource_id,
		       detail, ip, user_agent, created_at
		FROM audit_logs ` + whereClause + `
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`

	args = append(args, filter.PageSize, offset)

	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query audit logs: %w", err)
	}
	defer rows.Close()

	var logs []*model.AuditLog
	for rows.Next() {
		var log model.AuditLog
		err := rows.Scan(
			&log.ID, &log.ProjectID, &log.UserID, &log.Username,
			&log.Action, &log.Resource, &log.ResourceID,
			&log.Detail, &log.IP, &log.UserAgent, &log.CreatedAt,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan audit log: %w", err)
		}
		logs = append(logs, &log)
	}

	return logs, total, nil
}

// EnsureAuditLogsTable creates the audit_logs table if it doesn't exist
func (db *DB) EnsureAuditLogsTable() error {
	schema := `
		CREATE TABLE IF NOT EXISTS audit_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			project_id INTEGER DEFAULT NULL,
			user_id INTEGER NOT NULL,
			username TEXT NOT NULL,
			action TEXT NOT NULL,
			resource TEXT NOT NULL,
			resource_id TEXT DEFAULT '',
			detail TEXT DEFAULT '',
			ip TEXT DEFAULT '',
			user_agent TEXT DEFAULT '',
			created_at INTEGER NOT NULL
		);
	`

	_, err := db.conn.Exec(schema)
	if err != nil {
		return fmt.Errorf("failed to create audit_logs table: %w", err)
	}

	// Create indexes for efficient querying
	indexes := []string{
		`CREATE INDEX IF NOT EXISTS idx_audit_logs_project ON audit_logs(project_id, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_logs_user ON audit_logs(user_id, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_logs_action ON audit_logs(action, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_logs_resource ON audit_logs(resource, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_logs_created ON audit_logs(created_at DESC)`,
	}

	for _, idx := range indexes {
		if _, err := db.conn.Exec(idx); err != nil {
			slog.Warn("Failed to create audit_logs index", "error", err)
		}
	}

	return nil
}
