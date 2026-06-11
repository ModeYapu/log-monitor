package storage

import (
	"database/sql"
	"fmt"
	"time"
)

// Issue constants
const (
	IssueStatusOpen      = "open"
	IssueStatusResolved  = "resolved"
	IssueStatusIgnored   = "ignored"
	IssueStatusMuted     = "muted"

	IssuePriorityLow      = "low"
	IssuePriorityMedium   = "medium"
	IssuePriorityHigh     = "high"
	IssuePriorityCritical = "critical"

	IssueTypeError        = "error"
	IssueTypePerformance  = "performance"
	IssueTypeResource     = "resource"
)

// CreateOrUpdateIssues creates or updates issues based on events with fingerprints
func (db *DB) CreateOrUpdateIssues(events []EventRecord) error {
	if len(events) == 0 {
		return nil
	}

	db.mu.Lock()
	defer db.mu.Unlock()

	if db.closed {
		return fmt.Errorf("database is closed")
	}

	// Group events by (app_id, fingerprint)
	type key struct {
		AppID       string
		Fingerprint string
	}

	eventGroups := make(map[key][]EventRecord)
	for _, event := range events {
		if event.Fingerprint != "" && event.Type == "error" {
			k := key{AppID: event.AppID, Fingerprint: event.Fingerprint}
			eventGroups[k] = append(eventGroups[k], event)
		}
	}

	now := time.Now().UnixMilli()

	// Process each group
	for k, groupEvents := range eventGroups {
		// Get representative message (truncate to 200 chars)
		title := groupEvents[0].Message
		if len(title) > 200 {
			title = title[:200]
		}

		// Check if issue exists
		var existingIssue Issue
		err := db.conn.QueryRow(`
			SELECT id, fingerprint, app_id, title, type, status, priority, assignee,
			       first_seen_at, last_seen_at, event_count, user_count, resolved_at, created_at, updated_at
			FROM issues
			WHERE app_id = ? AND fingerprint = ?
		`, k.AppID, k.Fingerprint).Scan(
			&existingIssue.ID, &existingIssue.Fingerprint, &existingIssue.AppID, &existingIssue.Title,
			&existingIssue.Type, &existingIssue.Status, &existingIssue.Priority, &existingIssue.Assignee,
			&existingIssue.FirstSeenAt, &existingIssue.LastSeenAt, &existingIssue.EventCount,
			&existingIssue.UserCount, &existingIssue.ResolvedAt, &existingIssue.CreatedAt, &existingIssue.UpdatedAt,
		)

		if err == sql.ErrNoRows {
			// Create new issue
			_, err = db.conn.Exec(`
				INSERT INTO issues (fingerprint, app_id, title, type, status, priority, assignee,
				                  first_seen_at, last_seen_at, event_count, user_count, resolved_at, created_at, updated_at)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			`, k.Fingerprint, k.AppID, title, IssueTypeError, IssueStatusOpen, IssuePriorityMedium, "",
				now, now, int64(len(groupEvents)), int64(countDistinctUsers(groupEvents)), 0, now, now)

			if err != nil {
				return fmt.Errorf("failed to create issue: %w", err)
			}
		} else if err != nil {
			return fmt.Errorf("failed to query existing issue: %w", err)
		} else {
			// Update existing issue
			newEventCount := existingIssue.EventCount + int64(len(groupEvents))
			newUserCount := existingIssue.UserCount + int64(countDistinctUsers(groupEvents))

			// Auto-reopen if status is resolved and new events come in
			newStatus := existingIssue.Status
			newResolvedAt := existingIssue.ResolvedAt
			if newStatus == IssueStatusResolved {
				newStatus = IssueStatusOpen
				newResolvedAt = 0
			}

			// Don't auto-reopen muted or ignored issues
			if newStatus == IssueStatusMuted || newStatus == IssueStatusIgnored {
				newStatus = existingIssue.Status
				newResolvedAt = existingIssue.ResolvedAt
			}

			_, err = db.conn.Exec(`
				UPDATE issues
				SET last_seen_at = ?, event_count = ?, user_count = ?, status = ?, resolved_at = ?, updated_at = ?
				WHERE id = ?
			`, now, newEventCount, newUserCount, newStatus, newResolvedAt, now, existingIssue.ID)

			if err != nil {
				return fmt.Errorf("failed to update issue: %w", err)
			}
		}
	}

	return nil
}

