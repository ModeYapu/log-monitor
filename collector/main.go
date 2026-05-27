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

	// Setup HTTP handlers
	mux := http.NewServeMux()

	// Report endpoint
	reportHandler := handler.NewReportHandler(writer)
	mux.Handle("/api/report", reportHandler)
	mux.Handle("/api/events", reportHandler)

	// Screenshot endpoint
	screenshotHandler := handler.NewScreenshotHandler("./data/screenshots")
	mux.Handle("/api/report/screenshot", screenshotHandler)

	// Serve screenshot files
	mux.Handle("/api/screenshots/", http.StripPrefix("/api/screenshots/", http.FileServer(http.Dir("./data/screenshots"))))

	// Query endpoints
	queryHandler := handler.NewQueryHandler(db)
	queryHandler.RegisterRoutes(mux)

	// Alerts endpoints
	alertsHandler := handler.NewAlertsHandler(db)
	alertsHandler.RegisterRoutes(mux)

	// Initialize alert checker
	alertChecker := alerter.NewChecker(db)
	go alertChecker.Start(time.Duration(cfg.Alert.CheckInterval) * time.Millisecond)
	defer alertChecker.Stop()

	log.Printf("Alert checker started: interval=%dms", cfg.Alert.CheckInterval)

	// Initialize cobrowse hub
	cobrowseHub := handler.NewCoBrowseHub(db)

	// Configure auth from config
	if len(cfg.Server.AdminTokens) > 0 {
		auth := &middleware.AuthConfig{
			AdminTokens: make(map[string]bool),
			UserTokens:  make(map[string]bool),
			Enabled:     true,
		}
		for _, t := range cfg.Server.AdminTokens {
			auth.AddAdminToken(t)
		}
		cobrowseHub.SetAuthConfig(auth)
		log.Printf("Auth enabled with %d admin token(s)", len(cfg.Server.AdminTokens))
	} else {
		// No tokens configured = auth disabled
		auth := &middleware.AuthConfig{Enabled: false}
		cobrowseHub.SetAuthConfig(auth)
		log.Println("Auth disabled (no admin_tokens configured)")
	}
	defer cobrowseHub.Close()

	// Register cobrowse WebSocket routes
	cobrowseHub.RegisterRoutes(mux)

	// Register cobrowse HTTP API routes
	recordingHandler := handler.NewRecordingHandler(cobrowseHub, db)
	recordingHandler.RegisterRoutes(mux)

	log.Println("Cobrowse hub initialized")

	// Server with timeout
	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:      mux,
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
