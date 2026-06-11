package storage

import (
	"fmt"
)

// AnalyticsStore methods for DB

// GetReleaseHealth retrieves crash-free rate and error count grouped by release
func (db *DB) GetReleaseHealth(appID string, startTime, endTime int64) (*ReleaseHealthResult, error) {
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

	releases := make([]ReleaseStats, 0)
	totalCrashFree := 0.0

	for _, s := range stats {
		crashFreeRate := 0.0
		if s.TotalSessions > 0 {
			crashFreeRate = float64(s.TotalSessions-s.CrashSessions) / float64(s.TotalSessions) * 100
		}
		totalCrashFree += crashFreeRate

		errorRate := 0.0
		if s.TotalSessions > 0 {
			errorRate = float64(s.ErrorCount) / float64(s.TotalSessions) * 100
		}

		releases = append(releases, ReleaseStats{
			Release:    s.Release,
			Env:        s.Env,
			Count:      s.TotalSessions,
			ErrorRate:  errorRate,
			CrashFree:  crashFreeRate,
		})
	}

	avgCrashFree := 0.0
	if len(releases) > 0 {
		avgCrashFree = totalCrashFree / float64(len(releases))
	}

	return &ReleaseHealthResult{
		Releases:      releases,
		TotalSessions: totalSessionsAll,
		CrashFreeRate: avgCrashFree,
	}, nil
}

// GetSessionStats retrieves overall session statistics
func (db *DB) GetSessionStats(appID string, startTime, endTime int64) (*SessionStatsResult, error) {
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

	return &SessionStatsResult{
		TotalSessions:     totalSessions,
		CrashSessions:     crashSessions,
		CrashFreeRate:     crashFreeRate,
		ErrorCount:        errorCount,
		AvgSessionLength:  avgDuration,
		StartTime:         firstSeen,
		EndTime:           lastSeen,
	}, nil
}