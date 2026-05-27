package handler

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
)

// ScreenshotHandler handles screenshot upload requests
type ScreenshotHandler struct {
	screenshotDir string
}

// ScreenshotRequest is the request body for screenshot upload
type ScreenshotRequest struct {
	AppID   string `json:"appId"`
	EventID string `json:"eventId"`
	Image   string `json:"image"` // base64 encoded
}

// NewScreenshotHandler creates a new screenshot handler
func NewScreenshotHandler(screenshotDir string) *ScreenshotHandler {
	// Create directory if it doesn't exist
	if err := os.MkdirAll(screenshotDir, 0755); err != nil {
		log.Printf("Failed to create screenshot directory: %v", err)
	}
	return &ScreenshotHandler{
		screenshotDir: screenshotDir,
	}
}

// ServeHTTP handles HTTP requests
func (h *ScreenshotHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Set CORS headers
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Content-Type", "application/json")

	// Handle preflight
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

		// Limit request body size to prevent DoS attacks
		const maxRequestSize = 10 * 1024 * 1024 // 10MB
		r.Body = http.MaxBytesReader(w, r.Body, maxRequestSize)

	// Read request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if len(body) == 0 {
		http.Error(w, "Empty request body", http.StatusBadRequest)
		return
	}

	// Parse request
	var req ScreenshotRequest
	if err := json.Unmarshal(body, &req); err != nil {
		log.Printf("Failed to parse screenshot request: %v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Validate request
	if req.AppID == "" {
		http.Error(w, "Missing appId", http.StatusBadRequest)
		return
	}
	if req.EventID == "" {
		http.Error(w, "Missing eventId", http.StatusBadRequest)
		return
	}
	// Validate eventId to prevent path traversal attacks
	if !safeEventID(req.EventID) {
		http.Error(w, "Invalid eventId format", http.StatusBadRequest)
		return
	}
	if req.Image == "" {
		http.Error(w, "Missing image data", http.StatusBadRequest)
		return
	}

	// Decode base64
	imageData, err := base64.StdEncoding.DecodeString(req.Image)
	if err != nil {
		log.Printf("Failed to decode base64 image: %v", err)
		http.Error(w, "Invalid base64 image data", http.StatusBadRequest)
		return
	}

	// Create app-specific directory
	appDir := filepath.Join(h.screenshotDir, req.AppID)
	if err := os.MkdirAll(appDir, 0755); err != nil {
		log.Printf("Failed to create app directory: %v", err)
		http.Error(w, "Failed to create directory", http.StatusInternalServerError)
		return
	}

	// Save screenshot to file
	filename := filepath.Join(appDir, req.EventID+".png")
	if err := os.WriteFile(filename, imageData, 0644); err != nil {
		log.Printf("Failed to save screenshot: %v", err)
		http.Error(w, "Failed to save screenshot", http.StatusInternalServerError)
		return
	}

	log.Printf("Screenshot saved: %s", filename)

	// Respond with success
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"path":    filename,
	})
}

// safeEventID validates that eventId contains only safe characters
func safeEventID(id string) bool {
	if id == "" || len(id) > 100 {
		return false
	}
	// Only allow alphanumeric, hyphen, underscore, and dot
	matched, _ := regexp.MatchString(`^[a-zA-Z0-9\-_.]+$`, id)
	return matched
}
