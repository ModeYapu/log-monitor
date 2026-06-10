package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/logmonitor/collector/storage"
)

// ClustersHandler handles error cluster queries
type ClustersHandler struct {
	db storage.EventStore
}

// NewClustersHandler creates a new clusters handler
func NewClustersHandler(db storage.EventStore) *ClustersHandler {
	return &ClustersHandler{
		db: db,
	}
}

// GetClusters retrieves error clusters grouped by fingerprint
// GET /api/query/clusters?appId=&startTime=&endTime=&limit=
func (h *ClustersHandler) GetClusters(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	appID := r.URL.Query().Get("appId")
	if appID == "" {
		http.Error(w, "Missing appId parameter", http.StatusBadRequest)
		return
	}

	// Parse time range
	var startTime, endTime int64
	if startTimeStr := r.URL.Query().Get("startTime"); startTimeStr != "" {
		if ts, err := strconv.ParseInt(startTimeStr, 10, 64); err == nil {
			startTime = ts
		}
	}
	if endTimeStr := r.URL.Query().Get("endTime"); endTimeStr != "" {
		if ts, err := strconv.ParseInt(endTimeStr, 10, 64); err == nil {
			endTime = ts
		}
	}

	// Parse limit
	limit := parseIntParam(r.URL.Query().Get("limit"), 20)
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	// Query error clusters
	clusters, err := h.db.GetErrorClustersByTime(appID, startTime, endTime, limit)
	if err != nil {
		slog.Error("Failed to get error clusters", "error", err)
		http.Error(w, "Failed to get error clusters", http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"total": len(clusters),
		"data":  clusters,
	}

	json.NewEncoder(w).Encode(response)
}

// GetClusterEvents retrieves events for a specific fingerprint
// GET /api/query/clusters/{fingerprint}/events?page=&pageSize=
func (h *ClustersHandler) GetClusterEvents(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Extract fingerprint from URL path
	// URL pattern: /api/query/clusters/{fingerprint}/events
	fingerprint := extractFingerprintFromPath(r.URL.Path)
	if fingerprint == "" {
		http.Error(w, "Missing fingerprint in path", http.StatusBadRequest)
		return
	}

	appID := r.URL.Query().Get("appId")
	if appID == "" {
		http.Error(w, "Missing appId parameter", http.StatusBadRequest)
		return
	}

	// Parse pagination
	page := parseIntParam(r.URL.Query().Get("page"), 1)
	pageSize := parseIntParam(r.URL.Query().Get("pageSize"), 50)

	if page < 1 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 1000 {
		pageSize = 50
	}

	// Query cluster events
	events, total, err := h.db.GetClusterEvents(appID, fingerprint, page, pageSize)
	if err != nil {
		slog.Error("Failed to get cluster events", "error", err)
		http.Error(w, "Failed to get cluster events", http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"fingerprint": fingerprint,
		"total":       total,
		"page":        page,
		"pageSize":    pageSize,
		"data":        events,
	}

	json.NewEncoder(w).Encode(response)
}

// GetClusterStats retrieves detailed statistics for a specific fingerprint
// GET /api/query/clusters/{fingerprint}/stats
func (h *ClustersHandler) GetClusterStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	fingerprint := extractFingerprintFromPath(r.URL.Path)
	if fingerprint == "" {
		http.Error(w, "Missing fingerprint in path", http.StatusBadRequest)
		return
	}

	appID := r.URL.Query().Get("appId")
	if appID == "" {
		http.Error(w, "Missing appId parameter", http.StatusBadRequest)
		return
	}

	stats, err := h.db.GetClusterStats(appID, fingerprint)
	if err != nil {
		slog.Error("Failed to get cluster stats", "error", err)
		http.Error(w, "Failed to get cluster stats", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(stats)
}

// RegisterRoutes registers all cluster routes
func (h *ClustersHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/query/clusters", h.GetClusters)
	mux.HandleFunc("GET /api/query/clusters/", h.handleClusterWithID)
}

// handleClusterWithID routes requests with fingerprint in path
func (h *ClustersHandler) handleClusterWithID(w http.ResponseWriter, r *http.Request) {
	// Check if this is an events or stats request
	if strings.HasSuffix(r.URL.Path, "/events") {
		h.GetClusterEvents(w, r)
		return
	}
	if strings.HasSuffix(r.URL.Path, "/stats") {
		h.GetClusterStats(w, r)
		return
	}

	// Default to getting cluster info
	h.GetClusters(w, r)
}

// extractFingerprintFromPath extracts fingerprint from URL path
// Path format: /api/query/clusters/{fingerprint}/events or /api/query/clusters/{fingerprint}/stats
func extractFingerprintFromPath(path string) string {
	// Remove prefix "/api/query/clusters/"
	prefix := "/api/query/clusters/"
	if !strings.HasPrefix(path, prefix) {
		return ""
	}

	remaining := strings.TrimPrefix(path, prefix)

	// Remove suffixes like "/events" or "/stats"
	if idx := strings.Index(remaining, "/events"); idx != -1 {
		remaining = remaining[:idx]
	}
	if idx := strings.Index(remaining, "/stats"); idx != -1 {
		remaining = remaining[:idx]
	}

	return remaining
}
