package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/logmonitor/collector/alerter"
	"github.com/logmonitor/collector/storage"
)

// ClustererHandler handles anomaly cluster API requests
type ClustererHandler struct {
	clusterer *alerter.Clusterer
	events    storage.EventStore
}

// NewClustererHandler creates a new clusterer handler
func NewClustererHandler(clusterer *alerter.Clusterer, events storage.EventStore) *ClustererHandler {
	return &ClustererHandler{
		clusterer: clusterer,
		events:    events,
	}
}

// RegisterRoutes registers clusterer routes
func (h *ClustererHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/alerts/clusters", h.ListClusters)
	mux.HandleFunc("GET /api/alerts/clusters/", h.GetCluster)
}

// ListClusters returns active anomaly clusters for an app
func (h *ClustererHandler) ListClusters(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	appID := r.URL.Query().Get("appId")
	if appID == "" {
		http.Error(w, "Missing appId parameter", http.StatusBadRequest)
		return
	}

	limit := 20
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	clusters := h.clusterer.ListClusters(appID, limit)

	response := make([]map[string]interface{}, 0, len(clusters))
	for _, cluster := range clusters {
		response = append(response, convertClusterToMap(cluster))
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"clusters": response,
		"count":    len(response),
	})
}

// GetCluster returns a specific cluster and its events
func (h *ClustererHandler) GetCluster(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Extract fingerprint from path
	// Path format: /api/alerts/clusters/{fingerprint}
	path := r.URL.Path
	fingerprint := ""
	if len(path) > len("/api/alerts/clusters/") {
		fingerprint = path[len("/api/alerts/clusters/"):]
	}

	if fingerprint == "" {
		http.Error(w, "Missing fingerprint", http.StatusBadRequest)
		return
	}

	// Parse query parameters
	appID := r.URL.Query().Get("appId")
	if appID == "" {
		http.Error(w, "Missing appId parameter", http.StatusBadRequest)
		return
	}

	page := 1
	pageSize := 50
	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}
	if pageSizeStr := r.URL.Query().Get("pageSize"); pageSizeStr != "" {
		if ps, err := strconv.Atoi(pageSizeStr); err == nil && ps > 0 {
			pageSize = ps
		}
	}

	// Get cluster info
	cluster, ok := h.clusterer.GetCluster(fingerprint)
	if !ok {
		http.Error(w, "Cluster not found", http.StatusNotFound)
		return
	}

	// Get cluster events with pagination
	events, total, err := h.clusterer.GetClusterEvents(appID, fingerprint, page, pageSize)
	if err != nil {
		slog.Error("Failed to get cluster events", "error", err)
		http.Error(w, "Failed to get cluster events", http.StatusInternalServerError)
		return
	}

	// Convert events to response format
	eventMaps := make([]map[string]interface{}, 0, len(events))
	for _, e := range events {
		eventMaps = append(eventMaps, convertEventRecordToMap(e))
	}

	response := map[string]interface{}{
		"cluster": convertClusterToMap(cluster),
		"events": map[string]interface{}{
			"data":  eventMaps,
			"total": total,
			"page":  page,
			"size":  pageSize,
		},
	}

	json.NewEncoder(w).Encode(response)
}

// Helper functions

func convertClusterToMap(cluster *alerter.Cluster) map[string]interface{} {
	return map[string]interface{}{
		"id":              cluster.ID,
		"fingerprint":     cluster.Fingerprint,
		"message":         cluster.Message,
		"count":           cluster.Count,
		"first_seen":      cluster.FirstSeen,
		"last_seen":       cluster.LastSeen,
		"app_id":          cluster.AppID,
		"severity":        cluster.Severity,
		"similar_clusters": cluster.SimilarClusters,
		"unique_users":    cluster.UniqueUsers,
	}
}

func convertEventRecordToMap(e storage.EventRecord) map[string]interface{} {
	return map[string]interface{}{
		"id":          e.ID,
		"app_id":      e.AppID,
		"release":     e.Release,
		"env":         e.Env,
		"user_id":     e.UserID,
		"session_id":  e.SessionID,
		"type":        e.Type,
		"level":       e.Level,
		"message":     e.Message,
		"stack":       e.Stack,
		"url":         e.URL,
		"line":        e.Line,
		"col":         e.Col,
		"ua":          e.UA,
		"created_at":  e.CreatedAt,
		"fingerprint": e.Fingerprint,
	}
}
