package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/logmonitor/collector/sourcemap"
	"github.com/logmonitor/collector/storage"
)

// SourceMapHandler handles source map upload and retrieval
type SourceMapHandler struct {
	db             *storage.DB
	smStorage      *storage.SourceMapStorage
	allowedOrigins []string
}

// NewSourceMapHandler creates a new source map handler
func NewSourceMapHandler(db *storage.DB, smStorage *storage.SourceMapStorage) *SourceMapHandler {
	return &SourceMapHandler{
		db:        db,
		smStorage: smStorage,
	}
}

// SetAllowedOrigins sets the allowed CORS origins
func (h *SourceMapHandler) SetAllowedOrigins(origins []string) {
	h.allowedOrigins = origins
}

// RegisterRoutes registers all source map routes
func (h *SourceMapHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/sourcemaps/upload", h.Upload)
	mux.HandleFunc("GET /api/sourcemaps", h.List)
	mux.HandleFunc("GET /api/sourcemaps/download", h.Download)
	mux.HandleFunc("DELETE /api/sourcemaps/", h.Delete)
	mux.HandleFunc("POST /api/sourcemaps/deobfuscate", h.Deobfuscate)
}

// Upload handles source map file upload
func (h *SourceMapHandler) Upload(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Parse multipart form (max 32MB)
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	// Get metadata from form
	appID := r.FormValue("appId")
	release := r.FormValue("release")
	env := r.FormValue("env")
	buildID := r.FormValue("buildId")
	originalURL := r.FormValue("originalUrl")

	// Validate required fields
	if appID == "" || release == "" || buildID == "" {
		http.Error(w, "Missing required fields: appId, release, buildId", http.StatusBadRequest)
		return
	}

	// Set default env if not provided
	if env == "" {
		env = "production"
	}

	// Get uploaded file
	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "No file uploaded", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Validate file extension
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if ext != ".map" && ext != ".json" {
		http.Error(w, "Invalid file type. Only .map and .json files are allowed", http.StatusBadRequest)
		return
	}

	// Read file content
	content, err := io.ReadAll(file)
	if err != nil {
		log.Printf("Failed to read uploaded file: %v", err)
		http.Error(w, "Failed to read file", http.StatusInternalServerError)
		return
	}

	// Validate it's a valid source map (has "version" field)
	if !isValidSourceMap(content) {
		http.Error(w, "Invalid source map format", http.StatusBadRequest)
		return
	}

	// Save file to storage
	filePath, fileSize, err := h.smStorage.Save(appID, release, buildID, content)
	if err != nil {
		log.Printf("Failed to save source map: %v", err)
		http.Error(w, "Failed to save source map", http.StatusInternalServerError)
		return
	}

	// Create database record
	record := storage.SourceMapRecord{
		AppID:       appID,
		Release:     release,
		Env:         env,
		BuildID:     buildID,
		FilePath:    filePath,
		OriginalURL: originalURL,
		FileSize:    fileSize,
		UploadedAt:  time.Now().UnixMilli(),
	}

	id, err := h.db.CreateSourceMap(record)
	if err != nil {
		log.Printf("Failed to create source map record: %v", err)
		http.Error(w, "Failed to create source map record", http.StatusInternalServerError)
		return
	}

	log.Printf("Source map uploaded: app=%s release=%s env=%s build=%s size=%d",
		appID, release, env, buildID, fileSize)

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id":          id,
		"appId":       appID,
		"release":     release,
		"env":         env,
		"buildId":     buildID,
		"originalUrl": originalURL,
		"fileSize":    fileSize,
		"uploadedAt":  record.UploadedAt,
	})
}

// List lists all source maps for an app
func (h *SourceMapHandler) List(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	appID := r.URL.Query().Get("appId")
	if appID == "" {
		http.Error(w, "Missing appId parameter", http.StatusBadRequest)
		return
	}

	records, err := h.db.ListSourceMaps(appID, 100)
	if err != nil {
		log.Printf("Failed to list source maps: %v", err)
		http.Error(w, "Failed to list source maps", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"data":  records,
		"total": len(records),
	})
}

