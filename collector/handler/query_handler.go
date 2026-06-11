package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/logmonitor/collector/storage"
)

// QueryHandler handles log query requests

type QueryHandler struct {
	eventStore     storage.EventStore
	analyticsStore storage.AnalyticsStore
	db             *storage.DB // Keep for legacy methods
	screenshotDir  string
}

// NewQueryHandler creates a new query handler
func NewQueryHandler(db *storage.DB) *QueryHandler {
	return &QueryHandler{
		eventStore:     db,
		analyticsStore: db,
		db:             db, // Keep for backward compatibility
		screenshotDir:  "./data/screenshots",
	}
}

// NewQueryHandlerWithStores creates a new query handler with explicit stores
func NewQueryHandlerWithStores(eventStore storage.EventStore, analyticsStore storage.AnalyticsStore, db *storage.DB) *QueryHandler {
	return &QueryHandler{
		eventStore:     eventStore,
		analyticsStore: analyticsStore,
		db:             db,
		screenshotDir:  "./data/screenshots",
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

	// Performance endpoints
	mux.HandleFunc("GET /api/query/performance/summary", h.QueryPerformanceSummary)
	mux.HandleFunc("GET /api/query/performance/trend", h.QueryPerformanceTrend)
	mux.HandleFunc("GET /api/query/performance/pages", h.QueryPerformancePages)
	mux.HandleFunc("GET /api/query/performance/regression", h.QueryPerformanceRegression)
}

// QueryLogs handles log queries

func (h *QueryHandler) Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "ok",
		"time":   time.Now().UnixMilli(),
	})
}

// generateErrorTrend generates error trend data for the specified duration

func (h *QueryHandler) generateErrorTrend(appID string, duration time.Duration) []storage.ErrorTrendPoint {
	points := make([]storage.ErrorTrendPoint, 0)

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

		points = append([]storage.ErrorTrendPoint{
			{
				Timestamp: startTime.UnixMilli(),
				Count:     count,
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
