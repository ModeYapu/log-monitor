package storage

import (
	"time"

	"github.com/logmonitor/collector/model"
)

// CleanupResult represents the result of a cleanup operation
type CleanupResult struct {
	DeletedEvents      int64
	DeletedScreenshots int64
	TotalFilesFreed    int64
	TotalBytesFreed    int64
	Duration           time.Duration
}

// EventStore handles event CRUD operations and queries
type EventStore interface {
	InsertEvents(events []EventRecord) error
	QueryEvents(query QueryParams) (*QueryResult, error)
	GetStats(appID string, projectID int64) (*Stats, error)
	GetApps(projectID int64) ([]AppStats, error)
	GetTopN(appID, topType, orderBy string, limit int, filters AnalyticsFilters) (*TopNResult, error)
	GetSimilarErrors(appID, message string, threshold float64, limit int, projectID int64) ([]ErrorCluster, error)
	GetSessionEvents(sessionID string, limit int) ([]EventRecord, error)
	GetSessionErrorCount(sessionID string) (int64, error)
	GetTopErrors(params TopListParams) ([]TopError, error)
	GetTopPages(params TopListParams) ([]TopPage, error)
	GetTopReleases(params TopListParams) ([]TopRelease, error)
	GetTopBrowsers(params TopListParams) ([]TopBrowser, error)
	GetErrorClustersByTime(appID string, startTime, endTime int64, limit int) ([]ErrorClusterResult, error)
	GetClusterEvents(appID, fingerprint string, page, pageSize int) ([]EventRecord, int64, error)
	GetClusterStats(appID, fingerprint string) (ClusterStats, error)
	GetErrorClusters(appID, errorMessage string, threshold float64, limit int, projectID int64) ([]ErrorCluster, error)
	GetRecentEvents(limit int) ([]EventRecord, error)
}

// IssueStore handles issue CRUD operations and lifecycle management
type IssueStore interface {
	CreateOrUpdateIssues(events []EventRecord) error
	GetIssues(filter IssueFilter) ([]Issue, int64, error)
	GetIssue(id int64) (*Issue, error)
	GetIssueEvents(issueID int64, page, pageSize int) ([]EventRecord, int64, error)
	UpdateIssue(id int64, updates map[string]interface{}) error
	GetIssueStats(appID string) (*IssueStats, error)
}

// ProjectStore handles project CRUD operations and member management
type ProjectStore interface {
	CreateProject(name, slug, description string) (*Project, error)
	GetProject(idOrSlug interface{}) (*Project, error)
	GetProjectByAPIKey(apiKey string) (*Project, error)
	ListProjects(userID int64) ([]Project, error)
	UpdateProject(id int64, updates map[string]interface{}) error
	DeleteProject(id int64) error
	AutoCreateDefaultProject() error
	AddProjectMember(projectID, userID int64, role string) error
	RemoveProjectMember(projectID, userID int64) error
	GetProjectMembers(projectID int64) ([]ProjectMember, error)
	UpdateProjectMemberRole(projectID, userID int64, newRole string) error
	RegenerateApiKey(projectID int64) (string, error)
}

// AlertStore handles alert rules and alert logs
type AlertStore interface {
	CreateAlertRule(rule AlertRule) (int64, error)
	GetAlertRules(appID string) ([]AlertRule, error)
	GetAllAlertRules() ([]AlertRule, error)
	UpdateAlertRuleLastTriggered(id int64, timestamp int64) error
	DeleteAlertRule(id int64) error
	SilenceAlertRule(id int64, until int64) error
	UnsilenceAlertRule(id int64) error
	CreateAlertLog(log AlertLog) error
	GetAlertLogs(appID string, limit int) ([]AlertLog, error)
}

// AnalyticsStore handles statistics, performance, and anomaly queries
type AnalyticsStore interface {
	GetStats(appID string, projectID int64) (*Stats, error)
	GetApps(projectID int64) ([]AppStats, error)
	GetTopN(appID, topType, orderBy string, limit int, filters AnalyticsFilters) (*TopNResult, error)
	GetTopErrors(params TopListParams) ([]TopError, error)
	GetTopPages(params TopListParams) ([]TopPage, error)
	GetTopReleases(params TopListParams) ([]TopRelease, error)
	GetTopBrowsers(params TopListParams) ([]TopBrowser, error)
	GetReleaseHealth(appID string, startTime, endTime int64) (*ReleaseHealthResult, error)
	GetSessionStats(appID string, startTime, endTime int64) (*SessionStatsResult, error)
	GetClusterStats(appID, fingerprint string) (ClusterStats, error)
}

// SystemStore handles system metadata and storage operations
type SystemStore interface {
	GetStorageStats() (*StorageStats, error)
	GetRetentionPolicySimple() (int, error)
	SetRetentionPolicySimple(days int) error
	TriggerManualCleanup() error
	GetLastCleanupTime() int64
	SetLastCleanupTime(timestamp int64) error
	CleanupOldDataWithDays(days int) CleanupResult
	DeleteEventsBefore(before time.Time) (int64, error)
	DeleteRecordingsBefore(before time.Time) (int64, error)
}

// RecordingRepository handles session recordings
type RecordingRepository interface {
	CreateRecording(recording RecordingInfo) (int64, error)
	GetRecording(sessionID string) (*RecordingInfo, error)
	GetRecordings(limit, offset int, filters map[string]interface{}) ([]RecordingInfo, error)
	AddRecordingEvent(sessionID string, seq int, timestamp int64, eventData []byte) error
	GetRecordingEvents(sessionID string, limit, offset int) ([]RecordingEventData, error)
	DeleteRecording(sessionID string) error
	UpdateRecording(sessionID string, endTime int64, durationMs int64, eventCount int, status string) error
	GetRecordingStats(sessionID string) (interface{}, error)
}

// SourceMapRepository handles source map storage
type SourceMapRepository interface {
	CreateSourceMap(record SourceMapRecord) (int64, error)
	GetSourceMap(appID, release, env, buildID string) (*SourceMapRecord, error)
	GetSourceMapByBuildID(buildID string) (*SourceMapRecord, error)
	ListSourceMaps(appID string, limit int) ([]SourceMapRecord, error)
	DeleteSourceMap(id int64) error
	EnsureSourceMapsTable() error
}

// UserRepository handles user management
type UserRepository interface {
	CreateUser(username, passwordHash, displayName, role string) (int64, error)
	GetUserByUsername(username string) (*model.User, string, error)
	GetUserByID(id int64) (*model.User, error)
	ListUsers() ([]model.User, error)
	UpdateUser(id int64, displayName, role string, enabled bool) error
	UpdatePassword(id int64, passwordHash string) error
	UpdateLastLogin(id int64) error
	DeleteUser(id int64) error
	CountUsers() (int64, error)
}

// Store combines all repositories for easy dependency injection
type Store interface {
	Events() EventStore
	Issues() IssueStore
	Projects() ProjectStore
	Alerts() AlertStore
	Analytics() AnalyticsStore
	System() SystemStore
	Recordings() RecordingRepository
	SourceMaps() SourceMapRepository
	Users() UserRepository
	Close() error
}
