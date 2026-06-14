package storage

import (
	"testing"
)

// TestPerformanceIndexesExist verifies that the R013 composite indexes were
// created. This is the deterministic check (independent of planner heuristics).
func TestPerformanceIndexesExist(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()

	want := []string{
		"idx_events_app_project_level_created",
		"idx_events_project_created",
		"idx_events_app_level_message",
		"idx_events_app_url",
		"idx_events_created_at",
	}

	rows, err := db.conn.Query("SELECT name FROM sqlite_master WHERE type='index' AND tbl_name='events'")
	if err != nil {
		t.Fatalf("failed to query indexes: %v", err)
	}
	defer rows.Close()

	got := map[string]bool{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan: %v", err)
		}
		got[name] = true
	}

	for _, idx := range want {
		if !got[idx] {
			t.Errorf("expected index %q to exist on events table", idx)
		}
	}
}

// TestExplainQueryPlan verifies the generic EXPLAIN QUERY PLAN helper runs.
func TestExplainQueryPlan(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()

	plan, err := db.ExplainQueryPlan("SELECT COUNT(*) FROM events WHERE app_id = ?", "app-1")
	if err != nil {
		t.Fatalf("ExplainQueryPlan failed: %v", err)
	}
	if plan == "" {
		t.Fatal("expected non-empty plan")
	}
}

// TestAnalyzeTopNQueryUsesIndex inserts enough rows for the planner to prefer an
// index, runs ANALYZE, then asserts the GetTopN "errors" plan uses an index
// rather than a full table scan.
func TestAnalyzeTopNQueryUsesIndex(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()

	seedEvents(t, db, 1500)

	plan, err := db.AnalyzeTopNQuery("app-1", "errors", "count", 20, AnalyticsFilters{})
	if err != nil {
		t.Fatalf("AnalyzeTopNQuery failed: %v", err)
	}
	t.Logf("GetTopN(errors) plan:\n%s", plan)

	if !PlanUsesIndex(plan) {
		t.Errorf("expected GetTopN errors query to use an index, got plan:\n%s", plan)
	}
}

// TestAnalyzeErrorClustersQueryUsesIndex verifies GetErrorClusters' aggregation
// query is served by an index after seeding data and ANALYZE.
func TestAnalyzeErrorClustersQueryUsesIndex(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()

	seedEvents(t, db, 1500)

	plan, err := db.AnalyzeErrorClustersQuery("app-1", 0)
	if err != nil {
		t.Fatalf("AnalyzeErrorClustersQuery failed: %v", err)
	}
	t.Logf("GetErrorClusters plan:\n%s", plan)

	if !PlanUsesIndex(plan) {
		t.Errorf("expected GetErrorClusters query to use an index, got plan:\n%s", plan)
	}
}

// TestPlanUsesIndex unit-tests the helper.
func TestPlanUsesIndex(t *testing.T) {
	tests := []struct {
		plan string
		want bool
	}{
		{"SEARCH TABLE events USING INDEX idx_events_app_level_message", true},
		{"SEARCH TABLE events USING COVERING INDEX idx_x", true},
		{"SCAN events", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := PlanUsesIndex(tt.plan); got != tt.want {
			t.Errorf("PlanUsesIndex(%q) = %v, want %v", tt.plan, got, tt.want)
		}
	}
}

// newTestDB creates an in-memory database for testing.
func newTestDB(t *testing.T) *DB {
	t.Helper()
	db, err := NewDB(Config{Path: ":memory:", RetentionDays: 30})
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	return db
}

// seedEvents inserts count events across a few apps/projects/levels/messages so
// the SQLite planner has enough rows + statistics to prefer an index.
func seedEvents(t *testing.T, db *DB, count int) {
	t.Helper()

	apps := []string{"app-1", "app-2"}
	messages := []string{"TypeError: cannot read props of undefined", "NetworkError: timeout", "RangeError: max stack"}
	levels := []string{"error", "info", "warn"}

	events := make([]EventRecord, 0, count)
	for i := 0; i < count; i++ {
		events = append(events, EventRecord{
			AppID:     apps[i%len(apps)],
			ProjectID: int64(i%2 + 1),
			Type:      "error",
			Level:     levels[i%len(levels)],
			Message:   messages[i%len(messages)],
			URL:       "https://example.com/page" + string(rune('0'+i%5)),
			UserID:    "user-" + string(rune('0'+i%10)),
			Env:       "production",
			Release:   "1.0.0",
			CreatedAt: int64(1700000000000 + i),
		})
	}

	if err := db.InsertEvents(events); err != nil {
		t.Fatalf("failed to seed events: %v", err)
	}

	// Refresh planner statistics so index choices reflect the seeded data.
	if _, err := db.conn.Exec("ANALYZE"); err != nil {
		t.Fatalf("ANALYZE failed: %v", err)
	}
}
