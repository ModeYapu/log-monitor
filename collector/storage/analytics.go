package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"time"

)

// GetTopN retrieves top N items grouped by type (errors/pages/releases/browsers)
func (db *DB) GetTopN(appID, topType, orderBy string, limit int, filters AnalyticsFilters) (*TopNResult, error) {

	if db.closed.Load() {
		return nil, fmt.Errorf("database is closed")
	}

	if limit <= 0 || limit > 100 {
		limit = 20
	}

	// Build WHERE clause
	whereClause := "WHERE app_id = ?"
	args := []interface{}{appID}

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
		Type:  topType,
		Items: items,
		Total: int64(len(items)),
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

// PerformanceMetricsSummary represents Web Vitals summary with grades
type PerformanceMetricsSummary struct {
	FCP PerformanceMetric `json:"fcp"`
	LCP PerformanceMetric `json:"lcp"`
	CLS PerformanceMetric `json:"cls"`
	INP PerformanceMetric `json:"inp"`
	TTFB PerformanceMetric `json:"ttfb"`
}

// PerformanceMetric represents a single performance metric with grade
type PerformanceMetric struct {
	P75  float64 `json:"p75"`
	Grade string  `json:"grade"` // good|needs-improvement|poor
}

// PerformanceTrendData represents time-series performance data
type PerformanceTrendData struct {
	Timestamp int64   `json:"timestamp"`
	Value     float64 `json:"value"`
	Count     int     `json:"count"`
}

// PagePerformanceData represents page-level performance
type PagePerformanceData struct {
	URL             string                      `json:"url"`
	FCP_P75         float64                     `json:"fcp_p75"`
	LCP_P75         float64                     `json:"lcp_p75"`
	CLS_P75         float64                     `json:"cls_p75"`
	INP_P75         float64                     `json:"inp_p75"`
	TTFB_P75        float64                     `json:"ttfb_p75"`
	Samples         int64                       `json:"samples"`
	PreviousPeriod  *PagePerformanceComparison  `json:"previous_period,omitempty"`
}

// PagePerformanceComparison represents comparison with previous period
type PagePerformanceComparison struct {
	FCP_Change  float64 `json:"fcp_change,omitempty"`
	LCP_Change  float64 `json:"lcp_change,omitempty"`
	CLS_Change  float64 `json:"cls_change,omitempty"`
	INP_Change  float64 `json:"inp_change,omitempty"`
	TTFB_Change float64 `json:"ttfb_change,omitempty"`
}

// PerformanceRegression represents performance regression detection
type PerformanceRegression struct {
	URL           string  `json:"url"`
	Metric        string  `json:"metric"` // fcp|lcp|cls|inp|ttfb
	CurrentValue  float64 `json:"current_value"`
	PreviousValue float64 `json:"previous_value"`
	ChangePercent float64 `json:"change_percent"`
	Grade         string  `json:"grade"`
}

// GetPerformanceSummary returns Web Vitals P75 metrics with grades
func (db *DB) GetPerformanceSummary(appID string, timeRange string) (*PerformanceMetricsSummary, error) {

	if db.closed.Load() {
		return nil, fmt.Errorf("database is closed")
	}

	// Calculate time range
	now := time.Now()
	var startTime time.Time
	switch timeRange {
	case "24h":
		startTime = now.Add(-24 * time.Hour)
	case "7d":
		startTime = now.Add(-7 * 24 * time.Hour)
	case "30d":
		startTime = now.Add(-30 * 24 * time.Hour)
	default:
		startTime = now.Add(-24 * time.Hour)
	}

	// Query performance events
	query := `SELECT performance FROM events
	          WHERE app_id = ? AND type = 'performance'
	          AND created_at >= ? AND created_at <= ?`

	rows, err := db.conn.Query(query, appID, startTime.UnixMilli(), now.UnixMilli())
	if err != nil {
		return nil, fmt.Errorf("failed to query performance events: %w", err)
	}
	defer rows.Close()

	// Collect metric values
	fcpValues := []float64{}
	lcpValues := []float64{}
	clsValues := []float64{}
	inpValues := []float64{}
	ttfbValues := []float64{}

	for rows.Next() {
		var performanceJSON string
		if err := rows.Scan(&performanceJSON); err != nil {
			continue
		}

		var perfData map[string]interface{}
		if err := json.Unmarshal([]byte(performanceJSON), &perfData); err != nil {
			continue
		}

		// Extract metric values
		if v, ok := perfData["fcp"].(float64); ok && v > 0 {
			fcpValues = append(fcpValues, v)
		}
		if v, ok := perfData["lcp"].(float64); ok && v > 0 {
			lcpValues = append(lcpValues, v)
		}
		if v, ok := perfData["cls"].(float64); ok && v > 0 {
			clsValues = append(clsValues, v)
		}
		if v, ok := perfData["inp"].(float64); ok && v > 0 {
			inpValues = append(inpValues, v)
		}
		if v, ok := perfData["ttfb"].(float64); ok && v > 0 {
			ttfbValues = append(ttfbValues, v)
		}
	}

	// Calculate P75 and grades
	return &PerformanceMetricsSummary{
		FCP: PerformanceMetric{
			P75:  calculateP75(fcpValues),
			Grade: getWebVitalsGrade("fcp", calculateP75(fcpValues)),
		},
		LCP: PerformanceMetric{
			P75:  calculateP75(lcpValues),
			Grade: getWebVitalsGrade("lcp", calculateP75(lcpValues)),
		},
		CLS: PerformanceMetric{
			P75:  calculateP75(clsValues),
			Grade: getWebVitalsGrade("cls", calculateP75(clsValues)),
		},
		INP: PerformanceMetric{
			P75:  calculateP75(inpValues),
			Grade: getWebVitalsGrade("inp", calculateP75(inpValues)),
		},
		TTFB: PerformanceMetric{
			P75:  calculateP75(ttfbValues),
			Grade: getWebVitalsGrade("ttfb", calculateP75(ttfbValues)),
		},
	}, nil
}

// GetPerformanceTrend returns time-series performance data for a specific metric
func (db *DB) GetPerformanceTrend(appID, metric, granularity string) ([]PerformanceTrendData, error) {

	if db.closed.Load() {
		return nil, fmt.Errorf("database is closed")
	}

	// Calculate bucket size and time range
	now := time.Now()
	var bucketSize time.Duration
	var buckets int

	switch granularity {
	case "1h":
		bucketSize = 1 * time.Hour
		buckets = 24
	case "6h":
		bucketSize = 6 * time.Hour
		buckets = 28 // 7 days
	case "1d":
		bucketSize = 24 * time.Hour
		buckets = 30
	default:
		bucketSize = 1 * time.Hour
		buckets = 24
	}

	trendData := []PerformanceTrendData{}

	// Query data for each time bucket
	for i := 0; i < buckets; i++ {
		endTime := now.Add(-time.Duration(i) * bucketSize)
		startTime := endTime.Add(-bucketSize)

		query := `SELECT performance FROM events
		          WHERE app_id = ? AND type = 'performance'
		          AND created_at >= ? AND created_at <= ?`

		rows, err := db.conn.Query(query, appID, startTime.UnixMilli(), endTime.UnixMilli())
		if err != nil {
			continue
		}

		values := []float64{}
		for rows.Next() {
			var performanceJSON string
			if err := rows.Scan(&performanceJSON); err != nil {
				continue
			}

			var perfData map[string]interface{}
			if err := json.Unmarshal([]byte(performanceJSON), &perfData); err != nil {
				continue
			}

			if v, ok := perfData[metric].(float64); ok && v > 0 {
				values = append(values, v)
			}
		}
		rows.Close()

		if len(values) > 0 {
			trendData = append([]PerformanceTrendData{{
				Timestamp: startTime.UnixMilli(),
				Value:     calculateP75(values),
				Count:     len(values),
			}}, trendData...)
		}
	}

	return trendData, nil
}

// GetPagePerformanceRanking returns page-level performance ranking
func (db *DB) GetPagePerformanceRanking(appID, timeRange string) ([]PagePerformanceData, error) {

	if db.closed.Load() {
		return nil, fmt.Errorf("database is closed")
	}

	// Calculate time range
	now := time.Now()
	var startTime time.Time
	switch timeRange {
	case "24h":
		startTime = now.Add(-24 * time.Hour)
	case "7d":
		startTime = now.Add(-7 * 24 * time.Hour)
	case "30d":
		startTime = now.Add(-30 * 24 * time.Hour)
	default:
		startTime = now.Add(-24 * time.Hour)
	}

	// Query current period data
	currentQuery := `SELECT url, performance FROM events
	                 WHERE app_id = ? AND type = 'performance'
	                 AND created_at >= ? AND created_at <= ?
	                 AND url IS NOT NULL AND url != ''`

	currentRows, err := db.conn.Query(currentQuery, appID, startTime.UnixMilli(), now.UnixMilli())
	if err != nil {
		return nil, fmt.Errorf("failed to query current period: %w", err)
	}
	defer currentRows.Close()

	// Collect page-level metrics
	pageMetrics := make(map[string]*PagePerformanceData)

	for currentRows.Next() {
		var url, performanceJSON string
		if err := currentRows.Scan(&url, &performanceJSON); err != nil {
			continue
		}

		var perfData map[string]interface{}
		if err := json.Unmarshal([]byte(performanceJSON), &perfData); err != nil {
			continue
		}

		if _, exists := pageMetrics[url]; !exists {
			pageMetrics[url] = &PagePerformanceData{
				URL:      url,
				Samples:  0,
			}
		}

		page := pageMetrics[url]
		page.Samples++
	}

	currentRows.Close()

	// Query previous period data for comparison
	previousStartTime := startTime.Add(-now.Sub(startTime))
	previousQuery := `SELECT url, performance FROM events
	                   WHERE app_id = ? AND type = 'performance'
	                   AND created_at >= ? AND created_at <= ?
	                   AND url IS NOT NULL AND url != ''`

	previousRows, err := db.conn.Query(previousQuery, appID, previousStartTime.UnixMilli(), startTime.UnixMilli())
	if err != nil {
		return nil, fmt.Errorf("failed to query previous period: %w", err)
	}
	defer previousRows.Close()

	// Collect previous period data
	previousPageMetrics := make(map[string][]float64)

	for previousRows.Next() {
		var url, performanceJSON string
		if err := previousRows.Scan(&url, &performanceJSON); err != nil {
			continue
		}

		var perfData map[string]interface{}
		if err := json.Unmarshal([]byte(performanceJSON), &perfData); err != nil {
			continue
		}

		if _, exists := previousPageMetrics[url]; !exists {
			previousPageMetrics[url] = []float64{}
		}

		// Store LCP values for comparison
		if v, ok := perfData["lcp"].(float64); ok && v > 0 {
			previousPageMetrics[url] = append(previousPageMetrics[url], v)
		}
	}

	// Build final result with calculated P75 values
	result := []PagePerformanceData{}
	for url, page := range pageMetrics {
		// Query detailed metrics for this page
		pageQuery := `SELECT performance FROM events
		             WHERE app_id = ? AND type = 'performance'
		             AND created_at >= ? AND created_at <= ?
		             AND url = ?`

		pageRows, err := db.conn.Query(pageQuery, appID, startTime.UnixMilli(), now.UnixMilli(), url)
		if err != nil {
			continue
		}

		fcpValues := []float64{}
		lcpValues := []float64{}
		clsValues := []float64{}
		inpValues := []float64{}
		ttfbValues := []float64{}

		for pageRows.Next() {
			var performanceJSON string
			if err := pageRows.Scan(&performanceJSON); err != nil {
				continue
			}

			var perfData map[string]interface{}
			if err := json.Unmarshal([]byte(performanceJSON), &perfData); err != nil {
				continue
			}

			if v, ok := perfData["fcp"].(float64); ok && v > 0 {
				fcpValues = append(fcpValues, v)
			}
			if v, ok := perfData["lcp"].(float64); ok && v > 0 {
				lcpValues = append(lcpValues, v)
			}
			if v, ok := perfData["cls"].(float64); ok && v > 0 {
				clsValues = append(clsValues, v)
			}
			if v, ok := perfData["inp"].(float64); ok && v > 0 {
				inpValues = append(inpValues, v)
			}
			if v, ok := perfData["ttfb"].(float64); ok && v > 0 {
				ttfbValues = append(ttfbValues, v)
			}
		}
		pageRows.Close()

		page.FCP_P75 = calculateP75(fcpValues)
		page.LCP_P75 = calculateP75(lcpValues)
		page.CLS_P75 = calculateP75(clsValues)
		page.INP_P75 = calculateP75(inpValues)
		page.TTFB_P75 = calculateP75(ttfbValues)

		// Add comparison data if available
		if prevValues, exists := previousPageMetrics[url]; exists && len(prevValues) > 0 {
			prevLCP := calculateP75(prevValues)
			if prevLCP > 0 && page.LCP_P75 > 0 {
				change := ((page.LCP_P75 - prevLCP) / prevLCP) * 100
				page.PreviousPeriod = &PagePerformanceComparison{
					LCP_Change: change,
				}
			}
		}

		result = append(result, *page)
	}

	// Sort by LCP P75 descending
	sort.Slice(result, func(i, j int) bool {
		return result[i].LCP_P75 > result[j].LCP_P75
	})

	return result, nil
}

// GetPerformanceRegressions detects pages where metrics worsened >20%
func (db *DB) GetPerformanceRegressions(appID string) ([]PerformanceRegression, error) {

	if db.closed.Load() {
		return nil, fmt.Errorf("database is closed")
	}

	now := time.Now()
	// Current period: last 24 hours
	currentStartTime := now.Add(-24 * time.Hour)
	// Previous period: 24-48 hours ago
	previousStartTime := now.Add(-48 * time.Hour)
	previousEndTime := currentStartTime

	// Query current period data
	currentQuery := `SELECT url, performance FROM events
	                 WHERE app_id = ? AND type = 'performance'
	                 AND created_at >= ? AND created_at <= ?
	                 AND url IS NOT NULL AND url != ''`

	currentRows, err := db.conn.Query(currentQuery, appID, currentStartTime.UnixMilli(), now.UnixMilli())
	if err != nil {
		return nil, fmt.Errorf("failed to query current period: %w", err)
	}
	defer currentRows.Close()

	// Collect current period metrics by page and metric
	currentMetrics := make(map[string]map[string][]float64) // url -> metric -> values

	for currentRows.Next() {
		var url, performanceJSON string
		if err := currentRows.Scan(&url, &performanceJSON); err != nil {
			continue
		}

		var perfData map[string]interface{}
		if err := json.Unmarshal([]byte(performanceJSON), &perfData); err != nil {
			continue
		}

		if _, exists := currentMetrics[url]; !exists {
			currentMetrics[url] = map[string][]float64{
				"fcp": {}, "lcp": {}, "cls": {}, "inp": {}, "ttfb": {},
			}
		}

		metrics := []string{"fcp", "lcp", "cls", "inp", "ttfb"}
		for _, metric := range metrics {
			if v, ok := perfData[metric].(float64); ok && v > 0 {
				currentMetrics[url][metric] = append(currentMetrics[url][metric], v)
			}
		}
	}

	currentRows.Close()

	// Query previous period data
	previousQuery := `SELECT url, performance FROM events
	                  WHERE app_id = ? AND type = 'performance'
	                  AND created_at >= ? AND created_at <= ?
	                  AND url IS NOT NULL AND url != ''`

	previousRows, err := db.conn.Query(previousQuery, appID, previousStartTime.UnixMilli(), previousEndTime.UnixMilli())
	if err != nil {
		return nil, fmt.Errorf("failed to query previous period: %w", err)
	}
	defer previousRows.Close()

	// Collect previous period metrics by page and metric
	previousMetrics := make(map[string]map[string][]float64) // url -> metric -> values

	for previousRows.Next() {
		var url, performanceJSON string
		if err := previousRows.Scan(&url, &performanceJSON); err != nil {
			continue
		}

		var perfData map[string]interface{}
		if err := json.Unmarshal([]byte(performanceJSON), &perfData); err != nil {
			continue
		}

		if _, exists := previousMetrics[url]; !exists {
			previousMetrics[url] = map[string][]float64{
				"fcp": {}, "lcp": {}, "cls": {}, "inp": {}, "ttfb": {},
			}
		}

		metrics := []string{"fcp", "lcp", "cls", "inp", "ttfb"}
		for _, metric := range metrics {
			if v, ok := perfData[metric].(float64); ok && v > 0 {
				previousMetrics[url][metric] = append(previousMetrics[url][metric], v)
			}
		}
	}

	previousRows.Close()

	// Detect regressions (>20% worsening)
	regressions := []PerformanceRegression{}

	for url, currentData := range currentMetrics {
		previousData, hasPrevious := previousMetrics[url]
		if !hasPrevious {
			continue
		}

		metrics := []string{"fcp", "lcp", "cls", "inp", "ttfb"}
		for _, metric := range metrics {
			currentValues := currentData[metric]
			previousValues := previousData[metric]

			if len(currentValues) == 0 || len(previousValues) == 0 {
				continue
			}

			currentP75 := calculateP75(currentValues)
			previousP75 := calculateP75(previousValues)

			if currentP75 <= 0 || previousP75 <= 0 {
				continue
			}

			changePercent := ((currentP75 - previousP75) / previousP75) * 100

			// Check if regression (>20% worsening)
			if changePercent > 20 {
				regressions = append(regressions, PerformanceRegression{
					URL:           url,
					Metric:        metric,
					CurrentValue:  currentP75,
					PreviousValue: previousP75,
					ChangePercent: changePercent,
					Grade:         getWebVitalsGrade(metric, currentP75),
				})
			}
		}
	}

	// Sort by change percent descending
	sort.Slice(regressions, func(i, j int) bool {
		return regressions[i].ChangePercent > regressions[j].ChangePercent
	})

	return regressions, nil
}

// calculateP75 calculates 75th percentile
func calculateP75(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	sortedValues := make([]float64, len(values))
	copy(sortedValues, values)
	sort.Float64s(sortedValues)

	index := int(float64(len(sortedValues)-1) * 0.75)
	return sortedValues[index]
}

// getWebVitalsGrade returns the grade for a Web Vitals metric
func getWebVitalsGrade(metric string, value float64) string {
	thresholds := map[string]struct {
		good            float64
		needsImprovement float64
	}{
		"fcp":  {good: 1800, needsImprovement: 3000},
		"lcp":  {good: 2500, needsImprovement: 4000},
		"cls":  {good: 0.1, needsImprovement: 0.25},
		"inp":  {good: 200, needsImprovement: 500},
		"ttfb": {good: 800, needsImprovement: 1800},
	}

	t, exists := thresholds[metric]
	if !exists {
		return "unknown"
	}

	if value <= t.good {
		return "good"
	}
	if value <= t.needsImprovement {
		return "needs-improvement"
	}
	return "poor"
}

// NewError represents an error that recently appeared
type NewError struct {
	Message      string `json:"message"`
	Count        int64  `json:"count"`
	FirstSeen    int64  `json:"first_seen"`
	LastSeen     int64  `json:"last_seen"`
	AffectedUsers int64 `json:"affected_users"`
}

// GetNewErrors returns errors that first appeared in the last N minutes
func (db *DB) GetNewErrors(appID string, sinceMinutes int) ([]NewError, error) {

	if db.closed.Load() {
		return nil, fmt.Errorf("database is closed")
	}

	now := time.Now()
	startTime := now.Add(-time.Duration(sinceMinutes) * time.Minute).UnixMilli()

	query := `SELECT message, COUNT(*) as count, COUNT(DISTINCT user_id) as affected_users,
	          MIN(created_at) as first_seen, MAX(created_at) as last_seen
	          FROM events
	          WHERE app_id = ? AND level = 'error' AND created_at >= ?
	          GROUP BY message
	          HAVING MIN(created_at) >= ?
	          ORDER BY first_seen DESC, count DESC
	          LIMIT 20`

	rows, err := db.conn.Query(query, appID, startTime, startTime)
	if err != nil {
		return nil, fmt.Errorf("failed to query new errors: %w", err)
	}
	defer rows.Close()

	var newErrors []NewError
	for rows.Next() {
		var newError NewError
		err := rows.Scan(&newError.Message, &newError.Count, &newError.AffectedUsers, &newError.FirstSeen, &newError.LastSeen)
		if err != nil {
			return nil, fmt.Errorf("failed to scan new error: %w", err)
		}
		newErrors = append(newErrors, newError)
	}

	return newErrors, nil
}

// AlertTrigger represents a triggered alert event
type AlertTrigger struct {
	ID         int64  `json:"id"`
	AlertID    int64  `json:"alert_id"`
	AlertName  string `json:"alert_name"`
	Severity   string `json:"severity"`
	TriggeredAt int64 `json:"triggered_at"`
	Message    string `json:"message"`
}

// GetRecentAlertTriggers returns the last N triggered alerts
func (db *DB) GetRecentAlertTriggers(limit int) ([]AlertTrigger, error) {

	if db.closed.Load() {
		return nil, fmt.Errorf("database is closed")
	}

	if limit <= 0 || limit > 100 {
		limit = 10
	}

	query := `SELECT al.id, al.alert_id, ar.name as alert_name, ar.severity,
	          al.triggered_at, al.message
	          FROM alert_logs al
	          JOIN alert_rules ar ON al.alert_id = ar.id
	          ORDER BY al.triggered_at DESC
	          LIMIT ?`

	rows, err := db.conn.Query(query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query alert triggers: %w", err)
	}
	defer rows.Close()

	var triggers []AlertTrigger
	for rows.Next() {
		var trigger AlertTrigger
		var alertName, severity, message sql.NullString

		err := rows.Scan(&trigger.ID, &trigger.AlertID, &alertName, &severity, &trigger.TriggeredAt, &message)
		if err != nil {
			return nil, fmt.Errorf("failed to scan alert trigger: %w", err)
		}

		if alertName.Valid {
			trigger.AlertName = alertName.String
		}
		if severity.Valid {
			trigger.Severity = severity.String
		}
		if message.Valid {
			trigger.Message = message.String
		}

		triggers = append(triggers, trigger)
	}

	return triggers, nil
}

// ActiveSession represents an active user session
type ActiveSession struct {
	SessionID    string `json:"session_id"`
	URL         string `json:"url"`
	EventCount  int64  `json:"event_count"`
	LastActivity int64 `json:"last_activity"`
	UserID      string `json:"user_id"`
}

// GetActiveSessions returns recent active sessions
func (db *DB) GetActiveSessions(appID string, limit int) ([]ActiveSession, error) {

	if db.closed.Load() {
		return nil, fmt.Errorf("database is closed")
	}

	if limit <= 0 || limit > 50 {
		limit = 10
	}

	// Consider sessions active in last 24 hours
	activeThreshold := time.Now().Add(-24 * time.Hour).UnixMilli()

	query := `SELECT session_id, MAX(url) as url, COUNT(*) as event_count,
	          MAX(created_at) as last_activity, MAX(user_id) as user_id
	          FROM events
	          WHERE app_id = ? AND session_id IS NOT NULL AND session_id != ''
	          AND created_at >= ?
	          GROUP BY session_id
	          ORDER BY last_activity DESC
	          LIMIT ?`

	rows, err := db.conn.Query(query, appID, activeThreshold, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query active sessions: %w", err)
	}
	defer rows.Close()

	var sessions []ActiveSession
	for rows.Next() {
		var session ActiveSession
		var url, userID sql.NullString

		err := rows.Scan(&session.SessionID, &url, &session.EventCount, &session.LastActivity, &userID)
		if err != nil {
			return nil, fmt.Errorf("failed to scan active session: %w", err)
		}

		if url.Valid {
			session.URL = url.String
		}
		if userID.Valid {
			session.UserID = userID.String
		}

		sessions = append(sessions, session)
	}

	return sessions, nil
}

// StatsComparison represents today vs yesterday statistics comparison
type StatsComparison struct {
	TodayEvents       int64   `json:"today_events"`
	TodayErrors       int64   `json:"today_errors"`
	TodayAffectedUsers int64  `json:"today_affected_users"`

	YesterdayEvents        int64   `json:"yesterday_events"`
	YesterdayErrors        int64   `json:"yesterday_errors"`
	YesterdayAffectedUsers int64   `json:"yesterday_affected_users"`

	EventsChange        float64 `json:"events_change"`
	ErrorsChange        float64 `json:"errors_change"`
	AffectedUsersChange float64 `json:"affected_users_change"`
}

// GetStatsComparison returns today vs yesterday statistics comparison
func (db *DB) GetStatsComparison(appID string) (*StatsComparison, error) {

	if db.closed.Load() {
		return nil, fmt.Errorf("database is closed")
	}

	now := time.Now()
	// Today: from 00:00 today to now
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).UnixMilli()
	todayEnd := now.UnixMilli()

	// Yesterday: from 00:00 yesterday to 23:59:59 yesterday
	yesterdayStart := time.Date(now.Year(), now.Month(), now.Day()-1, 0, 0, 0, 0, now.Location()).UnixMilli()
	yesterdayEnd := time.Date(now.Year(), now.Month(), now.Day()-1, 23, 59, 59, 999999999, now.Location()).UnixMilli()

	// Query today's stats
	todayQuery := `SELECT COUNT(*) as total_events,
	              SUM(CASE WHEN level = 'error' THEN 1 ELSE 0 END) as error_count,
	              COUNT(DISTINCT user_id) as affected_users
	              FROM events
	              WHERE app_id = ? AND created_at >= ? AND created_at <= ?`

	var todayEvents, todayErrors, todayAffectedUsers int64
	err := db.conn.QueryRow(todayQuery, appID, todayStart, todayEnd).Scan(&todayEvents, &todayErrors, &todayAffectedUsers)
	if err != nil {
		return nil, fmt.Errorf("failed to query today stats: %w", err)
	}

	// Query yesterday's stats
	yesterdayQuery := `SELECT COUNT(*) as total_events,
	                 SUM(CASE WHEN level = 'error' THEN 1 ELSE 0 END) as error_count,
	                 COUNT(DISTINCT user_id) as affected_users
	                 FROM events
	                 WHERE app_id = ? AND created_at >= ? AND created_at <= ?`

	var yesterdayEvents, yesterdayErrors, yesterdayAffectedUsers int64
	err = db.conn.QueryRow(yesterdayQuery, appID, yesterdayStart, yesterdayEnd).Scan(&yesterdayEvents, &yesterdayErrors, &yesterdayAffectedUsers)
	if err != nil {
		return nil, fmt.Errorf("failed to query yesterday stats: %w", err)
	}

	// Calculate changes
	eventsChange := calculateChange(todayEvents, yesterdayEvents)
	errorsChange := calculateChange(todayErrors, yesterdayErrors)
	affectedUsersChange := calculateChange(todayAffectedUsers, yesterdayAffectedUsers)

	return &StatsComparison{
		TodayEvents:        todayEvents,
		TodayErrors:        todayErrors,
		TodayAffectedUsers: todayAffectedUsers,

		YesterdayEvents:        yesterdayEvents,
		YesterdayErrors:        yesterdayErrors,
		YesterdayAffectedUsers: yesterdayAffectedUsers,

		EventsChange:        eventsChange,
		ErrorsChange:        errorsChange,
		AffectedUsersChange: affectedUsersChange,
	}, nil
}

// calculateChange calculates percentage change
func calculateChange(today, yesterday int64) float64 {
	if yesterday == 0 {
		if today > 0 {
			return 100.0 // +100% if starting from 0
		}
		return 0.0 // no change if both 0
	}
	return ((float64(today) - float64(yesterday)) / float64(yesterday)) * 100.0
}

