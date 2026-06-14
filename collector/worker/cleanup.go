package worker

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
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
	mu             sync.RWMutex // guards the retention/screenshot config fields below
	retentionDays  int
	recordingDays  int
	screenshotDays int
	screenshotDir  string

	checkInterval time.Duration
	systemStore   CleanupSystemStore
	stopChan      chan struct{}
	doneChan      chan struct{}
	stopOnce      sync.Once // guards stopChan/doneChan close against double-close panic
}

// NewCleanupWorker creates a new cleanup worker
func NewCleanupWorker(systemStore CleanupSystemStore, retentionDays int, checkInterval time.Duration) *CleanupWorker {
	return &CleanupWorker{
		retentionDays:  retentionDays,
		recordingDays:  14, // Default 14 days for recordings
		screenshotDays: 30, // Default 30 days for screenshots
		checkInterval:  checkInterval,
		systemStore:    systemStore,
		stopChan:       make(chan struct{}),
		doneChan:       make(chan struct{}),
	}
}

// SetScreenshotDir sets the screenshot directory for cleanup
func (w *CleanupWorker) SetScreenshotDir(dir string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.screenshotDir = dir
}

// SetRecordingRetention sets the recording retention days
func (w *CleanupWorker) SetRecordingRetention(days int) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.recordingDays = days
	slog.Info("Recording retention updated", "days", days)
}

// SetScreenshotRetention sets the screenshot retention days
func (w *CleanupWorker) SetScreenshotRetention(days int) {
	w.mu.Lock()
	defer w.mu.Unlock()
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

// Stop stops the cleanup worker. It is idempotent: Start() calls Stop() when
// its stop channel fires, and the manager calls Stop() during shutdown, so the
// channel closes are guarded by sync.Once to avoid a double-close panic.
func (w *CleanupWorker) Stop() error {
	w.stopOnce.Do(func() {
		close(w.stopChan)
		close(w.doneChan)
	})
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

	// Snapshot mutable retention/screenshot config under the lock so the
	// config-watcher goroutine can update it concurrently without a data race.
	defaultRetention, recordingDays, screenshotDays, screenshotDir := w.snapshotConfig()

	// Get current retention policy from system store
	retentionDays, err := w.systemStore.GetRetentionPolicySimple()
	if err != nil {
		slog.Error("Failed to get retention policy, using default", "error", err)
		retentionDays = defaultRetention
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
	recordingsCutoff := time.Now().AddDate(0, 0, -recordingDays)
	deletedRecordings, err := w.systemStore.DeleteRecordingsBefore(recordingsCutoff)
	if err != nil {
		slog.Error("Failed to delete old recordings", "error", err)
	} else if deletedRecordings > 0 {
		slog.Info("Deleted old recordings", "count", deletedRecordings, "olderThan", recordingDays)
	}

	// Clean screenshots
	deletedScreenshots := w.cleanupScreenshots(screenshotDir, screenshotDays)

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

// cleanupScreenshots deletes screenshot files older than the retention period.
// dir/days are passed in (already snapshotted under the lock) to avoid a data
// race with the config-watcher goroutine.
func (w *CleanupWorker) cleanupScreenshots(dir string, days int) int64 {
	if dir == "" {
		return 0
	}

	cutoff := time.Now().AddDate(0, 0, -days)
	var deletedCount int64

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
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
		slog.Info("Deleted old screenshots", "count", deletedCount, "olderThan", days)
	}

	return deletedCount
}

// UpdateRetention updates the retention days
func (w *CleanupWorker) UpdateRetention(days int) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.retentionDays = days
	slog.Info("Cleanup worker retention updated", "days", days)
}

// snapshotConfig returns a consistent snapshot of the mutable retention/screenshot
// configuration under the read lock, eliminating data races with the setters
// (which run on the config-watcher goroutine) and the cleanup loop.
func (w *CleanupWorker) snapshotConfig() (retention, recording, screenshot int, screenshotDir string) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.retentionDays, w.recordingDays, w.screenshotDays, w.screenshotDir
}
