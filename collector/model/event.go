package model

import (
	"encoding/json"
	"fmt"
)

// Event type constants - standard event types for LogMonitor
const (
	EventTypeError       = "error"
	EventTypePerformance = "performance"
	EventTypeResource    = "resource"      // Resource loading errors (scripts, stylesheets, images)
	EventTypeAPIError    = "api_error"     // API/fetch request failures
	EventTypeUserAction  = "user_action"   // User behavioral events (clicks, navigation)
	EventTypeInfo        = "info"
	EventTypeWarn        = "warn"
	EventTypeTrack       = "track"
	EventTypeConsole     = "console"
	EventTypeXHR         = "xhr"
	EventBreadcrumb      = "breadcrumb"
)

// Event represents a log event from the SDK
type Event struct {
	ID          int64                  `json:"id"`
	AppID       string                 `json:"appId"`
	Release     string                 `json:"release"`
	Env         string                 `json:"env"`       // production, staging, etc.
	BuildID     string                 `json:"buildId"`   // build identifier for source map lookup
	UserID      string                 `json:"userId"`    // user identifier
	SessionID   string                 `json:"sessionId"` // session identifier for recording association
	Type        string                 `json:"type"`      // error|performance|info|warn|track
	Level       string                 `json:"level"`
	Message     string                 `json:"message"`
	Stack       string                 `json:"stack"`
	URL         string                 `json:"url"`
	Line        int                    `json:"line"`
	Col         int                    `json:"col"`
	Tags        map[string]interface{} `json:"tags"`
	Extra       map[string]interface{} `json:"extra"`
	UA          string                 `json:"ua"`
	Screen      string                 `json:"screen"`
	Viewport    string                 `json:"viewport"`
	Performance map[string]interface{} `json:"performance"`
	IP          string                 `json:"ip"`
	CreatedAt   int64                  `json:"timestamp"` // unix ms
}

// ReportRequest is the batch report request from SDK
type ReportRequest struct {
	AppID     string  `json:"appId"`
	Release   string  `json:"release"`
	Events    []Event `json:"events"`
	ProjectID int64   `json:"projectId"` // Optional: project ID for multi-tenant support
	APIKey    string  `json:"apiKey"`    // Optional: API key to identify project
}

// LogsQuery is the query parameters for /api/query/logs
type LogsQuery struct {
	AppID     string `json:"appId"`
	Type      string `json:"type"`      // optional filter
	Level     string `json:"level"`     // optional filter
	StartTime int64  `json:"startTime"` // unix ms, optional
	EndTime   int64  `json:"endTime"`   // unix ms, optional
	Keyword   string `json:"keyword"`   // optional search in message
	Page      int    `json:"page"`      // page number, 1-indexed
	PageSize  int    `json:"pageSize"`  // items per page
}

// LogsResponse is the response for /api/query/logs
type LogsResponse struct {
	Total int64   `json:"total"`
	Page  int     `json:"page"`
	Size  int     `json:"size"`
	Data  []Event `json:"data"`
}

// StatsResponse is the response for /api/query/stats
type StatsResponse struct {
	TotalEvents int64        `json:"totalEvents"`
	ErrorCount  int64        `json:"errorCount"`
	WarnCount   int64        `json:"warnCount"`
	InfoCount   int64        `json:"infoCount"`
	TopErrors   []ErrorStat  `json:"topErrors"`
	ErrorTrend  []TrendPoint `json:"errorTrend"`
}

// ErrorStat represents a single error type statistic
type ErrorStat struct {
	Message  string `json:"message"`
	Count    int64  `json:"count"`
	LastSeen int64  `json:"lastSeen"` // unix ms
}

// TrendPoint represents a data point in the trend chart
type TrendPoint struct {
	Timestamp int64 `json:"timestamp"` // unix ms
	Count     int64 `json:"count"`
}

// AppInfo represents basic app information
type AppInfo struct {
	AppID       string `json:"appId"`
	Release     string `json:"release"`
	FirstSeen   int64  `json:"firstSeen"`
	LastSeen    int64  `json:"lastSeen"`
	ErrorCount  int64  `json:"errorCount"`
	TotalEvents int64  `json:"totalEvents"`
}

// ToDBRecord converts Event to database record fields
func (e *Event) ToDBRecord() map[string]interface{} {
	return map[string]interface{}{
		"app_id":      e.AppID,
		"release":     e.Release,
		"env":         e.Env,
		"build_id":    e.BuildID,
		"user_id":     e.UserID,
		"session_id":  e.SessionID,
		"type":        e.Type,
		"level":       e.Level,
		"message":     e.Message,
		"stack":       e.Stack,
		"url":         e.URL,
		"line":        e.Line,
		"col":         e.Col,
		"tags":        toJSON(e.Tags),
		"extra":       toJSON(e.Extra),
		"ua":          e.UA,
		"screen":      e.Screen,
		"viewport":    e.Viewport,
		"performance": toJSON(e.Performance),
		"ip":          e.IP,
		"created_at":  e.CreatedAt,
	}
}

