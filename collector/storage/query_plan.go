package storage

import (
	"fmt"
	"strings"
)

// ExplainQueryPlan runs EXPLAIN QUERY PLAN for the given statement and returns
// a human-readable plan (one line per plan step). It does not execute the query.
//
// This is a diagnostic helper: the returned text reveals whether the planner
// performs a full TABLE SCAN (slow on large tables) or uses an index
// (SEARCH ... USING INDEX "idx_..."). Call it from tests or debug endpoints to
// verify that high-frequency queries are covered by composite indexes.
func (db *DB) ExplainQueryPlan(query string, args ...interface{}) (string, error) {
	if db.closed.Load() {
		return "", fmt.Errorf("database is closed")
	}
	rows, err := db.conn.Query("EXPLAIN QUERY PLAN "+query, args...)
	if err != nil {
		return "", fmt.Errorf("failed to explain query plan: %w", err)
	}
	defer rows.Close()

	var lines []string
	for rows.Next() {
		// EXPLAIN QUERY PLAN columns: id | parent | notused | detail
		var id, parent, notused int
		var detail string
		if err := rows.Scan(&id, &parent, &notused, &detail); err != nil {
			return "", fmt.Errorf("failed to scan plan row: %w", err)
		}
		lines = append(lines, detail)
	}
	return strings.Join(lines, "\n"), nil
}

// AnalyzeTopNQuery builds the same query shape used by GetTopN (for the given
// topType) and returns its EXPLAIN QUERY PLAN. The query is reconstructed from
// the production WHERE/GROUP BY clauses so the plan reflects real execution.
//
// topType is one of: "errors", "pages", "releases", "browsers".
// filters mirrors the AnalyticsFilters applied at runtime (project/env/release
// and time range). The result lets operators confirm idx_events_app_level_message
// / idx_events_app_url are selected instead of a full scan.
func (db *DB) AnalyzeTopNQuery(appID, topType, orderBy string, limit int, filters AnalyticsFilters) (string, error) {
	whereClause := "WHERE app_id = ?"
	args := []interface{}{appID}
	if filters.ProjectID > 0 {
		whereClause += " AND project_id = ?"
		args = append(args, filters.ProjectID)
	}
	if filters.Env != "" {
		whereClause += " AND env = ?"
		args = append(args, filters.Env)
	}
	if filters.Release != "" {
		whereClause += " AND release = ?"
		args = append(args, filters.Release)
	}
	if filters.StartTime > 0 {
		whereClause += " AND created_at >= ?"
		args = append(args, filters.StartTime)
	}
	if filters.EndTime > 0 {
		whereClause += " AND created_at <= ?"
		args = append(args, filters.EndTime)
	}

	var groupField string
	switch topType {
	case "errors":
		whereClause += " AND level = 'error'"
		groupField = "message"
	case "pages":
		groupField = "url"
	case "releases":
		groupField = "release"
	case "browsers":
		groupField = "ua"
	default:
		return "", fmt.Errorf("invalid top type: %s", topType)
	}

	orderClause := "ORDER BY count DESC"
	switch orderBy {
	case "count":
		orderClause = "ORDER BY count DESC"
	case "lastSeen":
		orderClause = "ORDER BY last_seen DESC"
	}

	query := fmt.Sprintf(`
		SELECT %s as key, COUNT(*) as count, COUNT(DISTINCT user_id) as users,
			MAX(created_at) as last_seen, MIN(created_at) as first_seen
		FROM events
		%s
		GROUP BY %s
		%s
		LIMIT ?
	`, groupField, whereClause, groupField, orderClause)
	args = append(args, limit)

	return db.ExplainQueryPlan(query, args...)
}

// AnalyzeErrorClustersQuery builds the aggregation query used by GetErrorClusters
// (WHERE app_id [+project_id] AND level='error', GROUP BY message prefix) and
// returns its EXPLAIN QUERY PLAN. Use it to verify idx_events_app_level_message
// is chosen over a full TABLE SCAN on large events tables.
func (db *DB) AnalyzeErrorClustersQuery(appID string, projectID int64) (string, error) {
	whereClause := "WHERE app_id = ?"
	args := []interface{}{appID}
	if projectID > 0 {
		whereClause += " AND project_id = ?"
		args = append(args, projectID)
	}
	query := fmt.Sprintf(`
		SELECT SUBSTR(message, 1, 100) as pattern, COUNT(*) as count, MIN(created_at) as first_seen,
			MAX(created_at) as last_seen, COUNT(DISTINCT user_id) as affected_users
		FROM events %s AND level = 'error'
		GROUP BY SUBSTR(message, 1, 100)
		ORDER BY count DESC
		LIMIT 20
	`, whereClause)
	return db.ExplainQueryPlan(query, args...)
}

// PlanUsesIndex returns true if the given EXPLAIN QUERY PLAN output references
// an index (i.e. is not a pure full-table scan). A "SEARCH ... USING INDEX" or
// "SEARCH ... USING COVERING INDEX" line counts as index usage; a bare
// "SCAN events" does not.
func PlanUsesIndex(plan string) bool {
	if plan == "" {
		return false
	}
	if strings.Contains(plan, "USING INDEX") || strings.Contains(plan, "USING COVERING INDEX") {
		return true
	}
	return false
}
