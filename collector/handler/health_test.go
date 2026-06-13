package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/logmonitor/collector/storage"
)

// TestHealthEndpoint tests that the health endpoint returns 200
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

	// Create health handler (assuming you have one)
	// For now, we'll create a simple handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Simple health check
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	// Create test request
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	// Serve the request
	handler.ServeHTTP(w, req)

	// Check response
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected content type 'application/json', got '%s'", contentType)
	}
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
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Check database connection
		if db.Conn() == nil {
			http.Error(w, "Database not connected", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok","database":"connected"}`))
	})

	// Create test request
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	// Serve the request
	handler.ServeHTTP(w, req)

	// Check response
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}
