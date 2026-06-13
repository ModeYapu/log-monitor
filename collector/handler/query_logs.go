package handler

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/logmonitor/collector/middleware"
	"github.com/logmonitor/collector/storage"
)

// QueryHandler handles log query requests

func (h *QueryHandler) QueryLogs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Parse query parameters
	query := storage.QueryParams{
		AppID:    r.URL.Query().Get("appId"),
		Release:  r.URL.Query().Get("release"),
		Env:      r.URL.Query().Get("env"),
		Type:     r.URL.Query().Get("type"),
		Level:    r.URL.Query().Get("level"),
		Keyword:  r.URL.Query().Get("keyword"),
		Page:     parseIntParam(r.URL.Query().Get("page"), 1),
		PageSize: parseIntParam(r.URL.Query().Get("pageSize"), 50),
	}

	// Extract project_id from context for data isolation
	projectID := middleware.GetProjectIDFromContext(r)
	if projectID > 0 {
		query.ProjectID = projectID
	}

	// Validate required params
	if query.AppID == "" {
		http.Error(w, "Missing appId parameter", http.StatusBadRequest)
		return
	}

	// Parse time range
	if startTime := r.URL.Query().Get("startTime"); startTime != "" {
		if ts, err := strconv.ParseInt(startTime, 10, 64); err == nil {
			query.StartTime = ts
		}
	}
	if endTime := r.URL.Query().Get("endTime"); endTime != "" {
		if ts, err := strconv.ParseInt(endTime, 10, 64); err == nil {
			query.EndTime = ts
		}
	}

	// Clamp page size
	if query.PageSize > 500 {
		query.PageSize = 500
	}
	if query.PageSize < 1 {
		query.PageSize = 50
	}
	if query.Page < 1 {
		query.Page = 1
	}

	// Query database
	result, err := h.db.QueryEvents(query)
	if err != nil {
		slog.Error("Failed to query logs", "error", err)
		http.Error(w, "Failed to query logs", http.StatusInternalServerError)
		return
	}

	// Add screenshot URLs to events
	dataWithScreenshots := make([]map[string]interface{}, 0, len(result.Data))
	for _, event := range result.Data {
		eventMap := h.eventToMap(event)
		dataWithScreenshots = append(dataWithScreenshots, eventMap)
	}

	// Convert to response format
	json.NewEncoder(w).Encode(storage.LogsResponse{
		Total: result.Total,
		Page:  result.Page,
		Size:  result.Size,
		Data:  dataWithScreenshots,
	})
}

// QueryStats returns statistics for an app
func (h *QueryHandler) QueryStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	appID := r.URL.Query().Get("appId")
	if appID == "" {
		http.Error(w, "Missing appId parameter", http.StatusBadRequest)
		return
	}

	// Extract project_id from context for data isolation
	projectID := middleware.GetProjectIDFromContext(r)

	stats, err := h.db.GetStats(appID, projectID)
	if err != nil {
		slog.Error("Failed to get stats", "error", err)
		http.Error(w, "Failed to get stats", http.StatusInternalServerError)
		return
	}

	// Generate 24h error trend
	trend := h.generateErrorTrend(appID, 24*time.Hour)

	json.NewEncoder(w).Encode(storage.StatsResponse{
		TotalEvents: stats.TotalEvents,
		ErrorCount:  stats.ErrorCount,
		WarnCount:   stats.WarnCount,
		InfoCount:   stats.InfoCount,
		TopErrors:   stats.TopErrors,
		ErrorTrend:  trend,
	})
}

