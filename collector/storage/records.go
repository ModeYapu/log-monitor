package storage

// EventRecord represents an event record from the database
type EventRecord struct {
	ID          int64  `json:"id"`
	AppID       string `json:"app_id"`
	Release     string `json:"release"`
	Env         string `json:"env"`
	BuildID     string `json:"build_id"`
	UserID      string `json:"user_id"`
	SessionID   string `json:"session_id"`
	Type        string `json:"type"`
	Level       string `json:"level"`
	Message     string `json:"message"`
	Stack       string `json:"stack"`
	URL         string `json:"url"`
	Line        int    `json:"line"`
	Col         int    `json:"col"`
	Tags        string `json:"tags"`  // JSON string
	Extra       string `json:"extra"` // JSON string
	UA          string `json:"ua"`
	Screen      string `json:"screen"`
	Viewport    string `json:"viewport"`
	Performance string `json:"performance"` // JSON string
	IP          string `json:"ip"`
	Fingerprint string `json:"fingerprint"`
	Breadcrumbs string `json:"breadcrumbs"` // JSON string
	ProjectID   int64  `json:"project_id"`
	CreatedAt   int64  `json:"created_at"`
}

// SessionSummary represents a summary of a user session
type SessionSummary struct {
	SessionID  string `json:"sessionId"`
	AppID      string `json:"appId"`
	UserID     string `json:"userId"`
	StartTime  int64  `json:"startTime"`
	EndTime    int64  `json:"endTime"`
	DurationMs int64  `json:"durationMs"`
	EventCount int64  `json:"eventCount"`
	ErrorCount int64  `json:"errorCount"`
	LastURL    string `json:"lastUrl"`
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
	ID          int64  `json:"id"`
	Fingerprint string `json:"fingerprint"`
	AppID       string `json:"app_id"`
	Title       string `json:"title"`
	Type        string `json:"type"`
	Status      string `json:"status"`
	Priority    string `json:"priority"`
	Assignee    string `json:"assignee"`
	FirstSeenAt int64  `json:"first_seen_at"`
	LastSeenAt  int64  `json:"last_seen_at"`
	EventCount  int64  `json:"event_count"`
	UserCount   int64  `json:"user_count"`
	ResolvedAt  int64  `json:"resolved_at"`
	CreatedAt   int64  `json:"created_at"`
	UpdatedAt   int64  `json:"updated_at"`
	ProjectID   int64  `json:"project_id"`
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

// TopListParams represents parameters for top lists queries
type TopListParams struct {
	AppID     string
	StartTime int64
	EndTime   int64
	Limit     int
	SortBy    string
}

// TopError represents an error in the top errors list
type TopError struct {
	Message       string `json:"message"`
	Count         int64  `json:"count"`
	FirstSeen     int64  `json:"firstSeen"`
	LastSeen      int64  `json:"lastSeen"`
	AffectedUsers int64  `json:"affectedUsers"`
	SampleStack   string `json:"sampleStack,omitempty"`
}

// TopPage represents a page in the top pages list
type TopPage struct {
	URL         string  `json:"url"`
	ErrorCount  int64   `json:"errorCount"`
	TotalEvents int64   `json:"totalEvents"`
	ErrorRate   float64 `json:"errorRate"`
	FirstSeen   int64   `json:"firstSeen"`
	LastSeen    int64   `json:"lastSeen"`
}

// TopRelease represents a release in the top releases list
type TopRelease struct {
	Release     string  `json:"release"`
	Env         string  `json:"env"`
	ErrorCount  int64   `json:"errorCount"`
	TotalEvents int64   `json:"totalEvents"`
	ErrorRate   float64 `json:"errorRate"`
	FirstSeen   int64   `json:"firstSeen"`
	LastSeen    int64   `json:"lastSeen"`
	NewErrors   int64   `json:"newErrors"`
	IsLatest    bool    `json:"isLatest"`
}

// TopBrowser represents a browser in the top browsers list
type TopBrowser struct {
	Browser     string  `json:"browser"`
	Version     string  `json:"version"`
	ErrorCount  int64   `json:"errorCount"`
	TotalEvents int64   `json:"totalEvents"`
	ErrorRate   float64 `json:"errorRate"`
}

// ErrorClusterResult represents an error cluster from the database
type ErrorClusterResult struct {
	Fingerprint string   `json:"fingerprint"`
	Message     string   `json:"message"`
	Count       int64    `json:"count"`
	Users       int64    `json:"users"`
	FirstSeen   int64    `json:"firstSeen"`
	LastSeen    int64    `json:"lastSeen"`
	URLs        []string `json:"urls"`
	Releases    []string `json:"releases"`
}

// ClusterStats represents detailed statistics for a cluster
type ClusterStats struct {
	Fingerprint         string            `json:"fingerprint"`
	TotalCount          int64             `json:"totalCount"`
	UniqueUsers         int64             `json:"uniqueUsers"`
	FirstSeen           int64             `json:"firstSeen"`
	LastSeen            int64             `json:"lastSeen"`
	ReleaseDistribution map[string]int64  `json:"releaseDistribution"`
	EnvDistribution     map[string]int64  `json:"envDistribution"`
	TimeSeries          []TimeSeriesPoint `json:"timeSeries"`
}

// ErrorCluster represents a cluster of similar errors
type ErrorCluster struct {
	ClusterID     string        `json:"cluster_id"`
	Pattern       string        `json:"pattern"`
	Message       string        `json:"message"`
	Count         int64         `json:"count"`
	FirstSeen     int64         `json:"firstSeen"`
	LastSeen      int64         `json:"lastSeen"`
	AffectedUsers int64         `json:"affectedUsers"`
	SampleEvents  []EventRecord `json:"sampleEvents"`
	ID            string        `json:"id,omitempty"`
	Users         int64         `json:"users,omitempty"`
	Stack         string        `json:"stack,omitempty"`
	AffectedURLs  []string      `json:"affected_urls,omitempty"`
	Releases      []string      `json:"releases,omitempty"`
}

// TopNItem represents an item in a top N list
type TopNItem struct {
	Key         string `json:"key"`
	Count       int64  `json:"count"`
	Users       int64  `json:"users"`
	FirstSeen   int64  `json:"firstSeen"`
	LastSeen    int64  `json:"lastSeen"`
	IsNew       bool   `json:"isNew"`
	ImpactScore int64  `json:"impactScore"`
}

// TopNResult represents the result of a top N query
type TopNResult struct {
	Type  string     `json:"type"`
	Items []TopNItem `json:"items"`
	Total int64      `json:"total"`
}