// ScanFromDB scans database columns into Event with safe type assertions
func (e *Event) ScanFromDB(rows map[string]interface{}) {
	// Use safe type assertions for required fields
	if id, ok := rows["id"].(int64); ok {
		e.ID = id
	}
	if appID, ok := rows["app_id"].(string); ok {
		e.AppID = appID
	}
	if eventType, ok := rows["type"].(string); ok {
		e.Type = eventType
	}
	if level, ok := rows["level"].(string); ok {
		e.Level = level
	}
	if message, ok := rows["message"].(string); ok {
		e.Message = message
	}
	if createdAt, ok := rows["created_at"].(int64); ok {
		e.CreatedAt = createdAt
	}

	// Use safe conversion for optional fields
	e.Release = toString(rows["release"])
	e.Env = toString(rows["env"])
	e.BuildID = toString(rows["build_id"])
	e.UserID = toString(rows["user_id"])
	e.SessionID = toString(rows["session_id"])
	e.Stack = toString(rows["stack"])
	e.URL = toString(rows["url"])
	e.Line = toInt(rows["line"])
	e.Col = toInt(rows["col"])
	e.Tags = parseJSON(toString(rows["tag"]))
	e.Extra = parseJSON(toString(rows["extra"]))
	e.UA = toString(rows["ua"])
	e.Screen = toString(rows["screen"])
	e.Viewport = toString(rows["viewport"])
	e.Performance = parseJSON(toString(rows["performance"]))
	e.IP = toString(rows["ip"])
}

func toJSON(v interface{}) string {
	if v == nil {
		return "{}"
	}
	data, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(data)
}

func toString(v interface{}) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case []byte:
		return string(val)
	default:
		return fmt.Sprintf("%v", val)
	}
}

func toInt(v interface{}) int {
	if v == nil {
		return 0
	}
	switch val := v.(type) {
	case int64:
		return int(val)
	case int:
		return val
	default:
		return 0
	}
}

func parseJSON(s string) map[string]interface{} {
	if s == "" || s == "{}" {
		return make(map[string]interface{})
	}
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(s), &result); err != nil {
		return make(map[string]interface{})
	}
	return result
}

// TopNQuery is the query parameters for /api/query/top
type TopNQuery struct {
	AppID     string `json:"appId"`
	Type      string `json:"type"`      // errors|pages|releases|browsers
	OrderBy   string `json:"orderBy"`   // count|impact|regression|lastSeen
	Limit     int    `json:"limit"`     // default 20
	Env       string `json:"env"`       // optional env filter
	Release   string `json:"release"`   // optional release filter
	StartTime int64  `json:"startTime"` // unix ms, optional
	EndTime   int64  `json:"endTime"`   // unix ms, optional
}

// TopNResponse is the response for /api/query/top
type TopNResponse struct {
	Type string     `json:"type"`
	Data []TopNItem `json:"data"`
}

// TopNItem represents a single item in top N results
type TopNItem struct {
	Key         string `json:"key"` // error message, page URL, release, browser
	Count       int64  `json:"count"`
	Users       int64  `json:"users"`       // unique users affected
	LastSeen    int64  `json:"lastSeen"`    // unix ms
	FirstSeen   int64  `json:"firstSeen"`   // unix ms
	IsNew       bool   `json:"isNew"`       // first seen in last 24h (for regression)
	ImpactScore int64  `json:"impactScore"` // count * users (for impact)
}

// ExportQuery is the query parameters for /api/query/export
type ExportQuery struct {
	AppID     string `json:"appId"`
	Type      string `json:"type"`      // optional filter
	Level     string `json:"level"`     // optional filter
	Release   string `json:"release"`   // optional filter
	Env       string `json:"env"`       // optional filter
	StartTime int64  `json:"startTime"` // unix ms, optional
	EndTime   int64  `json:"endTime"`   // unix ms, optional
	Keyword   string `json:"keyword"`   // optional search
	Format    string `json:"format"`    // json|csv
}

// ErrorCluster represents a cluster of similar errors
type ErrorCluster struct {
	ID           string   `json:"id"`      // cluster fingerprint
	Message      string   `json:"message"` // representative error message
	Stack        string   `json:"stack"`   // representative stack snippet
	Count        int64    `json:"count"`
	Users        int64    `json:"users"`     // unique users affected
	FirstSeen    int64    `json:"firstSeen"` // unix ms
	LastSeen     int64    `json:"lastSeen"`  // unix ms
	AffectedURLs []string `json:"affectedUrls"`
	Releases     []string `json:"releases"` // affected releases
	Pattern      string   `json:"pattern"`  // clustering pattern used
}

// SimilarErrorsQuery is the query for finding similar errors
type SimilarErrorsQuery struct {
	AppID     string  `json:"appId"`
	Message   string  `json:"message"`   // the error message to find similar errors for
	Threshold float64 `json:"threshold"` // similarity threshold 0-1, default 0.7
	Limit     int     `json:"limit"`     // default 10
}

// SimilarErrorsResponse is the response for similar errors
type SimilarErrorsResponse struct {
	Clusters []ErrorCluster `json:"clusters"`
}
