package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// InsertEvents batch inserts events into the database
func (db *DB) InsertEvents(events []EventRecord) error {
	if len(events) == 0 {
		return nil
	}

	// Insert events under lock
	if err := db.insertEventsLocked(events); err != nil {
		return err
	}

	// Create or update Issues outside the event lock to avoid deadlock
	// (CreateOrUpdateIssues acquires its own lock)
	if err := db.CreateOrUpdateIssues(events); err != nil {
		// Log error but don't fail the event insertion
		fmt.Printf("Warning: Failed to create/update issues: %v\n", err)
	}

	return nil
}

// insertEventsLocked inserts events under the DB mutex
func (db *DB) insertEventsLocked(events []EventRecord) error {
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
			fingerprint, breadcrumbs, project_id
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
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
			e.Fingerprint, e.Breadcrumbs, e.ProjectID,
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
		       fingerprint, breadcrumbs, project_id
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
			&e.Fingerprint, &e.Breadcrumbs, &e.ProjectID,
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

// GetTopErrors retrieves top errors for an app
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

// GetTopPages retrieves top pages for an app
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

// GetTopReleases retrieves top releases for an app
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

// GetTopBrowsers retrieves top browsers for an app
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
		       fingerprint, breadcrumbs, project_id
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
			&e.Fingerprint, &e.Breadcrumbs, &e.ProjectID,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan cluster event: %w", err)
		}
		events = append(events, e)
	}

	return events, total, nil
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
	now := getCurrentTimeMillis()
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

// GetErrorClusters retrieves error clusters based on similarity
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

// GetRecentEvents retrieves recent events for issue processing
func (db *DB) GetRecentEvents(limit int) ([]EventRecord, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	if db.closed {
		return nil, fmt.Errorf("database is closed")
	}

	if limit <= 0 || limit > 10000 {
		limit = 1000
	}

	rows, err := db.conn.Query(`
		SELECT id, app_id, release, env, build_id, user_id, session_id, type, level,
		       message, stack, url, line, col, tags, extra, ua, screen, viewport,
		       performance, ip, fingerprint, created_at
		FROM events
		WHERE type = 'error' AND fingerprint != ''
		ORDER BY created_at DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query recent events: %w", err)
	}
	defer rows.Close()

	var events []EventRecord
	for rows.Next() {
		var e EventRecord
		err := rows.Scan(&e.ID, &e.AppID, &e.Release, &e.Env, &e.BuildID, &e.UserID, &e.SessionID,
			&e.Type, &e.Level, &e.Message, &e.Stack, &e.URL, &e.Line, &e.Col, &e.Tags, &e.Extra,
			&e.UA, &e.Screen, &e.Viewport, &e.Performance, &e.IP, &e.Fingerprint, &e.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan event: %w", err)
		}
		events = append(events, e)
	}

	return events, nil
}

// Helper function to get current time in milliseconds
func getCurrentTimeMillis() int64 {
	return int64(float64(time.Now().UnixNano()) / 1e6)
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