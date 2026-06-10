package storage

// EventRecord represents an event record from the database
type EventRecord struct {
	ID          int64                  `json:"id"`
	AppID       string                 `json:"app_id"`
	Release     string                 `json:"release"`
	Env         string                 `json:"env"`
	BuildID     string                 `json:"build_id"`
	UserID      string                 `json:"user_id"`
	SessionID   string                 `json:"session_id"`
	Type        string                 `json:"type"`
	Level       string                 `json:"level"`
	Message     string                 `json:"message"`
	Stack       string                 `json:"stack"`
	URL         string                 `json:"url"`
	Line        int                    `json:"line"`
	Col         int                    `json:"col"`
	Tags        string                 `json:"tags"`        // JSON string
	Extra       string                 `json:"extra"`       // JSON string
	UA          string                 `json:"ua"`
	Screen      string                 `json:"screen"`
	Viewport    string                 `json:"viewport"`
	Performance string                 `json:"performance"` // JSON string
	IP          string                 `json:"ip"`
	Fingerprint string                 `json:"fingerprint"`
	Breadcrumbs string                 `json:"breadcrumbs"` // JSON string
	ProjectID   int64                  `json:"project_id"`
	CreatedAt   int64                  `json:"created_at"`
}

// QueryParams represents parameters for querying events
type QueryParams struct {
	AppID       string
	Type        string
	Level       string
	StartTime   int64
	EndTime     int64
	Keyword     string
	Release     string
	Env         string
	UserID      string
	SessionID   string
	Page        int
	PageSize    int
	SortBy      string
	SortOrder   string
}

// QueryResult represents the result of a query
type QueryResult struct {
	Total    int64         `json:"total"`
	Page     int           `json:"page"`
	PageSize int           `json:"page_size"`
	Size     int           `json:"size"`
	Data     []EventRecord `json:"data"`
}

// Stats represents event statistics
type Stats struct {
	TotalEvents int64            `json:"total_events"`
	ErrorCount  int64            `json:"error_count"`
	WarnCount   int64            `json:"warn_count"`
	InfoCount   int64            `json:"info_count"`
	TopErrors   []ErrorStat      `json:"top_errors"`
	ByLevel     map[string]int64 `json:"by_level"`
	ByType      map[string]int64 `json:"by_type"`
}

// ErrorStat represents error statistics
type ErrorStat struct {
	Message  string `json:"message"`
	Count    int64  `json:"count"`
	LastSeen int64  `json:"last_seen"`
}

// AppStats represents application statistics
type AppStats struct {
	AppID       string `json:"app_id"`
	Release     string `json:"release"`
	FirstSeen   int64  `json:"first_seen"`
	LastSeen    int64  `json:"last_seen"`
	ErrorCount  int64  `json:"error_count"`
	TotalEvents int64  `json:"total_events"`
	EventCount  int64  `json:"event_count"`
}

// Issue represents an issue in the system
type Issue struct {
	ID           int64  `json:"id"`
	Fingerprint string `json:"fingerprint"`
	AppID        string `json:"app_id"`
	Title        string `json:"title"`
	Type         string `json:"type"`
	Status       string `json:"status"`
	Priority     string `json:"priority"`
	Assignee     string `json:"assignee"`
	FirstSeenAt  int64  `json:"first_seen_at"`
	LastSeenAt   int64  `json:"last_seen_at"`
	EventCount   int64  `json:"event_count"`
	UserCount    int64  `json:"user_count"`
	ResolvedAt   int64  `json:"resolved_at"`
	CreatedAt    int64  `json:"created_at"`
	UpdatedAt    int64  `json:"updated_at"`
	ProjectID    int64  `json:"project_id"`
}

// IssueFilter represents filter parameters for querying issues
type IssueFilter struct {
	AppID    string
	Status   string
	Priority string
	Search   string
	SortBy   string
	Page     int
	PageSize int
}

// IssueStats represents statistics for issues
type IssueStats struct {
	OpenCount        int64            `json:"open_count"`
	ResolvedCount    int64            `json:"resolved_count"`
	IgnoredCount     int64            `json:"ignored_count"`
	MutedCount       int64            `json:"muted_count"`
	TotalCount       int64            `json:"total_count"`
	HighPriority     int64            `json:"high_priority"`
	CriticalPriority int64            `json:"critical_priority"`
	ByStatus         map[string]int64 `json:"by_status"`
	ByPriority       map[string]int64 `json:"by_priority"`
	TrendData        []TrendPoint     `json:"trend_data"`
}

// TrendPoint represents a single point in a trend
type TrendPoint struct {
	Timestamp int64 `json:"timestamp"`
	Count     int64 `json:"count"`
}

// cleanupResultInternal represents the internal result of a cleanup operation
type cleanupResultInternal struct {
	EventsDeleted          int64
	RecordingEventsDeleted int64
	ScreenshotsDeleted     int64
	AlertLogsDeleted       int64
}