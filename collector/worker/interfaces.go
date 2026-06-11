package worker

import "github.com/logmonitor/collector/storage"

// EventStore defines the interface for event operations in workers
type EventStore interface {
	GetRecentEvents(limit int) ([]storage.EventRecord, error)
}

// IssueStore defines the interface for issue operations in workers
type IssueStore interface {
	CreateOrUpdateIssues(events []storage.EventRecord) error
}

// AlertStore defines the interface for alert operations in workers
type AlertStore interface {
	GetAllAlertRules() ([]storage.AlertRule, error)
	UpdateAlertRuleLastTriggered(id int64, timestamp int64) error
	CreateAlertLog(log storage.AlertLog) error
}

// SystemStore defines the interface for system operations in workers
type SystemStore interface {
	GetRetentionPolicySimple() (int, error)
	CleanupOldDataWithDays(days int) storage.CleanupResult
	GetLastCleanupTime() int64
	SetLastCleanupTime(timestamp int64) error
}