// GetIssues retrieves issues with pagination and filters
func (db *DB) GetIssues(filter IssueFilter) ([]Issue, int64, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	if db.closed {
		return nil, 0, fmt.Errorf("database is closed")
	}

	// Build WHERE clause
	whereClause := "WHERE 1=1"
	args := []interface{}{}

	if filter.AppID != "" {
		whereClause += " AND app_id = ?"
		args = append(args, filter.AppID)
	}

	if filter.Status != "" && filter.Status != "all" {
		whereClause += " AND status = ?"
		args = append(args, filter.Status)
	}

	if filter.Priority != "" && filter.Priority != "all" {
		whereClause += " AND priority = ?"
		args = append(args, filter.Priority)
	}

	if filter.Search != "" {
		whereClause += " AND title LIKE ? ESCAPE '\\'"
		args = append(args, "%"+escapeLike(filter.Search)+"%")
	}

	// Count total
	var total int64
	countQuery := "SELECT COUNT(*) FROM issues " + whereClause
	err := db.conn.QueryRow(countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count issues: %w", err)
	}

	// Build ORDER BY clause
	orderBy := "ORDER BY updated_at DESC"
	switch filter.SortBy {
	case "last_seen":
		orderBy = "ORDER BY last_seen_at DESC"
	case "event_count":
		orderBy = "ORDER BY event_count DESC"
	case "priority":
		orderBy = "ORDER BY CASE priority WHEN 'critical' THEN 1 WHEN 'high' THEN 2 WHEN 'medium' THEN 3 WHEN 'low' THEN 4 END ASC, updated_at DESC"
	}

	// Pagination
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PageSize <= 0 || filter.PageSize > 100 {
		filter.PageSize = 20
	}
	offset := (filter.Page - 1) * filter.PageSize

	// Query issues
	query := `
		SELECT id, fingerprint, app_id, title, type, status, priority, assignee,
		       first_seen_at, last_seen_at, event_count, user_count, resolved_at, created_at, updated_at
		FROM issues ` + whereClause + " " + orderBy + " LIMIT ? OFFSET ?"

	args = append(args, filter.PageSize, offset)

	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query issues: %w", err)
	}
	defer rows.Close()

	var issues []Issue
	for rows.Next() {
		var issue Issue
		err := rows.Scan(
			&issue.ID, &issue.Fingerprint, &issue.AppID, &issue.Title, &issue.Type,
			&issue.Status, &issue.Priority, &issue.Assignee, &issue.FirstSeenAt,
			&issue.LastSeenAt, &issue.EventCount, &issue.UserCount, &issue.ResolvedAt,
			&issue.CreatedAt, &issue.UpdatedAt,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan issue: %w", err)
		}
		issues = append(issues, issue)
	}

	return issues, total, nil
}

// GetIssue retrieves a single issue by ID
func (db *DB) GetIssue(id int64) (*Issue, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	if db.closed {
		return nil, fmt.Errorf("database is closed")
	}

	var issue Issue
	err := db.conn.QueryRow(`
		SELECT id, fingerprint, app_id, title, type, status, priority, assignee,
		       first_seen_at, last_seen_at, event_count, user_count, resolved_at, created_at, updated_at
		FROM issues WHERE id = ?
	`, id).Scan(
		&issue.ID, &issue.Fingerprint, &issue.AppID, &issue.Title, &issue.Type,
		&issue.Status, &issue.Priority, &issue.Assignee, &issue.FirstSeenAt,
		&issue.LastSeenAt, &issue.EventCount, &issue.UserCount, &issue.ResolvedAt,
		&issue.CreatedAt, &issue.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get issue: %w", err)
	}

	return &issue, nil
}

// GetIssueEvents retrieves events associated with an issue
func (db *DB) GetIssueEvents(issueID int64, page, pageSize int) ([]EventRecord, int64, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	if db.closed {
		return nil, 0, fmt.Errorf("database is closed")
	}

	// First get the issue to find fingerprint
	issue, err := db.GetIssue(issueID)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get issue: %w", err)
	}

	// Pagination
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	// Count total events for this issue
	var total int64
	err = db.conn.QueryRow(`
		SELECT COUNT(*) FROM events WHERE app_id = ? AND fingerprint = ?
	`, issue.AppID, issue.Fingerprint).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count issue events: %w", err)
	}

	// Query events
	query := `
		SELECT id, app_id, release, env, build_id, user_id, session_id,
		       type, level, message, stack, url, line, col,
		       tags, extra, ua, screen, viewport, performance, ip, created_at,
		       fingerprint, breadcrumbs, project_id
		FROM events
		WHERE app_id = ? AND fingerprint = ?
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`

	rows, err := db.conn.Query(query, issue.AppID, issue.Fingerprint, pageSize, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query issue events: %w", err)
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
			&e.Fingerprint, &e.Breadcrumbs, &e.ProjectID,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan issue event: %w", err)
		}
		events = append(events, e)
	}

	return events, total, nil
}

