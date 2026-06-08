package storage

import "github.com/logmonitor/collector/model"

// EventRepository handles event CRUD operations
type EventRepository interface {
	InsertEvents(events []EventRecord) error
	QueryEvents(query QueryParams) (*QueryResult, error)
	GetStats(appID string) (*Stats, error)
	GetApps() ([]AppStats, error)
	GetTopN(appID, topType, orderBy string, limit int, filters map[string]interface{}) (*TopNResult, error)
	GetSimilarErrors(appID, message string, threshold float64, limit int) ([]ErrorCluster, error)
	GetSessionEvents(sessionID string, limit int) ([]EventRecord, error)
	GetSessionErrorCount(sessionID string) (int64, error)
	GetTopErrors(params TopListParams) ([]TopError, error)
	GetTopPages(params TopListParams) ([]TopPage, error)
	GetTopReleases(params TopListParams) ([]TopRelease, error)
	GetTopBrowsers(params TopListParams) ([]TopBrowser, error)
	GetErrorClustersByTime(appID string, startTime, endTime int64, limit int) ([]ErrorClusterResult, error)
	GetClusterEvents(appID, fingerprint string, page, pageSize int) ([]EventRecord, int64, error)
	GetClusterStats(appID, fingerprint string) (ClusterStats, error)
}

// AlertRepository handles alert rules and logs
type AlertRepository interface {
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
	Events() EventRepository
	Alerts() AlertRepository
	Recordings() RecordingRepository
	SourceMaps() SourceMapRepository
	Users() UserRepository
	Close() error
}
