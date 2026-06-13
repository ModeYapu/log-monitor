package storage

import (
	"testing"
	"time"

	"github.com/logmonitor/collector/model"
)

func TestInsertAuditLog(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Ensure audit_logs table exists
	if err := db.EnsureAuditLogsTable(); err != nil {
		t.Fatalf("Failed to ensure audit_logs table: %v", err)
	}

	// Create a test audit log
	log := &model.AuditLog{
		ProjectID:  1,
		UserID:     100,
		Username:   "testuser",
		Action:     model.AuditActionCreate,
		Resource:   model.AuditResourceProject,
		ResourceID: "proj-123",
		Detail:     "Created new project",
		IP:         "127.0.0.1",
		UserAgent:  "TestAgent/1.0",
		CreatedAt:  time.Now().UnixMilli(),
	}

	// Insert the audit log
	if err := db.InsertAuditLog(log); err != nil {
		t.Fatalf("Failed to insert audit log: %v", err)
	}

	// Verify ID was set
	if log.ID == 0 {
		t.Error("Expected audit log ID to be set")
	}

	// Verify CreatedAt was set if it was 0
	log2 := &model.AuditLog{
		ProjectID: 1,
		UserID:    100,
		Username:  "testuser",
		Action:    model.AuditActionDelete,
		Resource:  model.AuditResourceUser,
		ResourceID: "user-456",
	}

	if err := db.InsertAuditLog(log2); err != nil {
		t.Fatalf("Failed to insert second audit log: %v", err)
	}

	if log2.CreatedAt == 0 {
		t.Error("Expected CreatedAt to be auto-set")
	}
}

