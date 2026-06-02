package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/logmonitor/collector/alerter"
	"github.com/logmonitor/collector/buffer"
	"github.com/logmonitor/collector/config"
	"github.com/logmonitor/collector/handler"
	"github.com/logmonitor/collector/middleware"
	"github.com/logmonitor/collector/storage"
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
		log.Printf("Failed to load config from %s, using defaults: %v", *configPath, err)
		cfg = config.Default()
	}

	log.Printf("LogMonitor Collector v%s starting...", version)
	log.Printf("Config: port=%d, db=%s, retention=%d days",
		cfg.Server.Port, cfg.Database.Path, cfg.Database.RetentionDays)

	// Initialize database
	db, err := storage.NewDB(storage.Config{
		Path:          cfg.Database.Path,
		RetentionDays: cfg.Database.RetentionDays,
	})
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Printf("Failed to close database: %v", err)
		}
	}()

	log.Println("Database initialized successfully")

	// Ensure cobrowsing tables exist
	db.EnsureCobrowseTables()

	// Ensure source maps table exists
	db.EnsureSourceMapsTable()

	// Initialize source map storage
	smStorage, err := storage.NewSourceMapStorage("./data")
	if err != nil {
		log.Fatalf("Failed to initialize source map storage: %v", err)
	}
	log.Println("Source map storage initialized")

	// Initialize user storage and create users table
	userStorage := storage.NewUserStorage(db)
	if err := userStorage.EnsureUsersTable(); err != nil {
		log.Fatalf("Failed to create users table: %v", err)
	}

	// Seed admin user if no users exist
	if err := seedAdminUser(userStorage, &cfg.Auth); err != nil {
		log.Fatalf("Failed to seed admin user: %v", err)
	}

	// Initialize JWT middleware
	jwtMiddleware := middleware.NewJWT(cfg.Auth.JWTSecret, cfg.Auth.TokenExpireHours)
	if cfg.Auth.JWTSecret == "" {
		log.Printf("JWT secret: auto-generated (set auth.jwt_secret in config to persist)")
	} else {
		log.Printf("JWT secret: loaded from config")
	}

	// Initialize CORS middleware
	corsMiddleware := middleware.NewCORS(cfg.Server.AllowedOrigins)

	// Initialize system handler
	systemHandler := handler.NewSystemHandler(db, cfg.Database.Path, cfg.Database.RetentionDays)

	// Initialize buffer writer
	writer := buffer.NewWriter(db, buffer.Config{
		BufferSize:    cfg.Buffer.Size,
		FlushInterval: time.Duration(cfg.Buffer.FlushInterval) * time.Millisecond,
		BatchSize:     cfg.Buffer.FlushBatchSize,
	})
	defer func() {
		if err := writer.Close(); err != nil {
			log.Printf("Failed to close writer: %v", err)
		}
	}()

	log.Printf("Buffer writer initialized: size=%d, interval=%dms, batch=%d",
		cfg.Buffer.Size, cfg.Buffer.FlushInterval, cfg.Buffer.FlushBatchSize)

	// Setup HTTP handlers with route groups
	mux := http.NewServeMux()

	// Public routes (no authentication required)
	reportHandler := handler.NewReportHandler(writer, &cfg.Server)
	mux.Handle("/api/report", reportHandler)
	mux.Handle("/api/events", reportHandler)
	mux.Handle("/api/report/screenshot", handler.NewScreenshotHandler("./data/screenshots"))
	mux.Handle("/api/screenshots/", corsMiddleware.Handler(jwtMiddleware.Handler(handler.NewScreenshotFileHandler("./data/screenshots"))))
	mux.Handle("/api/auth/login", corsMiddleware.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler.NewAuthHandler(userStorage, jwtMiddleware).Login(w, r)
	})))
	mux.Handle("/api/health", corsMiddleware.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler.NewQueryHandler(db).Health(w, r)
	})))

	// Protected routes (require authentication)
	queryHandler := handler.NewQueryHandler(db)
	authHandler := handler.NewAuthHandler(userStorage, jwtMiddleware)
	sourceMapHandler := handler.NewSourceMapHandler(db, smStorage)
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

	for _, route := range adminRoutes {
		pattern := route.pattern
		handler := corsMiddleware.Handler(jwtMiddleware.Handler(middleware.RequireAdmin(http.HandlerFunc(route.handler))))
		mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
			handler.ServeHTTP(w, r)
		})
	}

	// Initialize alert checker
	alertChecker := alerter.NewChecker(db)
	go alertChecker.Start(time.Duration(cfg.Alert.CheckInterval) * time.Millisecond)
	defer alertChecker.Stop()

	log.Printf("Alert checker started: interval=%dms", cfg.Alert.CheckInterval)

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
		log.Printf("Legacy cobrowse auth enabled with %d admin token(s)", len(cfg.Server.AdminTokens))
	} else {
		auth := &middleware.AuthConfig{
			AdminTokens: make(map[string]bool),
			UserTokens:  make(map[string]bool),
			Enabled:     true,
		}
		auth.SetJWTValidator(jwtMiddleware)
		cobrowseHub.SetAuthConfig(auth)
		log.Println("Cobrowse admin access requires JWT login (no legacy admin_tokens configured)")
	}
	cobrowseHub.SetAllowedOrigins(cfg.Server.AllowedOrigins)
	defer cobrowseHub.Close()

	// Register cobrowse WebSocket routes (with JWT auth support)
	cobrowseHub.RegisterRoutes(mux)

	// Register cobrowse HTTP API routes
	recordingHandler := handler.NewRecordingHandler(cobrowseHub, db)
	recordingHandler.RegisterRoutes(mux)

	log.Println("Cobrowse hub initialized")

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
		log.Printf("HTTP server listening on :%d", cfg.Server.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}

	log.Println("Server stopped")
}

// seedAdminUser creates the default admin user if no users exist
func seedAdminUser(userStorage *storage.UserStorage, authCfg *config.AuthConfig) error {
	count, err := userStorage.CountUsers()
	if err != nil {
		return fmt.Errorf("failed to count users: %w", err)
	}

	if count > 0 {
		log.Printf("Found %d existing user(s), skipping admin seed", count)
		return nil
	}

	// No users exist, create default admin
	password := authCfg.DefaultPassword
	if password == "" {
		password = "admin123"
	}

	log.Printf("Creating default admin user (username: admin, password: %s)", password)

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

	log.Printf("Admin user created successfully (ID: %d)", userID)
	log.Println("IMPORTANT: Please change the default admin password after first login!")

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
