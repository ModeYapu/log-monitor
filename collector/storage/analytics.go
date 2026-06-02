package storage

import (
	"fmt"
	"time"
)

// GetTopN retrieves top N items grouped by type (errors/pages/releases/browsers)
func (db *DB) GetTopN(appID, topType, orderBy string, limit int, filters map[string]interface{}) (*TopNResult, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	if db.closed {
		return nil, fmt.Errorf("database is closed")
	}

	if limit <= 0 || limit > 100 {
		limit = 20
	}

	// Build WHERE clause
	whereClause := "WHERE app_id = ?"
	args := []interface{}{appID}

	if env, ok := filters["env"].(string); ok && env != "" {
		whereClause += " AND env = ?"
		args = append(args, env)
	}
	if release, ok := filters["release"].(string); ok && release != "" {
		whereClause += " AND release = ?"
		args = append(args, release)
	}
	if startTime, ok := filters["startTime"].(int64); ok && startTime > 0 {
		whereClause += " AND created_at >= ?"
		args = append(args, startTime)
	}
	if endTime, ok := filters["endTime"].(int64); ok && endTime > 0 {
		whereClause += " AND created_at <= ?"
		args = append(args, endTime)
	}

	// Calculate regression threshold (24 hours ago)
	regressionThreshold := time.Now().Add(-24 * time.Hour).UnixMilli()

	var selectField, groupField string
	switch topType {
	case "errors":
		whereClause += " AND level = 'error'"
		selectField = "message"
		groupField = "message"
	case "pages":
		selectField = "url"
		groupField = "url"
	case "releases":
		selectField = "release"
		groupField = "release"
	case "browsers":
		selectField = "ua"
		groupField = "ua"
	default:
		return nil, fmt.Errorf("invalid top type: %s", topType)
	}

	// Build ORDER BY clause
	orderClause := "ORDER BY "
	switch orderBy {
	case "count":
		orderClause += "count DESC"
	case "impact":
		orderClause += "impact_score DESC"
	case "regression":
		orderClause += "is_new DESC, first_seen DESC"
	case "lastSeen":
		orderClause += "last_seen DESC"
	default:
		orderClause += "count DESC"
	}

	query := fmt.Sprintf(`
		SELECT
			%s as key,
			COUNT(*) as count,
			COUNT(DISTINCT user_id) as users,
			MAX(created_at) as last_seen,
			MIN(created_at) as first_seen,
			MIN(created_at) >= ? as is_new,
			COUNT(*) * COUNT(DISTINCT user_id) as impact_score
		FROM events
		%s
		GROUP BY %s
		%s
		LIMIT ?
	`, selectField, whereClause, groupField, orderClause)

	args = append(args, regressionThreshold, limit)

	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query top N: %w", err)
	}
	defer rows.Close()

	var items []TopNItem
	for rows.Next() {
		var item TopNItem
		var isNewInt int64

		err := rows.Scan(&item.Key, &item.Count, &item.Users, &item.LastSeen, &item.FirstSeen, &isNewInt, &item.ImpactScore)
		if err != nil {
			return nil, fmt.Errorf("failed to scan top N item: %w", err)
		}
		item.IsNew = isNewInt != 0
		items = append(items, item)
	}

	return &TopNResult{
		Type: topType,
		Data: items,
	}, nil
}

// GetSimilarErrors finds errors similar to the given message
// This is a wrapper around GetErrorClusters that adds additional fields
func (db *DB) GetSimilarErrors(appID, message string, threshold float64, limit int) ([]ErrorCluster, error) {
	clusters, err := db.GetErrorClusters(appID, message, threshold, limit)
	if err != nil {
		return nil, err
	}

	// Add additional fields for compatibility
	for i := range clusters {
		clusters[i].ID = clusters[i].ClusterID
		clusters[i].Users = clusters[i].AffectedUsers
	}

	return clusters, nil
}
