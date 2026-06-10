package worker

import (
	"context"
	"log/slog"
	"time"

	"github.com/logmonitor/collector/storage"
)

// CleanupResult represents the result of a cleanup operation (alias to storage.CleanupResult)
type CleanupResult = storage.CleanupResult

// CleanupSystemStore defines the interface for cleanup operations
type CleanupSystemStore interface {
	GetRetentionPolicy() (int, error)
	CleanupOldDataWithDays(days int) CleanupResult
	GetLastCleanupTime() int64
	SetLastCleanupTime(timestamp int64) error
}

// CleanupWorker handles retention cleanup
type CleanupWorker struct {
	retentionDays int
	checkInterval time.Duration
	systemStore   CleanupSystemStore
	stopChan      chan struct{}
	doneChan      chan struct{}
}

// NewCleanupWorker creates a new cleanup worker
func NewCleanupWorker(systemStore CleanupSystemStore, retentionDays int, checkInterval time.Duration) *CleanupWorker {
	return &CleanupWorker{
		retentionDays: retentionDays,
		checkInterval: checkInterval,
		systemStore:   systemStore,
		stopChan:      make(chan struct{}),
		doneChan:      make(chan struct{}),
	}
}

// Start begins the cleanup worker
func (w *CleanupWorker) Start(ctx context.Context) error {
	slog.Info("Starting cleanup worker", "retentionDays", w.retentionDays, "interval", w.checkInterval)

	ticker := time.NewTicker(w.checkInterval)
	defer ticker.Stop()

	// Run cleanup on startup
	w.runCleanup()

	for {
		select {
		case <-ticker.C:
			w.runCleanup()
		case <-ctx.Done():
			slog.Info("Cleanup worker context cancelled")
			return w.Stop()
		case <-w.stopChan:
			slog.Info("Cleanup worker stop signal received")
			return w.Stop()
		}
	}
}

// Stop stops the cleanup worker
func (w *CleanupWorker) Stop() error {
	close(w.stopChan)
	close(w.doneChan)
	slog.Info("Cleanup worker stopped")
	return nil
}

// Name returns the worker name
func (w *CleanupWorker) Name() string {
	return "CleanupWorker"
}

// runCleanup executes the cleanup operation
func (w *CleanupWorker) runCleanup() {
	slog.Debug("Running cleanup operation")

	// Get current retention policy from system store
	retentionDays, err := w.systemStore.GetRetentionPolicy()
	if err != nil {
		slog.Error("Failed to get retention policy, using default", "error", err)
		retentionDays = w.retentionDays
	}

	// Check if cleanup is needed (run once per day)
	lastCleanup := w.systemStore.GetLastCleanupTime()
	now := time.Now().UnixMilli()
	dayInMillis := 24 * time.Hour.Milliseconds()

	if lastCleanup > 0 && (now-lastCleanup) < dayInMillis {
		slog.Debug("Cleanup already ran recently, skipping", "lastCleanup", time.UnixMilli(lastCleanup))
		return
	}

	startTime := time.Now()

	// Perform cleanup
	result := w.systemStore.CleanupOldDataWithDays(retentionDays)
	result.Duration = time.Since(startTime)

	// Update last cleanup time (cleanup succeeded if we got here)
	if err := w.systemStore.SetLastCleanupTime(now); err != nil {
		slog.Error("Failed to update last cleanup time", "error", err)
	}
	slog.Info("Cleanup completed",
		"deletedEvents", result.DeletedEvents,
		"duration", result.Duration,
		"retentionDays", retentionDays)
}

// UpdateRetention updates the retention days
func (w *CleanupWorker) UpdateRetention(days int) {
	w.retentionDays = days
	slog.Info("Cleanup worker retention updated", "days", days)
}
