package storage

import (
	"testing"
	"time"
)

// setupTestDB creates a test database and returns it, with cleanup function
func setupTestDB(t *testing.T) *DB {
	t.Helper()

	cfg := Config{
		Path:          ":memory:",
		RetentionDays: 30,
	}

	db, err := NewDB(cfg)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	return db
}

// TestInsertAndGetEvent tests inserting and retrieving an event
func TestInsertAndGetEvent(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	event := EventRecord{
		AppID:      "test-app",
		Type:       "error",
		Level:      "error",
		Message:    "Test error message",
		URL:        "https://example.com",
		UserID:     "user123",
		SessionID:  "session456",
		CreatedAt:  time.Now().UnixMilli(),
		Tags:       "{}",
		Extra:      "{}",
		Performance: "{}",
		Fingerprint: "test-fingerprint",
	}

	events := []EventRecord{event}
	err := db.InsertEvents(events)
	if err != nil {
		t.Fatalf("Failed to insert event: %v", err)
	}

	// Query the event back
	query := QueryParams{
		AppID:    "test-app",
		Page:     1,
		PageSize: 10,
	}

	result, err := db.QueryEvents(query)
	if err != nil {
		t.Fatalf("Failed to query events: %v", err)
	}

	if result.Total != 1 {
		t.Errorf("Expected 1 event, got %d", result.Total)
	}

	if len(result.Data) != 1 {
		t.Fatalf("Expected 1 event in data, got %d", len(result.Data))
	}

	retrieved := result.Data[0]
	if retrieved.Message != "Test error message" {
		t.Errorf("Expected message 'Test error message', got '%s'", retrieved.Message)
	}
}

// TestQueryEvents tests querying events with filters
func TestQueryEvents(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Insert multiple events
	events := []EventRecord{
		{
			AppID:     "test-app",
			Type:      "error",
			Level:     "error",
			Message:   "Error 1",
			CreatedAt: time.Now().UnixMilli(),
			Tags:      "{}",
			Extra:     "{}",
			Performance: "{}",
		},
		{
			AppID:     "test-app",
			Type:      "info",
			Level:     "info",
			Message:   "Info 1",
			CreatedAt: time.Now().UnixMilli(),
			Tags:      "{}",
			Extra:     "{}",
			Performance: "{}",
		},
		{
			AppID:     "test-app",
			Type:      "error",
			Level:     "error",
			Message:   "Error 2",
			CreatedAt: time.Now().UnixMilli(),
			Tags:      "{}",
			Extra:     "{}",
			Performance: "{}",
		},
	}

	err := db.InsertEvents(events)
	if err != nil {
		t.Fatalf("Failed to insert events: %v", err)
	}

	// Test filtering by level
	query := QueryParams{
		AppID:    "test-app",
		Level:    "error",
		Page:     1,
		PageSize: 10,
	}

	result, err := db.QueryEvents(query)
	if err != nil {
		t.Fatalf("Failed to query events by level: %v", err)
	}

	if result.Total != 2 {
		t.Errorf("Expected 2 error events, got %d", result.Total)
	}

	// Test filtering by type
	query.Level = ""
	query.Type = "info"

	result, err = db.QueryEvents(query)
	if err != nil {
		t.Fatalf("Failed to query events by type: %v", err)
	}

	if result.Total != 1 {
		t.Errorf("Expected 1 info event, got %d", result.Total)
	}
}

// TestGetEventStats tests getting statistics for an app
func TestGetEventStats(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Insert test events
	events := []EventRecord{
		{
			AppID:     "stats-app",
			Type:      "error",
			Level:     "error",
			Message:   "Error 1",
			CreatedAt: time.Now().UnixMilli(),
			Tags:      "{}",
			Extra:     "{}",
			Performance: "{}",
		},
		{
			AppID:     "stats-app",
			Type:      "error",
			Level:     "error",
			Message:   "Error 2",
			CreatedAt: time.Now().UnixMilli(),
			Tags:      "{}",
			Extra:     "{}",
			Performance: "{}",
		},
		{
			AppID:     "stats-app",
			Type:      "info",
			Level:     "info",
			Message:   "Info 1",
			CreatedAt: time.Now().UnixMilli(),
			Tags:      "{}",
			Extra:     "{}",
			Performance: "{}",
		},
	}

	err := db.InsertEvents(events)
	if err != nil {
		t.Fatalf("Failed to insert events: %v", err)
	}

	// Get stats
	stats, err := db.GetStats("stats-app")
	if err != nil {
		t.Fatalf("Failed to get stats: %v", err)
	}

	if stats.ErrorCount != 2 {
		t.Errorf("Expected 2 errors, got %d", stats.ErrorCount)
	}

	if stats.InfoCount != 1 {
		t.Errorf("Expected 1 info, got %d", stats.InfoCount)
	}

	if stats.TotalEvents != 3 {
		t.Errorf("Expected 3 total events, got %d", stats.TotalEvents)
	}
}