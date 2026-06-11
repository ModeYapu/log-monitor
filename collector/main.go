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

	// Initialize admin handler

	// Initialize projects handler

	// Initialize webhook manager and handler (Slice 4)
	webhookManager := webhook.NewManager(db, webhook.ManagerConfig{
		BufferSize:    100,
		FlushInterval: 5 * time.Second,
	})

	// Initialize OpenAPI handler (Slice 4)
	openapiSpec, err := os.ReadFile("api/openapi.yaml")
	if err != nil {
		slog.Warn("Failed to read OpenAPI spec", "error", err)
		// Continue without OpenAPI spec
		openapiSpec = []byte("openapi: 3.0.0\ninfo:\n  title: LogMonitor API\n  version: 1.0.0")
	}

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

	// Setup HTTP routes
	mux := SetupRoutes(&RouterConfig{
		DB:             db,
		Store:          store,
		UserStorage:    userStorage,
		SMStorage:      smStorage,
		Writer:         writer,
		Config:         cfg,
		JWT:            jwtMiddleware,
		CORS:           corsMiddleware,
		WebhookManager: webhookManager,
		OpenAPISpec:    openapiSpec,
	})

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

	// Start persistent webhook retry worker
	persistentQueue := webhook.NewPersistentQueue(db)
	retryWorker := webhook.NewRetryWorker(persistentQueue, db)
	go retryWorker.Start(30 * time.Second)
	defer retryWorker.Stop()
	slog.Info("Webhook persistent retry worker started")

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