// QueryApps returns list of all apps
func (h *QueryHandler) QueryApps(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Extract project_id from context for data isolation
	projectID := middleware.GetProjectIDFromContext(r)

	apps, err := h.db.GetApps(projectID)
	if err != nil {
		slog.Error("Failed to get apps", "error", err)
		http.Error(w, "Failed to get apps", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(apps)
}

// QueryTop handles top N queries for errors/pages/releases/browsers
func (h *QueryHandler) QueryTop(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Parse query parameters
	appID := r.URL.Query().Get("appId")
	topType := r.URL.Query().Get("type")    // errors|pages|releases|browsers
	orderBy := r.URL.Query().Get("orderBy") // count|users|impact|recent|regression
	limit := parseIntParam(r.URL.Query().Get("limit"), 20)

	// Validate required params
	if appID == "" {
		http.Error(w, "Missing appId parameter", http.StatusBadRequest)
		return
	}
	if topType == "" {
		topType = "errors"
	}

	// Map topType to groupBy field
	// Parse time range
	var startTime, endTime int64
	if startTimeStr := r.URL.Query().Get("startTime"); startTimeStr != "" {
		if ts, err := strconv.ParseInt(startTimeStr, 10, 64); err == nil {
			startTime = ts
		}
	}
	if endTimeStr := r.URL.Query().Get("endTime"); endTimeStr != "" {
		if ts, err := strconv.ParseInt(endTimeStr, 10, 64); err == nil {
			endTime = ts
		}
	}

	// Build query params
	env := r.URL.Query().Get("env")
	release := r.URL.Query().Get("release")

	if orderBy == "" {
		orderBy = "count"
	}

	// Build filters
	filters := storage.AnalyticsFilters{
		Env:       env,
		Release:   release,
		StartTime: startTime,
		EndTime:   endTime,
		ProjectID: middleware.GetProjectIDFromContext(r),
	}

	// Query database
	result, err := h.db.GetTopN(appID, topType, orderBy, limit, filters)
	if err != nil {
		slog.Error("Failed to query top N", "error", err)
		http.Error(w, "Failed to query top N", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(storage.TopResponse{
		Type:  result.Type,
		Data:  result.Items,
		Total: result.Total,
	})
}

// QuerySimilar handles similar error queries
func (h *QueryHandler) QuerySimilar(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Parse query parameters
	appID := r.URL.Query().Get("appId")
	message := r.URL.Query().Get("message")
	threshold := parseFloatParam(r.URL.Query().Get("threshold"), 0.7)
	limit := parseIntParam(r.URL.Query().Get("limit"), 10)

	// Validate required params
	if appID == "" {
		http.Error(w, "Missing appId parameter", http.StatusBadRequest)
		return
	}
	if message == "" {
		http.Error(w, "Missing message parameter", http.StatusBadRequest)
		return
	}

	// Query database for error clustering
	clusters, err := h.db.GetSimilarErrors(appID, message, threshold, limit, middleware.GetProjectIDFromContext(r))
	if err != nil {
		slog.Error("Failed to query similar errors", "error", err)
		http.Error(w, "Failed to query similar errors", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(storage.SimilarResponse{
		Query:    message,
		Clusters: clusters,
	})
}

// QueryExport handles event export requests
func (h *QueryHandler) QueryExport(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	appID := r.URL.Query().Get("appId")
	exportType := r.URL.Query().Get("type")
	level := r.URL.Query().Get("level")
	release := r.URL.Query().Get("release")
	env := r.URL.Query().Get("env")
	keyword := r.URL.Query().Get("keyword")
	format := r.URL.Query().Get("format") // json|csv

	// Validate required params
	if appID == "" {
		http.Error(w, "Missing appId parameter", http.StatusBadRequest)
		return
	}
	if format == "" {
		format = "json"
	}

	// Build query params
	query := storage.QueryParams{
		AppID:     appID,
		Type:      exportType,
		Level:     level,
		Release:   release,
		Env:       env,
		Keyword:   keyword,
		Page:      1,
		PageSize:  10000, // Large limit for export
		ProjectID: middleware.GetProjectIDFromContext(r),
	}

	// Parse time range
	if startTime := r.URL.Query().Get("startTime"); startTime != "" {
		if ts, err := strconv.ParseInt(startTime, 10, 64); err == nil {
			query.StartTime = ts
		}
	}
	if endTime := r.URL.Query().Get("endTime"); endTime != "" {
		if ts, err := strconv.ParseInt(endTime, 10, 64); err == nil {
			query.EndTime = ts
		}
	}

	// Query database
	result, err := h.db.QueryEvents(query)
	if err != nil {
		slog.Error("Failed to query events for export", "error", err)
		http.Error(w, "Failed to query events", http.StatusInternalServerError)
		return
	}

	// Export based on format
	if format == "csv" {
		w.Header().Set("Content-Type", "text/csv")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=events_%s.csv", appID))
		h.exportCSV(w, result.Data)
	} else {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=events_%s.json", appID))
		h.exportJSON(w, result.Data)
	}
}

// exportCSV exports events as CSV
func (h *QueryHandler) exportCSV(w http.ResponseWriter, events []storage.EventRecord) {
	writer := csv.NewWriter(w)
	defer writer.Flush()

	// Write CSV header
	headers := []string{"ID", "AppID", "Release", "Env", "BuildID", "UserID", "SessionID",
		"Type", "Level", "Message", "Stack", "URL", "Line", "Col",
		"UA", "Screen", "Viewport", "IP", "CreatedAt"}
	if err := writer.Write(headers); err != nil {
		slog.Error("Failed to write CSV header", "error", err)
		return
	}

	for _, event := range events {
		row := []string{
			strconv.FormatInt(event.ID, 10),
			event.AppID,
			event.Release,
			event.Env,
			event.BuildID,
			event.UserID,
			event.SessionID,
			event.Type,
			event.Level,
			event.Message,
			event.Stack,
			event.URL,
			strconv.Itoa(event.Line),
			strconv.Itoa(event.Col),
			event.UA,
			event.Screen,
			event.Viewport,
			event.IP,
			strconv.FormatInt(event.CreatedAt, 10),
		}
		if err := writer.Write(row); err != nil {
			slog.Error("Failed to write CSV row", "error", err)
		}
	}
}

// exportJSON exports events as JSON
func (h *QueryHandler) exportJSON(w http.ResponseWriter, events []storage.EventRecord) {
	data := make([]map[string]interface{}, 0, len(events))
	for _, event := range events {
		data = append(data, map[string]interface{}{
			"id":          event.ID,
			"appId":       event.AppID,
			"release":     event.Release,
			"env":         event.Env,
			"buildId":     event.BuildID,
			"userId":      event.UserID,
			"sessionId":   event.SessionID,
			"type":        event.Type,
			"level":       event.Level,
			"message":     event.Message,
			"stack":       event.Stack,
			"url":         event.URL,
			"line":        event.Line,
			"col":         event.Col,
			"tags":        parseJSONString(event.Tags),
			"extra":       parseJSONString(event.Extra),
			"ua":          event.UA,
			"screen":      event.Screen,
			"viewport":    event.Viewport,
			"performance": parseJSONString(event.Performance),
			"ip":          event.IP,
			"timestamp":   event.CreatedAt,
		})
	}
	json.NewEncoder(w).Encode(data)
}

// Health returns health status
