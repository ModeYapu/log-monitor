package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/logmonitor/collector/buffer"
	"github.com/logmonitor/collector/config"
	"github.com/logmonitor/collector/handler"
	"github.com/logmonitor/collector/internal/logger"
	"github.com/logmonitor/collector/middleware"
	"github.com/logmonitor/collector/storage"
	"github.com/logmonitor/collector/webhook"
	"github.com/logmonitor/collector/worker"
	"golang.org/x/crypto/bcrypt"
)

var (
	configPath = flag.String("config", "config.yaml", "Path to config file")
	version    = "1.0.0"
)

func main() {
	flag.Parse()

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		slog.Warn("Failed to load config, using defaults", "path", *configPath, "error", err)
		cfg = config.Default()
	}

	// Initialize logger
	logger.Init(logger.Config{
		Level:  logger.LevelInfo,
		Format: "text",
		Output: os.Stdout,
	})

	slog.Info("LogMonitor Collector starting", "version", version)
	slog.Info("Configuration", "port", cfg.Server.Port, "db", cfg.Database.Path, "retentionDays", cfg.Database.RetentionDays)

	// Initialize database store
	store, err := storage.NewSQLiteStore(storage.Config{
		Path:          cfg.Database.Path,
		RetentionDays: cfg.Database.RetentionDays,
	})
	if err != nil {
		slog.Error("Failed to initialize database", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := store.Close(); err != nil {
			slog.Error("Failed to close database", "error", err)
		}
	}()

	slog.Info("Database initialized successfully")

	// Get underlying DB for legacy handlers that still need direct access
	// This will be gradually removed as we migrate all handlers to use repositories
	db, err := storage.NewDB(storage.Config{
		Path:          cfg.Database.Path,
		RetentionDays: cfg.Database.RetentionDays,
	})
	if err != nil {
		slog.Error("Failed to initialize legacy DB", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	// Ensure cobrowsing tables exist
	db.EnsureCobrowseTables()

	// Ensure source maps table exists
	db.EnsureSourceMapsTable()

	// Initialize source map storage
	smStorage, err := storage.NewSourceMapStorage("./data")
	if err != nil {
		slog.Error("Failed to initialize source map storage", "error", err)
		os.Exit(1)
	}
	slog.Info("Source map storage initialized")

	// Initialize user storage and create users table
	userStorage := storage.NewUserStorage(db)
	if err := userStorage.EnsureUsersTable(); err != nil {
		slog.Error("Failed to create users table", "error", err)
		os.Exit(1)
	}

	// Seed admin user if no users exist
	if err := seedAdminUser(userStorage, &cfg.Auth); err != nil {
		slog.Error("Failed to seed admin user", "error", err)
		os.Exit(1)
	}

	// Initialize JWT middleware
	jwtMiddleware := middleware.NewJWT(cfg.Auth.JWTSecret, cfg.Auth.TokenExpireHours)
	if cfg.Auth.JWTSecret == "" {
		slog.Info("JWT secret: auto-generated (set auth.jwt_secret in config to persist)")
	} else {
		slog.Info("JWT secret: loaded from config")
	}

	// Initialize CORS middleware
	corsMiddleware := middleware.NewCORS(cfg.Server.AllowedOrigins)

	// Initialize system handler
	systemHandler := handler.NewSystemHandler(db, cfg.Database.Path, cfg.Database.RetentionDays)

	// Initialize admin handler
	adminHandler := handler.NewAdminHandler(db)

	// Initialize projects handler
	projectsHandler := handler.NewProjectsHandler(db, userStorage)

	// Initialize webhook manager and handler (Slice 4)
	webhookManager := webhook.NewManager(db, webhook.ManagerConfig{
		BufferSize:    100,
		FlushInterval: 5 * time.Second,
	})
	webhooksHandler := handler.NewWebhooksHandler(db, webhookManager)

	// Initialize OpenAPI handler (Slice 4)
	openapiSpec, err := os.ReadFile("api/openapi.yaml")
	if err != nil {
		slog.Warn("Failed to read OpenAPI spec", "error", err)
		// Continue without OpenAPI spec
		openapiSpec = []byte("openapi: 3.0.0\ninfo:\n  title: LogMonitor API\n  version: 1.0.0")
	}
	openapiHandler := handler.NewOpenAPIHandler(openapiSpec)

	// Auto-create default project if none exist
	if err := db.AutoCreateDefaultProject(); err != nil {
		slog.Error("Failed to auto-create default project", "error", err)
		// Don't exit on error, just log it
	} else {
		slog.Info("Default project check completed")
	}

	// Initialize buffer writer
	writer := buffer.NewWriter(store.Events(), buffer.Config{
		BufferSize:    cfg.Buffer.Size,
		FlushInterval: time.Duration(cfg.Buffer.FlushInterval) * time.Millisecond,
		BatchSize:     cfg.Buffer.FlushBatchSize,
	})
	defer func() {
		if err = writer.Close(); err != nil {
			slog.Error("Failed to close writer", "error", err)
		}
	}()

	slog.Info("Buffer writer initialized", "size", cfg.Buffer.Size, "intervalMs", cfg.Buffer.FlushInterval, "batch", cfg.Buffer.FlushBatchSize)

	// Setup HTTP handlers with route groups
	mux := http.NewServeMux()

	// Public routes (no authentication required)
	reportHandler := handler.NewReportHandler(writer, &cfg.Server, db)
	mux.Handle("/api/report", reportHandler)
	mux.Handle("/api/events", reportHandler)
	mux.Handle("/api/report/screenshot", handler.NewScreenshotHandler("./data/screenshots"))
	mux.Handle("/api/screenshots/", corsMiddleware.Handler(jwtMiddleware.Handler(handler.NewScreenshotFileHandler("./data/screenshots"))))
	mux.Handle("/api/auth/login", corsMiddleware.Handler(middleware.NewRateLimiter(5, time.Minute).Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler.NewAuthHandler(userStorage, jwtMiddleware).Login(w, r)
	}))))
	mux.Handle("/api/health", corsMiddleware.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler.NewQueryHandler(db).Health(w, r)
	})))
	mux.Handle("/api/docs", corsMiddleware.Handler(http.HandlerFunc(openapiHandler.GetSpec)))
	mux.Handle("/api/docs/ui", corsMiddleware.Handler(http.HandlerFunc(openapiHandler.GetSwaggerUI)))

	// Protected routes (require authentication)
	queryHandler := handler.NewQueryHandler(db)
	authHandler := handler.NewAuthHandler(userStorage, jwtMiddleware)
	clustersHandler := handler.NewClustersHandler(store.Events())
	sourceMapHandler := handler.NewSourceMapHandler(db, smStorage)
	healthHandler := handler.NewHealthHandler(db)
	issuesHandler := handler.NewIssuesHandler(db)
	sourceMapHandler.SetAllowedOrigins(cfg.Server.AllowedOrigins)

	// API routes that require JWT authentication
	authRoutes := []struct {
		pattern string
		handler http.HandlerFunc
	}{
		{"GET /api/auth/me", authHandler.Me},
		{"PUT /api/auth/password", authHandler.ChangePassword},
		{"GET /api/query/logs", queryHandler.QueryLogs},
		{"GET /api/query/stats", queryHandler.QueryStats},
		{"GET /api/query/apps", queryHandler.QueryApps},
			{"GET /api/query/top", queryHandler.QueryTop},
			{"GET /api/query/similar", queryHandler.QuerySimilar},
			{"GET /api/query/export", queryHandler.QueryExport},
			{"GET /api/query/performance/summary", queryHandler.QueryPerformanceSummary},
			{"GET /api/query/performance/trend", queryHandler.QueryPerformanceTrend},
			{"GET /api/query/performance/pages", queryHandler.QueryPerformancePages},
			{"GET /api/query/performance/regression", queryHandler.QueryPerformanceRegression},
			{"GET /api/query/anomaly/new-errors", queryHandler.QueryNewErrors},
			{"GET /api/query/anomaly/alert-triggers", queryHandler.QueryAlertTriggers},
			{"GET /api/query/anomaly/active-sessions", queryHandler.QueryActiveSessions},
			{"GET /api/query/stats/comparison", queryHandler.QueryStatsComparison},
			{"GET /api/query/issues", issuesHandler.GetIssues},
			{"GET /api/query/issues/", issuesHandler.GetIssue},
			{"PUT /api/query/issues/", issuesHandler.UpdateIssue},
			{"POST /api/query/issues/", func(w http.ResponseWriter, r *http.Request) {
				// Handle resolve/ignore actions based on query parameter
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
		{"GET /api/query/clusters", clustersHandler.GetClusters},
		{"GET /api/query/release-health", healthHandler.GetReleaseHealth},
		{"GET /api/query/session-stats", healthHandler.GetSessionStats},
		{"GET /api/query/alerts", handler.NewAlertsHandler(db).GetAlerts},
		{"POST /api/query/alerts", handler.NewAlertsHandler(db).CreateAlert},
		{"DELETE /api/query/alerts/", handler.NewAlertsHandler(db).DeleteAlert},
		{"POST /api/alerts/test", handler.NewAlertsHandler(db).TestAlert},
		{"GET /api/system/info", systemHandler.GetSystemInfo},
		{"POST /api/system/cleanup", systemHandler.TriggerCleanup},
		{"GET /api/sourcemaps", sourceMapHandler.List},
		{"GET /api/sourcemaps/download", sourceMapHandler.Download},
		{"DELETE /api/sourcemaps/", sourceMapHandler.Delete},
		{"POST /api/sourcemaps/deobfuscate", sourceMapHandler.Deobfuscate},
	}

	// Admin-only routes for source map upload
	adminRoutes := []struct {
		pattern string
		handler http.HandlerFunc
	}{
		{"POST /api/sourcemaps/upload", sourceMapHandler.Upload},
		{"GET /api/users", authHandler.ListUsers},
		{"POST /api/users", authHandler.CreateUser},
		{"PUT /api/users/", authHandler.UpdateUser},
		{"DELETE /api/users/", authHandler.DeleteUser},
		// Slice 4: Admin storage governance endpoints
		{"GET /api/admin/storage/stats", adminHandler.GetStorageStats},
		{"GET /api/admin/retention/policy", adminHandler.GetRetentionPolicy},
		{"PUT /api/admin/retention/policy", adminHandler.SetRetentionPolicy},
		{"POST /api/admin/cleanup/manual", adminHandler.TriggerManualCleanup},
			// Slice 2: Multi-tenant project endpoints
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
			// Slice 4: Webhook management endpoints
			{"GET /api/admin/webhooks", webhooksHandler.GetWebhooks},
			{"POST /api/admin/webhooks", webhooksHandler.CreateWebhook},
			{"PUT /api/admin/webhooks/", webhooksHandler.UpdateWebhook},
			{"DELETE /api/admin/webhooks/", webhooksHandler.DeleteWebhook},
			{"POST /api/admin/webhooks/test", webhooksHandler.TestWebhook},
	}

	for _, route := range authRoutes {
		pattern := route.pattern
		handler := corsMiddleware.Handler(jwtMiddleware.Handler(http.HandlerFunc(route.handler)))
		mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
			handler.ServeHTTP(w, r)
		})
	}

	// Admin-only routes
	for _, route := range adminRoutes {
		pattern := route.pattern
		handler := corsMiddleware.Handler(jwtMiddleware.Handler(middleware.RequireAdmin(http.HandlerFunc(route.handler))))
		mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
			handler.ServeHTTP(w, r)
		})
	}

	// Initialize worker manager
	workerManager := worker.NewManager()

	// Register cleanup worker
	cleanupWorker := worker.NewCleanupWorker(
		store.System(),
		cfg.Database.RetentionDays,
		24*time.Hour, // Check daily
	)
	workerManager.RegisterWorker(cleanupWorker)

	// Register issue aggregator worker
	issueAggregatorWorker := worker.NewIssueAggregatorWorker(
		store.Issues(),
		store.Events(),
		5*time.Minute, // Check every 5 minutes
	)
	workerManager.RegisterWorker(issueAggregatorWorker)

	// Register alert checker worker
	alertCheckerWorker := worker.NewAlertCheckerWorker(
		store.Alerts(),
		store.Events(),
		time.Duration(cfg.Alert.CheckInterval)*time.Millisecond,
	)

	// Configure email notifications for alert checker
	alertCheckerWorker.SetEmailConfig(worker.EmailConfig{
		Enabled:   cfg.Alert.Email.Enabled,
		SMTPHost:  cfg.Alert.Email.SMTPHost,
		SMTPPort:  cfg.Alert.Email.SMTPPort,
		SMTPUser:  cfg.Alert.Email.SMTPUser,
		SMTPPass:  cfg.Alert.Email.SMTPPass,
		FromEmail: cfg.Alert.Email.FromEmail,
		FromName:  cfg.Alert.Email.FromName,
	})
	workerManager.RegisterWorker(alertCheckerWorker)

	// Start worker manager
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := workerManager.Start(ctx); err != nil {
		slog.Error("Failed to start worker manager", "error", err)
		os.Exit(1)
	}
	defer workerManager.Stop()

	slog.Info("Worker manager started", "workers", workerManager.WorkerCount())

	// Start webhook manager (Slice 4)
	// Note: Webhook manager runs in background goroutines, no need to start explicitly
	defer webhookManager.Stop()
	slog.Info("Webhook manager initialized")

	// Initialize config watcher
	configWatcher := config.NewWatcher(*configPath, func(oldCfg, newCfg *config.Config) error {
		slog.Info("Config reloaded", "retentionDays", newCfg.Database.RetentionDays)

		// Update cleanup worker if retention days changed
		if oldCfg.Database.RetentionDays != newCfg.Database.RetentionDays {
			cleanupWorker.UpdateRetention(newCfg.Database.RetentionDays)
		}

		return nil
	})

	// Start config watcher
	if err := configWatcher.Start(cfg); err != nil {
		slog.Warn("Failed to start config watcher, continuing without hot reload", "error", err)
	}
	defer configWatcher.Stop()

	// Initialize cobrowse hub
	cobrowseHub := handler.NewCoBrowseHub(db)

	// Configure auth from config (for cobrowse - legacy token-based auth)
	if len(cfg.Server.AdminTokens) > 0 {
		auth := &middleware.AuthConfig{
			AdminTokens: make(map[string]bool),
			UserTokens:  make(map[string]bool),
			Enabled:     true,
		}
		for _, t := range cfg.Server.AdminTokens {
			auth.AddAdminToken(t)
		}
		auth.SetJWTValidator(jwtMiddleware)
		cobrowseHub.SetAuthConfig(auth)
		slog.Info("Legacy cobrowse auth enabled", "adminTokens", len(cfg.Server.AdminTokens))
	} else {
		auth := &middleware.AuthConfig{
			AdminTokens: make(map[string]bool),
			UserTokens:  make(map[string]bool),
			Enabled:     true,
		}
		auth.SetJWTValidator(jwtMiddleware)
		cobrowseHub.SetAuthConfig(auth)
		slog.Info("Cobrowse admin access requires JWT login (no legacy admin_tokens configured)")
	}
	cobrowseHub.SetAllowedOrigins(cfg.Server.AllowedOrigins)
	defer cobrowseHub.Close()

	// Register cobrowse WebSocket routes (with JWT auth support)
	cobrowseHub.RegisterRoutes(mux)

	// Register cobrowse HTTP API routes
	recordingHandler := handler.NewRecordingHandler(cobrowseHub, db)
	recordingHandler.RegisterRoutes(mux)

	slog.Info("Cobrowse hub initialized")

	// Apply CORS middleware to all routes
	handlerWithCORS := corsMiddleware.Handler(mux)

	// Server with timeout
	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:      handlerWithCORS,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	go func() {
		slog.Info("HTTP server listening", "port", cfg.Server.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Server failed", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("Shutting down server...")

	// Graceful shutdown
	ctx, cancel = context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		slog.Error("Server shutdown error", "error", err)
	}

	slog.Info("Server stopped")
}

// seedAdminUser creates the default admin user if no users exist
func seedAdminUser(userStorage *storage.UserStorage, authCfg *config.AuthConfig) error {
	count, err := userStorage.CountUsers()
	if err != nil {
		return fmt.Errorf("failed to count users: %w", err)
	}

	if count > 0 {
		slog.Info("Existing users found, skipping admin seed", "count", count)
		return nil
	}

	// No users exist, create default admin
	password := authCfg.DefaultPassword
	if password == "" {
		password = "admin123"
	}

	slog.Info("Creating default admin user", "username", "admin", "note", "change default password after first login")

	// Hash password
	hashedPassword, err := hashPassword(password)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	// Create admin user
	userID, err := userStorage.CreateUser("admin", hashedPassword, "Administrator", "admin")
	if err != nil {
		return fmt.Errorf("failed to create admin user: %w", err)
	}

	slog.Info("Admin user created successfully", "userID", userID)
	slog.Info("IMPORTANT: Please change the default admin password after first login!")

	return nil
}

// hashPassword hashes a password using bcrypt
func hashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}
