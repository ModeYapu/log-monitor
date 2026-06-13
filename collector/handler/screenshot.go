package handler

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
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
		slog.Error("Failed to create screenshot directory", "error", err)
	}
	return &ScreenshotHandler{
		screenshotDir: screenshotDir,
	}
}

// ServeHTTP handles HTTP requests
func (h *ScreenshotHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
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
		slog.Error("Failed to parse screenshot request", "error", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Validate request
	if req.AppID == "" {
		http.Error(w, "Missing appId", http.StatusBadRequest)
		return
	}
	if !safePathSegment(req.AppID) {
		http.Error(w, "Invalid appId format", http.StatusBadRequest)
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
		slog.Error("Failed to decode base64 image", "error", err)
		http.Error(w, "Invalid base64 image data", http.StatusBadRequest)
		return
	}

	// Create app-specific directory
	appDir, err := safeJoinUnderBase(h.screenshotDir, req.AppID)
	if err != nil {
		http.Error(w, "Invalid appId format", http.StatusBadRequest)
		return
	}
	if err := os.MkdirAll(appDir, 0755); err != nil {
		slog.Error("Failed to create app directory", "error", err)
		http.Error(w, "Failed to create directory", http.StatusInternalServerError)
		return
	}

	// Save screenshot to file
	filename, err := safeJoinUnderBase(appDir, req.EventID+".png")
	if err != nil {
		http.Error(w, "Invalid eventId format", http.StatusBadRequest)
		return
	}
	if err := os.WriteFile(filename, imageData, 0644); err != nil {
		slog.Error("Failed to save screenshot", "error", err)
		http.Error(w, "Failed to save screenshot", http.StatusInternalServerError)
		return
	}

	slog.Info("Screenshot saved", "filename", filename)

	// Respond with success
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"path":    "/api/screenshots/" + req.AppID + "/" + req.EventID + ".png",
	})
}

// ScreenshotFileHandler serves stored screenshots to authenticated users.
type ScreenshotFileHandler struct {
	screenshotDir string
}

func NewScreenshotFileHandler(screenshotDir string) *ScreenshotFileHandler {
	return &ScreenshotFileHandler{screenshotDir: screenshotDir}
}

func (h *ScreenshotFileHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/screenshots/")
	parts := strings.Split(path, "/")
	if len(parts) != 2 {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	appID := parts[0]
	filename := parts[1]
	if !safePathSegment(appID) || !safeScreenshotFilename(filename) {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	fullPath, err := safeJoinUnderBase(h.screenshotDir, appID, filename)
	if err != nil {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	info, err := os.Stat(fullPath)
	if err != nil || info.IsDir() {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	contentType := mime.TypeByExtension(filepath.Ext(fullPath))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	w.Header().Set("Content-Type", contentType)
	http.ServeFile(w, r, fullPath)
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

func safePathSegment(id string) bool {
	if id == "" || len(id) > 100 {
		return false
	}
	matched, _ := regexp.MatchString(`^[a-zA-Z0-9\-_.]+$`, id)
	return matched
}

func safeScreenshotFilename(name string) bool {
	return safeEventID(strings.TrimSuffix(name, filepath.Ext(name))) && strings.EqualFold(filepath.Ext(name), ".png")
}

func safeJoinUnderBase(base string, parts ...string) (string, error) {
	cleanBase := filepath.Clean(base)
	allParts := append([]string{cleanBase}, parts...)
	joined := filepath.Join(allParts...)
	rel, err := filepath.Rel(cleanBase, joined)
	if err != nil {
		return "", err
	}
	if rel == "." {
		return joined, nil
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path escapes base directory")
	}
	return joined, nil
}
