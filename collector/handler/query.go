package handler

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/logmonitor/collector/storage"
)

// QueryHandler handles log query requests
type QueryHandler struct {
	db            *storage.DB
	screenshotDir string
}

// NewQueryHandler creates a new query handler
func NewQueryHandler(db *storage.DB) *QueryHandler {
	return &QueryHandler{
		db:            db,
		screenshotDir: "./data/screenshots",
	}
}

// RegisterRoutes registers all query routes
func (h *QueryHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/query/logs", h.QueryLogs)
	mux.HandleFunc("GET /api/query/stats", h.QueryStats)
	mux.HandleFunc("GET /api/query/apps", h.QueryApps)
	mux.HandleFunc("GET /api/query/top", h.QueryTop)
	mux.HandleFunc("GET /api/query/similar", h.QuerySimilar)
	mux.HandleFunc("GET /api/query/export", h.QueryExport)
	mux.HandleFunc("GET /api/health", h.Health)
}

// QueryLogs handles log queries
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
		log.Printf("Failed to query logs: %v", err)
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
	response := map[string]interface{}{
		"total": result.Total,
		"page":  result.Page,
		"size":  result.Size,
		"data":  dataWithScreenshots,
	}

	json.NewEncoder(w).Encode(response)
}

// QueryStats returns statistics for an app
func (h *QueryHandler) QueryStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	appID := r.URL.Query().Get("appId")
	if appID == "" {
		http.Error(w, "Missing appId parameter", http.StatusBadRequest)
		return
	}

	stats, err := h.db.GetStats(appID)
	if err != nil {
		log.Printf("Failed to get stats: %v", err)
		http.Error(w, "Failed to get stats", http.StatusInternalServerError)
		return
	}

	// Generate 24h error trend
	trend := h.generateErrorTrend(appID, 24*time.Hour)

	response := map[string]interface{}{
		"totalEvents": stats.TotalEvents,
		"errorCount":  stats.ErrorCount,
		"warnCount":   stats.WarnCount,
		"infoCount":   stats.InfoCount,
		"topErrors":   stats.TopErrors,
		"errorTrend":  trend,
	}

	json.NewEncoder(w).Encode(response)
}

// QueryApps returns list of all apps
func (h *QueryHandler) QueryApps(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	apps, err := h.db.GetApps()
	if err != nil {
		log.Printf("Failed to get apps: %v", err)
		http.Error(w, "Failed to get apps", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(apps)
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
		AppID:    appID,
		Type:     exportType,
		Level:    level,
		Release:  release,
		Env:      env,
		Keyword:  keyword,
		Page:     1,
		PageSize: 10000, // Large limit for export
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
		log.Printf("Failed to query events for export: %v", err)
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
		log.Printf("Failed to write CSV header: %v", err)
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
			log.Printf("Failed to write CSV row: %v", err)
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
func (h *QueryHandler) Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "ok",
		"time":   time.Now().UnixMilli(),
	})
}

// generateErrorTrend generates error trend data for the specified duration
func (h *QueryHandler) generateErrorTrend(appID string, duration time.Duration) []map[string]interface{} {
	points := make([]map[string]interface{}, 0)

	now := time.Now()
	bucketSize := duration / 24 // Divide into 24 buckets

	for i := 0; i < 24; i++ {
		endTime := now.Add(-time.Duration(i) * bucketSize)
		startTime := endTime.Add(-bucketSize)

		query := storage.QueryParams{
			AppID:     appID,
			Level:     "error",
			StartTime: startTime.UnixMilli(),
			EndTime:   endTime.UnixMilli(),
			Page:      1,
			PageSize:  1,
		}

		result, err := h.db.QueryEvents(query)
		count := int64(0)
		if err == nil {
			count = result.Total
		}

		points = append([]map[string]interface{}{
			{
				"timestamp": startTime.UnixMilli(),
				"count":     count,
			},
		}, points...)
	}

	return points
}

// parseIntParam parses an integer parameter with a default value
func parseIntParam(s string, defaultValue int) int {
	if s == "" {
		return defaultValue
	}
	val, err := strconv.Atoi(s)
	if err != nil {
		return defaultValue
	}
	return val
}

// parseFloatParam parses a float parameter with a default value
func parseFloatParam(s string, defaultValue float64) float64 {
	if s == "" {
		return defaultValue
	}
	val, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return defaultValue
	}
	return val
}

// eventToMap converts storage event record to map with screenshot URL
func (h *QueryHandler) eventToMap(event storage.EventRecord) map[string]interface{} {
	result := map[string]interface{}{
		"id":          0, // placeholder
		"app_id":      event.AppID,
		"release":     event.Release,
		"env":         event.Env,
		"build_id":    event.BuildID,
		"user_id":     event.UserID,
		"session_id":  event.SessionID,
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
	}

	// Check for screenshot files
	if !safePathSegment(event.AppID) {
		return result
	}

	screenshotPath, err := safeJoinUnderBase(h.screenshotDir, event.AppID)
	if err != nil {
		return result
	}

	if entries, err := os.ReadDir(screenshotPath); err == nil {
		// Look for screenshot files that match the event timestamp
		// Screenshots are named {eventId}.png where eventId contains timestamp
		for _, entry := range entries {
			if !entry.IsDir() && filepath.Ext(entry.Name()) == ".png" {
				if len(entry.Name()) < 10 {
					continue
				}
				// Extract timestamp from filename to match with event
				eventTime := event.CreatedAt
				fileTimeStr := entry.Name()[:10]
				var fileTime int64
				if _, err := fmt.Sscanf(fileTimeStr, "%d", &fileTime); err == nil {
					// If screenshot was taken within 5 seconds of event time, consider it a match
					if abs(eventTime-fileTime) < 5000 {
						result["screenshot_url"] = "/api/screenshots/" + event.AppID + "/" + entry.Name()
						break
					}
				}
			}
		}
	}

	return result
}

func parseJSONString(s string) map[string]interface{} {
	if s == "" || s == "{}" {
		return make(map[string]interface{})
	}
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(s), &result); err != nil {
		return make(map[string]interface{})
	}
	return result
}

func abs(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}
