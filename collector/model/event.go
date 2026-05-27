package model

import (
	"encoding/json"
	"fmt"
)

// Event represents a log event from the SDK
type Event struct {
	ID          int64                  `json:"id"`
	AppID       string                 `json:"appId"`
	Release     string                 `json:"release"`
	Type        string                 `json:"type"`        // error|performance|info|warn|track
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
	AppID   string   `json:"appId"`
	Release string   `json:"release"`
	Events  []Event  `json:"events"`
}

// LogsQuery is the query parameters for /api/query/logs
type LogsQuery struct {
	AppID      string `json:"appId"`
	Type       string `json:"type"`       // optional filter
	Level      string `json:"level"`      // optional filter
	StartTime  int64  `json:"startTime"`  // unix ms, optional
	EndTime    int64  `json:"endTime"`    // unix ms, optional
	Keyword    string `json:"keyword"`    // optional search in message
	Page       int    `json:"page"`       // page number, 1-indexed
	PageSize   int    `json:"pageSize"`   // items per page
}

// LogsResponse is the response for /api/query/logs
type LogsResponse struct {
	Total int64    `json:"total"`
	Page  int      `json:"page"`
	Size  int      `json:"size"`
	Data  []Event  `json:"data"`
}

// StatsResponse is the response for /api/query/stats
type StatsResponse struct {
	TotalEvents    int64            `json:"totalEvents"`
	ErrorCount     int64            `json:"errorCount"`
	WarnCount      int64            `json:"warnCount"`
	InfoCount      int64            `json:"infoCount"`
	TopErrors      []ErrorStat      `json:"topErrors"`
	ErrorTrend     []TrendPoint     `json:"errorTrend"`
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
	AppID         string `json:"appId"`
	Release       string `json:"release"`
	FirstSeen     int64  `json:"firstSeen"`
	LastSeen      int64  `json:"lastSeen"`
	ErrorCount    int64  `json:"errorCount"`
	TotalEvents   int64  `json:"totalEvents"`
}

// ToDBRecord converts Event to database record fields
func (e *Event) ToDBRecord() map[string]interface{} {
	return map[string]interface{}{
		"app_id":      e.AppID,
		"release":     e.Release,
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
