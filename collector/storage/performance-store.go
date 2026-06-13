package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/logmonitor/collector/model"
)

// InsertPerformanceMetric inserts a single performance metric
func (db *DB) InsertPerformanceMetric(metric *model.PerformanceMetric) error {
	if db.closed.Load() {
		return fmt.Errorf("database is closed")
	}

	query := `
		INSERT INTO performance_metrics (
			project_id, app_id, page_url, metric_name, value, rating,
			release, user_id, session_id, ua, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := db.conn.Exec(
		query,
		metric.ProjectID, metric.AppID, metric.PageURL, metric.MetricName,
		metric.Value, metric.Rating, metric.Release, metric.UserID,
		metric.SessionID, metric.UA, metric.CreatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to insert performance metric: %w", err)
	}

	return nil
}

// GetPerformanceSummaryByPage returns performance metrics aggregated by page URL
func (db *DB) GetPerformanceSummaryByPage(projectID int64, metricName string, period string) ([]*model.PagePerformanceSummary, error) {
	if db.closed.Load() {
		return nil, fmt.Errorf("database is closed")
	}

	// Calculate time threshold based on period
	var threshold int64
	now := time.Now()
	switch period {
	case "1d":
		threshold = now.Add(-24 * time.Hour).UnixMilli()
	case "7d":
		threshold = now.Add(-7 * 24 * time.Hour).UnixMilli()
	case "30d":
		threshold = now.Add(-30 * 24 * time.Hour).UnixMilli()
	default:
		threshold = now.Add(-24 * time.Hour).UnixMilli()
	}

	// Get all metrics for this project and time period, then aggregate in Go
	// This avoids complex SQL correlated subqueries
	query := `
		SELECT page_url, value
		FROM performance_metrics
		WHERE metric_name = ?
		AND project_id = ?
		AND created_at >= ?
		ORDER BY page_url, value
	`

	rows, err := db.conn.Query(query, metricName, projectID, threshold)
	if err != nil {
		return nil, fmt.Errorf("failed to query performance metrics: %w", err)
	}
	defer rows.Close()

	// Group by page and calculate percentiles
	pageValues := make(map[string][]float64)
	for rows.Next() {
		var pageURL string
		var value float64
		if err := rows.Scan(&pageURL, &value); err != nil {
			slog.Warn("Failed to scan metric row", "error", err)
			continue
		}
		pageValues[pageURL] = append(pageValues[pageURL], value)
	}

	// Calculate percentiles for each page
	var results []*model.PagePerformanceSummary
	for pageURL, values := range pageValues {
		if len(values) < 3 {
			continue // Skip pages with insufficient data
		}

		// Values are already sorted from the ORDER BY
		p50 := percentile(values, 0.50)
		p75 := percentile(values, 0.75)
		p95 := percentile(values, 0.95)

		results = append(results, &model.PagePerformanceSummary{
			PageURL: pageURL,
			P50:     p50,
			P75:     p75,
			P95:     p95,
			Count:   int64(len(values)),
		})
	}

	// Sort by P75 ascending (better performance first)
	for i := 0; i < len(results); i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].P75 < results[i].P75 {
				results[i], results[j] = results[j], results[i]
			}
		}
	}

	// Limit results
	if len(results) > 50 {
		results = results[:50]
	}

	return results, nil
}

// percentile calculates the percentile value from a sorted slice
func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	if len(sorted) == 1 {
		return sorted[0]
	}
	index := int(float64(len(sorted)-1) * p)
	if index >= len(sorted) {
		index = len(sorted) - 1
	}
	return sorted[index]
}

// GetPerformanceTrendByPage returns daily performance trend for a specific page and metric
func (db *DB) GetPerformanceTrendByPage(projectID int64, pageURL string, metricName string, days int) ([]*model.DailyMetric, error) {
	if db.closed.Load() {
		return nil, fmt.Errorf("database is closed")
	}

	if days <= 0 || days > 90 {
		days = 30
	}

	threshold := time.Now().Add(-time.Duration(days) * 24 * time.Hour).UnixMilli()

	// Get all metrics for this page and time period
	query := `
		SELECT created_at, value
		FROM performance_metrics
		WHERE project_id = ?
		AND page_url = ?
		AND metric_name = ?
		AND created_at >= ?
		ORDER BY created_at
	`

	rows, err := db.conn.Query(query, projectID, pageURL, metricName, threshold)
	if err != nil {
		return nil, fmt.Errorf("failed to query performance trend: %w", err)
	}
	defer rows.Close()

	// Group by date and calculate P75
	dayValues := make(map[string][]float64)
	for rows.Next() {
		var createdAt int64
		var value float64
		if err := rows.Scan(&createdAt, &value); err != nil {
			slog.Warn("Failed to scan metric row", "error", err)
			continue
		}

		date := time.UnixMilli(createdAt).Format("2006-01-02")
		dayValues[date] = append(dayValues[date], value)
	}

	// Calculate P75 for each day
	var results []*model.DailyMetric
	for date, values := range dayValues {
		if len(values) == 0 {
			continue
		}

		// Sort values
		for i := 0; i < len(values); i++ {
			for j := i + 1; j < len(values); j++ {
				if values[j] < values[i] {
					values[i], values[j] = values[j], values[i]
				}
			}
		}

		p75 := percentile(values, 0.75)

		results = append(results, &model.DailyMetric{
			Date:      date,
			P75:       p75,
			Count:     int64(len(values)),
			AvgRating: model.GetRating(metricName, p75),
		})
	}

	// Sort by date
	for i := 0; i < len(results); i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].Date < results[i].Date {
				results[i], results[j] = results[j], results[i]
			}
		}
	}

	return results, nil
}

// GetPerformanceComparison compares metrics between two releases
func (db *DB) GetPerformanceComparison(projectID int64, metricName string, releaseA, releaseB string) ([]*model.ReleaseComparison, error) {
	if db.closed.Load() {
		return nil, fmt.Errorf("database is closed")
	}

	// Use 7 days lookback for comparison
	threshold := time.Now().Add(-7 * 24 * time.Hour).UnixMilli()

	// Get metrics for both releases
	query := `
		SELECT release, page_url, value
		FROM performance_metrics
		WHERE project_id = ?
		AND metric_name = ?
		AND release IN (?, ?)
		AND created_at >= ?
		ORDER BY release, page_url, value
	`

	rows, err := db.conn.Query(query, projectID, metricName, releaseA, releaseB, threshold)
	if err != nil {
		return nil, fmt.Errorf("failed to query performance comparison: %w", err)
	}
	defer rows.Close()

	// Group by release and page
	releaseAValues := make(map[string][]float64)
	releaseBValues := make(map[string][]float64)

	for rows.Next() {
		var release string
		var pageURL string
		var value float64
		if err := rows.Scan(&release, &pageURL, &value); err != nil {
			slog.Warn("Failed to scan metric row", "error", err)
			continue
		}

		if release == releaseA {
			releaseAValues[pageURL] = append(releaseAValues[pageURL], value)
		} else if release == releaseB {
			releaseBValues[pageURL] = append(releaseBValues[pageURL], value)
		}
	}

	// Find pages that exist in both releases
	var results []*model.ReleaseComparison
	for pageURL := range releaseAValues {
		if _, exists := releaseBValues[pageURL]; !exists {
			continue
		}

		valuesA := releaseAValues[pageURL]
		valuesB := releaseBValues[pageURL]

		// Calculate P75 for each release
		p75A := percentile(valuesA, 0.75)
		p75B := percentile(valuesB, 0.75)

		comp := &model.ReleaseComparison{
			MetricName: metricName,
			ReleaseA:   releaseA,
			ReleaseB:   releaseB,
			ValueA:     p75A,
			ValueB:     p75B,
			CountA:     int64(len(valuesA)),
			CountB:     int64(len(valuesB)),
		}

		// Calculate percentage change
		if comp.ValueA > 0 {
			comp.Change = ((comp.ValueB - comp.ValueA) / comp.ValueA) * 100
		}
		// For most metrics, lower is better
		comp.Improved = comp.ValueB < comp.ValueA

		results = append(results, comp)
	}

	// Limit results
	if len(results) > 50 {
		results = results[:50]
	}

	return results, nil
}

// EnsurePerformanceMetricsTable creates the performance_metrics table if it doesn't exist
func (db *DB) EnsurePerformanceMetricsTable() error {
	query := `
		CREATE TABLE IF NOT EXISTS performance_metrics (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			project_id INTEGER NOT NULL,
			app_id TEXT NOT NULL,
			page_url TEXT NOT NULL,
			metric_name TEXT NOT NULL,
			value REAL NOT NULL,
			rating TEXT NOT NULL,
			release TEXT DEFAULT '',
			user_id TEXT DEFAULT '',
			session_id TEXT DEFAULT '',
			ua TEXT DEFAULT '',
			created_at INTEGER NOT NULL
		);
	`

	_, err := db.conn.Exec(query)
	if err != nil {
		return fmt.Errorf("failed to create performance_metrics table: %w", err)
	}

	// Create indexes for better query performance
	indexes := []string{
		"CREATE INDEX IF NOT EXISTS idx_perf_metrics_project ON performance_metrics(project_id)",
		"CREATE INDEX IF NOT EXISTS idx_perf_metrics_metric ON performance_metrics(metric_name)",
		"CREATE INDEX IF NOT EXISTS idx_perf_metrics_page ON performance_metrics(page_url)",
		"CREATE INDEX IF NOT EXISTS idx_perf_metrics_created ON performance_metrics(created_at)",
		"CREATE INDEX IF NOT EXISTS idx_perf_metrics_composite ON performance_metrics(project_id, metric_name, created_at)",
	}

	for _, idx := range indexes {
		_, err := db.conn.Exec(idx)
		if err != nil {
			slog.Warn("Failed to create index", "index", idx, "error", err)
		}
	}

	// Create migration tracking table
	_, err = db.conn.Exec(`
		CREATE TABLE IF NOT EXISTS performance_migration_log (
			id INTEGER PRIMARY KEY,
			migrated_at INTEGER NOT NULL
		)
	`)
	if err != nil {
		slog.Warn("Failed to create migration_log table", "error", err)
	}

	return nil
}

// MigratePerformanceEvents migrates existing performance data from events table
func (db *DB) MigratePerformanceEvents(limit int) (int, error) {
	if db.closed.Load() {
		return 0, fmt.Errorf("database is closed")
	}

	if limit <= 0 || limit > 10000 {
		limit = 1000
	}

	// Query events with performance data
	query := `
		SELECT id, project_id, app_id, url, performance, release, user_id, session_id, ua, created_at
		FROM events
		WHERE performance IS NOT NULL
		AND performance != '{}'
		AND id > COALESCE((SELECT MAX(id) FROM performance_migration_log), 0)
		ORDER BY id
		LIMIT ?
	`

	rows, err := db.conn.Query(query, limit)
	if err != nil {
		return 0, fmt.Errorf("failed to query events for migration: %w", err)
	}
	defer rows.Close()

	migrated := 0
	var lastEventID int64

	for rows.Next() {
		var eventID int64
		var projectID int64
		var appID, url, performanceJSON, release, userID, sessionID, ua sql.NullString
		var createdAt int64

		err := rows.Scan(&eventID, &projectID, &appID, &url, &performanceJSON, &release, &userID, &sessionID, &ua, &createdAt)
		if err != nil {
			slog.Warn("Failed to scan event row", "error", err)
			continue
		}

		if !performanceJSON.Valid || performanceJSON.String == "" {
			continue
		}

		// Parse performance JSON
		var perfData map[string]interface{}
		if err := json.Unmarshal([]byte(performanceJSON.String), &perfData); err != nil {
			slog.Warn("Failed to parse performance JSON", "event_id", eventID, "error", err)
			continue
		}

		// Insert each metric
		for metricName, value := range perfData {
			var val float64
			switch v := value.(type) {
			case float64:
				val = v
			case int64:
				val = float64(v)
			case int:
				val = float64(v)
			default:
				continue
			}

			if val <= 0 {
				continue
			}

			metric := &model.PerformanceMetric{
				ProjectID:  projectID,
				AppID:      appID.String,
				PageURL:    url.String,
				MetricName: metricName,
				Value:      val,
				Rating:     model.GetRating(metricName, val),
				Release:    release.String,
				UserID:     userID.String,
				SessionID:  sessionID.String,
				UA:         ua.String,
				CreatedAt:  createdAt,
			}

			if err := db.InsertPerformanceMetric(metric); err != nil {
				slog.Warn("Failed to insert migrated metric", "event_id", eventID, "metric", metricName, "error", err)
			} else {
				migrated++
			}
		}

		lastEventID = eventID
	}

	// Log the migration progress
	if lastEventID > 0 {
		_, err := db.conn.Exec("INSERT OR REPLACE INTO performance_migration_log (id, migrated_at) VALUES (?, ?)", lastEventID, time.Now().UnixMilli())
		if err != nil {
			slog.Warn("Failed to log migration progress", "error", err)
		}
	}

	return migrated, nil
}