// Download downloads a source map file
func (h *SourceMapHandler) Download(w http.ResponseWriter, r *http.Request) {
	appID := r.URL.Query().Get("appId")
	release := r.URL.Query().Get("release")
	env := r.URL.Query().Get("env")
	buildID := r.URL.Query().Get("buildId")

	if appID == "" || buildID == "" {
		http.Error(w, "Missing required parameters: appId, buildId", http.StatusBadRequest)
		return
	}

	// Get source map record
	var record *storage.SourceMapRecord
	var err error

	if release != "" && env != "" {
		record, err = h.db.GetSourceMap(appID, release, env, buildID)
	} else {
		record, err = h.db.GetSourceMapByBuildID(buildID)
	}

	if err != nil {
		log.Printf("Failed to get source map: %v", err)
		http.Error(w, "Failed to get source map", http.StatusInternalServerError)
		return
	}

	if record == nil {
		http.Error(w, "Source map not found", http.StatusNotFound)
		return
	}

	// Read file content
	content, err := h.smStorage.GetByPath(record.FilePath)
	if err != nil {
		log.Printf("Failed to read source map file: %v", err)
		http.Error(w, "Failed to read source map file", http.StatusInternalServerError)
		return
	}

	// Set headers for download
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s-%s.map", record.Release, record.BuildID))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(content)))

	w.Write(content)
}

// Delete deletes a source map
func (h *SourceMapHandler) Delete(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Extract ID from path
	path := r.URL.Path
	// Remove /api/sourcemaps/ prefix
	idStr := strings.TrimPrefix(path, "/api/sourcemaps/")
	if idStr == path {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	var id int64
	if _, err := fmt.Sscanf(idStr, "%d", &id); err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	if err := h.db.DeleteSourceMap(id); err != nil {
		log.Printf("Failed to delete source map: %v", err)
		http.Error(w, "Failed to delete source map", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Source map deleted successfully",
		"id":      id,
	})
}

// Deobfuscate deobfuscates a stack trace using source maps
func (h *SourceMapHandler) Deobfuscate(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req struct {
		AppID   string `json:"appId"`
		Release string `json:"release"`
		Env     string `json:"env"`
		BuildID string `json:"buildId"`
		Stack   string `json:"stack"`
		Frames  []struct {
			Filename     string `json:"filename"`
			Line         int    `json:"line"`
			Column       int    `json:"column"`
			FunctionName string `json:"functionName,omitempty"`
		} `json:"frames"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.AppID == "" {
		http.Error(w, "Missing required field: appId", http.StatusBadRequest)
		return
	}

	// Get source map
	var record *storage.SourceMapRecord
	var err error

	if req.BuildID != "" {
		if req.Release != "" && req.Env != "" {
			record, err = h.db.GetSourceMap(req.AppID, req.Release, req.Env, req.BuildID)
		} else {
			record, err = h.db.GetSourceMapByBuildID(req.BuildID)
		}
	} else {
		// Try to find by app_id and release only (most recent)
		records, listErr := h.db.ListSourceMaps(req.AppID, 1)
		if listErr == nil && len(records) > 0 {
			record = &records[0]
		}
	}

	if err != nil {
		log.Printf("Failed to get source map: %v", err)
	}

	if record == nil {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"deobfuscated": false,
			"reason":       "Source map not found",
			"frames":       req.Frames,
		})
		return
	}

	// Read source map file
	smContent, err := h.smStorage.GetByPath(record.FilePath)
	if err != nil {
		log.Printf("Failed to read source map file: %v", err)
		http.Error(w, "Failed to read source map", http.StatusInternalServerError)
		return
	}

	// Parse source map
	parser, err := sourcemap.NewParser(smContent)
	if err != nil {
		log.Printf("Failed to parse source map: %v", err)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"deobfuscated": false,
			"reason":       "Failed to parse source map",
			"error":        err.Error(),
		})
		return
	}

	// Convert frames to StackFrame format
	frames := make([]sourcemap.StackFrame, len(req.Frames))
	for i, f := range req.Frames {
		frames[i] = sourcemap.StackFrame{
			Filename:     f.Filename,
			Line:         f.Line,
			Column:       f.Column,
			FunctionName: f.FunctionName,
		}
	}

	// Deobfuscate
	results := parser.DeobfuscateStackTrace(frames)

	// Convert results to response format
	response := map[string]interface{}{
		"deobfuscated": true,
		"buildId":     record.BuildID,
		"release":     record.Release,
		"env":         record.Env,
		"frames":      results,
	}

	json.NewEncoder(w).Encode(response)
}

// isValidSourceMap checks if the content is a valid source map
func isValidSourceMap(content []byte) bool {
	var data map[string]interface{}
	if err := json.Unmarshal(content, &data); err != nil {
		return false
	}

	// Check for version field (required for source maps)
	if _, ok := data["version"]; !ok {
		return false
	}

	// Check for mappings field
	if _, ok := data["mappings"]; !ok {
		return false
	}

	return true
}
