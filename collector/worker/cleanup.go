package worker

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/logmonitor/collector/storage"
)

// CleanupResult represents the result of a cleanup operation (alias to storage.CleanupResult)
type CleanupResult = storage.CleanupResult

// CleanupSystemStore defines the interface for cleanup operations
type CleanupSystemStore interface {
	GetRetentionPolicySimple() (int, error)
	CleanupOldDataWithDays(days int) CleanupResult
	GetLastCleanupTime() int64
	SetLastCleanupTime(timestamp int64) error
	DeleteEventsBefore(before time.Time) (int64, error)
	DeleteRecordingsBefore(before time.Time) (int64, error)
}

// CleanupWorker handles retention cleanup
type CleanupWorker struct {
	retentionDays      int
	recordingDays      int
	screenshotDays     int
	checkInterval      time.Duration
	systemStore        CleanupSystemStore
	screenshotDir      string
	stopChan           chan struct{}
	doneChan           chan struct{}
}

// NewCleanupWorker creates a new cleanup worker
func NewCleanupWorker(systemStore CleanupSystemStore, retentionDays int, checkInterval time.Duration) *CleanupWorker {
	return &CleanupWorker{
		retentionDays:  retentionDays,
		recordingDays: 14,  // Default 14 days for recordings
		screenshotDays: 30, // Default 30 days for screenshots
		checkInterval:  checkInterval,
		systemStore:    systemStore,
		stopChan:       make(chan struct{}),
		doneChan:       make(chan struct{}),
	}
}

// SetScreenshotDir sets the screenshot directory for cleanup
func (w *CleanupWorker) SetScreenshotDir(dir string) {
	w.screenshotDir = dir
}

// SetRecordingRetention sets the recording retention days
func (w *CleanupWorker) SetRecordingRetention(days int) {
	w.recordingDays = days
	slog.Info("Recording retention updated", "days", days)
}

// SetScreenshotRetention sets the screenshot retention days
func (w *CleanupWorker) SetScreenshotRetention(days int) {
	w.screenshotDays = days
	slog.Info("Screenshot retention updated", "days", days)
}

// Start begins the cleanup worker
func (w *CleanupWorker) Start(ctx context.Context) error {
	slog.Info("Starting cleanup worker",
		"eventsRetentionDays", w.retentionDays,
		"recordingsRetentionDays", w.recordingDays,
		"screenshotsRetentionDays", w.screenshotDays,
		"interval", w.checkInterval)

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
	retentionDays, err := w.systemStore.GetRetentionPolicySimple()
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

	// Clean events
	eventsCutoff := time.Now().AddDate(0, 0, -retentionDays)
	deletedEvents, err := w.systemStore.DeleteEventsBefore(eventsCutoff)
	if err != nil {
		slog.Error("Failed to delete old events", "error", err)
	} else if deletedEvents > 0 {
		slog.Info("Deleted old events", "count", deletedEvents, "olderThan", retentionDays)
	}

	// Clean recordings
	recordingsCutoff := time.Now().AddDate(0, 0, -w.recordingDays)
	deletedRecordings, err := w.systemStore.DeleteRecordingsBefore(recordingsCutoff)
	if err != nil {
		slog.Error("Failed to delete old recordings", "error", err)
	} else if deletedRecordings > 0 {
		slog.Info("Deleted old recordings", "count", deletedRecordings, "olderThan", w.recordingDays)
	}

	// Clean screenshots
	deletedScreenshots := w.cleanupScreenshots()

	duration := time.Since(startTime)

	// Update last cleanup time
	if err := w.systemStore.SetLastCleanupTime(now); err != nil {
		slog.Error("Failed to update last cleanup time", "error", err)
	}

	slog.Info("Cleanup completed",
		"deletedEvents", deletedEvents,
		"deletedRecordings", deletedRecordings,
		"deletedScreenshots", deletedScreenshots,
		"duration", duration,
		"retentionDays", retentionDays)
}

// cleanupScreenshots deletes screenshot files older than the retention period
func (w *CleanupWorker) cleanupScreenshots() int64 {
	if w.screenshotDir == "" {
		return 0
	}

	cutoff := time.Now().AddDate(0, 0, -w.screenshotDays)
	var deletedCount int64

	err := filepath.Walk(w.screenshotDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		// Only process image files
		ext := filepath.Ext(path)
		if ext != ".png" && ext != ".jpg" && ext != ".jpeg" && ext != ".gif" && ext != ".webp" {
			return nil
		}

		// Check if file is older than cutoff
		if info.ModTime().Before(cutoff) {
			if err := os.Remove(path); err != nil {
				slog.Warn("Failed to delete old screenshot", "path", path, "error", err)
			} else {
				deletedCount++
				slog.Debug("Deleted old screenshot", "path", path, "age", time.Since(info.ModTime()))
			}
		}
		return nil
	})

	if err != nil {
		slog.Error("Failed to walk screenshot directory", "error", err)
	}

	if deletedCount > 0 {
		slog.Info("Deleted old screenshots", "count", deletedCount, "olderThan", w.screenshotDays)
	}

	return deletedCount
}

// UpdateRetention updates the retention days
func (w *CleanupWorker) UpdateRetention(days int) {
	w.retentionDays = days
	slog.Info("Cleanup worker retention updated", "days", days)
}
