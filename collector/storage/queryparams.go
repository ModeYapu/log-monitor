package storage

// QueryParams represents typed parameters for querying events
type QueryParams struct {
	AppID     string
	Type      string
	Level     string
	StartTime int64
	EndTime   int64
	Keyword   string
	Release   string
	Env       string
	UserID    string
	SessionID string
	Page      int
	PageSize  int
	SortBy    string
	SortOrder string
}

// AnalyticsFilters represents typed filters for analytics queries
type AnalyticsFilters struct {
	Env       string
	Release   string
	StartTime int64
	EndTime   int64
}

// EventTimelinePoint represents a data point in event timeline
type EventTimelinePoint struct {
	Date  string `json:"date"`
	Count int64  `json:"count"`
}

// ErrorDistribution represents error distribution by message
type ErrorDistribution struct {
	Message    string  `json:"message"`
	Count      int64   `json:"count"`
	Percentage float64 `json:"percentage"`
}

// BrowserStats represents browser statistics
type BrowserStats struct {
	Browser    string  `json:"browser"`
	Version    string  `json:"version"`
	Count      int64   `json:"count"`
	Percentage float64 `json:"percentage"`
	ErrorRate  float64 `json:"error_rate"`
}

// PageStats represents page statistics
type PageStats struct {
	Path      string  `json:"path"`
	Count     int64   `json:"count"`
	ErrorRate float64 `json:"error_rate"`
	AvgLoad   float64 `json:"avg_load_time"`
}

// ReleaseStats represents release statistics
type ReleaseStats struct {
	Release   string  `json:"release"`
	Env       string  `json:"env"`
	Count     int64   `json:"count"`
	ErrorRate float64 `json:"error_rate"`
	CrashFree float64 `json:"crash_free_rate"`
}

// SessionStatsResult represents session statistics result
type SessionStatsResult struct {
	TotalSessions    int64   `json:"total_sessions"`
	CrashSessions    int64   `json:"crash_sessions"`
	CrashFreeRate    float64 `json:"crash_free_rate"`
	ErrorCount       int64   `json:"error_count"`
	AvgSessionLength float64 `json:"avg_session_length"`
	StartTime        int64   `json:"start_time"`
	EndTime          int64   `json:"end_time"`
}

// ReleaseHealthResult represents release health result
type ReleaseHealthResult struct {
	Releases      []ReleaseStats `json:"releases"`
	TotalSessions int64          `json:"total_sessions"`
	CrashFreeRate float64        `json:"avg_crash_free_rate"`
}
