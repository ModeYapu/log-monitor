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
	PerformanceHandler *handler.PerformanceHandler
	RuleEngineHandler   *handler.RuleEngineHandler
	ChannelManagerHandler *handler.ChannelManagerHandler
	ClustererHandler    *handler.ClustererHandler
}

// Route represents a single HTTP route.
type Route struct {
	Pattern string
	Handler http.Handler
}

// RouteGroup manages a group of routes with common middleware.
type RouteGroup struct {
	mux       *http.ServeMux
	prefix    string
	middleware []func(http.Handler) http.Handler
}

// NewRouteGroup creates a new route group.
func NewRouteGroup(mux *http.ServeMux, prefix string) *RouteGroup {
	return &RouteGroup{
		mux:    mux,
		prefix: prefix,
	}
}

// Use adds middleware to the group.
func (g *RouteGroup) Use(middleware func(http.Handler) http.Handler) *RouteGroup {
	g.middleware = append(g.middleware, middleware)
	return g
}

// Handle registers a route with the group's middleware chain.
func (g *RouteGroup) Handle(pattern string, handler http.Handler) {
	fullPattern := g.prefix + pattern
	h := handler
	// Apply middleware in reverse order (last added is first to execute)
	for i := len(g.middleware) - 1; i >= 0; i-- {
		h = g.middleware[i](h)
	}
	g.mux.Handle(fullPattern, h)
}

// HandleFunc registers a route with an HTTP handler function.
func (g *RouteGroup) HandleFunc(pattern string, handler http.HandlerFunc) {
	g.Handle(pattern, handler)
}

