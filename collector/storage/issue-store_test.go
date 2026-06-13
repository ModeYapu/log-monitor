package storage

import (
	"testing"
	"time"
)

// TestCreateAndGetIssue tests creating and retrieving an issue
func TestCreateAndGetIssue(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Insert an event with fingerprint to create an issue
	event := EventRecord{
		AppID:       "test-app",
		Type:        "error",
		Level:       "error",
		Message:     "Test error message for issue",
		URL:         "https://example.com",
		UserID:      "user123",
		SessionID:   "session456",
		CreatedAt:   time.Now().UnixMilli(),
		Tags:        "{}",
		Extra:       "{}",
		Performance: "{}",
		Fingerprint: "test-issue-fingerprint",
	}

	events := []EventRecord{event}
	err := db.InsertEvents(events)
	if err != nil {
		t.Fatalf("Failed to insert event: %v", err)
	}

	// Get the issue
	filter := IssueFilter{
		AppID:    "test-app",
		Status:   "open",
		Page:     1,
		PageSize: 10,
	}

	issues, total, err := db.GetIssues(filter)
	if err != nil {
		t.Fatalf("Failed to get issues: %v", err)
	}

	if total != 1 {
		t.Errorf("Expected 1 issue, got %d", total)
	}

	if len(issues) != 1 {
		t.Fatalf("Expected 1 issue in results, got %d", len(issues))
	}

	issue := issues[0]
	if issue.Status != "open" {
		t.Errorf("Expected status 'open', got '%s'", issue.Status)
	}

	if issue.Fingerprint != "test-issue-fingerprint" {
		t.Errorf("Expected fingerprint 'test-issue-fingerprint', got '%s'", issue.Fingerprint)
	}
}

// TestUpdateIssueStatus tests updating issue status (resolve/ignore/reopen)
func TestUpdateIssueStatus(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Create an issue
	event := EventRecord{
		AppID:       "test-app",
		Type:        "error",
		Level:       "error",
		Message:     "Test error for issue update",
		CreatedAt:   time.Now().UnixMilli(),
		Tags:        "{}",
		Extra:       "{}",
		Performance: "{}",
		Fingerprint: "update-test-fingerprint",
	}

	events := []EventRecord{event}
	err := db.InsertEvents(events)
	if err != nil {
		t.Fatalf("Failed to insert event: %v", err)
	}

	// Get the issue
	filter := IssueFilter{
		AppID:    "test-app",
		Status:   "open",
		Page:     1,
		PageSize: 10,
	}

	issues, _, err := db.GetIssues(filter)
	if err != nil {
		t.Fatalf("Failed to get issues: %v", err)
	}

	if len(issues) != 1 {
		t.Fatalf("Expected 1 issue, got %d", len(issues))
	}

	issue := issues[0]

	// Test resolving the issue
	updates := map[string]interface{}{
		"status": "resolved",
	}

	err = db.UpdateIssue(issue.ID, updates)
	if err != nil {
		t.Fatalf("Failed to update issue status: %v", err)
	}

	// Verify the update
	updatedIssue, err := db.GetIssue(issue.ID)
	if err != nil {
		t.Fatalf("Failed to get updated issue: %v", err)
	}

	if updatedIssue.Status != "resolved" {
		t.Errorf("Expected status 'resolved', got '%s'", updatedIssue.Status)
	}

	if updatedIssue.ResolvedAt == 0 {
		t.Error("Expected resolved_at to be set")
	}
}

// TestListIssues tests listing issues with filters
func TestListIssues(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Create multiple issues
	events := []EventRecord{
		{
			AppID:       "test-app",
			Type:        "error",
			Level:       "error",
			Message:     "Critical error",
			CreatedAt:   time.Now().UnixMilli(),
			Tags:        "{}",
			Extra:       "{}",
			Performance: "{}",
			Fingerprint: "critical-fingerprint",
		},
		{
			AppID:       "test-app",
			Type:        "error",
			Level:       "error",
			Message:     "Warning error",
			CreatedAt:   time.Now().UnixMilli(),
			Tags:        "{}",
			Extra:       "{}",
			Performance: "{}",
			Fingerprint: "warning-fingerprint",
		},
		{
			AppID:       "test-app",
			Type:        "error",
			Level:       "error",
			Message:     "Info error",
			CreatedAt:   time.Now().UnixMilli(),
			Tags:        "{}",
			Extra:       "{}",
			Performance: "{}",
			Fingerprint: "info-fingerprint",
		},
	}

	err := db.InsertEvents(events)
	if err != nil {
		t.Fatalf("Failed to insert events: %v", err)
	}

	// Test getting all issues
	filter := IssueFilter{
		AppID:    "test-app",
		Page:     1,
		PageSize: 10,
	}

	issues, total, err := db.GetIssues(filter)
	if err != nil {
		t.Fatalf("Failed to get issues: %v", err)
	}

	if total != 3 {
		t.Errorf("Expected 3 issues, got %d", total)
	}

	if len(issues) != 3 {
		t.Errorf("Expected 3 issues in results, got %d", len(issues))
	}

	// Test filtering by status (should only be open)
	filter.Status = "open"
	issues, total, err = db.GetIssues(filter)
	if err != nil {
		t.Fatalf("Failed to get filtered issues: %v", err)
	}

	if total != 3 {
		t.Errorf("Expected 3 open issues, got %d", total)
	}
}
