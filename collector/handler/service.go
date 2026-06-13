package handler

import (
	"github.com/logmonitor/collector/storage"
)

// DashboardService defines the storage service interface for dashboard handlers.
// It combines all store interfaces used across the application.
// The *storage.DB type satisfies all these interfaces through duck typing.
type DashboardService interface {
	storage.EventStore
	storage.IssueStore
	storage.ProjectStore
	storage.AlertStore
	storage.AnalyticsStore
	storage.SystemStore
	storage.AuditStore
	storage.PerformanceStore
}

// ProjectsService defines the storage capabilities needed by ProjectsHandler.
type ProjectsService interface {
	storage.ProjectStore
}

// AlertsService defines the storage capabilities needed by AlertsHandler.
type AlertsService interface {
	storage.AlertStore
}

// IssuesService defines the storage capabilities needed by IssuesHandler.
type IssuesService interface {
	storage.IssueStore
}

// SystemService defines the storage capabilities needed by SystemHandler.
type SystemService interface {
	storage.SystemStore
}

// QueryService defines the storage capabilities needed by QueryHandler.
type QueryService interface {
	storage.EventStore
	storage.AnalyticsStore
}

// PerformanceService defines the storage capabilities needed by PerformanceHandler.
type PerformanceService interface {
	storage.PerformanceStore
}

// ReportService defines the storage capabilities needed by ReportHandler.
type ReportService interface {
	storage.EventStore
	storage.AnalyticsStore
}
