package worker

import (
	"testing"
	"time"

	"github.com/logmonitor/collector/storage"
)

// TestCleanupWorker_DeletesOldEvents verifies the TTL cleanup deletes events older
// than the retention window while keeping recent events (R013).
func TestCleanupWorker_DeletesOldEvents(t *testing.T) {
	db := newCleanupTestDB(t)
	defer db.Close()

	now := time.Now()
	oldMs := now.Add(-40 * 24 * time.Hour).UnixMilli() // 40 days ago (> 30-day retention)
	newMs := now.Add(-1 * 24 * time.Hour).UnixMilli()  // 1 day ago (within retention)

	events := []storage.EventRecord{
		{AppID: "app-1", Type: "error", Level: "error", Message: "old-1", CreatedAt: oldMs},
		{AppID: "app-1", Type: "error", Level: "error", Message: "old-2", CreatedAt: oldMs},
		{AppID: "app-1", Type: "info", Level: "info", Message: "new-1", CreatedAt: newMs},
		{AppID: "app-1", Type: "info", Level: "info", Message: "new-2", CreatedAt: newMs},
	}
	if err := db.InsertEvents(events); err != nil {
		t.Fatalf("failed to seed events: %v", err)
	}

	// Fresh DB: lastCleanupTime is 0, so runCleanup will execute.
	worker := NewCleanupWorker(db, 30, time.Hour)
	worker.runCleanup()

	// Query remaining events: the 2 recent ones should survive, the 2 old ones gone.
	remaining, err := db.QueryEvents(storage.QueryParams{
		AppID: "app-1", Page: 1, PageSize: 100,
	})
	if err != nil {
		t.Fatalf("failed to query remaining events: %v", err)
	}
	if remaining.Total != 2 {
		t.Errorf("expected 2 events remaining after cleanup, got %d", remaining.Total)
	}

	// lastCleanupTime should now be recorded so the next run within a day is skipped.
	if db.GetLastCleanupTime() == 0 {
		t.Error("expected lastCleanupTime to be recorded after cleanup")
	}
}

// TestCleanupWorker_SkipsRecentRun verifies the daily throttle: a second run
// within 24h is a no-op (no extra deletion).
func TestCleanupWorker_SkipsRecentRun(t *testing.T) {
	db := newCleanupTestDB(t)
	defer db.Close()

	now := time.Now()
	if err := db.InsertEvents([]storage.EventRecord{
		{AppID: "app-1", Type: "error", Level: "error", Message: "old", CreatedAt: now.Add(-40 * 24 * time.Hour).UnixMilli()},
	}); err != nil {
		t.Fatalf("failed to seed: %v", err)
	}

	worker := NewCleanupWorker(db, 30, time.Hour)
	worker.runCleanup() // deletes the old event + records lastCleanupTime

	remaining, _ := db.QueryEvents(storage.QueryParams{AppID: "app-1", Page: 1, PageSize: 100})
	if remaining.Total != 0 {
		t.Fatalf("expected 0 events after first cleanup, got %d", remaining.Total)
	}

	// Insert another old event and run again within the same day: it must NOT be deleted.
	if err := db.InsertEvents([]storage.EventRecord{
		{AppID: "app-1", Type: "error", Level: "error", Message: "old-2", CreatedAt: now.Add(-40 * 24 * time.Hour).UnixMilli()},
	}); err != nil {
		t.Fatalf("failed to seed second batch: %v", err)
	}
	worker.runCleanup()

	remaining, _ = db.QueryEvents(storage.QueryParams{AppID: "app-1", Page: 1, PageSize: 100})
	if remaining.Total != 1 {
		t.Errorf("expected throttled run to skip deletion (1 event), got %d", remaining.Total)
	}
}

// newCleanupTestDB builds an in-memory DB whose *storage.DB satisfies the
// CleanupSystemStore interface (duck-typed).
func newCleanupTestDB(t *testing.T) *storage.DB {
	t.Helper()
	db, err := storage.NewDB(storage.Config{Path: ":memory:", RetentionDays: 30})
	if err != nil {
		t.Fatalf("failed to create db: %v", err)
	}
	return db
}
