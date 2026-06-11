package storage

// Project represents a project in the system
type Project struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	Slug          string `json:"slug"`
	Description   string `json:"description"`
	APIKey        string `json:"api_key"`
	RetentionDays int    `json:"retention_days"`
	CreatedAt     int64  `json:"created_at"`
	UpdatedAt     int64  `json:"updated_at"`
	DeletedAt     int64  `json:"deleted_at"`
}

// ProjectMember represents a project member
type ProjectMember struct {
	ID        int64  `json:"id"`
	ProjectID int64  `json:"project_id"`
	UserID    int64  `json:"user_id"`
	Role      string `json:"role"` // owner|developer|viewer
	CreatedAt int64  `json:"created_at"`
}

// StorageStats represents storage statistics
type StorageStats struct {
	DatabaseSize    int64 `json:"database_size"`
	EventCount      int64 `json:"event_count"`
	IssueCount      int64 `json:"issue_count"`
	OldestEventTime int64 `json:"oldest_event_time"`
	NewestEventTime int64 `json:"newest_event_time"`
}

// RetentionPolicy represents the retention policy
type RetentionPolicy struct {
	Events          int `json:"events"`
	RecordingsDays  int `json:"recordings_days"`
	ScreenshotsDays int `json:"screenshots_days"`
	AlertLogs       int `json:"alert_logs"`
}