// SetupRoutes configures all HTTP routes and returns the serve mux.
func SetupRoutes(rc *RouterConfig) *http.ServeMux {
	mux := http.NewServeMux()

	// Initialize handlers
	reportHandler := handler.NewReportHandler(rc.Writer, rc.Store.Projects())
	queryHandler := handler.NewQueryHandler(rc.Store.Events(), rc.Store.Analytics())
	authHandler := handler.NewAuthHandler(rc.UserStorage, rc.JWT)
	clustersHandler := handler.NewClustersHandler(rc.Store.Events())
	issuesHandler := handler.NewIssuesHandler(rc.Store.Issues())
	healthHandler := handler.NewHealthHandler(rc.Config.Database.Path, rc.Store.Analytics(), rc.Store.System(), rc.Store.Events())
	alertsHandler := handler.NewAlertsHandler(rc.Store.Alerts())
	systemHandler := handler.NewSystemHandler(rc.Store.System(), rc.Store.Events(), rc.Store.Recordings(), rc.Config.Database.Path, rc.Config.Database.RetentionDays)
	adminHandler := handler.NewAdminHandler(rc.Store.System())
	projectsHandler := handler.NewProjectsHandler(rc.Store.Projects(), rc.UserStorage)
	webhooksHandler := handler.NewWebhooksHandler(rc.WebhookManager)
	sourceMapHandler := handler.NewSourceMapHandler(rc.Store.SourceMaps(), rc.SMStorage)
	openapiHandler := handler.NewOpenAPIHandler(rc.OpenAPISpec)
	screenshotHandler := handler.NewScreenshotHandler("./data/screenshots")
	screenshotFileHandler := handler.NewScreenshotFileHandler("./data/screenshots")
	auditHandler := handler.NewAuditHandler(rc.Store.AuditLogs())
	performanceHandler := rc.PerformanceHandler
	if performanceHandler == nil {
		performanceHandler = handler.NewPerformanceHandler(rc.Store.PerformanceMetrics())
	}
	sessionHandler := handler.NewSessionHandler(rc.Store.Events(), rc.Store.Recordings())

	sourceMapHandler.SetAllowedOrigins(rc.Config.Server.AllowedOrigins)

	// === Write API group (SDK data ingestion) ===
	// - Rate limit: 100 req/s
	// - CORS enabled
	// - No auth required (SDK uses API key in request body)
	writeGroup := NewRouteGroup(mux, "")
	writeGroup.Use(rc.CORS.Handler)
	writeGroup.Use(middleware.NewRateLimiter(100, time.Second).Handler)
	writeGroup.Handle("/api/report", reportHandler)
	writeGroup.Handle("/api/events", reportHandler)
	writeGroup.Handle("/api/report/screenshot", screenshotHandler)

	// === Public routes (no auth, special cases) ===
	mux.Handle("/api/screenshots/", rc.CORS.Handler(rc.JWT.Handler(screenshotFileHandler)))
	mux.Handle("/api/auth/login", rc.CORS.Handler(middleware.NewRateLimiter(5, time.Minute).Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHandler.Login(w, r)
	}))))
	mux.Handle("/api/health", rc.CORS.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		healthHandler.Health(w, r)
	})))
	mux.Handle("/api/docs", rc.CORS.Handler(http.HandlerFunc(openapiHandler.GetSpec)))
	mux.Handle("/api/docs/ui", rc.CORS.Handler(http.HandlerFunc(openapiHandler.GetSwaggerUI)))

	// E2E Verifier webhook (public, uses API key auth)
	e2eVerifier := webhook.NewE2EVerifierHook("", rc.DB, nil)
	mux.Handle("/api/webhooks/e2e-verifier", rc.CORS.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			e2eVerifier.HandleVerificationResult(w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})))
	mux.Handle("/api/webhooks/e2e-verifier/results", rc.CORS.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			e2eVerifier.GetVerificationResults(w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})))

	// === Read API group (query APIs with JWT auth) ===
	// - JWT auth required
	// - CORS enabled
	readGroup := NewRouteGroup(mux, "")
	readGroup.Use(rc.CORS.Handler)
	readGroup.Use(rc.JWT.Handler)

	// Auth endpoints
	readGroup.HandleFunc("GET /api/auth/me", authHandler.Me)
	readGroup.HandleFunc("PUT /api/auth/password", authHandler.ChangePassword)

	// Query - logs & stats
	readGroup.HandleFunc("GET /api/query/logs", queryHandler.QueryLogs)
	readGroup.HandleFunc("GET /api/query/stats", queryHandler.QueryStats)
	readGroup.HandleFunc("GET /api/query/apps", queryHandler.QueryApps)
	readGroup.HandleFunc("GET /api/query/top", queryHandler.QueryTop)
	readGroup.HandleFunc("GET /api/query/similar", queryHandler.QuerySimilar)
	readGroup.HandleFunc("GET /api/query/export", queryHandler.QueryExport)

	// Query - performance
	readGroup.HandleFunc("GET /api/query/performance/summary", queryHandler.QueryPerformanceSummary)
	readGroup.HandleFunc("GET /api/query/performance/trend", queryHandler.QueryPerformanceTrend)
	readGroup.HandleFunc("GET /api/query/performance/pages", queryHandler.QueryPerformancePages)
	readGroup.HandleFunc("GET /api/query/performance/regression", queryHandler.QueryPerformanceRegression)

	// Performance metrics - Web Vitals (R003)
	readGroup.HandleFunc("GET /api/query/performance/summary-by-page", performanceHandler.GetPerformanceSummary)
	readGroup.HandleFunc("GET /api/query/performance/trend-by-page", performanceHandler.GetPerformanceTrend)
	readGroup.HandleFunc("GET /api/query/performance/compare-releases", performanceHandler.GetPerformanceComparison)

	// Query - anomaly
	readGroup.HandleFunc("GET /api/query/anomaly/new-errors", queryHandler.QueryNewErrors)
	readGroup.HandleFunc("GET /api/query/anomaly/alert-triggers", queryHandler.QueryAlertTriggers)
	readGroup.HandleFunc("GET /api/query/anomaly/active-sessions", queryHandler.QueryActiveSessions)
	readGroup.HandleFunc("GET /api/query/stats/comparison", queryHandler.QueryStatsComparison)

	// Issues
	readGroup.HandleFunc("GET /api/query/issues", issuesHandler.GetIssues)
	readGroup.HandleFunc("GET /api/query/issues/", issuesHandler.GetIssue)
	readGroup.HandleFunc("PUT /api/query/issues/", issuesHandler.UpdateIssue)
	readGroup.HandleFunc("POST /api/query/issues/", func(w http.ResponseWriter, r *http.Request) {
		action := r.URL.Query().Get("action")
		if action == "resolve" {
			issuesHandler.ResolveIssue(w, r)
		} else if action == "ignore" {
			issuesHandler.IgnoreIssue(w, r)
		} else {
			http.Error(w, "Invalid action", http.StatusBadRequest)
		}
	})
	readGroup.HandleFunc("GET /api/query/issues/stats", issuesHandler.GetIssueStats)

	// Clusters
	readGroup.HandleFunc("GET /api/query/clusters", clustersHandler.GetClusters)

	// Health & analytics
	readGroup.HandleFunc("GET /api/query/release-health", healthHandler.GetReleaseHealth)
	readGroup.HandleFunc("GET /api/query/session-stats", healthHandler.GetSessionStats)

	// User session tracking
	readGroup.HandleFunc("GET /api/query/sessions", sessionHandler.ListSessions)
	readGroup.HandleFunc("GET /api/query/sessions/", sessionHandler.GetSessionDetail)

	// Alerts
	readGroup.HandleFunc("GET /api/query/alerts", alertsHandler.GetAlerts)
	readGroup.HandleFunc("POST /api/query/alerts", alertsHandler.CreateAlert)
	readGroup.HandleFunc("DELETE /api/query/alerts/", alertsHandler.DeleteAlert)
	readGroup.HandleFunc("POST /api/alerts/test", alertsHandler.TestAlert)

	// Rule Engine (R012)
	if rc.RuleEngineHandler != nil {
		rc.RuleEngineHandler.RegisterRoutes(mux)
	}
	if rc.ChannelManagerHandler != nil {
		rc.ChannelManagerHandler.RegisterRoutes(mux)
	}
	if rc.ClustererHandler != nil {
		rc.ClustererHandler.RegisterRoutes(mux)
	}

	// System
	readGroup.HandleFunc("GET /api/system/info", systemHandler.GetSystemInfo)
	readGroup.HandleFunc("POST /api/system/cleanup", systemHandler.TriggerCleanup)

	// Source maps (read)
	readGroup.HandleFunc("GET /api/sourcemaps", sourceMapHandler.List)
	readGroup.HandleFunc("GET /api/sourcemaps/download", sourceMapHandler.Download)
	readGroup.HandleFunc("DELETE /api/sourcemaps/", sourceMapHandler.Delete)
	readGroup.HandleFunc("POST /api/sourcemaps/deobfuscate", sourceMapHandler.Deobfuscate)
	readGroup.HandleFunc("POST /api/sourcemaps/resolve", sourceMapHandler.Resolve)

	// === Admin API group (admin operations with JWT + admin role) ===
	// - JWT auth required
	// - Admin role required
	// - CORS enabled
	adminGroup := NewRouteGroup(mux, "")
	adminGroup.Use(rc.CORS.Handler)
	adminGroup.Use(rc.JWT.Handler)
	adminGroup.Use(middleware.RequireAdmin)

	// Source map upload
	adminGroup.HandleFunc("POST /api/sourcemaps/upload", sourceMapHandler.Upload)

	// User management
	adminGroup.HandleFunc("GET /api/users", authHandler.ListUsers)
	adminGroup.HandleFunc("POST /api/users", authHandler.CreateUser)
	adminGroup.HandleFunc("PUT /api/users/", authHandler.UpdateUser)
	adminGroup.HandleFunc("DELETE /api/users/", authHandler.DeleteUser)

	// Storage governance
	adminGroup.HandleFunc("GET /api/admin/storage/stats", adminHandler.GetStorageStats)
	adminGroup.HandleFunc("GET /api/admin/retention/policy", adminHandler.GetRetentionPolicy)
	adminGroup.HandleFunc("PUT /api/admin/retention/policy", adminHandler.SetRetentionPolicy)
	adminGroup.HandleFunc("POST /api/admin/cleanup/manual", adminHandler.TriggerManualCleanup)

	// Projects
	adminGroup.HandleFunc("POST /api/admin/projects", projectsHandler.CreateProject)
	adminGroup.HandleFunc("GET /api/admin/projects", projectsHandler.ListProjects)
	adminGroup.HandleFunc("GET /api/admin/projects/", projectsHandler.GetProject)
	adminGroup.HandleFunc("PUT /api/admin/projects/", projectsHandler.UpdateProject)
	adminGroup.HandleFunc("DELETE /api/admin/projects/", projectsHandler.DeleteProject)
	adminGroup.HandleFunc("POST /api/admin/projects/api-key", projectsHandler.RegenerateApiKey)
	adminGroup.HandleFunc("GET /api/admin/projects/members", projectsHandler.ListMembers)
	adminGroup.HandleFunc("POST /api/admin/projects/members", projectsHandler.AddMember)
	adminGroup.HandleFunc("PUT /api/admin/projects/members/", projectsHandler.UpdateMemberRole)
	adminGroup.HandleFunc("DELETE /api/admin/projects/members/", projectsHandler.RemoveMember)

	// Webhooks
	adminGroup.HandleFunc("GET /api/admin/webhooks", webhooksHandler.GetWebhooks)
	adminGroup.HandleFunc("POST /api/admin/webhooks", webhooksHandler.CreateWebhook)
	adminGroup.HandleFunc("PUT /api/admin/webhooks/", webhooksHandler.UpdateWebhook)
	adminGroup.HandleFunc("DELETE /api/admin/webhooks/", webhooksHandler.DeleteWebhook)
	adminGroup.HandleFunc("POST /api/admin/webhooks/test", webhooksHandler.TestWebhook)

	// Audit logs
	adminGroup.HandleFunc("GET /api/admin/audit-logs", auditHandler.GetAuditLogs)

	// === Serve dashboard static files (SPA fallback) ===
	// This must come last so it doesn't interfere with API routes
	dashboardFS := http.FileServer(http.Dir("../dashboard"))
	mux.Handle("/", dashboardFS)

	return mux
}