func TestQueryAuditLogs(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Ensure audit_logs table exists
	if err := db.EnsureAuditLogsTable(); err != nil {
		t.Fatalf("Failed to ensure audit_logs table: %v", err)
	}

	// Insert test data
	now := time.Now().UnixMilli()
	testLogs := []*model.AuditLog{
		{
			ProjectID:  1,
			UserID:     100,
			Username:   "admin",
			Action:     model.AuditActionCreate,
			Resource:   model.AuditResourceProject,
			ResourceID: "1",
			Detail:     "Created project",
			IP:         "10.0.0.1",
			UserAgent:  "Mozilla/5.0",
			CreatedAt:  now - 3600000, // 1 hour ago
		},
		{
			ProjectID:  1,
			UserID:     101,
			Username:   "user1",
			Action:     model.AuditActionUpdate,
			Resource:   model.AuditResourceAlert,
			ResourceID: "alert-1",
			Detail:     "Updated alert configuration",
			IP:         "10.0.0.2",
			UserAgent:  "Chrome/90.0",
			CreatedAt:  now - 1800000, // 30 minutes ago
		},
		{
			ProjectID:  2,
			UserID:     100,
			Username:   "admin",
			Action:     model.AuditActionDelete,
			Resource:   model.AuditResourceUser,
			ResourceID: "user-2",
			Detail:     "Deleted user",
			IP:         "10.0.0.1",
			UserAgent:  "Mozilla/5.0",
			CreatedAt:  now - 900000, // 15 minutes ago
		},
		{
			ProjectID:  1,
			UserID:     100,
			Username:   "admin",
			Action:     model.AuditActionLogin,
			Resource:   "auth",
			ResourceID: "",
			Detail:     "User logged in",
			IP:         "10.0.0.3",
			UserAgent:  "Safari/13.0",
			CreatedAt:  now,
		},
	}

	for _, log := range testLogs {
		if err := db.InsertAuditLog(log); err != nil {
			t.Fatalf("Failed to insert test audit log: %v", err)
		}
	}

	t.Run("Query all logs", func(t *testing.T) {
		filter := model.AuditFilter{
			Page:     1,
			PageSize: 10,
		}

		logs, total, err := db.QueryAuditLogs(filter)
		if err != nil {
			t.Fatalf("Failed to query audit logs: %v", err)
		}

		if total != 4 {
			t.Errorf("Expected total count 4, got %d", total)
		}

		if len(logs) != 4 {
			t.Errorf("Expected 4 logs, got %d", len(logs))
		}

		// Verify logs are ordered by created_at DESC (most recent first)
		if logs[0].Action != model.AuditActionLogin {
			t.Errorf("Expected first log to be login, got %s", logs[0].Action)
		}
	})

	t.Run("Filter by project_id", func(t *testing.T) {
		filter := model.AuditFilter{
			ProjectID: 1,
			Page:      1,
			PageSize:  10,
		}

		logs, total, err := db.QueryAuditLogs(filter)
		if err != nil {
			t.Fatalf("Failed to query audit logs: %v", err)
		}

		if total != 3 {
			t.Errorf("Expected total count 3 for project 1, got %d", total)
		}

		for _, log := range logs {
			if log.ProjectID != 1 {
				t.Errorf("Expected all logs to have project_id 1, got %d", log.ProjectID)
			}
		}
	})

	t.Run("Filter by user_id", func(t *testing.T) {
		filter := model.AuditFilter{
			UserID:   100,
			Page:     1,
			PageSize: 10,
		}

		logs, total, err := db.QueryAuditLogs(filter)
		if err != nil {
			t.Fatalf("Failed to query audit logs: %v", err)
		}

		if total != 3 {
			t.Errorf("Expected total count 3 for user 100, got %d", total)
		}

		for _, log := range logs {
			if log.UserID != 100 {
				t.Errorf("Expected all logs to have user_id 100, got %d", log.UserID)
			}
		}
	})

	t.Run("Filter by action", func(t *testing.T) {
		filter := model.AuditFilter{
			Action:   model.AuditActionCreate,
			Page:     1,
			PageSize: 10,
		}

		logs, total, err := db.QueryAuditLogs(filter)
		if err != nil {
			t.Fatalf("Failed to query audit logs: %v", err)
		}

		if total != 1 {
			t.Errorf("Expected total count 1 for create action, got %d", total)
		}

		if len(logs) != 1 || logs[0].Action != model.AuditActionCreate {
			t.Error("Expected single create action log")
		}
	})

	t.Run("Filter by resource", func(t *testing.T) {
		filter := model.AuditFilter{
			Resource: model.AuditResourceProject,
			Page:     1,
			PageSize: 10,
		}

		_, total, err := db.QueryAuditLogs(filter)
		if err != nil {
			t.Fatalf("Failed to query audit logs: %v", err)
		}

		if total != 1 {
			t.Errorf("Expected total count 1 for project resource, got %d", total)
		}
	})

	t.Run("Filter by date range", func(t *testing.T) {
		// Query logs from the last 30 minutes
		startDate := now - 1800000 // 30 minutes ago
		filter := model.AuditFilter{
			StartDate: startDate,
			Page:      1,
			PageSize:  10,
		}

		logs, total, err := db.QueryAuditLogs(filter)
		if err != nil {
			t.Fatalf("Failed to query audit logs: %v", err)
		}

		if total != 3 {
			t.Errorf("Expected total count 3 for date range, got %d", total)
		}

		for _, log := range logs {
			if log.CreatedAt < startDate {
				t.Errorf("Expected all logs to be after start date, got %d", log.CreatedAt)
			}
		}
	})

	t.Run("Combined filters", func(t *testing.T) {
		filter := model.AuditFilter{
			ProjectID: 1,
			UserID:    100,
			Page:      1,
			PageSize:  10,
		}

		logs, total, err := db.QueryAuditLogs(filter)
		if err != nil {
			t.Fatalf("Failed to query audit logs: %v", err)
		}

		if total != 2 {
			t.Errorf("Expected total count 2 for combined filters, got %d", total)
		}

		for _, log := range logs {
			if log.ProjectID != 1 || log.UserID != 100 {
				t.Error("Expected all logs to match both filters")
			}
		}
	})
}

