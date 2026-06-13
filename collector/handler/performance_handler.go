package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/logmonitor/collector/storage"
)

// PerformanceHandler handles performance metric queries
type PerformanceHandler struct {
	db *storage.DB
}

// NewPerformanceHandler creates a new performance handler
func NewPerformanceHandler(db *storage.DB) *PerformanceHandler {
	return &PerformanceHandler{
		db: db,
	}
}

// RegisterRoutes registers performance query routes
func (h *PerformanceHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/query/performance/summary-by-page", h.GetPerformanceSummary)
	mux.HandleFunc("GET /api/query/performance/trend-by-page", h.GetPerformanceTrend)
	mux.HandleFunc("GET /api/query/performance/compare-releases", h.GetPerformanceComparison)
}

// GetPerformanceSummary returns performance metrics aggregated by page URL
func (h *PerformanceHandler) GetPerformanceSummary(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Parse project_id
	projectIDStr := r.URL.Query().Get("project_id")
	if projectIDStr == "" {
		http.Error(w, "Missing project_id parameter", http.StatusBadRequest)
		return
	}
	projectID, err := strconv.ParseInt(projectIDStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid project_id", http.StatusBadRequest)
		return
	}

	// Parse metric_name
	metricName := r.URL.Query().Get("metric_name")
	if metricName == "" {
		metricName = "lcp" // Default to LCP
	}

	// Validate metric name
	validMetrics := map[string]bool{
		"fcp":  true,
		"lcp":  true,
		"cls":  true,
		"inp":  true,
		"ttfb": true,
	}
	if !validMetrics[metricName] {
		http.Error(w, "Invalid metric_name. Must be one of: fcp, lcp, cls, inp, ttfb", http.StatusBadRequest)
		return
	}

	// Parse period (default to 7d)
	period := r.URL.Query().Get("period")
	if period == "" {
		period = "7d"
	}

	summary, err := h.db.GetPerformanceSummaryByPage(projectID, metricName, period)
	if err != nil {
		slog.Error("Failed to get performance summary", "error", err)
		http.Error(w, "Failed to get performance summary", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"metric_name": metricName,
		"period":      period,
		"pages":       summary,
		"count":       len(summary),
	})
}

// GetPerformanceTrend returns daily performance trend for a specific page and metric
func (h *PerformanceHandler) GetPerformanceTrend(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Parse project_id
	projectIDStr := r.URL.Query().Get("project_id")
	if projectIDStr == "" {
		http.Error(w, "Missing project_id parameter", http.StatusBadRequest)
		return
	}
	projectID, err := strconv.ParseInt(projectIDStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid project_id", http.StatusBadRequest)
		return
	}

	// Parse page_url
	pageURL := r.URL.Query().Get("page_url")
	if pageURL == "" {
		http.Error(w, "Missing page_url parameter", http.StatusBadRequest)
		return
	}

	// Parse metric_name
	metricName := r.URL.Query().Get("metric_name")
	if metricName == "" {
		metricName = "lcp" // Default to LCP
	}

	// Validate metric name
	validMetrics := map[string]bool{
		"fcp":  true,
		"lcp":  true,
		"cls":  true,
		"inp":  true,
		"ttfb": true,
	}
	if !validMetrics[metricName] {
		http.Error(w, "Invalid metric_name. Must be one of: fcp, lcp, cls, inp, ttfb", http.StatusBadRequest)
		return
	}

	// Parse days (default to 30)
	daysStr := r.URL.Query().Get("days")
	days := 30
	if daysStr != "" {
		days, err = strconv.Atoi(daysStr)
		if err != nil || days <= 0 || days > 90 {
			http.Error(w, "Invalid days. Must be between 1 and 90", http.StatusBadRequest)
			return
		}
	}

	trend, err := h.db.GetPerformanceTrendByPage(projectID, pageURL, metricName, days)
	if err != nil {
		slog.Error("Failed to get performance trend", "error", err)
		http.Error(w, "Failed to get performance trend", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"metric_name": metricName,
		"page_url":    pageURL,
		"days":        days,
		"data":        trend,
		"count":       len(trend),
	})
}

// GetPerformanceComparison compares metrics between two releases
func (h *PerformanceHandler) GetPerformanceComparison(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Parse project_id
	projectIDStr := r.URL.Query().Get("project_id")
	if projectIDStr == "" {
		http.Error(w, "Missing project_id parameter", http.StatusBadRequest)
		return
	}
	projectID, err := strconv.ParseInt(projectIDStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid project_id", http.StatusBadRequest)
		return
	}

	// Parse metric_name
	metricName := r.URL.Query().Get("metric_name")
	if metricName == "" {
		metricName = "lcp" // Default to LCP
	}

	// Validate metric name
	validMetrics := map[string]bool{
		"fcp":  true,
		"lcp":  true,
		"cls":  true,
		"inp":  true,
		"ttfb": true,
	}
	if !validMetrics[metricName] {
		http.Error(w, "Invalid metric_name. Must be one of: fcp, lcp, cls, inp, ttfb", http.StatusBadRequest)
		return
	}

	// Parse releases
	releaseA := r.URL.Query().Get("release_a")
	if releaseA == "" {
		http.Error(w, "Missing release_a parameter", http.StatusBadRequest)
		return
	}

	releaseB := r.URL.Query().Get("release_b")
	if releaseB == "" {
		http.Error(w, "Missing release_b parameter", http.StatusBadRequest)
		return
	}

	comparison, err := h.db.GetPerformanceComparison(projectID, metricName, releaseA, releaseB)
	if err != nil {
		slog.Error("Failed to get performance comparison", "error", err)
		http.Error(w, "Failed to get performance comparison", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"metric_name": metricName,
		"release_a":   releaseA,
		"release_b":   releaseB,
		"comparisons": comparison,
		"count":       len(comparison),
	})
}
