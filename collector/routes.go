package main

import (
	"net/http"
	"time"

	"github.com/logmonitor/collector/buffer"
	"github.com/logmonitor/collector/config"
	"github.com/logmonitor/collector/handler"
	"github.com/logmonitor/collector/middleware"
	"github.com/logmonitor/collector/storage"
	"github.com/logmonitor/collector/webhook"
)

// RouterConfig holds all dependencies needed to set up HTTP routes.
type RouterConfig struct {
	DB             *storage.DB
	Store          storage.Store
	UserStorage    *storage.UserStorage
	SMStorage      *storage.SourceMapStorage
	Writer         *buffer.Writer
	Config         *config.Config
	JWT            *middleware.JWT
	CORS           *middleware.CORS
	WebhookManager *webhook.Manager
	OpenAPISpec    []byte
}

// SetupRoutes configures all HTTP routes and returns the serve mux.
func SetupRoutes(rc *RouterConfig) *http.ServeMux {
	mux := http.NewServeMux()

	// Initialize handlers
	reportHandler := handler.NewReportHandler(rc.Writer, &rc.Config.Server, rc.DB)
	queryHandler := handler.NewQueryHandler(rc.DB)
	authHandler := handler.NewAuthHandler(rc.UserStorage, rc.JWT)
	clustersHandler := handler.NewClustersHandler(rc.Store.Events())
	issuesHandler := handler.NewIssuesHandler(rc.DB)
	healthHandler := handler.NewHealthHandler(rc.DB)
	alertsHandler := handler.NewAlertsHandler(rc.DB)
	systemHandler := handler.NewSystemHandler(rc.DB, rc.Config.Database.Path, rc.Config.Database.RetentionDays)
	adminHandler := handler.NewAdminHandler(rc.DB)
	projectsHandler := handler.NewProjectsHandler(rc.DB, rc.UserStorage)
	webhooksHandler := handler.NewWebhooksHandler(rc.DB, rc.WebhookManager)
	sourceMapHandler := handler.NewSourceMapHandler(rc.DB, rc.SMStorage)
	openapiHandler := handler.NewOpenAPIHandler(rc.OpenAPISpec)
	screenshotHandler := handler.NewScreenshotHandler("./data/screenshots")
	screenshotFileHandler := handler.NewScreenshotFileHandler("./data/screenshots")

	sourceMapHandler.SetAllowedOrigins(rc.Config.Server.AllowedOrigins)

	// === Public routes (no auth) ===
	mux.Handle("/api/report", reportHandler)
	mux.Handle("/api/events", reportHandler)
	mux.Handle("/api/report/screenshot", screenshotHandler)
	mux.Handle("/api/screenshots/", rc.CORS.Handler(rc.JWT.Handler(screenshotFileHandler)))
	mux.Handle("/api/auth/login", rc.CORS.Handler(middleware.NewRateLimiter(5, time.Minute).Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHandler.Login(w, r)
	}))))
	mux.Handle("/api/health", rc.CORS.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		queryHandler.Health(w, r)
	})))
	mux.Handle("/api/docs", rc.CORS.Handler(http.HandlerFunc(openapiHandler.GetSpec)))
	mux.Handle("/api/docs/ui", rc.CORS.Handler(http.HandlerFunc(openapiHandler.GetSwaggerUI)))

	// === Authenticated routes ===
	authRoutes := []struct {
		pattern string
		handler http.HandlerFunc
	}{
		// Auth
		{"GET /api/auth/me", authHandler.Me},
		{"PUT /api/auth/password", authHandler.ChangePassword},

		// Query - logs & stats
		{"GET /api/query/logs", queryHandler.QueryLogs},
		{"GET /api/query/stats", queryHandler.QueryStats},
		{"GET /api/query/apps", queryHandler.QueryApps},
		{"GET /api/query/top", queryHandler.QueryTop},
		{"GET /api/query/similar", queryHandler.QuerySimilar},
		{"GET /api/query/export", queryHandler.QueryExport},

		// Query - performance
		{"GET /api/query/performance/summary", queryHandler.QueryPerformanceSummary},
		{"GET /api/query/performance/trend", queryHandler.QueryPerformanceTrend},
		{"GET /api/query/performance/pages", queryHandler.QueryPerformancePages},
		{"GET /api/query/performance/regression", queryHandler.QueryPerformanceRegression},

		// Query - anomaly
		{"GET /api/query/anomaly/new-errors", queryHandler.QueryNewErrors},
		{"GET /api/query/anomaly/alert-triggers", queryHandler.QueryAlertTriggers},
		{"GET /api/query/anomaly/active-sessions", queryHandler.QueryActiveSessions},
		{"GET /api/query/stats/comparison", queryHandler.QueryStatsComparison},

		// Issues
		{"GET /api/query/issues", issuesHandler.GetIssues},
		{"GET /api/query/issues/", issuesHandler.GetIssue},
		{"PUT /api/query/issues/", issuesHandler.UpdateIssue},
		{"POST /api/query/issues/", func(w http.ResponseWriter, r *http.Request) {
			action := r.URL.Query().Get("action")
			if action == "resolve" {
				issuesHandler.ResolveIssue(w, r)
			} else if action == "ignore" {
				issuesHandler.IgnoreIssue(w, r)
			} else {
				http.Error(w, "Invalid action", http.StatusBadRequest)
			}
		}},
		{"GET /api/query/issues/stats", issuesHandler.GetIssueStats},

		// Clusters
		{"GET /api/query/clusters", clustersHandler.GetClusters},

		// Health & analytics
		{"GET /api/query/release-health", healthHandler.GetReleaseHealth},
		{"GET /api/query/session-stats", healthHandler.GetSessionStats},

		// Alerts
		{"GET /api/query/alerts", alertsHandler.GetAlerts},
		{"POST /api/query/alerts", alertsHandler.CreateAlert},
		{"DELETE /api/query/alerts/", alertsHandler.DeleteAlert},
		{"POST /api/alerts/test", alertsHandler.TestAlert},

		// System
		{"GET /api/system/info", systemHandler.GetSystemInfo},
		{"POST /api/system/cleanup", systemHandler.TriggerCleanup},

		// Source maps (read)
		{"GET /api/sourcemaps", sourceMapHandler.List},
		{"GET /api/sourcemaps/download", sourceMapHandler.Download},
		{"DELETE /api/sourcemaps/", sourceMapHandler.Delete},
		{"POST /api/sourcemaps/deobfuscate", sourceMapHandler.Deobfuscate},
		{"POST /api/sourcemaps/resolve", sourceMapHandler.Resolve},
	}

	for _, route := range authRoutes {
		h := rc.CORS.Handler(rc.JWT.Handler(http.HandlerFunc(route.handler)))
		mux.HandleFunc(route.pattern, func(w http.ResponseWriter, r *http.Request) {
			h.ServeHTTP(w, r)
		})
	}

	// === Admin-only routes ===
	adminRoutes := []struct {
		pattern string
		handler http.HandlerFunc
	}{
		// Source map upload
		{"POST /api/sourcemaps/upload", sourceMapHandler.Upload},

		// User management
		{"GET /api/users", authHandler.ListUsers},
		{"POST /api/users", authHandler.CreateUser},
		{"PUT /api/users/", authHandler.UpdateUser},
		{"DELETE /api/users/", authHandler.DeleteUser},

		// Storage governance
		{"GET /api/admin/storage/stats", adminHandler.GetStorageStats},
		{"GET /api/admin/retention/policy", adminHandler.GetRetentionPolicy},
		{"PUT /api/admin/retention/policy", adminHandler.SetRetentionPolicy},
		{"POST /api/admin/cleanup/manual", adminHandler.TriggerManualCleanup},

		// Projects
		{"POST /api/admin/projects", projectsHandler.CreateProject},
		{"GET /api/admin/projects", projectsHandler.ListProjects},
		{"GET /api/admin/projects/", projectsHandler.GetProject},
		{"PUT /api/admin/projects/", projectsHandler.UpdateProject},
		{"DELETE /api/admin/projects/", projectsHandler.DeleteProject},
		{"POST /api/admin/projects/api-key", projectsHandler.RegenerateApiKey},
		{"GET /api/admin/projects/members", projectsHandler.ListMembers},
		{"POST /api/admin/projects/members", projectsHandler.AddMember},
		{"PUT /api/admin/projects/members/", projectsHandler.UpdateMemberRole},
		{"DELETE /api/admin/projects/members/", projectsHandler.RemoveMember},

		// Webhooks
		{"GET /api/admin/webhooks", webhooksHandler.GetWebhooks},
		{"POST /api/admin/webhooks", webhooksHandler.CreateWebhook},
		{"PUT /api/admin/webhooks/", webhooksHandler.UpdateWebhook},
		{"DELETE /api/admin/webhooks/", webhooksHandler.DeleteWebhook},
		{"POST /api/admin/webhooks/test", webhooksHandler.TestWebhook},
	}

	for _, route := range adminRoutes {
		h := rc.CORS.Handler(rc.JWT.Handler(middleware.RequireAdmin(http.HandlerFunc(route.handler))))
		mux.HandleFunc(route.pattern, func(w http.ResponseWriter, r *http.Request) {
			h.ServeHTTP(w, r)
		})
	}

	return mux
}