// UpdateIssue updates an issue's fields
func (db *DB) UpdateIssue(id int64, updates map[string]interface{}) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	if db.closed {
		return fmt.Errorf("database is closed")
	}

	// Build SET clause
	setParts := []string{"updated_at = ?"}
	args := []interface{}{time.Now().UnixMilli()}

	if status, ok := updates["status"].(string); ok {
		setParts = append(setParts, "status = ?")
		args = append(args, status)

		// Set or clear resolved_at based on status
		if status == IssueStatusResolved {
			setParts = append(setParts, "resolved_at = ?")
			args = append(args, time.Now().UnixMilli())
		} else if status == IssueStatusOpen {
			setParts = append(setParts, "resolved_at = 0")
		}
	}

	if priority, ok := updates["priority"].(string); ok {
		setParts = append(setParts, "priority = ?")
		args = append(args, priority)
	}

	if assignee, ok := updates["assignee"].(string); ok {
		setParts = append(setParts, "assignee = ?")
		args = append(args, assignee)
	}

	if len(setParts) == 1 {
		return fmt.Errorf("no fields to update")
	}

	// Add ID to args
	args = append(args, id)

	query := "UPDATE issues SET " + stringJoin(setParts, ", ") + " WHERE id = ?"
	_, err := db.conn.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("failed to update issue: %w", err)
	}

	return nil
}

// GetIssueStats retrieves statistics for issues
func (db *DB) GetIssueStats(appID string) (*IssueStats, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	if db.closed {
		return nil, fmt.Errorf("database is closed")
	}

	stats := &IssueStats{
		ByStatus:   make(map[string]int64),
		ByPriority: make(map[string]int64),
	}

	// Get counts by status
	statusRows, err := db.conn.Query(`
		SELECT status, COUNT(*) as count
		FROM issues
		WHERE app_id = ?
		GROUP BY status
	`, appID)
	if err == nil {
		defer statusRows.Close()
		for statusRows.Next() {
			var status string
			var count int64
			if statusRows.Scan(&status, &count) == nil {
				stats.ByStatus[status] = count
				stats.TotalCount += count

				switch status {
				case IssueStatusOpen:
					stats.OpenCount = count
				case IssueStatusResolved:
					stats.ResolvedCount = count
				case IssueStatusIgnored:
					stats.IgnoredCount = count
				case IssueStatusMuted:
					stats.MutedCount = count
				}
			}
		}
	}

	// Get counts by priority
	priorityRows, err := db.conn.Query(`
		SELECT priority, COUNT(*) as count
		FROM issues
		WHERE app_id = ? AND status != 'resolved'
		GROUP BY priority
	`, appID)
	if err == nil {
		defer priorityRows.Close()
		for priorityRows.Next() {
			var priority string
			var count int64
			if priorityRows.Scan(&priority, &count) == nil {
				stats.ByPriority[priority] = count

				switch priority {
				case IssuePriorityHigh:
					stats.HighPriority = count
				case IssuePriorityCritical:
					stats.CriticalPriority = count
				}
			}
		}
	}

	// Get trend data (last 24 hours)
	now := time.Now().UnixMilli()
	dayAgo := now - 24*60*60*1000

	trendRows, err := db.conn.Query(`
		SELECT (created_at / 3600000) * 3600000 as hour, COUNT(*) as count
		FROM issues
		WHERE app_id = ? AND created_at >= ?
		GROUP BY hour
		ORDER BY hour ASC
	`, appID, dayAgo)
	if err == nil {
		defer trendRows.Close()
		for trendRows.Next() {
			var hour int64
			var count int64
			if trendRows.Scan(&hour, &count) == nil {
				stats.TrendData = append(stats.TrendData, TrendPoint{
					Timestamp: hour,
					Count:     count,
				})
			}
		}
	}

	return stats, nil
}

// Helper function to join strings
func stringJoin(parts []string, sep string) string {
	if len(parts) == 0 {
		return ""
	}
	result := parts[0]
	for i := 1; i < len(parts); i++ {
		result += sep + parts[i]
	}
	return result
}