func TestQueryAuditLogsPagination(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Ensure audit_logs table exists
	if err := db.EnsureAuditLogsTable(); err != nil {
		t.Fatalf("Failed to ensure audit_logs table: %v", err)
	}

	// Insert 25 test logs
	now := time.Now().UnixMilli()
	for i := 0; i < 25; i++ {
		log := &model.AuditLog{
			ProjectID:  1,
			UserID:     100,
			Username:   "testuser",
			Action:     model.AuditActionView,
			Resource:   model.AuditResourceEvent,
			ResourceID: string(rune(i)),
			IP:         "127.0.0.1",
			CreatedAt:  now - int64(i)*1000,
		}
		if err := db.InsertAuditLog(log); err != nil {
			t.Fatalf("Failed to insert test log: %v", err)
		}
	}

	t.Run("First page", func(t *testing.T) {
		filter := model.AuditFilter{
			Page:     1,
			PageSize: 10,
		}

		logs, total, err := db.QueryAuditLogs(filter)
		if err != nil {
			t.Fatalf("Failed to query audit logs: %v", err)
		}

		if total != 25 {
			t.Errorf("Expected total count 25, got %d", total)
		}

		if len(logs) != 10 {
			t.Errorf("Expected 10 logs on first page, got %d", len(logs))
		}
	})

	t.Run("Second page", func(t *testing.T) {
		filter := model.AuditFilter{
			Page:     2,
			PageSize: 10,
		}

		logs, total, err := db.QueryAuditLogs(filter)
		if err != nil {
			t.Fatalf("Failed to query audit logs: %v", err)
		}

		if total != 25 {
			t.Errorf("Expected total count 25, got %d", total)
		}

		if len(logs) != 10 {
			t.Errorf("Expected 10 logs on second page, got %d", len(logs))
		}
	})

	t.Run("Third page (partial)", func(t *testing.T) {
		filter := model.AuditFilter{
			Page:     3,
			PageSize: 10,
		}

		logs, total, err := db.QueryAuditLogs(filter)
		if err != nil {
			t.Fatalf("Failed to query audit logs: %v", err)
		}

		if total != 25 {
			t.Errorf("Expected total count 25, got %d", total)
		}

		if len(logs) != 5 {
			t.Errorf("Expected 5 logs on third page, got %d", len(logs))
		}
	})

	t.Run("Page size limit", func(t *testing.T) {
		filter := model.AuditFilter{
			Page:     1,
			PageSize: 200, // Should be capped at 100
		}

		logs, total, err := db.QueryAuditLogs(filter)
		if err != nil {
			t.Fatalf("Failed to query audit logs: %v", err)
		}

		if len(logs) > 100 {
			t.Errorf("Expected page size to be capped at 100, got %d", len(logs))
		}

		if total != 25 {
			t.Errorf("Expected total count 25, got %d", total)
		}
	})

	t.Run("Invalid page defaults to 1", func(t *testing.T) {
		filter := model.AuditFilter{
			Page:     0,
			PageSize: 10,
		}

		logs, total, err := db.QueryAuditLogs(filter)
		if err != nil {
			t.Fatalf("Failed to query audit logs: %v", err)
		}

		if len(logs) != 10 {
			t.Errorf("Expected 10 logs, got %d", len(logs))
		}

		if total != 25 {
			t.Errorf("Expected total count 25, got %d", total)
		}
	})
}

func TestQueryAuditLogsWithClosedDB(t *testing.T) {
	db := setupTestDB(t)

	// Ensure audit_logs table exists
	if err := db.EnsureAuditLogsTable(); err != nil {
		t.Fatalf("Failed to ensure audit_logs table: %v", err)
	}

	// Close the database
	if err := db.Close(); err != nil {
		t.Fatalf("Failed to close database: %v", err)
	}

	// Try to insert audit log
	log := &model.AuditLog{
		ProjectID: 1,
		UserID:    100,
		Username:  "testuser",
		Action:    model.AuditActionView,
		Resource:  model.AuditResourceEvent,
	}

	if err := db.InsertAuditLog(log); err == nil {
		t.Error("Expected error when inserting to closed database")
	}

	// Try to query audit logs
	filter := model.AuditFilter{Page: 1, PageSize: 10}
	_, _, err := db.QueryAuditLogs(filter)
	if err == nil {
		t.Error("Expected error when querying closed database")
	}
}
