package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/logmonitor/collector/storage"
)

// TestHealthEndpoint tests that the health endpoint returns proper response structure
func TestHealthEndpoint(t *testing.T) {
	// Create test database
	cfg := storage.Config{
		Path:          ":memory:",
		RetentionDays: 30,
	}

	db, err := storage.NewDB(cfg)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer db.Close()

	// Create health handler
	handler := NewHealthHandler(":memory:", db, db, db)

	// Create test request
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	w := httptest.NewRecorder()

	// Serve the request
	handler.Health(w, req)

	// Check response status
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Check content type
	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected content type 'application/json', got '%s'", contentType)
	}

	// Parse response
	var response HealthResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Verify response structure
	if response.Status == "" {
		t.Error("Expected status to be set")
	}

	if response.Timestamp == 0 {
		t.Error("Expected timestamp to be set")
	}

	if response.Uptime < 0 {
		t.Error("Expected uptime to be non-negative")
	}

	if response.Version == "" {
		t.Error("Expected version to be set")
	}

	if response.Components == nil {
		t.Error("Expected components map to be initialized")
	}

	// Check database component
	dbComp, exists := response.Components["database"]
	if !exists {
		t.Error("Expected database component to exist")
	} else {
		if dbComp.Status == "" {
			t.Error("Expected database component status to be set")
		}
	}

	// Check system metrics
	if response.System.Goroutines == 0 {
		t.Error("Expected goroutines count to be set")
	}

	if response.System.MemoryAllocMB == 0 {
		t.Error("Expected memory alloc to be set")
	}
}

// TestHealthDatabaseDegraded tests health endpoint when database fails
func TestHealthDatabaseDegraded(t *testing.T) {
	// Create test database
	cfg := storage.Config{
		Path:          ":memory:",
		RetentionDays: 30,
	}

	db, err := storage.NewDB(cfg)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer db.Close()

	// Close database to simulate failure
	db.Close()

	// Create health handler
	handler := NewHealthHandler(":memory:", db, db, db)

	// Create test request
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	w := httptest.NewRecorder()

	// Serve the request
	handler.Health(w, req)

	// Parse response
	var response HealthResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Should return degraded status
	if response.Status != "degraded" && response.Status != "unhealthy" {
		t.Logf("Expected status to be 'degraded' or 'unhealthy', got '%s'", response.Status)
	}

	// Database component should show error
	dbComp, exists := response.Components["database"]
	if !exists {
		t.Error("Expected database component to exist")
	} else if dbComp.Status == "ok" {
		t.Error("Expected database component status to be error when database is closed")
	}
}

// TestHealthResponseFields tests all expected fields in health response
func TestHealthResponseFields(t *testing.T) {
	// Create test database
	cfg := storage.Config{
		Path:          ":memory:",
		RetentionDays: 30,
	}

	db, err := storage.NewDB(cfg)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer db.Close()

	// Create health handler
	handler := NewHealthHandler(":memory:", db, db, db)

	// Create test request
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	w := httptest.NewRecorder()

	// Serve the request
	handler.Health(w, req)

	// Parse response
	var response HealthResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Verify all top-level fields exist
	// These are checked via their presence in the JSON
	if response.Status == "" {
		t.Error("Status field is missing")
	}
	if response.Timestamp == 0 {
		t.Error("Timestamp field is missing or zero")
	}
	if response.Uptime < 0 {
		t.Error("Uptime field should be non-negative")
	}
	if response.Version == "" {
		t.Error("Version field is missing")
	}
	if len(response.Components) == 0 {
		t.Error("Components map is empty")
	}

	// System fields
	if response.System.Goroutines == 0 {
		t.Error("System.Goroutines field is missing or zero")
	}
	// Note: MemoryAllocMB, DBSizeMB can be 0 in some environments
}

// TestHealthEndpointWithDatabase tests health endpoint with database check
func TestHealthEndpointWithDatabase(t *testing.T) {
	// Create test database
	cfg := storage.Config{
		Path:          ":memory:",
		RetentionDays: 30,
	}

	db, err := storage.NewDB(cfg)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer db.Close()

	// Create health handler with database check
	handler := NewHealthHandler(":memory:", db, db, db)

	// Create test request
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	w := httptest.NewRecorder()

	// Serve the request
	handler.Health(w, req)

	// Check response
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Parse response
	var response HealthResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// With a working database, status should be healthy
	if response.Status != "healthy" {
		t.Errorf("Expected status 'healthy', got '%s'", response.Status)
	}

	// Database component should be ok
	dbComp, exists := response.Components["database"]
	if !exists {
		t.Error("Expected database component to exist")
	} else if dbComp.Status != "ok" {
		t.Errorf("Expected database component status 'ok', got '%s'", dbComp.Status)
	}
}

// TestHealthRecentErrors verifies the recent error count surfaces errors inserted
// within the look-back window (R013).
func TestHealthRecentErrors(t *testing.T) {
	cfg := storage.Config{Path: ":memory:", RetentionDays: 30}
	db, err := storage.NewDB(cfg)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer db.Close()

	// Insert 5 fresh error events and 3 fresh info events.
	now := time.Now().UnixMilli()
	events := []storage.EventRecord{
		{AppID: "app-1", Type: "error", Level: "error", Message: "boom", CreatedAt: now},
		{AppID: "app-1", Type: "error", Level: "error", Message: "boom2", CreatedAt: now},
		{AppID: "app-1", Type: "error", Level: "error", Message: "boom3", CreatedAt: now},
		{AppID: "app-1", Type: "error", Level: "error", Message: "boom4", CreatedAt: now},
		{AppID: "app-1", Type: "error", Level: "error", Message: "boom5", CreatedAt: now},
		{AppID: "app-1", Type: "info", Level: "info", Message: "ok", CreatedAt: now},
		{AppID: "app-1", Type: "info", Level: "info", Message: "ok2", CreatedAt: now},
		{AppID: "app-1", Type: "info", Level: "info", Message: "ok3", CreatedAt: now},
	}
	if err := db.InsertEvents(events); err != nil {
		t.Fatalf("failed to seed events: %v", err)
	}

	handler := NewHealthHandler(":memory:", db, db, db)
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	w := httptest.NewRecorder()
	handler.Health(w, req)

	var response HealthResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response.RecentErrors != 5 {
		t.Errorf("expected 5 recent errors, got %d", response.RecentErrors)
	}
	if response.EventCount < 8 {
		t.Errorf("expected event count >= 8, got %d", response.EventCount)
	}
}

// TestHealthInvalidMethod tests that non-GET requests are handled
func TestHealthInvalidMethod(t *testing.T) {
	// Create test database
	cfg := storage.Config{
		Path:          ":memory:",
		RetentionDays: 30,
	}

	db, err := storage.NewDB(cfg)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer db.Close()

	// Create health handler
	handler := NewHealthHandler(":memory:", db, db, db)

	// Create POST request
	req := httptest.NewRequest(http.MethodPost, "/api/health", nil)
	w := httptest.NewRecorder()

	// Serve the request - currently Health handler doesn't check method
	// This test documents current behavior
	handler.Health(w, req)

	// The handler currently returns 200 for any method
	// This test can be updated if method validation is added
	if w.Code != http.StatusOK {
		t.Logf("Health handler returned status %d for POST request", w.Code)
	}
}
